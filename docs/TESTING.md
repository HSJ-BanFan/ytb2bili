# 测试指南 (Testing Guide)

本文档说明如何运行和编写 YTB2BILI 项目的测试。

---

## 📑 目录

- [快速开始](#快速开始)
- [测试概览](#测试概览)
- [运行测试](#运行测试)
- [测试结构](#测试结构)
- [编写测试](#编写测试)
- [CI/CD 流程](#cicd-流程)
- [最佳实践](#最佳实践)
- [常见问题](#常见问题)

---

## 🚀 快速开始

### 最小操作（运行所有测试）

```bash
# 1. 运行所有测试（单元测试 + 集成测试）
make test

# 2. 运行测试并生成覆盖率报告
make test-cover

# 3. 快速检查（lint + 测试）
make quick-check
```

### 推荐工作流

```bash
# 开发前
go fmt ./...
go vet ./...

# 运行测试
make test

# 提交前检查
make quick-check
```

---

## 📊 测试概览

### 测试金字塔

```
        /\
       /  \
      / E2E \           ← 端到端测试（手动）
     /------\
    / 集成   \          ← 集成测试（自动化）
   /----------\
  /  单元测试   \       ← 单元测试（自动化）
 /--------------\
```

### 当前测试覆盖

| 测试类型 | 测试数量 | 覆盖率 | 位置 |
|---------|---------|--------|------|
| **单元测试** | 51+ | 32.9% (auth 包) | `internal/` |
| **集成测试** | 3 (smoke) | 50% (核心流程) | `tests/integration/` |
| **基准测试** | 7+ | - | `*_test.go` |

### 核心模块测试状态

| 模块 | 状态 | 覆盖率 | 重要性 |
|------|------|--------|--------|
| TaskChain (状态机) | ✅ | 70%+ | 🔴 核心 |
| ConcurrencyLimiter | ✅ | 100% | 🔴 核心 |
| AuthMiddleware | ✅ | 100% (核心方法) | 🔴 核心 |
| SavedVideoService | ✅ | 27% | 🟡 重要 |
| TaskStepService | ✅ | 60%+ | 🟡 重要 |

---

## 🧪 运行测试

### 基础命令

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/auth/

# 运行特定测试
go test -run TestAuthMiddleware_JWTAuth_ValidToken ./internal/auth/

# 运行测试并显示详细输出
go test -v ./internal/auth/

# 运行测试并启用竞态检测
go test -race ./...
```

### 使用 Makefile

```bash
# 完整测试流程
make test              # 运行所有测试

# 快速检查（lint + 测试）
make quick-check       # golangci-lint + go test

# 覆盖率报告
make test-cover        # 生成 coverage.html

# 安全检查
make security-check    # gosec + gofmt + go vet

# 性能测试
make bench            # 运行基准测试

# 清理
make clean
```

### 测试选项

```bash
# 跳过长时间运行的测试
go test -short ./...

# 设置超时时间
go test -timeout 30s ./...

# 运行特定标记的测试
go test -run TestIntegration ./...

# 并行运行测试（更快）
go test -parallel 4 ./...

# 生成覆盖率 profile
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 竞态检测

```bash
# 启用竞态检测器
go test -race ./...

# 示例：检测并发问题
go test -race -run TestConcurrencyLimiter_ConcurrentAccess
```

---

## 📁 测试结构

### 目录组织

```
ytb2bili/
├── internal/
│   ├── auth/
│   │   ├── auth_test.go              # 认证中间件单元测试
│   │   ├── jwt.go                    # JWT 服务
│   │   └── middleware.go             # 中间件实现
│   ├── chain_task/
│   │   ├── manager/
│   │   │   ├── state_test.go         # StateManager 测试
│   │   │   └── chain_test.go         # TaskChain 测试
│   │   └── concurrency_limiter_test.go # 并发控制器测试
│   ├── core/
│   │   └── services/
│   │       └── video_service_test.go  # 视频服务测试
│   └── handler/
│       └── video_handler_test.go      # API Handler 测试
├── tests/
│   ├── testhelpers/
│   │   └── helpers.go                # 测试辅助函数
│   └── integration/
│       └── smoke_test.go            # 集成测试（冒烟测试）
└── .github/workflows/
    └── quality-gate.yml              # CI 配置
```

### 测试辅助函数 (`tests/testhelpers/helpers.go`)

```go
// 创建测试上下文
ctx := testhelpers.Setup(t)
defer ctx.Cleanup()

// 创建测试用户
user := ctx.CreateTestUser()

// 创建测试视频
video := ctx.CreateTestVideo(user.ID)

// 创建 B 站账号
biliAccount := ctx.CreateTestBiliAccount(user.ID)

// 创建任务步骤
step := ctx.CreateTestStep(video.ID)
```

---

## ✍️ 编写测试

### 测试命名规范

```go
// 格式：Test<被测对象>_<场景>_<预期结果>

func TestStateManager_Init_UserIsolation(t *testing.T) {
    // ...
}

func TestConcurrencyLimiter_TryAcquire_ReachLimit(t *testing.T) {
    // ...
}

func TestAuthMiddleware_JWTAuth_ValidToken(t *testing.T) {
    // ...
}
```

### 测试结构模板

```go
func TestFeature_Scenario_ExpectedOutcome(t *testing.T) {
    // 1. 准备 (Setup)
    ctx := testhelpers.Setup(t)
    defer ctx.Cleanup()

    // 2. 执行 (Execute)
    result := functionUnderTest()

    // 3. 断言 (Assert)
    assert.Equal(t, expected, result)
    assert.True(t, condition)
}
```

### 使用测试辅助函数

```go
func TestSavedVideoService_GetVideoByID_UserIsolation(t *testing.T) {
    // 使用 testhelpers 简化测试代码
    ctx := testhelpers.Setup(t)
    defer ctx.Cleanup()

    // 创建两个用户
    userA := ctx.CreateTestUser()
    userB := ctx.CreateTestUser()

    // 创建用户 A 的视频
    video := ctx.CreateTestVideo(userA.ID)

    // 测试用户 B 无法访问用户 A 的视频
    _, err := service.GetVideoByIDForUser(video.ID, userB.ID)
    assert.Error(t, err, "用户 B 不应该访问用户 A 的视频")
}
```

### Mock 外部依赖

```go
// 示例：Mock PermissionService
type MockPermissionService struct {
    maxConcurrent map[string]int
    defaultMax     int
    mu             sync.RWMutex
}

func (m *MockPermissionService) GetMaxConcurrentTasks(ctx context.Context, userID string) (int, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    if max, exists := m.maxConcurrent[userID]; exists {
        return max, nil
    }
    return m.defaultMax, nil
}

// 在测试中使用
func TestConcurrencyLimiter_TryAcquire_Success(t *testing.T) {
    mockPerm := &MockPermissionService{
        maxConcurrent: make(map[string]int),
        defaultMax:     3,
    }
    limiter := NewConcurrencyLimiter(mockPerm, logger)
    // ...
}
```

### 表驱动测试

```go
func TestValidateTimestamp(t *testing.T) {
    now := time.Now()

    tests := []struct {
        name      string
        timestamp string
        maxAge    time.Duration
        expected  bool
    }{
        {
            name:      "当前时间",
            timestamp: fmt.Sprintf("%d", now.Unix()),
            maxAge:    5 * time.Minute,
            expected:  true,
        },
        {
            name:      "过期时间戳",
            timestamp: fmt.Sprintf("%d", now.Add(-10*time.Minute).Unix()),
            maxAge:    5 * time.Minute,
            expected:  false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := ValidateTimestamp(tt.timestamp, tt.maxAge)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

---

## 🔄 CI/CD 流程

### GitHub Actions 工作流

**配置文件**：`.github/workflows/quality-gate.yml`

```yaml
# 工作流程
Pull Request → Trigger CI →
  ├─ quick-check (2分钟)
  │   ├─ golangci-lint
  │   ├─ go vet
  │   └─ go test -short
  ├─ full-check (5分钟)
  │   ├─ golangci-lint (严格模式)
  │   ├─ go test -race
  │   └─ go test -cover
  └─ build
      ├─ go build
      └─ 测试编译结果
```

### 质量门禁

#### 快速检查 (quick-check)

```yaml
- 必须通过：✅
  - golangci-lint (基础规则)
  - go vet
  - go test -short (跳过慢速测试)
```

#### 完整检查 (full-check)

```yaml
- 必须通过：✅
  - golangci-lint (严格规则)
  - go test -race (竞态检测)
  - go test -cover (覆盖率检查)
```

#### 覆盖率要求

```yaml
# 当前状态（警告模式）
当前阈值: 20%+
状态: 警告模式（不阻断 PR）

# 未来状态（强制模式）
目标阈值: 50%
状态: 强制模式（阻断 PR）
```

### Branch Protection Rules

**配置位置**：`Repository → Settings → Branches → main`

```yaml
✅ 必需检查项:
  - quick-check
  - full-check
  - build

✅ 其他保护:
  - 禁止直接 push 到 main
  - 必须通过 PR 合并
  - 至少 1 个 reviewer 批准
  - 合并前必须同步最新代码
```

---

## 🎯 最佳实践

### 1. 测试先行 (Test-Driven Development)

```go
// 1. 先写测试
func TestCalculateTotal_NegativeValues(t *testing.T) {
    result := CalculateTotal(-10, -20)
    assert.Equal(t, -30, result)
}

// 2. 再实现功能
func CalculateTotal(a, b int) int {
    return a + b
}
```

### 2. 保持测试独立性

```go
// ❌ 错误：测试之间共享状态
var globalUser *User

func TestA(t *testing.T) {
    globalUser = &User{...}
}

func TestB(t *testing.T) {
    // 依赖 TestA 的执行顺序
}

// ✅ 正确：每个测试独立
func TestA(t *testing.T) {
    user := &User{...}
    // ...
}

func TestB(t *testing.T) {
    user := &User{...}
    // ...
}
```

### 3. 使用表驱动测试

```go
// ✅ 推荐：表驱动测试
tests := []struct {
    input    int
    expected int
}{
    {1, 2},
    {2, 4},
    {3, 6},
}

for _, tt := range tests {
    t.Run(fmt.Sprintf("input=%d", tt.input), func(t *testing.T) {
        result := double(tt.input)
        assert.Equal(t, tt.expected, result)
    })
}
```

### 4. 测试边界条件

```go
func TestStateManager_UserIsolation(t *testing.T) {
    // ✅ 正常情况
    sm := NewStateManager(1, 100, "video_1", dir, time.Now())

    // ✅ 边界条件：空 ID
    sm := NewStateManager(1, 100, "", dir, time.Now())

    // ✅ 边界条件：特殊字符
    sm := NewStateManager(1, 100, "video with spaces", dir, time.Now())

    // ✅ 边界条件：零用户 ID
    sm := NewStateManager(1, 0, "video_1", dir, time.Now())
}
```

### 5. 测试并发安全

```go
func TestConcurrencyLimiter_ConcurrentAccess(t *testing.T) {
    if testing.Short() {
        t.Skip("跳过并发测试")
    }

    // 多个 goroutine 并发访问
    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            limiter.TryAcquire(ctx, userID)
        }()
    }
    wg.Wait()
}
```

### 6. 使用子测试分组

```go
func TestValidateTimestamp(t *testing.T) {
    tests := []struct {
        name string
        ...
    }{
        {"Unix 格式", ...},
        {"RFC3339 格式", ...},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 子测试逻辑
        })
    }
}
```

---

## 🛠️ 常用工具

### 1. 生成覆盖率报告

```bash
# 生成 HTML 报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# 在浏览器中打开
# macOS
open coverage.html

# Linux
xdg-open coverage.html

# Windows
start coverage.html
```

### 2. 查看详细覆盖率

```bash
# 查看每个函数的覆盖率
go tool cover -func=coverage.out

# 示例输出：
# github.com/.../state.go:47:    NewStateManager          100.0%
# github.com/.../state.go:84:    GetCache                 100.0%
# github.com/.../state.go:99:    SetCache                 100.0%
```

### 3. 运行特定测试

```bash
# 运行特定函数的测试
go test -run TestStateManager_GetCache ./internal/chain_task/manager/

# 运行所有集成测试
go test -v ./tests/integration/

# 运行所有认证测试
go test -v ./internal/auth/ -run "TestAuth"
```

### 4. 性能分析

```bash
# 运行基准测试
go test -bench=. -benchmem

# 生成 CPU profile
go test -cpuprofile=cpu.prof -bench=.

# 分析 profile
go tool pprof cpu.prof
```

---

## ❓ 常见问题

### Q1: 测试失败怎么办？

```bash
# 1. 查看详细错误信息
go test -v ./...

# 2. 只运行失败的测试
go test -v -run TestFailedFunction ./...

# 3. 启用竞态检测查看并发问题
go test -race ./...
```

### Q2: 测试运行缓慢？

```bash
# 1. 跳过慢速测试
go test -short ./...

# 2. 只运行单元测试（跳过集成测试）
go test ./internal/...

# 3. 并行运行测试
go test -parallel 4 ./...
```

### Q3: 数据库测试失败？

```bash
# 1. 确保测试数据库已启动
docker ps | grep mysql

# 2. 检查连接配置
# 查看 config.toml 中的数据库配置

# 3. 清理测试数据
make clean-db  # 如果有这个命令
```

### Q4: CI 失败但本地通过？

```bash
# 1. 检查环境差异
go version  # 确保 Go 版本一致

# 2. 清理缓存
go clean -testcache
go mod tidy

# 3. 运行完整的 CI 流程
make quick-check
```

### Q5: 如何调试测试？

```bash
# 1. 使用 delve 调试器
dlv test ./internal/auth/

# 2. 在测试中打印日志
t.Logf("Value: %v", someValue)

# 3. 使用 testify/suite 的辅助函数
suite := assert.New(t)
suite.Require().Equal(expected, actual)
```

### Q6: Mock 外部 API？

```go
// 使用 httptest.Mock HTTP 服务器
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status": "success"}`))
}))
defer server.Close()

// 使用 server.URL 替代真实的 API URL
```

---

## 📚 参考资源

### 内部文档

- [架构文档](./README.md) - 项目架构概览
- [Makefile](../Makefile) - 构建和测试命令
- [CI 配置](.github/workflows/quality-gate.yml) - GitHub Actions 配置

### 外部资源

- [Go 官方测试指南](https://golang.org/pkg/testing/)
- [Testify 文档](https://github.com/stretchr/testify)
- [GORM 测试指南](https://gorm.io/docs/testing.html)
- [Go 测试最佳实践](https://github.com/golang/go/wiki/TableDrivenTests)

---

## 🎯 快速参考

### 常用命令

```bash
# 运行所有测试
make test

# 快速检查
make quick-check

# 覆盖率报告
make test-cover

# 安全检查
make security-check

# 基准测试
make bench
```

### 测试模板

```go
// 单元测试模板
func TestFeature_Scenario_Expected(t *testing.T) {
    ctx := testhelpers.Setup(t)
    defer ctx.Cleanup()

    user := ctx.CreateTestUser()
    result := DoSomething(user.ID)

    assert.Equal(t, expected, result)
}

// 集成测试模板
func TestAPI_Endpoint_EndToEnd(t *testing.T) {
    ctx := testhelpers.Setup(t)
    defer ctx.Cleanup()

    user := ctx.CreateTestUser()
    token := ctx.CreateTestToken(user)

    req := httptest.NewRequest("GET", "/api/videos", nil)
    req.Header.Set("Authorization", "Bearer " + token)
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, 200, w.Code)
}
```

---

## 📝 更新日志

- **2025-01-03**: 初始版本
  - 添加测试概览
  - 添加运行测试指南
  - 添加编写测试最佳实践

---

## 🤝 贡献指南

### 添加新测试时

1. **确定测试类型**
   - 单元测试：测试单个函数或方法
   - 集成测试：测试多个组件交互

2. **选择测试位置**
   - 单元测试：与源代码放在同一目录
   - 集成测试：`tests/integration/`

3. **使用 testhelpers**
   - 复用 `tests/testhelpers/helpers.go` 中的函数
   - 避免重复代码

4. **编写测试**
   - 遵循命名规范
   - 使用表驱动测试
   - 测试边界条件

5. **验证测试**
   ```bash
   make test
   make test-cover
   ```

### 提交前检查

```bash
# 1. 格式化代码
go fmt ./...

# 2. 运行 linter
make lint

# 3. 运行测试
make test

# 4. 运行安全检查
make security-check
```

---

**有疑问吗？** 请查看 [常见问题](#常见问题) 或联系团队。
