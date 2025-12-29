# 修复配额用完的错误提示

## 🐛 问题描述

当用户配额用完后，前端显示的错误提示不友好：
```
提交失败：https://youtube.com/shorts/xxx: 网络错误
```

应该显示：
```
提交失败：https://youtube.com/shorts/xxx: 配额已用完，请升级会员或购买加油包
```

---

## 🔍 根本原因

### 后端错误响应（正确）

**`internal/handler/subtitle_handler.go:87-97`**：
```go
// 检查配额
if h.quotaService != nil {
    quotaInfo, err := h.quotaService.GetQuotaInfo(c.Request.Context(), userID)
    if err == nil && !quotaInfo.IsUnlimited && quotaInfo.TotalRemaining <= 0 {
        c.JSON(http.StatusForbidden, gin.H{
            "success": false,
            "message": "配额已用完，请升级会员或购买加油包",
            "code":    "QUOTA_EXCEEDED",
        })
        return
    }
}
```

✅ 后端返回的错误信息是正确的

### 前端错误处理（有问题）

**问题 1：axios 响应拦截器**
```typescript
// ❌ 错误：没有提取错误信息
(error) => {
  console.error("API Error:", error);
  return Promise.reject(error);
}
```

**问题 2：前端页面错误处理**
```typescript
// ❌ 错误：没有正确从响应中提取错误信息
} catch (error: any) {
  const errorMessage = error?.message || error?.data?.message || '网络错误';
  // 问题：error.message 可能不包含后端返回的错误
}
```

---

## ✅ 修复方案

### 修复 1：axios 响应拦截器

**文件**：`web/src/lib/api.ts`

```typescript
// ✅ 修复后：正确提取错误信息
api.interceptors.response.use(
  (response) => {
    return response.data;
  },
  (error) => {
    console.error("API Error:", error);

    // 尝试从错误响应中提取错误信息
    if (error.response) {
      const { data, status } = error.response;

      // 如果响应中有错误信息，使用它
      if (data && (data.message || data.error)) {
        error.message = data.message || data.error;
        error.data = data;
        error.code = status;
      }
    }

    return Promise.reject(error);
  }
);
```

### 修复 2：前端页面错误处理

**文件**：`web/src/app/page.tsx`

```typescript
// ✅ 修复后：正确提取并显示错误信息
} catch (error: any) {
  failCount++;
  // 尝试从错误响应中提取信息
  let errorMessage = '网络错误';

  if (error.response) {
    // 后端返回的错误响应
    const data = error.response.data;
    if (data) {
      errorMessage = data.message || data.error || errorMessage;
    }
  } else if (error.message) {
    // 网络错误或其他错误
    errorMessage = error.message;
  }

  // 特殊处理配额错误
  if (error.response?.status === 403 || errorMessage.includes('配额')) {
    errorMessage = '配额已用完，请升级会员或购买加油包';
  }

  errors.push(`${url.substring(0, 50)}...: ${errorMessage}`);
}
```

---

## 🚀 部署步骤

### 1. 重新构建前端

```bash
cd web
npm run build
```

### 2. 重新构建后端

```bash
cd ..
go build -o ytb2bili.exe .
```

### 3. 重启服务器

```bash
# 停止旧服务
pkill ytb2bili

# 启动新服务
./ytb2bili.exe
```

### 4. 清除浏览器缓存

- Windows: `Ctrl + Shift + R`
- Mac: `Cmd + Shift + R`

---

## 🧪 验证步骤

### 测试场景：配额用完

1. **检查当前配额**
   ```bash
   curl -H "Authorization: Bearer <token>" \
     http://localhost:8096/api/v1/membership/quota
   ```

   响应：
   ```json
   {
     "daily_limit": 5,
     "daily_used": 5,
     "daily_remaining": 0,
     "boost_pack_remaining": 0,
     "total_remaining": 0
   }
   ```

2. **提交视频 URL**
   - 登录 Free 账户
   - 输入视频 URL
   - 点击"提交"

3. **验证错误提示**
   - ✅ 显示："配额已用完，请升级会员或购买加油包"
   - ❌ 不显示："网络错误"

4. **检查响应状态**
   - HTTP 状态码：`403 Forbidden`
   - 错误码：`QUOTA_EXCEEDED`

---

## 📊 错误响应格式

### 配额用完

**HTTP 状态**：`403 Forbidden`

**响应体**：
```json
{
  "success": false,
  "message": "配额已用完，请升级会员或购买加油包",
  "code": "QUOTA_EXCEEDED"
}
```

**前端显示**：
```
提交失败：https://youtube.com/shorts/xxx: 配额已用完，请升级会员或购买加油包
```

### 其他错误

**网络错误**（无响应）：
```
提交失败：https://youtube.com/shorts/xxx: 网络错误
```

**服务器错误**（500）：
```
提交失败：https://youtube.com/shorts/xxx: 服务器内部错误
```

---

## 🎯 修复效果对比

### 修复前

```
❌ 提交失败：https://youtube.com/shorts/xLA70Z4-pKI?si=IdwHZENN...: 网络错误
```

**用户困惑**：不知道是什么问题，以为是网络故障

### 修复后

```
✅ 提交失败：https://youtube.com/shorts/xLA70Z4-pKI?si=IdwHZENN...: 配额已用完，请升级会员或购买加油包
```

**用户明确知道**：配额用完了，需要升级会员或购买加油包

---

## 💡 额外改进建议

### 1. 配额提示优化

在提交视频前显示剩余配额：

```tsx
<div className="mt-2 p-2 bg-blue-50 border border-blue-200 rounded">
  <p className="text-sm text-blue-800">
    今日剩余配额：{quotaInfo.daily_remaining} / {quotaInfo.daily_limit}
  </p>
</div>
```

### 2. 会员升级引导

配额用完时显示升级引导：

```tsx
{error.includes('配额') && (
  <div className="mt-4 p-4 bg-yellow-50 border border-yellow-200 rounded-lg">
    <h3 className="font-medium text-yellow-900 mb-2">
      配额已用完
    </h3>
    <p className="text-sm text-yellow-800 mb-3">
      升级到 Pro 会员即可获得：
    </p>
    <ul className="text-sm text-yellow-800 space-y-1 mb-3">
      <li>• 每日 100 个视频配额</li>
      <li>• 自动上传功能</li>
      <li>• 5 个并发任务</li>
    </ul>
    <Link href="/membership" className="text-blue-600 hover:text-blue-800 font-medium">
      立即升级 →
    </Link>
  </div>
)}
```

### 3. 加油包购买

提供快速购买配额的选项：

```tsx
{error.includes('配额') && (
  <div className="mt-4 flex space-x-2">
    <Link href="/membership" className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700">
      升级会员
    </Link>
    <button className="flex-1 px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700">
      购买加油包
    </button>
  </div>
)}
```

---

## ✅ 修复检查清单

- [x] 修改 axios 响应拦截器（`api.ts`）
- [x] 修改前端错误处理逻辑（`page.tsx`）
- [ ] 重新构建前端
- [ ] 重新构建后端
- [ ] 重启服务器
- [ ] 清除浏览器缓存
- [ ] 测试配额用完时的错误提示
- [ ] 验证错误信息显示正确

---

## 🚀 快速构建

```bash
# 自动构建脚本
cd web && npm run build && cd .. && go build -o ytb2bili.exe .

# 启动服务器
./ytb2bili.exe
```

**修复完成！** 🎉
