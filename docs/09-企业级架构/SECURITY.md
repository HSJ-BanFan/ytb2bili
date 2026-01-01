# ytb2bili 安全开发指南

> **版本**: v1.0
> **日期**: 2025-12-29
> **目标读者**: 开发人员、安全工程师、运维人员

---

## 📋 目录

1. [执行摘要](#执行摘要)
2. [当前安全状态评估](#当前安全状态评估)
3. [认证与授权](#认证与授权)
4. [敏感数据保护](#敏感数据保护)
5. [SQL 注入防护](#sql-注入防护)
6. [审计日志](#审计日志)
7. [API 安全](#api-安全)
8. [配置安全](#配置安全)
9. [依赖安全](#依赖安全)
10. [安全检查清单](#安全检查清单)
11. [实施路线图](#实施路线图)

---

## 执行摘要

### 🎯 当前状态

| 维度 | 当前状态 | 企业级要求 | 差距 |
|------|---------|-----------|------|
| **认证机制** | ✅ JWT 认证 | ✅ JWT + Refresh Token | **良好** |
| **权限控制** | ⚠️ 基础角色 | ✅ 细粒度 RBAC | **中等** |
| **密码存储** | ✅ bcrypt 哈希 | ✅ bcrypt/argon2 | **良好** |
| **敏感数据加密** | ❌ 明文存储 | ✅ AES-256 加密 | **严重** |
| **SQL 注入防护** | ✅ GORM 参数化 | ✅ 参数化 + WAF | **良好** |
| **审计日志** | ❌ 无 | ✅ 完整操作记录 | **严重** |
| **配置安全** | ⚠️ 明文配置 | ✅ 环境变量/密钥管理 | **中等** |
| **HTTPS** | ⚠️ 可选 | ✅ 强制 HTTPS | **中等** |

### 📊 总体评估

**安全性评分**: **5/10**
**合规性评分**: **4/10**
**综合评分**: **4.5/10**

**结论**: 系统具备基本的安全机制，但缺少敏感数据加密和审计日志，不满足企业级数据保护要求。

---

## 当前安全状态评估

### ✅ 已实现的安全机制

```go
// 1. JWT 认证 ✅
// 位置: internal/auth/jwt.go
func GenerateToken(userID uint) (string, error) {
    claims := jwt.MapClaims{
        "user_id": userID,
        "exp":     time.Now().Add(24 * time.Hour).Unix(),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(jwtSecret))
}

// 2. 密码哈希 ✅
// 位置: internal/auth/password.go
func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
    return string(bytes), err
}

// 3. GORM 参数化查询 ✅
// 自动防止 SQL 注入
db.Where("user_id = ?", userID).Find(&videos)
```

### ❌ 安全缺陷

```go
// 1. 敏感数据明文存储 ❌
// 位置: pkg/store/model/user.go
type User struct {
    BiliCookies string `gorm:"column:bili_cookies"` // ❌ 明文存储
    AccessToken string `gorm:"column:access_token"` // ❌ 明文存储
}

// 2. 无审计日志 ❌
// 用户操作无记录，无法追溯安全事件

// 3. 配置明文存储 ❌
// config.toml
[deepseek]
api_key = "sk-xxxxxx"  // ❌ 明文
```

---

## 认证与授权

### 🔐 JWT 最佳实践

#### 当前实现改进

```go
// jwt_service.go
package auth

import (
    "crypto/rand"
    "encoding/base64"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

type TokenPair struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
}

type Claims struct {
    UserID   uint   `json:"user_id"`
    Username string `json:"username"`
    Role     string `json:"role"`
    jwt.RegisteredClaims
}

type JWTService struct {
    accessSecret  []byte
    refreshSecret []byte
    accessTTL     time.Duration
    refreshTTL    time.Duration
}

func NewJWTService(accessSecret, refreshSecret string) *JWTService {
    return &JWTService{
        accessSecret:  []byte(accessSecret),
        refreshSecret: []byte(refreshSecret),
        accessTTL:     15 * time.Minute,  // Access Token 短期有效
        refreshTTL:    7 * 24 * time.Hour, // Refresh Token 长期有效
    }
}

// GenerateTokenPair 生成 Access + Refresh Token 对
func (s *JWTService) GenerateTokenPair(userID uint, username, role string) (*TokenPair, error) {
    now := time.Now()

    // Access Token
    accessClaims := Claims{
        UserID:   userID,
        Username: username,
        Role:     role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
            IssuedAt:  jwt.NewNumericDate(now),
            NotBefore: jwt.NewNumericDate(now),
            Issuer:    "ytb2bili",
            Subject:   fmt.Sprintf("%d", userID),
        },
    }
    accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
    accessTokenStr, err := accessToken.SignedString(s.accessSecret)
    if err != nil {
        return nil, fmt.Errorf("生成 access token 失败: %w", err)
    }

    // Refresh Token (使用不同密钥)
    refreshClaims := jwt.RegisteredClaims{
        ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshTTL)),
        IssuedAt:  jwt.NewNumericDate(now),
        Subject:   fmt.Sprintf("%d", userID),
    }
    refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
    refreshTokenStr, err := refreshToken.SignedString(s.refreshSecret)
    if err != nil {
        return nil, fmt.Errorf("生成 refresh token 失败: %w", err)
    }

    return &TokenPair{
        AccessToken:  accessTokenStr,
        RefreshToken: refreshTokenStr,
        ExpiresIn:    int64(s.accessTTL.Seconds()),
    }, nil
}

// ValidateAccessToken 验证 Access Token
func (s *JWTService) ValidateAccessToken(tokenStr string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
        }
        return s.accessSecret, nil
    })

    if err != nil {
        return nil, fmt.Errorf("token 验证失败: %w", err)
    }

    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }

    return nil, fmt.Errorf("无效的 token claims")
}
```

### 👥 RBAC 权限控制

#### 角色定义

```go
// rbac/roles.go
package rbac

type Role string

const (
    RoleAdmin   Role = "admin"    // 管理员：所有权限
    RolePro     Role = "pro"      // 付费用户：高级功能
    RoleUser    Role = "user"     // 普通用户：基础功能
    RoleGuest   Role = "guest"    // 游客：只读
)

// 权限定义
type Permission string

const (
    // 视频管理
    PermVideoCreate  Permission = "video:create"
    PermVideoRead    Permission = "video:read"
    PermVideoUpdate  Permission = "video:update"
    PermVideoDelete  Permission = "video:delete"
    PermVideoUpload  Permission = "video:upload"
    
    // 用户管理
    PermUserRead     Permission = "user:read"
    PermUserUpdate   Permission = "user:update"
    PermUserDelete   Permission = "user:delete"
    
    // 系统管理
    PermConfigRead   Permission = "config:read"
    PermConfigUpdate Permission = "config:update"
    PermAuditRead    Permission = "audit:read"
)

// 角色权限映射
var RolePermissions = map[Role][]Permission{
    RoleAdmin: {
        PermVideoCreate, PermVideoRead, PermVideoUpdate, PermVideoDelete, PermVideoUpload,
        PermUserRead, PermUserUpdate, PermUserDelete,
        PermConfigRead, PermConfigUpdate, PermAuditRead,
    },
    RolePro: {
        PermVideoCreate, PermVideoRead, PermVideoUpdate, PermVideoDelete, PermVideoUpload,
        PermUserRead, PermUserUpdate,
    },
    RoleUser: {
        PermVideoCreate, PermVideoRead, PermVideoUpdate, PermVideoDelete,
        PermUserRead, PermUserUpdate,
    },
    RoleGuest: {
        PermVideoRead,
        PermUserRead,
    },
}

// HasPermission 检查角色是否有指定权限
func HasPermission(role Role, perm Permission) bool {
    perms, ok := RolePermissions[role]
    if !ok {
        return false
    }
    for _, p := range perms {
        if p == perm {
            return true
        }
    }
    return false
}
```

#### RBAC 中间件

```go
// middleware/rbac.go
package middleware

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

// RequirePermission 权限检查中间件
func RequirePermission(perm rbac.Permission) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 从 context 获取用户信息
        claims, exists := c.Get("claims")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
            c.Abort()
            return
        }

        userClaims := claims.(*auth.Claims)
        role := rbac.Role(userClaims.Role)

        // 检查权限
        if !rbac.HasPermission(role, perm) {
            c.JSON(http.StatusForbidden, gin.H{
                "error": "权限不足",
                "required": string(perm),
                "current_role": string(role),
            })
            c.Abort()
            return
        }

        c.Next()
    }
}

// 使用示例
router.DELETE("/api/v1/videos/:id", 
    authMiddleware.Authenticate(),
    RequirePermission(rbac.PermVideoDelete),
    videoHandler.DeleteVideo,
)
```

---

## 敏感数据保护

### 🔒 数据加密方案

#### 加密服务实现

```go
// crypto/encryption.go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "io"
)

type EncryptionService struct {
    key []byte // 32 字节密钥 (AES-256)
}

func NewEncryptionService(key string) (*EncryptionService, error) {
    // 密钥长度必须是 16, 24, 或 32 字节
    keyBytes := []byte(key)
    if len(keyBytes) != 32 {
        return nil, fmt.Errorf("密钥长度必须是 32 字节 (当前: %d)", len(keyBytes))
    }
    return &EncryptionService{key: keyBytes}, nil
}

// Encrypt 加密数据
func (s *EncryptionService) Encrypt(plaintext string) (string, error) {
    block, err := aes.NewCipher(s.key)
    if err != nil {
        return "", fmt.Errorf("创建 cipher 失败: %w", err)
    }

    // 使用 GCM 模式 (提供认证加密)
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("创建 GCM 失败: %w", err)
    }

    // 生成随机 nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", fmt.Errorf("生成 nonce 失败: %w", err)
    }

    // 加密 (nonce 附加在密文前面)
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密数据
func (s *EncryptionService) Decrypt(ciphertext string) (string, error) {
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return "", fmt.Errorf("base64 解码失败: %w", err)
    }

    block, err := aes.NewCipher(s.key)
    if err != nil {
        return "", fmt.Errorf("创建 cipher 失败: %w", err)
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("创建 GCM 失败: %w", err)
    }

    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize {
        return "", fmt.Errorf("密文长度不足")
    }

    nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
    if err != nil {
        return "", fmt.Errorf("解密失败: %w", err)
    }

    return string(plaintext), nil
}
```

#### 敏感字段加密存储

```go
// models/user_secure.go
package models

import (
    "github.com/difyz9/ytb2bili/pkg/crypto"
    "gorm.io/gorm"
)

type User struct {
    gorm.Model
    Username        string `gorm:"uniqueIndex"`
    PasswordHash    string // bcrypt 哈希
    Email           string
    Role            string
    
    // 敏感字段 (加密存储)
    BiliCookiesEnc  string `gorm:"column:bili_cookies_enc"`  // 加密后的 cookies
    AccessTokenEnc  string `gorm:"column:access_token_enc"`  // 加密后的 token
    RefreshTokenEnc string `gorm:"column:refresh_token_enc"` // 加密后的 refresh token
}

// UserService 带加密功能的用户服务
type UserService struct {
    db        *gorm.DB
    encryptor *crypto.EncryptionService
}

// SetBiliCookies 加密存储 Bilibili Cookies
func (s *UserService) SetBiliCookies(userID uint, cookies string) error {
    encrypted, err := s.encryptor.Encrypt(cookies)
    if err != nil {
        return fmt.Errorf("加密 cookies 失败: %w", err)
    }
    
    return s.db.Model(&User{}).
        Where("id = ?", userID).
        Update("bili_cookies_enc", encrypted).Error
}

// GetBiliCookies 解密获取 Bilibili Cookies
func (s *UserService) GetBiliCookies(userID uint) (string, error) {
    var user User
    if err := s.db.Select("bili_cookies_enc").First(&user, userID).Error; err != nil {
        return "", err
    }
    
    if user.BiliCookiesEnc == "" {
        return "", nil
    }
    
    return s.encryptor.Decrypt(user.BiliCookiesEnc)
}
```

### 🔑 密钥管理

```bash
# 生成安全的加密密钥 (32 字节)
openssl rand -base64 32

# 存储在环境变量中 (推荐)
export YTB2BILI_ENCRYPTION_KEY="your-32-byte-base64-key-here"
export YTB2BILI_JWT_SECRET="your-jwt-secret-here"
export YTB2BILI_JWT_REFRESH_SECRET="your-refresh-secret-here"
```

```go
// config/security.go
func LoadSecurityConfig() (*SecurityConfig, error) {
    encKey := os.Getenv("YTB2BILI_ENCRYPTION_KEY")
    if encKey == "" {
        return nil, fmt.Errorf("❌ 未设置 YTB2BILI_ENCRYPTION_KEY 环境变量")
    }
    
    jwtSecret := os.Getenv("YTB2BILI_JWT_SECRET")
    if jwtSecret == "" {
        return nil, fmt.Errorf("❌ 未设置 YTB2BILI_JWT_SECRET 环境变量")
    }
    
    return &SecurityConfig{
        EncryptionKey:    encKey,
        JWTSecret:        jwtSecret,
        JWTRefreshSecret: os.Getenv("YTB2BILI_JWT_REFRESH_SECRET"),
    }, nil
}
```

---

## SQL 注入防护

### ✅ 安全实践

#### 参数化查询 (已实现)

```go
// ✅ 正确：使用 GORM 参数化查询
db.Where("user_id = ? AND status = ?", userID, status).Find(&videos)

// ✅ 正确：使用命名参数
db.Where("user_id = @uid AND status = @status", 
    sql.Named("uid", userID), 
    sql.Named("status", status)).Find(&videos)

// ❌ 错误：字符串拼接
query := fmt.Sprintf("SELECT * FROM videos WHERE user_id = '%s'", userID)
db.Raw(query).Scan(&videos)
```

#### 输入验证

```go
// validator/input.go
package validator

import (
    "regexp"
    "strings"
)

// VideoID 验证
var videoIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)

func ValidateVideoID(videoID string) error {
    if !videoIDRegex.MatchString(videoID) {
        return fmt.Errorf("无效的视频 ID 格式")
    }
    return nil
}

// 用户名验证
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`)

func ValidateUsername(username string) error {
    if !usernameRegex.MatchString(username) {
        return fmt.Errorf("用户名必须是 3-20 位字母数字或下划线")
    }
    return nil
}

// 防止 XSS
func SanitizeInput(input string) string {
    // 移除 HTML 标签
    input = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(input, "")
    // 转义特殊字符
    input = strings.ReplaceAll(input, "<", "&lt;")
    input = strings.ReplaceAll(input, ">", "&gt;")
    input = strings.ReplaceAll(input, "'", "&#39;")
    input = strings.ReplaceAll(input, "\"", "&quot;")
    return input
}
```

---

## 审计日志

### 📝 审计日志实现

#### 审计日志模型

```go
// models/audit_log.go
package models

import (
    "time"
    "gorm.io/gorm"
)

type AuditLog struct {
    ID          uint      `gorm:"primaryKey"`
    Timestamp   time.Time `gorm:"index"`
    UserID      uint      `gorm:"index"`
    Username    string
    Action      string    `gorm:"index"` // CREATE, READ, UPDATE, DELETE, LOGIN, LOGOUT
    Resource    string    `gorm:"index"` // video, user, config, etc.
    ResourceID  string    // 资源的具体 ID
    OldValue    string    `gorm:"type:text"` // 修改前的值 (JSON)
    NewValue    string    `gorm:"type:text"` // 修改后的值 (JSON)
    IPAddress   string
    UserAgent   string
    RequestID   string    `gorm:"index"` // 请求追踪 ID
    Duration    int64     // 操作耗时 (毫秒)
    Status      string    // success, failed
    ErrorMsg    string    // 失败时的错误信息
}

func (AuditLog) TableName() string {
    return "cw_audit_logs"
}
```

#### 审计服务

```go
// services/audit_service.go
package services

import (
    "context"
    "encoding/json"
    "time"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

type AuditService struct {
    db *gorm.DB
}

func NewAuditService(db *gorm.DB) *AuditService {
    return &AuditService{db: db}
}

type AuditEntry struct {
    UserID     uint
    Username   string
    Action     string
    Resource   string
    ResourceID string
    OldValue   interface{}
    NewValue   interface{}
    Status     string
    ErrorMsg   string
}

// Log 记录审计日志
func (s *AuditService) Log(ctx context.Context, entry AuditEntry) error {
    var oldValueJSON, newValueJSON string
    
    if entry.OldValue != nil {
        if data, err := json.Marshal(entry.OldValue); err == nil {
            oldValueJSON = string(data)
        }
    }
    if entry.NewValue != nil {
        if data, err := json.Marshal(entry.NewValue); err == nil {
            newValueJSON = string(data)
        }
    }
    
    // 从 context 获取请求信息
    var ipAddress, userAgent, requestID string
    if ginCtx, ok := ctx.(*gin.Context); ok {
        ipAddress = ginCtx.ClientIP()
        userAgent = ginCtx.Request.UserAgent()
        requestID = ginCtx.GetHeader("X-Request-ID")
    }
    
    log := &models.AuditLog{
        Timestamp:  time.Now(),
        UserID:     entry.UserID,
        Username:   entry.Username,
        Action:     entry.Action,
        Resource:   entry.Resource,
        ResourceID: entry.ResourceID,
        OldValue:   oldValueJSON,
        NewValue:   newValueJSON,
        IPAddress:  ipAddress,
        UserAgent:  userAgent,
        RequestID:  requestID,
        Status:     entry.Status,
        ErrorMsg:   entry.ErrorMsg,
    }
    
    return s.db.Create(log).Error
}

// 使用示例
func (h *VideoHandler) DeleteVideo(c *gin.Context) {
    videoID := c.Param("id")
    userID := c.GetUint("user_id")
    username := c.GetString("username")
    
    // 获取旧数据（用于审计）
    oldVideo, _ := h.videoService.GetByID(videoID)
    
    // 执行删除
    err := h.videoService.Delete(videoID)
    
    // 记录审计日志
    status := "success"
    errMsg := ""
    if err != nil {
        status = "failed"
        errMsg = err.Error()
    }
    
    h.auditService.Log(c, AuditEntry{
        UserID:     userID,
        Username:   username,
        Action:     "DELETE",
        Resource:   "video",
        ResourceID: videoID,
        OldValue:   oldVideo,
        NewValue:   nil,
        Status:     status,
        ErrorMsg:   errMsg,
    })
    
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, gin.H{"message": "删除成功"})
}
```

#### 审计日志中间件

```go
// middleware/audit.go
package middleware

func AuditMiddleware(auditService *services.AuditService) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        // 生成请求 ID
        requestID := uuid.New().String()
        c.Header("X-Request-ID", requestID)
        c.Set("request_id", requestID)
        
        // 处理请求
        c.Next()
        
        // 记录请求审计 (仅记录写操作)
        if c.Request.Method != "GET" && c.Request.Method != "OPTIONS" {
            duration := time.Since(start).Milliseconds()
            
            status := "success"
            if c.Writer.Status() >= 400 {
                status = "failed"
            }
            
            auditService.Log(c, services.AuditEntry{
                UserID:   c.GetUint("user_id"),
                Username: c.GetString("username"),
                Action:   c.Request.Method,
                Resource: c.FullPath(),
                Status:   status,
            })
        }
    }
}
```

### 📊 审计日志查询

```sql
-- 查询用户最近操作
SELECT * FROM cw_audit_logs 
WHERE user_id = 123 
ORDER BY timestamp DESC 
LIMIT 100;

-- 查询删除操作
SELECT * FROM cw_audit_logs 
WHERE action = 'DELETE' 
AND timestamp > DATE_SUB(NOW(), INTERVAL 7 DAY);

-- 查询失败操作
SELECT * FROM cw_audit_logs 
WHERE status = 'failed' 
ORDER BY timestamp DESC;

-- 查询敏感资源访问
SELECT * FROM cw_audit_logs 
WHERE resource IN ('config', 'user') 
AND action IN ('UPDATE', 'DELETE');
```

---

## API 安全

### 🛡️ 速率限制

```go
// middleware/rate_limit.go
package middleware

import (
    "net/http"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
    "golang.org/x/time/rate"
)

type RateLimiter struct {
    visitors map[string]*rate.Limiter
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
    return &RateLimiter{
        visitors: make(map[string]*rate.Limiter),
        rate:     r,
        burst:    b,
    }
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    limiter, exists := rl.visitors[ip]
    if !exists {
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.visitors[ip] = limiter
    }

    return limiter
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        limiter := rl.getLimiter(ip)

        if !limiter.Allow() {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": "请求过于频繁，请稍后重试",
            })
            c.Abort()
            return
        }

        c.Next()
    }
}

// 使用
limiter := NewRateLimiter(10, 20) // 每秒 10 个请求，突发 20 个
router.Use(limiter.Middleware())
```

### 🔐 CORS 配置

```go
// middleware/cors.go
func CORSMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        origin := c.Request.Header.Get("Origin")
        
        // 白名单域名
        allowedOrigins := map[string]bool{
            "https://ytb2bili.com":     true,
            "https://app.ytb2bili.com": true,
        }
        
        if allowedOrigins[origin] {
            c.Header("Access-Control-Allow-Origin", origin)
        }
        
        c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
        c.Header("Access-Control-Max-Age", "86400")
        c.Header("Access-Control-Allow-Credentials", "true")
        
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        
        c.Next()
    }
}
```

### 📋 安全响应头

```go
// middleware/security_headers.go
func SecurityHeadersMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 防止点击劫持
        c.Header("X-Frame-Options", "DENY")
        
        // XSS 保护
        c.Header("X-XSS-Protection", "1; mode=block")
        
        // 防止 MIME 类型嗅探
        c.Header("X-Content-Type-Options", "nosniff")
        
        // 引用策略
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        
        // 内容安全策略
        c.Header("Content-Security-Policy", 
            "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
        
        // HSTS (仅 HTTPS)
        if c.Request.TLS != nil {
            c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        }
        
        c.Next()
    }
}
```

---

## 配置安全

### 🔐 敏感配置管理

#### 环境变量方案

```bash
# .env.example (提交到 Git)
# 复制此文件为 .env 并填入实际值

# 数据库
DB_HOST=localhost
DB_PORT=3306
DB_USER=ytb2bili
DB_PASSWORD=<your-password>
DB_NAME=ytb2bili

# 安全密钥
YTB2BILI_ENCRYPTION_KEY=<32-byte-base64-key>
YTB2BILI_JWT_SECRET=<jwt-secret>
YTB2BILI_JWT_REFRESH_SECRET=<refresh-secret>

# 第三方 API
DEEPSEEK_API_KEY=<api-key>
BILIBILI_APP_KEY=<app-key>
BILIBILI_APP_SECRET=<app-secret>
COS_SECRET_ID=<cos-secret-id>
COS_SECRET_KEY=<cos-secret-key>
```

```gitignore
# .gitignore
.env
.env.local
.env.*.local
config.toml
```

#### 配置加载

```go
// config/loader.go
package config

import (
    "os"
    "github.com/joho/godotenv"
)

func LoadConfig() (*AppConfig, error) {
    // 开发环境从 .env 加载
    if os.Getenv("GO_ENV") != "production" {
        godotenv.Load()
    }
    
    config := &AppConfig{
        Database: DatabaseConfig{
            Host:     getEnvOrDefault("DB_HOST", "localhost"),
            Port:     getEnvAsInt("DB_PORT", 3306),
            User:     getEnvOrPanic("DB_USER"),
            Password: getEnvOrPanic("DB_PASSWORD"),
            DBName:   getEnvOrDefault("DB_NAME", "ytb2bili"),
        },
        Security: SecurityConfig{
            EncryptionKey:    getEnvOrPanic("YTB2BILI_ENCRYPTION_KEY"),
            JWTSecret:        getEnvOrPanic("YTB2BILI_JWT_SECRET"),
            JWTRefreshSecret: getEnvOrDefault("YTB2BILI_JWT_REFRESH_SECRET", ""),
        },
    }
    
    return config, nil
}

func getEnvOrPanic(key string) string {
    value := os.Getenv(key)
    if value == "" {
        panic(fmt.Sprintf("❌ 必需的环境变量 %s 未设置", key))
    }
    return value
}
```

---

## 依赖安全

### 🔍 依赖扫描

```bash
# 使用 govulncheck 扫描漏洞
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

# 使用 nancy 扫描依赖
go install github.com/sonatype-nexus-community/nancy@latest
go list -json -m all | nancy sleuth
```

### 📝 GitHub Actions 安全扫描

```yaml
# .github/workflows/security.yml
name: Security Scan

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 0 * * 0'  # 每周日扫描

jobs:
  vulnerability-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          
      - name: Run govulncheck
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...
          
      - name: Run gosec
        uses: securego/gosec@master
        with:
          args: ./...
```

---

## 安全检查清单

### 🔍 上线前检查

```markdown
## 安全上线检查清单

### 认证与授权
- [ ] JWT 密钥是否足够强 (>= 256 位)
- [ ] Token 过期时间是否合理 (< 24 小时)
- [ ] 是否实现了 Refresh Token 机制
- [ ] 权限检查是否覆盖所有敏感 API

### 数据保护
- [ ] 敏感数据是否加密存储 (cookies, tokens)
- [ ] 密码是否使用 bcrypt/argon2 哈希
- [ ] 加密密钥是否安全存储 (非代码/配置)
- [ ] 数据库连接是否使用 TLS

### 输入验证
- [ ] 所有用户输入是否验证
- [ ] SQL 查询是否全部参数化
- [ ] 是否防止 XSS 攻击
- [ ] 文件上传是否验证类型和大小

### 网络安全
- [ ] 是否强制 HTTPS
- [ ] CORS 是否正确配置白名单
- [ ] 安全响应头是否配置
- [ ] 是否实现速率限制

### 审计与监控
- [ ] 是否记录所有写操作
- [ ] 登录失败是否记录
- [ ] 异常行为是否告警
- [ ] 日志是否保留足够时间

### 依赖安全
- [ ] 是否运行 govulncheck
- [ ] 是否定期更新依赖
- [ ] 是否移除未使用依赖
```

---

## 实施路线图

### 🎯 第一阶段: 紧急修复 (Week 1)

- [ ] 敏感数据加密存储实现
- [ ] 环境变量配置迁移
- [ ] 审计日志基础实现

**验收标准**:
- ✅ Cookies/Token 加密存储
- ✅ 敏感配置从环境变量读取
- ✅ 关键操作有审计日志

### 🎯 第二阶段: 增强认证 (Week 2)

- [ ] Refresh Token 机制
- [ ] RBAC 权限系统
- [ ] 登录失败锁定

**验收标准**:
- ✅ Token 自动续期
- ✅ 权限粒度到 API 级别
- ✅ 暴力破解防护

### 🎯 第三阶段: API 加固 (Week 3)

- [ ] 速率限制实现
- [ ] 安全响应头配置
- [ ] CORS 白名单配置
- [ ] HTTPS 强制

**验收标准**:
- ✅ API 有频率限制
- ✅ 安全扫描无高危漏洞

### 🎯 第四阶段: 监控告警 (Week 4)

- [ ] 安全事件告警
- [ ] 异常登录检测
- [ ] 依赖漏洞扫描 CI

**验收标准**:
- ✅ 异常行为实时告警
- ✅ 每周自动漏洞扫描

---

## 附录

### 📚 安全资源

- [OWASP Go 安全指南](https://owasp.org/www-project-go-secure-coding-practices-guide/)
- [CWE Top 25](https://cwe.mitre.org/top25/)
- [Go 安全最佳实践](https://blog.golang.org/secure-coding)

### 🔧 推荐工具

| 工具 | 用途 |
|------|------|
| govulncheck | Go 依赖漏洞扫描 |
| gosec | Go 代码安全扫描 |
| trivy | 容器/依赖漏洞扫描 |
| OWASP ZAP | Web 应用安全测试 |

---

**文档维护**: 请随着安全策略更新本文档
**安全问题报告**: security@ytb2bili.com

---

*最后更新: 2025-12-29*
