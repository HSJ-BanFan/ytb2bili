package security

import (
	"fmt"
	"os"
	"strings"

	"github.com/difyz9/ytb2bili/internal/core/types"
)

// SecurityIssue 安全问题
type SecurityIssue struct {
	Level   string // "critical", "high", "medium", "low"
	Message string
	Fix     string
}

// ValidateSecurityConfig 验证安全配置
func ValidateSecurityConfig(config *types.AppConfig) []SecurityIssue {
	issues := []SecurityIssue{}

	// ========== 生产环境检查 ==========
	if config.Environment == "production" {
		// 检查 1: CORS 白名单
		if len(config.Security.CORSAllowedOrigins) == 0 {
			issues = append(issues, SecurityIssue{
				Level:   "critical",
				Message: "生产环境未配置 CORS 白名单",
				Fix:     "在 config.toml 中设置 [security] cors_allowed_origins",
			})
		}

		// 检查 2: 白名单中不应有 localhost
		for _, origin := range config.Security.CORSAllowedOrigins {
			if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
				issues = append(issues, SecurityIssue{
					Level:   "high",
					Message: fmt.Sprintf("CORS 白名单包含本地地址: %s", origin),
					Fix:     "移除开发环境的地址",
				})
			}
		}

		// 检查 3: CSP 是否启用
		if !config.Security.CSPEnabled {
			issues = append(issues, SecurityIssue{
				Level:   "high",
				Message: "生产环境未启用 Content-Security-Policy",
				Fix:     "设置 security.csp_enabled = true",
			})
		}

		// 检查 4: HSTS 是否启用
		if !config.Security.HSTSEnabled {
			issues = append(issues, SecurityIssue{
				Level:   "medium",
				Message: "生产环境未启用 HSTS (Strict-Transport-Security)",
				Fix:     "设置 security.hsts_enabled = true（需先配置 HTTPS）",
			})
		}

		// 检查 5: debug 模式
		if config.Debug {
			issues = append(issues, SecurityIssue{
				Level:   "medium",
				Message: "生产环境启用了 debug 模式",
				Fix:     "设置 debug = false",
			})
		}
	}

	// ========== 通用安全检查 ==========

	// 检查 7: CSP 策略中是否有 unsafe-inline
	if config.Security.CSPEnabled {
		if strings.Contains(config.Security.CSPScriptSrc, "unsafe-inline") ||
			strings.Contains(config.Security.CSPScriptSrc, "unsafe-eval") {
			issues = append(issues, SecurityIssue{
				Level:   "medium",
				Message: "CSP 策略包含 'unsafe-inline' 或 'unsafe-eval'，降低安全性",
				Fix:     "考虑使用 nonce 或 hash 替代",
			})
		}
	}

	// 检查 8: HSTS preload 需要确认
	if config.Security.HSTSEnabled && config.Security.HSTSPreload {
		issues = append(issues, SecurityIssue{
			Level:   "low",
			Message: "HSTS preload 已启用，这是不可逆的",
			Fix:     "确保已提交到 https://hstspreload.org/",
		})
	}

	return issues
}

// PrintSecurityReport 打印安全报告
func PrintSecurityReport(issues []SecurityIssue) {
	if len(issues) == 0 {
		fmt.Println("✅ 安全配置检查通过")
		return
	}

	fmt.Printf("\n⚠️  发现 %d 个安全问题：\n\n", len(issues))

	criticalCount := 0
	highCount := 0

	for i, issue := range issues {
		prefix := "🔴"
		if issue.Level == "critical" {
			prefix = "🚨"
			criticalCount++
		} else if issue.Level == "high" {
			prefix = "⚠️ "
			highCount++
		} else if issue.Level == "medium" {
			prefix = "📌"
		} else {
			prefix = "💡"
		}

		fmt.Printf("%d. %s [%s] %s\n", i+1, prefix, strings.ToUpper(issue.Level), issue.Message)
		fmt.Printf("   修复建议: %s\n\n", issue.Fix)
	}

	// 严重问题阻止启动
	if criticalCount > 0 {
		fmt.Println("🚨 存在严重安全问题，无法启动！")
		fmt.Println("   请修复上述 critical 级别的问题后重试。")
		os.Exit(1)
	}

	if highCount > 0 {
		fmt.Println("⚠️  建议修复上述 high 级别的问题后再部署到生产环境。")
	}
}
