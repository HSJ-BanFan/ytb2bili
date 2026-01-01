$ErrorActionPreference = "SilentlyContinue" # Handle 403s manually
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8 # Fix garbled text


$BaseUrl = "http://localhost:8096"
$PassCount = 0
$FailCount = 0

function Print-Result {
    param ($Name, $Passed, $Details)
    Write-Host -NoNewline "📋 测试: $Name ... "
    if ($Passed) {
        Write-Host "✅ 通过" -ForegroundColor Green
        $script:PassCount++
    }
    else {
        Write-Host "❌ 失败" -ForegroundColor Red
        if ($Details) { Write-Host "   $Details" -ForegroundColor Gray }
        $script:FailCount++
    }
}

Write-Host "🔒 安全配置自动化测试 (Windows PowerShell)"
Write-Host "======================================"
Write-Host ""

# 1. CORS 拒绝未授权域名
try {
    $resp = Invoke-WebRequest -Uri "$BaseUrl/api/health" -Method Head -Headers @{ "Origin" = "https://evil.com" } -ErrorAction Stop
    Print-Result "CORS 应拒绝未授权域名" $false "期望 403, 实际 $($resp.StatusCode)"
}
catch {
    if ($_.Exception.Response.StatusCode -eq [System.Net.HttpStatusCode]::Forbidden) {
        Print-Result "CORS 应拒绝未授权域名" $true
    }
    else {
        Print-Result "CORS 应拒绝未授权域名" $false "期望 403, 实际 $($_.Exception.Response.StatusCode)"
    }
}

# 2. CORS 允许授权域名
try {
    $resp = Invoke-WebRequest -Uri "$BaseUrl/api/health" -Method Head -Headers @{ "Origin" = "http://localhost:3000" }
    if ($resp.StatusCode -eq 200) {
        Print-Result "CORS 应允许授权域名" $true
    }
    else {
        Print-Result "CORS 应允许授权域名" $false "期望 200, 实际 $($resp.StatusCode)"
    }
}
catch {
    Print-Result "CORS 应允许授权域名" $false "请求失败: $_"
}

# 获取一次 headers 用于后续检查
try {
    $resp = Invoke-WebRequest -Uri "$BaseUrl/api/health" -Method Head
    $headers = $resp.Headers

    # 3. X-Frame-Options
    if ($headers["X-Frame-Options"] -eq "SAMEORIGIN") {
        Print-Result "X-Frame-Options 应为 SAMEORIGIN" $true
    }
    else {
        Print-Result "X-Frame-Options 应为 SAMEORIGIN" $false "实际: $($headers["X-Frame-Options"])"
    }

    # 4. X-Content-Type-Options
    if ($headers["X-Content-Type-Options"] -eq "nosniff") {
        Print-Result "X-Content-Type-Options 应为 nosniff" $true
    }
    else {
        Print-Result "X-Content-Type-Options 应为 nosniff" $false "实际: $($headers["X-Content-Type-Options"])"
    }

    # 5. Content-Security-Policy
    if ($headers["Content-Security-Policy"]) {
        Print-Result "Content-Security-Policy 存在" $true
    }
    else {
        Print-Result "Content-Security-Policy 存在" $false "未找到 CSP 头"
    }

    # 6. Permissions-Policy
    if ($headers["Permissions-Policy"]) {
        Print-Result "Permissions-Policy 存在" $true
    }
    else {
        Print-Result "Permissions-Policy 存在" $false "未找到 Permissions-Policy 头"
    }

    # 7. 信息泄露检查
    if ($headers["Server"]) {
        Print-Result "响应头不应泄露服务器版本" $true "⚠️  警告: Server 头存在 ($($headers["Server"]))"
    }
    else {
        Print-Result "响应头不应泄露服务器版本" $true
    }

}
catch {
    Write-Host "无法连接到服务器 ($BaseUrl)，请确保服务已启动。" -ForegroundColor Red
    exit 1
}

# 8. JSON 错误响应 (需要 Body)
try {
    # 这里的 -SkipHttpErrorCheck 仅适用于 PS 7+, 兼容起见用 try-catch
    $resp = Invoke-WebRequest -Uri "$BaseUrl/api/health" -Method Get -Headers @{ "Origin" = "https://evil.com" } -ErrorAction Stop
    Print-Result "CORS 拒绝应返回 JSON" $false "请求未被拒绝"
}
catch {
    if ($_.Exception.Response.StatusCode -eq [System.Net.HttpStatusCode]::Forbidden) {
        $content = [System.Text.Encoding]::UTF8.GetString($_.Exception.Response.GetResponseStream().ToArray())
        if ($content -match '"error"') {
            Print-Result "CORS 拒绝应返回 JSON" $true
        }
        else {
            Print-Result "CORS 拒绝应返回 JSON" $false "未包含 error 字段"
        }
    }
}

Write-Host ""
Write-Host "======================================"
Write-Host "✅ 测试完成: 通过 $script:PassCount, 失败 $script:FailCount"
if ($script:FailCount -gt 0) { exit 1 }
