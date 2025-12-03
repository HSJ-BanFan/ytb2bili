package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/membership"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/difyz9/ytb2bili/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SubtitleHandler struct {
	BaseHandler
	quotaService   *membership.QuotaService
	featureChecker *membership.FeatureChecker
	jwtMiddleware  func() gin.HandlerFunc
}

func NewSubtitleHandler(app *core.AppServer, membershipStore membership.MembershipStore, jwtMiddleware func() gin.HandlerFunc) *SubtitleHandler {
	checker := membership.NewFeatureChecker(membershipStore)
	return &SubtitleHandler{
		BaseHandler:    BaseHandler{App: app},
		quotaService:   membership.NewQuotaService(membershipStore, checker),
		featureChecker: checker,
		jwtMiddleware:  jwtMiddleware,
	}
}

// SaveVideoRequest 保存视频请求
type SaveVideoRequest struct {
	URL           string                     `json:"url" binding:"required"`
	Title         string                     `json:"title"`
	Description   string                     `json:"description"`
	OperationType string                     `json:"operationType"`
	Subtitles     []model.SavedVideoSubtitle `json:"subtitles"`
	PlaylistID    string                     `json:"playlistId"`
	Timestamp     string                     `json:"timestamp"`
	SavedAt       string                     `json:"savedAt"`
}

