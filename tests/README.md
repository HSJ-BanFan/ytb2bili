# 集成测试文档

## 运行测试

```bash
# 运行所有集成测试
go test -v ./tests/integration/...

# 运行特定测试
go test -v ./tests/integration/... -run TestEncryption

# 运行测试并显示覆盖率
go test -v ./tests/integration/... -cover

# 运行测试并生成覆盖率报告
go test -v ./tests/integration/... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## 测试结构

| 文件 | 说明 |
|------|------|
| `test_setup.go` | 测试环境初始化和辅助函数 |
| `encryption_test.go` | 加密服务集成测试 |
| `audit_log_test.go` | 审计日志集成测试 |
| `backup_test.go` | 备份服务集成测试 |

## 测试覆盖

### 加密服务
- [x] 基本加密解密往返测试
- [x] 无效密文处理
- [x] JSON 加密/解密
- [x] 大数据加密

### 审计日志
- [x] 成功操作日志记录
- [x] 失败操作日志记录
- [x] 日志查询功能
- [x] 日志清理功能

### 备份服务
- [x] 加密备份创建
- [x] 多次备份
- [x] 空数据库备份

## 注意事项

1. 测试使用 SQLite 内存数据库，不影响生产数据
2. 每个测试独立运行，互不影响
3. 测试完成后自动清理临时文件
4. 加密服务使用测试密钥，不影响生产环境
