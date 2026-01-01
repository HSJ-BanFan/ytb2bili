package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// EncryptionService 加密服务 (AES-256-GCM)
type EncryptionService struct {
	key []byte // 32 bytes for AES-256
	mu  sync.RWMutex
}

var (
	globalEncryptionService *EncryptionService
	encryptionServiceOnce   sync.Once
)

// GetEncryptionService 获取加密服务单例
func GetEncryptionService() (*EncryptionService, error) {
	var initErr error
	encryptionServiceOnce.Do(func() {
		globalEncryptionService, initErr = initEncryptionService()
	})
	if initErr != nil {
		return nil, initErr
	}
	return globalEncryptionService, nil
}

// initEncryptionService 初始化加密服务
func initEncryptionService() (*EncryptionService, error) {
	// 优先级 1: 从环境变量读取
	if keyStr := os.Getenv("COOKIE_ENCRYPTION_KEY"); keyStr != "" {
		key := []byte(keyStr)
		if len(key) != 32 {
			return nil, fmt.Errorf(
				"COOKIE_ENCRYPTION_KEY 必须是 32 字节 (AES-256)，当前长度: %d\n"+
					"生成方法: openssl rand -base64 32 | head -c 32",
				len(key),
			)
		}
		log.Println("✅ 加密密钥已从环境变量 COOKIE_ENCRYPTION_KEY 读取")
		return &EncryptionService{key: key}, nil
	}

	// 优先级 2: 从文件读取或自动生成
	keyFilePath := getKeyFilePath()

	// 尝试读取现有密钥
	if keyData, err := os.ReadFile(keyFilePath); err == nil {
		if len(keyData) >= 32 {
			log.Printf("✅ 加密密钥已从文件加载: %s", keyFilePath)
			return &EncryptionService{key: keyData[:32]}, nil
		}
	}

	// 自动生成新密钥（带警告）
	log.Println("═════════════════════════════════════════════════════════")
	log.Println("⚠️  正在自动生成加密密钥")
	log.Printf("📁 密钥将保存到: %s", keyFilePath)
	log.Println("🔐 请立即备份此文件到安全位置（U盘/云盘/密码管理器）")
	log.Println("❌ 如果密钥文件丢失，加密的账号数据将无法恢复！")
	log.Println("═════════════════════════════════════════════════════════")

	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		return nil, fmt.Errorf("生成随机密钥失败: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(keyFilePath), 0700); err != nil {
		return nil, fmt.Errorf("创建密钥目录失败: %w", err)
	}

	// 保存密钥文件（权限 0600 = 仅所有者可读写）
	if err := os.WriteFile(keyFilePath, newKey, 0600); err != nil {
		return nil, fmt.Errorf("保存密钥文件失败: %w", err)
	}

	log.Printf("✅ 加密密钥已生成并保存到: %s", keyFilePath)
	return &EncryptionService{key: newKey}, nil
}

// getKeyFilePath 获取密钥文件路径
func getKeyFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, ".bili_up", ".encryption_key")
}

// Encrypt 加密数据并返回 base64 编码的密文
func (s *EncryptionService) Encrypt(plaintext []byte) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 加密：nonce + ciphertext (包含 auth tag)
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密 base64 编码的密文
func (s *EncryptionService) Decrypt(ciphertextBase64 string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return nil, fmt.Errorf("base64 解码失败: %w", err)
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("密文太短")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	return plaintext, nil
}

// EncryptJSON 加密 JSON 对象
func (s *EncryptionService) EncryptJSON(v interface{}) (string, error) {
	jsonData, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("JSON 序列化失败: %w", err)
	}
	return s.Encrypt(jsonData)
}

// DecryptJSON 解密 JSON 对象
func (s *EncryptionService) DecryptJSON(ciphertextBase64 string, v interface{}) error {
	plaintext, err := s.Decrypt(ciphertextBase64)
	if err != nil {
		return err
	}
	return json.Unmarshal(plaintext, v)
}

// EncryptString 加密字符串
func (s *EncryptionService) EncryptString(plaintext string) (string, error) {
	return s.Encrypt([]byte(plaintext))
}

// DecryptString 解密字符串
func (s *EncryptionService) DecryptString(ciphertextBase64 string) (string, error) {
	plaintext, err := s.Decrypt(ciphertextBase64)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// IsInitialized 检查加密服务是否已初始化
func (s *EncryptionService) IsInitialized() bool {
	return s != nil && len(s.key) == 32
}
