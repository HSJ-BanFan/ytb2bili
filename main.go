package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/difyz9/ytb2bili/internal/auth"
	"github.com/difyz9/ytb2bili/internal/chain_task"
	"github.com/difyz9/ytb2bili/internal/chain_task/handlers"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/difyz9/ytb2bili/internal/handler"
	"github.com/difyz9/ytb2bili/internal/membership"
	"github.com/difyz9/ytb2bili/internal/web"
	"github.com/difyz9/ytb2bili/pkg/analytics"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/logger"
	"github.com/difyz9/ytb2bili/pkg/store"
	"github.com/difyz9/ytb2bili/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AppLifecycle 应用程序生命周期
type AppLifecycle struct {
}

// OnStart 应用程序启动时执行
func (l *AppLifecycle) OnStart(context.Context) error {
	log.Println("AppLifecycle OnStart")
	return nil
}

// OnStop 应用程序停止时执行
func (l *AppLifecycle) OnStop(context.Context) error {
	log.Println("AppLifecycle OnStop")
	return nil
}

// testGeminiConnection 测试 Gemini API 连接
func testGeminiConnection(config *types.AppConfig, logger *zap.SugaredLogger) error {
	// 使用轮询 API Key
	apiKey := config.GeminiConfig.GetCurrentApiKey()
	keyCount := config.GeminiConfig.GetApiKeysCount()
	if keyCount > 1 {
		logger.Infof("│  🔑 使用 API Key 轮询 (%d 个密钥)", keyCount)
	}

	client, err := handlers.NewGeminiClient(
		apiKey,
		config.GeminiConfig.Model,
		config.GeminiConfig.Timeout,
		config.GeminiConfig.MaxTokens,
	)
	if err != nil {
		return fmt.Errorf("创建客户端失败: %v", err)
	}
	defer client.Close()

	// 增加超时时间到 30 秒，因为首次连接可能较慢
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = client.TestConnection(ctx)
	if err != nil {
		return fmt.Errorf("连接测试失败: %v", err)
	}
	return nil
}

