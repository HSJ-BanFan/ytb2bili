package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// SourceChannel 来源媒体频道模型
// 描述外部媒体来源（YouTube 频道、YouTube 播放列表、Twitch 频道等）
type SourceChannel struct {
	BaseModel
	Platform       string     `gorm:"type:varchar(50);not null;index" json:"platform"`             // 平台: youtube, twitch
	ChannelID      string     `gorm:"type:varchar(150);not null;index" json:"channel_id"`          // 原生频道ID或标识（如 UCxxxx 或频道 Handle）
	ChannelName    string     `gorm:"type:varchar(255);not null" json:"channel_name"`              // 频道显示名称
	ChannelURL     string     `gorm:"type:varchar(500);not null" json:"channel_url"`               // 频道主页URL
	FetchType      string     `gorm:"type:varchar(50);default:channel_video" json:"fetch_type"`    // 采集类型: channel_video (点播搬运), live_stream (直播录制), playlist (播放列表)
	IsEnabled      bool       `gorm:"default:true;index" json:"is_enabled"`                        // 频道级开关（三级熔断第2级）
	Status         string     `gorm:"type:varchar(30);default:active" json:"status"`               // 状态: active, paused, error
	CronExpression string     `gorm:"type:varchar(100);default:@every 30m" json:"cron_expression"` // 轮询检查周期
	LastCheckedAt  *time.Time `json:"last_checked_at,omitempty"`                                   // 最后轮询时间
	ExtraConfig    string     `gorm:"type:text" json:"extra_config,omitempty"`                     // 扩展配置 (JSON): 分辨率偏好、关键字过滤等

	// 关联
	StrategyRules []StrategyRule `gorm:"foreignKey:SourceChannelID;references:ID" json:"strategy_rules,omitempty"`
}

// TableName 指定表名
func (SourceChannel) TableName() string {
	return "cw_source_channels"
}

// StrategyRule 搬运策略规则模型
// 定义来源频道与投稿账号之间的多对多映射关系与定制加工规则
type StrategyRule struct {
	BaseModel
	SourceChannelID      uint   `gorm:"not null;index" json:"source_channel_id"`                  // 关联 SourceChannel.ID
	BiliAccountID        uint   `gorm:"not null;index" json:"bili_account_id"`                     // 关联 UserBiliAccount.ID
	RuleName             string `gorm:"type:varchar(150);not null" json:"rule_name"`               // 规则标识名称
	IsEnabled            bool   `gorm:"default:true;index" json:"is_enabled"`                      // 规则开关
	Priority             int    `gorm:"default:0" json:"priority"`                                 // 调度优先级
	AutoPublish          bool   `gorm:"default:false" json:"auto_publish"`                         // 是否全自动投稿（false 为草稿/待人工确认）
	DynamicTitleTemplate string `gorm:"type:varchar(500)" json:"dynamic_title_template,omitempty"` // 标题模板或规则提示
	DescTemplate         string `gorm:"type:text" json:"desc_template,omitempty"`                  // 简介模板（如保留作者声明）
	DefaultTags          string `gorm:"type:varchar(500)" json:"default_tags,omitempty"`           // 缺省标签（逗号分隔）
	CategoryID           int    `gorm:"default:17" json:"category_id"`                             // B站主分区 TID
	Copyright            int    `gorm:"default:2" json:"copyright"`                                // 投稿类型: 1=自制, 2=转载
	SourceOrigin         string `gorm:"type:varchar(500)" json:"source_origin,omitempty"`          // 转载来源地址说明
	DtimeDelayMinutes    int    `gorm:"default:0" json:"dtime_delay_minutes"`                      // 定时发布延迟分钟数 (0 为立即发布)
	ExtraFields          string `gorm:"type:text" json:"extra_fields,omitempty"`                   // 扩展字段 (JSON: 分P偏好、转码参数等)

	// 关联
	SourceChannel   *SourceChannel   `gorm:"foreignKey:SourceChannelID;references:ID" json:"source_channel,omitempty"`
	UserBiliAccount *UserBiliAccount `gorm:"foreignKey:BiliAccountID;references:ID" json:"user_bili_account,omitempty"`
}

// TableName 指定表名
func (StrategyRule) TableName() string {
	return "cw_strategy_rules"
}

