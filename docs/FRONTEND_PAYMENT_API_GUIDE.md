# 前端支付接口对接指南

## 文档说明

**目标读者**: 前端开发工程师  
**更新日期**: 2025年12月28日  
**后端项目**: pay-unify-backend  
**适用场景**: 前端调用支付相关接口

---

## 📌 快速开始

### ⚠️ 重要：使用实际配置

根据后端 `config.toml` 配置文件，当前可用的应用配置为：

**方案一：管理后台应用（推荐前端使用）**
- **AppID**: `admin-frontend`
- **AppSecret**: `admin-frontend-secret-key-32chars`
- **速率限制**: 500 请求/分钟
- **IP白名单**: 允许所有（`*`）

**方案二：测试应用**
- **AppID**: `test-app-001`
- **AppSecret**: `test-secret-key-12345678901234567890`
- **速率限制**: 100 请求/分钟
- **IP白名单**: 允许所有（`*`）

**后端服务地址**: `http://localhost:8097`（注意端口是 8097，不是 8089）

### 核心要点

1. **所有支付接口都需要 GoAuth 签名认证**，不能使用普通的 JWT Token
2. **签名算法**: HMAC-SHA256
3. **签名参数**: 只需要 `appId`、`timestamp`、`nonce` 三个参数
4. **请求头**: 需要添加 4 个自定义 Header

### 5 分钟快速接入

```typescript
// 1. 安装依赖（Node.js 内置 crypto，无需安装）

// 2. 复制以下代码到你的项目
import crypto from 'crypto';

// 使用实际配置的 AppID 和 AppSecret
const APP_ID = 'admin-frontend';  // 或 'test-app-001'
const APP_SECRET = 'admin-frontend-secret-key-32chars';  // 对应的密钥
const API_BASE_URL = 'http://localhost:8097';  // 注意端口是 8097

// 生成签名
function generateSignature(appId: string, appSecret: string, timestamp: string, nonce: string): string {
  const signStr = `appId=${appId}&nonce=${nonce}&timestamp=${timestamp}`;
  return crypto.createHmac('sha256', appSecret).update(signStr).digest('hex').toLowerCase();
}

// 生成随机字符串
function generateNonce(length = 32): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let result = '';
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
}

// 3. 调用支付接口
async function createPayment() {
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const nonce = generateNonce();
  const signature = generateSignature(APP_ID, APP_SECRET, timestamp, nonce);

  const response = await fetch(`${API_BASE_URL}/api/v1/payment/pay`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-App-Id': APP_ID,
      'X-Timestamp': timestamp,
      'X-Nonce': nonce,
      'X-Sign': signature,
    },
    body: JSON.stringify({
      subject: '测试商品',
      amount: 0.01,
      payWay: 'alipay',
    }),
  });

  const result = await response.json();
  console.log(result);
}

createPayment();
```

---

## 📖 详细说明

### 1. 签名算法实现

#### 1.1 签名流程图

```
┌─────────────────────────────────────────────────────────┐
│ 第一步: 准备参数                                         │
│ - appId: "admin-frontend"                               │
│ - timestamp: "1735372800" (当前时间戳，秒)               │
│ - nonce: "abc123xyz789" (随机字符串)                    │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ 第二步: 按字母顺序拼接签名字符串                          │
│ signString = "appId=admin-frontend&nonce=abc123xyz789    │
│               &timestamp=1735372800"                     │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ 第三步: 使用 HMAC-SHA256 加密                            │
│ signature = HMAC_SHA256(signString, appSecret)          │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ 第四步: 转换为小写十六进制字符串                          │
│ signature = "3f2a8b9c1d4e5f6g7h8..."                    │
└─────────────────────────────────────────────────────────┘
```

#### 1.2 签名算法关键点

**⚠️ 重要提示**:

- ✅ **只签名这 3 个参数**: `appId`, `nonce`, `timestamp`
- ❌ **不要**包含请求 body 内容
- ❌ **不要**包含 URL 查询参数（如 `?page=1&pageSize=10`）
- ✅ 参数必须按字母顺序排列: `appId` → `nonce` → `timestamp`
- ✅ 签名结果必须是小写十六进制字符串

