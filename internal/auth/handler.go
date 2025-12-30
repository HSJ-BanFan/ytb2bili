package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/storage"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// GetBiliStore 获取B站登录存储
func GetBiliStore() *storage.LoginStore {
	return storage.GetDefaultStore()
}

// AuthHandler 认证处理器
type AuthHandler struct {
	db           *gorm.DB
	jwtService   *JWTService
	middleware   *AuthMiddleware
	emailService *services.EmailService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(db *gorm.DB, jwtService *JWTService, emailService *services.EmailService) *AuthHandler {
	return &AuthHandler{
		db:           db,
		jwtService:   jwtService,
		middleware:   NewAuthMiddleware(db, jwtService),
		emailService: emailService,
	}
}

// RegisterRoutes 注册路由
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	// 用户认证 (JWT) - 公开路由
	userAuth := rg.Group("/user")
	{
		userAuth.POST("/send-verification-code", h.SendVerificationCode)
		userAuth.POST("/register", h.Register)
		userAuth.POST("/login", h.Login)
		userAuth.POST("/login-with-code", h.LoginWithCode) // 验证码登录
		userAuth.POST("/refresh", h.RefreshToken)
		userAuth.POST("/forgot-password", h.ForgotPassword) // 发送重置密码验证码
		userAuth.POST("/reset-password", h.ResetPassword)   // 验证码重置密码
		userAuth.GET("/bili-userinfo", h.GetBiliUserInfo)   // 获取B站用户信息（用于绑定）
	}

	// 需要认证的路由
	userAuthProtected := rg.Group("/user")
	userAuthProtected.Use(h.middleware.JWTAuth())
	{
		userAuthProtected.POST("/logout", h.Logout)
		userAuthProtected.GET("/me", h.GetCurrentUser)
	}

	// App 管理
	apps := rg.Group("/apps")
	{
		apps.POST("", h.CreateApp)
		apps.GET("", h.ListApps)
		apps.GET("/:id", h.GetApp)
		apps.PUT("/:id", h.UpdateApp)
		apps.DELETE("/:id", h.DeleteApp)
		apps.POST("/:id/regenerate-secret", h.RegenerateAppSecret)
	}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username         string `json:"username" binding:"required,min=3,max=50"`
	Email            string `json:"email" binding:"required,email"`
	Password         string `json:"password" binding:"required,min=6"`
	VerificationCode string `json:"verification_code" binding:"required,len=6"`
}

// Register 用户注册
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 检查邮箱是否已存在
	var existingUser model.User

	// 检查邮箱是否已存在
	if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": "邮箱已被注册",
		})
		return
	}

	// 验证邮箱验证码（使用事务避免竞态条件）
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var verification model.EmailVerification
	err := tx.Where("email = ? AND code = ? AND type = ? AND used = false",
		req.Email, req.VerificationCode, "register").
		Order("created_at DESC").
		First(&verification).Error

	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "验证码错误或已过期",
		})
		return
	}

	// 检查验证码是否过期
	if time.Now().After(verification.ExpiresAt) {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "验证码已过期，请重新获取",
		})
		return
	}

	// 检查验证码尝试次数
	if verification.AttemptCount >= 3 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "验证码尝试次数过多，请重新获取",
		})
		return
	}

	// 标记验证码为已使用
	tx.Model(&verification).Updates(map[string]interface{}{
		"used":            true,
		"attempt_count":   verification.AttemptCount + 1,
		"last_attempt_at": time.Now(),
	})

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "密码加密失败",
		})
		return
	}

	// 创建用户
	now := time.Now()
	user := model.User{
		Username:        req.Username,
		Email:           req.Email,
		Password:        string(hashedPassword),
		Status:          1,
		MembershipTier:  "free",
		EmailVerified:   true, // 通过验证码注册，自动标记为已验证
		EmailVerifiedAt: &now, // 记录验证时间
	}

	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		fmt.Printf("❌ 创建用户失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建用户失败: " + err.Error(),
		})
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "注册失败，请重试",
		})
		return
	}

	// 生成 Token
	appID := GetAppID(c)
	tokenPair, err := h.jwtService.GenerateTokenPair(user.ID, user.Username, user.MembershipTier, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成 Token 失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "注册成功",
		"data": gin.H{
			"user": gin.H{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
				"tier":     user.MembershipTier,
			},
			"token": tokenPair,
		},
	})
}

