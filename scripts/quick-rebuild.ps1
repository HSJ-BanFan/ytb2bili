# 快速重新构建（仅后端）
# 使用场景：前端没改动，只改了 Go 代码
# 使用方法：.\quick-rebuild.ps1

Write-Host "⚡ 快速构建（仅后端）..." -ForegroundColor Cyan
Write-Host ""

go build -v -o ytb2bili.exe .

if ($LASTEXITCODE -eq 0) {
    $size = [math]::Round((Get-Item ytb2bili.exe).Length / 1MB, 2)
    Write-Host "`n✅ 构建完成！" -ForegroundColor Green
    Write-Host "📦 大小: ${size}MB" -ForegroundColor White
    Write-Host "`n启动程序: .\ytb2bili.exe" -ForegroundColor Yellow
} else {
    Write-Host "`n❌ 构建失败" -ForegroundColor Red
    exit 1
}
