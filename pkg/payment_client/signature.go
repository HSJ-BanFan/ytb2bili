package payment_client

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateSignature 生成 GoAuth 签名
// 签名字符串格式: appId=xxx&nonce=xxx&timestamp=xxx（按字母顺序 a-n-t）
func GenerateSignature(appID, appSecret, timestamp, nonce string) string {
	// 按字母顺序拼接参数：appId → nonce → timestamp
	signStr := fmt.Sprintf("appId=%s&nonce=%s&timestamp=%s", appID, nonce, timestamp)

	// HMAC-SHA256 加密
	h := hmac.New(sha256.New, []byte(appSecret))
	h.Write([]byte(signStr))

	// 转换为小写十六进制字符串
	return hex.EncodeToString(h.Sum(nil))
}