func (h *SubtitleHandler) saveVideoSubtitles(c *gin.Context) {
	var req SaveVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request parameters: " + err.Error(),
		})
		return
	}

	// 获取用户ID（从 JWT context 获取，JWT 中间件已验证）
	var userID string
	var userIDUint uint
	if uid, exists := c.Get("user_id"); exists {
		switch v := uid.(type) {
		case uint:
			userID = fmt.Sprintf("%d", v)
			userIDUint = v
		case string:
			userID = v
			if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
				userIDUint = uint(parsed)
			}
		default:
			userID = fmt.Sprintf("%v", v)
		}
	}

	// 如果没有用户ID，说明 JWT 认证失败（理论上不会到这里，因为中间件会拦截）
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "请先登录",
			"code":    "UNAUTHORIZED",
		})
		return
	}

	fmt.Printf("📋 视频提交请求 - UserID: '%s', URL: %s\n", userID, req.URL)

	// 检查配额
	if h.quotaService != nil {
		quotaInfo, err := h.quotaService.GetQuotaInfo(c.Request.Context(), userID)
		if err == nil && !quotaInfo.IsUnlimited && quotaInfo.TotalRemaining <= 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "配额已用完，请升级会员或购买加油包",
				"code":    "QUOTA_EXCEEDED",
			})
			return
		}
	}

	// 检查 AI 功能权限（Gemini 视频分析需要 Pro 会员）
	// 注意：这里只是警告，不阻止提交，因为基础功能（下载、字幕）不需要会员
	if h.featureChecker != nil {
		geminiCheck := h.featureChecker.CanUseFeature(c.Request.Context(), userID, "gemini_video_analysis")
		if !geminiCheck.Allowed {
			fmt.Printf("⚠️ 用户 %s 没有 Gemini 视频分析权限: %s (建议升级到: %s)\n", userID, geminiCheck.Reason, geminiCheck.Upgrade)
			// 不阻止提交，但在日志中记录，任务执行时会跳过 AI 元数据生成
		}
	}

	fmt.Println("Received saveVideoSubtitles request for URL:", req.URL)
	// 从 URL 中提取 videoId
	videoID := utils.ExtractVideoID(req.URL)
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid video URL: cannot extract video ID",
		})
		return
	}
	fmt.Println("Extracted videoId:", videoID)

	// 将字幕数组转换为JSON字符串
	subtitlesJSON, err := json.Marshal(req.Subtitles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to marshal subtitles: " + err.Error(),
		})
		return
	}

	// 检查字幕数据大小
	subtitlesJSONStr := string(subtitlesJSON)
	fmt.Printf("字幕数据长度: %d 字符\n", len(subtitlesJSONStr))
	fmt.Printf("字幕条目数量: %d\n", len(req.Subtitles))

	// 如果数据太大，截断前100个字符用于调试
	if len(subtitlesJSONStr) > 100 {
		fmt.Printf("字幕数据前100字符: %s...\n", subtitlesJSONStr[:100])
	} else {
		fmt.Printf("字幕数据: %s\n", subtitlesJSONStr)
	}

	// 检查是否已存在相同的 videoId（包括已删除的记录）
	var existingVideo model.SavedVideo
	err = h.App.DB.Unscoped().Where("video_id = ?", videoID).First(&existingVideo).Error

	var savedVideo *model.SavedVideo
	isExisting := false

	if err == nil {
		// 找到了记录（可能是已删除的），更新字段
		isExisting = true
		existingVideo.URL = req.URL
		existingVideo.Title = req.Title
		existingVideo.Description = req.Description
		existingVideo.OperationType = req.OperationType
		existingVideo.Subtitles = subtitlesJSONStr
		existingVideo.PlaylistID = req.PlaylistID
		existingVideo.Timestamp = req.Timestamp
		existingVideo.SavedAt = req.SavedAt
		existingVideo.Status = "001"               // 重置状态为待处理
		existingVideo.DeletedAt = gorm.DeletedAt{} // 恢复记录（清除删除标记）
		existingVideo.UserID = userIDUint          // 更新提交用户ID

		// 更新到数据库（使用 Unscoped 以便更新已删除的记录）
		if err := h.App.DB.Unscoped().Save(&existingVideo).Error; err != nil {
			fmt.Printf("更新视频失败，字幕数据长度: %d\n", len(subtitlesJSONStr))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to update video: " + err.Error(),
			})
			return
		}
		savedVideo = &existingVideo

		if existingVideo.DeletedAt.Valid {
			fmt.Printf("✅ 恢复已删除的视频: %s\n", videoID)
		}
	} else if err == gorm.ErrRecordNotFound {
		// 记录不存在，创建新记录
		savedVideo = &model.SavedVideo{
			VideoID:       videoID,
			URL:           req.URL,
			Title:         req.Title,
			Status:        "001",
			Description:   req.Description,
			OperationType: req.OperationType,
			Subtitles:     subtitlesJSONStr,
			PlaylistID:    req.PlaylistID,
			Timestamp:     req.Timestamp,
			SavedAt:       req.SavedAt,
			UserID:        userIDUint, // 保存提交用户ID
		}

		// 保存到数据库
		if err := h.App.DB.Create(savedVideo).Error; err != nil {
			fmt.Printf("创建视频失败，字幕数据长度: %d\n", len(subtitlesJSONStr))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to save video: " + err.Error(),
			})
			return
		}
	} else {
		// 数据库查询出错
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Database error: " + err.Error(),
		})
		return
	}

	// 计算字幕数量
	subtitleCount := len(req.Subtitles)

	// 消耗配额（仅对新视频，不是更新已存在的视频）
	if !isExisting && userID != "" && h.quotaService != nil {
		if err := h.quotaService.ConsumeQuota(c.Request.Context(), userID); err != nil {
			fmt.Printf("⚠️ 消耗配额失败: %v\n", err)
		} else {
			fmt.Printf("✅ 已消耗用户 %s 的配额\n", userID)
		}
	}

	message := "Video saved successfully"
	if isExisting {
		message = "Video updated successfully"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data": gin.H{
			"id":            savedVideo.ID,
			"title":         savedVideo.Title,
			"operationType": savedVideo.OperationType,
			"subtitleCount": subtitleCount,
			"isExisting":    isExisting,
		},
	})
}

// RegisterRoutes 注册上传相关路由
func (h *SubtitleHandler) RegisterRoutes(server *core.AppServer) {
	api := server.Engine.Group("/api/v1")

	// /submit 接口需要 JWT 认证
	if h.jwtMiddleware != nil {
		api.POST("/submit", h.jwtMiddleware(), h.saveVideoSubtitles)
	} else {
		api.POST("/submit", h.saveVideoSubtitles)
	}
}
