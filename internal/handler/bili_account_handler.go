package handler

import (
	"net/http"
	"strconv"

	"github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/difyz9/ytb2bili/internal/auth"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/membership"
	"github.com/gin-gonic/gin"
)

// BiliAccountHandler B站账号管理 Handler
type BiliAccountHandler struct {
	BaseHandler
	BiliAccountService *services.BiliAccountService
	FeatureChecker     *membership.FeatureChecker
}

// NewBiliAccountHandler 创建 B站账号管理 Handler
func NewBiliAccountHandler(app *core.AppServer, biliAccountService *services.BiliAccountService, featureChecker *membership.FeatureChecker) *BiliAccountHandler {
	return &BiliAccountHandler{
		BaseHandler:        BaseHandler{App: app},
		BiliAccountService: biliAccountService,
		FeatureChecker:     featureChecker,
	}
}

// RegisterRoutes 注册路由
func (h *BiliAccountHandler) RegisterRoutes(server *core.AppServer, authMiddleware *auth.AuthMiddleware) {
	api := server.Engine.Group("/api/v1")

	// B站账号管理（需要登录）
	biliAccount := api.Group("/bili-accounts")
	biliAccount.Use(authMiddleware.JWTAuth())
	{
		biliAccount.GET("", h.listAccounts)                     // 获取用户的所有B站账号
		biliAccount.POST("/bind", h.bindAccount)                // 绑定新账号（传入 login_info）
		biliAccount.POST("/bind-from-qrcode", h.bindFromQRCode) // 从扫码结果绑定
		biliAccount.DELETE("/:id", h.unbindAccount)             // 解绑账号
		biliAccount.PUT("/:id/primary", h.setPrimary)           // 设置主账号
		biliAccount.PUT("/:id/enable", h.enableAccount)
		biliAccount.PUT("/:id/disable", h.disableAccount)
	}
}

// listAccounts 获取用户的所有B站账号
func (h *BiliAccountHandler) listAccounts(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	accounts, err := h.BiliAccountService.GetUserAccounts(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取账号列表失败"})
		return
	}

	// 转换为前端友好的格式（隐藏敏感信息）
	var result []gin.H
	for _, acc := range accounts {
		result = append(result, gin.H{
			"id":           acc.ID,
			"bili_mid":     acc.BiliMid,
			"bili_name":    acc.BiliName,
			"bili_face":    acc.BiliFace,
			"is_enabled":   acc.IsEnabled,
			"is_primary":   acc.IsPrimary,
			"expires_at":   acc.ExpiresAt,
			"last_used_at": acc.LastUsedAt,
			"created_at":   acc.CreatedAt,
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
	userID := h.getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	var req BindAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	// 所有用户都可以绑定多个账号，多账号上传限制在上传环节检查
	account, err := h.BiliAccountService.BindAccount(userID, req.LoginInfo, req.IsPrimary)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "绑定失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "绑定成功",
		"data": gin.H{
			"id":         account.ID,
			"bili_mid":   account.BiliMid,
			"bili_name":  account.BiliName,
			"bili_face":  account.BiliFace,
			"is_primary": account.IsPrimary,
		},
	})
}

// bindFromQRCode 从B站扫码结果绑定账号（用户登录后扫码绑定）
func (h *BiliAccountHandler) bindFromQRCode(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	// 从全局 BiliStore 获取扫码登录信息
	store := auth.GetBiliStore()
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

	// 绑定账号到当前用户
	// 如果用户没有其他账号，设为主账号
	existingAccounts, _ := h.BiliAccountService.GetUserAccounts(userID)
	isPrimary := len(existingAccounts) == 0

	account, err := h.BiliAccountService.BindAccount(userID, loginInfo, isPrimary)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "绑定失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "绑定成功",
		"data": gin.H{
			"id":         account.ID,
			"bili_mid":   account.BiliMid,
			"bili_name":  account.BiliName,
			"bili_face":  account.BiliFace,
			"is_primary": account.IsPrimary,
		},
	})
}

// unbindAccount 解绑B站账号
func (h *BiliAccountHandler) unbindAccount(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的账号ID"})
		return
	}

	if err := h.BiliAccountService.UnbindAccount(userID, uint(accountID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "解绑失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "解绑成功"})
}

// setPrimary 设置主账号
func (h *BiliAccountHandler) setPrimary(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的账号ID"})
		return
	}

	if err := h.BiliAccountService.SetPrimaryAccount(userID, uint(accountID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "设置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "设置成功"})
}

// enableAccount 启用账号
func (h *BiliAccountHandler) enableAccount(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的账号ID"})
		return
	}

	// 验证账号属于当前用户
	account, err := h.BiliAccountService.GetAccountByID(uint(accountID))
	if err != nil || account.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "无权操作此账号"})
		return
	}

	if err := h.BiliAccountService.EnableAccount(userID, uint(accountID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "启用失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "启用成功"})
}

// disableAccount 禁用账号
func (h *BiliAccountHandler) disableAccount(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的账号ID"})
		return
	}

	// 验证账号属于当前用户
	account, err := h.BiliAccountService.GetAccountByID(uint(accountID))
	if err != nil || account.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "无权操作此账号"})
		return
	}

	if err := h.BiliAccountService.DisableAccount(userID, uint(accountID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "禁用失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "禁用成功"})
}

// getUserID 从 context 获取用户ID
func (h *BiliAccountHandler) getUserID(c *gin.Context) uint {
	if userID, exists := c.Get("user_id"); exists {
		switch v := userID.(type) {
		case uint:
			return v
		case int:
			return uint(v)
		case float64:
			return uint(v)
		}
	}
	return 0
}
