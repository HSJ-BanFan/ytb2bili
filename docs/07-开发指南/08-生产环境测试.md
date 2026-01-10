# ytb2bili 生产环境测试指南

> **目标**：安全地验证细粒度锁功能在生产环境的表现
>
> **原则**：渐进式部署、可回滚、充分监控

---

## 📋 目录

1. [前置准备](#前置准备)
2. [功能开关配置](#功能开关配置)
3. [渐进式部署流程](#渐进式部署流程)
4. [监控指标](#监控指标)
5. [回滚方案](#回滚方案)
6. [常见问题](#常见问题)

---

## 前置准备

### 1. 备份当前配置

```bash
# 备份配置文件
cp config.toml config.toml.backup.$(date +%Y%m%d_%H%M%S)

# 备份二进制文件
cp ytb2bili ytb2bili.backup.$(date +%Y%m%d_%H%M%S)
```

### 2. 准备监控工具

确保以下监控工具已安装：

- [ ] Prometheus + Grafana（指标采集和可视化）
- [ ] 日志聚合工具（ELK/Loki）
- [ ] 数据库监控工具
- [ ] 系统监控工具（top/htop/iotop）

### 3. 准备回滚脚本

```bash
# 快速回滚脚本
cat > rollback.sh << 'EOF'
#!/bin/bash
set -e
echo "紧急回滚中..."
sed -i 's/enable_fine_grained_lock = true/enable_fine_grained_lock = false/' config.toml
systemctl restart ytb2bili
echo "回滚完成"
EOF

chmod +x rollback.sh
```

---

## 功能开关配置

### 配置文件修改

编辑 `config.toml`：

```toml
[DownloadConfig]
# 启用细粒度锁（新逻辑）
enable_fine_grained_lock = true

# 最大并发上传数
max_concurrent_uploads = 5
```

### 环境变量方式

```bash
# 方式1：环境变量
export YTB2BILI_USE_FINE_GRAINED_LOCK=true

# 方式2：命令行参数
./ytb2bili --enable-fine-grained-lock=true
```

### 验证配置

```bash
# 检查配置是否生效
./ytb2bili --check-config

# 或查看日志
tail -f /var/log/ytb2bili/app.log | grep "细粒度锁"
```

---

## 渐进式部署流程

### 阶段0：准备工作（5分钟）

```bash
# 1. 健康检查
curl http://localhost:8096/health

# 2. 查看当前配置
grep "enable_fine_grained_lock" config.toml

# 3. 记录基线指标
# - 当前上传成功率
# - 平均上传延迟
# - 数据库连接池使用率
```

### 阶段1：5% 流量测试（30分钟）

**目标**：验证基本功能正确性

```bash
# 1. 启用细粒度锁
vim config.toml
# 修改：enable_fine_grained_lock = true

# 2. 重启服务
systemctl restart ytb2bili

# 3. 观察日志
tail -f /var/log/ytb2bili/app.log

# 4. 监控指标（每5分钟检查一次）
watch -n 300 'curl -s http://localhost:8096/metrics | grep upload'
```

**验收标准**：
- [ ] 错误率 < 1%
- [ ] P99 延迟无增加
- [ ] 数据库连接池健康
- [ ] 无异常日志

**如果通过** → 进入阶段2
**如果失败** → 执行回滚（见[回滚方案](#回滚方案)）

### 阶段2：25% 流量测试（1小时）

**目标**：验证中等负载下的稳定性

```bash
# 继续观察（配置已启用）
# 无需修改配置

# 运行监控脚本
./scripts/production_test.sh monitor 1h
```

**验收标准**：
- [ ] 所有阶段1的标准
- [ ] 并发上传数正常（3-8个）
- [ ] 数据库连接池使用率 < 70%
- [ ] Bilibili API 错误率 < 1%

**如果通过** → 进入阶段3
**如果失败** → 执行回滚

### 阶段3：50% 流量测试（2小时）

**目标**：验证高负载下的性能

```bash
# 继续观察
./scripts/production_test.sh monitor 2h
```

**验收标准**：
- [ ] 所有阶段2的标准
- [ ] 系统资源使用正常（CPU < 80%, 内存 < 80%）
- [ ] 无锁冲突（transaction_lock_conflicts < 5%）

**如果通过** → 进入阶段4
**如果失败** → 执行回滚

### 阶段4：100% 全量发布

**目标**：长期稳定性

```bash
# 持续监控
./scripts/production_test.sh monitor
```

**长期监控**：
- 每日检查错误率
- 每周检查性能趋势
- 每月进行容量规划

---

## 监控指标

### 关键指标

| 指标 | 公式 | 告警阈值 | 说明 |
|------|------|---------|------|
| **上传成功率** | 成功数 / 总数 | < 99% | 核心业务指标 |
| **上传延迟 P50** | 50%分位延迟 | > 5min | 用户体验 |
| **上传延迟 P99** | 99%分位延迟 | > 15min | 极端情况 |
| **并发上传数** | 当前上传数 | > 10 | 资源控制 |
| **数据库连接池** | 活跃/最大 | > 80% | 资源瓶颈 |
| **事务锁冲突率** | 冲突/总事务 | > 5% | 性能问题 |
| **Bilibili API 错误率** | 错误/总调用 | > 1% | 外部依赖 |

### 快速检查命令

```bash
# 1. 检查服务健康状态
curl -s http://localhost:8096/health | jq .

# 2. 查看上传成功率（需要 Prometheus）
curl -s 'http://localhost:9090/api/v1/query?query=rate(upload_success_total[5m])'

# 3. 查看数据库连接
mysql -e "SHOW PROCESSLIST" | grep ytb2bili | wc -l

# 4. 查看系统资源
top -bn1 | head -20

# 5. 查看错误日志
tail -1000 /var/log/ytb2bili/app.log | grep ERROR
```

### Grafana 仪表板

推荐创建以下仪表板：

1. **上传性能仪表板**
   - 上传成功率（折线图）
   - 上传延迟（热力图）
   - 并发上传数（折线图）

2. **系统资源仪表板**
   - CPU 使用率
   - 内存使用率
   - 磁盘 I/O
   - 网络带宽

3. **数据库仪表板**
   - 连接池使用率
   - 查询延迟
   - 慢查询 Top10

---

## 回滚方案

### 自动回滚条件

出现以下情况**立即回滚**：

- ❌ 错误率 > 5%
- ❌ P99 延迟增加 > 50%
- ❌ 数据库连接池耗尽
- ❌ 出现 panic 或 deadlock
- ❌ 用户投诉上传失败

### 回滚步骤

#### 方式1：配置文件回滚（推荐）

```bash
# 1. 修改配置
vim config.toml
# 修改：enable_fine_grained_lock = false

# 2. 重启服务
systemctl restart ytb2bili

# 3. 验证回滚成功
tail -f /var/log/ytb2bili/app.log | grep "全局锁"
```

#### 方式2：快速回滚脚本

```bash
# 使用准备的回滚脚本
./rollback.sh

# 验证回滚
systemctl status ytb2bili
```

#### 方式3：Git 回滚

```bash
# 回滚到上一个稳定版本
git revert <commit_hash>
go build ./...
systemctl restart ytb2bili
```

### 回滚后验证

```bash
# 1. 检查服务状态
systemctl status ytb2bili

# 2. 检查错误日志
tail -100 /var/log/ytb2bili/app.log | grep ERROR

# 3. 检查上传功能
# 手动触发一个上传测试

# 4. 通知团队
# "细粒度锁已回滚，正在分析问题"
```

---

## 常见问题

### Q1: 性能提升不明显？

**可能原因**：
- 网络带宽成为瓶颈
- Bilibili API 限流
- 数据库连接池不足

**解决方案**：
```toml
[DownloadConfig]
max_concurrent_uploads = 10  # 增加并发数

[database]
max_open_conns = 50  # 增加数据库连接数
```

### Q2: 数据库连接池耗尽？

**可能原因**：
- 并发数设置过高
- 事务持有时间过长

**解决方案**：
```toml
max_concurrent_uploads = 3  # 降低并发数

[database]
max_open_conns = 25
max_idle_conns = 10
conn_max_lifetime = 60  # 连接最大生命周期（秒）
```

### Q3: Bilibili API 错误率上升？

**可能原因**：
- 并发上传触发 B 站限流

**解决方案**：
```go
// 添加速率限制
import "golang.org/x/time/rate"

limiter := rate.NewLimiter(1, 3)  // 每秒1个，最多3个突发
if !limiter.Allow() {
    return errors.New("rate limit exceeded")
}
```

### Q4: 出现事务死锁？

**症状**：日志中出现 "deadlock" 错误

**紧急回滚**：
```bash
./rollback.sh
```

**分析原因**：
```bash
# 查看死锁日志
tail -100 /var/log/ytb2bili/app.log | grep -i deadlock

# 查看数据库锁等待
mysql -e "SHOW ENGINE INNODB STATUS" | grep -A 20 "LATEST DETECTED DEADLOCK"
```

---

## 总结

### 成功标志

✅ **功能正常**：所有视频和字幕都能正确上传
✅ **性能提升**：多视频场景下，性能提升 2-10 倍
✅ **系统稳定**：无 panic、deadlock、内存泄漏
✅ **资源可控**：数据库连接池、CPU、内存使用正常

### 最佳实践

1. **渐进式部署**：不要一次性全量发布
2. **充分监控**：建立完善的监控体系
3. **快速回滚**：保留回滚能力，直到完全稳定
4. **文档记录**：记录所有配置和变更
5. **团队沟通**：确保所有相关人员了解变更

### 下一步优化

1. **添加速率限制**：防止触发 B 站限流
2. **优化并发控制**：动态调整并发数
3. **实施分布式锁**：支持多实例部署
4. **完善监控告警**：自动化问题发现

---

**最后更新**: 2025-12-30
**维护者**: DevOps Team
**联系方式**: devops@example.com