#### 1.3 浏览器端实现（无需后端）

对于纯前端项目，可以使用 `crypto-js` 库：

```bash
npm install crypto-js
```

```typescript
import CryptoJS from 'crypto-js';

/**
 * 生成 GoAuth 签名（浏览器端）
 */
function generateSignature(appId: string, appSecret: string, timestamp: string, nonce: string): string {
  // 拼接签名字符串（注意顺序）
  const signStr = `appId=${appId}&nonce=${nonce}&timestamp=${timestamp}`;
  
  // HMAC-SHA256 加密
  const hash = CryptoJS.HmacSHA256(signStr, appSecret);
  
  // 转换为小写十六进制
  return hash.toString(CryptoJS.enc.Hex).toLowerCase();
}

/**
 * 生成随机字符串
 */
function generateNonce(length = 32): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let result = '';
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
}

// 使用示例
const appId = 'admin-frontend';  // 管理后台应用ID
const appSecret = 'admin-frontend-secret-key-32chars';  // 管理后台密钥
const timestamp = Math.floor(Date.now() / 1000).toString();
const nonce = generateNonce();
const signature = generateSignature(appId, appSecret, timestamp, nonce);

console.log('签名参数:', {
  appId,
  timestamp,
  nonce,
  signature
});
```

---

### 2. 完整的 API 客户端封装

#### 2.1 TypeScript 版本（推荐）

```typescript
// payment-client.ts
import CryptoJS from 'crypto-js';

interface PaymentConfig {
  baseURL: string;
  appId: string;
  appSecret: string;
}

interface PaymentRequest {
  subject: string;           // 商品标题
  amount: number;            // 金额（元）
  payWay: 'alipay' | 'wechat' | 'paypal';  // 支付方式
  userId?: string;           // 用户ID（可选）
  orderType?: string;        // 订单类型（可选）
  extra?: string;            // 额外信息（可选）
}

interface PaymentResponse {
  code: number;
  message: string;
  data: {
    payUrl: string;          // 支付链接
    orderNo: string;         // 订单号
    amount: number;          // 金额
    payWay: string;          // 支付方式
  };
}

interface OrderQuery {
  page?: number;
  pageSize?: number;
  userId?: string;
  status?: string;
  payWay?: string;
}

export class PaymentClient {
  private config: PaymentConfig;

  constructor(config: PaymentConfig) {
    this.config = config;
  }

  /**
   * 生成签名
   */
  private generateSignature(timestamp: string, nonce: string): string {
    const { appId, appSecret } = this.config;
    const signStr = `appId=${appId}&nonce=${nonce}&timestamp=${timestamp}`;
    const hash = CryptoJS.HmacSHA256(signStr, appSecret);
    return hash.toString(CryptoJS.enc.Hex).toLowerCase();
  }

  /**
   * 生成随机字符串
   */
  private generateNonce(length = 32): string {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    let result = '';
    for (let i = 0; i < length; i++) {
      result += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return result;
  }

  /**
   * 获取认证请求头
   */
  private getAuthHeaders(): Record<string, string> {
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const nonce = this.generateNonce();
    const signature = this.generateSignature(timestamp, nonce);

    return {
      'Content-Type': 'application/json',
      'X-App-Id': this.config.appId,
      'X-Timestamp': timestamp,
      'X-Nonce': nonce,
      'X-Sign': signature,
    };
  }

  /**
   * 发起支付
   */
  async createPayment(request: PaymentRequest): Promise<PaymentResponse> {
    const headers = this.getAuthHeaders();
    const url = `${this.config.baseURL}/api/v1/payment/pay`;

    const response = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(request),
    });

    return response.json();
  }

  /**
   * 查询订单列表
   */
  async getOrders(query: OrderQuery = {}): Promise<any> {
    const headers = this.getAuthHeaders();
    const params = new URLSearchParams();
    
    if (query.page) params.append('page', query.page.toString());
    if (query.pageSize) params.append('pageSize', query.pageSize.toString());
    if (query.userId) params.append('userId', query.userId);
    if (query.status) params.append('status', query.status);
    if (query.payWay) params.append('payWay', query.payWay);

    const url = `${this.config.baseURL}/api/v1/payment/orders?${params.toString()}`;

    const response = await fetch(url, {
      method: 'GET',
      headers,
    });

    return response.json();
  }

  /**
   * 关闭微信订单
   */
  async closeWechatOrder(outTradeNo: string): Promise<any> {
    const headers = this.getAuthHeaders();
    const url = `${this.config.baseURL}/api/v1/payment/wechat/close/${outTradeNo}`;

    const response = await fetch(url, {
      method: 'POST',
      headers,
    });

    return response.json();
  }

  /**
   * 微信退款
   */
  async wechatRefund(outTradeNo: string, refundAmount: number): Promise<any> {
    const headers = this.getAuthHeaders();
    const url = `${this.config.baseURL}/api/v1/payment/wechat/refund`;

    const response = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        outTradeNo,
        refundAmount,
      }),
    });

    return response.json();
  }
}

// 使用示例
const client = new PaymentClient({
  baseURL: 'http://localhost:8097',
  appId: 'admin-frontend',
  appSecret: 'admin-frontend-secret-key-32chars',
});

// 发起支付
const paymentResult = await client.createPayment({
  subject: '会员充值',
  amount: 99.00,
  payWay: 'alipay',
  userId: 'user_123',
});

console.log('支付链接:', paymentResult.data.payUrl);

// 查询订单
const orders = await client.getOrders({
  page: 1,
  pageSize: 20,
  userId: 'user_123',
});

console.log('订单列表:', orders.data.list);
```

