# 阶段二：质量护栏实施计划

> **版本**: v1.0  
> **日期**: 2026-01-02  
> **目标**: 建立测试体系和 CI/CD 质量门禁，让重构变得安全

---

## 📊 执行摘要

### 当前状态

| 维度 | 当前状态 | 目标 |
|-----|---------|------|
| **Linter 配置** | ❌ 无 `.golangci.yml` | ✅ 渐进式配置 |
| **Pre-commit Hooks** | ❌ 无 | ✅ 格式化 + Lint |
| **单元测试** | ⚠️ 20 个测试文件，覆盖不全 | ✅ 核心模块 70%+ |
| **CI/CD 质量门禁** | ⚠️ 仅 build + release | ✅ lint + test + 覆盖率 |
| **整体覆盖率** | ~15-20% | ✅ 50%+ |

### 已有测试基础

项目已有 **20 个测试文件**，分布如下：

```
tests/integration/           ← 3 个集成测试（加密、审计、备份）
pkg/crypto/                  ← encryption_service_test.go
pkg/audit/                   ← audit_service_test.go
internal/chain_task/         ← 3 个 UploadScheduler 测试（478行）
internal/core/services/      ← ai_service_test.go
internal/auth/               ← JWT认证测试
```

> **重要发现**：`UploadScheduler` 已有详细的**特征测试**，记录了状态机、重试逻辑、延迟策略等核心行为。

---

## 🎯 实施阶段

### 第一阶段：基础设施搭建 (Week 1, 6小时)

#### 任务 1.1: 配置 `.golangci.yml` (2h)

**目标**：渐进式 Linter 配置，不阻断历史代码

**创建文件**：`.golangci.yml`

```yaml
# .golangci.yml - 渐进式配置
run:
  timeout: 5m
  go: "1.24"
  skip-dirs:
    - internal/web  # 前端生成代码

linters:
  # 第一阶段：安全优先
  enable:
    - errcheck      # 检查未处理的错误
    - govet         # 可疑代码检查
    - gosec         # 安全检查
    - staticcheck   # 静态分析

linters-settings:
  errcheck:
    check-blank: true
    exclude-functions:
      - (*os.File).Close
      - (io.Closer).Close
  
  govet:
    enable:
      - shadow
  
  gosec:
    excludes:
      - G104  # 忽略未检查的错误（与 errcheck 重复）
      - G304  # 文件路径变量（项目正常使用）

issues:
  exclude-rules:
    # 测试文件放宽要求
    - path: _test\.go
      linters:
        - errcheck
        - gosec
    
    # 排除已知问题（后续修复）
    - path: pkg/logger/
      linters:
        - govet
      text: "non-constant format string"
```

---

#### 任务 1.2: 配置 Pre-commit Hooks (1h)

**创建文件**：`.pre-commit-config.yaml`

```yaml
repos:
  - repo: https://github.com/dnephin/pre-commit-golang
    rev: v0.5.1
    hooks:
      - id: go-fmt
      - id: go-imports
      - id: go-vet
  
  - repo: local
    hooks:
      - id: golangci-lint
        name: golangci-lint
        entry: golangci-lint run --new-from-rev=HEAD~1
        language: system
        types: [go]
        pass_filenames: false
```

**安装说明**：
```bash
pip install pre-commit
pre-commit install
```

---

#### 任务 1.3: 创建 GitHub Actions 质量门禁 (2h)

**创建文件**：`.github/workflows/quality-gate.yml`

```yaml
name: Quality Gate

on:
  push:
    branches: [main, dev]
  pull_request:
    branches: [main]

jobs:
  # 快速检查（2分钟内反馈）
  quick-check:
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
          version: v1.55
          args: --timeout=5m
      
      - name: Quick Test
        run: go test -short ./...

  # 完整检查（PR 合并前）
  full-check:
    needs: quick-check
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      
      - name: Run Tests with Coverage
        run: |
          go test -race -coverprofile=coverage.out -covermode=atomic ./...
      
      - name: Check Coverage
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "当前覆盖率: ${COVERAGE}%"
          
          # 第一阶段：警告模式（不阻断）
          if (( $(echo "$COVERAGE < 30" | bc -l) )); then
            echo "⚠️ 覆盖率低于 30%，建议增加测试"
          fi
          
          # 第二阶段（Week 4后）：强制模式
          # if (( $(echo "$COVERAGE < 50" | bc -l) )); then
          #   echo "❌ 覆盖率低于 50%，禁止合并"
          #   exit 1
          # fi
      
      - name: Upload Coverage
        uses: codecov/codecov-action@v4
        with:
          file: ./coverage.out
          fail_ci_if_error: false
```

---

#### 任务 1.4: 修复已知 Linter 问题 (1h)

**问题**：`pkg/logger/logger.go` 的 linter 错误预计会被排除规则处理，无需代码修改。

如果仍有问题，可选择：
1. 在 `.golangci.yml` 中排除
2. 或修复代码（如果问题不多）

---

### 第二阶段：单元测试攻坚 (Week 2-3, 18小时)

#### 任务 2.1: TaskChain 状态机测试 (4h)

**文件**：`internal/chain_task/manager/chain_test.go`

