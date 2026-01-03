package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncryptionService_BasicEncryption 测试基本加密解密功能
func TestEncryptionService_BasicEncryption(t *testing.T) {
	// 创建一个 32 字节的测试密钥
	testKey := "test1234567890123456789012345678"
	svc, err := NewEncryptionService(testKey)
	require.NoError(t, err)
	require.NotNil(t, svc)

	// 测试数据
	testCases := []struct {
		name      string
		plaintext []byte
	}{
		{
			name:      "简单文本",
			plaintext: []byte("Hello, World!"),
		},
		{
			name:      "空字符串",
			plaintext: []byte(""),
		},
		{
			name:      "nil 切片",
			plaintext: nil,
		},
		{
			name:      "长文本",
			plaintext: []byte("这是很长的中文测试内容，包含各种字符：!@#$%^&*()_+-=[]{}|;':\",./<>?"),
		},
		{
			name:      "二进制数据",
			plaintext: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 加密
			ciphertext, err := svc.Encrypt(tc.plaintext)
			require.NoError(t, err)
			assert.NotEmpty(t, ciphertext)
			// 密文应该与明文不同
			assert.NotEqual(t, string(tc.plaintext), ciphertext)

			// 解密
			decrypted, err := svc.Decrypt(ciphertext)
			require.NoError(t, err)
			// 解密后应该与原文相同
			// 注意：[]byte{} 和 []byte(nil) 虽然不同，但长度都是 0
			assert.Equal(t, len(tc.plaintext), len(decrypted), "长度应该相同")
			if len(tc.plaintext) > 0 {
				assert.Equal(t, tc.plaintext, decrypted)
			}
		})
	}
}

// TestEncryptionService_EncryptString 测试字符串加密
func TestEncryptionService_EncryptString(t *testing.T) {
	testKey := "test1234567890123456789012345678"
	svc, err := NewEncryptionService(testKey)
	require.NoError(t, err)

	plaintext := "sensitive string data"

	// 加密字符串
	ciphertext, err := svc.EncryptString(plaintext)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	assert.NotEqual(t, plaintext, ciphertext)

	// 解密字符串
	decrypted, err := svc.DecryptString(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

// TestEncryptionService_EncryptJSON 测试 JSON 加密
func TestEncryptionService_EncryptJSON(t *testing.T) {
	testKey := "test1234567890123456789012345678"
	svc, err := NewEncryptionService(testKey)
	require.NoError(t, err)

	type TestStruct struct {
		Name   string `json:"name"`
		Age    int    `json:"age"`
		Secret string `json:"secret"`
		Active bool   `json:"active"`
	}

	original := TestStruct{
		Name:   "Alice",
		Age:    30,
		Secret: "my-secret-password",
		Active: true,
	}

	// 加密 JSON
	ciphertext, err := svc.EncryptJSON(original)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)

	// 解密 JSON
	var decrypted TestStruct
	err = svc.DecryptJSON(ciphertext, &decrypted)
	require.NoError(t, err)
	assert.Equal(t, original, decrypted)
}

// TestEncryptionService_InvalidKeyLength 测试无效密钥长度
func TestEncryptionService_InvalidKeyLength(t *testing.T) {
	testCases := []struct {
		name       string
		key        string
		shouldFail bool
	}{
		{
			name:       "正确长度 32 字节",
			key:        "12345678901234567890123456789012",
			shouldFail: false,
		},
		{
			name:       "太短 16 字节",
			key:        "1234567890123456",
			shouldFail: true,
		},
		{
			name:       "太短 1 字节",
			key:        "1",
			shouldFail: true,
		},
		{
			name:       "空字符串",
			key:        "",
			shouldFail: true,
		},
		{
			name:       "太长 64 字节",
			key:        "1234567890123456789012345678901234567890123456789012345678901234",
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, err := NewEncryptionService(tc.key)

			if tc.shouldFail {
				assert.Error(t, err)
				assert.Nil(t, svc)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, svc)
			}
		})
	}
}

// TestEncryptionService_InvalidCiphertext 测试无效密文解密
func TestEncryptionService_InvalidCiphertext(t *testing.T) {
	testKey := "test1234567890123456789012345678"
	svc, err := NewEncryptionService(testKey)
	require.NoError(t, err)

	testCases := []struct {
		name       string
		ciphertext string
	}{
		{
			name:       "空字符串",
			ciphertext: "",
		},
		{
			name:       "无效 base64",
			ciphertext: "not-valid-base64!!!",
		},
		{
			name:       "有效的 base64 但不是加密数据",
			ciphertext: "SGVsbG8gV29ybGQ=", // "Hello World" in base64
		},
		{
			name:       "被篡改的密文",
			ciphertext: "dGVzdGRhdGE=",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Decrypt(tc.ciphertext)
			assert.Error(t, err)
		})
	}
}