#### 2.2 React Hook 版本

```typescript
// usePayment.ts
import { useState } from 'react';
import { PaymentClient } from './payment-client';

const paymentClient = new PaymentClient({
  baseURL: process.env.REACT_APP_API_URL || 'http://localhost:8097',
  appId: process.env.REACT_APP_PAYMENT_APP_ID || 'admin-frontend',
  appSecret: process.env.REACT_APP_PAYMENT_APP_SECRET || 'admin-frontend-secret-key-32chars',
});

export function usePayment() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  /**
   * 创建支付订单
   */
  const createPayment = async (
    subject: string,
    amount: number,
    payWay: 'alipay' | 'wechat' | 'paypal',
    userId?: string
  ) => {
    setLoading(true);
    setError(null);

    try {
      const result = await paymentClient.createPayment({
        subject,
        amount,
        payWay,
        userId,
      });

      if (result.code !== 200) {
        throw new Error(result.message || '支付创建失败');
      }

      // 跳转到支付页面
      window.location.href = result.data.payUrl;

      return result.data;
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : '支付失败';
      setError(errorMessage);
      throw err;
    } finally {
      setLoading(false);
    }
  };

  /**
   * 查询订单列表
   */
  const getOrders = async (page = 1, pageSize = 20, userId?: string) => {
    setLoading(true);
    setError(null);

    try {
      const result = await paymentClient.getOrders({
        page,
        pageSize,
        userId,
      });

      if (result.code !== 200) {
        throw new Error(result.message || '查询订单失败');
      }

      return result.data;
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : '查询失败';
      setError(errorMessage);
      throw err;
    } finally {
      setLoading(false);
    }
  };

  return {
    loading,
    error,
    createPayment,
    getOrders,
  };
}

// 组件中使用
function PaymentButton() {
  const { createPayment, loading, error } = usePayment();

  const handlePay = async () => {
    try {
      await createPayment('VIP会员月卡', 29.9, 'alipay', 'user_123');
    } catch (err) {
      console.error('支付失败:', err);
    }
  };

  return (
    <div>
      <button onClick={handlePay} disabled={loading}>
        {loading ? '创建支付中...' : '立即支付'}
      </button>
      {error && <p style={{ color: 'red' }}>{error}</p>}
    </div>
  );
}
```

#### 2.3 Vue3 组合式 API 版本

