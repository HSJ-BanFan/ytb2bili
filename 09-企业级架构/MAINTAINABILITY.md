# ytb2bili 可维护性与质量栅栏开发指南

> **版本**: v1.0
> **日期**: 2025-12-29
> **目标读者**: 开发人员、测试工程师、技术负责人

---

## 📋 目录

1. [执行摘要](#执行摘要)
2. [当前状态评估](#当前状态评估)
3. [代码规范](#代码规范)
4. [单元测试指南](#单元测试指南)
5. [集成测试指南](#集成测试指南)
6. [测试覆盖率要求](#测试覆盖率要求)
7. [CI/CD 质量门禁](#cicd-质量门禁)
8. [代码审查规范](#代码审查规范)
9. [文档规范](#文档规范)
10. [实施路线图](#实施路线图)

---

## 执行摘要

### 🎯 当前状态

| 维度 | 当前状态 | 企业级要求 | 差距 |
|------|---------|-----------|------|
| **代码规范** | ⚠️ 无强制检查 | ✅ Linter + 自动格式化 | **中等** |
| **单元测试** | ❌ 几乎无测试 | ✅ 核心逻辑 80%+ 覆盖 | **严重** |
| **集成测试** | ❌ 无 | ✅ 关键流程覆盖 | **严重** |
| **CI/CD** | ⚠️ 仅构建 | ✅ 测试+构建+部署 | **中等** |
| **代码审查** | ❌ 无强制 | ✅ PR 必须审查 | **中等** |

### 📊 总体评估

**可维护性评分**: **4/10**
**质量栅栏评分**: **2/10**
**综合评分**: **3/10**

**结论**: 项目架构良好，但缺乏测试和质量保障机制，修改核心逻辑存在高回归风险。

---

## 当前状态评估

### ✅ 优势

```
1. Go 语言强类型系统
   - 编译期类型检查
   - 避免大量运行时错误

2. 清晰的项目结构
   ytb2bili/
   ├── internal/          # 内部业务逻辑
   │   ├── chain_task/    # 任务链
   │   ├── core/          # 核心模型
   │   └── handler/       # HTTP 处理器
   └── pkg/               # 可复用组件

3. 依赖注入 (Uber FX)
   - 模块解耦
   - 便于 Mock 测试

4. 分层架构
   Handler -> Service -> Repository -> Database
```

### ❌ 短板

```
1. 零测试覆盖
   $ go test ./... -cover
   ?       github.com/difyz9/ytb2bili  [no test files]
   ?       github.com/difyz9/ytb2bili/internal/chain_task  [no test files]
   ...

2. 无代码规范检查
   - 未配置 golangci-lint
   - 无 pre-commit hooks

3. 无 CI 测试流程
   - GitHub Actions 仅构建
   - 无测试阻断

4. 核心逻辑裸奔
   - UploadScheduler 复杂重试逻辑无测试
   - TaskChain 状态流转无测试
   - 并发控制逻辑无测试
```

### 🎯 高风险模块识别

| 模块 | 复杂度 | 修改频率 | 测试覆盖 | 风险等级 |
|------|--------|---------|---------|----------|
| `UploadScheduler` | 高 | 高 | 0% | 🔴 **极高** |
| `ChainTaskHandler` | 高 | 中 | 0% | 🔴 **极高** |
| `TaskChain` | 中 | 低 | 0% | 🟡 中 |
| `ConcurrencyLimiter` | 中 | 低 | 0% | 🟡 中 |
| `SavedVideoService` | 中 | 高 | 0% | 🟡 中 |

---

## 代码规范

### 📏 Go 代码规范

#### 强制规范 (Lint Error)

```go
// ❌ 错误示例
func getUser(id int) *User {
    user, _ := db.GetUser(id)  // 忽略错误
    return user
}

// ✅ 正确示例
func getUser(id int) (*User, error) {
    user, err := db.GetUser(id)
    if err != nil {
        return nil, fmt.Errorf("获取用户失败: %w", err)
    }
    return user, nil
}
```

#### 命名规范

```go
// 包名：小写单词，无下划线
package chaintask  // ✅
package chain_task // ❌

// 导出函数：大驼峰
func ProcessVideo()  // ✅
func processVideo()  // ❌ (如需导出)

// 私有函数：小驼峰
func validateInput() // ✅

// 常量：全大写 + 下划线
const MaxRetryCount = 3     // ✅
const MAX_RETRY_COUNT = 3   // ❌ (Go 风格用驼峰)

// 接口命名：动词 + er
type VideoProcessor interface{}  // ✅
type ProcessVideo interface{}    // ❌
```

#### 错误处理规范

```go
// ✅ 使用 errors.Is / errors.As 判断错误
if errors.Is(err, sql.ErrNoRows) {
    return nil, ErrNotFound
}

// ✅ 使用 fmt.Errorf + %w 包装错误
return fmt.Errorf("处理视频失败: %w", err)

// ❌ 避免直接字符串比较
if err.Error() == "record not found" { ... }
```

### 🔧 Linter 配置

创建 `.golangci.yml`:

```yaml
# .golangci.yml
run:
  timeout: 5m
  go: "1.24"

linters:
  enable:
    # 默认启用
    - errcheck      # 检查未处理的错误
    - gosimple      # 简化代码建议
    - govet         # 可疑代码检查
    - ineffassign   # 无效赋值检查
    - staticcheck   # 静态分析
    - unused        # 未使用代码检查
    
    # 额外启用
    - gofmt         # 格式化检查
    - goimports     # import 排序
    - misspell      # 拼写检查
    - bodyclose     # HTTP body 关闭检查
    - contextcheck  # context 传递检查
    - dupl          # 重复代码检查
    - goconst       # 可常量化字符串检查
    - gocritic      # 代码风格检查
    - gosec         # 安全检查

linters-settings:
  errcheck:
    check-blank: true
    check-type-assertions: true
  
  govet:
    enable:
      - shadow  # 变量遮蔽检查
  
  dupl:
    threshold: 100  # 重复代码行数阈值
  
  goconst:
    min-len: 3
    min-occurrences: 3

issues:
  exclude-rules:
    # 测试文件可以忽略部分规则
    - path: _test\.go
      linters:
        - dupl
        - gosec
```

### 🪝 Pre-commit Hooks

创建 `.pre-commit-config.yaml`:

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/golangci/golangci-lint
    rev: v1.55.2
    hooks:
      - id: golangci-lint
        args: [--fix]

  - repo: https://github.com/dnephin/pre-commit-golang
    rev: v0.5.1
    hooks:
      - id: go-fmt
      - id: go-imports
      - id: go-unit-tests
      - id: go-build
```

安装:
```bash
pip install pre-commit
pre-commit install
```

---

## 单元测试指南

### 🎯 测试原则

1. **测试金字塔**: 单元测试 > 集成测试 > E2E 测试
2. **测试边界**: 每个函数/方法独立测试
3. **Mock 外部依赖**: 数据库、HTTP、文件系统等
4. **表驱动测试**: Go 惯用模式

### 📁 测试文件结构

```
internal/
├── chain_task/
│   ├── upload_scheduler.go
│   ├── upload_scheduler_test.go      # 单元测试
│   ├── upload_scheduler_mock.go      # Mock 定义
│   └── testdata/                      # 测试数据
│       └── sample_video.json
├── core/
│   └── services/
│       ├── saved_video_service.go
│       └── saved_video_service_test.go
```

### 🧪 核心模块测试示例

#### 1. UploadScheduler 测试

```go
// upload_scheduler_test.go
package chain_task

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// MockSavedVideoService Mock 视频服务
type MockSavedVideoService struct {
    mock.Mock
}

func (m *MockSavedVideoService) GetVideoByVideoID(videoID string) (*model.SavedVideo, error) {
    args := m.Called(videoID)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*model.SavedVideo), args.Error(1)
}

func (m *MockSavedVideoService) UpdateStatus(id uint, status string) error {
    args := m.Called(id, status)
    return args.Error(0)
}

// 测试用例：正常上传流程
func TestUploadScheduler_UploadNextVideo_Success(t *testing.T) {
    // Arrange
    mockService := new(MockSavedVideoService)
    mockService.On("GetVideoByVideoID", "test123").Return(&model.SavedVideo{
        ID:      1,
        VideoID: "test123",
        Status:  "200",
        Title:   "Test Video",
    }, nil)
    mockService.On("UpdateStatus", uint(1), "201").Return(nil)
    mockService.On("UpdateStatus", uint(1), "300").Return(nil)

    scheduler := &UploadScheduler{
        SavedVideoService: mockService,
        // ... 其他依赖注入
    }

    // Act
    err := scheduler.uploadNextVideo()

    // Assert
    assert.NoError(t, err)
    mockService.AssertExpectations(t)
}

// 测试用例：重试逻辑
func TestUploadScheduler_RetryLogic(t *testing.T) {
    tests := []struct {
        name           string
        retryCount     int
        expectedDelay  time.Duration
        shouldGiveUp   bool
    }{
        {"第1次重试", 1, 10 * time.Minute, false},
        {"第2次重试", 2, 20 * time.Minute, false},
        {"第3次重试", 3, 40 * time.Minute, true},  // 超过最大重试
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            scheduler := &UploadScheduler{}
            
            nextRetry := scheduler.calculateNextRetryTime(tt.retryCount)
            delay := time.Until(nextRetry)

            // 允许 1 秒误差
            assert.InDelta(t, tt.expectedDelay.Seconds(), delay.Seconds(), 1)
        })
    }
}

// 测试用例：并发安全
func TestUploadScheduler_ConcurrentSafety(t *testing.T) {
    scheduler := &UploadScheduler{
        mutex: sync.Mutex{},
    }

    var wg sync.WaitGroup
    errors := make(chan error, 10)

    // 并发调用 10 次
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := scheduler.uploadNextVideo()
            if err != nil {
                errors <- err
            }
        }()
    }

    wg.Wait()
    close(errors)

    // 验证无竞态错误
    for err := range errors {
        t.Errorf("并发错误: %v", err)
    }
}
```

#### 2. TaskChain 状态流转测试

```go
// chain_test.go
package manager

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
)

func TestTaskChain_DependencyCheck(t *testing.T) {
    tests := []struct {
        name           string
        taskName       string
        completedTasks map[string]bool
        failedTasks    map[string]bool
        expectOK       bool
        expectReason   string
    }{
        {
            name:           "无依赖任务-通过",
            taskName:       "获取元数据",
            completedTasks: map[string]bool{},
            failedTasks:    map[string]bool{},
            expectOK:       true,
        },
        {
            name:           "依赖已完成-通过",
            taskName:       "下载封面",
            completedTasks: map[string]bool{"获取元数据": true},
            failedTasks:    map[string]bool{},
            expectOK:       true,
        },
        {
            name:           "依赖失败-阻塞",
            taskName:       "下载封面",
            completedTasks: map[string]bool{},
            failedTasks:    map[string]bool{"获取元数据": true},
            expectOK:       false,
            expectReason:   "依赖任务 [获取元数据] 执行失败",
        },
        {
            name:           "依赖未执行-阻塞",
            taskName:       "翻译字幕",
            completedTasks: map[string]bool{},
            failedTasks:    map[string]bool{},
            expectOK:       false,
            expectReason:   "依赖任务 [下载字幕] 未执行",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            chain := &TaskChain{
                CompletedTasks: tt.completedTasks,
                FailedTasks:    tt.failedTasks,
                SkippedTasks:   make(map[string]bool),
            }

            ok, reason := chain.checkDependencies(tt.taskName)

            assert.Equal(t, tt.expectOK, ok)
            if !tt.expectOK {
                assert.Contains(t, reason, tt.expectReason)
            }
        })
    }
}
```

#### 3. ConcurrencyLimiter 测试

```go
// concurrency_limiter_test.go
package chain_task

import (
    "context"
    "sync"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
)

func TestConcurrencyLimiter_TryAcquire(t *testing.T) {
    limiter := NewConcurrencyLimiter(2) // 最大 2 并发

    // 第1次获取 - 应成功
    acquired1, current1, max1, _ := limiter.TryAcquire(context.Background(), 1)
    assert.True(t, acquired1)
    assert.Equal(t, 1, current1)
    assert.Equal(t, 2, max1)

    // 第2次获取 - 应成功
    acquired2, current2, _, _ := limiter.TryAcquire(context.Background(), 1)
    assert.True(t, acquired2)
    assert.Equal(t, 2, current2)

    // 第3次获取 - 应失败（达到上限）
    acquired3, current3, _, _ := limiter.TryAcquire(context.Background(), 1)
    assert.False(t, acquired3)
    assert.Equal(t, 2, current3) // 仍然是 2

    // 释放一个
    limiter.Release(1)

    // 第4次获取 - 应成功
    acquired4, current4, _, _ := limiter.TryAcquire(context.Background(), 1)
    assert.True(t, acquired4)
    assert.Equal(t, 2, current4)
}

func TestConcurrencyLimiter_UserIsolation(t *testing.T) {
    limiter := NewConcurrencyLimiter(1) // 每用户最大 1 并发

    // 用户1 获取
    acquired1, _, _, _ := limiter.TryAcquire(context.Background(), 1)
    assert.True(t, acquired1)

    // 用户1 再次获取 - 应失败
    acquired2, _, _, _ := limiter.TryAcquire(context.Background(), 1)
    assert.False(t, acquired2)

    // 用户2 获取 - 应成功（用户隔离）
    acquired3, _, _, _ := limiter.TryAcquire(context.Background(), 2)
    assert.True(t, acquired3)
}
```

### 🛠️ 运行测试命令

```bash
# 运行所有测试
go test ./...

# 运行测试 + 覆盖率
go test ./... -cover

# 生成覆盖率报告
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# 运行特定包的测试
go test ./internal/chain_task/... -v

# 运行特定测试用例
go test ./internal/chain_task/... -run TestUploadScheduler_RetryLogic -v

# 竞态检测
go test ./... -race
```

---

## 集成测试指南

### 🎯 集成测试范围

```
┌─────────────────────────────────────────────────────────────┐
│                      集成测试边界                            │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                  应用层                              │    │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐          │    │
│  │  │ Handler  │──│ Service  │──│ Repository│          │    │
│  │  └──────────┘  └──────────┘  └──────────┘          │    │
│  └─────────────────────────────────────────────────────┘    │
│                         │                                    │
│                         ▼                                    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                外部依赖 (真实或容器化)                │    │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐          │    │
│  │  │  MySQL   │  │  Redis   │  │   COS    │          │    │
│  │  │ (testdb) │  │ (testdb) │  │  (mock)  │          │    │
│  │  └──────────┘  └──────────┘  └──────────┘          │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### 🐳 测试环境 (Docker Compose)

```yaml
# docker-compose.test.yml
version: '3.8'

services:
  test-mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: test
      MYSQL_DATABASE: ytb2bili_test
    ports:
      - "33060:3306"
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 5s
      timeout: 5s
      retries: 5

  test-redis:
    image: redis:7
    ports:
      - "63790:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5
```

### 🧪 集成测试示例

```go
// integration_test.go
// +build integration

package integration

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/stretchr/testify/suite"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
)

type IntegrationTestSuite struct {
    suite.Suite
    db     *gorm.DB
    server *httptest.Server
}

func (s *IntegrationTestSuite) SetupSuite() {
    // 连接测试数据库
    dsn := "root:test@tcp(localhost:33060)/ytb2bili_test?parseTime=true"
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    s.Require().NoError(err)
    s.db = db

    // 自动迁移
    err = db.AutoMigrate(&model.SavedVideo{}, &model.TaskStep{}, &model.User{})
    s.Require().NoError(err)

    // 启动测试服务器
    // s.server = httptest.NewServer(router)
}

func (s *IntegrationTestSuite) TearDownSuite() {
    // 清理测试数据
    s.db.Exec("DROP TABLE IF EXISTS cw_saved_videos")
    s.db.Exec("DROP TABLE IF EXISTS cw_task_steps")
    s.db.Exec("DROP TABLE IF EXISTS cw_users")
}

func (s *IntegrationTestSuite) SetupTest() {
    // 每个测试前清空数据
    s.db.Exec("TRUNCATE TABLE cw_saved_videos")
    s.db.Exec("TRUNCATE TABLE cw_task_steps")
}

// 测试完整任务链流程
func (s *IntegrationTestSuite) TestFullTaskChainFlow() {
    // 1. 创建视频任务
    video := &model.SavedVideo{
        VideoID: "test123",
        URL:     "https://youtube.com/watch?v=test123",
        Status:  "001",
        UserID:  1,
    }
    err := s.db.Create(video).Error
    s.Require().NoError(err)

    // 2. 初始化任务步骤
    taskStepService := services.NewTaskStepService(s.db)
    err = taskStepService.InitTaskSteps("test123")
    s.Require().NoError(err)

    // 3. 验证任务步骤创建正确
    steps, err := taskStepService.GetTaskStepsByVideoID("test123")
    s.Require().NoError(err)
    s.Assert().Len(steps, 9)  // 9 个步骤

    // 4. 模拟任务状态更新
    err = taskStepService.UpdateTaskStepStatus("test123", "获取元数据", "completed")
    s.Require().NoError(err)

    // 5. 验证进度
    progress, err := taskStepService.GetTaskProgress("test123")
    s.Require().NoError(err)
    s.Assert().Equal(1, progress["completed_steps"])
}

// 测试 API 端点
func (s *IntegrationTestSuite) TestVideoListAPI() {
    // 准备测试数据
    s.db.Create(&model.SavedVideo{
        VideoID: "video1",
        Title:   "Test Video 1",
        Status:  "200",
        UserID:  1,
    })

    // 发送请求
    req, _ := http.NewRequest("GET", "/api/v1/videos", nil)
    rr := httptest.NewRecorder()
    s.server.Config.Handler.ServeHTTP(rr, req)

    // 验证响应
    s.Assert().Equal(http.StatusOK, rr.Code)
    // 验证 JSON 响应内容...
}

func TestIntegrationSuite(t *testing.T) {
    suite.Run(t, new(IntegrationTestSuite))
}
```

### 🏃 运行集成测试

```bash
# 启动测试环境
docker-compose -f docker-compose.test.yml up -d

# 等待服务就绪
sleep 10

# 运行集成测试
go test ./tests/integration/... -tags=integration -v

# 清理测试环境
docker-compose -f docker-compose.test.yml down -v
```

---

## 测试覆盖率要求

### 📊 覆盖率目标

| 模块 | 当前覆盖率 | 目标覆盖率 | 优先级 |
|------|-----------|-----------|--------|
| `chain_task/upload_scheduler.go` | 0% | **80%** | 🔴 P0 |
| `chain_task/chain_task_handler.go` | 0% | **70%** | 🔴 P0 |
| `chain_task/manager/chain.go` | 0% | **90%** | 🔴 P0 |
| `chain_task/concurrency_limiter.go` | 0% | **90%** | 🟡 P1 |
| `core/services/` | 0% | **70%** | 🟡 P1 |
| `handler/` | 0% | **50%** | 🟢 P2 |
| `pkg/utils/` | 0% | **60%** | 🟢 P2 |

### 📈 覆盖率监控

```bash
# 生成覆盖率报告
go test ./... -coverprofile=coverage.out -covermode=atomic

# 查看总体覆盖率
go tool cover -func=coverage.out | grep total

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html

# 上传到 Codecov (CI 中)
bash <(curl -s https://codecov.io/bash) -f coverage.out
```

### 🚫 覆盖率门禁

```yaml
# .github/workflows/test.yml
- name: Check coverage
  run: |
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    echo "当前覆盖率: ${COVERAGE}%"
    
    # 最低覆盖率要求: 50%
    if (( $(echo "$COVERAGE < 50" | bc -l) )); then
      echo "❌ 覆盖率低于 50%，禁止合并"
      exit 1
    fi
```

---

## CI/CD 质量门禁

### 🔒 质量门禁流程

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   代码提交   │────▶│   Lint 检查  │────▶│   单元测试   │────▶│   覆盖率检查  │
└─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
                           │                   │                   │
                           ▼                   ▼                   ▼
                    ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
                    │  格式检查   │     │  竞态检测   │     │  安全扫描   │
                    └─────────────┘     └─────────────┘     └─────────────┘
                                               │
                                               ▼
                    ┌─────────────────────────────────────────────────────┐
                    │                    全部通过？                        │
                    │           ┌────────┐     ┌────────┐                 │
                    │           │   是   │     │   否   │                 │
                    │           └───┬────┘     └───┬────┘                 │
                    │               ▼              ▼                      │
                    │         允许合并         阻止合并                    │
                    └─────────────────────────────────────────────────────┘
```

### 📝 GitHub Actions 配置

```yaml
# .github/workflows/quality-gate.yml
name: Quality Gate

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  lint:
    name: Lint Check
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: v1.55.2
          args: --timeout=5m

  test:
    name: Unit Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          
      - name: Run tests with coverage
        run: go test ./... -coverprofile=coverage.out -covermode=atomic -race
        
      - name: Check coverage
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "coverage=${COVERAGE}" >> $GITHUB_OUTPUT
          echo "📊 当前测试覆盖率: ${COVERAGE}%"
          
          if (( $(echo "$COVERAGE < 50" | bc -l) )); then
            echo "❌ 覆盖率低于 50%"
            exit 1
          fi
          
      - name: Upload coverage to Codecov
        uses: codecov/codecov-action@v4
        with:
          files: coverage.out
          fail_ci_if_error: true

  security:
    name: Security Scan
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Run Gosec Security Scanner
        uses: securego/gosec@master
        with:
          args: ./...

  build:
    name: Build Check
    runs-on: ubuntu-latest
    needs: [lint, test, security]
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          
      - name: Build
        run: go build -o ytb2bili ./main.go
        
      - name: Verify binary
        run: ./ytb2bili --version || true
```

### 🛡️ Branch Protection Rules

在 GitHub 仓库设置中配置:

```
Settings > Branches > Branch protection rules > Add rule

Branch name pattern: main

☑️ Require a pull request before merging
  ☑️ Require approvals (1)
  ☑️ Dismiss stale pull request approvals when new commits are pushed

☑️ Require status checks to pass before merging
  ☑️ Require branches to be up to date before merging
  Required checks:
    - lint
    - test
    - security
    - build

☑️ Require conversation resolution before merging
☑️ Do not allow bypassing the above settings
```

---

## 代码审查规范

### 📋 审查清单

```markdown
## PR 审查清单

### 功能正确性
- [ ] 代码实现符合需求描述
- [ ] 边界条件处理正确
- [ ] 错误处理完善

### 代码质量
- [ ] 遵循项目代码规范
- [ ] 无重复代码
- [ ] 函数职责单一
- [ ] 变量/函数命名清晰

### 测试
- [ ] 新增/修改代码有对应测试
- [ ] 测试覆盖核心逻辑
- [ ] 测试用例可读性好

### 性能
- [ ] 无明显性能问题
- [ ] 无内存泄漏风险
- [ ] 数据库查询优化

### 安全
- [ ] 无硬编码敏感信息
- [ ] 输入参数已验证
- [ ] 无 SQL 注入风险

### 文档
- [ ] 复杂逻辑有注释
- [ ] API 变更已更新文档
```

### 💬 审查反馈模板

```markdown
### 🔴 Must Fix (必须修改)
- [ ] `upload_scheduler.go:123` - 未处理错误返回值

### 🟡 Should Fix (建议修改)
- [ ] `chain.go:45` - 建议使用 errors.Is 替代字符串比较

### 🟢 Nitpick (小建议)
- [ ] `handler.go:78` - 变量名 `x` 可以更具描述性
```

---

## 文档规范

### 📝 代码注释规范

```go
// ProcessVideo 处理视频任务
//
// 该函数执行完整的视频处理流程，包括:
//   - 下载视频
//   - 生成字幕
//   - 翻译字幕
//   - 上传到 Bilibili
//
// 参数:
//   - ctx: 上下文，用于取消控制
//   - videoID: YouTube 视频 ID
//   - userID: 用户 ID
//
// 返回值:
//   - error: 处理失败时返回错误
//
// 示例:
//
//	err := processor.ProcessVideo(ctx, "dQw4w9WgXcQ", 123)
//	if err != nil {
//	    log.Printf("处理失败: %v", err)
//	}
func (p *Processor) ProcessVideo(ctx context.Context, videoID string, userID uint) error {
    // ...
}
```

### 📚 README 模板

```markdown
# 模块名称

## 概述
简要描述模块功能

## 安装
```bash
go get github.com/difyz9/ytb2bili/pkg/xxx
```

## 快速开始
```go
// 代码示例
```

## API 文档
### 函数1
```go
func Foo(param string) error
```
**参数**: ...
**返回值**: ...

## 配置选项
| 选项 | 类型 | 默认值 | 描述 |
|------|------|--------|------|

## 常见问题
### Q1: xxx?
A: xxx

## 变更日志
- v1.0.0 - 初始版本
```

---

## 实施路线图

### 🎯 第一阶段: 基础设施 (Week 1-2)

- [ ] 配置 golangci-lint
- [ ] 配置 pre-commit hooks
- [ ] 配置 GitHub Actions 基础流水线
- [ ] 设置 Branch Protection Rules

**验收标准**:
- ✅ `golangci-lint run` 通过
- ✅ 提交前自动检查
- ✅ PR 必须通过 CI

### 🎯 第二阶段: 核心模块测试 (Week 3-4)

- [ ] UploadScheduler 单元测试 (80%+)
- [ ] TaskChain 单元测试 (90%+)
- [ ] ConcurrencyLimiter 单元测试 (90%+)
- [ ] ChainTaskHandler 单元测试 (70%+)

**验收标准**:
- ✅ 核心模块覆盖率 > 70%
- ✅ 所有测试通过
- ✅ 无竞态条件

### 🎯 第三阶段: 服务层测试 (Week 5-6)

- [ ] SavedVideoService 测试
- [ ] TaskStepService 测试
- [ ] UserService 测试
- [ ] Handler 层测试

**验收标准**:
- ✅ 总体覆盖率 > 50%
- ✅ 服务层覆盖率 > 60%

### 🎯 第四阶段: 集成测试 (Week 7-8)

- [ ] Docker Compose 测试环境
- [ ] 数据库集成测试
- [ ] API 端点测试
- [ ] 全流程 E2E 测试

**验收标准**:
- ✅ 集成测试环境可一键启动
- ✅ 关键流程有 E2E 覆盖

---

## 附录

### 🔧 推荐工具

| 工具 | 用途 | 链接 |
|------|------|------|
| golangci-lint | 代码检查 | https://golangci-lint.run |
| testify | 测试框架 | https://github.com/stretchr/testify |
| mockery | Mock 生成 | https://github.com/vektra/mockery |
| gotestsum | 测试输出美化 | https://github.com/gotestyourself/gotestsum |
| go-cover-treemap | 覆盖率可视化 | https://github.com/nikolaydubina/go-cover-treemap |
| pre-commit | Git Hooks | https://pre-commit.com |

### 📚 参考资料

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Practical Go Lessons](https://www.practical-go-lessons.com)
- [Testing in Go](https://go.dev/doc/tutorial/add-a-test)

---

**文档维护**: 请随着项目演进及时更新本文档
**反馈渠道**: 提交 Issue 或 PR 到 GitHub 仓库

---

*最后更新: 2025-12-29*
