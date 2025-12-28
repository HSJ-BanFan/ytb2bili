package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 基础模型
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// AudioResult 音频处理结果
type AudioResult struct {
	SID            int     `json:"sid"`
	Text           string  `json:"text"`
	TranslatedText string  `json:"translated_text,omitempty"`
	AudioURL       string  `json:"audio_url"`
	Language       string  `json:"language"`
	Duration       float64 `json:"duration"`
}

// TranslationSettings 翻译设置
type TranslationSettings struct {
	SourceLanguage string  `json:"source_language"`
	TargetLanguage string  `json:"target_language"`
	Service        string  `json:"service"`
	Gender         string  `json:"gender"`
	Tier           string  `json:"tier"`
	VoiceName      string  `json:"voice_name"`
	VoiceSpeed     float64 `json:"voice_speed"`
}

// VideoProcessingRequest 视频处理请求（根据用户提供的JSON格式）
type VideoProcessingRequest struct {
	VideoID             string              `json:"video_id"`
	Platform            string              `json:"platform"`
	Subtitles           []SubtitleItem      `json:"subtitles"`
	TranslationSettings TranslationSettings `json:"translation_settings"`
}

// User 用户模型
type User struct {
	BaseModel
	Username    string     `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Email       string     `gorm:"uniqueIndex;size:100" json:"email"`
	Phone       string     `gorm:"size:20" json:"phone"`
	Password    string     `gorm:"size:100" json:"-"` // B站用户可为空
	Avatar      string     `gorm:"size:255" json:"avatar"`
	Status      int        `gorm:"default:1" json:"status"` // 1:正常 0:禁用
	LastLoginAt *time.Time `json:"last_login_at"`
	BiliMid     string     `gorm:"index;size:50" json:"bili_mid"` // B站用户MID，用于关联B站账号（允许为空）

	// 会员系统字段
	MembershipTier   string     `gorm:"size:20;default:free" json:"membership_tier"` // 会员等级: free/basic/pro/enterprise
	MembershipExpire *time.Time `json:"membership_expire"`                           // 会员到期时间
	SubscriptionID   string     `gorm:"size:100" json:"subscription_id"`             // 支付平台订阅ID
	BoostPackVideos  int        `gorm:"default:0" json:"boost_pack_videos"`          // 加油包剩余视频数
	BoostPackExpire  *time.Time `json:"boost_pack_expire"`                           // 加油包到期时间
	DailyUsageCount  int        `gorm:"default:0" json:"daily_usage_count"`          // 今日使用量
	DailyUsageDate   string     `gorm:"size:10" json:"daily_usage_date"`             // 使用量统计日期 (YYYY-MM-DD)
}

type SubtitleItem struct {
	SID      int     `json:"sid" gorm:"column:sid"`           // 字幕ID
	From     float64 `json:"from" gorm:"column:from_time"`    // 开始时间
	To       float64 `json:"to" gorm:"column:to_time"`        // 结束时间
	Text     string  `json:"text" gorm:"column:content"`      // 字幕内容（兼容用户格式）
	Content  string  `json:"content" gorm:"column:content"`   // 字幕内容（兼容数据库格式）
	Location int     `json:"location" gorm:"column:location"` // 位置信息
}

// SavedVideoSubtitle 用户提交的字幕条目（用于API接收）
type SavedVideoSubtitle struct {
	Text     string  `json:"text"`     // 字幕文本
	Duration float64 `json:"duration"` // 持续时间
	Offset   float64 `json:"offset"`   // 偏移时间
	Lang     string  `json:"lang"`     // 语言
}

// SavedVideo 保存的视频信息
type SavedVideo struct {
	BaseModel
	VideoID        string `gorm:"type:varchar(100);uniqueIndex;not null" json:"video_id"` // 视频ID（唯一）
	URL            string `gorm:"type:varchar(500);not null;index" json:"url"`            // 视频URL
	Title          string `gorm:"type:varchar(500)" json:"title"`                         // 视频标题
	Status         string `gorm:"type:varchar(20)" json:"status"`                         // 视频状态
	Description    string `gorm:"type:text" json:"description"`                           // 视频描述
	GeneratedTitle string `gorm:"type:varchar(500)" json:"generated_title"`               // AI生成的标题
	GeneratedDesc  string `gorm:"type:text" json:"generated_desc"`                        // AI生成的描述
	GeneratedTags  string `gorm:"type:varchar(1000)" json:"generated_tags"`               // AI生成的标签（逗号分隔）
	BiliBVID       string `gorm:"type:varchar(50)" json:"bili_bvid"`                      // Bilibili BVID (主账号)
	BiliAID        int64  `gorm:"type:bigint" json:"bili_aid"`                            // Bilibili AID (主账号)
	BiliMultiBVIDs string `gorm:"type:text" json:"bili_multi_bvids"`                      // 多账号上传的所有BVID (JSON数组)
	OperationType  string `gorm:"type:varchar(50)" json:"operation_type"`                 // 操作类型 (download/upload等)
	Subtitles      string `gorm:"type:longtext" json:"subtitles"`                         // 字幕JSON字符串
	PlaylistID     string `gorm:"type:varchar(100);index" json:"playlist_id"`             // 播放列表ID
	Timestamp      string `gorm:"type:varchar(50)" json:"timestamp"`                      // 时间戳
	SavedAt        string `gorm:"type:varchar(50)" json:"saved_at"`                       // 保存时间
	UserID         uint   `gorm:"index" json:"user_id"`                                   // 提交用户ID

	// 字幕上传调度字段
	VideoSizeMB           float64    `gorm:"type:decimal(10,2)" json:"video_size_mb"`                  // 视频大小(MB)
	SubtitleScheduledAt   *time.Time `gorm:"index" json:"subtitle_scheduled_at"`                       // 字幕计划上传时间
	SubtitleUploadRetries int        `gorm:"default:0" json:"subtitle_upload_retries"`                 // 字幕上传重试次数
	SubtitleUploadError   string     `gorm:"type:varchar(500)" json:"subtitle_upload_error,omitempty"` // 字幕上传错误信息

	// 自动上传调度字段
	ProcessingCompletedAt *time.Time `gorm:"index" json:"processing_completed_at"` // 视频处理完成时间（用于视频上传调度）
	VideoUploadedAt       *time.Time `gorm:"index" json:"video_uploaded_at"`       // 视频上传完成时间（用于字幕上传调度）

	// 下载进度字段
	DownloadProgress string `gorm:"type:text" json:"download_progress,omitempty"` // JSON格式的下载进度
}

// TableName 指定表名
func (SavedVideo) TableName() string {
	return "cw_saved_videos"
}

// App 应用/客户端模型 (用于 appId + appSecret 鉴权)
type App struct {
	BaseModel
	AppID       string `gorm:"uniqueIndex;size:64;not null" json:"app_id"` // 应用ID
	AppSecret   string `gorm:"size:128;not null" json:"-"`                 // 应用密钥 (不返回给前端)
	Name        string `gorm:"size:100;not null" json:"name"`              // 应用名称
	Description string `gorm:"size:500" json:"description"`                // 描述
	Status      int    `gorm:"default:1" json:"status"`                    // 状态: 1=启用, 0=禁用
	RateLimit   int    `gorm:"default:1000" json:"rate_limit"`             // 每日请求限制
	AllowedIPs  string `gorm:"type:text" json:"allowed_ips"`               // 允许的IP白名单 (JSON数组)
	OwnerID     *uint  `gorm:"index" json:"owner_id"`                      // 所属用户ID (可选)
	Owner       *User  `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`  // 所属用户
}

// TableName 指定表名
func (App) TableName() string {
	return "cw_apps"
}

// UserToken 用户 Token 记录 (用于 JWT 黑名单/刷新等)
type UserToken struct {
	BaseModel
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	User         User      `gorm:"foreignKey:UserID" json:"-"`
	TokenHash    string    `gorm:"uniqueIndex;size:64;not null" json:"-"` // Token 哈希 (用于黑名单)
	RefreshToken string    `gorm:"size:256" json:"-"`                     // 刷新 Token
	DeviceInfo   string    `gorm:"size:255" json:"device_info"`           // 设备信息
	IP           string    `gorm:"size:45" json:"ip"`                     // 登录IP
	ExpiresAt    time.Time `json:"expires_at"`                            // 过期时间
	IsRevoked    bool      `gorm:"default:false" json:"is_revoked"`       // 是否已撤销
}

// TableName 指定表名
func (UserToken) TableName() string {
	return "cw_user_tokens"
}

// UserBiliAccount 用户绑定的B站账号
type UserBiliAccount struct {
	BaseModel
	UserID       uint       `gorm:"index;not null" json:"user_id"`   // 系统用户ID
	BiliMid      int64      `gorm:"index;not null" json:"bili_mid"`  // B站用户MID
	BiliName     string     `gorm:"size:100" json:"bili_name"`       // B站用户名
	BiliFace     string     `gorm:"size:500" json:"bili_face"`       // B站头像URL
	IsEnabled    bool       `gorm:"default:true" json:"is_enabled"`  // 是否启用
	IsPrimary    bool       `gorm:"default:false" json:"is_primary"` // 是否为主账号（用于上传）
	Cookies      string     `gorm:"type:text" json:"-"`              // 登录凭证（加密存储）
	AccessToken  string     `gorm:"type:text" json:"-"`              // Access Token
	RefreshToken string     `gorm:"type:text" json:"-"`              // Refresh Token
	ExpiresAt    *time.Time `json:"expires_at"`                      // 凭证过期时间
	LastUsedAt   *time.Time `json:"last_used_at"`                    // 最后使用时间

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"` // 关联用户
}

// TableName 指定表名
func (UserBiliAccount) TableName() string {
	return "cw_user_bili_accounts"
}