```typescript
// usePayment.ts
import { ref } from 'vue';
import { PaymentClient } from './payment-client';

const paymentClient = new PaymentClient({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8097',
  appId: import.meta.env.VITE_PAYMENT_APP_ID || 'admin-frontend',
  appSecret: import.meta.env.VITE_PAYMENT_APP_SECRET || 'admin-frontend-secret-key-32chars',
});

export function usePayment() {
  const loading = ref(false);
  const error = ref<string | null>(null);

  /**
   * 创建支付订单
   */
  const createPayment = async (
    subject: string,
    amount: number,
    payWay: 'alipay' | 'wechat' | 'paypal',
    userId?: string
  ) => {
    loading.value = true;
    error.value = null;

    try {
      const result = await paymentClient.createPayment({
        subject,
        amount,
        payWay,
        userId,
      });

      if (result.code !== 200) {
        throw new Error(result.message || '支付创建失败');
      }

      // 跳转到支付页面
      window.location.href = result.data.payUrl;

      return result.data;
    } catch (err) {
      error.value = err instanceof Error ? err.message : '支付失败';
      throw err;
    } finally {
      loading.value = false;
    }
  };

  /**
   * 查询订单列表
   */
  const getOrders = async (page = 1, pageSize = 20, userId?: string) => {
    loading.value = true;
    error.value = null;

    try {
      const result = await paymentClient.getOrders({
        page,
        pageSize,
        userId,
      });

      if (result.code !== 200) {
        throw new Error(result.message || '查询订单失败');
      }

      return result.data;
    } catch (err) {
      error.value = err instanceof Error ? err.message : '查询失败';
      throw err;
    } finally {
      loading.value = false;
    }
  };

  return {
    loading,
    error,
    createPayment,
    getOrders,
  };
}

// 组件中使用
// <script setup>
// import { usePayment } from '@/composables/usePayment';
// 
// const { createPayment, loading, error } = usePayment();
// 
// const handlePay = async () => {
//   await createPayment('VIP会员月卡', 29.9, 'alipay', 'user_123');
// };
// </script>
```

---

### 3. 接口列表

#### 3.1 发起支付

**接口地址**: `POST /api/v1/payment/pay`

**请求头**:
```
Content-Type: application/json
X-App-Id: admin-frontend
X-Timestamp: 1735372800
X-Nonce: abc123xyz789
X-Sign: 3f2a8b9c1d4e5f6g7h8...
```

**请求参数**:
```json
{
  "subject": "VIP会员月卡",          // 必填：商品名称
  "amount": 29.9,                    // 必填：金额（元）
  "payWay": "alipay",                // 必填：支付方式 alipay/wechat/paypal
  "userId": "user_123",              // 可选：用户ID
  "orderType": "vip",                // 可选：订单类型
  "extra": "{\"level\":\"gold\"}"    // 可选：额外信息（JSON字符串）
}
```

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "payUrl": "https://openapi.alipay.com/gateway.do?...",
    "orderNo": "202512280001234567",
    "amount": 29.9,
    "payWay": "alipay"
  }
}
```

**前端处理**:
```typescript
const result = await client.createPayment({
  subject: 'VIP会员月卡',
  amount: 29.9,
  payWay: 'alipay',
  userId: 'user_123',
});

// 跳转到支付页面
window.location.href = result.data.payUrl;
```

#### 3.2 查询订单列表

**接口地址**: `GET /api/v1/payment/orders`

**请求头**: 同上

**查询参数**:
```
page=1              // 页码（默认1）
pageSize=20         // 每页数量（默认10）
userId=user_123     // 可选：用户ID
status=1            // 可选：订单状态 0-未支付 1-已支付 2-已关闭
payWay=alipay       // 可选：支付方式
```

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "list": [
      {
        "orderId": "1",
        "orderNo": "202512280001234567",
        "subject": "VIP会员月卡",
        "amount": 29.9,
        "status": 1,
        "payWay": "alipay",
        "userId": "user_123",
        "createdAt": "2025-12-28 10:00:00",
        "paidAt": "2025-12-28 10:05:00"
      }
    ],
    "total": 100,
    "page": 1,
    "pageSize": 20
  }
}
```

