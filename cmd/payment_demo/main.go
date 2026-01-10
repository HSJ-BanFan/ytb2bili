// Package main 支付演示程序
// 用于测试与 pay-unify 支付服务的集成
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/difyz9/ytb2bili/pkg/payment_client"
)

func main() {
	// 配置（生产环境应从配置文件或环境变量读取）
	// 注意：证书在远程服务器上，所以直接调用远程 API
	baseURL := getEnvOrDefault("PAYMENT_BASE_URL", "https://api.vtranslink.com")
	appID := getEnvOrDefault("PAYMENT_APP_ID", "admin-frontend")
	appSecret := getEnvOrDefault("PAYMENT_APP_SECRET", "admin-frontend-secret-key-32chars")

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("💳 支付服务演示程序")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("🔗 服务地址: %s\n", baseURL)
	fmt.Printf("🔑 AppID: %s\n", appID)
	fmt.Println("───────────────────────────────────────")

	// 初始化客户端
	client := payment_client.New(baseURL, appID, appSecret)

	// 创建 VIP月卡 订单
	fmt.Println("\n📝 创建支付订单...")
	resp, err := client.CreatePayment(payment_client.CreatePaymentRequest{
		Subject:   "VIP月卡",
		Amount:    29.9,
		PayWay:    "wechat", // 可选: alipay, wechat, paypal
		UserID:    "demo_user_001",
		OrderType: "vip",
	})

	if err != nil {
		log.Fatalf("❌ 创建支付订单失败: %v", err)
	}

	if resp.Code == 200 {
		fmt.Println("\n✅ 支付订单创建成功！")
		fmt.Println("───────────────────────────────────────")
		fmt.Printf("📋 订单号:   %s\n", resp.Data.OrderNo)
		fmt.Printf("💰 金额:     ¥%.2f\n", resp.Data.Amount)
		fmt.Printf("💳 支付方式: %s\n", resp.Data.PayWay)
		fmt.Println("───────────────────────────────────────")
		fmt.Printf("🔗 支付链接:\n%s\n", resp.Data.PayURL)
		fmt.Println("═══════════════════════════════════════")
		fmt.Println("\n💡 请复制上面的链接到浏览器完成支付")
	} else {
		log.Fatalf("❌ 支付失败: [%d] %s", resp.Code, resp.Message)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
