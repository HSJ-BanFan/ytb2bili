package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GoAuthMiddleware GoAuth认证中间件
type GoAuthMiddleware struct {
	config *types.GoAuthConfig
	logger *zap.SugaredLogger
}

// NewGoAuthMiddleware 创建GoAuth认证中间件
func NewGoAuthMiddleware(config *types.GoAuthConfig, logger *zap.SugaredLogger) *GoAuthMiddleware {
	return &GoAuthMiddleware{
		config: config,
		logger: logger,
	}
}

// Handle 处理请求
func (m *GoAuthMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取 Authorization Header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Authorization header is required",
			})
			c.Abort()
			return
		}

		// 2. 解析 Authorization Header
		params, err := parseAuthHeader(authHeader)
		if err != nil {
			m.logger.Warnf("GoAuth: Failed to parse auth header: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid Authorization header format",
			})
			c.Abort()
			return
		}

		appID := params["AppID"]
		if appID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "AppID is required",
			})
			c.Abort()
			return
		}

		// 3. 查找 App 配置
		var appConfig *types.GoAuthApp
		for _, app := range m.config.Apps {
			if app.AppID == appID {
				appConfig = &app
				break
			}
		}

		if appConfig == nil || !appConfig.Enabled {
			m.logger.Warnf("GoAuth: Invalid AppID or App is disabled: %s", appID)
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid AppID or App is disabled",
			})
			c.Abort()
			return
		}

		// 4. 检查IP白名单
		if m.config.EnableIPCheck && len(appConfig.IPWhitelist) > 0 {
			clientIP := c.ClientIP()
			allowed := false
			for _, ip := range appConfig.IPWhitelist {
				if ip == clientIP {
					allowed = true
					break
				}
			}
			if !allowed {
				m.logger.Warnf("GoAuth: Access denied for IP %s (AppID: %s)", clientIP, appID)
				c.JSON(http.StatusForbidden, gin.H{
					"code":    403,
					"message": "IP address not allowed",
				})
				c.Abort()
				return
			}
		}

		// 5. 验证时间戳
		timestampStr := params["Timestamp"]
		if timestampStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Timestamp is required",
			})
			c.Abort()
			return
		}

		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid timestamp format",
			})
			c.Abort()
			return
		}

		tolerance := m.config.TimestampTolerance
		if tolerance <= 0 {
			tolerance = 300 // 默认 5 分钟
		}
		if !isTimestampValid(timestamp, tolerance) {
			m.logger.Warnf("GoAuth: Timestamp expired or invalid: %d (AppID: %s)", timestamp, appID)
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Timestamp expired or invalid",
			})
			c.Abort()
			return
		}

		// 6. 验证 Nonce
		nonce := params["Nonce"]
		if nonce == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Nonce is required",
			})
			c.Abort()
			return
		}

		// 7. 获取签名
		signature := params["Signature"]
		if signature == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Signature is required",
			})
			c.Abort()
			return
		}

		// 8. 读取请求体（如果需要）
		var body []byte
		signIncludeBody := m.config.SignIncludeBody
		if appConfig.SignIncludeBody != nil {
			signIncludeBody = *appConfig.SignIncludeBody
		}

		if signIncludeBody && (c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH") {
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    500,
					"message": "Failed to read request body",
				})
				c.Abort()
				return
			}
			// 重新写回 Body，供后续 Handler 使用
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		}

		// 9. 计算并验证签名
		expectedSignature := calculateSignature(
			appID,
			appConfig.AppSecret,
			timestampStr,
			nonce,
			c.Request.Method,
			c.Request.URL.Path,
			c.Request.URL.RawQuery,
			body,
		)

		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			m.logger.Warnf("GoAuth: Signature mismatch (AppID: %s)", appID)
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Signature verification failed",
			})
			c.Abort()
			return
		}

		// 认证通过，将 AppID 注入上下文
		c.Set("goauth_app_id", appID)
		c.Set("goauth_app_name", appConfig.AppName)

		m.logger.Debugf("GoAuth: Request authenticated (AppID: %s)", appID)
		c.Next()
	}
}

// parseAuthHeader 解析 Authorization header
// 格式: GOAUTH-V1-HMAC-SHA256 AppID=xxx;Timestamp=xxx;Nonce=xxx;Signature=xxx
func parseAuthHeader(header string) (map[string]string, error) {
	result := make(map[string]string)

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid header format")
	}

	scheme := parts[0]
	if scheme != "GOAUTH-V1-HMAC-SHA256" {
		return nil, fmt.Errorf("unsupported auth scheme: %s", scheme)
	}

	// 解析参数
	params := strings.Split(parts[1], ";")
	for _, param := range params {
		kv := strings.SplitN(strings.TrimSpace(param), "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}

	return result, nil
}

// calculateSignature 计算 HMAC-SHA256 签名
func calculateSignature(appID, appSecret, timestamp, nonce, method, path, query string, body []byte) string {
	// 构建待签名字符串
	// 格式: METHOD\nPATH\nQUERY\nTIMESTAMP\nNONCE\nBODY
	var signParts []string
	signParts = append(signParts, method)
	signParts = append(signParts, path)

	// 对查询参数排序
	if query != "" {
		sortedQuery := sortQueryParams(query)
		signParts = append(signParts, sortedQuery)
	} else {
		signParts = append(signParts, "")
	}

	signParts = append(signParts, timestamp)
	signParts = append(signParts, nonce)

	if len(body) > 0 {
		signParts = append(signParts, string(body))
	} else {
		signParts = append(signParts, "")
	}

	stringToSign := strings.Join(signParts, "\n")

	// 使用 HMAC-SHA256 计算签名
	h := hmac.New(sha256.New, []byte(appSecret))
	h.Write([]byte(stringToSign))
	return hex.EncodeToString(h.Sum(nil))
}

// sortQueryParams 对查询参数进行排序
func sortQueryParams(query string) string {
	params := strings.Split(query, "&")
	sort.Strings(params)
	return strings.Join(params, "&")
}

// isTimestampValid 检查时间戳是否在允许范围内
func isTimestampValid(timestamp int64, toleranceSeconds int64) bool {
	now := time.Now().Unix()
	diff := now - timestamp
	if diff < 0 {
		diff = -diff
	}
	return diff <= toleranceSeconds
}
