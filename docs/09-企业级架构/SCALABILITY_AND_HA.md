# ytb2bili 可扩展性与高可用性开发文档

> **版本**: v1.0
> **日期**: 2025-12-29
> **目标读者**: 架构师、高级开发人员、DevOps 工程师

---

## 📋 目录

1. [执行摘要](#执行摘要)
2. [当前架构分析](#当前架构分析)
3. [单点故障识别](#单点故障识别)
4. [性能瓶颈分析](#性能瓶颈分析)
5. [分布式改造方案](#分布式改造方案)
6. [实施路线图](#实施路线图)
7. [监控与运维](#监控与运维)
8. [成本与收益分析](#成本与收益分析)
9. [风险评估与应对措施](#风险评估与应对措施)
10. [灰度发布策略](#灰度发布策略)
11. [数据迁移策略](#数据迁移策略)
12. [总结与建议](#总结与建议)

---

## 执行摘要

### 🎯 当前状态

ytb2bili 是一个**单机架构**的视频转存系统，具备良好的代码结构，但存在以下可扩展性和高可用性限制：

| 维度 | 当前状态 | 企业级要求 | 差距 |
|------|---------|-----------|------|
| **水平扩展** | ❌ 不支持多实例 | ✅ 可动态扩容 | **严重** |
| **故障恢复** | ⚠️ 应用重启恢复 | ✅ 自动故障转移 | **中等** |
| **任务调度** | ⚠️ 单机内存调度 | ✅ 分布式队列 | **严重** |
| **数据一致性** | ✅ 基本保证 | ✅ 强一致性 + 最终一致性 | **良好** |
| **服务发现** | ❌ 无 | ✅ 服务注册中心 | **严重** |

### 📊 总体评估

**可扩展性评分**: **3/10**
**高可用性评分**: **4/10**
**综合评分**: **3.5/10**

**结论**: 系统为**单机应用**，不支持水平扩展，存在多个单点故障，距离企业级要求有明显差距。

---

## 当前架构分析

### 架构图（单机版本）

```
┌─────────────────────────────────────────────────────────────┐
│                       用户浏览器                             │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                   Nginx (可选)                               │
│                   反向代理 + 静态文件                          │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                  ytb2bili.exe (单实例)                       │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Gin Web Server (端口 8096)                          │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  ChainTaskHandler (准备阶段调度器)                    │  │
│  │  - Cron: 每5秒扫描                                   │  │
│  │  - Worker Pool: 10个并发                             │  │
│  │  - 内存状态: inFlightTasks (sync.Map)                │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  UploadScheduler (上传阶段调度器)                      │  │
│  │  - Cron: 每10秒扫描                                  │  │
│  │  - 全局锁: mutex (保护整个调度逻辑)                   │  │
│  │  - 内存状态: lastVideoUploadTime                      │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  TaskChain (任务链执行器)                             │  │
│  │  - 依赖关系管理                                       │  │
│  │  - 并行执行无依赖任务                                 │  │
│  │  - 状态持久化到数据库                                 │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                        │
        ┌───────────────┼───────────────┐
        ▼               ▼               ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│   SQLite/    │ │  Bilibili    │ │   yt-dlp     │
│   MySQL DB   │ │    API       │ │  (本地二进制) │
└──────────────┘ └──────────────┘ └──────────────┘
```

### 🔴 关键限制

#### 1. **基于内存的任务调度** ⚠️

**位置**: `internal/chain_task/chain_task_handler.go:37-44`

```go
type ChainTaskHandler struct {
    workerPool    chan struct{}  // ❌ 单机内存 channel
    inFlightTasks sync.Map       // ❌ 单机内存 map
    Task          *cron.Cron      // ❌ 单机定时器
}
```

**问题**:
- ❌ **不支持多实例**: 如果部署2台服务器，会重复处理同一个任务
- ❌ **任务丢失风险**: 应用崩溃时，内存中正在执行的任务状态丢失
- ❌ **无法负载均衡**: 所有流量集中在一台服务器

#### 2. **全局锁限制并发** ⚠️

**位置**: `internal/chain_task/upload_scheduler.go:83`

```go
func (s *UploadScheduler) uploadNextVideo() error {
    s.mutex.Lock()         // ❌ 全局互斥锁
    defer s.mutex.Unlock()

    // 整个调度逻辑被锁住
    s.uploadNextVideo()
}
```

**问题**:
- ❌ **串行上传**: 即使用户A和用户B同时上传，也必须排队
- ❌ **性能瓶颈**: 锁竞争严重时，CPU利用率低
- ❌ **无法水平扩展**: 加服务器也无法解决锁问题

#### 3. **单点故障** ⚠️

**单点列表**:
| 组件 | 故障影响 | 恢复时间 | HA要求 |
|------|---------|---------|--------|
| ytb2bili.exe | 服务完全不可用 | 需人工重启 | ❌ 无HA |
| MySQL/SQLite | 数据不可访问 | 需备份恢复 | ❌ 无主从 |
| Cron调度器 | 任务停止调度 | 应用重启后恢复 | ❌ 无HA |
| 内存状态 | 任务状态丢失 | 无法恢复 | ❌ 数据丢失 |

---

## 单点故障识别

### 🔴 P0 - 严重单点故障

#### 1. **应用程序实例** ❌

**故障场景**:
```
场景1: 服务器宕机
  - 影响: 所有服务不可用
  - 恢复: 需人工重启应用
  - 数据丢失: 内存中正在执行的任务状态丢失

场景2: 应用崩溃 (panic/OOM)
  - 影响: 服务不可用
  - 恢复: 需人工重启或进程管理器重启
  - 数据丢失: 未持久化的状态丢失
```

**HA要求**:
- ✅ 部署多个实例
- ✅ 负载均衡器自动故障转移
- ✅ 健康检查 + 自动重启

#### 2. **数据库** ⚠️

**当前状态**:
- SQLite: 单机文件数据库，完全不支持并发写入
- MySQL: 可能是单实例（需确认配置）

**故障场景**:
```
场景1: 数据库服务器宕机
  - 影响: 无法访问数据，服务降级
  - 恢复: 需重启数据库
  - 数据风险: 可能数据损坏

场景2: 磁盘满
  - 影响: 写入失败，任务卡住
  - 恢复: 清理磁盘
```

**HA要求**:
- ✅ 主从复制 (Master-Slave)
- ✅ 自动故障转移 (MHA/Orchestrator)
- ✅ 读写分离
- ✅ 定期备份

#### 3. **任务调度器** ❌

**位置**: `cron.Cron` 运行在应用进程内

**故障场景**:
```
场景: 应用重启时
  - 影响: 调度器停止，新任务不再被调度
  - 恢复: 应用重启后自动恢复
  - 问题: 重启期间的调度任务不会补偿执行
```

**HA要求**:
- ✅ 分布式任务队列 (Redis/Kafka)
- ✅ 任务持久化到数据库
- ✅ 多实例竞争消费 (避免重复执行)

---

### 🟡 P1 - 次要单点故障

#### 4. **文件存储** ⚠️

**当前**: 本地文件系统 `data/media/`

**问题**:
- ❌ 无法多实例共享 (每台服务器有自己的文件)
- ❌ 磁盘空间限制
- ❌ 无备份机制

**HA要求**:
- ✅ 对象存储 (S3/OSS/COS)
- ✅ CDN加速
- ✅ 文件同步 (多实例共享)

#### 5. **配置管理** ⚠️

**当前**: `config.toml` 本地文件

**问题**:
- ❌ 修改配置需重启应用
- ❌ 多实例配置可能不一致
- ❌ 敏感信息明文存储

**HA要求**:
- ✅ 配置中心 (Nacos/Consul)
- ✅ 动态配置刷新
- ✅ 敏感信息加密存储

---

## 性能瓶颈分析

### 📊 当前性能特征

#### 1. **并发处理能力**

**准备阶段** (ChainTaskHandler):
```
最大并发: 10个任务 (可配置)
下载并发: 3个 (可配置)

瓶颈:
- ❌ 单机内存限制: goroutine数量受内存限制
- ❌ 锁竞争: 全局mutex限制并发效率
- ❌ 磁盘IO: 大量并发下载会耗尽磁盘IO
```

**上传阶段** (UploadScheduler):
```
视频上传: 1个/小时 (可配置延迟)
字幕上传: 视频完成后1小时

瓶颈:
- ❌ 全局锁: 所有用户串行上传
- ❌ B站API限流: 无法避免
- ❌ 无并发: 即使多用户也排队
```

#### 2. **数据库性能**

**查询模式**:
```go
// 每5秒执行一次
pendingTasks := s.DB.Table("cw_saved_videos").
    Where("status = ?", "001").
    Find(&tasks)

// 问题: 无索引优化
// 问题: 每次全表扫描 status='001'
```

**瓶颈**:
- ⚠️ **缺少复合索引**: `WHERE user_id=? AND status=?`
- ⚠️ **无连接池配置**: 使用默认配置，可能连接数不足
- ⚠️ **N+1查询**: 获取任务详情时可能多次查询

#### 3. **网络IO瓶颈**

**yt-dlp下载**:
```
限制: 3个并发下载
瓶颈:
- ❌ 本地带宽: 单机出口带宽有限
- ❌ YouTube限流: 可能被封IP
- ❌ 磁盘写入: 大量并发写盘
```

**B站上传**:
```
限制: 1个/小时
瓶颈:
- ❌ B站审核时间: 上传后需等待审核
- ❌ 无并发: 无法利用多账号并行
```

---

## 分布式改造方案

### 🎯 改造目标

1. **水平扩展**: 支持部署多个实例，线性提升处理能力
2. **故障隔离**: 单个实例故障不影响其他实例
3. **任务可靠**: 任务持久化，不丢失、不重复
4. **状态一致**: 多实例状态下数据一致性

---

### 📐 推荐架构（分布式版本）

```
┌─────────────────────────────────────────────────────────────────┐
│                          负载均衡层                              │
│                   Nginx / CloudFlare / ALB                     │
└───────────────────────────────┬─────────────────────────────────┘
                                │
            ┌───────────────────┼───────────────────┐
            ▼                   ▼                   ▼
┌───────────────────┐ ┌───────────────────┐ ┌───────────────────┐
│  ytb2bili Node 1  │ │  ytb2bili Node 2  │ │  ytb2bili Node N  │
│  ┌─────────────┐  │ │  ┌─────────────┐  │ │  ┌─────────────┐  │
│  │ Gin Server  │  │ │  │ Gin Server  │  │ │  │ Gin Server  │  │
│  └─────────────┘  │ │  └─────────────┘  │ │  └─────────────┘  │
│  ┌─────────────┐  │ │  ┌─────────────┐  │ │  ┌─────────────┐  │
│  │  Task Worker│  │ │  │  Task Worker│  │ │  │  Task Worker│  │
│  └─────────────┘  │ │  └─────────────┘  │ │  └─────────────┘  │
└───────────┬───────┘ └───────┬───────────┘ └───────┬───────────┘
            │                   │                   │
            └───────────────────┼───────────────────┘
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                        消息队列层 (Redis)                       │
│  ┌───────────────────┐    ┌───────────────────┐                 │
│  │  Task Queue      │    │  Result Queue    │                 │
│  │  (Asynq/Machinery)│    │  (Pub/Sub)        │                 │
│  └───────────────────┘    └───────────────────┘                 │
│  ┌───────────────────┐    ┌───────────────────┐                 │
│  │  Distributed Lock│    │  Rate Limiter     │                 │
│  │  (Redsync)        │    │  (Redis+Lua)      │                 │
│  └───────────────────┘    └───────────────────┘                 │
└─────────────────────────────────────────────────────────────────┘
                                │
            ┌───────────────────┼───────────────────┐
            ▼                   ▼                   ▼
┌───────────────────┐ ┌───────────────────┐ ┌───────────────────┐
│   Redis Cluster   │ │   MySQL Master    │ │   Object Storage  │
│   (Cache+Queue)   │ │   (Primary DB)     │ │   (S3/OSS/COS)    │
└───────────────────┘ └───────────────────┘ └───────────────────┘
                            │
                            ▼
                  ┌───────────────────┐
                  │   MySQL Slave     │
                  │   (Read Replica)  │
                  └───────────────────┘
```

---

### 🔧 核心改造方案

#### 方案1: 分布式任务队列

**目标**: 替换单机 Cron + 内存 Channel

**技术选型**:
- **推荐**: [hibiken/asynq](https://github.com/hibiken/asynq) (基于Redis)
- **备选**: [RabbitMQ](https://www.rabbitmq.com/) + [amqp091-go](https://github.com/rabbitmq/amqp091-go)

**改造内容**:

**当前代码** (`chain_task_handler.go:91-100`):
```go
// ❌ 单机 Cron + 内存 Channel
h.Task.AddFunc("*/5 * * * * *", func() {
    tasks := h.getPendingTasks()

    for _, task := range tasks {
        select {
        case h.workerPool <- struct{}{}:
            go h.RunTaskChain(task)
        default:
            // 满了，跳过
        }
    }
})
```

**改造后** (基于 Asynq):
```go
// ✅ 分布式任务队列
package tasks

import (
    "context"
    "github.com/hibiken/asynq"
)

type TaskProcessor struct {
    asynqClient *asynq.Client
    asynqServer *asynq.Server
    workerPool  chan struct{}
}

// 1. 生产者：入队任务
func (p *TaskProcessor) EnqueueTask(videoID string, userID uint) error {
    task := asynq.NewTask(
        "process-video",
        asynq.Payload{
            "video_id": videoID,
            "user_id":  userID,
        },
    )

    _, err := p.asynqClient.Enqueue(
        task,
        asynq.Queue("video-processing"),  // 独立队列
    )
    return err
}

// 2. 消费者：处理任务
func (p *TaskProcessor) StartWorker() {
    srv := asynq.NewServer(
        asynq.RedisClientOpt{Addr: "localhost:6379"},
        asynq.Config{
            Concurrency: 10,  // 最大并发
            Queues: []string{
                "video-processing",  // 优先级1
                "subtitle-upload",   // 优先级2
            },
        },
    )

    srv.HandleFunc("process-video", p.ProcessVideoTask)

    if err := srv.Run(); err != nil {
        log.Fatalf("启动 worker 失败: %v", err)
    }
}

func (p *TaskProcessor) ProcessVideoTask(ctx context.Context, t *asynq.Task) error {
    videoID := t.Payload()["video_id"].(string)
    userID := t.Payload()["user_id"].(uint)

    // 执行任务链
    h.RunTaskChain(videoID, userID)

    return nil
}
```

**优势**:
- ✅ **多实例安全**: Redis 作为消息代理，多实例不会重复消费
- ✅ **任务持久化**: 任务保存在 Redis，应用重启不丢失
- ✅ **重试机制**: 内置指数退避重试
- ✅ **优先级队列**: 支持高优先级任务插队
- ✅ **可观测性**: 提供 Web UI 监控队列状态

---

#### 方案2: 分布式锁

**目标**: 替换全局 `mutex.Mutex`

**技术选型**:
- **推荐**: [redsync](https://github.com/go-redsync/redsync) (基于Redis)
- **备选**: [etcd](https://etcd.io/) + [clientv3](https://github.com/etcd-io/etcd)

**改造内容**:

**当前代码** (`upload_scheduler.go:83`):
```go
// ❌ 全局互斥锁
type UploadScheduler struct {
    mutex sync.Mutex
}

func (s *UploadScheduler) uploadNextVideo() error {
    s.mutex.Lock()
    defer s.mutex.Unlock()

    // 串行上传逻辑
}
```

**改造后** (基于 Redsync):
```go
// ✅ 分布式锁（用户级）
type UploadScheduler struct {
    redisClient *redis.Client
    pool        *redsync.Pool
}

func (s *UploadScheduler) uploadNextVideo() error {
    // 获取用户级锁
    mutexname := fmt.Sprintf("upload:user:%d", userID)
    mutex := s.pool.NewMutex(
        mutexname,
        redsync.WithExpiry(10*time.Minute),
        redsync.WithTries(1),
    )

    if err := mutex.Lock(); err != nil {
        return fmt.Errorf("获取锁失败: %w", err)
    }
    defer mutex.Unlock()

    // 执行上传（用户级并发，全局串行）
}
```

**优势**:
- ✅ **用户级并发**: 不同用户可并行上传
- ✅ **跨实例同步**: 多实例共享锁状态
- ✅ **自动过期**: 锁过期自动释放，避免死锁
- ✅ **可重试**: 内置重试机制

---

#### 方案3: 读写分离 + 连接池

**目标**: 优化数据库性能

**改造内容**:

**当前代码** (推测):
```go
// ❌ 默认配置
db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
```

**改造后**:
```go
// ✅ 主从配置
func NewDatabase(config *types.AppConfig) (*gorm.DB, error) {
    // 主库（写）
    writeDSN := config.DBConfig.MasterDSN
    db, err := gorm.Open(mysql.Open(writeDSN), &gorm.Config{
        Logger: logger.Default.LogMode(logger.Info),
    })

    sqlDB, _ := db.DB()
    sqlDB.SetMaxIdleConns(10)           // 最大空闲连接
    sqlDB.SetMaxOpenConns(100)          // 最大打开连接
    sqlDB.SetConnMaxLifetime(time.Hour) // 连接最大存活时间

    // 从库（读）
    readDB, err := gorm.Open(mysql.Open(config.DBConfig.SlaveDSN), &gorm.Config{})

    // 使用插件实现读写分离
    db.Use(dbresolver.Register(dbresolver.Config{
        // 主库
        Replicas: []gorm.Dialector{
            mysql.Open(config.DBConfig.SlaveDSN),
        },
        // 负载均衡策略
        Policy: dbresolver.RandomPolicy{},
    }))

    return db, nil
}
```

**优势**:
- ✅ **读写分离**: 查询分散到从库，减轻主库压力
- ✅ **连接池复用**: 避免频繁创建连接
- ✅ **负载均衡**: 多个从库自动分配查询

---

#### 方案4: 对象存储

**目标**: 替换本地文件系统

**技术选型**:
- **推荐**: [腾讯云 COS](https://cloud.tencent.com/product/cos) (已集成)
- **备选**: AWS S3 / 阿里云 OSS

**改造内容**:

**当前代码** (推测):
```go
// ❌ 本地文件系统
filePath := fmt.Sprintf("data/media/%s.mp4", videoID)
os.WriteFile(filePath, data, 0644)
```

**改造后**:
```go
// ✅ 对象存储
func (s *StorageService) SaveVideo(videoID string, data []byte) (string, error) {
    key := fmt.Sprintf("videos/%s/%s.mp4", userID, videoID)

    _, err := s.cosClient.Upload(context.Background(),
        bucketName, key, bytes.NewReader(data))

    if err != nil {
        return "", fmt.Errorf("上传到COS失败: %w", err)
    }

    // 返回访问URL
    url := s.cosClient.GetObjectURL(bucketName, key)
    return url, nil
}

// 使用时
videoURL, err := storageService.SaveVideo(videoID, videoData)
// 保存 videoURL 到数据库，而非本地路径
```

**优势**:
- ✅ **无限扩展**: 存储空间不受本地磁盘限制
- ✅ **多实例共享**: 所有实例访问同一对象存储
- ✅ **CDN加速**: 对象存储自带CDN
- ✅ **高可用**: 对象存储厂商保证99.999999999%持久性

---

### 📊 改造前后对比

| 维度 | 改造前 | 改造后 |
|------|--------|--------|
| **实例数量** | 1个 | N个 (动态扩容) |
| **任务调度** | 单机Cron | 分布式队列 |
| **并发模型** | 全局锁 | 用户级锁 |
| **文件存储** | 本地FS | 对象存储 |
| **数据库** | 单实例 | 主从复制 |
| **配置管理** | 本地文件 | 配置中心 |
| **可观测性** | 本地日志 | Prometheus + Grafana |

---

## 实施路线图

### 🎯 第一阶段: 夯实基础（1-2周）

**目标**: 修复P0问题，为分布式改造打基础

**任务清单**:
1. ✅ **修复事务问题** (P0)
   - 添加 `defer Rollback` 到所有事务
   - 添加事务隔离级别配置

2. ✅ **修复槽位泄漏** (P0)
   - 修正 ConcurrencyLimiter.Release() 调用时机
   - 添加槽位泄漏检测

3. ✅ **数据库连接池配置** (P0)
   - 设置 MaxIdleConns, MaxOpenConns
   - 设置 ConnMaxLifetime

4. ✅ **添加数据库索引** (P1)
   ```sql
   CREATE INDEX idx_user_status ON cw_saved_videos(user_id, status);
   CREATE INDEX idx_status_created ON cw_saved_videos(status, created_at);
   CREATE UNIQUE INDEX idx_video_step ON cw_task_steps(video_id, step_name);
   ```

**验收标准**:
- ✅ 所有事务都有 defer Rollback 保护
- ✅ 无槽位泄漏（运行24小时无异常）
- ✅ 数据库连接池正常工作
- ✅ 索引生效，查询性能提升50%+

---

### 🎯 第二阶段: 分布式改造（3-4周）

**目标**: 核心组件分布式化

**任务清单**:

#### Week 1: Redis 基础设施
- [ ] 搭建 Redis 集群（或使用云 Redis）
- [ ] 集成 Redsync 分布式锁
- [ ] 迁移 UploadScheduler 全局锁
- [ ] 单元测试 + 压力测试

#### Week 2: 任务队列
- [ ] 集成 Asynq 任务队列
- [ ] 迁移 ChainTaskHandler 到 Asynq
- [ ] 实现 Task 生产者/消费者
- [ ] Web UI 监控队列状态

#### Week 3: 数据库优化
- [ ] 搭建 MySQL 主从复制
- [ ] 实现读写分离
- [ ] 数据迁移脚本（如果有数据）
- [ ] 压力测试主从切换

#### Week 4: 存储迁移
- [ ] 迁移视频文件到 COS
- [ ] 更新数据库存储URL而非本地路径
- [ ] 清理本地旧文件
- [ ] CDN配置

**验收标准**:
- ✅ 可部署多个实例
- ✅ 多实例不会重复处理任务
- ✅ 主库宕机，从库可接管读流量
- ✅ 对象存储可正常访问

---

### 🎯 第三阶段: 云原生与监控（2-3周）

**目标**: 完善运维体系

**任务清单**:

#### Week 1: 容器化
- [ ] 编写 Dockerfile
- [ ] Docker Compose 编排
- [ ] K8s Deployment/Service 配置
- [ ] Helm Chart (可选)

#### Week 2: 监控与告警
- [ ] 集成 Prometheus Metrics
- [ ] Grafana 仪表板
- [ ] 告警规则（任务失败、队列积压等）
- [ ] 日志聚合（ELK/Loki）

#### Week 3: CI/CD
- [ ] GitHub Actions 工作流
- [ ] 自动化测试
- [ ] 自动化构建镜像
- [ ] 自动化部署

**验收标准**:
- ✅ 可一键部署到 K8s
- ✅ Prometheus 采集指标正常
- ✅ 告警及时触发
- ✅ CI/CD 全自动

---

## 监控与运维

### 📊 关键指标

#### 业务指标

```go
// 任务处理速率
var taskProcessedTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "task_processed_total",
        Help: "任务处理总数",
    },
    []string{"task_type", "status"},
)

// 任务处理时长
var taskDurationSeconds = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "task_duration_seconds",
        Help: "任务处理时长",
        Buckets: prometheus.LinearBuckets(0, 60, 10), // 0-600秒
    },
    []string{"task_name"},
)

// 队列积压数
var queueBacklogGauge = prometheus.NewGauge(
    prometheus.GaugeOpts{
        Name: "queue_backlog",
        Help: "队列积压任务数",
    },
)

// 用户并发数
var activeUsersGauge = prometheus.NewGauge(
    prometheus.GaugeOpts{
        Name: "active_users",
        Help: "当前活跃用户数",
    },
)
```

#### 系统指标

| 指标 | 目标值 | 告警阈值 |
|------|--------|---------|
| CPU使用率 | <70% | >90% 持续5分钟 |
| 内存使用率 | <80% | >95% |
| 磁盘使用率 | <70% | >90% |
| 任务处理延迟 | <30s | >60s P99 |
| 队列积压 | <100 | >1000 |
| 数据库连接数 | <80% | >90% |

### 🔔 告警规则

```yaml
# Prometheus 告警规则
groups:
  - name: ytb2bili_alerts
    interval: 30s
    rules:
      # 任务积压
      - alert: HighTaskBacklog
        expr: queue_backlog > 1000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "任务队列积压过多"
          description: "当前积压 {{ $value }} 个任务"

      # 处理延迟过高
      - alert: HighTaskLatency
        expr: histogram_quantile(0.99, task_duration_seconds) > 60
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "任务处理延迟过高"
          description: "P99延迟 {{ $value }}秒"

      # 应用宕机
      - alert: ApplicationDown
        expr: up{job="ytb2bili"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "应用实例宕机"
          description: "{{ $labels.instance }} 不可达"

      # 数据库连接满
      - alert: DatabaseConnectionPoolFull
        expr: mysql_global_status_threads_connected / mysql_global_variables_max_connections > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "数据库连接池接近满"
```

---

## 成本与收益分析

### 💰 成本估算

#### 基础设施成本（月度）

| 组件 | 规格 | 单价 | 数量 | 小计 |
|------|------|------|------|------|
| **应用服务器** | 4核8G | ¥200/月 | 2台 | ¥400 |
| **Redis** | 1G标准版 | ¥150/月 | 1个 | ¥150 |
| **MySQL** | 2核4G | ¥300/月 | 1主1从 | ¥600 |
| **对象存储** | 1TB | ¥120/月 | 1个 | ¥120 |
| **负载均衡** | 标准版 | ¥100/月 | 1个 | ¥100 |
| **监控** | Grafana Cloud | 免费 | 1套 | ¥0 |
| **合计** | - | - | - | **¥1,370/月** |

> **年度成本**: ¥16,440

#### 开发成本

| 任务 | 工时 | 人天 | 单价 | 小计 |
|------|------|------|------|------|
| 架构设计 | 3天 | 3 | ¥2000/天 | ¥6,000 |
| 分布式改造 | 20天 | 20 | ¥1500/天 | ¥30,000 |
| 测试验证 | 10天 | 10 | ¥1000/天 | ¥10,000 |
| 文档编写 | 3天 | 3 | ¥1000/天 | ¥3,000 |
| **合计** | - | - | - | **¥49,000** |

> **一次性投入**: ¥49,000

---

### 📈 收益分析

#### 性能提升

| 指标 | 改造前 | 改造后 | 提升 |
|------|--------|--------|------|
| 并发处理能力 | 10任务/次 | 100+任务/次 | **10倍+** |
| 上传并发 | 1用户/次 | N用户并行 | **无限** |
| 可用性 | 95% | 99.9% | **+4.9%** |
| 故障恢复时间 | 人工重启 | 自动转移 | **<1分钟** |

#### 运维效率

| 场景 | 改造前 | 改造后 |
|------|--------|--------|
| 扩容 | 需人工迁移配置 | 加镜像自动扩容 |
| 部署 | 手动部署 | CI/CD自动部署 |
| 故障发现 | 用户反馈 | 告警主动通知 |
| 问题定位 | 查日志grep | Grafana仪表板 |

---

### 💡 ROI分析

**场景1: 用户增长10倍**
- 改造前: 需要优化代码、垂直升级硬件（成本高）
- 改造后: 加服务器即可（成本线性增长）

**场景2: 服务器故障**
- 改造前: 服务不可用，用户流失
- 改造后: 自动切换，用户无感知

**场景3: 双11大促**
- 改造前: 不敢活动，怕系统扛不住
- 改造后: 提前扩容，活动结束缩容

**投资回报期**: 约 **6-12个月**

---

## 风险评估与应对措施

### 🔴 P0 - 高风险项

#### 1. **数据迁移风险**

| 风险点 | 影响 | 概率 | 应对措施 |
|--------|------|------|----------|
| 数据丢失 | 用户任务记录丢失 | 低 | 迁移前全量备份，迁移后数据校验 |
| 数据不一致 | 任务状态错乱 | 中 | 停机迁移 + 双写验证 |
| 迁移时间超预期 | 停机时间过长 | 中 | 分批迁移 + 预演测试 |

**回滚方案**:
```bash
# 数据迁移失败时的回滚脚本
#!/bin/bash
echo "🔄 开始回滚数据库..."

# 1. 停止新版本应用
systemctl stop ytb2bili-new

# 2. 恢复旧版本数据库
mysql -u root -p ytb2bili < /backup/pre_migration_$(date +%Y%m%d).sql

# 3. 启动旧版本应用
systemctl start ytb2bili-old

echo "✅ 回滚完成"
```

#### 2. **服务中断风险**

| 风险点 | 影响 | 概率 | 应对措施 |
|--------|------|------|----------|
| 新版本 Bug | 服务不可用 | 中 | 灰度发布，小流量验证 |
| 配置错误 | 启动失败 | 中 | 配置校验脚本 + 健康检查 |
| 依赖服务故障 | Redis/MySQL 不可达 | 低 | 熔断降级 + 告警通知 |

**熔断降级配置**:
```go
// 当 Redis 不可用时，降级到本地内存锁
func (s *UploadScheduler) acquireLock(userID uint) (bool, error) {
    // 尝试分布式锁
    if s.redisClient != nil {
        lock, err := s.redsync.NewMutex(fmt.Sprintf("upload:%d", userID)).Lock()
        if err == nil {
            return true, nil
        }
        log.Warnf("⚠️ Redis 锁获取失败，降级到本地锁: %v", err)
    }
    
    // 降级：使用本地 sync.Mutex
    s.fallbackMutex.Lock()
    return true, nil
}
```

---

### 🟡 P1 - 中风险项

#### 3. **性能回退风险**

| 风险点 | 影响 | 概率 | 应对措施 |
|--------|------|------|----------|
| 分布式锁性能开销 | 请求延迟增加 | 中 | 压测验证，设置锁超时 |
| 队列积压 | 任务处理变慢 | 中 | 监控告警，动态扩容 |
| 网络抖动 | Redis 连接超时 | 低 | 重试机制 + 连接池 |

#### 4. **运维复杂度增加**

| 风险点 | 影响 | 概率 | 应对措施 |
|--------|------|------|----------|
| 组件增多 | 故障排查困难 | 高 | 完善监控仪表板 |
| 配置分散 | 配置管理混乱 | 中 | 配置中心集中管理 |
| 学习成本 | 团队不熟悉 | 中 | 文档 + 培训 |

---

## 灰度发布策略

### 🎯 发布原则

1. **小步快跑**: 每次只发布一个核心改动
2. **可观测**: 灰度期间必须有完善监控
3. **可回滚**: 任何阶段都能在 5 分钟内回滚
4. **用户无感**: 灰度切换对用户透明

### 📐 灰度架构

```
                    ┌─────────────────────┐
                    │    负载均衡 (Nginx)  │
                    │    流量分发规则       │
                    └──────────┬──────────┘
                               │
              ┌────────────────┼────────────────┐
              │ 90% 流量       │                │ 10% 流量
              ▼                │                ▼
    ┌─────────────────┐        │      ┌─────────────────┐
    │  稳定版 v1.0    │        │      │  灰度版 v1.1    │
    │  (3 实例)       │        │      │  (1 实例)       │
    └─────────────────┘        │      └─────────────────┘
              │                │                │
              └────────────────┼────────────────┘
                               ▼
                    ┌─────────────────────┐
                    │   共享基础设施       │
                    │  Redis / MySQL / COS│
                    └─────────────────────┘
```

### 🔄 灰度流程

#### 阶段1: 内部测试 (Day 1-2)
```yaml
# 灰度规则：仅内部用户
upstream_rules:
  - match:
      headers:
        X-Internal-User: "true"
    route: v1.1-canary
  - default:
    route: v1.0-stable
```

**验证项**:
- [ ] 核心功能可用（下载、上传、字幕）
- [ ] 无 Error 级别日志
- [ ] 响应时间 P99 < 500ms

#### 阶段2: 小流量灰度 (Day 3-5)
```yaml
# 灰度规则：10% 随机流量
upstream_rules:
  - weight: 10
    route: v1.1-canary
  - weight: 90
    route: v1.0-stable
```

**监控指标**:
| 指标 | 稳定版基线 | 灰度版阈值 | 触发回滚 |
|------|-----------|-----------|----------|
| 错误率 | 0.1% | < 0.5% | > 1% |
| P99 延迟 | 200ms | < 300ms | > 500ms |
| 任务成功率 | 98% | > 95% | < 90% |

#### 阶段3: 扩大灰度 (Day 6-7)
```yaml
# 灰度规则：50% 流量
upstream_rules:
  - weight: 50
    route: v1.1-canary
  - weight: 50
    route: v1.0-stable
```

#### 阶段4: 全量发布 (Day 8)
```yaml
# 全量切换
upstream_rules:
  - weight: 100
    route: v1.1-stable
```

### ⏪ 快速回滚脚本

```bash
#!/bin/bash
# rollback.sh - 一键回滚脚本

STABLE_VERSION="v1.0.5"
CANARY_VERSION="v1.1.0"

echo "🚨 开始紧急回滚..."

# 1. 切换流量到稳定版
kubectl patch virtualservice ytb2bili -p '
spec:
  http:
  - route:
    - destination:
        host: ytb2bili
        subset: stable
      weight: 100
'

# 2. 缩容灰度版本
kubectl scale deployment ytb2bili-canary --replicas=0

# 3. 验证稳定版健康
for i in {1..5}; do
  if curl -s http://localhost:8096/health | grep -q "ok"; then
    echo "✅ 回滚成功，服务恢复正常"
    exit 0
  fi
  sleep 2
done

echo "❌ 回滚后健康检查失败，请人工介入"
exit 1
```

---

## 数据迁移策略

### 🎯 迁移场景

| 场景 | 复杂度 | 停机时间 | 推荐方案 |
|------|--------|----------|----------|
| SQLite → MySQL | 中 | 30分钟 | 停机迁移 |
| MySQL 单机 → 主从 | 低 | 5分钟 | 在线复制 |
| 本地文件 → 对象存储 | 高 | 无停机 | 增量同步 |

### 📋 SQLite → MySQL 迁移

#### 迁移前准备

```bash
# 1. 备份现有数据
cp data/ytb2bili.db data/ytb2bili.db.backup.$(date +%Y%m%d)

# 2. 导出为 SQL
sqlite3 data/ytb2bili.db .dump > /backup/sqlite_dump.sql

# 3. 统计数据量
sqlite3 data/ytb2bili.db "SELECT COUNT(*) FROM cw_saved_videos;"
sqlite3 data/ytb2bili.db "SELECT COUNT(*) FROM cw_task_steps;"
```

#### 迁移脚本

```go
// migrate_sqlite_to_mysql.go
package main

import (
    "database/sql"
    "log"
    
    _ "github.com/go-sql-driver/mysql"
    _ "github.com/mattn/go-sqlite3"
)

func main() {
    // 1. 连接 SQLite
    sqliteDB, err := sql.Open("sqlite3", "data/ytb2bili.db")
    if err != nil {
        log.Fatalf("连接 SQLite 失败: %v", err)
    }
    defer sqliteDB.Close()
    
    // 2. 连接 MySQL
    mysqlDB, err := sql.Open("mysql", "user:pass@tcp(localhost:3306)/ytb2bili")
    if err != nil {
        log.Fatalf("连接 MySQL 失败: %v", err)
    }
    defer mysqlDB.Close()
    
    // 3. 迁移视频表
    migrateTable(sqliteDB, mysqlDB, "cw_saved_videos")
    
    // 4. 迁移任务步骤表
    migrateTable(sqliteDB, mysqlDB, "cw_task_steps")
    
    // 5. 迁移用户表
    migrateTable(sqliteDB, mysqlDB, "cw_users")
    
    log.Println("✅ 数据迁移完成")
}

func migrateTable(src, dst *sql.DB, table string) {
    rows, _ := src.Query("SELECT * FROM " + table)
    defer rows.Close()
    
    cols, _ := rows.Columns()
    count := 0
    
    for rows.Next() {
        // 动态读取行数据并插入 MySQL
        // ... 实现略
        count++
    }
    
    log.Printf("📦 迁移 %s: %d 条记录", table, count)
}
```

#### 数据校验

```sql
-- 校验记录数一致性
SELECT 
    (SELECT COUNT(*) FROM cw_saved_videos) AS mysql_videos,
    -- 与 SQLite 导出的数量比对
    
-- 校验关键字段
SELECT video_id, status, created_at 
FROM cw_saved_videos 
ORDER BY id DESC 
LIMIT 10;
```

### 📋 本地文件 → 对象存储迁移

#### 增量同步脚本

```bash
#!/bin/bash
# sync_to_cos.sh - 增量同步到腾讯云 COS

LOCAL_DIR="/data/ytb2bili/media"
COS_BUCKET="ytb2bili-media-1234567890"
COS_REGION="ap-guangzhou"

echo "🔄 开始同步到 COS..."

# 使用 coscmd 增量同步
coscmd config -a $COS_SECRET_ID -s $COS_SECRET_KEY -b $COS_BUCKET -r $COS_REGION

# 增量同步（只上传新增/修改的文件）
coscmd upload -r --sync $LOCAL_DIR/ /

# 统计结果
echo "✅ 同步完成"
coscmd list / | wc -l
```

#### 双写过渡期

```go
// 双写策略：同时写本地和对象存储
func (s *StorageService) SaveVideo(videoID string, data []byte) error {
    // 1. 写本地（保底）
    localPath := fmt.Sprintf("data/media/%s.mp4", videoID)
    if err := os.WriteFile(localPath, data, 0644); err != nil {
        return fmt.Errorf("写本地失败: %w", err)
    }
    
    // 2. 写对象存储（主）
    cosKey := fmt.Sprintf("videos/%s.mp4", videoID)
    if err := s.cosClient.Upload(cosKey, data); err != nil {
        log.Warnf("⚠️ COS 上传失败，使用本地存储: %v", err)
        // 不返回错误，降级到本地
    }
    
    return nil
}
```

---

## 总结与建议

### ✅ 当前系统的优势

1. **代码结构优秀**: Uber FX + 分层架构，易于改造
2. **任务状态持久化**: 数据库存储任务状态，不丢失
3. **多用户隔离**: UserID隔离，天然支持多租户
4. **Go语言优势**: 并发性能好，部署简单

### ⚠️ 主要差距

1. **单机架构**: 不支持水平扩展
2. **内存状态**: 任务调度基于内存
3. **全局锁**: 并发性能受限
4. **缺少监控**: 无可观测性

### 🎯 实施建议

**对个人开发者/小团队**:
1. ✅ **优先修复P0问题** (事务、槽位泄漏)
2. ✅ **引入Redis** (分布式锁、缓存)
3. ✅ **使用云服务** (托管MySQL、对象存储)
4. ⚠️ **暂缓完整分布式** (除非用户量>1000)

**对企业/大规模应用**:
1. ✅ **完整分布式改造**
2. ✅ **容器化部署**
3. ✅ **完善监控体系**
4. ✅ **CI/CD自动化**

---

### 📚 参考资源

**技术文档**:
- [Asynq 文档](https://github.com/hibiken/asynq)
- [Redsync 文档](https://github.com/go-redsync/redsync)
- [GORM 读写分离](https://gorm.io/docs/dbresolver/)
- [Prometheus 最佳实践](https://prometheus.io/docs/practices/)

**架构案例**:
- [Bilibili 技术博客](https://www.bilibili.com/read/culture/industry-architecture)
- [Netflix 微服务架构](https://netflixtechblog.com/)

---

**文档维护**: 请随着架构演进及时更新本文档
**反馈渠道**: 提交 Issue 或 PR 到 GitHub 仓库

---

*最后更新: 2025-12-29*
