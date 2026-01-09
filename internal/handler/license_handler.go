package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/difyz9/ytb2bili/internal/auth"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/difyz9/ytb2bili/internal/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LicenseHandler 许可证处理器
type LicenseHandler struct {
	licenseService *services.LicenseService
	logger         *zap.SugaredLogger
}

// NewLicenseHandler 创建许可证处理器
func NewLicenseHandler(licenseService *services.LicenseService, logger *zap.SugaredLogger) *LicenseHandler {
	return &LicenseHandler{
		licenseService: licenseService,
		logger:         logger,
	}
}

// RegisterRoutes 注册License相关路由
func (h *LicenseHandler) RegisterRoutes(server *core.AppServer, authMiddleware *auth.AuthMiddleware, goAuthMiddleware *middleware.GoAuthMiddleware) {
	api := server.Engine.Group("/api/v1")

	// 公开接口 - 验证许可证（不需要登录）
	api.POST("/license/verify", h.VerifyLicense)

	// 需要认证的接口
	licenseGroup := api.Group("/license")
	licenseGroup.Use(authMiddleware.JWTAuth())
	{
		licenseGroup.POST("/activate", h.ActivateLicense)
		licenseGroup.GET("/status", h.GetStatus)
		licenseGroup.GET("/list", h.GetLicenseList)
	}

	// 管理员接口 - 生成许可证
	// TODO: 添加管理员权限验证中间件
	adminGroup := api.Group("/admin/license")
	adminGroup.Use(authMiddleware.JWTAuth()) // 需要登录
	{
		adminGroup.POST("/generate", h.GenerateLicense)
	}
}

// ActivateLicenseRequest 激活请求
type ActivateLicenseRequest struct {
	LicenseKey string `json:"license_key" binding:"required"`
}

// VerifyLicenseRequest 验证请求
type VerifyLicenseRequest struct {
	LicenseKey string `json:"license_key" binding:"required"`
}

// GenerateLicenseRequest 生成许可证请求
type GenerateLicenseRequest struct {
	Tier     string `json:"tier" binding:"required"` // basic, pro, enterprise
	Plan     string `json:"plan" binding:"required"` // trial, monthly, quarterly, yearly, lifetime
	PermCode string `json:"perm_code"`               // 4位权限代码（可选）
	Days     int    `json:"days"`                    // 自定义天数（可选）
	Months   int    `json:"months"`                  // 自定义月数（可选）
}

// VerifyLicense 验证许可证（不激活）
func (h *LicenseHandler) VerifyLicense(c *gin.Context) {
	var req VerifyLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	licenseInfo, err := h.licenseService.VerifyLicense(req.LicenseKey)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "License verification failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "许可证有效",
		"data": gin.H{
			"license_key": licenseInfo.LicenseKey,
			"tier":        licenseInfo.Tier,
			"plan":        licenseInfo.Plan,
			"expires_at":  licenseInfo.ExpiresAt,
			"is_expired":  licenseInfo.IsExpired,
			"is_valid":    licenseInfo.IsValid,
		},
	})
}

// ActivateLicense 激活许可证
func (h *LicenseHandler) ActivateLicense(c *gin.Context) {
	var req ActivateLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// 获取当前用户ID
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "User not authenticated",
		})
		return
	}

	// 先验证许可证获取信息
	licenseInfo, err := h.licenseService.VerifyLicense(req.LicenseKey)
	if err != nil {
		h.logger.Warnf("License verification failed for user %s: %v", userID, err)
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.licenseService.ActivateLicense(c.Request.Context(), userID, req.LicenseKey); err != nil {
		h.logger.Warnf("License activation failed for user %s: %v", userID, err)
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	h.logger.Infof("License activated successfully for user %s, tier: %s", userID, licenseInfo.Tier)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "许可证激活成功",
		"data": gin.H{
			"tier":       licenseInfo.Tier,
			"expires_at": licenseInfo.ExpiresAt,
		},
	})
}

