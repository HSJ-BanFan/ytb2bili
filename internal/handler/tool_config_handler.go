package handler

import (
	"net/http"

	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/gin-gonic/gin"
)

// ToolConfigHandler 工具配置处理器
type ToolConfigHandler struct {
	App               *core.AppServer
	ToolConfigService *services.ToolConfigService
}

// NewToolConfigHandler 创建工具配置处理器
func NewToolConfigHandler(app *core.AppServer, toolConfigService *services.ToolConfigService) *ToolConfigHandler {
	return &ToolConfigHandler{
		App:               app,
		ToolConfigService: toolConfigService,
	}
}

// RegisterRoutes 注册全局工具配置路由
func (h *ToolConfigHandler) RegisterRoutes(server *core.AppServer) {
	api := server.Engine.Group("/api/v1/tool/config")
	api.GET("/ai", h.getToolAIConfig)
	api.PUT("/ai", h.updateToolAIConfig)
	api.GET("/preferences", h.getToolPreferences)
	api.PUT("/preferences", h.updateToolPreferences)
}

// getToolAIConfig 获取工具AI配置
func (h *ToolConfigHandler) getToolAIConfig(c *gin.Context) {

	config, err := h.ToolConfigService.GetAIConfig()
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

// updateToolAIConfig 更新工具AI配置
func (h *ToolConfigHandler) updateToolAIConfig(c *gin.Context) {

	var config model.ToolAIConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 更新配置
	if err := h.ToolConfigService.UpdateAIConfig(&config); err != nil {
		h.App.Logger.Errorf("更新工具AI配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败",
		})
		return
	}

	h.App.Logger.Info("✅ 更新了AI配置")
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "更新成功",
		"data":    config,
	})
}

// getToolPreferences 获取工具偏好
func (h *ToolConfigHandler) getToolPreferences(c *gin.Context) {

	pref, err := h.ToolConfigService.GetPreference()
	if err != nil {
		// 偏好不存在，返回默认配置
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "获取成功",
			"data": gin.H{
				"default_auto_upload":    true,
				"default_upload_delay":   10,
				"default_subtitle_delay": 10,
				"default_copyright":      2,
				"default_source":         "YouTube",
				"default_tid":            122,
				"theme":                  "light",
				"language":               "zh",
				"items_per_page":         20,
				"show_advanced":          false,
				"enable_analytics":       false,
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

// updateToolPreferences 更新工具偏好
func (h *ToolConfigHandler) updateToolPreferences(c *gin.Context) {

	var pref model.ToolPreference
	if err := c.ShouldBindJSON(&pref); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 更新偏好
	if err := h.ToolConfigService.UpdatePreference(&pref); err != nil {
		h.App.Logger.Errorf("更新工具偏好失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败",
		})
		return
	}

	h.App.Logger.Info("✅ 更新了偏好设置")
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "更新成功",
		"data":    pref,
	})
}
