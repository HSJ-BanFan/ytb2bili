package logger

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// UserContext 用户上下文信息
type UserContext struct {
	UserID   string
	Username string
	Tier     string
}

// ContextLogger 带用户上下文的日志记录器
type ContextLogger struct {
	logger *zap.SugaredLogger
}

// NewContextLogger 创建上下文日志记录器
func NewContextLogger(baseLogger *zap.Logger) *ContextLogger {
	return &ContextLogger{
		logger: baseLogger.Sugar(),
	}
}

// getUserContext 从 gin.Context 提取用户信息
func getUserContext(c *gin.Context) UserContext {
	userID := c.GetString("user_id")
	username := c.GetString("username")
	tier := c.GetString("tier")

	return UserContext{
		UserID:   userID,
		Username: username,
		Tier:     tier,
	}
}

// formatUserPrefix 格式化用户前缀
func formatUserPrefix(user UserContext) string {
	if user.UserID == "" {
		return "[SYSTEM]"
	}

	tierSymbols := map[string]string{
		"free":       "🆓",
		"basic":      "🥉",
		"pro":        "💎",
		"enterprise": "🏢",
	}

	tierIcon := ""
	if icon, ok := tierSymbols[user.Tier]; ok {
		tierIcon = icon + " "
	}

	return fmt.Sprintf("[%s@%s %s]", user.Username, user.UserID, tierIcon)
}

// Infof 记录信息日志（带用户上下文）
func (l *ContextLogger) Infof(c *gin.Context, msg string, args ...interface{}) {
	user := getUserContext(c)
	prefix := formatUserPrefix(user)
	l.logger.Infof("%s %s", prefix, fmt.Sprintf(msg, args...))
}

// Errorf 记录错误日志（带用户上下文）
func (l *ContextLogger) Errorf(c *gin.Context, msg string, args ...interface{}) {
	user := getUserContext(c)
	prefix := formatUserPrefix(user)
	l.logger.Errorf("%s %s", prefix, fmt.Sprintf(msg, args...))
}

// Warnf 记录警告日志（带用户上下文）
func (l *ContextLogger) Warnf(c *gin.Context, msg string, args ...interface{}) {
	user := getUserContext(c)
	prefix := formatUserPrefix(user)
	l.logger.Warnf("%s %s", prefix, fmt.Sprintf(msg, args...))
}

// Debugf 记录调试日志（带用户上下文）
func (l *ContextLogger) Debugf(c *gin.Context, msg string, args ...interface{}) {
	user := getUserContext(c)
	prefix := formatUserPrefix(user)
	l.logger.Debugf("%s %s", prefix, fmt.Sprintf(msg, args...))
}

// WithFields 记录带字段的日志
func (l *ContextLogger) WithFields(c *gin.Context, msg string, fields map[string]interface{}) {
	user := getUserContext(c)

	// 构建字段
	allFields := make([]zap.Field, 0, len(fields)+3)
	allFields = append(allFields,
		zap.String("user_id", user.UserID),
		zap.String("username", user.Username),
		zap.String("tier", user.Tier),
	)

	for k, v := range fields {
		switch val := v.(type) {
		case string:
			allFields = append(allFields, zap.String(k, val))
		case int:
			allFields = append(allFields, zap.Int(k, val))
		case int64:
			allFields = append(allFields, zap.Int64(k, val))
		case bool:
			allFields = append(allFields, zap.Bool(k, val))
		case time.Time:
			allFields = append(allFields, zap.Time(k, val))
		default:
			allFields = append(allFields, zap.Any(k, v))
		}
	}

	prefix := formatUserPrefix(user)
	l.logger.Desugar().Info(prefix, allFields...)
}

// TaskLog 任务专用日志（带视频ID）
func (l *ContextLogger) TaskLog(c *gin.Context, videoID, action, status string, fields map[string]interface{}) {
	user := getUserContext(c)

	allFields := make([]zap.Field, 0, len(fields)+5)
	allFields = append(allFields,
		zap.String("user_id", user.UserID),
		zap.String("username", user.Username),
		zap.String("tier", user.Tier),
		zap.String("video_id", videoID),
		zap.String("action", action),
		zap.String("status", status),
	)

	for k, v := range fields {
		switch val := v.(type) {
		case string:
			allFields = append(allFields, zap.String(k, val))
		case int:
			allFields = append(allFields, zap.Int(k, val))
		case int64:
			allFields = append(allFields, zap.Int64(k, val))
		case error:
			if val != nil {
				allFields = append(allFields, zap.String(k, val.Error()))
			} else {
				allFields = append(allFields, zap.String(k, ""))
			}
		default:
			allFields = append(allFields, zap.Any(k, v))
		}
	}

	// 使用动作作为消息
	tierSymbols := map[string]string{
		"free":       "🆓",
		"basic":      "🥉",
		"pro":        "💎",
		"enterprise": "🏢",
	}
	tierIcon := ""
	if icon, ok := tierSymbols[user.Tier]; ok {
		tierIcon = icon
	}

	prefix := fmt.Sprintf("[%s@%s %s]", user.Username, user.UserID, tierIcon)
	l.logger.Desugar().Info(prefix, allFields...)
}

// 创建带颜色的日志编码器（Windows ANSI 支持）
func newColoredEncoder(isConsole bool) zapcore.Encoder {
	if !isConsole {
		return zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    "function",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		})
	}

	// 控制台编码器 - 带颜色和简化的格式
	config := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 使用标准控制台编码器
	return zapcore.NewConsoleEncoder(config)
}

// SetupUserLogging 设置用户日志记录中间件
func SetupUserLogging(app *gin.Engine, baseLogger *zap.Logger) {
	contextLogger := NewContextLogger(baseLogger)

	// 添加日志中间件到 gin
	app.Use(func(c *gin.Context) {
		// 将 logger 注入到 context
		c.Set("context_logger", contextLogger)
		c.Next()
	})
}

// GetContextLogger 从 context 获取日志记录器
func GetContextLogger(c *gin.Context) *ContextLogger {
	if logger, exists := c.Get("context_logger"); exists {
		if cl, ok := logger.(*ContextLogger); ok {
			return cl
		}
	}
	// 返回默认日志记录器
	return &ContextLogger{logger: zap.NewExample().Sugar()}
}
