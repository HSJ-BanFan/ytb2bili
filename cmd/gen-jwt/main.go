package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println("═══════════════════════════════════════")
	fmt.Println("🔑 JWT 密钥生成工具")
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()

	// 生成64字节（512位）的随机密钥
	key, err := generateSecureKey(64)
	if err != nil {
		fmt.Printf("❌ 生成密钥失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ 安全的 JWT 密钥已生成:")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════")
	fmt.Println(key)
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()
	fmt.Println("📝 使用说明:")
	fmt.Println()
	fmt.Println("1. 将以下内容添加到 config.toml:")
	fmt.Println()
	fmt.Println("   [auth]")
	fmt.Println("     jwt_secret = \"", key, "\"")
	fmt.Println()
	fmt.Println("2. 同时也建议修改 session_secret:")
	fmt.Println()
	fmt.Println("   [auth]")
	fmt.Println("     session_secret = \"", key, "\"")
	fmt.Println()
	fmt.Println("3. 重启服务使配置生效:")
	fmt.Println("   ./ytb2bili.exe")
	fmt.Println()
	fmt.Println("⚠️  重要提示:")
	fmt.Println("   - 请妥善保管此密钥，不要泄露给他人")
	fmt.Println("   - 密钥一旦设置，请勿随意修改，否则会导致所有用户的 token 失效")
	fmt.Println("   - 建议定期轮换密钥（如每6个月）")
	fmt.Println()
}

// generateSecureKey 生成安全的随机密钥
func generateSecureKey(length int) (string, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(bytes)), nil
}
