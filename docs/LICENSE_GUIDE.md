# 离线许可证使用指南

## 概述

本项目采用离线许可证系统实现会员权限管理，支持灵活的订阅计划和安全的许可证验证机制。

## 许可证类型

### 订阅计划

| 计划类型 | 代码 | 有效期 | 说明 |
|---------|------|--------|------|
| 7天体验 | trial | 7天 | 新用户体验，短期试用 |
| 月度订阅 | monthly | 30天 | 按月订阅 |
| 季度订阅 | quarterly | 90天 | 按季度订阅（3个月） |
| 年度订阅 | yearly | 365天 | 按年订阅（12个月） |
| 永久许可 | lifetime | 永久 | 一次购买，终身使用 |

### 会员等级

| 等级 | 代码 | 每日配额 | 并发任务 | 功能特性 |
|------|------|----------|----------|----------|
| Free | free | 1个视频 | 1个 | 基础下载功能 |
| Basic | basic | 10个视频 | 2个 | 自动上传、字幕翻译 |
| Pro | pro | 50个视频 | 5个 | AI元数据、自定义模板、优先支持 |
| Enterprise | enterprise | 无限 | 10个 | 全部功能、API访问、多账号 |

## 许可证格式

许可证格式: `ytb-v1-YBBAMOTTTTTTRRR-SSSSSSSSSSSS`

- `ytb-`: 产品前缀
- `v1`: 版本号
- `YB`: 产品代码 (YouTube to Bilibili)
- `BA/PR/EN`: 等级代码 (Basic/Pro/Enterprise)
- `TR/MO/QT/YR/LT`: 计划代码 (Trial/Monthly/Quarterly/Yearly/Lifetime)
- `TTTTTT`: 过期时间(Base36编码)
- `RRR`: 随机数
- `SSSSSSSSSSSS`: HMAC签名

### 示例许可证

```
# 7天体验版 - Basic
ytb-v1-YBBATR0000KNB3S-g8IBdKunw2LL

# 月度订阅 - Pro  
ytb-v1-YBPRMO0000LBQXR-oLsrT2qQeGEb

# 季度订阅 - Pro
ytb-v1-YBPRQT0000MYOR1-N7JVtYZRCPqN

# 年度订阅 - Enterprise
ytb-v1-YBENYR0000ULA8M-8qLYBCSkpyK8

# 永久许可 - Enterprise
ytb-v1-YBENLT000000RMU-lZyC2j6PTZZg
```

## API 接口

### 1. 验证许可证（不激活）

**请求**
```http
POST /api/v1/license/verify
Content-Type: application/json

{
  "license_key": "ytb-v1-YBPRMO0000LBQXR-oLsrT2qQeGEb"
}
```

**响应**
```json
{
  "code": 0,
  "message": "许可证有效",
  "data": {
    "license_key": "ytb-v1-YBPRMO0000LBQXR-oLsrT2qQeGEb",
    "tier": "pro",
    "plan": "monthly",
    "expires_at": "2026-02-06T10:30:00Z",
    "is_expired": false,
    "is_valid": true
  }
}
```

### 2. 激活许可证

> ⚠️ 需要登录认证

**请求**
```http
POST /api/v1/license/activate
Authorization: Bearer <token>
Content-Type: application/json

{
  "license_key": "ytb-v1-YBPRMO0000LBQXR-oLsrT2qQeGEb"
}
```

**响应**
```json
{
  "code": 0,
  "message": "许可证激活成功",
  "data": {
    "tier": "pro",
    "expires_at": "2026-02-06T10:30:00Z"
  }
}
```

### 3. 获取会员状态

> ⚠️ 需要登录认证

**请求**
```http
GET /api/v1/license/status
Authorization: Bearer <token>
```

**响应**
```json
{
  "code": 0,
  "data": {
    "tier": "pro",
    "expires_at": "2026-02-06T10:30:00Z",
    "activation_count": 2
  }
}
```

### 4. 获取许可证列表

> ⚠️ 需要登录认证

**请求**
```http
GET /api/v1/license/list
Authorization: Bearer <token>
```

