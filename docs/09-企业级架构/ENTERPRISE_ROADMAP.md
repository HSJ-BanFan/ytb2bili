# ytb2bili 企业级进化路线图

> **版本**: v1.0
> **日期**: 2025-12-30
> **目标**: 将 ytb2bili 从"优秀的单机应用"进化为"企业级生产系统"

---

## 📊 当前状态总览

```
┌─────────────────────────────────────────────────────────────────┐
│                    ytb2bili 企业级成熟度评估                     │
├──────────────────┬───────────┬───────────┬─────────────────────┤
│       维度       │  当前评分  │  目标评分  │       差距          │
├──────────────────┼───────────┼───────────┼─────────────────────┤
│ 可扩展性         │   3/10    │   8/10    │ 🔴 严重 (分布式)     │
│ 可维护性         │   4/10    │   8/10    │ 🔴 严重 (测试)       │
│ 安全性           │   5/10    │   8/10    │ 🟡 中等 (加密)       │
│ 可观测性         │   4/10    │   8/10    │ 🟡 中等 (监控)       │
│ 运维效率         │   3/10    │   8/10    │ 🔴 严重 (自动化)     │
├──────────────────┼───────────┼───────────┼─────────────────────┤
│ 综合评分         │   3.8/10  │   8/10    │ 需要系统性提升       │
└──────────────────┴───────────┴───────────┴─────────────────────┘
```

---

## 🗺️ 三阶段进化路径

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                                  企业级进化路径                                          │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                          │
│   阶段一                    阶段二                      阶段三                           │
│   稳固基石                  质量护栏                    云原生/分布式                    │
│   (1-2周)                   (3-4周)                     (4-6周)                          │
│                                                                                          │
│   ┌─────────────┐          ┌─────────────┐            ┌─────────────┐                   │
│   │ P0 Bug修复  │   ──▶    │  单元测试   │    ──▶     │ 分布式队列  │                   │
│   │ 事务安全    │          │  集成测试   │            │ Redis集群   │                   │
│   │ 并发安全    │          │  CI/CD门禁  │            │ K8s部署     │                   │
│   │ 数据加密    │          │  代码审查   │            │ 监控告警    │                   │
│   └─────────────┘          └─────────────┘            └─────────────┘                   │
│         │                        │                          │                           │
│         ▼                        ▼                          ▼                           │
│   ✅ 系统稳定               ✅ 可维护性提升              ✅ 可扩展性提升                │
│   ✅ 数据安全               ✅ 回归风险降低              ✅ 高可用保障                  │
│                                                                                          │
└─────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 🎯 阶段一：稳固基石 (Week 1-2)

> **核心目标**: 修复所有 P0 级别的稳定性和安全问题

### 📋 任务清单

#### Week 1: 稳定性修复

