package core

import (
	"context"
	"fmt"
	"net/http"

	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/difyz9/ytb2bili/pkg/cos"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AppServer 应用服务器
type AppServer struct {
	Config    *types.AppConfig
	Engine    *gin.Engine
	Logger    *zap.SugaredLogger
	DB        *gorm.DB
	CosClient *cos.CosClient // COS客户端

}

// NewServer 创建新的服务器实例
func NewServer(config *types.AppConfig, logger *zap.SugaredLogger) *AppServer {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)
	// gin.DefaultWriter = io.Discard // 不要丢弃日志，我们需要看到它！
	return &AppServer{
		Config: config,
		Engine: gin.New(), // 使用 New() 而不是 Default()，避免并在下面手动添加必要的中间件
		Logger: logger,
	}
}

// Init 初始化服务器
func (s *AppServer) Init(db *gorm.DB) {
	s.DB = db

	// 必须首先添加 Recovery 中间件，以防 panic
	s.Engine.Use(gin.Recovery())

	// 还有默认的 Logger
	s.Engine.Use(gin.Logger())

	// 设置中间件
	s.setupMiddleware()

	// 设置静态文件
	s.Engine.Static("/static", "./static")
}

// setupMiddleware 设置中间件
func (s *AppServer) setupMiddleware() {
	// 🔐 安全响应头中间件
	s.Engine.Use(func(c *gin.Context) {
		// 防止点击劫持 (Clickjacking)
		c.Header("X-Frame-Options", "SAMEORIGIN")

		// 防止 MIME 类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")

		// 启用 XSS 过滤（现代浏览器已内置，这是额外保护）
		c.Header("X-XSS-Protection", "1; mode=block")

		// 控制 Referer 信息泄露
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// 禁止浏览器缓存敏感信息（API 响应）
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
			c.Header("Pragma", "no-cache")
		}

		c.Next()
	})

	// CORS中间件
	s.Engine.Use(func(c *gin.Context) {
		method := c.Request.Method
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Accept, Cache-Control, X-Requested-With, X-User-Id, X-Device-Id") // 添加 X-Device-Id
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
			c.Header("Access-Control-Max-Age", "172800")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})
}

// Run 启动服务器
func (s *AppServer) Run() error {
	s.Logger.Infof("Starting server on %s", s.Config.Listen)
	s.Logger.Infof("Environment: %s", s.Config.Environment)

	fmt.Println("listening on ---> ", s.Config.Listen)

	return s.Engine.Run(s.Config.Listen)
}

// Shutdown 优雅关闭服务器
func (s *AppServer) Shutdown(ctx context.Context) error {
	s.Logger.Info("Shutting down server...")

	return nil
}