// SendVerificationCodeRequest 发送验证码请求
type SendVerificationCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
	Type  string `json:"type" binding:"required,oneof=register login reset_password"`
}

// SendVerificationCode 发送验证码
func (h *AuthHandler) SendVerificationCode(c *gin.Context) {
	var req SendVerificationCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 限流：检查 1 分钟内是否已发送验证码
	var recentVerification model.EmailVerification
	err := h.db.Where("email = ? AND type = ? AND created_at > ?",
		req.Email, req.Type, time.Now().Add(-1*time.Minute)).
		Order("created_at DESC").
		First(&recentVerification).Error

	if err == nil {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    429,
			"message": "验证码发送过于频繁，请 1 分钟后再试",
		})
		return
	}

	// 如果是注册类型，检查邮箱是否已被注册
	if req.Type == "register" {
		var existingUser model.User
		if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": "该邮箱已被注册",
			})
			return
		}
	}

	// 生成验证码
	code, err := h.emailService.GenerateCode()
	if err != nil {
		fmt.Printf("❌ 生成验证码失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成验证码失败",
		})
		return
	}

	// 发送邮件（如果服务启用）
	if h.emailService.IsEnabled() {
		if err := h.emailService.SendVerificationEmail(req.Email, code, req.Type); err != nil {
			fmt.Printf("❌ 发送邮件失败: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "发送邮件失败: " + err.Error(),
			})
			return
		}
		fmt.Printf("✅ 验证码邮件已发送到: %s\n", req.Email)
	} else {
		// 开发环境：在日志中打印验证码
		fmt.Printf("🔔 [开发模式] 验证码: %s (邮箱: %s, 类型: %s)\n", code, req.Email, req.Type)
	}

	// 清理旧验证码：将同一邮箱和类型的旧验证码标记为已使用
	h.db.Model(&model.EmailVerification{}).
		Where("email = ? AND type = ? AND used = false", req.Email, req.Type).
		Update("used", true)

	// 保存验证码到数据库
	verification := model.EmailVerification{
		Email:     req.Email,
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute), // 10 分钟有效期
		Type:      req.Type,
		Used:      false,
	}

	if err := h.db.Create(&verification).Error; err != nil {
		fmt.Printf("❌ 保存验证码失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存验证码失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "验证码已发送",
		"data": gin.H{
			"email":      req.Email,
			"expires_in": 600, // 10 分钟，单位秒
			"type":       req.Type,
		},
	})
}

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
		})
		return
	}

	// 查找用户 (邮箱登录)
	var user model.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "邮箱或密码错误",
		})
		return
	}

	// 检查用户状态
	if user.Status != 1 {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "账号已被禁用",
		})
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "邮箱或密码错误",
		})
		return
	}

	// 更新最后登录时间
	now := time.Now()
	h.db.Model(&user).Update("last_login_at", now)

	// 生成 Token
	appID := GetAppID(c)
	tokenPair, err := h.jwtService.GenerateTokenPair(user.ID, user.Username, user.MembershipTier, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成 Token 失败",
		})
		return
	}

	// 保存 Token 记录
	userToken := model.UserToken{
		UserID:    user.ID,
		TokenHash: HashToken(tokenPair.AccessToken),
		ExpiresAt: tokenPair.ExpiresAt,
		IP:        c.ClientIP(),
	}
	h.db.Create(&userToken)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "登录成功",
		"data": gin.H{
			"user": gin.H{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
				"tier":     user.MembershipTier,
				"role":     user.Role,
				"avatar":   user.Avatar,
			},
			"token": tokenPair,
		},
	})
}

