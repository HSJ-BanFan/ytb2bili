# CI/CD 本地测试脚本 (Windows PowerShell)
# 模拟 GitHub Actions 的测试流程

$ErrorActionPreference = "Stop"

Write-Host "=========================================" -ForegroundColor Cyan
Write-Host "🚀 CI/CD Local Test (Windows)" -ForegroundColor Cyan
Write-Host "=========================================" -ForegroundColor Cyan
Write-Host ""

# ========================================
# Step 1: 代码格式化检查
# ========================================
Write-Host "📝 Step 1: Checking code format..." -ForegroundColor Yellow
$formatted = gofmt -l .
if ($formatted) {
    Write-Host "❌ Code is not formatted. Run 'go fmt ./...'" -ForegroundColor Red
    gofmt -d .
    exit 1
} else {
    Write-Host "✅ Code is properly formatted" -ForegroundColor Green
}
Write-Host ""

# ========================================
# Step 2: 依赖验证
# ========================================
Write-Host "📦 Step 2: Verifying dependencies..." -ForegroundColor Yellow
go mod verify
Write-Host "✅ Dependencies verified" -ForegroundColor Green
Write-Host ""

# ========================================
# Step 3: Lint 检查
# ========================================
Write-Host "🔍 Step 3: Running linter..." -ForegroundColor Yellow
$lintInstalled = $true
try {
    golangci-lint --version | Out-Null
} catch {
    $lintInstalled = $false
}

if ($lintInstalled) {
    golangci-lint run --timeout=5m
    Write-Host "✅ Linter passed" -ForegroundColor Green
} else {
    Write-Host "⚠️  golangci-lint not found. Skipping..." -ForegroundColor Yellow
    Write-Host "   Install: https://golangci-lint.run/usage/install/" -ForegroundColor Gray
}
Write-Host ""

# ========================================
# Step 4: 单元测试
# ========================================
Write-Host "🧪 Step 4: Running unit tests..." -ForegroundColor Yellow
Write-Host "-------------------------------------------" -ForegroundColor Gray
go test -v -short ./... -exclude-dir tests/integration
Write-Host "✅ Unit tests passed" -ForegroundColor Green
Write-Host ""

# ========================================
# Step 5: 单元测试覆盖率
# ========================================
Write-Host "📊 Step 5: Unit test coverage..." -ForegroundColor Yellow
go test -race -coverprofile=coverage_unit.out -covermode=atomic ./...
go tool cover -func=coverage_unit.out | Select-String "total"
Write-Host "✅ Coverage report generated" -ForegroundColor Green
Write-Host ""

# ========================================
# Step 6: 集成测试
# ========================================
Write-Host "🧪 Step 6: Running integration tests..." -ForegroundColor Yellow
Write-Host "-------------------------------------------" -ForegroundColor Gray
go test -v -race -coverprofile=coverage_integration.out -covermode=atomic ./tests/integration/...
Write-Host "✅ Integration tests passed" -ForegroundColor Green
Write-Host ""

# ========================================
# Step 7: 合并覆盖率报告
# ========================================
Write-Host "📊 Step 7: Merging coverage reports..." -ForegroundColor Yellow
"mode: atomic" | Out-File -Encoding UTF8 coverage.out
Get-Content coverage_unit.out, coverage_integration.out | Select-String "^[^:]+:[0-9]+:.*" | Sort-Object | Get-Unique | Add-Content coverage.out

$coverageOutput = go tool cover -func=coverage.out | Select-String "total"
$totalCoverage = ($coverageOutput -split '\s+')[2]
Write-Host "✅ Total Coverage: $totalCoverage" -ForegroundColor Green
Write-Host ""

# ========================================
# Step 8: 生成 HTML 报告
# ========================================
Write-Host "📄 Step 8: Generating HTML coverage report..." -ForegroundColor Yellow
go tool cover -html=coverage.out -o coverage.html
Write-Host "✅ HTML report generated: coverage.html" -ForegroundColor Green
Write-Host ""

# ========================================
# 完成
# ========================================
Write-Host "=========================================" -ForegroundColor Cyan
Write-Host "🎉 All tests passed!" -ForegroundColor Green
Write-Host "=========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "📊 Coverage Summary:" -ForegroundColor Cyan
$unitCoverage = (go tool cover -func=coverage_unit.out | Select-String "total" | ForEach-Object { ($_ -split '\s+')[2] })
$intCoverage = (go tool cover -func=coverage_integration.out | Select-String "total" | ForEach-Object { ($_ -split '\s+')[2] })

Write-Host "   - Unit Tests: $unitCoverage" -ForegroundColor White
Write-Host "   - Integration Tests: $intCoverage" -ForegroundColor White
Write-Host "   - Total: $totalCoverage" -ForegroundColor White
Write-Host ""
Write-Host "📄 View HTML report:" -ForegroundColor Cyan
Write-Host "   Start-Process coverage.html" -ForegroundColor Gray
Write-Host ""
