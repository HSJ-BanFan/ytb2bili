package auth

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ============================================================================
// 测试辅助函数
// ============================================================================

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// 每个测试使用独立的内存数据库实例
	dbName := fmt.Sprintf("file:test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// 自动迁移
	err = db.AutoMigrate(&model.User{}, &model.UserToken{}, &model.App{})
	require.NoError(t, err)

	return db
}

func setupTestJWTService() *JWTService {
	config := JWTConfig{
		SecretKey:     "test_secret_key_for_testing_only",
		Issuer:        "bili-up-test",
		AccessExpiry:  24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
	return NewJWTService(config)
}

func createTestUser(t *testing.T, db *gorm.DB, role string) *model.User {
	t.Helper()

	// 使用时间戳确保邮箱唯一
	email := fmt.Sprintf("test_%d@example.com", time.Now().UnixNano())

	user := &model.User{
		Username:       "test_user",
		Email:          email,
		Password:       "hashed_password",
		Role:           role,
		Status:         1,
		EmailVerified:  true,
		MembershipTier: "free",
	}
	err := db.Create(user).Error
	require.NoError(t, err)
	return user
}

func createTestApp(t *testing.T, db *gorm.DB) *model.App {
	t.Helper()

	app := &model.App{
		AppID:      "test_app_id",
		AppSecret:  "test_app_secret",
		Name:       "Test App",
		Status:     1,
		RateLimit:  1000,
		AllowedIPs: "",
	}
	err := db.Create(app).Error
	require.NoError(t, err)
	return app
}

// ============================================================================
// JWTService 测试
// ============================================================================

func TestNewJWTService(t *testing.T) {
	config := DefaultJWTConfig()
	service := NewJWTService(config)

	assert.NotNil(t, service)
}

func TestDefaultJWTConfig(t *testing.T) {
	config := DefaultJWTConfig()

	assert.NotEmpty(t, config.SecretKey)
	assert.Equal(t, "bili-up", config.Issuer)
	assert.Equal(t, 24*time.Hour, config.AccessExpiry)
	assert.Equal(t, 7*24*time.Hour, config.RefreshExpiry)
}

func TestJWTService_GenerateAccessToken(t *testing.T) {
	service := setupTestJWTService()

	token, err := service.GenerateAccessToken(1, "testuser", "free", "test_app")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestJWTService_GenerateRefreshToken(t *testing.T) {
	service := setupTestJWTService()

	token, err := service.GenerateRefreshToken(1)

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestJWTService_ParseToken_Valid(t *testing.T) {
	service := setupTestJWTService()

	// 生成 Token
	token, err := service.GenerateAccessToken(123, "testuser", "pro", "test_app")
	require.NoError(t, err)

	// 解析 Token
	claims, err := service.ParseToken(token)

	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, uint(123), claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "pro", claims.Tier)
	assert.Equal(t, "test_app", claims.AppID)
}

func TestJWTService_ParseToken_Invalid(t *testing.T) {
	service := setupTestJWTService()

	// 无效 Token
	claims, err := service.ParseToken("invalid_token")

	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Equal(t, ErrInvalidToken, err)
}

func TestJWTService_ParseToken_Expired(t *testing.T) {
	// 创建一个短过期时间的服务
	config := JWTConfig{
		SecretKey:     "test_secret",
		Issuer:        "test",
		AccessExpiry:  1 * time.Millisecond, // 极短过期时间
		RefreshExpiry: 1 * time.Millisecond,
	}
	service := NewJWTService(config)

	// 生成 Token
	token, err := service.GenerateAccessToken(1, "testuser", "free", "app")
	require.NoError(t, err)

	// 等待过期
	time.Sleep(10 * time.Millisecond)

	// 解析应该失败
	claims, err := service.ParseToken(token)

	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Equal(t, ErrExpiredToken, err)
}

func TestJWTService_ParseToken_WrongSecret(t *testing.T) {
	// 使用不同密钥的两个服务
	service1 := NewJWTService(JWTConfig{
		SecretKey:    "secret_1",
		Issuer:       "test",
		AccessExpiry: 24 * time.Hour,
	})
	service2 := NewJWTService(JWTConfig{
		SecretKey:    "secret_2",
		Issuer:       "test",
		AccessExpiry: 24 * time.Hour,
	})

	// 用 service1 生成
	token, err := service1.GenerateAccessToken(1, "testuser", "free", "app")
	require.NoError(t, err)

	// 用 service2 解析
	claims, err := service2.ParseToken(token)

	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestJWTService_GenerateTokenPair(t *testing.T) {
	service := setupTestJWTService()

	tokenPair, err := service.GenerateTokenPair(1, "testuser", "free", "test_app")

	assert.NoError(t, err)
	assert.NotNil(t, tokenPair)
	assert.NotEmpty(t, tokenPair.AccessToken)
	assert.NotEmpty(t, tokenPair.RefreshToken)
	assert.Equal(t, "Bearer", tokenPair.TokenType)
	assert.True(t, tokenPair.ExpiresAt.After(time.Now()))
}

func TestHashToken(t *testing.T) {
	token1 := "token_12345"
	token2 := "token_67890"

	hash1 := HashToken(token1)
	hash2 := HashToken(token2)
	hash1Again := HashToken(token1)

	// 不同 token 应产生不同 hash
	assert.NotEqual(t, hash1, hash2)

	// 相同 token 应产生相同 hash
	assert.Equal(t, hash1, hash1Again)

	// Hash 应该是 64 个字符（256 位 SHA256）
	assert.Len(t, hash1, 64)
}

// ============================================================================
// AuthMiddleware - JWTAuth 测试
// ============================================================================

func TestAuthMiddleware_JWTAuth_NoHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	router := gin.New()
	router.Use(middleware.JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "未登录")
}

func TestAuthMiddleware_JWTAuth_InvalidFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	router := gin.New()
	router.Use(middleware.JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "InvalidFormat token123")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "无效的认证格式")
}

