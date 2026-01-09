package auth

import (
	goauth "github.com/difyz9/go-auth"
)

// GoAuthJWTService 包装 go-auth 的 JWTService，提供兼容旧接口的适配层
type GoAuthJWTService struct {
	service *goauth.JWTService
	config  JWTConfig
}

// NewGoAuthJWTService 创建使用 go-auth 的 JWT 服务
func NewGoAuthJWTService(config JWTConfig) *GoAuthJWTService {
	goauthConfig := goauth.JWTConfig{
		SecretKey:     config.SecretKey,
		Issuer:        config.Issuer,
		AccessExpiry:  config.AccessExpiry,
		RefreshExpiry: config.RefreshExpiry,
	}
	return &GoAuthJWTService{
		service: goauth.NewJWTService(goauthConfig),
		config:  config,
	}
}

// GenerateAccessToken 生成 Access Token（兼容旧接口）
func (s *GoAuthJWTService) GenerateAccessToken(userID uint, username, tier, appID string) (string, error) {
	custom := map[string]interface{}{
		"tier": tier,
	}
	return s.service.GenerateAccessToken(uint64(userID), username, tier, appID, custom)
}

// GenerateRefreshToken 生成 Refresh Token（兼容旧接口）
func (s *GoAuthJWTService) GenerateRefreshToken(userID uint) (string, error) {
	return s.service.GenerateRefreshToken(uint64(userID), "")
}

// ParseToken 解析 Token 并返回兼容的 UserClaims
func (s *GoAuthJWTService) ParseToken(tokenString string) (*UserClaims, error) {
	claims, err := s.service.ParseToken(tokenString)
	if err != nil {
		// 映射错误类型
		switch err {
		case goauth.ErrExpiredToken:
			return nil, ErrExpiredToken
		case goauth.ErrTokenRevoked:
			return nil, ErrTokenRevoked
		case goauth.ErrInvalidClaims:
			return nil, ErrInvalidClaims
		default:
			return nil, ErrInvalidToken
		}
	}

	// 从 Custom 或 Role 中提取 tier
	tier := ""
	if claims.Custom != nil {
		if t, ok := claims.Custom["tier"].(string); ok {
			tier = t
		}
	}
	if tier == "" {
		tier = claims.Role // fallback 到 Role 字段
	}

	return &UserClaims{
		UserID:           uint(claims.UserID),
		Username:         claims.Username,
		Tier:             tier,
		AppID:            claims.AppID,
		RegisteredClaims: claims.RegisteredClaims,
	}, nil
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

// RevokeToken 撤销 Token（新功能，来自 go-auth）
func (s *GoAuthJWTService) RevokeToken(tokenString string) error {
	return s.service.RevokeToken(tokenString)
}

// ValidateAccessToken 验证 Access Token
func (s *GoAuthJWTService) ValidateAccessToken(tokenString string) (*UserClaims, error) {
	claims, err := s.service.ValidateAccessToken(tokenString)
	if err != nil {
		switch err {
		case goauth.ErrExpiredToken:
			return nil, ErrExpiredToken
		case goauth.ErrTokenRevoked:
			return nil, ErrTokenRevoked
		case goauth.ErrInvalidTokenType:
			return nil, ErrInvalidToken
		default:
			return nil, ErrInvalidToken
		}
	}

	tier := ""
	if claims.Custom != nil {
		if t, ok := claims.Custom["tier"].(string); ok {
			tier = t
		}
	}
	if tier == "" {
		tier = claims.Role
	}

	return &UserClaims{
		UserID:           uint(claims.UserID),
		Username:         claims.Username,
		Tier:             tier,
		AppID:            claims.AppID,
		RegisteredClaims: claims.RegisteredClaims,
	}, nil
}

// GetConfig 获取配置
func (s *GoAuthJWTService) GetConfig() JWTConfig {
	return s.config
}

// GetGoAuthService 获取底层 go-auth JWTService（用于高级功能）
func (s *GoAuthJWTService) GetGoAuthService() *goauth.JWTService {
	return s.service
}

// RefreshAccessToken 刷新 Access Token
func (s *GoAuthJWTService) RefreshAccessToken(refreshToken, tier, appID string) (string, error) {
	custom := map[string]interface{}{
		"tier": tier,
	}
	return s.service.RefreshAccessToken(refreshToken, tier, appID, custom)
}

// WithDatabaseBlacklist 设置数据库黑名单（可选）
func (s *GoAuthJWTService) WithDatabaseBlacklist(blacklist goauth.TokenBlacklist) *GoAuthJWTService {
	s.service.WithBlacklist(blacklist)
	return s
}
