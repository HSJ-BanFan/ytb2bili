# 前端修改后重新构建
# 使用场景：修改了前端代码，需要重新嵌入到 Go 二进制
# 使用方法：.\rebuild-frontend.ps1

Write-Host "🔨 重新构建前端..." -ForegroundColor Cyan
Write-Host ""

Set-Location web

Write-Host "构建 Next.js..." -ForegroundColor Yellow
$env:BACKEND_URL = "http://localhost:8096"
npm run build

if ($LASTEXITCODE -eq 0) {
    Set-Location ..

    # 检查构建结果
    if (-not (Test-Path "internal\web\bili-up-web\index.html")) {
        Write-Host "`n❌ 前端构建失败" -ForegroundColor Red
        exit 1
    }

    Write-Host "`n✅ 前端构建完成！" -ForegroundColor Green
    Write-Host "`n现在重新构建 Go 程序：" -ForegroundColor Yellow
    Write-Host "   .\quick-rebuild.ps1`n" -ForegroundColor White
    Write-Host "或者完整构建：" -ForegroundColor Yellow
    Write-Host "   .\build.ps1 -SkipFrontend`n" -ForegroundColor White
} else {
    Set-Location ..
    Write-Host "`n❌ Next.js 构建失败" -ForegroundColor Red
    exit 1
}