// RefreshToken 刷新 Token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
		})
		return
	}

	// 解析 Refresh Token (简单验证)
	_, err := h.jwtService.ParseToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Refresh Token 无效或已过期",
		})
		return
	}

	// 从当前 Token 获取用户信息
	userID, ok := GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "请先登录",
		})
		return
	}

	// 查询用户
	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "用户不存在",
		})
		return
	}

	// 生成新 Token
	appID := GetAppID(c)
	tokenPair, err := h.jwtService.GenerateTokenPair(user.ID, user.Username, user.MembershipTier, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成 Token 失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "刷新成功",
		"data": gin.H{
			"token": tokenPair,
		},
	})
}

// Logout 退出登录
func (h *AuthHandler) Logout(c *gin.Context) {
	// 获取当前 Token
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			tokenHash := HashToken(parts[1])
			// 将 Token 加入黑名单
			h.db.Model(&model.UserToken{}).Where("token_hash = ?", tokenHash).Update("is_revoked", true)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "退出成功",
	})
}

// GetCurrentUser 获取当前用户信息
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, ok := GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "用户不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":                user.ID,
			"username":          user.Username,
			"email":             user.Email,
			"avatar":            user.Avatar,
			"tier":              user.MembershipTier,
			"role":              user.Role,
			"membership_expire": user.MembershipExpire,
			"created_at":        user.CreatedAt,
		},
	})
}

// GetBiliUserInfo 获取B站用户信息（不创建User，不返回JWT）
// 用于扫码绑定流程：用户扫码后，前端调用此接口获取B站用户信息，然后调用绑定接口
func (h *AuthHandler) GetBiliUserInfo(c *gin.Context) {
	// 1. 从 storage 获取 B站登录信息
	store := GetBiliStore()
	if store == nil || !store.IsValid() {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "B站未登录，请先扫码",
		})
		return
	}

	// 2. 获取B站用户信息
	biliUserInfo, err := store.GetUserInfo()
	if err != nil || biliUserInfo == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "无法获取B站用户信息",
		})
		return
	}

	// 3. 获取完整的登录信息（包括 cookies 和 tokens）
	loginInfo, err := store.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取B站登录信息失败: " + err.Error(),
		})
		return
	}

	// 4. 序列化 LoginInfo 为 JSON 字符串（用于存储到数据库）
	loginInfoJSON, err := json.Marshal(loginInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "序列化登录信息失败: " + err.Error(),
		})
		return
	}

	// 5. 只返回B站用户信息，不创建User，不返回JWT
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "获取B站用户信息成功",
		"data": gin.H{
			"bili_mid":      fmt.Sprintf("%d", biliUserInfo.Mid),
			"bili_name":     biliUserInfo.Name,
			"bili_face":     biliUserInfo.Face,
			"login_info":    string(loginInfoJSON), // 完整的登录信息
			"access_token":  loginInfo.TokenInfo.AccessToken,
			"refresh_token": loginInfo.TokenInfo.RefreshToken,
		},
	})
}

// ========== App 管理 ==========

// CreateAppRequest 创建 App 请求
type CreateAppRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	AllowedIPs  string `json:"allowed_ips"`
	RateLimit   int    `json:"rate_limit"`
}

// CreateApp 创建 App
func (h *AuthHandler) CreateApp(c *gin.Context) {
	var req CreateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
		})
		return
	}

	rateLimit := req.RateLimit
	if rateLimit <= 0 {
		rateLimit = 1000
	}

	app := model.App{
		AppID:       GenerateAppID(),
		AppSecret:   GenerateAppSecret(),
		Name:        req.Name,
		Description: req.Description,
		AllowedIPs:  req.AllowedIPs,
		RateLimit:   rateLimit,
		Status:      1,
	}

	// 如果有登录用户，设置为所有者
	if userID, ok := GetUserID(c); ok {
		app.OwnerID = &userID
	}

	if err := h.db.Create(&app).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data": gin.H{
			"id":         app.ID,
			"app_id":     app.AppID,
			"app_secret": app.AppSecret, // 只在创建时返回
			"name":       app.Name,
		},
	})
}

// ListApps 列出 Apps
func (h *AuthHandler) ListApps(c *gin.Context) {
	var apps []model.App
	query := h.db.Model(&model.App{})

	// 如果有登录用户，只显示自己的 App
	if userID, ok := GetUserID(c); ok {
		query = query.Where("owner_id = ?", userID)
	}

	if err := query.Find(&apps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    apps,
	})
}