| 任务 | 优先级 | 参考文档 | 预估工时 | 状态 |
|------|--------|----------|----------|------|
| 修复 `ResetAllRunningTasks` 事务安全 | P0 | [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md#风险评估与应对措施) | 2h | ⬜ |
| 修复 `UploadScheduler` 全局锁问题 | P0 | [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md#性能瓶颈分析) | 4h | ⬜ |
| 添加数据库复合索引 | P1 | [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md#实施路线图) | 1h | ⬜ |
| 验证数据库连接池配置 | P1 | [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md#方案3-读写分离--连接池) | 1h | ⬜ |

```sql
-- 数据库索引优化
CREATE INDEX idx_user_status ON cw_saved_videos(user_id, status);
CREATE INDEX idx_status_created ON cw_saved_videos(status, created_at);
CREATE UNIQUE INDEX idx_video_step ON cw_task_steps(video_id, step_name);
```

#### Week 2: 安全加固

| 任务 | 优先级 | 参考文档 | 预估工时 | 状态 |
|------|--------|----------|----------|------|
| 实现敏感数据加密存储 | P0 | [SECURITY.md](./SECURITY.md#敏感数据保护) | 4h | ⬜ |
| 迁移敏感配置到环境变量 | P0 | [SECURITY.md](./SECURITY.md#配置安全) | 2h | ⬜ |
| 实现基础审计日志 | P1 | [SECURITY.md](./SECURITY.md#审计日志) | 4h | ⬜ |
| 配置安全响应头 | P2 | [SECURITY.md](./SECURITY.md#api-安全) | 1h | ⬜ |

### ✅ 阶段一验收标准

```markdown
- [ ] `go build ./...` 无错误
- [ ] 所有事务都有 `defer tx.Rollback()` 保护
- [ ] UploadScheduler 上传视频时不阻塞字幕上传
- [ ] Cookies/Token 加密存储
- [ ] 敏感配置从环境变量读取
- [ ] 关键操作有审计日志
- [ ] 运行 24 小时无崩溃
```

### 📦 阶段一交付物

- `internal/core/services/task_step_service.go` - 事务安全修复
- `internal/chain_task/upload_scheduler.go` - 锁粒度优化
- `pkg/crypto/encryption.go` - 加密服务
- `migrations/xxx_add_indexes.sql` - 索引迁移
- `.env.example` - 环境变量模板

---

## 🎯 阶段二：质量护栏 (Week 3-6)

> **核心目标**: 建立测试体系，让重构变得安全

### 📋 任务清单

#### Week 3-4: 单元测试

| 任务 | 覆盖率目标 | 参考文档 | 预估工时 | 状态 |
|------|-----------|----------|----------|------|
| `UploadScheduler` 单元测试 | 80% | [MAINTAINABILITY.md](./MAINTAINABILITY.md#单元测试指南) | 8h | ⬜ |
| `ChainTaskHandler` 单元测试 | 70% | [MAINTAINABILITY.md](./MAINTAINABILITY.md#单元测试指南) | 6h | ⬜ |
| `TaskChain` 状态机测试 | 90% | [MAINTAINABILITY.md](./MAINTAINABILITY.md#2-taskchain-状态流转测试) | 4h | ⬜ |
| `ConcurrencyLimiter` 测试 | 90% | [MAINTAINABILITY.md](./MAINTAINABILITY.md#3-concurrencylimiter-测试) | 3h | ⬜ |
| `SavedVideoService` 测试 | 70% | [MAINTAINABILITY.md](./MAINTAINABILITY.md#服务层测试) | 4h | ⬜ |

#### Week 5: CI/CD 质量门禁

| 任务 | 优先级 | 参考文档 | 预估工时 | 状态 |
|------|--------|----------|----------|------|
| 配置 golangci-lint | P0 | [MAINTAINABILITY.md](./MAINTAINABILITY.md#-linter-配置) | 2h | ⬜ |
| 配置 pre-commit hooks | P1 | [MAINTAINABILITY.md](./MAINTAINABILITY.md#-pre-commit-hooks) | 1h | ⬜ |
| GitHub Actions 测试流水线 | P0 | [MAINTAINABILITY.md](./MAINTAINABILITY.md#-github-actions-配置) | 3h | ⬜ |
| 覆盖率门禁 (>50%) | P1 | [MAINTAINABILITY.md](./MAINTAINABILITY.md#-覆盖率门禁) | 1h | ⬜ |
| 安全扫描 (gosec/govulncheck) | P1 | [SECURITY.md](./SECURITY.md#依赖安全) | 2h | ⬜ |

#### Week 6: 集成测试 & 文档

| 任务 | 优先级 | 参考文档 | 预估工时 | 状态 |
|------|--------|----------|----------|------|
| Docker Compose 测试环境 | P1 | [MAINTAINABILITY.md](./MAINTAINABILITY.md#-测试环境-docker-compose) | 3h | ⬜ |
| API 集成测试 | P1 | [MAINTAINABILITY.md](./MAINTAINABILITY.md#集成测试指南) | 6h | ⬜ |
| Branch Protection Rules | P0 | [MAINTAINABILITY.md](./MAINTAINABILITY.md#️-branch-protection-rules) | 1h | ⬜ |
| 代码审查规范文档 | P2 | [MAINTAINABILITY.md](./MAINTAINABILITY.md#代码审查规范) | 2h | ⬜ |

### ✅ 阶段二验收标准

```markdown
- [ ] `go test ./... -cover` 总覆盖率 > 50%
- [ ] 核心模块 (chain_task, manager) 覆盖率 > 70%
- [ ] `golangci-lint run` 无错误
- [ ] PR 必须通过 CI 才能合并
- [ ] PR 必须有 1 人审查
- [ ] 集成测试环境可一键启动
```

### 📦 阶段二交付物

- `internal/chain_task/*_test.go` - 单元测试
- `tests/integration/` - 集成测试
- `.golangci.yml` - Linter 配置
- `.pre-commit-config.yaml` - Git Hooks
- `.github/workflows/quality-gate.yml` - CI 流水线
- `docker-compose.test.yml` - 测试环境

---

## 🎯 阶段三：云原生/分布式 (Week 7-12)

> **核心目标**: 支持水平扩展，实现高可用

### 📋 任务清单

#### Week 7-8: Redis 基础设施

| 任务 | 优先级 | 参考文档 | 预估工时 | 状态 |
|------|--------|----------|----------|------|
| 集成 Redis 客户端 | P0 | [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md#方案1-分布式任务队列) | 4h | ⬜ |
| 实现分布式锁 (Redsync) | P0 | [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md#方案2-分布式锁) | 6h | ⬜ |
| 迁移 UploadScheduler 到分布式锁 | P0 | [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md#方案2-分布式锁) | 4h | ⬜ |
| 分布式锁单元测试 | P1 | - | 4h | ⬜ |

#### Week 9-10: 分布式任务队列

| 任务 | 优先级 | 参考文档 | 预估工时 | 状态 |
|------|--------|----------|----------|------|
| 集成 Asynq 任务队列 | P0 | [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md#方案1-分布式任务队列) | 8h | ⬜ |
| 迁移 ChainTaskHandler 到 Asynq | P0 | [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md#方案1-分布式任务队列) | 8h | ⬜ |
| 实现任务优先级队列 | P1 | - | 4h | ⬜ |
| Asynq Web UI 集成 | P2 | - | 2h | ⬜ |

#### Week 11: 容器化与编排

| 任务 | 优先级 | 参考文档 | 预估工时 | 状态 |
|------|--------|----------|----------|------|
| 编写 Dockerfile | P0 | [OPERATIONS.md](./OPERATIONS.md) | 3h | ⬜ |
| Docker Compose 生产编排 | P0 | [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md#-docker-部署-推荐) | 4h | ⬜ |
| K8s Deployment/Service | P1 | - | 6h | ⬜ |
| ConfigMap/Secret 管理 | P1 | - | 2h | ⬜ |

#### Week 12: 可观测性

| 任务 | 优先级 | 参考文档 | 预估工时 | 状态 |
|------|--------|----------|----------|------|
| 集成 Prometheus Metrics | P0 | [OBSERVABILITY.md](./OBSERVABILITY.md) | 4h | ⬜ |
| Grafana 仪表板 | P1 | [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md#监控与运维) | 4h | ⬜ |
| 告警规则配置 | P1 | [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md#-告警规则) | 3h | ⬜ |
| 日志聚合 (Loki/ELK) | P2 | [OBSERVABILITY.md](./OBSERVABILITY.md) | 4h | ⬜ |

### ✅ 阶段三验收标准

```markdown
- [ ] 可部署 3+ 实例，无任务重复执行
- [ ] 单实例故障，服务自动恢复
- [ ] 任务队列可视化监控
- [ ] Prometheus 采集指标正常
- [ ] 告警规则触发正常
- [ ] `docker-compose up` 一键启动全套服务
```

### 📦 阶段三交付物

- `pkg/queue/` - Asynq 任务队列封装
- `pkg/lock/` - 分布式锁封装
- `Dockerfile` - 容器镜像
- `docker-compose.yml` - 生产编排
- `k8s/` - Kubernetes 配置
- `monitoring/` - Prometheus + Grafana 配置

---

## 📚 相关文档索引

| 文档 | 主要内容 | 适用阶段 |
|------|----------|----------|
| [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md) | 可扩展性、高可用、分布式改造 | 阶段一、阶段三 |
| [MAINTAINABILITY.md](./MAINTAINABILITY.md) | 代码规范、测试、CI/CD | 阶段二 |
| [SECURITY.md](./SECURITY.md) | 认证、加密、审计、API安全 | 阶段一 |
| [OBSERVABILITY.md](./OBSERVABILITY.md) | 监控、日志、链路追踪 | 阶段三 |
| [OPERATIONS.md](./OPERATIONS.md) | 部署、运维、故障排查 | 阶段三 |

---

## 📅 时间线总览

```
Month 1                    Month 2                    Month 3
├─────────────────────────┼─────────────────────────┼─────────────────────────┤
│ Week 1-2                │ Week 3-6                │ Week 7-12               │
│ ┌─────────────────────┐ │ ┌─────────────────────┐ │ ┌─────────────────────┐ │
│ │ 阶段一: 稳固基石    │ │ │ 阶段二: 质量护栏    │ │ │ 阶段三: 云原生      │ │
│ │                     │ │ │                     │ │ │                     │ │
│ │ • 事务安全修复      │ │ │ • 单元测试          │ │ │ • Redis + 分布式锁  │ │
│ │ • 并发安全修复      │ │ │ • 集成测试          │ │ │ • Asynq 任务队列    │ │
│ │ • 数据加密          │ │ │ • CI/CD 门禁        │ │ │ • Docker/K8s        │ │
│ │ • 环境变量配置      │ │ │ • 代码审查规范      │ │ │ • Prometheus        │ │
│ └─────────────────────┘ │ └─────────────────────┘ │ └─────────────────────┘ │
│                         │                         │                         │
│ 🎯 稳定+安全            │ 🎯 可维护+可测试        │ 🎯 可扩展+高可用        │
└─────────────────────────┴─────────────────────────┴─────────────────────────┘
```

---

## 💰 成本估算

### 开发成本

| 阶段 | 预估工时 | 人天 | 备注 |
|------|----------|------|------|
| 阶段一 | 20h | 2.5天 | 可并行开发 |
| 阶段二 | 40h | 5天 | 测试编写耗时 |
| 阶段三 | 60h | 7.5天 | 涉及新技术栈 |
| **总计** | **120h** | **15天** | 约 3 周全职 |

### 基础设施成本 (月度)

| 组件 | 规格 | 月费用 | 备注 |
|------|------|--------|------|
| 云服务器 x2 | 4核8G | ¥400 | 应用节点 |
| Redis | 1G 标准版 | ¥150 | 阿里云/腾讯云 |
| MySQL | 2核4G 主从 | ¥600 | 可选，已有则免 |
| 对象存储 | 1TB | ¥120 | 已集成 COS |
| **合计** | - | **¥1,270/月** | 年度 ¥15,240 |

---

## 🚀 快速开始

### 立即执行 (阶段一第一步)

```bash
# 1. 修复事务安全
# 打开 internal/core/services/task_step_service.go
# 在 ResetAllRunningTasks 函数中添加:
# defer tx.Rollback()

# 2. 验证修复
go build ./...
go test ./internal/core/services/... -v

# 3. 提交修复
git add .
git commit -m "fix: add defer Rollback to ResetAllRunningTasks"
```

### 下一步行动

1. **阅读**: [SCALABILITY_AND_HA.md](./SCALABILITY_AND_HA.md) 的"实施路线图"章节
2. **执行**: 阶段一任务清单
3. **验证**: 运行验收标准检查
4. **迭代**: 进入阶段二

---

**路线图维护**: 请随着项目进展更新任务状态
**问题反馈**: 提交 Issue 到 GitHub 仓库

---

*最后更新: 2025-12-30*
