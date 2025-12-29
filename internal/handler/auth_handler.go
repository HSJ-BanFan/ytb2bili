package handler

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/difyz9/bilibili-go-sdk/bilibili"
	internalAuth "github.com/difyz9/ytb2bili/internal/auth"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/membership"
	"github.com/difyz9/ytb2bili/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
)

// currentTime 返回当前时间
func currentTime() time.Time {
	return time.Now()
}

type AuthHandler struct {
	BaseHandler
	FeatureChecker     *membership.FeatureChecker
	BiliAccountService *services.BiliAccountService
}

func NewAuthHandler(app *core.AppServer, featureChecker *membership.FeatureChecker, biliAccountService *services.BiliAccountService) *AuthHandler {
	return &AuthHandler{
		BaseHandler:        BaseHandler{App: app},
		FeatureChecker:     featureChecker,
		BiliAccountService: biliAccountService,
	}
}

// getUserID 从 context 获取用户ID
func (h *AuthHandler) getUserID(c *gin.Context) uint {
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

// RegisterRoutes 注册认证相关路由
func (h *AuthHandler) RegisterRoutes(server *core.AppServer, authMiddleware *internalAuth.AuthMiddleware) {
	api := server.Engine.Group("/api/v1")

	authGroup := api.Group("/auth")
	{
		authGroup.GET("/qrcode", h.getQRCode)
		authGroup.GET("/qrcode/image/:authCode", h.getQRCodeImage)
		authGroup.POST("/poll", h.pollQRCode)
		authGroup.GET("/login", h.loadLoginInfo)
		authGroup.GET("/status", h.checkLoginStatus)
		authGroup.GET("/userinfo", h.getUserInfo)
		authGroup.POST("/logout", h.logout)

		// 多账号管理 API（需要 JWT 认证，按用户隔离）
		accounts := authGroup.Group("/accounts")
		accounts.Use(authMiddleware.JWTAuth())
		{
			accounts.GET("", h.getAccounts)
			accounts.POST("", h.addAccount)
			accounts.DELETE("/:mid", h.removeAccount)
			accounts.PUT("/:mid/enable", h.setAccountEnabled)
			accounts.PUT("/:mid/primary", h.setPrimaryAccount)
		}
	}
}

// QRCodeRequest 二维码请求
type QRCodeRequest struct{}

// QRCodeResponse 二维码响应
type QRCodeResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	QRCodeURL string `json:"qr_code_url"`
	AuthCode  string `json:"auth_code"`
}

// getQRCode 获取登录二维码
func (h *AuthHandler) getQRCode(c *gin.Context) {
	client := bilibili.NewClient()

	qrResp, err := client.GetQRCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get QR code: " + err.Error(),
		})
		return
	}

	if qrResp.Code != 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    qrResp.Code,
			"message": "Failed to get QR code",
		})
		return
	}

	// 构造完整的二维码URL
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	host := c.Request.Host
	fullQRCodeURL := fmt.Sprintf("%s://%s/api/v1/auth/qrcode/image/%s", scheme, host, qrResp.Data.AuthCode)

	c.JSON(http.StatusOK, QRCodeResponse{
		Code:      0,
		Message:   "success",
		QRCodeURL: fullQRCodeURL,
		AuthCode:  qrResp.Data.AuthCode,
	})
}

// getQRCodeImage 生成二维码图片
func (h *AuthHandler) getQRCodeImage(c *gin.Context) {
	authCode := c.Param("authCode")
	if authCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Auth code is required",
		})
		return
	}

	// 构造B站二维码URL
	qrURL := fmt.Sprintf("https://passport.bilibili.com/x/passport-tv-login/h5/qrcode/auth?auth_code=%s", authCode)

	// 生成二维码图片
	qrCode, err := qrcode.New(qrURL, qrcode.Medium)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to generate QR code: " + err.Error(),
		})
		return
	}

	// 设置二维码颜色
	qrCode.BackgroundColor = color.RGBA{255, 255, 255, 255} // 白色背景
	qrCode.ForegroundColor = color.RGBA{0, 0, 0, 255}       // 黑色前景

	// 生成PNG图片
	img := qrCode.Image(240)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to encode QR code image: " + err.Error(),
		})
		return
	}

	// 设置响应头
	c.Header("Content-Type", "image/png")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// 返回图片数据
	c.Data(http.StatusOK, "image/png", buf.Bytes())
}

