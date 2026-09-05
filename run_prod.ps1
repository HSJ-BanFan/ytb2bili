# run_prod.ps1 - 生产环境启动脚本
# 此脚本用于启动生产环境配置

# 修复中文乱码：设置控制台代码页为 UTF-8
chcp 65001 | Out-Null
$OutputEncoding = [System.Text.Encoding]::UTF8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
[Console]::InputEncoding = [System.Text.Encoding]::UTF8


# === 1. 生产环境配置 ===
$env:ENVIRONMENT = "production"
$env:DEBUG = "true"

# === 2. 安全策略配置 ===
# 必须配置 CORS 白名单
$env:CORS_ALLOWED_ORIGINS = "http://localhost:3000,http://localhost:8096,https://yourdomain.com"

# 开启安全头
$env:CSP_ENABLED = "true"
$env:CSP_SCRIPT_SRC = "'self' 'unsafe-inline' 'unsafe-eval'" # Next.js 需要这些权限
$env:HSTS_ENABLED = "true"
$env:PERMISSIONS_POLICY_ENABLED = "true"

# === 3. 启动应用 ===
Write-Host "🚀 正在以生产模式启动 ytb2bili..." -ForegroundColor Cyan
Write-Host "🔒 已注入 CORS 和安全头配置" -ForegroundColor Gray
Write-Host "--------------------------------"
.\bili-up.exe
