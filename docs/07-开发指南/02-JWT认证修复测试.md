# JWT 认证修复测试指南

## 修复日期
2024-12-28

## 修复内容
为所有配置和上传相关的 API 添加了 JWT 认证保护，确保只有登录用户才能访问敏感功能。

---

## 🎯 功能测试

### 1. 登录测试

**步骤：**
1. 访问 `http://localhost:8096/login`
2. 使用管理员账号登录：
   - 用户名：`mei`
   - 密码：`123456`

**预期结果：**
- ✅ 登录成功，跳转到首页
- ✅ 浏览器开发者工具 > Application > Local Storage 中显示：
  - `jwt_token`: 包含 JWT 字符串
  - `jwt_user`: 包含用户信息（包括 `"role":"admin"`）

---

### 2. 设置页面测试（核心功能）

**步骤：**
1. 访问 `http://localhost:8096/settings`
2. 打开浏览器开发者工具（F12）> Network 标签页
3. 尝试以下操作：

#### 测试 A: 查看下载配置
**操作：** 页面加载时自动请求配置

**Network 请求：**
```
GET /api/v1/config/download
Request Headers:
  Authorization: Bearer eyJhbGci...
```

**预期结果：**
- ✅ 状态码：200
- ✅ 返回配置数据（auto_upload_enabled, auto_upload_mode 等）
- ✅ 请求头包含 `Authorization: Bearer <token>`

#### 测试 B: 更新自动上传开关
**操作：** 切换"自动上传"开关

**Network 请求：**
```
PUT /api/v1/config/download
Request Headers:
  Authorization: Bearer eyJhbGci...
  Content-Type: application/json
Body:
{
  "auto_upload_enabled": true,
  "auto_upload_mode": "delayed",
  "video_upload_delay": 10,
  "subtitle_upload_delay": 10
}
```

**预期结果：**
- ✅ 状态码：200
- ✅ 返回更新后的配置
- ✅ 网页上显示"配置更新成功"提示
- ❌ **不应该出现** 401 错误

#### 测试 C: AI 模型配置
**操作：** 点击 "AI 模型设置" 标签页

**Network 请求：**
```
GET /api/v1/config/openai-compatible
GET /api/v1/config/gemini
GET /api/v1/config/ai-services/status
Request Headers:
  Authorization: Bearer eyJhbGci...
```

**预期结果：**
- ✅ 所有请求状态码：200
- ✅ 显示 AI 配置界面
- ❌ **不应该出现** 401 错误

#### 测试 D: 修改 AI 配置（仅管理员）
**操作：**
1. 修改 OpenAI Compatible 配置
2. 点击"保存配置"按钮

**Network 请求：**
```
PUT /api/v1/config/openai-compatible
Request Headers:
  Authorization: Bearer eyJhbGci...
```

**预期结果：**
- ✅ **管理员用户**：状态码 200，保存成功
- ❌ **普通用户**：状态码 403，显示"权限不足"

---

### 3. 未登录访问测试

**步骤：**
1. 打开浏览器无痕窗口（或清除 Local Storage）
2. 访问 `http://localhost:8096/settings`

**Network 请求：**
```
GET /api/v1/config/download
Request Headers:
  (无 Authorization 头)
```

**预期结果：**
- ✅ 状态码：401
- ✅ 返回消息：`{"code":401,"message":"未登录"}`
- ✅ 页面不显示配置数据，或提示需要登录

---

### 4. Token 过期测试

**步骤：**
1. 登录后，手动修改 Local Storage 中的 `jwt_token` 为无效值
2. 刷新页面

**预期结果：**
- ✅ 检测到 Token 无效
- ✅ 自动跳转到登录页面
- ✅ 或显示"登录已过期"提示

---

### 5. 跨标签页同步测试

**步骤：**
1. 在标签页 A 中登录
2. 打开标签页 B，访问 `http://localhost:8096/settings`