// PollQRCodeRequest 轮询二维码请求
type PollQRCodeRequest struct {
	AuthCode string `json:"auth_code" binding:"required"`
}

// PollQRCodeResponse 轮询二维码响应
type PollQRCodeResponse struct {
	Code      int                 `json:"code"`
	Message   string              `json:"message"`
	LoginInfo *bilibili.LoginInfo `json:"login_info,omitempty"`
}

// pollQRCode 轮询二维码登录状态
func (h *AuthHandler) pollQRCode(c *gin.Context) {
	var req PollQRCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters: " + err.Error(),
		})
		return
	}

	fmt.Println("--轮询二维码--")

	client := bilibili.NewClient()

	// 使用带超时的 context，避免请求长时间阻塞
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 在 goroutine 中执行 PollQRCode，带超时控制
	type pollResult struct {
		loginInfo *bilibili.LoginInfo
		err       error
	}
	resultChan := make(chan pollResult, 1)

	go func() {
		loginInfo, err := client.PollQRCode(req.AuthCode)
		resultChan <- pollResult{loginInfo, err}
	}()

	var loginInfo *bilibili.LoginInfo
	var err error

	select {
	case <-ctx.Done():
		// 超时，返回等待状态
		c.JSON(http.StatusOK, gin.H{
			"code":    86101,
			"message": "等待扫码中",
		})
		return
	case result := <-resultChan:
		loginInfo = result.loginInfo
		err = result.err
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Login failed: " + err.Error(),
		})
		return
	}

	// 获取用户完整信息并补充到LoginInfo中
	if loginInfo.TokenInfo.Mid > 0 {
		// 构建cookie字符串用于API调用
		cookies := buildCookieString(loginInfo.CookieInfo)

		// 优先使用myinfo API获取完整用户信息 (参考biliup-1.1.16)
		myInfo, err := client.GetMyInfoWithRetry(cookies, 2)
		if err == nil {
			// 使用myinfo API的完整信息
			loginInfo.TokenInfo.Uname = myInfo.Uname
			loginInfo.TokenInfo.Face = myInfo.Face
			if myInfo.Mid > 0 {
				loginInfo.TokenInfo.Mid = myInfo.Mid
			}
		} else {
			log.Printf("Warning: Failed to get myinfo: %v", err)
		}
	}

	// 扫码成功，保存登录信息到 BiliStore 供 bindFromQRCode 使用
	biliStore := internalAuth.GetBiliStore()
	if biliStore != nil {
		biliStore.Save(loginInfo)
		log.Printf("[pollQRCode] 登录信息已保存到 BiliStore")
	}

	// 前端收到后会调用 /auth/accounts POST 或 /bili-accounts/bind-from-qrcode 接口保存到数据库
	log.Printf("[pollQRCode] 扫码成功, Mid: %d, Name: %s", loginInfo.TokenInfo.Mid, loginInfo.TokenInfo.Uname)

	c.JSON(http.StatusOK, PollQRCodeResponse{
		Code:      0,
		Message:   "Login successful",
		LoginInfo: loginInfo,
	})
}

// LoadLoginInfoResponse 加载登录信息响应
type LoadLoginInfoResponse struct {
	Code      int                 `json:"code"`
	Message   string              `json:"message"`
	LoginInfo *bilibili.LoginInfo `json:"login_info,omitempty"`
}

// loadLoginInfo 从本地加载已保存的登录信息
func (h *AuthHandler) loadLoginInfo(c *gin.Context) {
	store := storage.GetDefaultStore()

	loginInfo, err := store.Load()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "No saved login info or login expired: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, LoadLoginInfoResponse{
		Code:      0,
		Message:   "Login info loaded successfully",
		LoginInfo: loginInfo,
	})
}