func main() {

	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.toml"
	}

	// 加载配置
	config, err := types.LoadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}
	config.Path = configFile

	app := fx.New(
		// 初始化配置应用配置
		fx.Provide(func() *types.AppConfig {
			return config
		}),

		// 日志模块
		fx.Provide(func(config *types.AppConfig) (*zap.SugaredLogger, error) {
			return logger.NewLogger(config.Debug)
		}),

		// 数据库模块
		fx.Provide(store.NewDatabase),

		// 核心模块
		fx.Provide(core.NewServer),
		fx.Provide(cos.NewCosClient),

		// 分析客户端
		fx.Provide(func(config *types.AppConfig, logger *zap.SugaredLogger) (*analytics.Client, error) {
			if config.AnalyticsConfig == nil || !config.AnalyticsConfig.Enabled {
				logger.Info("Analytics is disabled")
				return nil, nil
			}

			analyticsConfig := &analytics.Config{
				ServerURL:     config.AnalyticsConfig.ServerURL,
				APIKey:        config.AnalyticsConfig.APIKey,
				ProductID:     config.AnalyticsConfig.ProductID,
				Debug:         config.AnalyticsConfig.Debug,
				EncryptionKey: config.AnalyticsConfig.EncryptionKey,
			}

			return analytics.NewClient(analyticsConfig, logger)
		}),

		// 分析中间件
		fx.Provide(func(client *analytics.Client, logger *zap.SugaredLogger) *analytics.Middleware {
			return analytics.NewMiddleware(client, logger)
		}),

		// 服务层
		fx.Provide(services.NewVideoService),
		fx.Provide(services.NewSavedVideoService),
		fx.Provide(services.NewTaskStepService),
		fx.Provide(services.NewBiliAccountService),

		// 认证系统
		fx.Provide(func() *auth.JWTService {
			return auth.NewJWTService(auth.DefaultJWTConfig())
		}),
		fx.Provide(auth.NewAuthMiddleware),
		fx.Provide(auth.NewAuthHandler),

		// 会员系统
		fx.Provide(func(db *gorm.DB) membership.MembershipStore {
			return membership.NewDBMembershipStore(db)
		}),
		fx.Provide(membership.NewMembershipHandler),
		fx.Provide(membership.NewMembershipMiddleware),

		// 注册cron
		fx.Provide(func() *cron.Cron {
			return cron.New(cron.WithSeconds())
		}),

		fx.Provide(handler.NewCronHandler),
		fx.Invoke(func(h *handler.CronHandler) {
			h.SetUp()
		}),

		// 生命周期管理
		fx.Provide(func() *AppLifecycle {
			return &AppLifecycle{}
		}),

		// 初始化数据库
		fx.Invoke(func(db *gorm.DB, logger *zap.SugaredLogger) error {
			logger.Info("Running database migrations...")
			return store.MigrateDatabase(db)
		}),

		// 初始化并检查 yt-dlp
		fx.Invoke(func(logger *zap.SugaredLogger, config *types.AppConfig) error {
			logger.Info("Checking yt-dlp installation...")
			return checkYtDlpInstallation(logger, config)
		}),

		fx.Provide(chain_task.NewChainTaskHandler),
		fx.Invoke(func(h *chain_task.ChainTaskHandler) {
			// 设置并启动任务消费者（准备阶段：下载、字幕、翻译、元数据）
			h.SetUp()
		}),

		// 添加上传调度器
		fx.Provide(chain_task.NewUploadScheduler),
		fx.Invoke(func(s *chain_task.UploadScheduler) {
			// 设置并启动上传调度器（上传阶段：每小时上传视频，1小时后上传字幕）
			s.SetUp()
		}),

		// 初始化应用服务器和基础路由
		fx.Invoke(func(
			server *core.AppServer,
			db *gorm.DB,
			logger *zap.SugaredLogger,
			savedVideoService *services.SavedVideoService,
			taskStepService *services.TaskStepService,
			uploadScheduler *chain_task.UploadScheduler,
			analyticsMiddleware *analytics.Middleware,
			analyticsClient *analytics.Client,
			membershipHandler *membership.MembershipHandler,
			membershipStore membership.MembershipStore,
			authHandler *auth.AuthHandler,
			authMiddleware *auth.AuthMiddleware,
			biliAccountService *services.BiliAccountService,
		) {
			// 初始化服务器
			server.Init(db)

			// 添加分析中间件
			if analyticsMiddleware != nil {
				server.Engine.Use(analyticsMiddleware.Handler())
				logger.Info("Analytics middleware registered")
			}

			// 注册所有 Handler 路由（包括连接 VideoHandler 和 UploadScheduler）
			registerHandlers(server, logger, savedVideoService, taskStepService, uploadScheduler, analyticsClient, membershipHandler, membershipStore, authHandler, authMiddleware, biliAccountService)

			// 健康检查
			server.Engine.GET("/health", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"status":  "ok",
					"message": "Bili Up Backend API is running",
					"time":    time.Now().Format(time.RFC3339),
				})
			})

			// 静态文件服务 (嵌入的前端文件)
			logger.Info("Setting up embedded static file server...")
			staticHandler := web.StaticFileHandler()

			// 对于根路径和非 API 路径，提供静态文件
			server.Engine.NoRoute(func(c *gin.Context) {
				path := c.Request.URL.Path
				// 如果不是 API 路径，提供静态文件
				if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/health") {
					staticHandler.ServeHTTP(c.Writer, c.Request)
					return
				}
				// 否则返回 404
				c.JSON(404, gin.H{
					"code":    404,
					"message": "API endpoint not found",
				})
			})

			logger.Info("✓ Static file server configured")

		}),
		fx.Invoke(func(s *core.AppServer, db *gorm.DB) {
			go func() {
				err := s.Run()
				if err != nil {
					os.Exit(0)
				}
			}()
		}),
		// 注册生命周期回调函数
		fx.Invoke(func(lifecycle fx.Lifecycle, lc *AppLifecycle, config *types.AppConfig, logger *zap.SugaredLogger) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					// 显示 AI 服务配置状态
					logger.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
					logger.Info("🤖 AI 服务配置检查")
					logger.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

					// 1. 检查 OpenAI 兼容 API（用户首选）
					if config.OpenAICompatibleConfig != nil && config.OpenAICompatibleConfig.Enabled && config.OpenAICompatibleConfig.ApiKey != "" {
						providerName := "自定义API"
						switch config.OpenAICompatibleConfig.Provider {
						case "openai":
							providerName = "OpenAI"
						case "deepseek":
							providerName = "DeepSeek (兼容模式)"
						case "qwen":
							providerName = "通义千问"
						case "zhipu":
							providerName = "智谱AI"
						case "gemini":
							providerName = "Gemini (代理)"
						case "custom":
							providerName = "自定义API"
						}
						logger.Info("┌─ 🌟 首选 AI 服务（用户配置）")
						logger.Infof("│  📦 提供商: %s", providerName)
						logger.Infof("│  🔧 模型: %s", config.OpenAICompatibleConfig.Model)
						logger.Infof("│  🌐 API地址: %s", config.OpenAICompatibleConfig.BaseURL)
						if len(config.OpenAICompatibleConfig.ApiKey) > 10 {
							logger.Infof("│  🔑 API Key: %s...%s",
								config.OpenAICompatibleConfig.ApiKey[:6],
								config.OpenAICompatibleConfig.ApiKey[len(config.OpenAICompatibleConfig.ApiKey)-4:])
						}
						logger.Info("└─ ✅ 已启用为首选服务")
					} else {
						logger.Info("│  ⚪ OpenAI兼容API: 未配置")
					}

					// 2. 检查 DeepSeek
					if config.DeepSeekTransConfig != nil && config.DeepSeekTransConfig.Enabled && config.DeepSeekTransConfig.ApiKey != "" {
						logger.Info("┌─ 📘 DeepSeek 服务")
						logger.Infof("│  🔧 模型: %s", config.DeepSeekTransConfig.Model)
						if len(config.DeepSeekTransConfig.ApiKey) > 10 {
							logger.Infof("│  🔑 API Key: %s...%s",
								config.DeepSeekTransConfig.ApiKey[:6],
								config.DeepSeekTransConfig.ApiKey[len(config.DeepSeekTransConfig.ApiKey)-4:])
						}
						if config.OpenAICompatibleConfig == nil || !config.OpenAICompatibleConfig.Enabled {
							logger.Info("└─ ✅ 已启用为首选服务")
						} else {
							logger.Info("└─ ✅ 已启用为备选服务")
						}
					} else {
						logger.Info("│  ⚪ DeepSeek: 未配置")
					}

					// 3. 检查 Gemini（原生多模态）
					geminiHasKey := config.GeminiConfig != nil && config.GeminiConfig.Enabled &&
						(config.GeminiConfig.ApiKey != "" || len(config.GeminiConfig.ApiKeys) > 0)
					if geminiHasKey {
						logger.Info("┌─ 🔮 Gemini 原生多模态服务")
						logger.Infof("│  🔧 模型: %s", config.GeminiConfig.Model)
						logger.Infof("│  🔑 使用 API Key 轮询 (%d 个密钥)", config.GeminiConfig.GetApiKeysCount())
						logger.Infof("│  🎬 视频分析: %v", config.GeminiConfig.AnalyzeVideo)
						logger.Infof("│  📝 用于元数据: %v", config.GeminiConfig.UseForMetadata)
						logger.Info("└─ ✅ 已配置 (请在设置页面验证 API Key)")
					} else {
						logger.Info("│  ⚪ Gemini原生: 未配置")
					}

					// 显示当前首选服务（用户选择）
					logger.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
					primaryService := config.PrimaryAIService
					if primaryService == "" {
						// 如果用户未选择，自动选择第一个启用的服务
						if config.OpenAICompatibleConfig != nil && config.OpenAICompatibleConfig.Enabled && config.OpenAICompatibleConfig.ApiKey != "" {
							primaryService = "openai_compatible"
						} else if config.DeepSeekTransConfig != nil && config.DeepSeekTransConfig.Enabled && config.DeepSeekTransConfig.ApiKey != "" {
							primaryService = "deepseek"
						} else if config.GeminiConfig != nil && config.GeminiConfig.Enabled &&
							(config.GeminiConfig.ApiKey != "" || len(config.GeminiConfig.ApiKeys) > 0) {
							primaryService = "gemini"
						}
					}

					// 显示翻译服务
					switch primaryService {
					case "openai_compatible":
						providerName := "自定义API"
						if config.OpenAICompatibleConfig != nil {
							switch config.OpenAICompatibleConfig.Provider {
							case "openai":
								providerName = "OpenAI"
							case "deepseek":
								providerName = "DeepSeek (兼容模式)"
							case "qwen":
								providerName = "通义千问"
							case "zhipu":
								providerName = "智谱AI"
							case "gemini":
								providerName = "Gemini (代理)"
							}
						}
						logger.Infof("🎯 翻译服务: %s (用户选择)", providerName)
					case "deepseek":
						logger.Info("🎯 翻译服务: DeepSeek (用户选择)")
					case "gemini":
						logger.Info("🎯 翻译服务: Gemini (用户选择)")
					default:
						logger.Warn("⚠️ 翻译服务: 未配置")
						logger.Warn("💡 请在设置页面配置并选择首选 AI 服务")
					}

					// 显示元数据生成服务（固定使用 Gemini）
					if geminiHasKey {
						logger.Infof("🎯 元数据生成: Gemini 原生多模态 (固定)")
						logger.Infof("   视频分析: %v, 模型: %s", config.GeminiConfig.AnalyzeVideo, config.GeminiConfig.Model)
					} else {
						logger.Warn("⚠️ 元数据生成: 需要配置 Gemini！")
						logger.Warn("💡 Gemini 具有多模态视频分析能力，是生成高质量元数据的最佳选择")
					}
					logger.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

					return lc.OnStart(ctx)
				},
				OnStop: func(ctx context.Context) error {
					return lc.OnStop(ctx)
				},
			})
		}),
	)

	// 启动应用程序
	go func() {

		if err := app.Start(context.Background()); err != nil {
			log.Fatal(err)
		}

	}()

	// 监听退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down gracefully...")

	// 关闭应用程序
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Stop(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("✅ Application stopped")

}

