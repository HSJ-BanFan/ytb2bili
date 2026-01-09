package payment_client

// CreatePaymentRequest 创建支付请求
type CreatePaymentRequest struct {
	Subject   string  `json:"subject"`             // 必填：商品标题
	Amount    float64 `json:"amount"`              // 必填：金额（元）
	PayWay    string  `json:"payWay"`              // 必填：支付方式 alipay/wechat/paypal
	UserID    string  `json:"userId,omitempty"`    // 可选：用户ID
	OrderType string  `json:"orderType,omitempty"` // 可选：订单类型
	Extra     string  `json:"extra,omitempty"`     // 可选：额外信息（JSON字符串）
}

// CreatePaymentResponse 创建支付响应
type CreatePaymentResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		PayURL  string  `json:"payUrl"`  // 支付链接
		OrderNo string  `json:"orderNo"` // 订单号
		Amount  float64 `json:"amount"`  // 金额
		PayWay  string  `json:"payWay"`  // 支付方式
	} `json:"data"`
}

// QueryOrderResponse 查询订单响应
type QueryOrderResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		OrderID   string  `json:"orderId"`
		OrderNo   string  `json:"orderNo"`
		Subject   string  `json:"subject"`
		Amount    float64 `json:"amount"`
		Status    int     `json:"status"` // 0-未支付 1-已支付 2-已关闭
		PayWay    string  `json:"payWay"`
		UserID    string  `json:"userId"`
		CreatedAt string  `json:"createdAt"`
		PaidAt    string  `json:"paidAt"`
	} `json:"data"`
}

// OrderListResponse 订单列表响应
type OrderListResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []OrderItem `json:"list"`
		Total    int         `json:"total"`
		Page     int         `json:"page"`
		PageSize int         `json:"pageSize"`
	} `json:"data"`
}

// OrderItem 订单项
type OrderItem struct {
	OrderID   string  `json:"orderId"`
	OrderNo   string  `json:"orderNo"`
	Subject   string  `json:"subject"`
	Amount    float64 `json:"amount"`
	Status    int     `json:"status"`
	PayWay    string  `json:"payWay"`
	UserID    string  `json:"userId"`
	CreatedAt string  `json:"createdAt"`
	PaidAt    string  `json:"paidAt"`
}
