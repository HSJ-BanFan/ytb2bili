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
	db *gorm.DB
}

// NewAuditService 创建审计服务
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
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
	go s.logAsync(entry)
}

// logAsync 异步记录日志（避免阻塞主流程）
func (s *AuditService) logAsync(entry LogEntry) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ 审计日志记录失败（panic）: %v", r)
		}
	}()

	auditLog := &model.AuditLog{
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

	// 序列化 Details
	if entry.Details != nil {
		detailsJSON, err := json.Marshal(entry.Details)
		if err == nil {
			auditLog.Details = string(detailsJSON)
		}
	}

	if err := s.db.Create(auditLog).Error; err != nil {
		log.Printf("⚠️ 审计日志记录失败: %v", err)
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

// GetLogs 查询审计日志
func (s *AuditService) GetLogs(userID uint, action string, limit, offset int) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	var total int64

	query := s.db.Model(&model.AuditLog{})

	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