func TestAuthMiddleware_JWTAuth_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	router := gin.New()
	router.Use(middleware.JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid_token")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "无效的登录凭证")
}

func TestAuthMiddleware_JWTAuth_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	// 创建测试用户
	user := createTestUser(t, db, "user")

	// 生成有效 Token
	token, err := jwtService.GenerateAccessToken(user.ID, user.Username, "free", "test_app")
	require.NoError(t, err)

	var capturedUserID uint
	var capturedUsername string
	var capturedRole string

	router := gin.New()
	router.Use(middleware.JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		capturedUserID, _ = GetUserID(c)
		if v, ok := c.Get(ContextKeyUsername); ok {
			capturedUsername, _ = v.(string)
		}
		if v, ok := c.Get(ContextKeyUserRole); ok {
			capturedRole, _ = v.(string)
		}
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, user.ID, capturedUserID)
	assert.Equal(t, user.Username, capturedUsername)
	assert.Equal(t, "user", capturedRole)
}

func TestAuthMiddleware_JWTAuth_RevokedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	// 创建测试用户
	user := createTestUser(t, db, "user")

	// 生成 Token
	token, err := jwtService.GenerateAccessToken(user.ID, user.Username, "free", "test_app")
	require.NoError(t, err)

	// 将 Token 加入黑名单
	tokenHash := HashToken(token)
	revokedToken := &model.UserToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		IsRevoked: true,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	db.Create(revokedToken)

	router := gin.New()
	router.Use(middleware.JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "登录已失效")
}

// ============================================================================
// AuthMiddleware - OptionalJWTAuth 测试
// ============================================================================

func TestAuthMiddleware_OptionalJWTAuth_NoHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	var userIDExists bool

	router := gin.New()
	router.Use(middleware.OptionalJWTAuth())
	router.GET("/public", func(c *gin.Context) {
		_, userIDExists = c.Get(ContextKeyUserID)
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, userIDExists) // 没有 token，不应设置 userID
}

func TestAuthMiddleware_OptionalJWTAuth_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	user := createTestUser(t, db, "admin")
	token, _ := jwtService.GenerateAccessToken(user.ID, user.Username, "pro", "app")

	var capturedUserID uint
	var capturedRole string

	router := gin.New()
	router.Use(middleware.OptionalJWTAuth())
	router.GET("/public", func(c *gin.Context) {
		capturedUserID, _ = GetUserID(c)
		if v, ok := c.Get(ContextKeyUserRole); ok {
			capturedRole, _ = v.(string)
		}
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, user.ID, capturedUserID)
	assert.Equal(t, "admin", capturedRole)
}

