# 会员功能限制测试指南

## 🚀 快速开始

### 1. 重新构建并启动

```bash
# 停止当前服务器（Ctrl+C）

# 重新构建
go build -o ytb2bili.exe .

# 启动服务器
./ytb2bili.exe
```

### 2. 准备测试账户

```sql
-- 查看 test_free 用户信息
SELECT id, username, membership_tier
FROM cw_users
WHERE username = 'test_free';

-- 如果需要，重置为免费用户
UPDATE cw_users
SET membership_tier = 'free'
WHERE username = 'test_free';

-- 创建 Pro 测试用户（可选）
UPDATE cw_users
SET membership_tier = 'pro'
WHERE username = 'test_pro';
```

---

## 🧪 测试场景

### 测试 1：Free 用户自动上传限制 ✅

**目的**：验证免费用户不能自动上传

**步骤**：
1. 使用 test_free 账户登录
2. 提交一个 YouTube 视频 URL
3. 等待处理完成（下载、字幕、翻译、元数据）
4. 观察最终状态

**预期结果**：
- ✅ 视频状态变为 `205`（等待手动上传）
- ✅ 不会上传到B站
- ✅ 日志显示：
  ```
  ⏭️ 用户 X 无自动上传权限，跳过视频 xxx (自动上传是 pro 会员功能)
  ```

**命令查看**：
```sql
SELECT video_id, status, title
FROM cw_saved_videos
WHERE user_id = (SELECT id FROM cw_users WHERE username = 'test_free')
ORDER BY created_at DESC
LIMIT 5;
```

---

### 测试 2：Pro 用户自动上传 ✅

**目的**：验证 Pro 用户可以自动上传

**步骤**：
1. 将 test_free 升级为 Pro：
   ```sql
   UPDATE cw_users
   SET membership_tier = 'pro'
   WHERE username = 'test_free';
   ```
2. **重启服务器**（让会员配置生效）
3. 提交新视频
4. 等待处理完成

**预期结果**：
- ✅ 视频自动上传到B站
- ✅ 状态变化：`001` → `002` → `200` → `201` → `300`
- ✅ 日志显示：
  ```
  📤 开始上传视频: xxx
  ✅ 视频上传成功: xxx
  ```

**注意**：
- ⚠️ 会员等级变更后需要**重启服务器**
- ⚠️ 确保已绑定B站账号

---

### 测试 3：并发控制 ✅

**目的**：验证不同等级的并发限制

#### Free 用户（MaxConcurrentTasks = 1）

**步骤**：
1. 确保 test_free 是 Free 用户
2. 快速连续提交 3 个视频（间隔 5 秒）
3. 观察处理顺序

**预期结果**：
- ✅ 视频 1 立即开始处理
- ✅ 视频 2 等待视频 1 完成
- ✅ 视频 3 等待视频 2 完成
- ✅ 日志显示：
  ```
  ✅ 用户 X 获取执行槽位 (1/1)
  ⏳ 用户 X 并发已达上限 (1/1)，任务 xxx 等待下次调度
  ```

**时间线**：
```
00:00 - 提交视频 1 → 立即开始
00:05 - 提交视频 2 → 等待中
00:10 - 提交视频 3 → 等待中
00:30 - 视频 1 完成 → 视频 2 开始
01:00 - 视频 2 完成 → 视频 3 开始
```

#### Pro 用户（MaxConcurrentTasks = 5）

**步骤**：
1. 升级为 Pro 用户并重启
2. 快速连续提交 10 个视频
3. 观察并发情况

**预期结果**：
- ✅ 前 5 个视频同时处理
- ✅ 后 5 个视频等待槽位释放
- ✅ 日志显示：
  ```
  ✅ 用户 X 获取执行槽位 (1/5)
  ✅ 用户 X 获取执行槽位 (2/5)
  ...
  ✅ 用户 X 获取执行槽位 (5/5)
  ⏳ 用户 X 并发已达上限 (5/5)
  ```

---

### 测试 4：账号隔离 ✅

**目的**：验证视频只上传到对应用户的B站账号

**前提**：
- test_free 绑定账号 A
- mei 绑定账号 B

