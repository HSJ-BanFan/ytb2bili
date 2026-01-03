package handler

import (
	"net/http"

	"github.com/difyz9/ytb2bili/internal/auth"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/gin-gonic/gin"
)

// UserConfigHandler 用户配置处理器
type UserConfigHandler struct {
	App               *core.AppServer
	UserConfigService *services.UserConfigService
}

// NewUserConfigHandler 创建用户配置处理器
func NewUserConfigHandler(app *core.AppServer, userConfigService *services.UserConfigService) *UserConfigHandler {
	return &UserConfigHandler{
		App:               app,
		UserConfigService: userConfigService,
	}
}

// RegisterRoutes 注册路由
func (h *UserConfigHandler) RegisterRoutes(server *core.AppServer, authMiddleware *auth.AuthMiddleware) {
	// 用户配置路由（需要 JWT 认证）
	api := server.Engine.Group("/api/v1")
	api.Use(authMiddleware.JWTAuth())

	userConfig := api.Group("/user/config")
	{
		// AI 配置
		userConfig.GET("/ai", h.getUserAIConfig)
		userConfig.PUT("/ai", h.updateUserAIConfig)

		// 偏好设置
		userConfig.GET("/preferences", h.getUserPreferences)
		userConfig.PUT("/preferences", h.updateUserPreferences)
	}
}

// getUserAIConfig 获取用户AI配置
func (h *UserConfigHandler) getUserAIConfig(c *gin.Context) {
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	config, err := h.UserConfigService.GetUserAIConfig(userID)
	if err != nil {
		// 配置不存在，返回默认配置
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "获取成功",
			"data": gin.H{
				"deepseek_enabled":   false,
				"gemini_enabled":     false,
				"openai_enabled":     false,
				"baidu_enabled":      false,
				"uses_system_config": true,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    config,
	})
}

// updateUserAIConfig 更新用户AI配置
func (h *UserConfigHandler) updateUserAIConfig(c *gin.Context) {
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	var config model.UserAIConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 更新配置
	if err := h.UserConfigService.UpdateUserAIConfig(userID, &config); err != nil {
		h.App.Logger.Errorf("更新用户AI配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败",
		})
		return
	}

	h.App.Logger.Infof("✅ 用户 %d 更新了AI配置", userID)
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "更新成功",
		"data":    config,
	})
}

// getUserPreferences 获取用户偏好
func (h *UserConfigHandler) getUserPreferences(c *gin.Context) {
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	pref, err := h.UserConfigService.GetUserPreference(userID)
	if err != nil {
		// 偏好不存在，返回默认配置
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "获取成功",
			"data": gin.H{
				"default_auto_upload":         true,
				"default_upload_delay":        10,
				"default_subtitle_delay":      10,
				"default_copyright":           2,
				"default_source":              "YouTube",
				"default_tid":                 122,
				"theme":                       "light",
				"language":                    "zh",
				"items_per_page":              20,
				"show_advanced":               false,
				"email_notifications_enabled": true,
				"enable_analytics":            false,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    pref,
	})
}

// updateUserPreferences 更新用户偏好
func (h *UserConfigHandler) updateUserPreferences(c *gin.Context) {
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	var pref model.UserPreference
	if err := c.ShouldBindJSON(&pref); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 更新偏好
	if err := h.UserConfigService.UpdateUserPreference(userID, &pref); err != nil {
		h.App.Logger.Errorf("更新用户偏好失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败",
		})
		return
	}

	h.App.Logger.Infof("✅ 用户 %d 更新了偏好设置", userID)
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "更新成功",
		"data":    pref,
	})
}