// GetApp 获取 App 详情
func (h *AuthHandler) GetApp(c *gin.Context) {
	id := c.Param("id")

	var app model.App
	if err := h.db.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "App 不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    app,
	})
}

// UpdateApp 更新 App
func (h *AuthHandler) UpdateApp(c *gin.Context) {
	id := c.Param("id")

	var app model.App
	if err := h.db.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "App 不存在",
		})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		AllowedIPs  string `json:"allowed_ips"`
		RateLimit   int    `json:"rate_limit"`
		Status      *int   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
		})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.AllowedIPs != "" {
		updates["allowed_ips"] = req.AllowedIPs
	}
	if req.RateLimit > 0 {
		updates["rate_limit"] = req.RateLimit
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if err := h.db.Model(&app).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// DeleteApp 删除 App
func (h *AuthHandler) DeleteApp(c *gin.Context) {
	id := c.Param("id")

	if err := h.db.Delete(&model.App{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// RegenerateAppSecret 重新生成 App Secret
func (h *AuthHandler) RegenerateAppSecret(c *gin.Context) {
	id := c.Param("id")

	var app model.App
	if err := h.db.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "App 不存在",
		})
		return
	}

	newSecret := GenerateAppSecret()
	if err := h.db.Model(&app).Update("app_secret", newSecret).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "重新生成失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "重新生成成功",
		"data": gin.H{
			"app_secret": newSecret,
		},
	})
}

// ========== 密码重置功能 ==========

// ForgotPasswordRequest 忘记密码请求
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ForgotPassword 发送重置密码验证码
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 检查邮箱是否存在
	var user model.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// 为了安全，不暴露邮箱是否存在，统一返回成功
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "如果该邮箱已注册，验证码将发送至您的邮箱",
		})
		return
	}

	// 使用 SendVerificationCode 发送验证码
	// 创建一个临时的 verification code request
	codeReq := SendVerificationCodeRequest{
		Email: req.Email,
		Type:  "reset_password",
	}

	// 生成验证码
	code, err := h.emailService.GenerateCode()
	if err != nil {
		fmt.Printf("❌ 生成验证码失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成验证码失败",
		})
		return
	}

	// 限流检查
	var recentVerification model.EmailVerification
	err = h.db.Where("email = ? AND type = ? AND created_at > ?",
		codeReq.Email, codeReq.Type, time.Now().Add(-1*time.Minute)).
		Order("created_at DESC").
		First(&recentVerification).Error

	if err == nil {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    429,
			"message": "验证码发送过于频繁，请 1 分钟后再试",
		})
		return
	}

	// 发送邮件
	if h.emailService.IsEnabled() {
		if err := h.emailService.SendVerificationEmail(codeReq.Email, code, codeReq.Type); err != nil {
			fmt.Printf("❌ 发送邮件失败: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "发送邮件失败: " + err.Error(),
			})
			return
		}
		fmt.Printf("✅ 密码重置验证码邮件已发送到: %s\n", codeReq.Email)
	} else {
		fmt.Printf("🔔 [开发模式] 密码重置验证码: %s (邮箱: %s)\n", code, codeReq.Email)
	}

	// 清理旧验证码
	h.db.Model(&model.EmailVerification{}).
		Where("email = ? AND type = ? AND used = false", codeReq.Email, codeReq.Type).
		Update("used", true)

	// 保存验证码
	verification := model.EmailVerification{
		Email:     codeReq.Email,
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Type:      codeReq.Type,
		Used:      false,
	}

	if err := h.db.Create(&verification).Error; err != nil {
		fmt.Printf("❌ 保存验证码失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存验证码失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "如果该邮箱已注册，验证码将发送至您的邮箱",
		"data": gin.H{
			"email":      codeReq.Email,
			"expires_in": 600,
		},
	})
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Email            string `json:"email" binding:"required,email"`
	VerificationCode string `json:"verification_code" binding:"required,len=6"`
	NewPassword      string `json:"new_password" binding:"required,min=6"`
}

