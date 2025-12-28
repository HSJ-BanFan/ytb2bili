package utils

import (
	"context"
	"fmt"
	"time"
)

// TimeoutHelper 超时控制助手
type TimeoutHelper struct {
	defaultTimeout time.Duration
}

// NewTimeoutHelper 创建超时控制助手
func NewTimeoutHelper(defaultTimeout time.Duration) *TimeoutHelper {
	if defaultTimeout <= 0 {
		defaultTimeout = 30 * time.Minute // 默认30分钟
	}
	return &TimeoutHelper{
		defaultTimeout: defaultTimeout,
	}
}

// WithTimeout 创建带超时的上下文（使用默认超时）
func (th *TimeoutHelper) WithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), th.defaultTimeout)
}

// WithTimeoutDuration 创建带超时的上下文（自定义超时时间）
func (th *TimeoutHelper) WithTimeoutDuration(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = th.defaultTimeout
	}
	return context.WithTimeout(context.Background(), timeout)
}

// RunWithTimeout 在超时时间内执行函数
func (th *TimeoutHelper) RunWithTimeout(fn func(ctx context.Context) error) error {
	ctx, cancel := th.WithTimeout()
	defer cancel()

	return fn(ctx)
}

// RunWithTimeoutCustom 在自定义超时时间内执行函数
func (th *TimeoutHelper) RunWithTimeoutCustom(timeout time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := th.WithTimeoutDuration(timeout)
	defer cancel()

	return fn(ctx)
}

// TaskTimeouts 任务超时配置（推荐值）
type TaskTimeouts struct {
	DownloadVideo     time.Duration // 下载视频：根据视频大小，默认2小时
	GenerateSubtitle  time.Duration // 生成字幕：10分钟
	TranslateSubtitle time.Duration // 翻译字幕：15分钟
	GenerateMetadata  time.Duration // 生成元数据：5分钟
	UploadVideo       time.Duration // 上传视频：1小时
	UploadSubtitle    time.Duration // 上传字幕：30分钟
	FetchMetadata     time.Duration // 获取元数据：5分钟
}

// DefaultTimeouts 返回默认的任务超时配置
func DefaultTimeouts() TaskTimeouts {
	return TaskTimeouts{
		DownloadVideo:     2 * time.Hour,
		GenerateSubtitle:  10 * time.Minute,
		TranslateSubtitle: 15 * time.Minute,
		GenerateMetadata:  5 * time.Minute,
		UploadVideo:       1 * time.Hour,
		UploadSubtitle:    30 * time.Minute,
		FetchMetadata:     5 * time.Minute,
	}
}

// GetTimeoutForTask 根据任务名称获取超时时间
func (tt TaskTimeouts) GetTimeoutForTask(taskName string) time.Duration {
	switch taskName {
	case "下载视频", "download_video":
		return tt.DownloadVideo
	case "下载字幕", "generate_subtitle":
		return tt.GenerateSubtitle
	case "翻译字幕", "translate_subtitle":
		return tt.TranslateSubtitle
	case "生成元数据", "generate_metadata":
		return tt.GenerateMetadata
	case "上传到Bilibili", "upload_video":
		return tt.UploadVideo
	case "上传字幕到Bilibili", "upload_subtitle":
		return tt.UploadSubtitle
	case "获取元数据", "fetch_metadata":
		return tt.FetchMetadata
	default:
		return 30 * time.Minute // 默认30分钟
	}
}

// FormatDuration 格式化时间间隔为可读字符串
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f秒", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0f分钟", d.Minutes())
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes > 0 {
			return fmt.Sprintf("%d小时%d分钟", hours, minutes)
		}
		return fmt.Sprintf("%d小时", hours)
	} else {
		days := int(d.Hours() / 24)
		hours := int(d.Hours()) % 24
		if hours > 0 {
			return fmt.Sprintf("%d天%d小时", days, hours)
		}
		return fmt.Sprintf("%d天", days)
	}
}
