# Bili-Up 快速构建脚本
# 使用方法：.\build.ps1

param(
    [switch]$SkipFrontend = $false,
    [switch]$SkipBackend = $false
)

$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host "`n$message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Green
}

function Write-Error {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Red
}

# 开始计时
$startTime = Get-Date

try {
    # 1. 构建前端
    if (-not $SkipFrontend) {
        Write-Step "🔨 步骤 1/3: 构建前端..."
        Set-Location web

        Write-Host "   安装依赖..." -ForegroundColor Yellow
        npm ci --silent

        Write-Host "   构建 Next.js 应用..." -ForegroundColor Yellow
        $env:BACKEND_URL = "http://localhost:8096"
        npm run build

        # 验证构建结果
        if (-not (Test-Path "..\internal\web\bili-up-web\index.html")) {
            throw "前端构建失败：找不到 index.html"
        }

        Set-Location ..
        Write-Success "   ✅ 前端构建完成"
    } else {
        Write-Step "⏭️  跳过前端构建"
    }

    # 2. 构建后端
    if (-not $SkipBackend) {
        Write-Step "🔧 步骤 2/3: 构建后端..."

        Write-Host "   整理依赖..." -ForegroundColor Yellow
        go mod tidy

        Write-Host "   编译 Go 程序..." -ForegroundColor Yellow
        go build -v -o ytb2bili.exe .

        if (-not (Test-Path "ytb2bili.exe")) {
            throw "后端构建失败：找不到 ytb2bili.exe"
        }

        $size = [math]::Round((Get-Item ytb2bili.exe).Length / 1MB, 2)
        Write-Success "   ✅ 后端构建完成 (大小: ${size}MB)"
    } else {
        Write-Step "⏭️  跳过后端构建"
    }

    # 3. 完成
    $duration = (Get-Date) - $startTime
    $seconds = [math]::Round($duration.TotalSeconds, 1)

    Write-Step "🎉 步骤 3/3: 构建完成！"
    Write-Host "   总耗时: ${seconds}秒" -ForegroundColor Green
    Write-Host "`n   启动程序：" -ForegroundColor White
    Write-Host "   .\ytb2bili.exe`n" -ForegroundColor Yellow

} catch {
    Write-Error "`n❌ 构建失败: $_"
    exit 1
}
