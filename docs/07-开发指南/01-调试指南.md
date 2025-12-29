# 🔧 JWT 认证调试指南

## ✅ 系统状态确认

### 后端状态
- ✅ 后端已重新编译
- ✅ 后端运行中：`http://localhost:8096`
- ✅ 健康检查正常：`{"status":"ok"}`

### 前端状态
- ✅ 前端已构建
- ✅ 所有配置页面已添加 `getAuthHeaders()`
- ✅ axios 拦截器自动添加 JWT Token

### 已知问题
**后端 API 测试结果**：
```bash
# ✅ 带正确的 Token - 成功
curl "http://localhost:8096/api/v1/config/download" \
  -H "Authorization: Bearer <token>"
# 返回：200 {"code":200,"data":{...}}

# ❌ 不带 Token 或格式错误 - 失败
curl "http://localhost:8096/api/v1/config/download"
# 返回：401 {"code":401,"message":"未登录"}
```

**结论**：后端 JWT 认证**工作正常**！

---

## 🎯 现在请测试

### 方法 1: 使用 HTML 测试页面（推荐）

1. **打开测试页面**
   ```
   在浏览器中打开：
   http://localhost:8096/test_jwt.html
   ```

2. **点击"登录"按钮**
   - 自动使用 `mei/123456` 登录
   - 查看"Token 信息"区域

3. **点击"运行所有测试"**
   - 自动测试 6 个 API 端点
   - 查看每个测试的结果
   - 查看底部日志

4. **预期结果**
   - ✅ 所有测试应该通过（绿色）
   - ✅ 日志显示所有请求成功
   - ❌ 如果有失败（红色），查看日志了解原因

---

### 方法 2: 在浏览器中手动测试

#### 步骤 1: 清除浏览器缓存
```
1. 按 Ctrl + Shift + Delete
2. 选择"缓存的图片和文件"
3. 点击"清除数据"
或者使用无痕窗口：
- Chrome/Edge: Ctrl + Shift + N
- Firefox: Ctrl + Shift + P
```

#### 步骤 2: 登录系统
```
1. 访问：http://localhost:8096/login
2. 输入：
   - 用户名：mei
   - 密码：123456
3. 点击"登录"
```

#### 步骤 3: 检查 Local Storage
```
1. 按 F12 打开开发者工具
2. 切换到"Application"标签页
3. 左侧找到"Local Storage"
4. 点击"http://localhost:8096"
5. 确认有以下内容：
   ✅ jwt_token: (很长的字符串，以 eyJhbGci 开头)
   ✅ jwt_user: {"id":"1","username":"mei","role":"admin",...}
```

#### 步骤 4: 测试设置页面
```
1. 访问：http://localhost:8096/settings
2. 打开开发者工具的"Network"标签页
3. 观察网络请求
```

**应该看到以下请求：**
```
GET /api/v1/config/download
Status: 200 OK
Request Headers:
  Authorization: Bearer eyJhbGci... (很长的 token)
```

#### 步骤 5: 测试自动上传开关
```
1. 在设置页面找到"自动上传"开关
2. 点击开关（切换状态）
3. 观察 Network 标签页

应该看到：
PUT /api/v1/config/download
Status: 200 OK
Request Headers:
  Authorization: Bearer eyJhbGci...
Request Body:
  {
    "auto_upload_enabled": true/false,
    ...
  }
```

---

### 方法 3: 命令行测试

```bash
# 1. 登录并获取 Token
TOKEN=$(curl -s -X POST "http://localhost:8096/api/v1/user/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"mei","password":"123456"}' \
  | grep -o '"access_token":"[^"]*' \
  | cut -d'"' -f4)

echo "Token: ${TOKEN:0:50}..."

# 2. 测试获取配置
echo "测试获取配置..."
curl -s "http://localhost:8096/api/v1/config/download" \
  -H "Authorization: Bearer $TOKEN" \
  | jq '.code, .message'

# 预期输出：200, "success"

# 3. 测试更新配置
echo "测试更新配置..."
curl -s -X PUT "http://localhost:8096/api/v1/config/download" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"auto_upload_enabled":true}' \
  | jq '.code, .message'

# 预期输出：200, "Configuration updated successfully"
```