// PublishFingerprint 投稿指纹与幂等性追踪模型
// 防线核心：保证同一来源视频不会被重复投向同一 B 站账号，同时提供并发排队锁
type PublishFingerprint struct {
	BaseModel
	SourcePlatform    string     `gorm:"type:varchar(50);not null;index" json:"source_platform"`        // 来源平台: youtube, twitch
	SourceVideoID     string     `gorm:"type:varchar(150);not null;index" json:"source_video_id"`       // 原生视频唯一ID
	SourceSegmentHash string     `gorm:"type:varchar(64);index" json:"source_segment_hash,omitempty"`   // 媒体文件/分段 SHA256 哈希
	BiliAccountID     uint       `gorm:"not null;index" json:"bili_account_id"`                         // 目标 B 站账号 ID
	StrategyRuleID    uint       `gorm:"not null;index" json:"strategy_rule_id"`                        // 匹配的策略规则 ID
	FingerprintHash   string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"fingerprint_hash"` // 组合唯一哈希: SHA256(Platform:VideoID:BiliAccountID)
	PublishStatus     string     `gorm:"type:varchar(30);default:pending;index" json:"publish_status"`  // 状态: pending, locked, published, failed, deadletter
	BiliBVID          string     `gorm:"column:bili_bvid;type:varchar(50);index" json:"bili_bvid,omitempty"`             // 成功后的 B 站 BV 号
	BiliAID           int64      `gorm:"column:bili_aid;type:bigint" json:"bili_aid,omitempty"`                         // 成功后的 B 站 AID
	PublishedAt       *time.Time `json:"published_at,omitempty"`                                        // 投稿成功时间
	RetryCount        int        `gorm:"default:0" json:"retry_count"`                                  // 当前重试次数
	MaxRetries        int        `gorm:"default:3" json:"max_retries"`                                  // 最大允许重试次数
	LockExpiresAt     *time.Time `gorm:"index" json:"lock_expires_at,omitempty"`                        // 执行锁过期时间（防止 worker 崩溃永久锁死）
	DeadLetterReason  string     `gorm:"type:text" json:"dead_letter_reason,omitempty"`                 // 隔离到死信的原因
}

// TableName 指定表名
func (PublishFingerprint) TableName() string {
	return "cw_publish_fingerprints"
}

// FingerprintStatus 常量
const (
	FingerprintStatusPending    = "pending"    // 待发布
	FingerprintStatusLocked     = "locked"     // 已被 Worker 锁定正在执行
	FingerprintStatusPublished  = "published"  // 已发布成功（终态）
	FingerprintStatusFailed     = "failed"     // 执行失败可重试
	FingerprintStatusDeadLetter = "deadletter" // 达到最大重试或严重错误，进入死信人工处理
)

// GenerateFingerprintHash 计算组合唯一指纹哈希
func GenerateFingerprintHash(platform, sourceVideoID string, biliAccountID uint) string {
	raw := fmt.Sprintf("%s:%s:%d", platform, sourceVideoID, biliAccountID)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// SystemGuardrail 系统投稿防线与三级熔断开关模型
// 包含全局级、频道级、账号级的三层保护机制
type SystemGuardrail struct {
	BaseModel
	Scope               string     `gorm:"type:varchar(30);not null;index" json:"scope"`      // 作用域: global, channel, account
	TargetID            string     `gorm:"type:varchar(100);not null;index" json:"target_id"` // 目标ID: global="0", channel=ChannelID, account=AccountID
	IsPaused            bool       `gorm:"default:false;index" json:"is_paused"`              // 是否处于暂停状态
	PauseReason         string     `gorm:"type:varchar(255)" json:"pause_reason,omitempty"`   // 暂停原因: manual_kill_switch, rate_limit_601, credential_expired, circuit_breaker
	ConsecutiveFailures int        `gorm:"default:0" json:"consecutive_failures"`             // 连续失败计数
	FailureThreshold    int        `gorm:"default:3" json:"failure_threshold"`                // 触发自动熔断的失败阈值
	AutoResumeAt        *time.Time `gorm:"index" json:"auto_resume_at,omitempty"`             // 熔断后自动恢复时间（冷却期）
	LastTriggeredAt     *time.Time `json:"last_triggered_at,omitempty"`                       // 最后触发/变更时间
}

// TableName 指定表名
func (SystemGuardrail) TableName() string {
	return "cw_system_guardrails"
}

// GuardrailScope 常量
const (
	GuardrailScopeGlobal  = "global"
	GuardrailScopeChannel = "channel"
	GuardrailScopeAccount = "account"
)
