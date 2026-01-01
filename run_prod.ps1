# run_prod.ps1 - 生产环境验证启动脚本
# 此脚本用于通过 Week 2 安全加固的启动检查

# === 1. 核心安全凭证 (满足长度要求 >= 32字节) ===
$env:JWT_SECRET = "12345678901234567890123456789012"      # 模拟32字节密钥
$env:SESSION_SECRET = "12345678901234567890123456789012"  # 模拟32字节密钥
$env:APP_AUTH_SECRET = "12345678901234567890123456789012" # Cookie加密密钥

# === 2. 生产环境配置 ===
$env:ENVIRONMENT = "production"
$env:DEBUG = "true"

# === 3. 安全策略配置 (修复启动报错的关键) ===
# 必须配置 CORS 白名单
$env:CORS_ALLOWED_ORIGINS = "http://localhost:3000,http://localhost:8096,https://yourdomain.com"

# 开启安全头
$env:CSP_ENABLED = "true"
$env:CSP_SCRIPT_SRC = "'self' 'unsafe-inline' 'unsafe-eval'" # Next.js 需要这些权限
$env:HSTS_ENABLED = "true"
$env:PERMISSIONS_POLICY_ENABLED = "true"

# === 4. 启动应用 ===
Write-Host "🚀 正在以生产模式启动 ytb2bili..." -ForegroundColor Cyan
Write-Host "🔑 已注入临时安全凭证和 CORS 配置" -ForegroundColor Gray
Write-Host "--------------------------------"
.\ytb2bili.exe