---

## 🐛 问题排查

### 问题 A: 测试页面打不开

**症状**：访问 `http://localhost:8096/test_jwt.html` 显示 404

**解决方案**：
```bash
# 直接在文件系统打开
file:///E:/githubitem/ytb2bili/test_jwt.html

# 或者检查后端是否运行
curl http://localhost:8096/health
```

---

### 问题 B: 登录成功但仍然提示 401

**可能原因**：

1. **Token 未正确存储**
   ```javascript
   // 在浏览器控制台执行
   console.log(localStorage.getItem('jwt_token'));
   // 应该输出一个很长的字符串
   ```

2. **Token 格式错误**
   - 检查 Authorization 头是否正确
   - 格式应该是：`Bearer eyJhbGci...`

3. **浏览器缓存了旧的 JS 文件**
   ```
   解决方法：清除缓存或使用无痕窗口
   ```

---

### 问题 C: Network 标签页看不到 Authorization 头

**检查步骤**：

1. 点击请求
2. 右侧切换到"Headers"标签页
3. 向下滚动找到"Request Headers"
4. 查找 `Authorization: Bearer ...`

**如果没有看到**：
- 检查 localStorage 中是否有 jwt_token
- 检查浏览器控制台是否有错误
- 尝试刷新页面

---

### 问题 D: axios 拦截器未生效

**验证方法**：

在浏览器控制台执行：
```javascript
// 检查 axios 是否配置了拦截器
import api from '/web/src/lib/api.ts';
console.log(api.interceptors.request.handlers.length);
// 应该 > 0

// 手动测试 API
api.get('/config/download').then(console.log).catch(console.error);
```

---

### 问题 E: 管理员修改配置仍然提示"权限不足"

**检查用户角色**：

```bash
# 方法 1: 使用测试页面
# 查看用户信息中的 role 字段

# 方法 2: 命令行
curl "http://localhost:8096/api/v1/user/me" \
  -H "Authorization: Bearer $TOKEN" \
  | jq '.data.role'

# 应该输出："admin"

# 如果不是 admin，手动设置
# 连接数据库并执行：
UPDATE cw_users SET role = 'admin' WHERE username = 'mei';
```

---

## 📊 测试检查清单

在完成测试后，请确认以下所有项：

### 基础功能
- [ ] 后端正常运行（health check 通过）
- [ ] 能够成功登录
- [ ] Token 正确保存到 localStorage
- [ ] Token 包含有效的用户信息

### 配置页面
- [ ] 设置页面能正常打开
- [ ] 配置数据能正常加载
- [ ] 自动上传开关能正常切换
- [ ] Network 标签显示所有请求都带 Authorization 头
- [ ] 所有请求返回 200（不是 401）

### 管理员权限
- [ ] 管理员用户能修改配置
- [ ] 修改配置后数据确实更新了
- [ ] 浏览器控制台没有错误

### 错误处理
- [ ] 未登录访问返回 401
- [ ] Token 过期能正确处理
- [ ] 错误信息显示友好

---

## 🎯 成功标准

如果以下所有条件都满足，说明 JWT 认证系统工作正常：

1. ✅ 登录后 localStorage 有 jwt_token
2. ✅ 访问配置页面不会看到 401 错误
3. ✅ 切换自动上传开关能成功保存
4. ✅ Network 标签显示所有请求都带 Authorization 头
5. ✅ 所有 API 请求返回 200 OK
6. ✅ 测试页面所有测试都通过（绿色）

---

## 📞 需要帮助？

如果按照以上步骤仍然有问题，请提供：

1. **浏览器控制台的错误信息**（F12 > Console）
2. **Network 标签的失败请求详情**（F12 > Network）
3. **localStorage 的内容**（F12 > Application > Local Storage）
4. **后端日志**（server.log 文件）

---

**最后更新**：2024-12-28 23:17
**测试页面**：http://localhost:8096/test_jwt.html
**后端状态**：✅ 运行中
