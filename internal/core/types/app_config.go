package types

import (
	"bytes"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// AppConfig 应用程序配置
type AppConfig struct {
	Path        string        `toml:"-"`
	Listen      string        `toml:"listen"`
	Environment string        `toml:"environment"`
	Debug       bool          `toml:"debug"`
	Database    Database      `toml:"database"`
	Auth        AuthConfig    `toml:"auth"`
	AppAuth     AppAuthConfig `toml:"app_auth"` // 应用启动认证配置
	FileUpDir   string        `toml:"fileUpDir"`
	YtDlpPath   string        `toml:"yt_dlp_path"` // yt-dlp 安装路径

	TenCosConfig           *TencentCosConfig       `toml:"TenCosConfig"`           // 腾讯云 COS 存储配置
	BaiduTransConfig       *BaiduTransConfig       `toml:"BaiduTransConfig"`       // 百度翻译服务配置
	DeepSeekTransConfig    *DeepSeekTransConfig    `toml:"DeepSeekTransConfig"`    // DeepSeek翻译服务配置
	GeminiConfig           *GeminiConfig           `toml:"GeminiConfig"`           // Gemini多模态服务配置
	OpenAICompatibleConfig *OpenAICompatibleConfig `toml:"OpenAICompatibleConfig"` // OpenAI兼容API配置
	TranslatorConfig       *TranslatorConfig       `toml:"TranslatorConfig"`       // 翻译器总配置
	ProxyConfig            *ProxyConfig            `toml:"ProxyConfig"`            // 代理配置
	DownloadConfig         *DownloadConfig         `toml:"DownloadConfig"`         // 下载配置
	AnalyticsConfig        *AnalyticsConfig        `toml:"AnalyticsConfig"`        // 数据分析配置
	BilibiliConfig         *BilibiliConfig         `toml:"BilibiliConfig"`         // Bilibili上传配置
	MembershipConfig       *MembershipConfig       `toml:"MembershipConfig"`       // 会员系统配置
	SMTPConfig             *SMTPConfig             `toml:"SMTPConfig"`             // SMTP邮件服务配置

	// AI服务选择配置
	PrimaryAIService string `toml:"primary_ai_service"` // 用户选择的首选AI服务: openai_compatible, deepseek, gemini
}

// MembershipConfig 会员系统配置
type MembershipConfig struct {
	Enabled bool        `toml:"enabled"` // 是否启用会员系统
	Redis   RedisConfig `toml:"redis"`   // Redis 配置
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Addr     string `toml:"addr"`     // Redis 地址 (例如: localhost:6379)
	Password string `toml:"password"` // Redis 密码
	DB       int    `toml:"db"`       // Redis 数据库编号
}

// BilibiliConfig Bilibili上传配置
type BilibiliConfig struct {
	Copyright           int    `toml:"copyright"`             // 1=自制, 2=转载
	Source              string `toml:"source"`                // 转载来源（当 Copyright=2 时必填）
	NoReprint           int    `toml:"no_reprint"`            // 0=允许转载, 1=禁止转载
	UseOriginalTitle    bool   `toml:"use_original_title"`    // true=使用原视频标题, false=使用AI生成标题
	UseOriginalDesc     bool   `toml:"use_original_desc"`     // true=使用原视频描述, false=使用AI生成描述
	CustomTitleTemplate string `toml:"custom_title_template"` // 自定义标题模板，支持变量: {original_title}, {ai_title}
	CustomDescTemplate  string `toml:"custom_desc_template"`  // 自定义描述模板，支持变量: {original_desc}, {ai_desc}

	// 新增配置项
	Tid              int    `toml:"tid"`                // 分区ID（默认122，可自定义）
	Dynamic          string `toml:"dynamic"`            // 动态文本（默认"发布了新视频！"）
	OpenElec         int    `toml:"open_elec"`          // 是否开启充电面板 0=关闭, 1=开启
	SelectionReserve int64  `toml:"selection_reserve"`  // 参与活动ID（0表示不参与）
	UpSelectionReply int    `toml:"up_selection_reply"` // 是否展示推荐评论 0=关闭, 1=开启
	UpCloseReply     int    `toml:"up_close_reply"`     // 是否关闭评论 0=开启评论, 1=关闭评论
	UpCloseReward    int    `toml:"up_close_reward"`    // 是否关闭打赏 0=开启, 1=关闭
}

type TencentCosConfig struct {
	Enabled      bool // 是否启用腾讯云 COS 存储
	CosBucketURL string
	CosSecretId  string
	CosSecretKey string
	CosRegion    string
	CosBucket    string
	SubAppId     string
	CosUrL       string
}

// Database 数据库配置
type Database struct {
	Type     string `toml:"type"`     // postgres, mysql, sqlite
	Host     string `toml:"host"`     // 对于 sqlite，这是数据库文件路径
	Port     int    `toml:"port"`     // sqlite 不需要
	Username string `toml:"username"` // sqlite 不需要
	Password string `toml:"password"` // sqlite 不需要
	Database string `toml:"database"` // 对于 sqlite，这是文件名
	SSLMode  string `toml:"ssl_mode"` // sqlite 不需要
	Timezone string `toml:"timezone"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	JWTSecret     string `toml:"jwt_secret"`
	JWTExpiration int    `toml:"jwt_expiration"` // 小时
	SessionSecret string `toml:"session_secret"`
}

// AppAuthConfig 应用启动认证配置
type AppAuthConfig struct {
	Enabled       bool   `toml:"enabled"`        // 是否启用应用认证
	APIURL        string `toml:"api_url"`        // 认证API地址
	AppID         string `toml:"app_id"`         // 应用ID
	AppSecret     string `toml:"app_secret"`     // 应用密钥
	CheckInterval int    `toml:"check_interval"` // 定期检查间隔（分钟），0表示只在启动时检查
	SkipOnError   bool   `toml:"skip_on_error"`  // 认证失败时是否跳过（开发环境可设置为true）
}

// GetDSN 获取数据库连接字符串
func (d Database) GetDSN() string {
	switch d.Type {
	case "postgres", "postgresql":
		return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
			d.Host, d.Username, d.Password, d.Database, d.Port, d.SSLMode, d.Timezone)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			d.Username, d.Password, d.Host, d.Port, d.Database)
	case "sqlite", "sqlite3":
		// SQLite 数据库文件路径
		if d.Host != "" {
			// 如果指定了 host，使用 host 作为完整路径
			return d.Host
		}
		// 否则使用 database 作为文件名，存储在当前目录
		if d.Database == "" {
			d.Database = "bili_up.db"
		}
		return d.Database
	default:
		return ""
	}
}

// BaiduTransConfig 百度翻译服务配置
type BaiduTransConfig struct {
	Enabled   bool   `toml:"enabled"`    // 是否启用翻译服务
	AppId     string `toml:"app_id"`     // 百度翻译AppID
	SecretKey string `toml:"secret_key"` // 百度翻译密钥
	Endpoint  string `toml:"endpoint"`   // API端点
}

// DeepSeekTransConfig DeepSeek翻译服务配置
type DeepSeekTransConfig struct {
	Enabled   bool   `toml:"enabled"`    // 是否启用翻译服务
	ApiKey    string `toml:"api_key"`    // DeepSeek API密钥
	Model     string `toml:"models"`     // 使用的模型，默认为 deepseek-chat
	Endpoint  string `toml:"endpoint"`   // API端点，默认为 https://api.deepseek.com
	Timeout   int    `toml:"timeout"`    // 超时时间（秒）
	MaxTokens int    `toml:"max_tokens"` // 最大token数
}

// GeminiConfig Gemini多模态服务配置
type GeminiConfig struct {
	Enabled           bool     `toml:"enabled"`             // 是否启用Gemini服务
	ApiKey            string   `toml:"api_key"`             // Google AI API密钥（主密钥，兼容旧配置）
	ApiKeys           []string `toml:"api_keys"`            // 多个API密钥，用于轮询（优先使用）
	CurrentKeyIndex   int      `toml:"-"`                   // 当前使用的密钥索引（运行时状态，不保存）
	Model             string   `toml:"model"`               // 使用的模型，默认为 gemini-1.5-pro
	Timeout           int      `toml:"timeout"`             // 超时时间（秒）
	MaxTokens         int      `toml:"max_tokens"`          // 最大输出token数
	UseForMetadata    bool     `toml:"use_for_metadata"`    // 是否使用Gemini生成元数据（优先于DeepSeek）
	AnalyzeVideo      bool     `toml:"analyze_video"`       // 是否分析视频文件（true=多模态，false=仅文本）
	VideoSampleFrames int      `toml:"video_sample_frames"` // 视频采样帧数（0=上传完整视频）
}

// GetCurrentApiKey 获取当前使用的API密钥
func (g *GeminiConfig) GetCurrentApiKey() string {
	// 优先使用 ApiKeys 数组
	if len(g.ApiKeys) > 0 {
		index := g.CurrentKeyIndex % len(g.ApiKeys)
		return g.ApiKeys[index]
	}
	// 兼容旧配置，使用单个 ApiKey
	return g.ApiKey
}

// RotateApiKey 轮换到下一个API密钥
func (g *GeminiConfig) RotateApiKey() string {
	if len(g.ApiKeys) > 1 {
		g.CurrentKeyIndex = (g.CurrentKeyIndex + 1) % len(g.ApiKeys)
	}
	return g.GetCurrentApiKey()
}

// GetApiKeysCount 获取API密钥数量
func (g *GeminiConfig) GetApiKeysCount() int {
	if len(g.ApiKeys) > 0 {
		return len(g.ApiKeys)
	}
	if g.ApiKey != "" {
		return 1
	}
	return 0
}

// OpenAICompatibleConfig OpenAI兼容API配置
// 支持任何兼容OpenAI API格式的服务
type OpenAICompatibleConfig struct {
	Enabled     bool    `toml:"enabled"`     // 是否启用
	Provider    string  `toml:"provider"`    // 提供商标识: openai, deepseek, qwen, zhipu, gemini, custom
	ApiKey      string  `toml:"api_key"`     // API密钥
	BaseURL     string  `toml:"base_url"`    // API基础URL
	Model       string  `toml:"model"`       // 使用的模型
	Timeout     int     `toml:"timeout"`     // 超时时间（秒）
	MaxTokens   int     `toml:"max_tokens"`  // 最大token数
	Temperature float64 `toml:"temperature"` // 温度参数 (0-2)
}

// TranslatorConfig 翻译器总配置
type TranslatorConfig struct {
	DefaultProvider   string   `toml:"default_provider"`   // 默认翻译提供商
	FallbackProviders []string `toml:"fallback_providers"` // 备选翻译提供商
	MaxRetries        int      `toml:"max_retries"`        // 最大重试次数
	Timeout           int      `toml:"timeout"`            // 超时时间（秒）
	EnableCache       bool     `toml:"enable_cache"`       // 是否启用缓存
	CacheExpiry       int      `toml:"cache_expiry"`       // 缓存过期时间（秒）
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	UseProxy  bool   `toml:"use_proxy"`  // 是否使用代理
	ProxyHost string `toml:"proxy_host"` // 代理地址 (例如: http://127.0.0.1:7890)
}

// DownloadConfig 下载配置
type DownloadConfig struct {
	UseAria2c              bool   `toml:"use_aria2c"`               // 是否使用 aria2c 多线程下载
	Aria2cPath             string `toml:"aria2c_path"`              // aria2c 可执行文件路径
	ConcurrentFragments    int    `toml:"concurrent_fragments"`     // 并发分片数（默认8）
	Aria2cConnections      int    `toml:"aria2c_connections"`       // aria2c 连接数（默认8，降低以避免资源竞争）
	HttpChunkSize          string `toml:"http_chunk_size"`          // HTTP 分块大小（默认10M）
	PreferFormat           string `toml:"prefer_format"`            // 首选格式: best, 1080p, 720p, 480p
	Aria2cWithProxy        bool   `toml:"aria2c_with_proxy"`        // 使用代理时是否仍尝试 aria2c（可能遇到403）
	CookiesFromBrowser     string `toml:"cookies_from_browser"`     // 从浏览器提取cookies: chrome/firefox/edge/brave/opera/safari，空字符串禁用
	MaxConcurrentTasks     int    `toml:"max_concurrent_tasks"`     // 准备阶段最大并发任务数（默认10）
	CookiesRefreshInterval int    `toml:"cookies_refresh_interval"` // Cookies 刷新间隔（分钟），默认30，0表示禁用

	// 下载并发控制配置（新增）
	MaxConcurrentDownloads int `toml:"max_concurrent_downloads"` // 最大并发下载数（默认3）
	LongVideoThreshold     int `toml:"long_video_threshold"`     // 长视频阈值（分钟，默认30）
	DownloadTimeout        int `toml:"download_timeout"`         // 下载超时时间（分钟，默认120）

	// 自动上传配置
	AutoUploadEnabled   bool   `toml:"auto_upload_enabled"`   // 是否启用自动上传（默认true）
	AutoUploadMode      string `toml:"auto_upload_mode"`      // 上传模式: immediate=立即, delayed=延迟（默认delayed）
	VideoUploadDelay    int    `toml:"video_upload_delay"`    // 视频处理完成后延迟上传时间（分钟，默认10）
	SubtitleUploadDelay int    `toml:"subtitle_upload_delay"` // 视频上传后字幕延迟上传时间（分钟，默认10）
	UploadCheckInterval int    `toml:"upload_check_interval"` // 上传调度器检查间隔（秒，默认10）

	// 并发上传配置（功能开关）
	EnableFineGrainedLock bool `toml:"enable_fine_grained_lock"` // 是否启用细粒度锁（默认true）- true=使用事务锁，false=使用全局锁（兼容模式）
	MaxConcurrentUploads  int  `toml:"max_concurrent_uploads"`   // 最大并发上传数（默认5，防止资源耗尽）

	// 自动清理配置
	AutoCleanupEnabled bool   `toml:"auto_cleanup_enabled"` // 是否启用自动清理（默认false）
	AutoCleanupMode    string `toml:"auto_cleanup_mode"`    // 清理模式: immediate=立即, delayed=延迟（默认delayed）
	AutoCleanupDelay   int    `toml:"auto_cleanup_delay"`   // 延迟清理时间（分钟，默认60）

	// 字幕翻译配置
	SubtitleTranslationEnabled bool `toml:"subtitle_translation_enabled"` // 是否启用字幕翻译（默认true）
}

// AnalyticsConfig 数据分析配置
type AnalyticsConfig struct {
	Enabled       bool   `toml:"enabled"`        // 是否启用数据分析
	ServerURL     string `toml:"server_url"`     // 分析服务器地址
	APIKey        string `toml:"api_key"`        // API密钥
	ProductID     string `toml:"product_id"`     // 产品ID
	Debug         bool   `toml:"debug"`          // 是否启用调试模式
	EncryptionKey string `toml:"encryption_key"` // AES加密密钥（可选，16/24/32字节）
}

// SMTPConfig SMTP邮件服务配置
type SMTPConfig struct {
	Enabled  bool   `toml:"enabled"`   // 是否启用邮件服务
	Host     string `toml:"host"`      // SMTP 主机 (如: smtp.gmail.com:587)
	From     string `toml:"from"`      // 发件人邮箱
	FromName string `toml:"from_name"` // 发件人名称
	Username string `toml:"username"`  // SMTP 用户名
	Password string `toml:"-"`         // SMTP 密码（从环境变量读取，不写入toml）
	UseTLS   bool   `toml:"use_tls"`   // 是否使用 TLS
}

// NewDefaultConfig 创建默认配置
func NewDefaultConfig() *AppConfig {
	return &AppConfig{
		Listen:      ":8096",
		Environment: "development",
		Debug:       true,
		Database: Database{
			Type:     "postgres",
			Host:     "localhost",
			Port:     5432,
			Username: "postgres",
			Password: "password",
			Database: "bili_up_db",
			SSLMode:  "disable",
			Timezone: "Asia/Shanghai",
		},

		Auth: AuthConfig{
			JWTSecret:     "your-jwt-secret-key",
			JWTExpiration: 24,
			SessionSecret: "your-session-secret",
		},

		// 腾讯云 COS 配置（默认值，可被 config.toml 覆盖）
		TenCosConfig: &TencentCosConfig{
			Enabled:      false,
			CosBucketURL: "",
			CosSecretId:  "",
			CosSecretKey: "",
			CosRegion:    "",
			CosBucket:    "",
			SubAppId:     "",
			CosUrL:       "",
		},

		// DeepSeek 翻译配置（默认值，可被 config.toml 覆盖）
		DeepSeekTransConfig: &DeepSeekTransConfig{
			Enabled:   false,
			ApiKey:    "",
			Model:     "deepseek-chat",
			Endpoint:  "https://api.deepseek.com",
			Timeout:   60,
			MaxTokens: 4000,
		},

		// Gemini 多模态配置（默认值，可被 config.toml 覆盖）
		GeminiConfig: &GeminiConfig{
			Enabled:           false,
			ApiKey:            "",
			Model:             "gemini-2.5-flash",
			Timeout:           120,
			MaxTokens:         8000,
			UseForMetadata:    false, // 默认不启用，优先使用DeepSeek
			AnalyzeVideo:      true,  // 默认启用视频分析
			VideoSampleFrames: 0,     // 默认上传完整视频
		},

		// OpenAI兼容API配置（默认值，可被 config.toml 覆盖）
		OpenAICompatibleConfig: &OpenAICompatibleConfig{
			Enabled:     false,
			Provider:    "openai",
			ApiKey:      "",
			BaseURL:     "https://api.openai.com/v1",
			Model:       "gpt-3.5-turbo",
			Timeout:     60,
			MaxTokens:   4000,
			Temperature: 0.7,
		},

		// 代理配置（默认值，可被 config.toml 覆盖）
		ProxyConfig: &ProxyConfig{
			UseProxy:  false,
			ProxyHost: "",
		},

		// 下载配置（默认值，可被 config.toml 覆盖）
		DownloadConfig: &DownloadConfig{
			UseAria2c:                  true,      // 默认启用 aria2c（如果可用）
			Aria2cPath:                 "",        // 空表示自动检测
			ConcurrentFragments:        8,         // yt-dlp 并发分片数
			Aria2cConnections:          8,         // aria2c 连接数（降低以避免资源竞争）
			HttpChunkSize:              "10M",     // HTTP 分块大小
			PreferFormat:               "best",    // 默认最佳质量
			MaxConcurrentDownloads:     3,         // 最大并发下载数（默认3）
			LongVideoThreshold:         30,        // 长视频阈值（分钟，默认30）
			DownloadTimeout:            120,       // 下载超时时间（分钟，默认120）
			AutoCleanupEnabled:         false,     // 默认不启用自动清理
			AutoCleanupMode:            "delayed", // 默认延迟清理
			AutoCleanupDelay:           60,        // 默认延迟60分钟
			SubtitleTranslationEnabled: true,      // 默认启用字幕翻译
		},

		// 数据分析配置（默认值，可被 config.toml 覆盖）
		AnalyticsConfig: &AnalyticsConfig{
			Enabled:   false,
			ServerURL: "http://localhost:8080",
			APIKey:    "",
			ProductID: "bili-up-api",
			Debug:     false,
		},

		// Bilibili 配置（默认值，可被 config.toml 覆盖）
		BilibiliConfig: &BilibiliConfig{
			Copyright:          1, // 默认自制
			Source:             "",
			NoReprint:          1,         // 默认禁止转载
			UseOriginalTitle:   true,      // 默认使用原视频标题
			UseOriginalDesc:    false,     // 默认使用AI生成的描述
			CustomDescTemplate: "",        // 默认不使用自定义模板
			Tid:                122,       // 默认分区：日常
			Dynamic:            "发布了新视频！", // 默认动态
			OpenElec:           0,         // 默认关闭充电
			SelectionReserve:   0,         // 默认不参与活动
			UpSelectionReply:   0,         // 默认不展示推荐评论
			UpCloseReply:       0,         // 默认开启评论
			UpCloseReward:      0,         // 默认开启打赏
		},

		// 会员系统配置（默认值，可被 config.toml 覆盖）
		MembershipConfig: &MembershipConfig{
			Enabled: false, // 默认不启用会员系统
			Redis: RedisConfig{
				Addr:     "localhost:6379",
				Password: "",
				DB:       1,
			},
		},

		// SMTP 邮件服务配置（默认值，可被 config.toml 覆盖）
		SMTPConfig: &SMTPConfig{
			Enabled:  false, // 默认禁用，开发环境验证码打印到日志
			Host:     "",
			From:     "",
			FromName: "YTB2Bili",
			Username: "",
			Password: "",
			UseTLS:   true,
		},
	}
}

// LoadConfig 加载配置
func LoadConfig(configFile string) (*AppConfig, error) {
	// 先创建默认配置（包含所有硬编码的配置）
	config := NewDefaultConfig()
	config.Path = configFile

	// 检查配置文件是否存在
	_, err := os.Stat(configFile)
	if err != nil {
		// 如果文件不存在，不创建默认配置文件，使用硬编码配置即可
		return config, nil
	}

	// 创建临时结构体用于读取 config.toml（只包含可配置字段）
	var fileConfig struct {
		Listen                 string                  `toml:"listen"`
		Environment            string                  `toml:"environment"`
		Debug                  bool                    `toml:"debug"`
		Database               Database                `toml:"database"`
		Auth                   AuthConfig              `toml:"auth"`
		FileUpDir              string                  `toml:"fileUpDir"`
		YtDlpPath              string                  `toml:"yt_dlp_path"`
		TenCosConfig           *TencentCosConfig       `toml:"TenCosConfig"`
		DeepSeekTransConfig    *DeepSeekTransConfig    `toml:"DeepSeekTransConfig"`
		GeminiConfig           *GeminiConfig           `toml:"GeminiConfig"`
		OpenAICompatibleConfig *OpenAICompatibleConfig `toml:"OpenAICompatibleConfig"`
		ProxyConfig            *ProxyConfig            `toml:"ProxyConfig"`
		DownloadConfig         *DownloadConfig         `toml:"DownloadConfig"`
		AnalyticsConfig        *AnalyticsConfig        `toml:"AnalyticsConfig"`
		BilibiliConfig         *BilibiliConfig         `toml:"BilibiliConfig"`
		MembershipConfig       *MembershipConfig       `toml:"MembershipConfig"`
		SMTPConfig             *SMTPConfig             `toml:"SMTPConfig"`
	}

	// 解码TOML配置文件
	_, err = toml.DecodeFile(configFile, &fileConfig)
	if err != nil {
		return nil, err
	}

	// 只覆盖配置文件中存在的字段，保留硬编码的配置
	config.Listen = fileConfig.Listen
	config.Environment = fileConfig.Environment
	config.Debug = fileConfig.Debug
	config.Database = fileConfig.Database
	config.Auth = fileConfig.Auth
	config.FileUpDir = fileConfig.FileUpDir
	config.YtDlpPath = fileConfig.YtDlpPath
	if fileConfig.TenCosConfig != nil {
		config.TenCosConfig = fileConfig.TenCosConfig
	}
	if fileConfig.DeepSeekTransConfig != nil {
		config.DeepSeekTransConfig = fileConfig.DeepSeekTransConfig
	}
	if fileConfig.GeminiConfig != nil {
		config.GeminiConfig = fileConfig.GeminiConfig
	}
	if fileConfig.OpenAICompatibleConfig != nil {
		config.OpenAICompatibleConfig = fileConfig.OpenAICompatibleConfig
	}
	if fileConfig.ProxyConfig != nil {
		config.ProxyConfig = fileConfig.ProxyConfig
	}
	if fileConfig.DownloadConfig != nil {
		config.DownloadConfig = fileConfig.DownloadConfig
	}
	if fileConfig.AnalyticsConfig != nil {
		config.AnalyticsConfig = fileConfig.AnalyticsConfig
	}
	if fileConfig.BilibiliConfig != nil {
		config.BilibiliConfig = fileConfig.BilibiliConfig
	}
	if fileConfig.MembershipConfig != nil {
		config.MembershipConfig = fileConfig.MembershipConfig
	}
	if fileConfig.SMTPConfig != nil {
		config.SMTPConfig = fileConfig.SMTPConfig
	}

	return config, nil
}

// SaveConfig 保存配置（只保存可配置字段，不保存硬编码配置）
func SaveConfig(config *AppConfig) error {
	// 只保存用户可配置的字段
	fileConfig := struct {
		Listen                 string                  `toml:"listen"`
		Environment            string                  `toml:"environment"`
		Debug                  bool                    `toml:"debug"`
		Database               Database                `toml:"database"`
		Auth                   AuthConfig              `toml:"auth"`
		FileUpDir              string                  `toml:"fileUpDir"`
		YtDlpPath              string                  `toml:"yt_dlp_path"`
		TenCosConfig           *TencentCosConfig       `toml:"TenCosConfig"`
		DeepSeekTransConfig    *DeepSeekTransConfig    `toml:"DeepSeekTransConfig"`
		GeminiConfig           *GeminiConfig           `toml:"GeminiConfig"`
		OpenAICompatibleConfig *OpenAICompatibleConfig `toml:"OpenAICompatibleConfig"`
		ProxyConfig            *ProxyConfig            `toml:"ProxyConfig"`
		DownloadConfig         *DownloadConfig         `toml:"DownloadConfig"`
		AnalyticsConfig        *AnalyticsConfig        `toml:"AnalyticsConfig"`
		BilibiliConfig         *BilibiliConfig         `toml:"BilibiliConfig"`
		MembershipConfig       *MembershipConfig       `toml:"MembershipConfig"`
		SMTPConfig             *SMTPConfig             `toml:"SMTPConfig"`
	}{
		Listen:                 config.Listen,
		Environment:            config.Environment,
		Debug:                  config.Debug,
		Database:               config.Database,
		Auth:                   config.Auth,
		FileUpDir:              config.FileUpDir,
		YtDlpPath:              config.YtDlpPath,
		TenCosConfig:           config.TenCosConfig,
		DeepSeekTransConfig:    config.DeepSeekTransConfig,
		GeminiConfig:           config.GeminiConfig,
		OpenAICompatibleConfig: config.OpenAICompatibleConfig,
		ProxyConfig:            config.ProxyConfig,
		DownloadConfig:         config.DownloadConfig,
		AnalyticsConfig:        config.AnalyticsConfig,
		BilibiliConfig:         config.BilibiliConfig,
		MembershipConfig:       config.MembershipConfig,
		SMTPConfig:             config.SMTPConfig,
	}

	buf := new(bytes.Buffer)

	// 写入注释说明
	buf.WriteString("# Bilibili 视频上传后端 - 配置文件\n\n")
	buf.WriteString("# 注意：以下配置已硬编码在代码中，无需在此配置：\n")
	buf.WriteString("# - BaiduTransConfig (百度翻译)\n")
	buf.WriteString("# - app_auth (应用认证)\n")
	buf.WriteString("# \n")
	buf.WriteString("# 所有配置都可以通过 config.toml 或 API 接口动态配置\n\n")

	encoder := toml.NewEncoder(buf)
	if err := encoder.Encode(&fileConfig); err != nil {
		return err
	}

	return os.WriteFile(config.Path, buf.Bytes(), 0644)
}
