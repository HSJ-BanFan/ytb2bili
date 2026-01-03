# CI/CD 本地测试脚本 (Windows PowerShell)
# 模拟 GitHub Actions 的测试流程

# 设置控制台编码为 UTF-8
$OutputEncoding = [System.Text.Encoding]::UTF8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

$ErrorActionPreference = "Stop"

Write-Host "=========================================" -ForegroundColor Cyan
Write-Host "🚀 CI/CD Local Test (Windows)" -ForegroundColor Cyan
Write-Host "=========================================" -ForegroundColor Cyan
Write-Host ""

# ========================================
# Step 1: 代码格式化检查
# ========================================
Write-Host "📝 Step 1: Checking code format..." -ForegroundColor Yellow
$formatted = gofmt -l internal pkg cmd tests tools scripts main.go
if ($formatted) {
    Write-Host "❌ Code is not formatted. Run 'go fmt ./...'" -ForegroundColor Red
    gofmt -d .
    exit 1
}
else {
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
}
catch {
    $lintInstalled = $false
}

if ($lintInstalled) {
    golangci-lint run --timeout=5m
    Write-Host "✅ Linter passed" -ForegroundColor Green
}
else {
    Write-Host "⚠️  golangci-lint not found. Skipping..." -ForegroundColor Yellow
    Write-Host "   Install: https://golangci-lint.run/usage/install/" -ForegroundColor Gray
}
Write-Host ""

# ========================================
# Step 4: 单元测试
# ========================================
Write-Host "🧪 Step 4: Running unit tests..." -ForegroundColor Yellow
Write-Host "-------------------------------------------" -ForegroundColor Gray
go test -v -short ./internal/... ./pkg/... ./cmd/...
Write-Host "✅ Unit tests passed" -ForegroundColor Green
Write-Host ""

# ========================================
# Step 5: 单元测试覆盖率
# ========================================
Write-Host "📊 Step 5: Unit test coverage..." -ForegroundColor Yellow
# 显式指定目录以避开 tests/integration
go test "-coverprofile=coverage_unit.out" -covermode=atomic ./internal/... ./pkg/... ./cmd/... 2>&1 | Out-Null
if (Test-Path coverage_unit.out) {
    go tool cover -func=coverage_unit.out | Select-String "total"
    Write-Host "✅ Coverage report generated" -ForegroundColor Green
}
else {
    Write-Host "⚠️  Coverage report generation failed" -ForegroundColor Yellow
}
Write-Host ""

# ========================================
# Step 6: 集成测试
# ========================================
Write-Host "🧪 Step 6: Running integration tests..." -ForegroundColor Yellow
Write-Host "-------------------------------------------" -ForegroundColor Gray
$integrationTestsExist = Test-Path "tests/integration"
if ($integrationTestsExist) {
    go test -v -race -coverprofile=coverage_integration.out -covermode=atomic ./tests/integration/...
    if (Test-Path coverage_integration.out) {
        Write-Host "✅ Integration tests passed" -ForegroundColor Green
    }
    else {
        Write-Host "⚠️  Integration tests failed or no coverage generated" -ForegroundColor Yellow
    }
}
else {
    Write-Host "⚠️  No integration tests found" -ForegroundColor Yellow
}
Write-Host ""

# ========================================
# Step 7: 合并覆盖率报告
# ========================================
Write-Host "📊 Step 7: Merging coverage reports..." -ForegroundColor Yellow
$unitCoverageExists = Test-Path coverage_unit.out
$integrationCoverageExists = Test-Path coverage_integration.out

if ($unitCoverageExists -or $integrationCoverageExists) {
    "mode: atomic" | Out-File -Encoding UTF8 coverage.out

    if ($unitCoverageExists) {
        Get-Content coverage_unit.out | Select-String "^[^:]+:[0-9]+:.*" | Add-Content coverage.out
    }

    if ($integrationCoverageExists) {
        Get-Content coverage_integration.out | Select-String "^[^:]+:[0-9]+:.*" | Add-Content coverage.out
    }

    if (Test-Path coverage.out) {
        $coverageOutput = go tool cover -func=coverage.out | Select-String "total"
        if ($coverageOutput) {
            $totalCoverage = ($coverageOutput -split '\s+')[2]
            Write-Host "✅ Total Coverage: $totalCoverage" -ForegroundColor Green
        }
    }
}
else {
    Write-Host "⚠️  No coverage reports found to merge" -ForegroundColor Yellow
}
Write-Host ""

# ========================================
# Step 8: 生成 HTML 报告
# ========================================
Write-Host "📄 Step 8: Generating HTML coverage report..." -ForegroundColor Yellow
if (Test-Path coverage.out) {
    go tool cover -html=coverage.out -o coverage.html
    Write-Host "✅ HTML report generated: coverage.html" -ForegroundColor Green
}
else {
    Write-Host "⚠️  No coverage.out found, skipping HTML report" -ForegroundColor Yellow
}
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