func TestAuthMiddleware_OptionalJWTAuth_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	var userIDExists bool

	router := gin.New()
	router.Use(middleware.OptionalJWTAuth())
	router.GET("/public", func(c *gin.Context) {
		_, userIDExists = c.Get(ContextKeyUserID)
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public", nil)
	req.Header.Set("Authorization", "Bearer invalid_token")
	router.ServeHTTP(w, req)

	// 可选认证，无效 token 不应阻止请求
	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, userIDExists)
}

// ============================================================================
// AuthMiddleware - AppAuth 测试
// ============================================================================

func TestAuthMiddleware_AppAuth_NoAppId(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	router := gin.New()
	router.Use(middleware.AppAuth())
	router.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "缺少 App ID")
}

func TestAuthMiddleware_AppAuth_InvalidAppId(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	router := gin.New()
	router.Use(middleware.AppAuth())
	router.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api", nil)
	req.Header.Set("X-App-Id", "nonexistent_app")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "无效的 App ID")
}

func TestAuthMiddleware_AppAuth_ValidSecretAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	// 创建测试 App
	app := createTestApp(t, db)

	var capturedAppID string

	router := gin.New()
	router.Use(middleware.AppAuth())
	router.GET("/api", func(c *gin.Context) {
		capturedAppID = GetAppID(c)
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api", nil)
	req.Header.Set("X-App-Id", app.AppID)
	req.Header.Set("X-App-Secret", app.AppSecret)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, app.AppID, capturedAppID)
}

func TestAuthMiddleware_AppAuth_WrongSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	app := createTestApp(t, db)

	router := gin.New()
	router.Use(middleware.AppAuth())
	router.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api", nil)
	req.Header.Set("X-App-Id", app.AppID)
	req.Header.Set("X-App-Secret", "wrong_secret")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "无效的 App Secret")
}

func TestAuthMiddleware_AppAuth_DisabledApp(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	// 创建禁用的 App
	// 注意：GORM default 会覆盖零值，需要创建后更新 Status
	app := &model.App{
		AppID:     "disabled_app",
		AppSecret: "secret",
		Name:      "Disabled App",
	}
	err := db.Create(app).Error
	require.NoError(t, err)

	// 使用 Update 强制设置 Status=0（绕过 GORM default 行为）
	err = db.Model(app).Update("status", 0).Error
	require.NoError(t, err)

	// 验证 App 正确写入
	var verifyApp model.App
	db.Where("app_id = ?", "disabled_app").First(&verifyApp)
	t.Logf("Verified App Status in DB: %d", verifyApp.Status)
	require.Equal(t, 0, verifyApp.Status, "App Status should be 0 (disabled)")

	router := gin.New()
	router.Use(middleware.AppAuth())
	router.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api", nil)
	req.Header.Set("X-App-Id", app.AppID)
	req.Header.Set("X-App-Secret", app.AppSecret)
	router.ServeHTTP(w, req)

	// 检查实际响应
	t.Logf("Response Code: %d, Body: %s", w.Code, w.Body.String())
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "应用已禁用")
}

// ============================================================================
// Context Helper 测试
// ============================================================================

func TestGetUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// 未设置时
	id, ok := GetUserID(c)
	assert.False(t, ok)
	assert.Equal(t, uint(0), id)

	// 设置后
	c.Set(ContextKeyUserID, uint(123))
	id, ok = GetUserID(c)
	assert.True(t, ok)
	assert.Equal(t, uint(123), id)
}

func TestGetUserIDString(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// 未设置时
	idStr := GetUserIDString(c)
	assert.Empty(t, idStr)

	// 设置后
	c.Set(ContextKeyUserID, uint(456))
	idStr = GetUserIDString(c)
	assert.Equal(t, "456", idStr)
}

func TestGetAppID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// 未设置时
	appID := GetAppID(c)
	assert.Empty(t, appID)

	// 设置后
	c.Set(ContextKeyAppID, "my_app_id")
	appID = GetAppID(c)
	assert.Equal(t, "my_app_id", appID)
}

func TestGetUserTier(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// 未设置时应返回 "free"
	tier := GetUserTier(c)
	assert.Equal(t, "free", tier)

	// 设置后
	c.Set(ContextKeyUserTier, "pro")
	tier = GetUserTier(c)
	assert.Equal(t, "pro", tier)
}

// ============================================================================
// 用户角色测试
// ============================================================================

