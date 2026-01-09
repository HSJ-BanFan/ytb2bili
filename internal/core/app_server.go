package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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

	// 设置静态文件服务 (Next.js 静态导出)
	s.setupStaticFileServing()
}

// setupMiddleware 设置中间件
func (s *AppServer) setupMiddleware() {
	// 🔐 1. 安全响应头中间件
	s.setupSecurityHeaders()

	// 🔐 2. CORS 中间件 (跨域资源共享)
	s.setupCORSMiddleware()

	// 🔐 3. CSP 中间件 (内容安全策略)
	s.setupCSPMiddleware()

	// 🔐 4. HSTS 中间件 (严格传输安全)
	s.setupHSTSMiddleware()
}

// setupStaticFileServing 设置静态文件服务 (支持 Next.js 静态导出)
func (s *AppServer) setupStaticFileServing() {
	// 静态资源目录
	staticDir := "./static"

	// 服务 Next.js 的 _next 静态资源
	s.Engine.Static("/_next", staticDir+"/_next")

	// 服务其他静态资源
	s.Engine.StaticFile("/favicon.ico", staticDir+"/favicon.ico")

	// 根路径返回 index.html
	s.Engine.GET("/", func(c *gin.Context) {
		c.File(staticDir + "/index.html")
	})

	// Next.js 页面路由 - 每个页面对应一个目录
	pages := []string{
		"/login",
		"/dashboard",
		"/bili-accounts",
		"/extension",
		"/membership",
		"/schedule",
		"/settings",
		"/user-settings",
	}

	for _, page := range pages {
		pagePath := page
		s.Engine.GET(pagePath, func(c *gin.Context) {
			c.File(staticDir + pagePath + "/index.html")
		})
		// 支持带斜杠的路径
		s.Engine.GET(pagePath+"/", func(c *gin.Context) {
			c.File(staticDir + pagePath + "/index.html")
		})
	}

	// 404 页面
	s.Engine.NoRoute(func(c *gin.Context) {
		// 如果是 API 请求，返回 JSON 404
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "API not found"})
			return
		}
		// 否则返回 404 HTML 页面
		c.File(staticDir + "/404.html")
	})
}

// setupSecurityHeaders 设置通用安全响应头
func (s *AppServer) setupSecurityHeaders() {
	s.Engine.Use(func(c *gin.Context) {
		// 防止点击劫持
		c.Header("X-Frame-Options", "SAMEORIGIN")

		// 防止 MIME 类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")

		// 启用 XSS 过滤
		c.Header("X-XSS-Protection", "1; mode=block")

		// 控制 Referer 泄露
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions-Policy (如果启用)
		if s.Config.Security.PermissionsPolicyEnabled {
			pp := s.Config.Security.PermissionsPolicy
			if pp == "" {
				// 默认策略
				pp = "geolocation=(), microphone=(), camera=(), payment=(), usb=()"
			}
			c.Header("Permissions-Policy", pp)
		}

		// API 响应禁止缓存
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
			c.Header("Pragma", "no-cache")
		}

		c.Next()
	})
}

// setupCORSMiddleware 设置 CORS 中间件
func (s *AppServer) setupCORSMiddleware() {
	s.Engine.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 非跨域请求直接放行
		if origin == "" {
			c.Next()
			return
		}

		allowed := false

		// 检查白名单
		// 逻辑变更 (P2-1): 只要配置了 CORSAllowedOrigins，无论什么环境都应生效。
		// 只有当 CORSAllowedOrigins 为空且不是生产环境时，才默认允许所有。
		if len(s.Config.Security.CORSAllowedOrigins) > 0 || s.Config.Environment == "production" {
			// 1. 验证 Origin 格式 (拒绝 null 或非法格式)
			if origin == "null" || (!strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://")) {
				s.Logger.Warnw("CORS Blocked: Invalid Origin", "origin", origin, "path", c.Request.URL.Path, "ip", c.ClientIP())
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid Origin header"})
				return
			}

			// 2. 检查白名单
			for _, allowedOrigin := range s.Config.Security.CORSAllowedOrigins {
				if origin == allowedOrigin {
					allowed = true
					break
				}
			}

			if !allowed {
				s.Logger.Warnw("CORS Blocked: Origin not allowed", "origin", origin, "path", c.Request.URL.Path, "ip", c.ClientIP())
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Origin not allowed"})
				return
			}
		} else {
			// 开发环境且未配置白名单: 默认允许（为了方便调试）
			allowed = true
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Accept, Cache-Control, X-Requested-With, X-User-Id, X-Device-Id")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
			c.Header("Access-Control-Max-Age", "172800")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin") // 告诉缓存这是基于 Origin 的
		}

		// 处理预检请求
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})
}

// setupCSPMiddleware 设置 CSP 中间件
func (s *AppServer) setupCSPMiddleware() {
	s.Engine.Use(func(c *gin.Context) {
		if !s.Config.Security.CSPEnabled {
			c.Next()
			return
		}

		// 动态构建 CSP 策略
		var cspParts []string

		// 默认策略
		cspParts = append(cspParts, "default-src 'self'")

		// 脚本源
		scriptSrc := s.Config.Security.CSPScriptSrc
		if scriptSrc == "" {
			scriptSrc = "'self'"
		}
		cspParts = append(cspParts, fmt.Sprintf("script-src %s", scriptSrc))

		// 样式源
		styleSrc := s.Config.Security.CSPStyleSrc
		if styleSrc == "" {
			styleSrc = "'self' 'unsafe-inline'" // 默认允许内联样式(许多UI库需要)
		}
		cspParts = append(cspParts, fmt.Sprintf("style-src %s", styleSrc))

		// 图片源
		cspParts = append(cspParts, "img-src 'self' data: https:")

		// 连接源 (允许连接自身和 B 站 API)
		cspParts = append(cspParts, "connect-src 'self' https://api.bilibili.com")

		// 框架源 (禁止被嵌入)
		cspParts = append(cspParts, "frame-ancestors 'none'")

		// 基础 URI
		cspParts = append(cspParts, "base-uri 'self'")

		// 表单提交
		cspParts = append(cspParts, "form-action 'self'")

		// 报告 URI
		cspParts = append(cspParts, "report-uri /api/v1/security/csp-report")

		cspHeader := strings.Join(cspParts, "; ")

		if s.Config.Security.CSPReportOnly {
			c.Header("Content-Security-Policy-Report-Only", cspHeader)
		} else {
			c.Header("Content-Security-Policy", cspHeader)
		}

		c.Next()
	})
}

// setupHSTSMiddleware 设置 HSTS 中间件
func (s *AppServer) setupHSTSMiddleware() {
	s.Engine.Use(func(c *gin.Context) {
		// 仅在 HTTPS 请求时启用
		if c.Request.TLS != nil && s.Config.Security.HSTSEnabled {
			maxAge := s.Config.Security.HSTSMaxAge
			if maxAge <= 0 {
				maxAge = 31536000 // 默认 1 年
			}

			directives := []string{
				fmt.Sprintf("max-age=%d", maxAge),
			}

			if s.Config.Security.HSTSIncludeSubdomains {
				directives = append(directives, "includeSubDomains")
			}

			if s.Config.Security.HSTSPreload {
				directives = append(directives, "preload")
			}

			c.Header("Strict-Transport-Security", strings.Join(directives, "; "))
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