// TestEncryptionService_Deterministic 测试加密的非确定性
// 相同的明文每次加密都应该产生不同的密文（因为 nonce 是随机的）
func TestEncryptionService_Nondeterministic(t *testing.T) {
	testKey := "test1234567890123456789012345678"
	svc, err := NewEncryptionService(testKey)
	require.NoError(t, err)

	plaintext := []byte("same data")

	// 加密两次
	ciphertext1, err := svc.Encrypt(plaintext)
	require.NoError(t, err)

	ciphertext2, err := svc.Encrypt(plaintext)
	require.NoError(t, err)

	// 密文应该不同
	assert.NotEqual(t, ciphertext1, ciphertext2, "相同明文两次加密应产生不同密文")

	// 但解密后应该得到相同的明文
	decrypted1, err := svc.Decrypt(ciphertext1)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted1)

	decrypted2, err := svc.Decrypt(ciphertext2)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted2)
}

// TestEncryptionService_LargeData 测试大量数据加密
func TestEncryptionService_LargeData(t *testing.T) {
	testKey := "test1234567890123456789012345678"
	svc, err := NewEncryptionService(testKey)
	require.NoError(t, err)

	// 创建 1MB 的数据
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	// 加密
	ciphertext, err := svc.Encrypt(largeData)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)

	// 解密
	decrypted, err := svc.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, largeData, decrypted)
}

// TestEncryptionService_ConcurrentAccess 测试并发访问安全性
func TestEncryptionService_ConcurrentAccess(t *testing.T) {
	testKey := "test1234567890123456789012345678"
	svc, err := NewEncryptionService(testKey)
	require.NoError(t, err)

	// 并发加密
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			plaintext := []byte("concurrent test data")
			ciphertext, err := svc.Encrypt(plaintext)
			assert.NoError(t, err)

			decrypted, err := svc.Decrypt(ciphertext)
			assert.NoError(t, err)
			assert.Equal(t, plaintext, decrypted)

			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestEncryptionService_IsInitialized 测试初始化状态检查
func TestEncryptionService_IsInitialized(t *testing.T) {
	t.Run("已初始化", func(t *testing.T) {
		testKey := "test1234567890123456789012345678"
		svc, err := NewEncryptionService(testKey)
		require.NoError(t, err)
		assert.True(t, svc.IsInitialized())
	})

	t.Run("未初始化（nil）", func(t *testing.T) {
		var svc *EncryptionService
		assert.False(t, svc.IsInitialized())
	})
}

// TestGetEncryptionService_Singleton 测试单例模式
func TestGetEncryptionService_Singleton(t *testing.T) {
	// 注意：这个测试会修改全局状态，可能会影响其他测试
	// 在实际应用中，应该使用环境变量设置密钥

	t.Skip("跳过单例测试，避免修改全局状态")

	// svc1, err := GetEncryptionService()
	// require.NoError(t, err)
	//
	// svc2, err := GetEncryptionService()
	// require.NoError(t, err)
	//
	// 应该返回同一个实例
	// assert.Equal(t, svc1, svc2)
}

// BenchmarkEncryptionService_Encrypt 性能测试：加密
func BenchmarkEncryptionService_Encrypt(b *testing.B) {
	testKey := "test1234567890123456789012345678"
	svc, _ := NewEncryptionService(testKey)
	plaintext := []byte("This is a test data for benchmarking encryption performance.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Encrypt(plaintext)
	}
}

// BenchmarkEncryptionService_Decrypt 性能测试：解密
func BenchmarkEncryptionService_Decrypt(b *testing.B) {
	testKey := "test1234567890123456789012345678"
	svc, _ := NewEncryptionService(testKey)
	plaintext := []byte("This is a test data for benchmarking decryption performance.")
	ciphertext, _ := svc.Encrypt(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Decrypt(ciphertext)
	}
}

// BenchmarkEncryptionService_EncryptLargeData 性能测试：加密大数据
func BenchmarkEncryptionService_EncryptLargeData(b *testing.B) {
	testKey := "test1234567890123456789012345678"
	svc, _ := NewEncryptionService(testKey)
	largeData := make([]byte, 1024*100) // 100KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Encrypt(largeData)
	}
}