#### 3.3 关闭微信订单

**接口地址**: `POST /api/v1/payment/wechat/close/:outTradeNo`

**请求头**: 同上

**路径参数**:
- `outTradeNo`: 订单号

**响应示例**:
```json
{
  "code": 200,
  "message": "订单已关闭"
}
```

#### 3.4 微信退款

**接口地址**: `POST /api/v1/payment/wechat/refund`

**请求头**: 同上

**请求参数**:
```json
{
  "outTradeNo": "202512280001234567",  // 订单号
  "refundAmount": 29.9                  // 退款金额
}
```

**响应示例**:
```json
{
  "code": 200,
  "message": "退款成功",
  "data": {
    "refundId": "refund_123",
    "amount": 29.9
  }
}
```

---

### 4. 环境配置

#### 4.1 开发环境配置

创建 `.env.development` 文件：

```bash
# API 配置
REACT_APP_API_URL=http://localhost:8097
REACT_APP_PAYMENT_APP_ID=admin-frontend
REACT_APP_PAYMENT_APP_SECRET=admin-frontend-secret-key-32chars

# Vue3 使用 VITE_ 前缀
VITE_API_URL=http://localhost:8097
VITE_PAYMENT_APP_ID=admin-frontend
VITE_PAYMENT_APP_SECRET=admin-frontend-secret-key-32chars
```

#### 4.2 生产环境配置

创建 `.env.production` 文件：

```bash
# API 配置
REACT_APP_API_URL=https://api.your-domain.com
REACT_APP_PAYMENT_APP_ID=production-app-001
REACT_APP_PAYMENT_APP_SECRET=production-secret-key-very-long-and-secure

# Vue3 使用 VITE_ 前缀
VITE_API_URL=https://api.your-domain.com
VITE_PAYMENT_APP_ID=production-app-001
VITE_PAYMENT_APP_SECRET=production-secret-key-very-long-and-secure
```

**⚠️ 安全提醒**:
- ❌ **不要**将 `.env` 文件提交到 Git 仓库
- ✅ 将 `.env` 添加到 `.gitignore`
- ✅ 生产环境使用环境变量或密钥管理服务
- ✅ AppSecret 至少 32 字符

---

### 5. 错误处理

#### 5.1 常见错误码

| 错误码 | 错误信息 | 原因 | 解决方法 |
|-------|---------|------|---------|
| 401 | Invalid signature | 签名错误 | 检查 AppID 和 AppSecret 是否正确 |
| 401 | Timestamp expired | 时间戳过期 | 检查本地时间是否正确 |
| 401 | AppID not found | AppID 不存在 | 联系后端确认 AppID 配置 |
| 403 | IP not in whitelist | IP 不在白名单 | 联系后端添加 IP 白名单 |
| 429 | Rate limit exceeded | 请求过于频繁 | 减少请求频率或联系后端提高限制 |
| 400 | Invalid request | 请求参数错误 | 检查请求参数格式 |
| 500 | Internal server error | 服务器错误 | 联系后端排查问题 |

#### 5.2 错误处理示例

```typescript
async function handlePayment() {
  try {
    const result = await client.createPayment({
      subject: 'VIP会员',
      amount: 99.00,
      payWay: 'alipay',
    });

    if (result.code === 200) {
      // 成功：跳转到支付页面
      window.location.href = result.data.payUrl;
    } else {
      // 业务错误
      showError(`支付失败: ${result.message}`);
    }
  } catch (error) {
    // 网络错误或其他异常
    if (error instanceof TypeError && error.message.includes('Failed to fetch')) {
      showError('网络连接失败，请检查网络');
    } else if (error instanceof Error) {
      showError(`系统错误: ${error.message}`);
    } else {
      showError('未知错误，请稍后重试');
    }
  }
}

function showError(message: string) {
  // 使用你的 UI 库显示错误提示
  // 例如: Toast.error(message)
  console.error(message);
  alert(message);
}
```

#### 5.3 调试模式

