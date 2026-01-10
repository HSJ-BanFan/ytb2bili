# CI/CD 配置指南

本文档说明 ytb2bili 项目的 CI/CD 配置和使用方法。

## 📋 目录

- [工作流概览](#工作流概览)
- [配置 Codecov](#配置-codecov)
- [本地测试](#本地测试)
- [故障排查](#故障排查)

---

## 🔄 工作流概览

项目包含 3 个主要 GitHub Actions 工作流：

### 1️⃣ Quality Gate (`.github/workflows/quality-gate.yml`)

**触发条件**：
- Push 到 `main`, `dev`, `develop` 分支
- Pull Request 到这些分支
- 手动触发（`workflow_dispatch`）

**检查项**：

| 阶段 | 内容 | 耗时 | 触发时机 |
|------|------|------|---------|
| Quick Check | golangci-lint + 单元测试（跳过集成） | ~2 分钟 | 每次 push |
| Full Check | 竞态检测 + 覆盖率报告 + 集成测试 | ~5 分钟 | Quick Check 通过 |
| Build | 多平台构建验证（Linux/Windows/macOS） | ~3 分钟 | Quick Check 通过 |

**关键特性**：
- ✅ 使用 Go 1.23
- ✅ 竞态检测（`-race`）
- ✅ 分离单元测试和集成测试
- ✅ 覆盖率阈值检查（当前 20%，目标 50%）
- ✅ 多平台构建验证

---

### 2️⃣ Test Report (`.github/workflows/test-report.yml`)

**触发条件**：
- Pull Request
- Push 到主分支

**功能**：
- 📊 自动生成测试报告
- 💬 在 PR 中添加测试结果注释
- 📈 显示单元测试和集成测试的覆盖率
- 📦 上传覆盖率报告和 HTML 文件

**PR 注释示例**：

```markdown
## 🧪 测试报告

### 总体状态: ✅ 全部测试通过

| 测试类型 | 通过 | 失败 | 覆盖率 |
|---------|------|------|--------|
| 单元测试 | 50 | 0 | 32.5% |
| 集成测试 | 15 | 0 | 85.2% |
| **总计** | **65** | **0** | **35.8%** |
```

---

### 3️⃣ Build Test (`.github/workflows/test.yml`)

**功能**：
- 快速构建验证
- 依赖检查
- 二进制文件验证

---

## 🔧 配置 Codecov

### 步骤 1：注册 Codecov

1. 访问 [codecov.io](https://codecov.io/)
2. 使用 GitHub 账号登录
3. 添加 `ytb2bili` 仓库

### 步骤 2：获取 Token

1. 在 Codecov 中选择 `ytb2bili` 仓库
2. 进入 **Settings** → **Repository Token**
3. 复制 Token

### 步骤 3：配置 GitHub Secrets

1. 进入 GitHub 仓库 **Settings** → **Secrets and variables** → **Actions**
2. 点击 **New repository secret**
3. 添加以下 secret：

| Name | Secret | 说明 |
|------|--------|------|
| `CODECOV_TOKEN` | 上一步复制的 Token | Codecov 上传权限 |

### 步骤 4：验证配置

推送代码后，检查：
1. GitHub Actions 运行成功
2. Codecov 显示覆盖率报告
3. PR 中有覆盖率徽章

**可选**：添加 Coverage Badge 到 README

```markdown
[![codecov](https://codecov.io/gh/difyz9/ytb2bili/branch/main/graph/badge.svg)](https://codecov.io/gh/difyz9/ytb2bili)
```

---

## 🧪 本地测试

在推送代码前，建议本地运行相同的测试命令：

### 快速检查（对应 Quick Check）

```bash
# Lint
golangci-lint run --timeout=5m

# 单元测试（跳过集成）
go test -v -short ./...
```

### 完整检查（对应 Full Check）

```bash
# 单元测试 + 覆盖率
go test -v -race -coverprofile=coverage_unit.out -covermode=atomic ./...

# 集成测试 + 覆盖率
go test -v -race -coverprofile=coverage_integration.out -covermode=atomic ./tests/integration/...

# 查看覆盖率
go tool cover -func=coverage_unit.out | grep total
go tool cover -func=coverage_integration.out | grep total

# 生成 HTML 报告
go tool cover -html=coverage_unit.out -o coverage.html
```

### 运行所有测试

```bash
# 运行所有测试（包括集成测试）
go test -v ./...

# 仅运行单元测试
go test -v -short ./...

# 仅运行集成测试
go test -v ./tests/integration/...
```

---

## 🔍 故障排查

### 问题 1：golangci-lint 失败

**症状**：
```
level=error msg="Running error: context deadline exceeded"
```

**解决方案**：
1. 增加 timeout：
```yaml
args: --timeout=10m
```

2. 或者禁用某些慢速 linter：
```yaml
args: --timeout=5m --disable=gosec
```

---

### 问题 2：测试超时

**症状**：
```
--- FAIL: TestSmokeTest (10.00s)
    testing.go:1349: test timed out after 10m0s
```

**解决方案**：
1. 检查是否有死锁或无限循环
2. 使用 `-short` 标志跳过慢速测试
3. 增加测试超时：
```bash
go test -v -timeout 30m ./...
```

---

### 问题 3：CodeCov 上传失败

**症状**：
```
Error: Codecov: Failed to properly upload: The provided token is invalid
```

**解决方案**：
1. 验证 GitHub Secret `CODECOV_TOKEN` 是否正确
2. 或者不使用 token（公开仓库）：
```yaml
- uses: codecov/codecov-action@v4
  with:
    token: ""  # 空字符串，不使用 token
    fail_ci_if_error: false
```

---

### 问题 4：PR 注释权限错误

**症状**：
```
Error: Resource not accessible by integration
```

**解决方案**：
1. 检查 workflow 权限：
```yaml
permissions:
  contents: read
  pull-requests: write  # 必须有写权限
```

2. 或在仓库设置中启用：
   **Settings** → **Actions** → **General** → **Workflow permissions**
   选择 **Read and write permissions**

---

### 问题 5：Go 版本不匹配

**症状**：
```
Error: go.mod requires go >= 1.23
```

**解决方案**：
```bash
# 更新本地 Go 版本
go install golang.org/dl/go1.23.0@latest
go1.23.0 download

# 或使用 gvm
gvm install go1.23.0
gvm use go1.23.0
```

---

## 📊 覆盖率目标

| 阶段 | 目标覆盖率 | 强制检查 | 时间线 |
|------|-----------|---------|--------|
| Week 1-3 | 20% | ❌ 仅警告 | ✅ 已完成 |
| Week 4+ | 50% | ✅ 阻止合并 | 🚧 进行中 |
| Production | 70%+ | ✅ 阻止合并 | 🎯 未来 |

启用强制检查（在 `quality-gate.yml` 中）：

```yaml
# 取消注释以启用强制模式
if (( $(echo "$COVERAGE < 50" | bc -l) )); then
  echo "❌ 覆盖率低于 50%，禁止合并"
  exit 1
fi
```

---

## 🎯 最佳实践

### 1. 提交前本地测试

```bash
# 1. 格式化代码
go fmt ./...

# 2. 运行 linter
golangci-lint run --timeout=5m

# 3. 运行快速测试
go test -v -short ./...

# 4. 如果通过，再推送
git push
```

### 2. 分支保护规则

在 **Settings** → **Branches** 中配置：

| 规则 | 设置 | 说明 |
|------|------|------|
| 保护分支 | `main`, `dev`, `develop` | 防止直接推送 |
| PR 要求 | ✅ 要求 PR | 强制代码审查 |
| 状态检查 | ✅ Quality Gate | 必须通过 CI |
| 状态检查 | ✅ Test Report | 必须通过测试 |
| 代码审查 | ⚠️ 至少 1 个审查者 | 可选 |

### 3. 自动化标签

创建 `.github/labeler.yml`：

```yaml
test:
  - match: tests/**/*_test.go

documentation:
  - match: docs/**/*.md

bug:
  - match: "fix:*"
```

---

## 📚 参考资源

- [GitHub Actions 文档](https://docs.github.com/en/actions)
- [Codecov 文档](https://docs.codecov.com/)
- [golangci-lint 配置](https://golangci-lint.run/usage/configuration/)
- [Go 测试指南](https://golang.org/doc/tutorial/add-a-test)

---

## 🤝 贡献

提交 PR 前，请确保：

1. ✅ 本地测试通过
2. ✅ 代码格式化（`go fmt`）
3. ✅ Linter 无错误
4. ✅ 新功能有测试覆盖
5. ✅ 文档已更新

---

📅 **最后更新**: 2025-01-03
📖 **维护者**: Claude Code
