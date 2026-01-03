# CI/CD 快速开始

## 🚀 本地测试

在推送代码前，运行本地 CI 测试脚本：

### Windows (PowerShell)
```powershell
.\scripts\ci-test.ps1
```

### Linux/macOS (Bash)
```bash
bash scripts/ci-test.sh
```

## 📋 CI/CD 检查项

推送代码后，GitHub Actions 会自动运行：

1. ✅ 代码格式检查
2. ✅ Lint 检查（golangci-lint）
3. ✅ 单元测试（带竞态检测）
4. ✅ 集成测试（Smoke Tests）
5. ✅ 覆盖率报告
6. ✅ 多平台构建验证

## 📊 查看测试报告

### PR 中自动注释
每次创建/更新 PR，会自动添加测试结果注释，包括：
- 单元测试结果
- 集成测试结果
- 覆盖率统计

### GitHub Actions
查看详细日志：
```
https://github.com/<repo>/actions
```

### Codecov（可选）
查看覆盖率趋势：
```
https://codecov.io/gh/<repo>
```

## 🔧 配置 Codecov（可选）

### 1. 获取 Token
访问 https://codecov.io/ → 注册 → 添加仓库 → 复制 Token

### 2. 配置 GitHub Secret
仓库 Settings → Secrets → New secret
- Name: `CODECOV_TOKEN`
- Value: 上一步复制的 Token

### 3. 验证
推送代码后，Codecov 会自动显示覆盖率报告。

## 📖 详细文档

查看 [CI_CD_GUIDE.md](./CI_CD_GUIDE.md) 获取更多信息。

## ⚡ 快速命令

```bash
# 仅运行单元测试
go test -v -short ./...

# 仅运行集成测试
go test -v ./tests/integration/...

# 运行所有测试
go test -v ./...

# 查看覆盖率
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 🎯 下一步

1. ✅ 本地测试通过
2. ✅ 推送代码
3. ✅ 等待 GitHub Actions 通过
4. ✅ 合并 PR

---

📅 **更新时间**: 2025-01-03