**响应**
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "license_key": "ytb-****GEb",
      "user_id": "12345",
      "tier": "pro",
      "plan": "monthly",
      "activated_at": "2026-01-06T10:30:00Z",
      "expires_at": "2026-02-06T10:30:00Z",
      "is_valid": true
    }
  ]
}
```

### 5. 管理员生成许可证

> ⚠️ 需要管理员权限

**请求**
```http
POST /api/v1/admin/license/generate
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "tier": "pro",
  "plan": "monthly"
}
```

或自定义天数：
```json
{
  "tier": "basic",
  "plan": "trial",
  "days": 7
}
```

或自定义月数：
```json
{
  "tier": "pro",
  "plan": "monthly",
  "months": 3
}
```

**响应**
```json
{
  "code": 0,
  "message": "许可证生成成功",
  "data": {
    "license_key": "ytb-v1-YBPRMO0000LBQXR-oLsrT2qQeGEb",
    "tier": "pro",
    "plan": "monthly",
    "expires_at": "2026-02-06T10:30:00Z"
  }
}
```

## 使用流程

### 用户激活流程

1. 用户购买许可证（通过支付系统或管理员分发）
2. 用户登录系统
3. 在会员页面输入许可证密钥
4. 系统验证许可证并激活
5. 会员等级立即生效

### 管理员操作流程

1. 管理员登录后台
2. 调用生成许可证 API
3. 选择等级（basic/pro/enterprise）和计划类型
4. 系统生成许可证密钥
5. 将许可证发送给用户

## 安全机制

### 1. HMAC签名验证
- 使用 HMAC-SHA256 算法
- 密钥从环境变量 `LICENSE_SECRET_KEY` 读取
- 签名包含版本号和所有许可证数据

### 2. 一次性激活
- 每个许可证只能激活一次
- 激活后绑定到特定用户
- 无法转移或重复使用

### 3. 过期检查
- 验证时检查当前时间
- 过期许可证无法激活
- 已激活的许可证过期后会员降级

### 4. 环境变量配置
```bash
# 生产环境必须设置独立密钥
export LICENSE_SECRET_KEY="your-secret-key-here"
```

## 数据库表结构

### cw_license_activations 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint | 主键 |
| license_key | varchar(64) | 许可证密钥（唯一索引） |
| user_id | varchar(64) | 用户ID |
| tier | varchar(32) | 会员等级 |
| plan | varchar(32) | 订阅计划 |
| expires_at | datetime | 过期时间 |
| activated_at | datetime | 激活时间 |
| created_at | datetime | 创建时间 |

### cw_user_memberships 表

| 字段 | 类型 | 说明 |
|------|------|------|
| user_id | varchar(64) | 用户ID（主键） |
| tier | varchar(32) | 当前会员等级 |
| expires_at | datetime | 会员过期时间 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |

## 常见问题

### Q: 许可证格式不正确怎么办？
A: 确保许可证完整复制，不要包含空格或换行符。系统会自动处理大小写。

### Q: 许可证已被激活如何处理？
A: 每个许可证只能使用一次。如需更换用户，请生成新的许可证。

### Q: 如何延长用户会员期限？
A: 为用户生成新的许可证并激活。系统会自动取较晚的过期时间。

### Q: 许可证密钥丢失怎么办？
A: 用户可以在"许可证列表"页面查看已激活的许可证（密钥会部分掩码显示）。

### Q: 如何查看用户的会员状态？
A: 调用 `GET /api/v1/license/status` 接口获取当前有效的最高等级。

## 相关文件

- `internal/core/services/license_service.go` - 许可证服务核心逻辑
- `internal/core/services/permission_service.go` - 权限检查服务
- `internal/handler/license_handler.go` - API 接口处理器
- `internal/core/types/membership.go` - 会员类型定义

## 优势特性

1. **离线验证** - 无需联网即可验证许可证有效性
2. **灵活计划** - 支持多种订阅周期（7天/月/季/年/永久）
3. **安全可靠** - HMAC签名防止伪造
4. **简单易用** - 用户只需一个密钥即可激活
5. **可追溯** - 完整的激活记录和审计日志
