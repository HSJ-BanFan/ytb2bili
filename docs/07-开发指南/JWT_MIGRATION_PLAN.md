# JWT 迁移计划：从 ytb2bili 本地实现迁移到 go-auth

## 概述

本文档说明如何将 `ytb2bili` 项目的 JWT 认证从本地实现迁移到统一的 `go-auth` 库，实现认证模块的复用。

## 当前状态分析

### ytb2bili 本地 JWT 实现

**文件位置**: `internal/auth/jwt.go` (148行)

```go
// 核心结构
type JWTConfig struct {
    SecretKey     string
    Issuer        string
    AccessExpiry  time.Duration
    RefreshExpiry time.Duration
}

type UserClaims struct {
    UserID   uint   `json:"user_id"`
    Username string `json:"username"`
    Role     string `json:"role"`      // 用户角色
    AppID    string `json:"app_id"`
    jwt.RegisteredClaims
}

// 核心方法
- GenerateAccessToken(userID uint, username, role, appID string)
- GenerateRefreshToken(userID uint)
- ParseToken(tokenString string) (*UserClaims, error)
- GenerateTokenPair(userID uint, username, role, appID string)
- HashToken(token string) string
```

### go-auth 现有 JWT 实现

**文件位置**: `jwt.go` (252行)

```go
// 核心结构
type JWTConfig struct {
    SecretKey     string
    Issuer        string
    AccessExpiry  time.Duration
    RefreshExpiry time.Duration
}

type Claims struct {
    UserID   uint64 `json:"user_id"`      // 注意：uint64 vs uint
    Username string `json:"username"`
    Role     string `json:"role"`         // 对应 ytb2bili 的角色
    AppID    string `json:"app_id"`
    Type     TokenType `json:"type"`      // go-auth 特有：Token 类型
    Custom   map[string]interface{} `json:"custom,omitempty"` // 扩展字段
    jwt.RegisteredClaims
}

// 核心方法
- GenerateAccessToken(userID uint64, username, role, appID string, custom map[string]interface{})
- GenerateRefreshToken(userID uint64, username string)
- ParseToken(tokenString string) (*Claims, error)
- GenerateTokenPair(userID uint64, username, role, appID string, custom map[string]interface{})
- WithBlacklist(blacklist TokenBlacklist) *JWTService
- RevokeToken(tokenString string) error
```

## 差异对比

| 特性 | ytb2bili 本地 | go-auth | 迁移影响 |
|------|--------------|---------|---------|
| UserID 类型 | `uint` | `uint64` | 需要类型转换 |
| 角色字段 | `Role` | `Role` | 无需转换 |
| Token 类型标识 | 无 | `Type` | go-auth 更安全 |
| 自定义字段 | 无 | `Custom` | 可用于扩展业务字段 |
| Token 黑名单 | 外部实现 | 内置支持 | 可简化代码 |
| Token 撤销 | 无 | `RevokeToken()` | 新功能 |

## 迁移方案

### 方案 A：直接使用 go-auth（推荐）

直接使用 go-auth 的 `Role` 字段，保持认证角色语义一致。

```go
// 使用 go-auth 生成 Token
tokenPair, err := jwtService.GenerateTokenPair(
    uint64(user.ID),
    user.Username,
    user.Role,      // 用户角色
    appID,
    nil,
)
```

### 方案 B：扩展 go-auth Claims

如需新的业务字段，可在 `Custom` 中扩展，但不要改变认证角色字段。
**推荐使用方案 A**。

---

## 详细迁移步骤

### 步骤 1：更新依赖

```bash
# 在 ytb2bili 项目中
go get github.com/difyz9/go-auth@latest
```

### 步骤 2：创建适配层

创建 `internal/auth/goauth_adapter.go`:

