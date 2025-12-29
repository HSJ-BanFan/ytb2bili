# 前端 API 调用问题诊断与修复指南

## 问题现象

- 浏览器中看不到任务链/视频列表
- 控制台可能出现 401 错误或空数据
- 手动添加 Authorization 头可以获取数据

## 根本原因

前端构建后的静态文件未正确嵌入到 Go 二进制中，或者浏览器缓存了旧版本。

## 诊断步骤

### 1. 检查当前运行的版本

在浏览器控制台执行：
```javascript
// 检查 localStorage 中是否有 token
console.log('JWT Token:', localStorage.getItem('jwt_token'));

// 检查 authFetch 函数是否存在
console.log('authFetch:', typeof window.authFetch);
```

### 2. 检查网络请求

打开浏览器开发者工具 → Network 标签页：
1. 刷新页面
2. 查找 `/api/v1/videos` 请求
3. 检查请求头中是否有 `Authorization` 字段
4. 查看响应状态码和内容

**正常情况应该看到：**
```
Request Headers:
  Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
  Content-Type: application/json

Response:
  Status Code: 200 OK
  Body: {"code":200,"message":"success","data":{"videos":[...],"total":0}}
```

**异常情况：**
```
Request Headers:
  Content-Type: application/json
  (没有 Authorization 头)

Response:
  Status Code: 401 Unauthorized
  Body: {"code":401,"message":"未登录"}
```

## 修复方案

### 方案 1：完整重新构建（推荐）

```bash
# 1. 清理旧的构建文件
cd web
rm -rf .next
rm -rf node_modules/.cache

# 2. 重新构建前端
npm run build

# 3. 回到项目根目录
cd ..

# 4. 清理旧的 Go 构建文件
rm -f ytb2bili.exe

# 5. 重新构建 Go 二进制
go build -o ytb2bili.exe .

# 6. 停止旧的服务器
# (在运行服务器的终端按 Ctrl+C)

# 7. 启动新的服务器
./ytb2bili.exe
```

### 方案 2：仅重新构建前端（快速测试）

```bash
# 1. 重新构建前端
cd web
npm run build

# 2. 复制到嵌入目录
# (Makefile 会自动执行这一步)
cd ..
make build-web

# 3. 重启服务器
```

### 方案 3：清除浏览器缓存

**Chrome/Edge:**
1. 按 `Ctrl + Shift + Delete`
2. 选择"缓存的图片和文件"
3. 点击"清除数据"

**或者使用硬刷新：**
- Windows: `Ctrl + Shift + R` 或 `Ctrl + F5`
- Mac: `Cmd + Shift + R`

### 方案 4：验证构建文件

检查构建后的文件是否包含 authFetch：

```bash
# Windows PowerShell
Select-String -Path "internal\web\bili-up-web\_next\static\chunks\*.js" -Pattern "authFetch"

# Linux/Mac
grep -r "authFetch" internal/web/bili-up-web/_next/static/chunks/
```

如果没有找到，说明构建没有成功，需要重新构建。

## 永久解决方案

### 1. 更新 Makefile

确保 `make build` 命令总是重新构建前端：

```makefile
build: build-web
	@echo "Building Go binary..."
	go build -o ytb2bili.exe .

build-web:
	@echo "Building frontend..."
	cd web && npm run build
	@echo "Embedding frontend..."
	# 复制构建文件（由 Next.js 自动处理）
```

### 2. 版本号控制

在前端代码中添加版本号，强制浏览器刷新：

```javascript
// web/src/lib/authFetch.ts
export const BUILD_TIMESTAMP = __BUILD_TIME__ || Date.now();

// 每次构建时自动更新版本号
```

在 `next.config.js` 中添加：
```javascript
module.exports = {
  env: {
    BUILD_TIME: new Date().getTime(),
  },
};
```

## 验证修复

修复后，在浏览器控制台执行：

```javascript
// 测试 API 调用
fetch('/api/v1/videos?page=1&limit=10', {
  headers: {
    'Authorization': 'Bearer ' + localStorage.getItem('jwt_token'),
    'Content-Type': 'application/json',
  },
})
.then(r => r.json())
.then(data => console.log('API 响应:', data));
```

**成功响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "videos": [],
    "total": 0,
    "page": 1,
    "limit": 10
  }
}
```

## 常见问题

### Q1: 重新构建后还是看不到数据？

**检查：**
1. 确认服务器已重启
2. 确认浏览器已清除缓存
3. 确认 localStorage 中有有效的 JWT token
4. 检查服务器日志是否有错误

### Q2: Token 存在但仍然 401？

**可能原因：**
1. Token 已过期（默认 24 小时）
2. Token 格式错误
3. 后端 JWT 密钥不匹配

**解决方案：**
```javascript
// 在控制台执行
localStorage.clear();
location.href = '/login';
```

### Q3: API 返回 500 错误？

**检查服务器日志：**
```bash
# 查看服务器输出
./ytb2bili.exe
```

常见错误：
- 数据库连接失败
- 用户权限配置错误
- 配置文件缺失

## 预防措施

### 1. 开发模式

开发时使用 Next.js 开发服务器：

```bash
cd web
npm run dev
```

这样修改代码后自动刷新，无需重新构建。

### 2. 版本检查

在 `web/src/app/layout.tsx` 中添加版本号显示：

```tsx
<footer className="text-xs text-gray-400">
  Build: {process.env.BUILD_TIME || 'dev'}
</footer>
```

### 3. 自动化部署

创建 `deploy.sh` 脚本：

```bash
#!/bin/bash
set -e

echo "🔨 构建前端..."
cd web && npm run build && cd ..

echo "📦 构建后端..."
go build -o ytb2bili.exe .

echo "✅ 构建完成！"
echo "🚀 启动服务器..."
./ytb2bili.exe
```

使用方法：
```bash
chmod +x deploy.sh
./deploy.sh
```