// ResetPassword 使用验证码重置密码
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 使用事务确保数据一致性
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 查找用户
	var user model.User
	if err := tx.Where("email = ?", req.Email).First(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "用户不存在",
		})
		return
	}

	// 验证验证码
	var verification model.EmailVerification
	err := tx.Where("email = ? AND code = ? AND type = ? AND used = false",
		req.Email, req.VerificationCode, "reset_password").
		Order("created_at DESC").
		First(&verification).Error

	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "验证码错误或已过期",
		})
		return
	}

	// 检查验证码是否过期
	if time.Now().After(verification.ExpiresAt) {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "验证码已过期，请重新获取",
		})
		return
	}

	// 检查验证码尝试次数
	if verification.AttemptCount >= 3 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "验证码尝试次数过多，请重新获取",
		})
		return
	}

	// 标记验证码为已使用
	tx.Model(&verification).Updates(map[string]interface{}{
		"used":            true,
		"attempt_count":   verification.AttemptCount + 1,
		"last_attempt_at": time.Now(),
	})

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "密码加密失败",
		})
		return
	}

	// 更新密码
	if err := tx.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		tx.Rollback()
		fmt.Printf("❌ 更新密码失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新密码失败",
		})
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "重置密码失败，请重试",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "密码重置成功，请使用新密码登录",
	})
}

// ========== 验证码登录功能 ==========

// LoginWithCodeRequest 验证码登录请求
type LoginWithCodeRequest struct {
	Email            string `json:"email" binding:"required,email"`
	VerificationCode string `json:"verification_code" binding:"required,len=6"`
}

// LoginWithCode 使用邮箱验证码登录（免密码登录）
func (h *AuthHandler) LoginWithCode(c *gin.Context) {
	var req LoginWithCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 使用事务确保数据一致性
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 查找用户
	var user model.User
	if err := tx.Where("email = ?", req.Email).First(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "邮箱未注册",
		})
		return
	}

	// 检查用户状态
	if user.Status != 1 {
		tx.Rollback()
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "账号已被禁用",
		})
		return
	}

	// 验证验证码
	var verification model.EmailVerification
	err := tx.Where("email = ? AND code = ? AND type = ? AND used = false",
		req.Email, req.VerificationCode, "login").
		Order("created_at DESC").
		First(&verification).Error

	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "验证码错误或已过期",
		})
		return
	}

	// 检查验证码是否过期
	if time.Now().After(verification.ExpiresAt) {
		tx.Rollback()
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "验证码已过期，请重新获取",
		})
		return
	}

	// 检查验证码尝试次数
	if verification.AttemptCount >= 3 {
		tx.Rollback()
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "验证码尝试次数过多，请重新获取",
		})
		return
	}

	// 标记验证码为已使用
	tx.Model(&verification).Updates(map[string]interface{}{
		"used":            true,
		"attempt_count":   verification.AttemptCount + 1,
		"last_attempt_at": time.Now(),
	})

	// 更新最后登录时间
	now := time.Now()
	tx.Model(&user).Update("last_login_at", now)

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "登录失败，请重试",
		})
		return
	}

	// 生成 Token
	appID := GetAppID(c)
	tokenPair, err := h.jwtService.GenerateTokenPair(user.ID, user.Username, user.MembershipTier, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成 Token 失败",
		})
		return
	}

	// 保存 Token 记录
	userToken := model.UserToken{
		UserID:    user.ID,
		TokenHash: HashToken(tokenPair.AccessToken),
		ExpiresAt: tokenPair.ExpiresAt,
		IP:        c.ClientIP(),
	}
	h.db.Create(&userToken)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "登录成功",
		"data": gin.H{
			"user": gin.H{
				"id":                user.ID,
				"username":          user.Username,
				"email":             user.Email,
				"tier":              user.MembershipTier,
				"role":              user.Role,
				"avatar":            user.Avatar,
				"email_verified":    user.EmailVerified,
				"email_verified_at": user.EmailVerifiedAt,
			},
			"token": tokenPair,
		},
	})
}