// registerHandlers 注册所有 Handler 路由
func registerHandlers(
	server *core.AppServer,
	logger *zap.SugaredLogger,
	savedVideoService *services.SavedVideoService,
	taskStepService *services.TaskStepService,
	uploadScheduler *chain_task.UploadScheduler,
	analyticsClient *analytics.Client,
	membershipHandler *membership.MembershipHandler,
	membershipStore membership.MembershipStore,
	authHandler *auth.AuthHandler,
	authMiddleware *auth.AuthMiddleware,
	biliAccountService *services.BiliAccountService,
) {
	logger.Info("Registering handlers...")

	// 旧的认证 Handler (B站扫码登录)
	oldFeatureChecker := membership.NewFeatureChecker(membershipStore)
	oldAuthHandler := handler.NewAuthHandler(server, oldFeatureChecker, biliAccountService)
	oldAuthHandler.RegisterRoutes(server, authMiddleware)
	logger.Info("✓ Bilibili Auth routes registered")

	// 新的认证 Handler (JWT + App 认证)
	authHandler.RegisterRoutes(server.Engine.Group("/api/v1"))
	logger.Info("✓ JWT Auth routes registered")

	// 上传 Handler
	uploadHandler := handler.NewUploadHandler(server)
	uploadHandler.RegisterRoutes(server)
	logger.Info("✓ Upload routes registered")

	// 分类 Handler
	categoryHandler := handler.NewCategoryHandler(server)
	categoryHandler.RegisterRoutes(server)
	logger.Info("✓ Category routes registered")

	// 字幕 Handler（需要 JWT 认证）
	subtitleHandler := handler.NewSubtitleHandler(server, membershipStore, authMiddleware.JWTAuth)
	subtitleHandler.RegisterRoutes(server)
	logger.Info("✓ Subtitle routes registered (JWT protected)")

	// 分析 Handler
	analyticsHandler := handler.NewAnalyticsHandler(analyticsClient, logger)

	// 视频 Handler
	videoHandler := handler.NewVideoHandler(server, savedVideoService, taskStepService)
	// 设置分析处理器
	videoHandler.AnalyticsHandler = analyticsHandler
	// 设置上传调度器（避免循环依赖）
	videoHandler.SetUploadScheduler(uploadScheduler)
	videoHandler.RegisterRoutes(server.Engine.Group("/api/v1"))
	logger.Info("✓ Video routes registered")

	// 迁移：为所有视频添加"上传字幕到Bilibili"步骤
	if count, err := taskStepService.MigrateAllVideosSubtitleStep(); err != nil {
		logger.Warnf("迁移字幕上传步骤失败: %v", err)
	} else if count > 0 {
		logger.Infof("✓ 已为 %d 个视频添加字幕上传步骤", count)
	}

	// 配置 Handler
	configHandler := handler.NewConfigHandler(server)
	configHandler.RegisterRoutes(server)
	logger.Info("✓ Config routes registered")

	// 会员 Handler（使用可选 JWT 中间件获取用户 ID）
	membershipGroup := server.Engine.Group("/api/v1")
	membershipGroup.Use(authMiddleware.OptionalJWTAuth())
	membershipHandler.RegisterRoutes(membershipGroup)
	logger.Info("✓ Membership routes registered")

	// B站账号管理 Handler
	featureChecker := membership.NewFeatureChecker(membershipStore)
	biliAccountHandler := handler.NewBiliAccountHandler(server, biliAccountService, featureChecker)
	biliAccountHandler.RegisterRoutes(server, authMiddleware)
	logger.Info("✓ Bili Account routes registered")

	logger.Info("All handlers registered successfully")
}