func TestAuthMiddleware_JWTAuth_AdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	// 创建管理员用户
	user := createTestUser(t, db, "admin")
	token, _ := jwtService.GenerateAccessToken(user.ID, user.Username, "enterprise", "app")

	var capturedRole string

	router := gin.New()
	router.Use(middleware.JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		if v, ok := c.Get(ContextKeyUserRole); ok {
			capturedRole, _ = v.(string)
		}
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "admin", capturedRole)
}

func TestAuthMiddleware_JWTAuth_DefaultRoleWhenEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	// 创建角色为空的用户
	user := createTestUser(t, db, "")
	token, _ := jwtService.GenerateAccessToken(user.ID, user.Username, "free", "app")

	var capturedRole string

	router := gin.New()
	router.Use(middleware.JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		if v, ok := c.Get(ContextKeyUserRole); ok {
			capturedRole, _ = v.(string)
		}
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user", capturedRole) // 空角色应默认为 "user"
}

// ============================================================================
// AdminMiddleware 测试
// ============================================================================

func TestRequireAdmin_NoRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequireAdmin())
	router.GET("/admin", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "未登录")
}

func TestRequireAdmin_UserRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 模拟普通用户 Context
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ContextKeyUserRole, "user")

	handlerCalled := false
	testHandler := func(c *gin.Context) {
		handlerCalled = true
		c.JSON(200, gin.H{"message": "should not reach here"})
	}

	// 创建包装 handler：先执行中间件，再执行 handler
	wrappedHandler := func(c *gin.Context) {
		RequireAdmin()(c)
		if !c.IsAborted() {
			testHandler(c)
		}
	}

	wrappedHandler(c)

	assert.False(t, handlerCalled, "Handler 不应该被调用")
	assert.Equal(t, http.StatusForbidden, c.Writer.Status())
}

func TestRequireAdmin_AdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 模拟管理员 Context
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ContextKeyUserRole, "admin")

	handlerCalled := false
	testHandler := func(c *gin.Context) {
		handlerCalled = true
		c.JSON(200, gin.H{"message": "admin access granted"})
	}

	// 创建包装 handler：先执行中间件，再执行 handler
	wrappedHandler := func(c *gin.Context) {
		RequireAdmin()(c)
		if !c.IsAborted() {
			testHandler(c)
		}
	}

	wrappedHandler(c)

	assert.True(t, handlerCalled, "Handler 应该被调用")
	assert.Equal(t, http.StatusOK, c.Writer.Status())
}

func TestRequireAdmin_WithJWTIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	authMiddleware := NewAuthMiddleware(db, jwtService)

	// 创建管理员用户
	admin := &model.User{
		Username:       "admin_user",
		Email:          fmt.Sprintf("admin_%d@example.com", time.Now().UnixNano()),
		Password:       "hashed_password",
		Role:           "admin",
		Status:         1,
		EmailVerified:  true,
		MembershipTier: "pro",
	}
	err := db.Create(admin).Error
	require.NoError(t, err)
	adminToken, _ := jwtService.GenerateAccessToken(admin.ID, admin.Username, "pro", "app")

	// 创建普通用户（添加微小延迟确保时间戳不同）
	time.Sleep(1 * time.Millisecond)
	user := &model.User{
		Username:       "normal_user",
		Email:          fmt.Sprintf("user_%d@example.com", time.Now().UnixNano()),
		Password:       "hashed_password",
		Role:           "user",
		Status:         1,
		EmailVerified:  true,
		MembershipTier: "free",
	}
	err = db.Create(user).Error
	require.NoError(t, err)
	userToken, _ := jwtService.GenerateAccessToken(user.ID, user.Username, "free", "app")

	router := gin.New()
	router.Use(authMiddleware.JWTAuth())
	router.Use(RequireAdmin())
	router.GET("/admin/config", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "admin config"})
	})

	// 管理员应该能访问
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/admin/config", nil)
	req1.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Contains(t, w1.Body.String(), "admin config")

	// 普通用户应该被拒绝
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/admin/config", nil)
	req2.Header.Set("Authorization", "Bearer "+userToken)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusForbidden, w2.Code)
	assert.Contains(t, w2.Body.String(), "权限不足")
}

func TestGetCurrentUserRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		setup    func(*gin.Context)
		expected string
	}{
		{
			name:     "未设置角色时返回默认值",
			setup:    func(c *gin.Context) {},
			expected: "user",
		},
		{
			name: "管理员角色",
			setup: func(c *gin.Context) {
				c.Set(ContextKeyUserRole, "admin")
			},
			expected: "admin",
		},
		{
			name: "普通用户角色",
			setup: func(c *gin.Context) {
				c.Set(ContextKeyUserRole, "user")
			},
			expected: "user",
		},
		{
			name: "自定义角色",
			setup: func(c *gin.Context) {
				c.Set(ContextKeyUserRole, "moderator")
			},
			expected: "moderator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			tt.setup(c)

			role := GetCurrentUserRole(c)
			assert.Equal(t, tt.expected, role)
		})
	}
}

func TestGetCurrentUserRole_TypeConversion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// 设置为非 string 类型（模拟类型错误）
	c.Set(ContextKeyUserRole, 123) // 设置为 int

	role := GetCurrentUserRole(c)
	assert.Equal(t, "user", role) // 应该返回默认值
}

func TestIsAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		setup    func(*gin.Context)
		expected bool
	}{
		{
			name:     "管理员返回 true",
			setup: func(c *gin.Context) {
				c.Set(ContextKeyUserRole, "admin")
			},
			expected: true,
		},
		{
			name:     "普通用户返回 false",
			setup: func(c *gin.Context) {
				c.Set(ContextKeyUserRole, "user")
			},
			expected: false,
		},
		{
			name:     "未登录返回 false",
			setup:    func(c *gin.Context) {},
			expected: false,
		},
		{
			name: "其他角色返回 false",
			setup: func(c *gin.Context) {
				c.Set(ContextKeyUserRole, "moderator")
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			tt.setup(c)

			result := IsAdmin(c)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadUserRole_LoggedInUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)

	// 创建管理员用户
	admin := createTestUser(t, db, "admin")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ContextKeyUserID, admin.ID)

	handlerCalled := false
	testHandler := func(c *gin.Context) {
		handlerCalled = true
		role, exists := c.Get(ContextKeyUserRole)
		assert.True(t, exists, "角色应该被设置")
		assert.Equal(t, "admin", role, "应该加载管理员角色")
	}

	// 创建包装 handler：先执行中间件，再执行 handler
	wrappedHandler := func(c *gin.Context) {
		LoadUserRole(db)(c)
		if !c.IsAborted() {
			testHandler(c)
		}
	}

	wrappedHandler(c)

	assert.True(t, handlerCalled, "Handler 应该被调用")
}

func TestLoadUserRole_NotLoggedIn(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	// 不设置 userID

	handlerCalled := false
	testHandler := func(c *gin.Context) {
		handlerCalled = true
		_, exists := c.Get(ContextKeyUserRole)
		assert.False(t, exists, "未登录时不应设置角色")
	}

	// 创建包装 handler：先执行中间件，再执行 handler
	wrappedHandler := func(c *gin.Context) {
		LoadUserRole(db)(c)
		if !c.IsAborted() {
			testHandler(c)
		}
	}

	wrappedHandler(c)

	assert.True(t, handlerCalled, "Handler 应该被调用")
}

func TestLoadUserRole_DatabaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)

	// 设置一个不存在的 userID
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ContextKeyUserID, uint(99999)) // 不存在的用户 ID

	handlerCalled := false
	testHandler := func(c *gin.Context) {
		handlerCalled = true
		role, exists := c.Get(ContextKeyUserRole)
		assert.True(t, exists, "应该设置默认角色")
		assert.Equal(t, "user", role, "查询失败时应该默认为 user")
	}

	// 创建包装 handler：先执行中间件，再执行 handler
	wrappedHandler := func(c *gin.Context) {
		LoadUserRole(db)(c)
		if !c.IsAborted() {
			testHandler(c)
		}
	}

	wrappedHandler(c)

	assert.True(t, handlerCalled)
}

func TestLoadUserRole_EmptyRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)

	// 创建角色为空的用户
	user := createTestUser(t, db, "")
	// 手动更新数据库，将角色设为空字符串
	db.Model(user).Update("role", "")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ContextKeyUserID, user.ID)

	handlerCalled := false
	testHandler := func(c *gin.Context) {
		handlerCalled = true
		role, exists := c.Get(ContextKeyUserRole)
		assert.True(t, exists)
		assert.Equal(t, "user", role, "空角色应该默认为 user")
	}

	// 创建包装 handler：先执行中间件，再执行 handler
	wrappedHandler := func(c *gin.Context) {
		LoadUserRole(db)(c)
		if !c.IsAborted() {
			testHandler(c)
		}
	}

	wrappedHandler(c)

	assert.True(t, handlerCalled)
}

