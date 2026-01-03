# 修复支付二维码显示问题

## 🐛 问题描述

**症状**：前端点击"立即购买"后，直接打开本地微信应用，没有显示微信支付二维码。

**原因**：
1. 后端返回微信协议链接 `weixin://wxpay/...`
2. 前端使用 `window.open()` 直接打开链接
3. 浏览器调用系统默认应用 → 打开本地微信

## ✅ 修复方案

### 修改的文件

1. **新增**: `web/src/components/membership/PaymentQRModal.tsx`
   - 支付二维码弹窗组件
   - 显示二维码、倒计时、支付状态
   - 自动轮询检查支付状态

2. **修改**: `web/src/components/membership/UpgradeModal.tsx`
   - 移除 `window.open()` 直接打开链接
   - 改为显示二维码弹窗

### 正确的支付流程

```
用户点击"立即购买"
    ↓
调用后端 API 创建订单
    ↓
后端返回支付信息：
  - pay_url (二维码 URL)
  - order_id (订单号)
    ↓
前端显示二维码弹窗
    ↓
用户扫码支付
    ↓
前端轮询检查支付状态 (每 2 秒)
    ↓
支付成功 → 关闭弹窗 → 刷新页面
```

## 🔧 安装依赖

```bash
cd web
npm install qrcode @types/qrcode
```

## 📝 后端要求

后端需要返回以下格式的数据：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "order_id": "1234567890",
    "pay_url": "https://wxpay.wxutil.com/.../qr.png",  // 二维码图片 URL
    "qr_code": "https://wxpay.wxutil.com/.../qr.png",  // 或者这个字段
    "amount": 99.00
  }
}
```

### 支付状态轮询 API

前端会调用以下 API 检查支付状态：

```
GET /api/v1/payment/order-status?order_id=1234567890
Authorization: Bearer <token>

响应：
{
  "code": 0,
  "data": {
    "order_id": "1234567890",
    "status": "paid"  // waiting, paid, expired, failed
  }
}
```

## 🎨 功能特性

### PaymentQRModal 组件功能

1. **二维码显示**
   - 自动生成二维码图片
   - 支持微信/支付宝

2. **倒计时**
   - 5 分钟支付倒计时
   - 过期后显示错误状态

3. **支付状态轮询**
   - 每 2 秒检查一次支付状态
   - 支付成功后自动刷新页面

4. **状态提示**
   - 等待支付（加载动画）
   - 支付成功（绿色勾）
   - 二维码过期（红色叉）

## 📸 效果预览

### 等待支付
```
┌─────────────────────┐
│   扫码支付           │
│   企业会员年付        │
├─────────────────────┤
│                     │
│     ¥299.00         │
│  请使用手机扫码支付   │
│                     │
│   ┌─────────┐       │
│   │  [二维码]  │       │
│   └─────────┘       │
│                     │
│  请在 4:59 内完成   │
│                     │
│  ℹ️ 支付提示         │
│  · 使用扫一扫功能    │
│  · 支付后自动刷新    │
│                     │
│   [ 取消支付 ]       │
└─────────────────────┘
```

### 支付成功
```
┌─────────────────────┐
│   ✓ 支付成功！       │
│   页面即将刷新...    │
└─────────────────────┘
```

### 二维码过期
```
┌─────────────────────┐
│   ✗ 二维码已过期     │
│   请重新下单         │
│                     │
│   [ 关闭 ]          │
└─────────────────────┘
```

## 🚀 使用方法

1. **构建前端**
   ```bash
   cd web
   npm install
   npm run build
   ```

2. **重启应用**
   ```bash
   cd ..
   go run main.go
   ```

3. **测试支付流程**
   - 打开会员升级页面
   - 选择商品和支付方式
   - 点击"立即购买"
   - 应该显示二维码弹窗，而不是打开本地微信

## 🔍 故障排查

### 问题 1：二维码不显示

**检查**：
1. 后端是否返回 `pay_url` 或 `qr_code`
2. URL 是否可访问
3. 浏览器控制台是否有错误

**解决方案**：
```typescript
// 在 handlePurchase 中添加日志
console.log('支付响应:', res.data);
console.log('二维码 URL:', res.data.pay_url || res.data.qr_code);
```

### 问题 2：支付状态检查失败

**检查**：
1. 后端是否实现了 `/api/v1/payment/order-status` 接口
2. JWT Token 是否正确传递

**解决方案**：
```typescript
// 添加更详细的错误日志
const res = await fetch(`/api/v1/payment/order-status?order_id=${orderId}`, {
  headers: {
    Authorization: `Bearer ${localStorage.getItem('token')}`,
  },
});

if (!res.ok) {
  console.error('检查支付状态失败:', res.status, res.statusText);
}
```

### 问题 3：支付成功后页面不刷新

**检查**：
1. 轮询是否正确检测到 `status === 'paid'`
2. `window.location.reload()` 是否被调用

**解决方案**：
```typescript
// 添加调试日志
if (data.data.status === 'paid') {
  console.log('支付成功，准备刷新页面');
  setStatus('paid');
  setTimeout(() => {
    console.log('正在刷新页面');
    window.location.reload();
  }, 1500);
}
```

## 📚 相关文档

- [支付 API 接口文档](../03-会员系统/06-支付API接口文档.md)
- [会员系统设计](../03-会员系统/01-会员设计方案.md)

---

📅 **修复日期**: 2025-01-03
📖 **维护者**: Claude Code
