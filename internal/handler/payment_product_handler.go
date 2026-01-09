// Package handler 支付产品 API Handler
package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/difyz9/ytb2bili/pkg/payment_client"
	"github.com/gin-gonic/gin"
)

// PaymentProductHandler 支付商品 API Handler
type PaymentProductHandler struct {
	productCache *payment_client.ProductCache
	payClient    *payment_client.Client
}

// PaymentConfig 支付配置
type PaymentConfig struct {
	Enabled               bool
	BaseURL               string
	AppID                 string
	AppSecret             string
	AllowedProductIDs     []string
	CacheDurationMinutes  int
	RequestTimeoutSeconds int
}

// NewPaymentProductHandler 创建支付商品 Handler
func NewPaymentProductHandler(config PaymentConfig) *PaymentProductHandler {
	if !config.Enabled {
		return nil
	}

	client := payment_client.New(config.BaseURL, config.AppID, config.AppSecret)

	cacheDuration := time.Duration(config.CacheDurationMinutes) * time.Minute
	if cacheDuration == 0 {
		cacheDuration = 10 * time.Minute
	}

	cache := payment_client.NewProductCache(client, config.AllowedProductIDs, cacheDuration)

	return &PaymentProductHandler{
		productCache: cache,
		payClient:    client,
	}
}

// RegisterRoutes 注册路由
func (h *PaymentProductHandler) RegisterRoutes(router *gin.RouterGroup) {
	if h == nil {
		return
	}

	payment := router.Group("/payment")
	{
		// 公开接口（无需登录）
		payment.GET("/products", h.GetProducts)
		payment.GET("/products/:productId", h.GetProduct)

		// 需要登录的接口
		payment.POST("/create-order", h.CreatePaymentOrder)
		payment.GET("/order-status", h.GetOrderStatus)
	}
}

// ProductResponse 商品响应
type ProductResponse struct {
	ProductID     string  `json:"product_id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Type          string  `json:"type"`
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"original_price"`
	VipDays       int     `json:"vip_days"`
	Icon          string  `json:"icon"`
	Status        string  `json:"status"`
}

// GetProducts 获取所有可用商品
func (h *PaymentProductHandler) GetProducts(c *gin.Context) {
	products, err := h.productCache.GetAllowedProducts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取商品列表失败: " + err.Error(),
		})
		return
	}

	// 转换响应格式
	var resp []ProductResponse
	for _, p := range products {
		resp = append(resp, ProductResponse{
			ProductID:     p.ProductID,
			Name:          p.Name,
			Description:   p.Description,
			Type:          p.Type,
			Price:         p.Price,
			OriginalPrice: p.OriginalPrice,
			VipDays:       p.VipDays,
			Icon:          p.Icon,
			Status:        p.Status,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    resp,
	})
}

// GetProduct 获取单个商品详情
func (h *PaymentProductHandler) GetProduct(c *gin.Context) {
	productID := c.Param("productId")

	// 检查是否在白名单
	if !h.productCache.IsProductAllowed(productID) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "商品不存在",
		})
		return
	}

	product, err := h.productCache.GetProduct(productID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取商品失败: " + err.Error(),
		})
		return
	}

	resp := ProductResponse{
		ProductID:     product.ProductID,
		Name:          product.Name,
		Description:   product.Description,
		Type:          product.Type,
		Price:         product.Price,
		OriginalPrice: product.OriginalPrice,
		VipDays:       product.VipDays,
		Icon:          product.Icon,
		Status:        product.Status,
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    resp,
	})
}

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	PayWay    string `json:"pay_way" binding:"required"` // wechat, alipay
}

// CreatePaymentOrder 创建支付订单
func (h *PaymentProductHandler) CreatePaymentOrder(c *gin.Context) {
	// 获取用户 ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 检查商品是否在白名单
	if !h.productCache.IsProductAllowed(req.ProductID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "商品不可用",
		})
		return
	}

	// 获取商品信息
	product, err := h.productCache.GetProduct(req.ProductID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取商品信息失败",
		})
		return
	}

	// 调用支付服务创建订单
	payResp, err := h.payClient.CreatePayment(payment_client.CreatePaymentRequest{
		Subject:   product.Name,
		Amount:    product.Price,
		PayWay:    req.PayWay,
		UserID:    fmt.Sprintf("%v", userID),
		OrderType: "vip",
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建订单失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"order_no": payResp.Data.OrderNo,
			"pay_url":  payResp.Data.PayURL,
			"amount":   payResp.Data.Amount,
			"pay_way":  payResp.Data.PayWay,
		},
	})
}

// GetOrderStatus 查询订单支付状态
func (h *PaymentProductHandler) GetOrderStatus(c *gin.Context) {
	orderID := c.Query("order_id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少 order_id 参数",
		})
		return
	}

	// 调用支付服务查询订单状态
	queryResp, err := h.payClient.QueryOrder(orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询订单状态失败: " + err.Error(),
		})
		return
	}

	// 转换状态
	var status string
	switch queryResp.Data.Status {
	case 0:
		status = "pending"
	case 1:
		status = "paid"
	case 2:
		status = "expired"
	default:
		status = "unknown"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"order_id": queryResp.Data.OrderNo,
			"status":   status,
			"amount":   queryResp.Data.Amount,
			"paid_at":  queryResp.Data.PaidAt,
		},
	})
}