**预期结果：**
- ✅ 标签页 B 能够读取配置
- ✅ 所有 API 请求都携带 Token
- ✅ 不需要重新登录

---

## 🐛 常见问题排查

### 问题 1: 仍然看到 401 错误

**可能原因：**
1. 浏览器缓存了旧的 JavaScript 文件
2. Token 未正确存储到 Local Storage
3. 后端未重启，还在使用旧代码

**解决方案：**
```bash
# 1. 清除浏览器缓存
# Ctrl + Shift + Delete (Chrome/Edge)
# 或者使用无痕模式测试

# 2. 检查 Local Storage
# F12 > Application > Local Storage
# 确认有 jwt_token 字段

# 3. 重启后端
cd E:\githubitem\ytb2bili
kill ytb2bili.exe
go build -o ytb2bili.exe .
./ytb2bili.exe
```

---

### 问题 2: 保存配置没有反应

**检查步骤：**
1. 打开开发者工具 > Console 标签页
2. 查看是否有 JavaScript 错误
3. 查看 Network 标签页，确认请求是否发出

**可能原因：**
- Token 过期
- 权限不足（非管理员）
- 后端错误

---

### 问题 3: 管理员用户仍然提示"权限不足"

**检查步骤：**
```bash
# 1. 查询用户角色
curl -X POST http://localhost:8096/api/v1/user/login \
  -H "Content-Type: application/json" \
  -d '{"username":"mei","password":"123456"}'

# 检查返回的用户信息中是否包含 "role":"admin"

# 2. 如果不是管理员，手动设置
# 连接到数据库，执行：
UPDATE cw_users SET role = 'admin' WHERE username = 'mei';
```

---

## ✅ 验证清单

在部署到生产环境前，请确认以下所有项都通过：

- [ ] 登录功能正常
- [ ] Token 正确保存到 Local Storage
- [ ] 配置页面能正常加载数据
- [ ] 自动上传开关能正常切换
- [ ] AI 模型设置页面能正常访问
- [ ] 管理员能修改配置
- [ ] 普通用户查看配置正常，但修改时提示权限不足
- [ ] 未登录用户访问配置页面返回 401
- [ ] 浏览器控制台没有 401 错误
- [ ] 跨标签页共享登录状态

---

## 📊 测试报告模板

```
测试人员：_________
测试日期：_________
浏览器版本：_________

| 测试项 | 结果 | 备注 |
|--------|------|------|
| 登录功能 | ✅/❌ | |
| 配置页面加载 | ✅/❌ | |
| 自动上传开关 | ✅/❌ | |
| AI 配置查看 | ✅/❌ | |
| AI 配置修改（管理员） | ✅/❌ | |
| AI 配置修改（普通用户） | ✅/❌ | |
| 未登录访问 | ✅/❌ | |

问题描述：
_________________________________________________
_________________________________________________

总体评价：✅ 通过 / ❌ 不通过
```

---

## 🔗 相关文件

### 前端修改
- `web/src/app/settings/page.tsx`: 添加 getAuthHeaders()
- `web/src/components/settings/AIModelSettings.tsx`: 添加 getAuthHeaders()
- `web/src/components/video/VideoSubmissionForm.tsx`: 添加 getAuthHeaders()
- `web/src/lib/api.ts`: axios 拦截器自动添加 Token

### 后端修改
- `internal/handler/config_handler.go`: 添加 JWT + Admin 权限检查
- `internal/handler/upload_handler.go`: 添加 JWT 认证
- `internal/auth/admin_middleware.go`: RequireAdmin() 中间件
- `internal/auth/middleware.go`: JWT 认证时加载用户角色

---

## 📞 联系方式

如果测试过程中遇到问题，请：
1. 查看浏览器控制台错误
2. 查看后端日志
3. 记录复现步骤

---

**最后更新：** 2024-12-28
**修复版本：** commit a65dab5, dc41e53