```typescript
export class PaymentClient {
  private debug = false;

  constructor(config: PaymentConfig, debug = false) {
    this.config = config;
    this.debug = debug;
  }

  private getAuthHeaders(): Record<string, string> {
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const nonce = this.generateNonce();
    const signature = this.generateSignature(timestamp, nonce);

    const headers = {
      'Content-Type': 'application/json',
      'X-App-Id': this.config.appId,
      'X-Timestamp': timestamp,
      'X-Nonce': nonce,
      'X-Sign': signature,
    };

    // 调试模式：打印签名信息
    if (this.debug) {
      console.group('🔐 GoAuth 签名信息');
      console.log('AppID:', this.config.appId);
      console.log('Timestamp:', timestamp);
      console.log('Nonce:', nonce);
      console.log('签名字符串:', `appId=${this.config.appId}&nonce=${nonce}&timestamp=${timestamp}`);
      console.log('Signature:', signature);
      console.log('请求头:', headers);
      console.groupEnd();
    }

    return headers;
  }
}

// 开启调试模式
const client = new PaymentClient({
  baseURL: 'http://localhost:8097',
  appId: 'admin-frontend',
  appSecret: 'admin-frontend-secret-key-32chars',
}, true); // 第二个参数设为 true
```

---

### 6. 测试工具

#### 6.1 在线签名测试页面