// checkYtDlpInstallation 检查并自动安装 yt-dlp
func checkYtDlpInstallation(logger *zap.SugaredLogger, config *types.AppConfig) error {
	// 从配置中获取安装目录，如果未配置则使用默认值
	var installDir string
	if config != nil && config.YtDlpPath != "" {
		installDir = config.YtDlpPath
	}

	// 创建 yt-dlp 管理器
	manager := utils.NewYtDlpManager(logger, installDir)

	// 检查并自动安装
	if err := manager.CheckAndInstall(); err != nil {
		logger.Errorf("❌ yt-dlp 检查/安装失败: %v", err)
		logger.Warn("⚠️  视频下载功能可能无法正常工作")
		logger.Info("💡 您可以手动安装 yt-dlp:")
		logger.Info("   macOS: brew install yt-dlp")
		logger.Info("   Windows: winget install yt-dlp")
		logger.Info("   Linux: pip install yt-dlp")
		return nil // 不阻止应用启动
	}

	// 验证安装
	if err := manager.Validate(); err != nil {
		logger.Errorf("❌ yt-dlp 验证失败: %v", err)
		return nil // 不阻止应用启动
	}

	logger.Infof("✅ yt-dlp 就绪，路径: %s", manager.GetBinaryPath())
	return nil
}
