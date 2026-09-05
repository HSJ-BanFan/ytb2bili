package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT 错误定义
var (
	ErrInvalidToken  = errors.New("无效的 token")
	ErrExpiredToken  = errors.New("token 已过期")
	ErrInvalidClaims = errors.New("无效的 token claims")
	ErrTokenRevoked  = errors.New("token 已被撤销")
)

// JWTConfig JWT 配置
type JWTConfig struct {
	SecretKey     string        // 密钥
	Issuer        string        // 签发者
	AccessExpiry  time.Duration // Access Token 有效期
	RefreshExpiry time.Duration // Refresh Token 有效期
}

// DefaultJWTConfig 默认配置
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		SecretKey:     "your-secret-key-change-in-production", // 生产环境必须修改
		Issuer:        "bili-up",
		AccessExpiry:  24 * time.Hour,     // Access Token 24小时
		RefreshExpiry: 7 * 24 * time.Hour, // Refresh Token 7天
	}
}

// UserClaims 用户 JWT Claims
type UserClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`   // 用户角色
	AppID    string `json:"app_id"` // 来源应用
	jwt.RegisteredClaims
}

// TokenPair Token 对
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// HashToken 计算 Token 哈希 (用于黑名单)
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// ===============================================
// 注意：以下旧的 JWTService 实现已废弃
// 请使用 goauth_adapter.go 中的 GoAuthJWTService
// ===============================================

// JWTService 旧版 JWT 服务（已废弃，保留用于向后兼容）
// Deprecated: 请使用 NewGoAuthJWTService 代替
type JWTService struct {
	config JWTConfig
}

// NewJWTService 创建 JWT 服务（已废弃）
// Deprecated: 请使用 NewGoAuthJWTService 代替
func NewJWTService(config JWTConfig) *JWTService {
	return &JWTService{config: config}
}

// GenerateAccessToken 生成 Access Token
func (s *JWTService) GenerateAccessToken(userID uint, username, role, appID string) (string, error) {
	now := time.Now()
	claims := UserClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		AppID:    appID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.config.Issuer,
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.AccessExpiry)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.SecretKey))
}

// GenerateRefreshToken 生成 Refresh Token
func (s *JWTService) GenerateRefreshToken(userID uint) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    s.config.Issuer,
		Subject:   "refresh",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.config.RefreshExpiry)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.SecretKey))
}

// ParseToken 解析 Token
func (s *JWTService) ParseToken(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(s.config.SecretKey), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}

// GenerateTokenPair 生成 Token 对
func (s *JWTService) GenerateTokenPair(userID uint, username, role, appID string) (*TokenPair, error) {
	accessToken, err := s.GenerateAccessToken(userID, username, role, appID)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.GenerateRefreshToken(userID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.config.AccessExpiry),
		TokenType:    "Bearer",
	}, nil
}
