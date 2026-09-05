package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/storage"
	"github.com/difyz9/ytb2bili/pkg/audit"
	"github.com/gin-gonic/gin"
)

// BiliAccountHandler B站账号管理 Handler
type BiliAccountHandler struct {
	BaseHandler
	BiliAccountService *services.BiliAccountService
	AuditService       *audit.AuditService // 审计服务
}

// NewBiliAccountHandler 创建 B站账号管理 Handler
func NewBiliAccountHandler(app *core.AppServer, biliAccountService *services.BiliAccountService, auditService *audit.AuditService) *BiliAccountHandler {
	return &BiliAccountHandler{
		BaseHandler:        BaseHandler{App: app},
		BiliAccountService: biliAccountService,
		AuditService:       auditService,
	}
}

// RegisterRoutes 注册路由
// RegisterRoutes 注册公开的 B站账号管理路由
func (h *BiliAccountHandler) RegisterRoutes(server *core.AppServer) {
	api := server.Engine.Group("/api/v1/bili-accounts")
	api.GET("", h.listAccounts)
	api.POST("/bind", h.bindAccount)
	api.POST("/bind-from-qrcode", h.bindFromQRCode)
	api.DELETE("/:id", h.unbindAccount)
	api.PUT("/:id/primary", h.setPrimary)
	api.PUT("/:id/enable", h.enableAccount)
	api.PUT("/:id/disable", h.disableAccount)
}

// listAccounts 获取全局B站账号
func (h *BiliAccountHandler) listAccounts(c *gin.Context) {

	accounts, err := h.BiliAccountService.GetAccounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取账号列表失败"})
		return
	}

	// 转换为前端友好的格式（隐藏敏感信息）
	var result []gin.H
	for _, acc := range accounts {
		result = append(result, gin.H{
			"id":                 acc.ID,
			"bili_mid":           acc.BiliMid,
			"bili_name":          acc.BiliName,
			"bili_face":          acc.BiliFace,
			"is_enabled":         acc.IsEnabled,
			"is_primary":         acc.IsPrimary,
			"expires_at":         acc.ExpiresAt,
			"last_used_at":       acc.LastUsedAt,
			"created_at":         acc.CreatedAt,
			"encryption_version": acc.EncryptionVersion, // P1-9: 暴露加密版本
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// BindAccountRequest 绑定账号请求
type BindAccountRequest struct {
	LoginInfo *bilibili.LoginInfo `json:"login_info" binding:"required"`
	IsPrimary bool                `json:"is_primary"`
}

// bindAccount 绑定B站账号
func (h *BiliAccountHandler) bindAccount(c *gin.Context) {

	var req BindAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	account, err := h.BiliAccountService.BindAccount(req.LoginInfo, req.IsPrimary)
	if err != nil {
		h.AuditService.LogFailure(
			0, "",
			audit.ActionBindBiliAccount,
			audit.ResourceBiliAccount,
			fmt.Sprintf("%d", req.LoginInfo.TokenInfo.Mid),
			c.ClientIP(),
			c.Request.UserAgent(),
			"绑定失败: "+err.Error(),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "绑定失败: " + err.Error()})
		return
	}

	h.AuditService.LogSuccess(
		0, "",
		audit.ActionBindBiliAccount,
		audit.ResourceBiliAccount,
		fmt.Sprintf("%d", account.BiliMid),
		c.ClientIP(),
		c.Request.UserAgent(),
		"绑定 B站账号成功: "+account.BiliName,
	)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "绑定成功",
		"data": gin.H{
			"id":                 account.ID,
			"bili_mid":           account.BiliMid,
			"bili_name":          account.BiliName,
			"bili_face":          account.BiliFace,
			"is_primary":         account.IsPrimary,
			"encryption_version": account.EncryptionVersion,
		},
	})
}

// bindFromQRCode 从B站扫码结果绑定账号
func (h *BiliAccountHandler) bindFromQRCode(c *gin.Context) {

	// 从全局 BiliStore 获取扫码登录信息
	store := storage.GetDefaultStore()
	if store == nil || !store.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "B站未扫码登录，请先完成扫码",
		})
		return
	}

	// 获取登录信息
	loginInfo, err := store.Load()
	if err != nil || loginInfo == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无法获取B站登录信息: " + err.Error(),
		})
		return
	}

	// 第一个账号自动设为主账号
	existingAccounts, _ := h.BiliAccountService.GetAccounts()
	isPrimary := len(existingAccounts) == 0
	account, err := h.BiliAccountService.BindAccount(loginInfo, isPrimary)
	if err != nil {
		h.AuditService.LogFailure(
			0, "",
			audit.ActionBindBiliAccount,
			audit.ResourceBiliAccount,
			fmt.Sprintf("%d", loginInfo.TokenInfo.Mid),
			c.ClientIP(),
			c.Request.UserAgent(),
			"扫码绑定失败: "+err.Error(),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "绑定失败: " + err.Error()})
		return
	}

	h.AuditService.LogSuccess(
		0, "",
		audit.ActionBindBiliAccount,
		audit.ResourceBiliAccount,
		fmt.Sprintf("%d", account.BiliMid),
		c.ClientIP(),
		c.Request.UserAgent(),
		"扫码绑定 B站账号成功: "+account.BiliName,
	)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "绑定成功",
		"data": gin.H{
			"id":                 account.ID,
			"bili_mid":           account.BiliMid,
			"bili_name":          account.BiliName,
			"bili_face":          account.BiliFace,
			"is_primary":         account.IsPrimary,
			"encryption_version": account.EncryptionVersion,
		},
	})
}

// unbindAccount 解绑B站账号
func (h *BiliAccountHandler) unbindAccount(c *gin.Context) {

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的账号ID"})
		return
	}

	if err := h.BiliAccountService.UnbindAccount(uint(accountID)); err != nil {
		h.AuditService.LogFailure(
			0, "",
			audit.ActionUnbindBiliAccount,
			audit.ResourceBiliAccount,
			c.Param("id"),
			c.ClientIP(),
			c.Request.UserAgent(),
			"解绑失败: "+err.Error(),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "解绑失败: " + err.Error()})
		return
	}

	h.AuditService.LogSuccess(
		0, "",
		audit.ActionUnbindBiliAccount,
		audit.ResourceBiliAccount,
		c.Param("id"),
		c.ClientIP(),
		c.Request.UserAgent(),
		"解绑成功",
	)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "解绑成功"})
}

// setPrimary 设置主账号
func (h *BiliAccountHandler) setPrimary(c *gin.Context) {

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的账号ID"})
		return
	}

	if err := h.BiliAccountService.SetPrimaryAccount(uint(accountID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "设置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "设置成功"})
}

// enableAccount 启用账号
func (h *BiliAccountHandler) enableAccount(c *gin.Context) {

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的账号ID"})
		return
	}

	if err := h.BiliAccountService.EnableAccount(uint(accountID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "启用失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "启用成功"})
}

// disableAccount 禁用账号
func (h *BiliAccountHandler) disableAccount(c *gin.Context) {

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的账号ID"})
		return
	}

	if err := h.BiliAccountService.DisableAccount(uint(accountID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "禁用失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "禁用成功"})
}
