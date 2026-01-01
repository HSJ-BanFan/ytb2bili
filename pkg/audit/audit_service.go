package audit

import (
	"encoding/json"
	"log"
	"time"

	"github.com/difyz9/ytb2bili/pkg/store/model"
	"gorm.io/gorm"
)

// 审计操作常量
const (
	ActionUserLogin         = "user_login"
	ActionUserLogout        = "user_logout"
	ActionUserRegister      = "user_register"
	ActionBindBiliAccount   = "bind_bili_account"
	ActionUnbindBiliAccount = "unbind_bili_account"
	ActionUploadVideo       = "upload_video"
	ActionUploadSubtitle    = "upload_subtitle"
)

// 资源类型常量
const (
	ResourceUser        = "user"
	ResourceBiliAccount = "bili_account"
	ResourceVideo       = "video"
)

// AuditService 审计服务
type AuditService struct {
	db      *gorm.DB
	logChan chan LogEntry
	done    chan struct{}
}

// NewAuditService 创建审计服务
func NewAuditService(db *gorm.DB) *AuditService {
	s := &AuditService{
		db:      db,
		logChan: make(chan LogEntry, 1000), // 缓冲区大小 1000
		done:    make(chan struct{}),
	}
	go s.worker() // 启动后台写入协程
	return s
}

// LogEntry 审计日志条目
type LogEntry struct {
	UserID     uint
	Username   string
	Action     string
	Resource   string
	ResourceID string
	IP         string
	UserAgent  string
	Success    bool
	Message    string
	Details    map[string]interface{}
}

// Log 记录审计日志
func (s *AuditService) Log(entry LogEntry) {
	select {
	case s.logChan <- entry:
	default:
		// 缓冲区已满，丢弃日志或降级处理，避免阻塞主流程
		log.Printf("⚠️ 审计日志缓冲区已满，丢弃日志: %s - %s", entry.Action, entry.Message)
	}
}

// worker 后台写入协程
func (s *AuditService) worker() {
	buffer := make([]LogEntry, 0, 10)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case entry, ok := <-s.logChan:
			if !ok {
				s.flush(buffer)
				close(s.done)
				return
			}
			buffer = append(buffer, entry)
			if len(buffer) >= 10 { // 批量写入阈值
				s.flush(buffer)
				buffer = buffer[:0]
			}
		case <-ticker.C:
			if len(buffer) > 0 {
				s.flush(buffer)
				buffer = buffer[:0]
			}
		}
	}
}

// flush 批量写入数据库
func (s *AuditService) flush(entries []LogEntry) {
	if len(entries) == 0 {
		return
	}

	var logs []model.AuditLog
	for _, entry := range entries {
		auditLog := model.AuditLog{
			CreatedAt:  time.Now(),
			UserID:     entry.UserID,
			Username:   entry.Username,
			Action:     entry.Action,
			Resource:   entry.Resource,
			ResourceID: entry.ResourceID,
			IP:         entry.IP,
			UserAgent:  entry.UserAgent,
			Success:    entry.Success,
			Message:    entry.Message,
		}
		if entry.Details != nil {
			detailsJSON, err := json.Marshal(entry.Details)
			if err == nil {
				auditLog.Details = string(detailsJSON)
			}
		}
		logs = append(logs, auditLog)
	}

	if err := s.db.CreateInBatches(logs, len(logs)).Error; err != nil {
		log.Printf("⚠️ 批量写入审计日志失败: %v", err)
	}
}

// LogSuccess 记录成功操作
func (s *AuditService) LogSuccess(userID uint, username, action, resource, resourceID, ip, userAgent, message string) {
	s.Log(LogEntry{
		UserID:     userID,
		Username:   username,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IP:         ip,
		UserAgent:  userAgent,
		Success:    true,
		Message:    message,
	})
}

// LogFailure 记录失败操作
func (s *AuditService) LogFailure(userID uint, username, action, resource, resourceID, ip, userAgent, message string) {
	s.Log(LogEntry{
		UserID:     userID,
		Username:   username,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IP:         ip,
		UserAgent:  userAgent,
		Success:    false,
		Message:    message,
	})
}

// QueryFilter 查询过滤器
type QueryFilter struct {
	UserID    uint
	Action    string
	Resource  string
	IP        string
	StartDate *time.Time
	EndDate   *time.Time
	Success   *bool
}

// GetLogs 查询审计日志 (保留旧接口兼容性)
func (s *AuditService) GetLogs(userID uint, action string, limit, offset int) ([]model.AuditLog, int64, error) {
	return s.Query(QueryFilter{
		UserID: userID,
		Action: action,
	}, limit, offset)
}

// Query 高级查询接口
func (s *AuditService) Query(filter QueryFilter, limit, offset int) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	var total int64

	query := s.db.Model(&model.AuditLog{})

	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.Resource != "" {
		query = query.Where("resource = ?", filter.Resource)
	}
	if filter.IP != "" {
		query = query.Where("ip LIKE ?", "%"+filter.IP+"%")
	}
	if filter.StartDate != nil {
		query = query.Where("created_at >= ?", filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("created_at <= ?", filter.EndDate)
	}
	if filter.Success != nil {
		query = query.Where("success = ?", *filter.Success)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// CleanupOldLogs 清理旧日志
func (s *AuditService) CleanupOldLogs(retentionDays int) error {
	if retentionDays <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := s.db.Where("created_at < ?", cutoff).Delete(&model.AuditLog{})
	if result.Error != nil {
		return result.Error
	}
	log.Printf("🧹 已清理 %d 天前的审计日志，共删除 %d 条记录", retentionDays, result.RowsAffected)
	return nil
}

// Close 关闭服务
func (s *AuditService) Close() {
	close(s.logChan)
	<-s.done
}