```go
package auth

import (
    "github.com/difyz9/go-auth"
)

// GoAuthJWTService 包装 go-auth 的 JWTService
type GoAuthJWTService struct {
    service *goauth.JWTService
}

// NewGoAuthJWTService 创建适配器
func NewGoAuthJWTService(config JWTConfig) *GoAuthJWTService {
    goauthConfig := goauth.JWTConfig{
        SecretKey:     config.SecretKey,
        Issuer:        config.Issuer,
        AccessExpiry:  config.AccessExpiry,
        RefreshExpiry: config.RefreshExpiry,
    }
    return &GoAuthJWTService{
        service: goauth.NewJWTService(goauthConfig),
    }
}

// GenerateTokenPair 生成 Token 对（兼容旧接口）
func (s *GoAuthJWTService) GenerateTokenPair(userID uint, username, tier, appID string) (*TokenPair, error) {
    custom := map[string]interface{}{
        "tier": tier,
    }
    
    pair, err := s.service.GenerateTokenPair(uint64(userID), username, tier, appID, custom)
    if err != nil {
        return nil, err
    }
    
    return &TokenPair{
        AccessToken:  pair.AccessToken,
        RefreshToken: pair.RefreshToken,
        ExpiresAt:    pair.ExpiresAt,
        TokenType:    pair.TokenType,
    }, nil
}

// ParseToken 解析 Token（返回兼容的 UserClaims）
func (s *GoAuthJWTService) ParseToken(tokenString string) (*UserClaims, error) {
    claims, err := s.service.ParseToken(tokenString)
    if err != nil {
        switch err {
        case goauth.ErrExpiredToken:
            return nil, ErrExpiredToken
        case goauth.ErrTokenRevoked:
            return nil, ErrTokenRevoked
        default:
            return nil, ErrInvalidToken
        }
    }
    
    // 从 Custom 中提取 tier
    tier := ""
    if claims.Custom != nil {
        if t, ok := claims.Custom["tier"].(string); ok {
            tier = t
        }
    }
    if tier == "" {
        tier = claims.Role // fallback
    }
    
    return &UserClaims{
        UserID:           uint(claims.UserID),
        Username:         claims.Username,
        Tier:             tier,
        AppID:            claims.AppID,
        RegisteredClaims: claims.RegisteredClaims,
    }, nil
}

// RevokeToken 撤销 Token（新功能）
func (s *GoAuthJWTService) RevokeToken(tokenString string) error {
    return s.service.RevokeToken(tokenString)
}
```

### 步骤 3：修改 main.go 初始化

```diff
// main.go
- jwtService := auth.NewJWTService(jwtConfig)
+ jwtService := auth.NewGoAuthJWTService(jwtConfig)
```

### 步骤 4：更新中间件

`internal/auth/middleware.go` 中的 `JWTAuth()` 方法需要更新：

```go
// 如果使用适配器，接口保持不变
func (m *AuthMiddleware) JWTAuth() gin.HandlerFunc {
    // 原有逻辑不变，因为适配器保持了接口兼容
}
```

### 步骤 5：验证测试

```bash
# 运行测试
go test ./internal/auth/... -v

# 启动应用验证
.\run_prod.ps1
```

---

## 迁移检查清单

- [ ] 安装 go-auth 依赖
- [ ] 创建 `goauth_adapter.go` 适配层
- [ ] 修改 `main.go` 使用新的 JWTService
- [ ] 更新中间件（如需要）
- [ ] 运行单元测试
- [ ] 运行集成测试
- [ ] 验证登录功能
- [ ] 验证 Token 刷新功能
- [ ] 验证 Token 撤销功能

---

## 后续优化（可选）

### 1. 移除本地 JWT 实现

迁移完成并验证后，可以删除：
- `internal/auth/jwt.go` (保留 `UserClaims` 和 `TokenPair` 类型定义)

### 2. 统一错误处理

```go
// 在适配器中统一错误映射
var errorMapping = map[error]error{
    goauth.ErrInvalidToken:  ErrInvalidToken,
    goauth.ErrExpiredToken:  ErrExpiredToken,
    goauth.ErrTokenRevoked:  ErrTokenRevoked,
    goauth.ErrInvalidClaims: ErrInvalidClaims,
}
```

### 3. 使用数据库黑名单

```go
// 替换内存黑名单为数据库实现
blacklist := NewDatabaseBlacklist(db)
jwtService.service.WithBlacklist(blacklist)
```

---

---

## 时间估算

| 任务 | 预计时间 |
|------|---------|
| 创建适配层 | 30分钟 |
| 修改 main.go | 10分钟 |
| 更新中间件 | 20分钟 |
| 测试验证 | 30分钟 |
| **总计** | **1.5小时** |

---

## 相关文件

| 项目 | 文件 | 说明 |
|------|------|------|
| ytb2bili | `internal/auth/jwt.go` | 当前本地实现 |
| ytb2bili | `internal/auth/middleware.go` | JWT 中间件 |
| ytb2bili | `internal/auth/handler.go` | 使用 JWT 的处理器 |
| go-auth | `jwt.go` | go-auth JWT 服务 |
| go-auth | `jwt_middleware.go` | go-auth JWT 中间件 |
| go-auth | `token_blacklist.go` | Token 黑名单实现 |
