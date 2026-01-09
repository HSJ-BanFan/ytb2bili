package payment_client

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client 支付客户端
type Client struct {
	BaseURL    string       // 支付服务基础 URL
	AppID      string       // 应用 ID
	AppSecret  string       // 应用密钥
	HTTPClient *http.Client // HTTP 客户端
}

// Config 客户端配置（用于从配置文件读取）
type Config struct {
	BaseURL   string `toml:"base_url"`
	AppID     string `toml:"app_id"`
	AppSecret string `toml:"app_secret"`
	Timeout   int    `toml:"timeout"` // 超时时间（秒）
}

// New 创建支付客户端
func New(baseURL, appID, appSecret string) *Client {
	return &Client{
		BaseURL:   baseURL,
		AppID:     appID,
		AppSecret: appSecret,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewFromConfig 从配置创建客户端
func NewFromConfig(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	return &Client{
		BaseURL:   cfg.BaseURL,
		AppID:     cfg.AppID,
		AppSecret: cfg.AppSecret,
		HTTPClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// GenerateNonce 生成随机字符串（32位十六进制）
func (c *Client) GenerateNonce() string {
	b := make([]byte, 16) // 16 字节 = 32 个十六进制字符
	rand.Read(b)
	return hex.EncodeToString(b)
}

// getAuthHeaders 获取认证请求头
func (c *Client) getAuthHeaders() map[string]string {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := c.GenerateNonce()
	signature := GenerateSignature(c.AppID, c.AppSecret, timestamp, nonce)

	return map[string]string{
		"Content-Type": "application/json",
		"X-App-Id":     c.AppID,
		"X-Timestamp":  timestamp,
		"X-Nonce":      nonce,
		"X-Sign":       signature,
	}
}

// doRequest 执行 HTTP 请求
func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, NewPaymentError(ErrCodeInvalidRequest, "请求序列化失败", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, NewPaymentError(ErrCodeNetworkError, "创建请求失败", err)
	}

	// 添加认证头
	for k, v := range c.getAuthHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, NewPaymentError(ErrCodeNetworkError, "请求发送失败", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewPaymentError(ErrCodeNetworkError, "读取响应失败", err)
	}

	// 根据 HTTP 状态码判断错误
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, NewPaymentError(ErrCodeSignatureInvalid, "签名验证失败", nil)
	case http.StatusForbidden:
		return nil, NewPaymentError(ErrCodeIPNotInWhitelist, "IP不在白名单", nil)
	case http.StatusTooManyRequests:
		return nil, NewPaymentError(ErrCodeRateLimitExceed, "请求过于频繁", nil)
	case http.StatusInternalServerError:
		return nil, NewPaymentError(ErrCodeServerError, "服务器内部错误", nil)
	}

	return respBody, nil
}

// CreatePayment 发起支付
func (c *Client) CreatePayment(req CreatePaymentRequest) (*CreatePaymentResponse, error) {
	respBody, err := c.doRequest("POST", "/api/v1/payment/pay", req)
	if err != nil {
		return nil, err
	}

	var result CreatePaymentResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, NewPaymentError(ErrCodeServerError, "响应解析失败", err)
	}

	return &result, nil
}

// QueryOrder 查询订单
func (c *Client) QueryOrder(orderNo string) (*QueryOrderResponse, error) {
	respBody, err := c.doRequest("GET", "/api/v1/payment/query/"+orderNo, nil)
	if err != nil {
		return nil, err
	}

	var result QueryOrderResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, NewPaymentError(ErrCodeServerError, "响应解析失败", err)
	}

	return &result, nil
}

// GetOrders 获取订单列表
func (c *Client) GetOrders(page, pageSize int, userID string) (*OrderListResponse, error) {
	path := fmt.Sprintf("/api/v2/orders?page=%d&pageSize=%d", page, pageSize)
	if userID != "" {
		path += "&userId=" + userID
	}

	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result OrderListResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, NewPaymentError(ErrCodeServerError, "响应解析失败", err)
	}

	return &result, nil
}

// Product 商品信息
type Product struct {
	ID            uint    `json:"id"`
	ProductID     string  `json:"productId"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Type          string  `json:"type"` // coin, vip, combo
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"originalPrice"`
	CoinAmount    int     `json:"coinAmount"`
	BonusCoin     int     `json:"bonusCoin"`
	VipDays       int     `json:"vipDays"`
	Icon          string  `json:"icon"`
	Sort          int     `json:"sort"`
	Status        string  `json:"status"` // active, inactive
	Sales         int64   `json:"sales"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
}

// ProductResponse 商品响应
type ProductResponse struct {
	Code    int     `json:"code"`
	Message string  `json:"message"`
	Data    Product `json:"data"`
}

// ProductListResponse 商品列表响应
type ProductListResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []Product `json:"list"`
		Total    int64     `json:"total"`
		Page     int       `json:"page"`
		PageSize int       `json:"pageSize"`
	} `json:"data"`
}

// GetProduct 获取单个商品详情
func (c *Client) GetProduct(productID string) (*Product, error) {
	respBody, err := c.doRequest("GET", "/api/v2/products/"+productID, nil)
	if err != nil {
		return nil, err
	}

	var result ProductResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, NewPaymentError(ErrCodeServerError, "响应解析失败", err)
	}

	if result.Code != 200 {
		return nil, NewPaymentError(ErrCodeServerError, result.Message, nil)
	}

	return &result.Data, nil
}

// GetProducts 获取商品列表（按类型筛选）
func (c *Client) GetProducts(productType, status string, page, pageSize int) (*ProductListResponse, error) {
	path := fmt.Sprintf("/api/v2/products?page=%d&pageSize=%d", page, pageSize)
	if productType != "" {
		path += "&type=" + productType
	}
	if status != "" {
		path += "&status=" + status
	}

	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result ProductListResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, NewPaymentError(ErrCodeServerError, "响应解析失败", err)
	}

	return &result, nil
}

// GetProductsByIDs 批量获取指定ID的商品
func (c *Client) GetProductsByIDs(productIDs []string) ([]Product, error) {
	var products []Product
	for _, id := range productIDs {
		product, err := c.GetProduct(id)
		if err != nil {
			// 跳过获取失败的商品，记录日志
			continue
		}
		if product.Status == "active" {
			products = append(products, *product)
		}
	}
	return products, nil
}
