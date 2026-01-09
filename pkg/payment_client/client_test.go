package payment_client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateSignature(t *testing.T) {
	tests := []struct {
		name      string
		appID     string
		appSecret string
		timestamp string
		nonce     string
		wantLen   int // 签名长度应该是 64 (SHA256 输出 32 字节 = 64 十六进制字符)
	}{
		{
			name:      "标准签名",
			appID:     "admin-frontend",
			appSecret: "admin-frontend-secret-key-32chars",
			timestamp: "1735372800",
			nonce:     "abc123xyz789",
			wantLen:   64,
		},
		{
			name:      "测试应用签名",
			appID:     "test-app-001",
			appSecret: "test-secret-key-12345678901234567890",
			timestamp: "1735372800",
			nonce:     "randomnonce123",
			wantLen:   64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSignature(tt.appID, tt.appSecret, tt.timestamp, tt.nonce)

			// 验证签名长度
			if len(got) != tt.wantLen {
				t.Errorf("GenerateSignature() 签名长度 = %d, 期望 %d", len(got), tt.wantLen)
			}

			// 验证签名一致性（相同输入应产生相同输出）
			got2 := GenerateSignature(tt.appID, tt.appSecret, tt.timestamp, tt.nonce)
			if got != got2 {
				t.Errorf("GenerateSignature() 签名不一致: %s != %s", got, got2)
			}

			// 验证签名是小写十六进制
			for _, c := range got {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("GenerateSignature() 签名包含非小写十六进制字符: %c", c)
				}
			}
		})
	}
}

func TestGenerateSignature_DifferentInputs(t *testing.T) {
	base := GenerateSignature("app1", "secret1", "12345", "nonce1")

	// 不同 appID
	diff1 := GenerateSignature("app2", "secret1", "12345", "nonce1")
	if base == diff1 {
		t.Error("不同 appID 应产生不同签名")
	}

	// 不同 appSecret
	diff2 := GenerateSignature("app1", "secret2", "12345", "nonce1")
	if base == diff2 {
		t.Error("不同 appSecret 应产生不同签名")
	}

	// 不同 timestamp
	diff3 := GenerateSignature("app1", "secret1", "12346", "nonce1")
	if base == diff3 {
		t.Error("不同 timestamp 应产生不同签名")
	}

	// 不同 nonce
	diff4 := GenerateSignature("app1", "secret1", "12345", "nonce2")
	if base == diff4 {
		t.Error("不同 nonce 应产生不同签名")
	}
}

func TestClient_CreatePayment_Success(t *testing.T) {
	// 创建 Mock 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法和路径
		if r.Method != "POST" {
			t.Errorf("期望 POST, 得到 %s", r.Method)
		}
		if r.URL.Path != "/api/v1/payment/pay" {
			t.Errorf("期望路径 /api/v1/payment/pay, 得到 %s", r.URL.Path)
		}

		// 验证必要的 Header
		if r.Header.Get("X-App-Id") == "" {
			t.Error("缺少 X-App-Id Header")
		}
		if r.Header.Get("X-Timestamp") == "" {
			t.Error("缺少 X-Timestamp Header")
		}
		if r.Header.Get("X-Nonce") == "" {
			t.Error("缺少 X-Nonce Header")
		}
		if r.Header.Get("X-Sign") == "" {
			t.Error("缺少 X-Sign Header")
		}

		// 返回模拟响应
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"code": 200,
			"message": "success",
			"data": {
				"payUrl": "https://example.com/pay?order=123",
				"orderNo": "202512280001234567",
				"amount": 29.9,
				"payWay": "alipay"
			}
		}`))
	}))
	defer server.Close()

	// 创建客户端
	client := New(server.URL, "admin-frontend", "admin-frontend-secret-key-32chars")

	// 发起支付请求
	resp, err := client.CreatePayment(CreatePaymentRequest{
		Subject: "VIP月卡",
		Amount:  29.9,
		PayWay:  "alipay",
		UserID:  "user_123",
	})

	if err != nil {
		t.Fatalf("CreatePayment() 错误: %v", err)
	}

	if resp.Code != 200 {
		t.Errorf("期望 code=200, 得到 %d", resp.Code)
	}

	if resp.Data.OrderNo != "202512280001234567" {
		t.Errorf("期望 orderNo=202512280001234567, 得到 %s", resp.Data.OrderNo)
	}

	if resp.Data.Amount != 29.9 {
		t.Errorf("期望 amount=29.9, 得到 %f", resp.Data.Amount)
	}
}

func TestClient_CreatePayment_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"code": 401, "message": "Invalid signature"}`))
	}))
	defer server.Close()

	client := New(server.URL, "wrong-app", "wrong-secret")

	_, err := client.CreatePayment(CreatePaymentRequest{
		Subject: "VIP月卡",
		Amount:  29.9,
		PayWay:  "alipay",
	})

	if err == nil {
		t.Error("期望返回错误，但没有")
	}

	payErr, ok := err.(*PaymentError)
	if !ok {
		t.Errorf("期望 *PaymentError, 得到 %T", err)
	}

	if payErr.Code != ErrCodeSignatureInvalid {
		t.Errorf("期望错误码 %s, 得到 %s", ErrCodeSignatureInvalid, payErr.Code)
	}
}

func TestClient_GenerateNonce(t *testing.T) {
	client := New("http://localhost", "app", "secret")

	nonce1 := client.GenerateNonce()
	nonce2 := client.GenerateNonce()

	// 验证长度
	if len(nonce1) != 32 {
		t.Errorf("期望 nonce 长度 32, 得到 %d", len(nonce1))
	}

	// 验证唯一性
	if nonce1 == nonce2 {
		t.Error("两次生成的 nonce 不应该相同")
	}
}
