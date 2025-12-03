package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

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
	db         *gorm.DB
	jwtService *JWTService
	middleware *AuthMiddleware
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(db *gorm.DB, jwtService *JWTService) *AuthHandler {
	return &AuthHandler{
		db:         db,
		jwtService: jwtService,
		middleware: NewAuthMiddleware(db, jwtService),
	}
}

// RegisterRoutes 注册路由
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	// 用户认证 (JWT) - 公开路由
	userAuth := rg.Group("/user")
	{
		userAuth.POST("/register", h.Register)
		userAuth.POST("/login", h.Login)
		userAuth.POST("/refresh", h.RefreshToken)
		userAuth.GET("/exchange-token", h.ExchangeBiliSessionForJWT) // B站Session换JWT
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
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
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

	// 检查用户名是否已存在
	var existingUser model.User
	if err := h.db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": "用户名已存在",
		})
		return
	}

	// 检查邮箱是否已存在
	if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": "邮箱已被注册",
		})
		return
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "密码加密失败",
		})
		return
	}

	// 创建用户
	user := model.User{
		Username:       req.Username,
		Email:          req.Email,
		Password:       string(hashedPassword),
		Status:         1,
		MembershipTier: "free",
	}

	if err := h.db.Create(&user).Error; err != nil {
		fmt.Printf("❌ 创建用户失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建用户失败: " + err.Error(),
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

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
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

	// 查找用户 (支持用户名或邮箱登录)
	var user model.User
	if err := h.db.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "用户名或密码错误",
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
			"message": "用户名或密码错误",
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
			"membership_expire": user.MembershipExpire,
			"created_at":        user.CreatedAt,
		},
	})
}

// ExchangeBiliSessionForJWT B站Session换取JWT Token
// 当用户通过B站扫码登录后，调用此接口获取JWT Token
func (h *AuthHandler) ExchangeBiliSessionForJWT(c *gin.Context) {
	// 1. 从 storage 获取 B站登录信息
	store := GetBiliStore()
	if store == nil || !store.IsValid() {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "B站未登录，请先扫码登录",
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

	// 3. 查找或创建对应的 JWT 用户
	biliMid := fmt.Sprintf("%d", biliUserInfo.Mid)
	biliUsername := biliUserInfo.Name
	if biliUsername == "" {
		biliUsername = fmt.Sprintf("bili_%s", biliMid)
	}

	var user model.User
	// 先按 bili_mid 查找
	err = h.db.Where("bili_mid = ?", biliMid).First(&user).Error
	if err != nil {
		// 用户不存在，创建新用户
		user = model.User{
			Username:       biliUsername,
			Email:          fmt.Sprintf("%s@bilibili.local", biliMid), // 虚拟邮箱
			Password:       "",                                        // B站用户无需密码
			Status:         1,
			MembershipTier: "free",
			Avatar:         biliUserInfo.Face,
			BiliMid:        biliMid,
		}

		// 检查用户名是否已存在，如果存在则添加后缀
		var existingUser model.User
		if h.db.Where("username = ?", user.Username).First(&existingUser).Error == nil {
			user.Username = fmt.Sprintf("%s_%s", biliUsername, biliMid[:4])
		}

		if err := h.db.Create(&user).Error; err != nil {
			fmt.Printf("❌ 创建B站关联用户失败: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "创建用户失败: " + err.Error(),
			})
			return
		}
		fmt.Printf("✅ 创建B站关联用户成功: %s (ID: %d, BiliMid: %s)\n", user.Username, user.ID, biliMid)
	} else {
		// 用户已存在，更新头像
		if biliUserInfo.Face != "" && user.Avatar != biliUserInfo.Face {
			h.db.Model(&user).Update("avatar", biliUserInfo.Face)
		}
	}

	// 4. 生成 JWT Token
	appID := GetAppID(c)
	tokenPair, err := h.jwtService.GenerateTokenPair(user.ID, user.Username, user.MembershipTier, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成 Token 失败",
		})
		return
	}

	// 5. 返回给前端
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Token 换取成功",
		"data": gin.H{
			"token": tokenPair,
			"user": gin.H{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
				"avatar":   user.Avatar,
				"tier":     user.MembershipTier,
				"bili_mid": user.BiliMid,
			},
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