// CheckLoginStatusResponse 检查登录状态响应
type CheckLoginStatusResponse struct {
	Code       int       `json:"code"`
	Message    string    `json:"message"`
	IsLoggedIn bool      `json:"is_logged_in"`
	User       *UserInfo `json:"user,omitempty"`
}

type UserInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Mid    string `json:"mid"`
	Avatar string `json:"avatar"`
}

// checkLoginStatus 检查本地登录信息是否有效
func (h *AuthHandler) checkLoginStatus(c *gin.Context) {
	store := storage.GetDefaultStore()
	isValid := store.IsValid()

	response := CheckLoginStatusResponse{
		Code:       0,
		Message:    "success",
		IsLoggedIn: isValid,
	}

	// 如果已登录，返回用户信息
	if isValid {
		// 优先从缓存中获取用户信息
		cachedUserInfo, err := store.GetUserInfo()
		if err == nil && cachedUserInfo != nil {
			// 使用缓存的用户信息
			response.User = &UserInfo{
				ID:     fmt.Sprintf("%d", cachedUserInfo.Mid),
				Name:   cachedUserInfo.Name,
				Mid:    fmt.Sprintf("%d", cachedUserInfo.Mid),
				Avatar: cachedUserInfo.Face,
			}
		} else {
			// 没有缓存的用户信息，从API获取
			loginInfo, _ := store.Load()
			if loginInfo != nil {
				client := bilibili.NewClient()

				// 构建cookie字符串
				cookies := buildCookieString(loginInfo.CookieInfo)

				// 尝试使用myinfo API获取完整用户信息 (参考biliup-1.1.16)
				userName := fmt.Sprintf("用户_%d", loginInfo.TokenInfo.Mid) // 默认用户名
				userAvatar := ""
				userMid := fmt.Sprintf("%d", loginInfo.TokenInfo.Mid)

				// 如果登录信息中有用户名，使用它
				if loginInfo.TokenInfo.Uname != "" {
					userName = loginInfo.TokenInfo.Uname
				}
				if loginInfo.TokenInfo.Face != "" {
					userAvatar = loginInfo.TokenInfo.Face
				}

				var userBasicInfo *storage.UserBasicInfo

				// 优先使用myinfo API获取最新用户信息
				myInfo, err := client.GetMyInfoWithRetry(cookies, 2)
				if err == nil {
					// 使用myinfo API的完整信息
					userName = myInfo.Uname
					userAvatar = myInfo.Face
					userMid = fmt.Sprintf("%d", myInfo.Mid)

					// 更新并保存登录信息和用户信息
					loginInfo.TokenInfo.Uname = myInfo.Uname
					loginInfo.TokenInfo.Face = myInfo.Face
					if myInfo.Mid > 0 {
						loginInfo.TokenInfo.Mid = myInfo.Mid
					}
					userBasicInfo = storage.ConvertMyInfoToUserInfo(myInfo)
				} else {
					log.Printf("Warning: Failed to get myinfo: %v", err)
				} // 保存更新后的信息（包括用户信息）
				if userBasicInfo != nil {
					store.SaveWithUserInfo(loginInfo, userBasicInfo)
				} else {
					store.Save(loginInfo)
				}

				response.User = &UserInfo{
					ID:     userMid,
					Name:   userName,
					Mid:    userMid,
					Avatar: userAvatar,
				}
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetUserInfoResponse 获取用户信息响应
type GetUserInfoResponse struct {
	Code     int                     `json:"code"`
	Message  string                  `json:"message"`
	UserInfo *bilibili.UserBasicInfo `json:"user_info,omitempty"`
}

// getUserInfo 获取当前登录用户的详细信息
func (h *AuthHandler) getUserInfo(c *gin.Context) {
	store := storage.GetDefaultStore()
	if !store.IsValid() {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "User not logged in",
		})
		return
	}

	loginInfo, err := store.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to load login info: " + err.Error(),
		})
		return
	}

	client := bilibili.NewClient()

	// 构建cookie字符串
	cookies := buildCookieString(loginInfo.CookieInfo)

	// 优先使用myinfo API获取用户信息 (参考biliup-1.1.16)
	myInfo, err := client.GetMyInfoWithRetry(cookies, 3)
	if err != nil {
		log.Printf("Failed to get myinfo: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get user info: " + err.Error(),
		})
		return
	}

	// 使用myinfo API返回的完整信息
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    myInfo,
	})
}

