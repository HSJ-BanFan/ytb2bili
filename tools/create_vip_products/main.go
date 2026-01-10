// 临时脚本：创建 VIP 商品
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL   = "http://localhost:8098"
	appID     = "admin-frontend"
	appSecret = "admin-frontend-secret-key-32chars"
)

type CreateProductRequest struct {
	ProductId     string  `json:"productId"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Type          string  `json:"type"`
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"originalPrice"`
	VipDays       int     `json:"vipDays"`
	Icon          string  `json:"icon"`
	Sort          int     `json:"sort"`
}

func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateSignature(appID, appSecret, timestamp, nonce string) string {
	signStr := fmt.Sprintf("appId=%s&nonce=%s&timestamp=%s", appID, nonce, timestamp)
	h := hmac.New(sha256.New, []byte(appSecret))
	h.Write([]byte(signStr))
	return hex.EncodeToString(h.Sum(nil))
}

func createProduct(product CreateProductRequest) error {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := generateNonce()
	signature := generateSignature(appID, appSecret, timestamp, nonce)

	body, _ := json.Marshal(product)
	req, _ := http.NewRequest("POST", baseURL+"/api/v2/products", bytes.NewReader(body))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Id", appID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Sign", signature)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("创建商品 %s: %s\n", product.ProductId, string(respBody))
	return nil
}

func main() {
	fmt.Println("═══════════════════════════════════════")
	fmt.Println("🛍️ 创建 VIP 商品")
	fmt.Println("═══════════════════════════════════════")

	products := []CreateProductRequest{
		{
			ProductId:     "ytb2bili_vip_free",
			Name:          "免费版",
			Description:   "基础功能，每日5个视频，适合个人体验",
			Type:          "vip",
			Price:         0,
			OriginalPrice: 0,
			VipDays:       0, // 永久
			Icon:          "🆓",
			Sort:          1,
		},
		{
			ProductId:     "ytb2bili_vip_enterprise_monthly",
			Name:          "企业版月卡",
			Description:   "完整功能，无限视频，适合专业用户",
			Type:          "vip",
			Price:         29.88,
			OriginalPrice: 99,
			VipDays:       30,
			Icon:          "👑",
			Sort:          2,
		},
		{
			ProductId:     "ytb2bili_vip_enterprise_yearly",
			Name:          "企业版年卡",
			Description:   "完整功能，无限视频，买10个月送2个月",
			Type:          "vip",
			Price:         298.80,
			OriginalPrice: 1188,
			VipDays:       365,
			Icon:          "💎",
			Sort:          3,
		},
	}

	for _, p := range products {
		if err := createProduct(p); err != nil {
			fmt.Printf("❌ 创建 %s 失败: %v\n", p.ProductId, err)
		}
	}

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("✅ 商品创建完成！")
}
