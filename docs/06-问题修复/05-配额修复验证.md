# 配额消耗逻辑验证指南

## 修复内容

### 1. 增强了错误信息
- 配额消耗失败时会显示详细的原因
- 区分"每日配额不足"和"加油包无效/过期"

### 2. 添加了调试日志
- 每次消耗配额时打印日志：
  - `✅ 用户 {userID} 消耗每日配额: {当前}/{限制}`
  - `✅ 用户 {userID} 消耗加油包配额`
  - `✅ 加油包扣除成功: 用户 {userID} 剩余 {数量} 个`

### 3. 改进了加油包验证
- 单独检查配额是否为 0
- 单独检查是否过期（显示过期时间）
- 提供更精确的错误提示

## 配额消耗优先级（正确逻辑）

系统按照以下顺序消耗配额：

1. **优先消耗每日配额**
   - 例如：Free 用户每日 5 个配额
   - 今天用了 4 个，提交第 5 个视频时 → 消耗每日配额

2. **每日配额用完后，消耗加油包**
   - 今天用了 5 个（已达上限），提交第 6 个视频时 → 消耗加油包

3. **加油包也用完后**
   - 拒绝请求，提示"配额已用完"

## 测试步骤

### 准备工作

1. 查看当前配额状态：
```bash
curl -H "Authorization: Bearer <your_token>" \
  http://localhost:8096/api/v1/membership/quota
```

响应示例：
```json
{
  "daily_limit": 5,
  "daily_used": 5,
  "daily_remaining": 0,
  "boost_pack_remaining": 10,
  "total_remaining": 10
}
```

### 场景 1：每日配额未用完

**前置条件**：
- `daily_used`: 3
- `daily_limit`: 5
- `boost_pack_remaining`: 10

**操作**：提交视频

**预期结果**：
- 日志显示：`✅ 用户 {userID} 消耗每日配额: 4/5`
- `daily_used` 变为 4
- `boost_pack_remaining` 保持 10

### 场景 2：每日配额已用完，有加油包

**前置条件**：
- `daily_used`: 5
- `daily_limit`: 5
- `boost_pack_remaining`: 10

**操作**：提交视频

**预期结果**：
- 日志显示：`✅ 用户 {userID} 消耗加油包配额`
- `daily_used` 保持 5
- `boost_pack_remaining` 变为 9

### 场景 3：加油包过期

**前置条件**：
- `daily_used`: 5
- `boost_pack_remaining`: 10
- 但加油包已过期

**操作**：提交视频

**预期结果**：
- 返回错误：`消耗加油包失败 (每日配额已用完 5/5): 加油包已过期 (过期时间: 2024-01-01 15:04:05)`

### 场景 4：加油包配额为 0

**前置条件**：
- `daily_used`: 5
- `boost_pack_remaining`: 0

**操作**：提交视频

**预期结果**：
- 返回错误：`消耗加油包失败 (每日配额已用完 5/5): 加油包配额已用完`

## 如何查看日志

在控制台输出中搜索以下关键字：
- `✅ 用户` - 成功消耗配额
- `⚠️ 消耗配额失败` - 配额消耗失败
- `加油包` - 加油包相关操作

## 常见问题排查

### Q1: 为什么我的加油包没有被消耗？

**原因**：每日配额还未用完。系统优先消耗每日配额。

**解决方案**：
- 等到明天每日配额重置后再试
- 或者继续使用直到每日配额用完（例如 Free 用户用完 5 个）

### Q2: 我买了加油包，但还是提示配额不足？

**可能原因**：
1. 每日配额和加油包都已用完
2. 加油包已过期（检查 `boost_pack_expire` 字段）
3. 数据库中加油包配额为 0（检查 `boost_pack_videos` 字段）

**排查SQL**：
```sql
SELECT
  id,
  membership_tier,
  daily_usage_count,
  daily_usage_date,
  boost_pack_videos,
  boost_pack_expire
FROM cw_users
WHERE id = '<your_user_id>';
```

### Q3: 加油包过期时间是多久？

**默认配置**：
- 小加油包：7 天
- 中加油包：15 天
- 大加油包：30 天

**查看过期时间**：
```bash
curl -H "Authorization: Bearer <your_token>" \
  http://localhost:8096/api/v1/membership/boost-pack/status
```

响应示例：
```json
{
  "has_pack": true,
  "videos_remaining": 10,
  "expires_at": "2024-01-15T12:00:00Z",
  "days_remaining": 5
}
```

## 代码改动摘要

### internal/membership/quota.go
- 添加了详细的日志输出
- 改进了错误信息包装
- 明确区分每日配额和加油包消耗

### internal/membership/db_store.go
- 拆分了加油包验证逻辑
- 提供更精确的错误信息（过期时间、配额数量）
- 添加了成功扣除的日志