// LogoutResponse 登出响应
type LogoutResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// logout 删除本地保存的登录信息（登出）
func (h *AuthHandler) logout(c *gin.Context) {
	store := storage.GetDefaultStore()

	if err := store.Delete(); err != nil {
		log.Printf("Warning: Failed to delete login info: %v", err)
	}

	c.JSON(http.StatusOK, LogoutResponse{
		Code:    0,
		Message: "Logout successful",
	})
}

// buildCookieString 构建正确的cookie字符串
func buildCookieString(cookieInfo map[string]interface{}) string {
	if cookieInfo == nil {
		return ""
	}

	// 检查是否是新的数组格式
	if cookies, ok := cookieInfo["cookies"].([]interface{}); ok {
		cookieParts := []string{}
		for _, cookie := range cookies {
			if cookieMap, ok := cookie.(map[string]interface{}); ok {
				if name, nameOk := cookieMap["name"].(string); nameOk {
					if value, valueOk := cookieMap["value"].(string); valueOk {
						cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", name, value))
					}
				}
			}
		}
		if len(cookieParts) > 0 {
			return strings.Join(cookieParts, "; ")
		}
	}

	// 回退到旧的key-value格式处理
	cookieParts := []string{}
	for key, value := range cookieInfo {
		if key == "cookies" || key == "domains" {
			continue // 跳过特殊字段
		}
		if valueStr, ok := value.(string); ok {
			cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", key, valueStr))
		}
	}

	if len(cookieParts) > 0 {
		return strings.Join(cookieParts, "; ")
	}

	return ""
}

// ============== 多账号管理 API ==============

