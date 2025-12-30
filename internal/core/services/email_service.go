package services

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/internal/core/types"
	"gopkg.in/gomail.v2"
)

// EmailService 邮件服务
type EmailService struct {
	config *types.SMTPConfig
}

// NewEmailService 创建邮件服务
func NewEmailService(config *types.SMTPConfig) *EmailService {
	return &EmailService{config: config}
}

// IsEnabled 检查邮件服务是否启用（需要同时设置 enabled=true 和 password）
func (s *EmailService) IsEnabled() bool {
	return s.config != nil && s.config.Enabled && s.config.Password != ""
}

// GenerateCode 生成 6 位数字验证码
func (s *EmailService) GenerateCode() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// SendVerificationEmail 发送验证码邮件
func (s *EmailService) SendVerificationEmail(to, code, codeType string) error {
	if !s.IsEnabled() {
		return fmt.Errorf("邮件服务未启用")
	}

	// 解析 SMTP 主机
	hostParts := strings.Split(s.config.Host, ":")
	smtpHost := hostParts[0]
	smtpPort := 587
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &smtpPort)
	}

	m := gomail.NewMessage()
	m.SetHeader("From", m.FormatAddress(s.config.From, s.config.FromName))
	m.SetHeader("To", to)

	// 根据类型设置邮件主题和内容
	var subject, body string
	switch codeType {
	case "register":
		subject = "注册验证码"
		body = s.buildRegisterEmailBody(code)
	case "login":
		subject = "登录验证码"
		body = s.buildLoginEmailBody(code)
	case "reset_password":
		subject = "重置密码验证码"
		body = s.buildResetPasswordEmailBody(code)
	default:
		subject = "验证码"
		body = s.buildGenericEmailBody(code)
	}

	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(smtpHost, smtpPort, s.config.Username, s.config.Password)

	// 设置 LocalName 防止 HELO 命令因 DNS 反向解析而阻塞
	d.LocalName = "localhost"

	if s.config.UseTLS {
		d.TLSConfig = &tls.Config{ServerName: smtpHost, InsecureSkipVerify: false}
	}

	// gomail v2 不支持超时字段，使用 goroutine + channel 实现超时
	// 注意：超时后底层连接可能仍在阻塞，但会快速返回错误给调用者
	type result struct {
		err error
	}
	done := make(chan result, 1)
	go func() {
		done <- result{err: d.DialAndSend(m)}
	}()

	select {
	case r := <-done:
		return r.err
	case <-time.After(10 * time.Second):
		return fmt.Errorf("SMTP 连接超时（10秒），请检查网络或 SMTP 配置")
	}
}

// buildRegisterEmailBody 构建注册验证码邮件内容
func (s *EmailService) buildRegisterEmailBody(code string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #007bff; color: white; padding: 20px; text-align: center; border-radius: 5px 5px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border: 1px solid #ddd; }
        .code { font-size: 32px; font-weight: bold; color: #007bff; text-align: center; margin: 20px 0; letter-spacing: 5px; }
        .footer { background: #f9f9f9; padding: 15px; text-align: center; font-size: 12px; color: #666; border: 1px solid #ddd; border-top: none; border-radius: 0 0 5px 5px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>🎉 欢迎注册</h2>
        </div>
        <div class="content">
            <p>您好，</p>
            <p>感谢您注册我们的服务！</p>
            <p>您的验证码是：</p>
            <div class="code">%s</div>
            <p><strong>验证码有效期为 10 分钟，请尽快完成验证。</strong></p>
            <p>如果这不是您的操作，请忽略此邮件。</p>
        </div>
        <div class="footer">
            <p>此邮件由系统自动发送，请勿直接回复。</p>
            <p>%s</p>
        </div>
    </div>
</body>
</html>
`, code, time.Now().Format("2006-01-02 15:04:05"))
}

// buildLoginEmailBody 构建登录验证码邮件内容
func (s *EmailService) buildLoginEmailBody(code string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #28a745; color: white; padding: 20px; text-align: center; border-radius: 5px 5px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border: 1px solid #ddd; }
        .code { font-size: 32px; font-weight: bold; color: #28a745; text-align: center; margin: 20px 0; letter-spacing: 5px; }
        .footer { background: #f9f9f9; padding: 15px; text-align: center; font-size: 12px; color: #666; border: 1px solid #ddd; border-top: none; border-radius: 0 0 5px 5px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>🔐 登录验证</h2>
        </div>
        <div class="content">
            <p>您好，</p>
            <p>您正在登录我们的服务。</p>
            <p>您的验证码是：</p>
            <div class="code">%s</div>
            <p><strong>验证码有效期为 10 分钟。</strong></p>
            <p>如果这不是您的操作，请立即修改密码以保护账号安全。</p>
        </div>
        <div class="footer">
            <p>此邮件由系统自动发送，请勿直接回复。</p>
            <p>%s</p>
        </div>
    </div>
</body>
</html>
`, code, time.Now().Format("2006-01-02 15:04:05"))
}

// buildResetPasswordEmailBody 构建重置密码验证码邮件内容
func (s *EmailService) buildResetPasswordEmailBody(code string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #dc3545; color: white; padding: 20px; text-align: center; border-radius: 5px 5px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border: 1px solid #ddd; }
        .code { font-size: 32px; font-weight: bold; color: #dc3545; text-align: center; margin: 20px 0; letter-spacing: 5px; }
        .footer { background: #f9f9f9; padding: 15px; text-align: center; font-size: 12px; color: #666; border: 1px solid #ddd; border-top: none; border-radius: 0 0 5px 5px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>🔑 重置密码</h2>
        </div>
        <div class="content">
            <p>您好，</p>
            <p>您正在重置密码。</p>
            <p>您的验证码是：</p>
            <div class="code">%s</div>
            <p><strong>验证码有效期为 10 分钟。</strong></p>
            <p>如果这不是您的操作，请忽略此邮件以保护账号安全。</p>
        </div>
        <div class="footer">
            <p>此邮件由系统自动发送，请勿直接回复。</p>
            <p>%s</p>
        </div>
    </div>
</body>
</html>
`, code, time.Now().Format("2006-01-02 15:04:05"))
}

// buildGenericEmailBody 构建通用验证码邮件内容
func (s *EmailService) buildGenericEmailBody(code string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #6c757d; color: white; padding: 20px; text-align: center; border-radius: 5px 5px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border: 1px solid #ddd; }
        .code { font-size: 32px; font-weight: bold; color: #007bff; text-align: center; margin: 20px 0; letter-spacing: 5px; }
        .footer { background: #f9f9f9; padding: 15px; text-align: center; font-size: 12px; color: #666; border: 1px solid #ddd; border-top: none; border-radius: 0 0 5px 5px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>📧 验证码</h2>
        </div>
        <div class="content">
            <p>您好，</p>
            <p>您的验证码是：</p>
            <div class="code">%s</div>
            <p><strong>验证码有效期为 10 分钟。</strong></p>
            <p>如果这不是您的操作，请忽略此邮件。</p>
        </div>
        <div class="footer">
            <p>此邮件由系统自动发送，请勿直接回复。</p>
            <p>%s</p>
        </div>
    </div>
</body>
</html>
`, code, time.Now().Format("2006-01-02 15:04:05"))
}