func TestLoadUserRole_WithJWTIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	authMiddleware := NewAuthMiddleware(db, jwtService)

	// 创建管理员用户
	admin := createTestUser(t, db, "admin")
	adminToken, _ := jwtService.GenerateAccessToken(admin.ID, admin.Username, "pro", "app")

	router := gin.New()
	router.Use(authMiddleware.JWTAuth())
	router.Use(LoadUserRole(db)) // ← 测试 LoadUserRole
	router.GET("/profile", func(c *gin.Context) {
		role := GetCurrentUserRole(c)
		c.JSON(200, gin.H{"role": role})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/profile", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"role":"admin"`)
}

func TestLoadUserRole_UserRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	authMiddleware := NewAuthMiddleware(db, jwtService)

	// 创建普通用户
	user := createTestUser(t, db, "user")
	userToken, _ := jwtService.GenerateAccessToken(user.ID, user.Username, "free", "app")

	router := gin.New()
	router.Use(authMiddleware.JWTAuth())
	router.Use(LoadUserRole(db))
	router.GET("/profile", func(c *gin.Context) {
		role := GetCurrentUserRole(c)
		isAdmin := IsAdmin(c)
		c.JSON(200, gin.H{
			"role":     role,
			"is_admin": isAdmin,
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/profile", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"role":"user"`)
	assert.Contains(t, w.Body.String(), `"is_admin":false`)
}

// ============================================================================
// App 签名验证测试
// ============================================================================

func TestGenerateAppID(t *testing.T) {
	appID1 := GenerateAppID()
	appID2 := GenerateAppID()

	// 验证格式
	assert.True(t, len(appID1) > 4, "AppID 长度应该大于 4")
	assert.True(t, len(appID2) > 4, "AppID 长度应该大于 4")
	assert.Contains(t, appID1, "app_", "应该以 'app_' 开头")
	assert.Contains(t, appID2, "app_", "应该以 'app_' 开头")

	// 验证唯一性
	assert.NotEqual(t, appID1, appID2, "每次生成的 AppID 应该不同")
}

func TestGenerateAppSecret(t *testing.T) {
	secret1 := GenerateAppSecret()
	secret2 := GenerateAppSecret()

	// 验证格式（64个十六进制字符 = 256位）
	assert.Len(t, secret1, 64, "Secret 应该是 64 个字符")
	assert.Len(t, secret2, 64, "Secret 应该是 64 个字符")

	// 验证是十六进制
	_, err := hex.DecodeString(secret1)
	assert.NoError(t, err, "应该是有效的十六进制字符串")

	// 验证唯一性
	assert.NotEqual(t, secret1, secret2, "每次生成的 Secret 应该不同")
}

func TestGenerateSignature(t *testing.T) {
	appID := "test_app_123"
	appSecret := "test_secret_456"
	timestamp := "1704067200"
	nonce := "random_nonce_789"

	signature := GenerateSignature(appID, appSecret, timestamp, nonce)

	// 验证格式（SHA256 = 64 个十六进制字符）
	assert.Len(t, signature, 64, "签名应该是 64 个字符")

	// 验证是十六进制
	_, err := hex.DecodeString(signature)
	assert.NoError(t, err, "应该是有效的十六进制字符串")

	// 验证确定性（相同输入产生相同输出）
	signature2 := GenerateSignature(appID, appSecret, timestamp, nonce)
	assert.Equal(t, signature, signature2, "相同输入应该产生相同签名")

	// 验证不同输入产生不同签名
	signature3 := GenerateSignature(appID, appSecret, timestamp, "different_nonce")
	assert.NotEqual(t, signature, signature3, "不同 nonce 应该产生不同签名")
}

func TestVerifySignature(t *testing.T) {
	appID := "test_app"
	appSecret := "test_secret"
	timestamp := "1704067200"
	nonce := "test_nonce"

	// 生成正确签名
	correctSignature := GenerateSignature(appID, appSecret, timestamp, nonce)

	tests := []struct {
		name      string
		appID     string
		appSecret string
		timestamp string
		nonce     string
		signature string
		expected  bool
	}{
		{
			name:      "正确签名",
			appID:     appID,
			appSecret: appSecret,
			timestamp: timestamp,
			nonce:     nonce,
			signature: correctSignature,
			expected:  true,
		},
		{
			name:      "错误签名",
			appID:     appID,
			appSecret: appSecret,
			timestamp: timestamp,
			nonce:     nonce,
			signature: "wrong_signature_1234567890abcdef",
			expected:  false,
		},
		{
			name:      "不同 AppID",
			appID:     "different_app",
			appSecret: appSecret,
			timestamp: timestamp,
			nonce:     nonce,
			signature: correctSignature,
			expected:  false,
		},
		{
			name:      "不同 Secret",
			appID:     appID,
			appSecret: "different_secret",
			timestamp: timestamp,
			nonce:     nonce,
			signature: correctSignature,
			expected:  false,
		},
		{
			name:      "不同 Timestamp",
			appID:     appID,
			appSecret: appSecret,
			timestamp: "1704067201",
			nonce:     nonce,
			signature: correctSignature,
			expected:  false,
		},
		{
			name:      "不同 Nonce",
			appID:     appID,
			appSecret: appSecret,
			timestamp: timestamp,
			nonce:     "different_nonce",
			signature: correctSignature,
			expected:  false,
		},
		{
			name:      "空签名",
			appID:     appID,
			appSecret: appSecret,
			timestamp: timestamp,
			nonce:     nonce,
			signature: "",
			expected:  false,
		},
		{
			name:      "签名长度错误",
			appID:     appID,
			appSecret: appSecret,
			timestamp: timestamp,
			nonce:     nonce,
			signature: "short",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifySignature(tt.appID, tt.appSecret, tt.timestamp, tt.nonce, tt.signature)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateTimestamp(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		timestamp string
		maxAge    time.Duration
		expected  bool
	}{
		{
			name:      "当前时间 - Unix 格式",
			timestamp: fmt.Sprintf("%d", now.Unix()),
			maxAge:    5 * time.Minute,
			expected:  true,
		},
		{
			name:      "1分钟前 - Unix 格式",
			timestamp: fmt.Sprintf("%d", now.Add(-1*time.Minute).Unix()),
			maxAge:    5 * time.Minute,
			expected:  true,
		},
		{
			name:      "4分钟前 - Unix 格式",
			timestamp: fmt.Sprintf("%d", now.Add(-4*time.Minute).Unix()),
			maxAge:    5 * time.Minute,
			expected:  true,
		},
		{
			name:      "6分钟前 - 超过 maxAge",
			timestamp: fmt.Sprintf("%d", now.Add(-6*time.Minute).Unix()),
			maxAge:    5 * time.Minute,
			expected:  false,
		},
		{
			name:      "未来时间 - Unix 格式",
			timestamp: fmt.Sprintf("%d", now.Add(1*time.Minute).Unix()),
			maxAge:    5 * time.Minute,
			expected:  true, // 在 maxAge 范围内
		},
		{
			name:      "10分钟后 - 超过 maxAge",
			timestamp: fmt.Sprintf("%d", now.Add(10*time.Minute).Unix()),
			maxAge:    5 * time.Minute,
			expected:  false,
		},
		{
			name:      "RFC3339 格式 - 当前时间",
			timestamp: now.Format(time.RFC3339),
			maxAge:    5 * time.Minute,
			expected:  true,
		},
		{
			name:      "RFC3339 格式 - 1分钟前",
			timestamp: now.Add(-1 * time.Minute).Format(time.RFC3339),
			maxAge:    5 * time.Minute,
			expected:  true,
		},
		{
			name:      "RFC3339 格式 - 6分钟前",
			timestamp: now.Add(-6 * time.Minute).Format(time.RFC3339),
			maxAge:    5 * time.Minute,
			expected:  false,
		},
		{
			name:      "无效格式 - 字符串",
			timestamp: "invalid_timestamp",
			maxAge:    5 * time.Minute,
			expected:  false,
		},
		{
			name:      "无效格式 - 空字符串",
			timestamp: "",
			maxAge:    5 * time.Minute,
			expected:  false,
		},
		{
			name:      "负数时间戳",
			timestamp: "-1000000",
			maxAge:    5 * time.Minute,
			expected:  false,
		},
		{
			name:      "零时间戳",
			timestamp: "0",
			maxAge:    5 * time.Minute,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateTimestamp(tt.timestamp, tt.maxAge)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppAuth_SignatureVerificationIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	// 创建测试 App
	app := createTestApp(t, db)

	// 生成签名参数
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("nonce_%d", time.Now().UnixNano())
	signature := GenerateSignature(app.AppID, app.AppSecret, timestamp, nonce)

	router := gin.New()
	router.Use(middleware.AppAuth())
	router.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api", nil)
	req.Header.Set("X-App-Id", app.AppID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", signature)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestAppAuth_SignatureVerification_WrongSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	app := createTestApp(t, db)

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := "test_nonce"
	wrongSignature := "wrong_signature_1234567890abcdef"

	router := gin.New()
	router.Use(middleware.AppAuth())
	router.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api", nil)
	req.Header.Set("X-App-Id", app.AppID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", wrongSignature)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "签名验证失败")
}

func TestAppAuth_SignatureVerification_ExpiredTimestamp(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	app := createTestApp(t, db)

	// 生成10分钟前的时间戳
	oldTimestamp := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	nonce := "test_nonce"
	signature := GenerateSignature(app.AppID, app.AppSecret, oldTimestamp, nonce)

	router := gin.New()
	router.Use(middleware.AppAuth())
	router.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api", nil)
	req.Header.Set("X-App-Id", app.AppID)
	req.Header.Set("X-Timestamp", oldTimestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", signature)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "请求已过期")
}

func TestAppAuth_SignatureVerification_MissingParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	jwtService := setupTestJWTService()
	middleware := NewAuthMiddleware(db, jwtService)

	app := createTestApp(t, db)

	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	router := gin.New()
	router.Use(middleware.AppAuth())
	router.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	tests := []struct {
		name         string
		setupHeaders func(*http.Request)
		expectedCode int
		containsMsg  string
	}{
		{
			name: "缺少 Timestamp",
			setupHeaders: func(req *http.Request) {
				req.Header.Set("X-App-Id", app.AppID)
				req.Header.Set("X-Nonce", "nonce")
				req.Header.Set("X-Signature", "signature")
			},
			expectedCode: http.StatusUnauthorized,
			containsMsg:  "缺少认证参数",
		},
		{
			name: "缺少 Nonce",
			setupHeaders: func(req *http.Request) {
				req.Header.Set("X-App-Id", app.AppID)
				req.Header.Set("X-Timestamp", timestamp)
				req.Header.Set("X-Signature", "signature")
			},
			expectedCode: http.StatusUnauthorized,
			containsMsg:  "缺少认证参数",
		},
		{
			name: "缺少 Signature",
			setupHeaders: func(req *http.Request) {
				req.Header.Set("X-App-Id", app.AppID)
				req.Header.Set("X-Timestamp", timestamp)
				req.Header.Set("X-Nonce", "nonce")
			},
			expectedCode: http.StatusUnauthorized,
			containsMsg:  "缺少认证参数",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api", nil)
			tt.setupHeaders(req)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
			assert.Contains(t, w.Body.String(), tt.containsMsg)
		})
	}
}

// ============================================================================
// 基准测试
// ============================================================================

func BenchmarkJWTService_GenerateAccessToken(b *testing.B) {
	service := NewJWTService(DefaultJWTConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GenerateAccessToken(1, "testuser", "free", "app")
	}
}

func BenchmarkJWTService_ParseToken(b *testing.B) {
	service := NewJWTService(DefaultJWTConfig())
	token, _ := service.GenerateAccessToken(1, "testuser", "free", "app")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.ParseToken(token)
	}
}

func BenchmarkHashToken(b *testing.B) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxfQ.test"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HashToken(token)
	}
}

func BenchmarkGetCurrentUserRole(b *testing.B) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ContextKeyUserRole, "admin")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetCurrentUserRole(c)
	}
}

func BenchmarkIsAdmin(b *testing.B) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ContextKeyUserRole, "admin")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsAdmin(c)
	}
}

func BenchmarkGenerateSignature(b *testing.B) {
	appID := "test_app"
	appSecret := "test_secret"
	timestamp := "1704067200"
	nonce := "test_nonce"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateSignature(appID, appSecret, timestamp, nonce)
	}
}

func BenchmarkVerifySignature(b *testing.B) {
	appID := "test_app"
	appSecret := "test_secret"
	timestamp := "1704067200"
	nonce := "test_nonce"
	signature := GenerateSignature(appID, appSecret, timestamp, nonce)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VerifySignature(appID, appSecret, timestamp, nonce, signature)
	}
}
