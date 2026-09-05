package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Context Keys
const (
	ContextKeyUserID   = "user_id"
	ContextKeyUsername = "username"
	ContextKeyUserRole = "user_role" // 用户角色: admin/user
	ContextKeyAppID    = "app_id"
	ContextKeyApp      = "app"
	ContextKeyClaims   = "claims"
)

// JWTServiceInterface JWT 服务接口（用于支持多种实现）
type JWTServiceInterface interface {
	ParseToken(tokenString string) (*UserClaims, error)
	GenerateTokenPair(userID uint, username, role, appID string) (*TokenPair, error)
	GenerateAccessToken(userID uint, username, role, appID string) (string, error)
	GenerateRefreshToken(userID uint) (string, error)
}

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	db         *gorm.DB
	jwtService JWTServiceInterface
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(db *gorm.DB, jwtService JWTServiceInterface) *AuthMiddleware {
	return &AuthMiddleware{
		db:         db,
		jwtService: jwtService,
	}
}

// AppAuth App 认证中间件 (appId + appSecret)
// 支持两种方式:
// 1. Header: X-App-Id + X-App-Secret
// 2. Header: X-App-Id + X-Timestamp + X-Nonce + X-Signature (签名验证)
func (m *AuthMiddleware) AppAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.GetHeader("X-App-Id")
		if appID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "缺少 App ID",
			})
			c.Abort()
			return
		}

		// 查询 App
		var app model.App
		if err := m.db.Where("app_id = ?", appID).First(&app).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "无效的 App ID",
			})
			c.Abort()
			return
		}

		// 检查 App 状态
		if app.Status != 1 {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "应用已禁用",
			})
			c.Abort()
			return
		}

		// 验证方式1: 简单密钥验证
		appSecret := c.GetHeader("X-App-Secret")
		if appSecret != "" {
			if appSecret != app.AppSecret {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    401,
					"message": "无效的 App Secret",
				})
				c.Abort()
				return
			}
		} else {
			// 验证方式2: 签名验证
			timestamp := c.GetHeader("X-Timestamp")
			nonce := c.GetHeader("X-Nonce")
			signature := c.GetHeader("X-Signature")

			if timestamp == "" || nonce == "" || signature == "" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    401,
					"message": "缺少认证参数",
				})
				c.Abort()
				return
			}

			// 验证时间戳 (5分钟内有效)
			if !ValidateTimestamp(timestamp, 5*time.Minute) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    401,
					"message": "请求已过期",
				})
				c.Abort()
				return
			}

			// 验证签名
			if !VerifySignature(appID, app.AppSecret, timestamp, nonce, signature) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    401,
					"message": "签名验证失败",
				})
				c.Abort()
				return
			}
		}

		// 检查 IP 白名单 (如果配置了)
		if app.AllowedIPs != "" && app.AllowedIPs != "[]" {
			clientIP := c.ClientIP()
			if !strings.Contains(app.AllowedIPs, clientIP) && !strings.Contains(app.AllowedIPs, "*") {
				c.JSON(http.StatusForbidden, gin.H{
					"code":    403,
					"message": "IP 不在白名单中",
				})
				c.Abort()
				return
			}
		}

		// 设置 App 信息到 Context
		c.Set(ContextKeyAppID, app.AppID)
		c.Set(ContextKeyApp, &app)

		c.Next()
	}
}

// JWTAuth JWT 用户认证中间件
// Header: Authorization: Bearer <token>
func (m *AuthMiddleware) JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未登录",
			})
			c.Abort()
			return
		}

		// 解析 Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "无效的认证格式",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// 解析 Token
		claims, err := m.jwtService.ParseToken(tokenString)
		if err != nil {
			var msg string
			switch err {
			case ErrExpiredToken:
				msg = "登录已过期，请重新登录"
			case ErrInvalidToken, ErrInvalidClaims:
				msg = "无效的登录凭证"
			default:
				msg = "认证失败"
			}
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": msg,
			})
			c.Abort()
			return
		}

		// 检查 Token 是否被撤销
		tokenHash := HashToken(tokenString)
		var userTokens []model.UserToken
		// 使用 Find 而不是 First，避免 record not found 错误日志
		m.db.Where("token_hash = ? AND is_revoked = ?", tokenHash, true).Limit(1).Find(&userTokens)
		if len(userTokens) > 0 {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "登录已失效，请重新登录",
			})
			c.Abort()
			return
		}

		// 设置用户信息到 Context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyClaims, claims)

		// 从数据库加载用户角色
		var user struct {
			Role string `gorm:"column:role"`
		}
		if err := m.db.Table("cw_users").Select("role").Where("id = ?", claims.UserID).First(&user).Error; err != nil {
			// 查询失败，默认为普通用户
			c.Set(ContextKeyUserRole, "user")
		} else {
			// 检查角色是否为空，如果为空则默认为 user
			if user.Role == "" {
				user.Role = "user"
			}
			c.Set(ContextKeyUserRole, user.Role)
		}

		c.Next()
	}
}

// OptionalJWTAuth 可选的 JWT 认证 (不强制要求登录)
func (m *AuthMiddleware) OptionalJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := m.jwtService.ParseToken(parts[1])
		if err == nil {
			c.Set(ContextKeyUserID, claims.UserID)
			c.Set(ContextKeyUsername, claims.Username)
			c.Set(ContextKeyClaims, claims)

			// 从数据库加载用户角色
			var user struct {
				Role string `gorm:"column:role"`
			}
			if err := m.db.Table("cw_users").Select("role").Where("id = ?", claims.UserID).First(&user).Error; err != nil {
				c.Set(ContextKeyUserRole, "user")
			} else {
				if user.Role == "" {
					user.Role = "user"
				}
				c.Set(ContextKeyUserRole, user.Role)
			}
		}

		c.Next()
	}
}

// GetUserID 从 Context 获取用户 ID
func GetUserID(c *gin.Context) (uint, bool) {
	userID, exists := c.Get(ContextKeyUserID)
	if !exists {
		return 0, false
	}
	id, ok := userID.(uint)
	return id, ok
}

// GetUserIDString 从 Context 获取用户 ID (字符串)
func GetUserIDString(c *gin.Context) string {
	userID, exists := c.Get(ContextKeyUserID)
	if !exists {
		return ""
	}
	if id, ok := userID.(uint); ok {
		return fmt.Sprintf("%d", id)
	}
	return ""
}

// GetAppID 从 Context 获取 App ID
func GetAppID(c *gin.Context) string {
	appID, _ := c.Get(ContextKeyAppID)
	if id, ok := appID.(string); ok {
		return id
	}
	return ""
}