// GetStatus 获取当前用户的会员状态
func (h *LicenseHandler) GetStatus(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "User not authenticated",
		})
		return
	}

	activations, err := h.licenseService.GetUserLicenses(c.Request.Context(), userID)
	if err != nil {
		h.logger.Errorf("Failed to get license status for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Internal server error",
		})
		return
	}

	// 找到当前有效的最高等级
	var currentTier types.Tier = types.TierBasic
	var expiresAt *time.Time

	for i := range activations {
		if services.IsLicenseValid(&activations[i]) {
			// 比较等级优先级
			if tierPriority(activations[i].Tier) > tierPriority(currentTier) {
				currentTier = activations[i].Tier
				expiresAt = activations[i].ExpiresAt
			}
		}
	}

	// 获取当前等级的详细配置
	config := types.GetTierConfig(currentTier)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"tier":             currentTier,
			"tier_name":        config.Name,
			"expires_at":       expiresAt,
			"activation_count": len(activations),
			"features":         config.Features,
			"limits":           config.Limits,
		},
	})
}

// GetLicenseList 获取用户许可证列表
func (h *LicenseHandler) GetLicenseList(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "User not authenticated",
		})
		return
	}

	activations, err := h.licenseService.GetUserLicenses(c.Request.Context(), userID)
	if err != nil {
		h.logger.Errorf("Failed to get license list for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Internal server error",
		})
		return
	}

	var result []gin.H
	for i := range activations {
		result = append(result, gin.H{
			"id":           activations[i].ID,
			"license_key":  maskLicenseKey(activations[i].LicenseKey),
			"user_id":      activations[i].UserID,
			"tier":         activations[i].Tier,
			"plan":         activations[i].Plan,
			"activated_at": activations[i].ActivatedAt,
			"expires_at":   activations[i].ExpiresAt,
			"is_active":    services.IsLicenseValid(&activations[i]),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// GenerateLicense 管理员生成许可证
func (h *LicenseHandler) GenerateLicense(c *gin.Context) {
	// TODO: 添加管理员权限检查
	// 目前只检查登录状态，生产环境应添加权限验证

	var req GenerateLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// 解析等级
	var tier types.Tier
	switch req.Tier {
	case "basic":
		tier = types.TierBasic
	case "pro":
		tier = types.TierPro
	case "enterprise":
		tier = types.TierEnterprise
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid tier: must be basic, pro, or enterprise",
		})
		return
	}

	// 计算过期时间
	var expiresAt *time.Time
	if req.Plan != "lifetime" {
		var duration time.Duration
		if req.Days > 0 {
			duration = time.Duration(req.Days) * 24 * time.Hour
		} else if req.Months > 0 {
			duration = time.Duration(req.Months) * 30 * 24 * time.Hour
		} else {
			// 根据 plan 设置默认时长
			switch req.Plan {
			case "trial":
				duration = 7 * 24 * time.Hour
			case "monthly":
				duration = 30 * 24 * time.Hour
			case "quarterly":
				duration = 90 * 24 * time.Hour
			case "yearly":
				duration = 365 * 24 * time.Hour
			default:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    400,
					"message": "Invalid plan: must be trial, monthly, quarterly, yearly, or lifetime",
				})
				return
			}
		}
		expireTime := time.Now().Add(duration)
		expiresAt = &expireTime
	}

	// 生成许可证 (传入 PermCode)
	licenseKey, err := h.licenseService.GenerateLicense(tier, req.Plan, expiresAt, req.PermCode)
	if err != nil {
		h.logger.Errorf("Failed to generate license: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to generate license: " + err.Error(),
		})
		return
	}

	h.logger.Infof("License generated: tier=%s, plan=%s, expires=%v, perm=%s", tier, req.Plan, expiresAt, req.PermCode)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "许可证生成成功",
		"data": gin.H{
			"license_key": licenseKey,
			"tier":        tier,
			"plan":        req.Plan,
			"perm_code":   req.PermCode,
			"expires_at":  expiresAt,
		},
	})
}

// getUserID 从上下文获取用户ID
func (h *LicenseHandler) getUserID(c *gin.Context) string {
	if uid, ok := c.Get("user_id_str"); ok {
		return uid.(string)
	}
	if uid, ok := c.Get("user_id"); ok {
		return fmt.Sprintf("%v", uid)
	}
	return ""
}

// maskLicenseKey 掩码许可证密钥
func maskLicenseKey(key string) string {
	if len(key) < 10 {
		return key
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// tierPriority 返回等级优先级
func tierPriority(tier types.Tier) int {
	switch tier {
	case types.TierEnterprise:
		return 3
	case types.TierPro:
		return 2
	case types.TierBasic:
		return 1
	default:
		return 0
	}
}
