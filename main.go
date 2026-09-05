package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/difyz9/ytb2bili/internal/auth"
	"github.com/difyz9/ytb2bili/internal/chain_task"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/difyz9/ytb2bili/internal/handler"
	"github.com/difyz9/ytb2bili/pkg/audit"
	"github.com/difyz9/ytb2bili/pkg/logger"
	"github.com/difyz9/ytb2bili/pkg/store"

	"github.com/robfig/cron/v3"
)

func main() {
	// 加载配置
	cfg, err := types.LoadConfig("config.toml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	appLogger, err := logger.NewLogger(cfg.Debug)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer appLogger.Sync()

	appLogger.Info("Starting application...")
	appLogger.Infof("Environment: %s", cfg.Environment)

	// 初始化数据库连接
	dbConn, err := store.NewDatabase(cfg)
	if err != nil {
		appLogger.Fatalf("Failed to initialize database: %v", err)
	}

	// 自动迁移数据库
	if err := store.AutoMigrate(dbConn); err != nil {
		appLogger.Fatalf("Failed to migrate database: %v", err)
	}

	savedVideoService := services.NewSavedVideoService(dbConn)
	taskStepService := services.NewTaskStepService(dbConn)
	biliAccountService := services.NewBiliAccountService(dbConn) // 只需要 db
	auditService := audit.NewAuditService(dbConn)
	userConfigService := services.NewUserConfigService(dbConn, appLogger) // 需要 db 和 logger

	// 创建 AppServer 实例
	server := core.NewServer(cfg, appLogger) // 使用 NewServer 而不是 NewAppServer
	server.Init(dbConn)

	// 初始化中间件（使用 go-auth 适配器）
	jwtConfig := auth.JWTConfig{
		SecretKey:     cfg.Auth.JWTSecret,
		Issuer:        "ytb2bili",
		AccessExpiry:  time.Duration(cfg.Auth.JWTExpiration) * time.Hour,
		RefreshExpiry: time.Duration(cfg.Auth.JWTExpiration*7) * time.Hour,
	}
	jwtService := auth.NewGoAuthJWTService(jwtConfig) // 使用 go-auth 适配器
	authMiddleware := auth.NewAuthMiddleware(dbConn, jwtService)
	// 初始化 Cron 任务调度
	cronTask := cron.New(cron.WithSeconds())

	// 初始化 Handlers
	authHandler := handler.NewAuthHandler(server, biliAccountService) // app, biliAccountService
	videoHandler := handler.NewVideoHandler(server, savedVideoService, taskStepService, auditService)
	subtitleHandler := handler.NewSubtitleHandler(server)
	biliAccountHandler := handler.NewBiliAccountHandler(server, biliAccountService, auditService)
	uploadHandler := handler.NewUploadHandler(server) // 只需要 app
	configHandler := handler.NewConfigHandler(server) // 只需要 app
	userConfigHandler := handler.NewUserConfigHandler(server, userConfigService)
	cronHandler := handler.NewCronHandler(server, dbConn, cronTask)

	// 初始化邮件服务（用于发送验证码）
	emailService := services.NewEmailService(cfg.SMTPConfig)

	// 注册用户认证路由（邮箱/密码登录、注册等）
	userAuthHandler := auth.NewAuthHandler(dbConn, jwtService, emailService, auditService)
	userAuthHandler.RegisterRoutes(server.Engine.Group("/api/v1"))

	// 初始化链式任务处理器
	chainTaskHandler := chain_task.NewChainTaskHandler(
		server, cronTask, dbConn,
		savedVideoService, taskStepService, biliAccountService, auditService,
	)

	// 设置 VideoHandler 和 SubtitleHandler 的 CancelManager
	videoHandler.SetCancelManager(chainTaskHandler.CancelManager)
	subtitleHandler.SetCancelManager(chainTaskHandler.CancelManager)

	// 启动链式任务处理器
	chainTaskHandler.SetUp()

	// 初始化上传调度器
	uploadScheduler := chain_task.NewUploadScheduler(
		server, cronTask, dbConn,
		savedVideoService, taskStepService, biliAccountService, auditService,
		chainTaskHandler.CancelManager,
	)

	// 注入 UploadScheduler 到 VideoHandler
	videoHandler.SetUploadScheduler(uploadScheduler)

	// 启动上传调度器
	uploadScheduler.SetUp()

	// 启动 Cron Handler
	cronHandler.SetUp()

	// 注册 API 路由
	authHandler.RegisterRoutes(server, authMiddleware)
	videoHandler.RegisterRoutes(server.Engine.Group("/api/v1"), authMiddleware)
	subtitleHandler.RegisterRoutes(server, authMiddleware)
	biliAccountHandler.RegisterRoutes(server, authMiddleware)
	uploadHandler.RegisterRoutes(server, authMiddleware)
	configHandler.RegisterRoutes(server, authMiddleware)
	userConfigHandler.RegisterRoutes(server, authMiddleware)

	// HTTP 服务器配置
	srv := &http.Server{
		Addr:    cfg.Listen,
		Handler: server.Engine,
	}

	// 启动服务器
	go func() {
		appLogger.Infof("Server is running on %s", cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Fatalf("listen: %s\n", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	appLogger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		appLogger.Fatal("Server forced to shutdown:", err)
	}

	appLogger.Info("Server exiting")
}