创建一个简单的测试页面来验证签名生成是否正确：

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <title>支付签名测试工具</title>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/crypto-js/4.1.1/crypto-js.min.js"></script>
  <style>
    body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
    input, textarea { width: 100%; padding: 8px; margin: 5px 0; box-sizing: border-box; }
    button { background: #007bff; color: white; padding: 10px 20px; border: none; cursor: pointer; }
    button:hover { background: #0056b3; }
    .result { background: #f5f5f5; padding: 15px; margin: 10px 0; border-radius: 4px; }
    .success { color: #28a745; }
    .error { color: #dc3545; }
  </style>
</head>
<body>
  <h1>🔐 支付签名测试工具</h1>
  
  <h3>配置信息</h3>
  <input type="text" id="appId" placeholder="App ID" value="admin-frontend">
  <input type="password" id="appSecret" placeholder="App Secret" value="admin-frontend-secret-key-32chars">
  
  <h3>签名参数（自动生成）</h3>
  <input type="text" id="timestamp" placeholder="时间戳" readonly>
  <input type="text" id="nonce" placeholder="随机字符串" readonly>
  
  <button onclick="generateSignature()">生成签名</button>
  <button onclick="testAPI()">测试接口调用</button>
  
  <div class="result">
    <h3>签名结果</h3>
    <p><strong>签名字符串:</strong> <span id="signString">-</span></p>
    <p><strong>签名值:</strong> <span id="signature">-</span></p>
  </div>
  
  <div class="result">
    <h3>接口测试结果</h3>
    <pre id="apiResult">点击"测试接口调用"按钮进行测试</pre>
  </div>

  <script>
    function generateNonce(length = 32) {
      const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
      let result = '';
      for (let i = 0; i < length; i++) {
        result += chars.charAt(Math.floor(Math.random() * chars.length));
      }
      return result;
    }

    function generateSignature() {
      const appId = document.getElementById('appId').value;
      const appSecret = document.getElementById('appSecret').value;
      const timestamp = Math.floor(Date.now() / 1000).toString();
      const nonce = generateNonce();

      document.getElementById('timestamp').value = timestamp;
      document.getElementById('nonce').value = nonce;

      // 生成签名字符串
      const signString = `appId=${appId}&nonce=${nonce}&timestamp=${timestamp}`;
      
      // HMAC-SHA256 签名
      const signature = CryptoJS.HmacSHA256(signString, appSecret).toString(CryptoJS.enc.Hex).toLowerCase();

      document.getElementById('signString').textContent = signString;
      document.getElementById('signature').textContent = signature;
    }

    async function testAPI() {
      const appId = document.getElementById('appId').value;
      const timestamp = document.getElementById('timestamp').value;
      const nonce = document.getElementById('nonce').value;
      const signature = document.getElementById('signature').textContent;

      if (!timestamp || signature === '-') {
        alert('请先点击"生成签名"按钮');
        return;
      }

      try {
        const response = await fetch('http://localhost:8097/api/v1/payment/orders?page=1&pageSize=10', {
          method: 'GET',
          headers: {
            'X-App-Id': appId,
            'X-Timestamp': timestamp,
            'X-Nonce': nonce,
            'X-Sign': signature,
          },
        });

        const result = await response.json();
        document.getElementById('apiResult').textContent = JSON.stringify(result, null, 2);
        document.getElementById('apiResult').className = response.ok ? 'success' : 'error';
      } catch (error) {
        document.getElementById('apiResult').textContent = `错误: ${error.message}`;
        document.getElementById('apiResult').className = 'error';
      }
    }

    // 页面加载时自动生成一次
    window.onload = generateSignature;
  </script>
</body>
</html>
```

#### 6.2 命令行测试脚本

创建 `test-payment-api.ts` 文件：

```typescript
import { PaymentClient } from './payment-client';

async function runTests() {
  const client = new PaymentClient({
    baseURL: 'http://localhost:8097',
    appId: 'admin-frontend',
    appSecret: 'admin-frontend-secret-key-32chars',
  }, true); // 开启调试模式

  console.log('========== 测试开始 ==========\n');

  // 测试1: 查询订单列表
  console.log('📋 测试1: 查询订单列表');
  try {
    const orders = await client.getOrders({ page: 1, pageSize: 10 });
    console.log('✅ 成功:', orders);
  } catch (error) {
    console.error('❌ 失败:', error);
  }
  console.log('\n');

  // 测试2: 创建支付订单（不会真的跳转）
  console.log('💰 测试2: 创建支付订单');
  try {
    const payment = await client.createPayment({
      subject: '测试商品',
      amount: 0.01,
      payWay: 'alipay',
      userId: 'test_user',
    });
    console.log('✅ 成功:', payment);
    console.log('支付链接:', payment.data.payUrl);
  } catch (error) {
    console.error('❌ 失败:', error);
  }

  console.log('\n========== 测试完成 ==========');
}

runTests();
```

运行测试：

```bash
npx ts-node test-payment-api.ts
```

---

### 7. 常见问题 FAQ

#### Q1: 为什么签名验证总是失败？

**A**: 检查以下几点：

1. **AppID 和 AppSecret 是否正确**
   ```typescript
   // 确保从后端获取正确的配置
   console.log('AppID:', appId);
   console.log('AppSecret:', appSecret); // 生产环境不要打印
   ```

2. **签名字符串拼接顺序**
   ```typescript
   // ✅ 正确：按字母顺序 appId, nonce, timestamp
   const signStr = `appId=${appId}&nonce=${nonce}&timestamp=${timestamp}`;
   
   // ❌ 错误：顺序不对
   const signStr = `timestamp=${timestamp}&appId=${appId}&nonce=${nonce}`;
   ```

3. **时间戳格式**
   ```typescript
   // ✅ 正确：Unix 时间戳（秒）
   const timestamp = Math.floor(Date.now() / 1000).toString();
   
   // ❌ 错误：毫秒
   const timestamp = Date.now().toString();
   ```

4. **签名结果大小写**
   ```typescript
   // ✅ 正确：转换为小写
   return hash.toString(CryptoJS.enc.Hex).toLowerCase();
   
   // ⚠️ 虽然服务端不区分大小写，但建议统一使用小写
   ```

#### Q2: 时间戳总是提示过期怎么办？

**A**: 
1. 检查本地时间是否正确
   ```javascript
   console.log('本地时间:', new Date());
   console.log('时间戳:', Math.floor(Date.now() / 1000));
   ```

2. 确保时间戳是秒不是毫秒
   ```javascript
   // ✅ 正确
   const timestamp = Math.floor(Date.now() / 1000);
   
   // ❌ 错误
   const timestamp = Date.now();
   ```

3. 如果跨时区，确保使用 UTC 时间
   ```javascript
   const timestamp = Math.floor(Date.now() / 1000).toString();
   ```

#### Q3: 浏览器控制台出现 CORS 错误怎么办？

**A**: 这是跨域问题，需要后端配置 CORS。联系后端工程师在服务端添加以下配置：

```go
// 后端已配置的 CORS 头
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: POST, GET, OPTIONS, PUT, DELETE
Access-Control-Allow-Headers: Authorization, Content-Type, X-App-Id, X-Timestamp, X-Nonce, X-Sign
```

如果是本地开发，可以使用代理：

**Vite 配置**:
```typescript
// vite.config.ts
export default defineConfig({
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8089',
        changeOrigin: true,
      },
    },
  },
});
```

**React (Create React App)**:
```json
// package.json
{
  "proxy": "http://localhost:8089"
}
```

#### Q4: 能不能在前端直接硬编码 AppSecret？

**A**: ❌ **强烈不建议！**

前端代码是公开的，任何人都可以查看。如果硬编码 AppSecret，会导致严重的安全问题。

**推荐方案**：
1. 使用环境变量（`.env` 文件）
2. 仅在开发和测试环境使用
3. 生产环境通过中间层（BFF）调用，不在前端暴露

**更安全的架构**：
```
前端 → 自己的后端（BFF）→ 支付后端
```

你的后端（BFF）持有 AppSecret，前端只调用你的后端。

#### Q5: 如何在 React Native 中使用？

**A**: 使用 `react-native-crypto-js`：

```bash
npm install react-native-crypto-js
```

```typescript
import CryptoJS from 'react-native-crypto-js';

function generateSignature(appId, appSecret, timestamp, nonce) {
  const signStr = `appId=${appId}&nonce=${nonce}&timestamp=${timestamp}`;
  const hash = CryptoJS.HmacSHA256(signStr, appSecret);
  return hash.toString(CryptoJS.enc.Hex).toLowerCase();
}
```

其他代码与 Web 版本相同。

---

### 8. 下一步

#### 8.1 集成检查清单

- [ ] 安装 `crypto-js` 依赖
- [ ] 复制 `PaymentClient` 类到项目中
- [ ] 配置环境变量（AppID、AppSecret、API URL）
- [ ] 实现签名生成函数
- [ ] 测试签名是否正确（使用测试工具）
- [ ] 调用接口并处理响应
- [ ] 实现错误处理
- [ ] 测试完整支付流程
- [ ] 配置生产环境变量

#### 8.2 联系方式

如有问题，请联系后端团队：

- **后端负责人**: [后端工程师姓名]
- **技术支持**: [技术支持邮箱/企业微信]
- **API 文档**: http://localhost:8089/swagger/index.html
- **项目仓库**: [Git 仓库地址]

---

## 附录

### A. 完整的 package.json 依赖

```json
{
  "dependencies": {
    "crypto-js": "^4.1.1"
  },
  "devDependencies": {
    "@types/crypto-js": "^4.1.1"
  }
}
```

### B. TypeScript 类型定义

```typescript
// types/payment.ts
export interface PaymentConfig {
  baseURL: string;
  appId: string;
  appSecret: string;
}

export interface PaymentRequest {
  subject: string;
  amount: number;
  payWay: 'alipay' | 'wechat' | 'paypal';
  userId?: string;
  orderType?: string;
  extra?: string;
}

export interface PaymentResponse {
  code: number;
  message: string;
  data: {
    payUrl: string;
    orderNo: string;
    amount: number;
    payWay: string;
  };
}

export interface Order {
  orderId: string;
  orderNo: string;
  subject: string;
  amount: number;
  status: number;
  payWay: string;
  userId: string;
  createdAt: string;
  paidAt?: string;
}

export interface OrderListResponse {
  code: number;
  message: string;
  data: {
    list: Order[];
    total: number;
    page: number;
    pageSize: number;
  };
}
```

### C. 签名生成在线工具

可以使用以下在线工具验证 HMAC-SHA256 签名：
- https://www.devglan.com/online-tools/hmac-sha256-online
- https://emn178.github.io/online-tools/sha256.html

---

**文档版本**: v1.0  
**最后更新**: 2025年12月28日  
**维护者**: 后端技术团队
