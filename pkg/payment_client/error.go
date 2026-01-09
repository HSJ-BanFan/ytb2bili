package payment_client

import "fmt"

// PaymentError 支付错误
type PaymentError struct {
	Code    string // 错误码，如 "SIGNATURE_INVALID"
	Message string // 错误信息
	Err     error  // 原始错误
}

func (e *PaymentError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *PaymentError) Unwrap() error {
	return e.Err
}

// 预定义错误码
const (
	ErrCodeNetworkError     = "NETWORK_ERROR"
	ErrCodeSignatureInvalid = "SIGNATURE_INVALID"
	ErrCodeTimestampExpired = "TIMESTAMP_EXPIRED"
	ErrCodeAppIDNotFound    = "APP_ID_NOT_FOUND"
	ErrCodeIPNotInWhitelist = "IP_NOT_IN_WHITELIST"
	ErrCodeRateLimitExceed  = "RATE_LIMIT_EXCEEDED"
	ErrCodeInvalidRequest   = "INVALID_REQUEST"
	ErrCodeServerError      = "SERVER_ERROR"
)

// NewPaymentError 创建支付错误
func NewPaymentError(code, message string, err error) *PaymentError {
	return &PaymentError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
