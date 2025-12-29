package logger

import (
	"fmt"

	"go.uber.org/zap"
)

// UserLogHelper 用户日志助手 - 为后台任务添加用户上下文
type UserLogHelper struct {
	logger *zap.Logger
	userID uint
}

// NewUserLogger 创建带用户上下文的日志助手
func NewUserLogger(logger *zap.Logger, userID uint) *UserLogHelper {
	return &UserLogHelper{
		logger: logger,
		userID: userID,
	}
}

// formatPrefix 格式化日志前缀
func (h *UserLogHelper) formatPrefix() string {
	// 从缓存或数据库获取用户信息
	// 简化版本：直接使用 userID
	return fmt.Sprintf("[user_%d]", h.userID)
}

// Infow 带字段的信息日志
func (h *UserLogHelper) Infow(msg string, fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.Uint("user_id", h.userID),
	}, fields...)
	h.logger.Info(msg, allFields...)
}

// Errorw 带字段的错误日志
func (h *UserLogHelper) Errorw(msg string, fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.Uint("user_id", h.userID),
	}, fields...)
	h.logger.Error(msg, allFields...)
}

// Warnw 带字段的警告日志
func (h *UserLogHelper) Warnw(msg string, fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.Uint("user_id", h.userID),
	}, fields...)
	h.logger.Warn(msg, allFields...)
}

// Debugw 带字段的调试日志
func (h *UserLogHelper) Debugw(msg string, fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.Uint("user_id", h.userID),
	}, fields...)
	h.logger.Debug(msg, allFields...)
}

// Warnm 带 map 字段的警告日志
func (h *UserLogHelper) Warnm(msg string, fields map[string]interface{}) {
	allFields := []zap.Field{
		zap.Uint("user_id", h.userID),
	}
	for k, v := range fields {
		switch val := v.(type) {
		case string:
			allFields = append(allFields, zap.String(k, val))
		case int:
			allFields = append(allFields, zap.Int(k, val))
		case int64:
			allFields = append(allFields, zap.Int64(k, val))
		case float64:
			allFields = append(allFields, zap.Float64(k, val))
		case bool:
			allFields = append(allFields, zap.Bool(k, val))
		case error:
			if val != nil {
				allFields = append(allFields, zap.String(k, val.Error()))
			}
		default:
			allFields = append(allFields, zap.Any(k, v))
		}
	}
	h.logger.Warn(msg, allFields...)
}

// Errorm 带 map 字段的错误日志
func (h *UserLogHelper) Errorm(msg string, fields map[string]interface{}) {
	allFields := []zap.Field{
		zap.Uint("user_id", h.userID),
	}
	for k, v := range fields {
		switch val := v.(type) {
		case string:
			allFields = append(allFields, zap.String(k, val))
		case int:
			allFields = append(allFields, zap.Int(k, val))
		case int64:
			allFields = append(allFields, zap.Int64(k, val))
		case float64:
			allFields = append(allFields, zap.Float64(k, val))
		case bool:
			allFields = append(allFields, zap.Bool(k, val))
		case error:
			if val != nil {
				allFields = append(allFields, zap.String(k, val.Error()))
			}
		default:
			allFields = append(allFields, zap.Any(k, v))
		}
	}
	h.logger.Error(msg, allFields...)
}

// TaskLog 任务专用日志（推荐）
func (h *UserLogHelper) TaskLog(videoID, action, status string, fields map[string]interface{}) {
	// 构建字段
	allFields := []zap.Field{
		zap.Uint("user_id", h.userID),
		zap.String("video_id", videoID),
		zap.String("action", action),
		zap.String("status", status),
	}

	// 添加自定义字段
	for k, v := range fields {
		switch val := v.(type) {
		case string:
			allFields = append(allFields, zap.String(k, val))
		case int:
			allFields = append(allFields, zap.Int(k, val))
		case int64:
			allFields = append(allFields, zap.Int64(k, val))
		case float64:
			allFields = append(allFields, zap.Float64(k, val))
		case bool:
			allFields = append(allFields, zap.Bool(k, val))
		case error:
			if val != nil {
				allFields = append(allFields, zap.String(k+"_error", val.Error()))
			}
		default:
			allFields = append(allFields, zap.Any(k, v))
		}
	}

	// 添加动作图标
	actionIcons := map[string]string{
		"download":        "📥",
		"subtitle":        "📝",
		"translate":       "🌐",
		"metadata":        "✨",
		"upload_video":    "📤",
		"upload_subtitle": "📄",
		"retry":           "🔄",
		"success":         "✅",
		"failed":          "❌",
		"pending":         "⏳",
	}

	icon := ""
	if ic, ok := actionIcons[action]; ok {
		icon = ic + " "
	}

	prefix := fmt.Sprintf("%s%s", h.formatPrefix(), icon)

	// 根据状态选择日志级别：skipped 使用 debug 避免免费用户日志刷屏
	if status == "skipped" {
		h.logger.Debug(prefix, allFields...)
	} else {
		h.logger.Info(prefix, allFields...)
	}
}

// WithFields 带任意字段的日志
func (h *UserLogHelper) WithFields(msg string, fields map[string]interface{}) {
	allFields := []zap.Field{
		zap.Uint("user_id", h.userID),
	}

	for k, v := range fields {
		switch val := v.(type) {
		case string:
			allFields = append(allFields, zap.String(k, val))
		case int:
			allFields = append(allFields, zap.Int(k, val))
		case error:
			if val != nil {
				allFields = append(allFields, zap.String(k, val.Error()))
			}
		default:
			allFields = append(allFields, zap.Any(k, v))
		}
	}

	h.logger.Info(msg, allFields...)
}