**步骤**：
1. 使用 test_free 账户登录
2. 提交视频（或升级为 Pro 后自动上传）
3. 检查B站账号 A 的上传记录

**预期结果**：
- ✅ 视频上传到账号 A
- ✅ **不会**上传到账号 B
- ✅ 日志显示使用的账号信息

**验证SQL**：
```sql
-- 查看上传记录
SELECT
    sv.video_id,
    sv.bili_bvid,
    sv.status,
    u.username as owner,
    ub.bili_name as uploaded_to
FROM cw_saved_videos sv
JOIN cw_users u ON sv.user_id = u.id
LEFT JOIN cw_user_bili_accounts ub ON ub.user_id = u.id AND ub.is_primary = 1
WHERE sv.user_id = (SELECT id FROM cw_users WHERE username = 'test_free')
ORDER BY sv.created_at DESC
LIMIT 5;
```

---

## 🐛 常见问题排查

### 问题 1：视频仍然自动上传

**可能原因**：
1. 服务器未重启
2. 会员等级缓存未刷新
3. PermissionService 未注入

**排查**：
```bash
# 1. 检查启动日志
grep "PermissionService" logs.txt

# 2. 检查用户等级
SELECT id, username, membership_tier FROM cw_users WHERE username = 'test_free';

# 3. 强制重启
pkill ytb2bili
./ytb2bili.exe
```

### 问题 2：并发控制不生效

**可能原因**：
1. ConcurrencyLimiter 未注入
2. UserID 为 0（旧数据）

**排查**：
```bash
# 检查日志
grep "获取执行槽位" logs.txt

# 检查 user_id
SELECT video_id, user_id FROM cw_saved_videos WHERE video_id = 'xxx';
```

### 问题 3：日志没有权限检查信息

**可能原因**：
1. PermissionService 为 nil
2. 日志级别过低

**排查**：
```bash
# 检查启动日志
grep "✓ Upload scheduler started" logs.txt

# 查看完整日志
tail -f logs.txt | grep -E "PermissionService|自动上传权限|并发"
```

---

## 📊 验证成功标准

### ✅ 核心功能

- [ ] Free 用户视频状态变为 205（等待手动上传）
- [ ] Pro 用户视频自动上传到B站
- [ ] Free 用户任务串行处理
- [ ] Pro 用户任务并发处理（最多 5 个）

### ✅ 账号隔离

- [ ] test_free 的视频不上传到 mei 的账号
- [ ] 每个用户只能看到/操作自己的视频

### ✅ 日志验证

- [ ] 权限检查日志：`⏭️ 用户 X 无自动上传权限`
- [ ] 并发控制日志：`✅ 用户 X 获取执行槽位 (1/1)`
- [ ] 账号隔离日志：使用正确的 B站账号上传

---

## 🎯 快速验证脚本

```bash
#!/bin/bash
# 快速测试脚本

echo "=== 会员功能限制测试 ==="

# 1. 检查用户等级
echo "1. 用户等级："
mysql -u root -p ytb2bili -e "SELECT id, username, membership_tier FROM cw_users WHERE username IN ('test_free', 'mei');"

# 2. 检查最近视频状态
echo "2. 最近视频状态（test_free）："
mysql -u root -p ytb2bili -e "SELECT video_id, status, created_at FROM cw_saved_videos WHERE user_id = (SELECT id FROM cw_users WHERE username = 'test_free') ORDER BY created_at DESC LIMIT 5;"

# 3. 检查并发统计
echo "3. 搜索并发日志："
tail -n 1000 logs.txt | grep -E "获取执行槽位|并发已达上限" | tail -n 20

# 4. 检查权限检查日志
echo "4. 搜索权限日志："
tail -n 1000 logs.txt | grep -E "自动上传权限" | tail -n 20
```

---

## 💡 提示

1. **测试前**：清空旧数据或使用测试账号
2. **测试中**：实时查看日志 `tail -f logs.txt`
3. **测试后**：恢复用户等级 `UPDATE cw_users SET membership_tier = 'free' WHERE username = 'test_free';`

**祝你测试顺利！** 🚀