**测试范围**：
- 依赖检查逻辑 (`checkDependencies`)
- 状态转换验证
- 跳过任务逻辑

**已有基础**：`upload_scheduler_test.go` 已记录状态机行为，可参考。

---

#### 任务 2.2: ConcurrencyLimiter 测试 (3h)

**文件**：`internal/chain_task/concurrency_limiter_test.go`

**测试范围**：
- 并发获取/释放
- 用户隔离
- 边界情况

---

#### 任务 2.3: Service 层测试 (11h)

| 服务 | 测试文件 | 目标覆盖率 | 工时 |
|-----|---------|-----------|-----|
| SavedVideoService | `saved_video_service_test.go` | 60% | 4h |
| TaskStepService | `task_step_service_test.go` | 70% | 4h |
| BiliAccountService | `bili_account_service_test.go` | 50% | 3h |

**测试方法**：使用内存 SQLite，复用 `tests/integration/test_setup.go` 的模式。

---

### 第三阶段：集成与文档 (Week 4, 6小时)

#### 任务 3.1: 集成测试完善 (3h)

扩展现有的 `tests/integration/` 测试：
- API 端点测试
- 任务链 E2E 测试

#### 任务 3.2: 覆盖率门禁启用 (1h)

修改 `quality-gate.yml`，将覆盖率检查从**警告模式**改为**强制模式**（>50%）。

#### 任务 3.3: 文档更新 (2h)

- 更新 `README.md` 添加测试运行说明
- 创建 `docs/TESTING.md` 测试指南

---

## 📁 新增/修改的文件清单

### 新增文件

| 文件 | 说明 |
|-----|------|
| `.golangci.yml` | Linter 配置 |
| `.pre-commit-config.yaml` | Git Hooks 配置 |
| `.github/workflows/quality-gate.yml` | CI 质量门禁 |
| `internal/chain_task/manager/chain_test.go` | TaskChain 测试 |
| `internal/chain_task/concurrency_limiter_test.go` | 并发控制测试 |
| `internal/core/services/saved_video_service_test.go` | 视频服务测试 |
| `internal/core/services/task_step_service_test.go` | 任务步骤测试 |
| `docs/TESTING.md` | 测试指南文档 |

### 修改文件

| 文件 | 说明 |
|-----|------|
| `README.md` | 添加测试运行说明 |
| `.github/workflows/` | 集成质量门禁 |

---

## ✅ 验证计划

### 自动化测试

#### 1. 运行 Linter

```bash
# 安装 golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 运行 lint
golangci-lint run

# 预期：无 Error 级别问题
```

#### 2. 运行单元测试

```bash
# 运行所有测试
go test ./... -v

# 运行测试 + 覆盖率
go test ./... -cover -coverprofile=coverage.out

# 查看覆盖率报告
go tool cover -func=coverage.out | grep total

# 预期：
# - 所有测试通过
# - Week 2 目标：覆盖率 > 35%
# - Week 4 目标：覆盖率 > 50%
```

#### 3. 运行集成测试

```bash
# 运行集成测试
go test -v ./tests/integration/...

# 预期：11/11 测试通过（现有 + 新增）
```

#### 4. 竞态检测

```bash
# 检测并发问题
go test -race ./...

# 预期：无竞态条件警告
```

### 手动验证

#### 1. 验证 Pre-commit Hooks

```bash
# 安装 hooks
pip install pre-commit
pre-commit install

# 创建一个故意有问题的提交
echo "package test; func x(){_ = 1}" > test_broken.go
git add test_broken.go
git commit -m "test"

# 预期：commit 被 hook 阻止，显示 lint 错误
# 清理
rm test_broken.go
```

#### 2. 验证 GitHub Actions

1. Push 代码到 GitHub
2. 打开 GitHub → Actions 页面
3. 验证 `Quality Gate` 工作流运行成功
4. 查看覆盖率报告

---

## 📆 里程碑

| 日期 | 里程碑 | 验收标准 |
|-----|-------|---------|
| **Week 1 末** | 基础设施完成 | `.golangci.yml` + CI 工作流运行成功 |
| **Week 2 末** | 覆盖率 35%+ | `go test ./... -cover` 达标 |
| **Week 3 末** | 覆盖率 50%+ | 核心模块测试完成 |
| **Week 4 末** | 质量门禁强制 | PR 必须通过 lint + 覆盖率检查 |

---

## ⚠️ 风险与缓解

| 风险 | 可能性 | 影响 | 缓解措施 |
|-----|-------|------|---------|
| Linter 产生大量历史问题 | 中 | 中 | 使用 `exclude-rules` 排除，仅对新代码生效 |
| 测试编写耗时超预期 | 中 | 中 | 优先覆盖纯逻辑模块，复杂模块延后 |
| CI 运行时间过长 | 低 | 低 | 使用 `quick-check` + `full-check` 两阶段策略 |

---

## 🚀 下一步行动

1. **用户审核本计划**
2. **确认后开始第一阶段**：
   - 创建 `.golangci.yml`
   - 创建 `.pre-commit-config.yaml`
   - 创建 `.github/workflows/quality-gate.yml`
3. **验证 CI 流程**
4. **进入第二阶段：单元测试编写**

---

**文档版本**: v1.0  
**更新日期**: 2026-01-02
