package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SecurityHandler struct {
	Logger *zap.SugaredLogger
}

func NewSecurityHandler(logger *zap.SugaredLogger) *SecurityHandler {
	return &SecurityHandler{Logger: logger}
}

// ReportCSPViolation 处理 CSP 违规报告
func (h *SecurityHandler) ReportCSPViolation(c *gin.Context) {
	var report map[string]interface{}
	if err := c.BindJSON(&report); err != nil {
		c.JSON(400, gin.H{"error": "Invalid report"})
		return
	}

	// 记录违规日志
	h.Logger.Warnw("CSP 违规报告",
		"documentURI", report["document-uri"],
		"referrer", report["referrer"],
		"blockedURI", report["blocked-uri"],
		"violatedDirective", report["violated-directive"],
		"originalPolicy", report["original-policy"],
	)

	c.JSON(200, gin.H{"status": "logged"})
}
