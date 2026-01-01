package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncryptionService_RoundTrip 测试加密解密往返
func TestEncryptionService_RoundTrip(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 测试数据
	testData := []string{
		"simple text",
		"中文测试数据",
		"special chars: !@#$%^&*()",
		"",
		"a", // 单字符
	}

	for _, data := range testData {
		t.Run(data, func(t *testing.T) {
			// 加密
			encrypted, err := app.EncryptionSvc.EncryptString(data)
			require.NoError(t, err, "加密应该成功")

			if data != "" {
				assert.NotEmpty(t, encrypted, "加密结果不应为空")
				assert.NotEqual(t, data, encrypted, "加密结果应与原文不同")
			}

			// 解密
			decrypted, err := app.EncryptionSvc.DecryptString(encrypted)
			require.NoError(t, err, "解密应该成功")
			assert.Equal(t, data, decrypted, "解密结果应与原文一致")
		})
	}
}

// TestEncryptionService_InvalidCiphertext 测试无效密文处理
func TestEncryptionService_InvalidCiphertext(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	invalidCiphertexts := []string{
		"invalid_base64!",
		"too_short",
	}

	for _, ciphertext := range invalidCiphertexts {
		t.Run(ciphertext, func(t *testing.T) {
			_, err := app.EncryptionSvc.DecryptString(ciphertext)
			assert.Error(t, err, "无效密文应该返回错误")
		})
	}
}

// TestEncryptionService_JSONEncryption 测试JSON加密
func TestEncryptionService_JSONEncryption(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 测试数据结构
	type TestStruct struct {
		Name   string `json:"name"`
		Age    int    `json:"age"`
		Secret string `json:"secret"`
	}

	original := TestStruct{
		Name:   "张三",
		Age:    30,
		Secret: "my_secret_password",
	}

	// 加密
	encrypted, err := app.EncryptionSvc.EncryptJSON(original)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	// 解密
	var decrypted TestStruct
	err = app.EncryptionSvc.DecryptJSON(encrypted, &decrypted)
	require.NoError(t, err)
	assert.Equal(t, original.Name, decrypted.Name)
	assert.Equal(t, original.Age, decrypted.Age)
	assert.Equal(t, original.Secret, decrypted.Secret)
}

// TestEncryptionService_LargeData 测试大数据加密
func TestEncryptionService_LargeData(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 创建 1KB 的测试数据
	largeData := make([]byte, 1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	// 加密
	encrypted, err := app.EncryptionSvc.Encrypt(largeData)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	// 解密
	decrypted, err := app.EncryptionSvc.Decrypt(encrypted)
	require.NoError(t, err)
	assert.Equal(t, largeData, decrypted)
}