// AccountInfo 账号信息（用于 API 响应）
type AccountInfo struct {
	ID        string `json:"id"`
	Mid       int64  `json:"mid"`
	Name      string `json:"name"`
	Face      string `json:"face"`
	IsEnabled bool   `json:"is_enabled"`
	IsPrimary bool   `json:"is_primary"`
	IsExpired bool   `json:"is_expired"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

// getAccounts 获取当前用户的所有账号列表（按用户隔离）
func (h *AuthHandler) getAccounts(c *gin.Context) {
	userID := h.getUserID(c)
	log.Printf("[getAccounts] 获取用户账号列表, userID=%d", userID)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	// 从数据库获取当前用户的账号
	accounts, err := h.BiliAccountService.GetUserAccounts(userID)
	log.Printf("[getAccounts] 查询结果: userID=%d, 账号数量=%d, err=%v", userID, len(accounts), err)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get accounts: " + err.Error(),
		})
		return
	}

	// 转换为 API 响应格式
	var accountInfos []AccountInfo
	now := currentTime()
	for _, acc := range accounts {
		isExpired := false
		expiresAt := ""
		if acc.ExpiresAt != nil {
			isExpired = now.After(*acc.ExpiresAt)
			expiresAt = acc.ExpiresAt.Format("2006-01-02 15:04:05")
		}
		accountInfos = append(accountInfos, AccountInfo{
			ID:        strconv.FormatUint(uint64(acc.ID), 10),
			Mid:       acc.BiliMid,
			Name:      acc.BiliName,
			Face:      acc.BiliFace,
			IsEnabled: acc.IsEnabled,
			IsPrimary: acc.IsPrimary,
			IsExpired: isExpired,
			CreatedAt: acc.CreatedAt.Format("2006-01-02 15:04:05"),
			ExpiresAt: expiresAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":     0,
		"message":  "success",
		"accounts": accountInfos,
	})
}

// AddAccountRequest 添加账号请求（接收前端传来的 login_info）
type AddAccountRequest struct {
	LoginInfo *bilibili.LoginInfo `json:"login_info"`
}

// addAccount 添加新账号（通过扫码登录后调用，按用户隔离）
func (h *AuthHandler) addAccount(c *gin.Context) {
	userID := h.getUserID(c)
	log.Printf("[addAccount] 添加账号, userID=%d", userID)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	// 先读取原始请求体用于调试
	bodyBytes, _ := c.GetRawData()
	log.Printf("[addAccount] 原始请求体: %s", string(bodyBytes))

	// 重新设置请求体供后续解析
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var req AddAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[addAccount] JSON解析失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters: " + err.Error(),
		})
		return
	}

	log.Printf("[addAccount] 解析后 req.LoginInfo=%v", req.LoginInfo)

	if req.LoginInfo == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "login_info is required",
		})
		return
	}

	loginInfo := req.LoginInfo
	log.Printf("[addAccount] 收到 login_info, Mid=%d, Name=%s", loginInfo.TokenInfo.Mid, loginInfo.TokenInfo.Uname)

	// 获取现有账号数量（用于判断是否设为主账号）
	existingAccounts, _ := h.BiliAccountService.GetUserAccounts(userID)
	// 注意：所有用户都可以绑定多个账号，只是非企业版上传时只用主账号

	// 检查是否为第一个账号（设为主账号）
	isPrimary := len(existingAccounts) == 0

	// 保存到数据库
	account, err := h.BiliAccountService.BindAccount(userID, loginInfo, isPrimary)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to add account: " + err.Error(),
		})
		return
	}

	// 同时保存到旧的本地存储（保持向后兼容）
	if isPrimary {
		multiStore := storage.GetMultiAccountStore()
		multiStore.AddAccount(loginInfo, nil)
	}

	expiresAt := ""
	if account.ExpiresAt != nil {
		expiresAt = account.ExpiresAt.Format("2006-01-02 15:04:05")
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Account added successfully",
		"account": AccountInfo{
			ID:        strconv.FormatUint(uint64(account.ID), 10),
			Mid:       account.BiliMid,
			Name:      account.BiliName,
			Face:      account.BiliFace,
			IsEnabled: account.IsEnabled,
			IsPrimary: account.IsPrimary,
			IsExpired: false,
			CreatedAt: account.CreatedAt.Format("2006-01-02 15:04:05"),
			ExpiresAt: expiresAt,
		},
	})
}

// removeAccount 删除账号（按用户隔离，通过B站MID）
func (h *AuthHandler) removeAccount(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	midStr := c.Param("mid")
	biliMid, err := strconv.ParseInt(midStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid B站 MID",
		})
		return
	}

	if err := h.BiliAccountService.UnbindAccountByMid(userID, biliMid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to remove account: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Account removed successfully",
	})
}

// SetAccountEnabledRequest 设置账号启用状态请求
type SetAccountEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// setAccountEnabled 设置账号启用/禁用状态（按用户隔离，通过B站MID）
func (h *AuthHandler) setAccountEnabled(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	midStr := c.Param("mid")
	biliMid, err := strconv.ParseInt(midStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid B站 MID",
		})
		return
	}

	var req SetAccountEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters: " + err.Error(),
		})
		return
	}

	var updateErr error
	if req.Enabled {
		updateErr = h.BiliAccountService.EnableAccountByMid(userID, biliMid)
	} else {
		updateErr = h.BiliAccountService.DisableAccountByMid(userID, biliMid)
	}

	if updateErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update account: " + updateErr.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Account updated successfully",
	})
}

// setPrimaryAccount 设置主账号（按用户隔离，通过B站MID）
func (h *AuthHandler) setPrimaryAccount(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	midStr := c.Param("mid")
	biliMid, err := strconv.ParseInt(midStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid B站 MID",
		})
		return
	}

	if err := h.BiliAccountService.SetPrimaryAccountByMid(userID, biliMid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to set primary account: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Primary account set successfully",
	})
}
