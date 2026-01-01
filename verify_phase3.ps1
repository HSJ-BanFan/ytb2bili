# verify_phase3.ps1
# 用于验证 Phase 3 安全修复的 API 脚本

$BaseUrl = "http://localhost:8096"
$LoginUrl = "$BaseUrl/api/v1/user/login"
$AccountUrl = "$BaseUrl/api/v1/bili-accounts"

# 1. 登录获取 Token (根据需要调整用户名密码)
$User = "" # 请确保这是数据库中存在的有效邮箱
$Pass = "" # 假设的默认密码，需根据实际情况修改

Write-Host "🔍 尝试登录..."
try {
    $Body = @{
        email    = $User
        password = $Pass
    } | ConvertTo-Json

    $LoginResponse = Invoke-RestMethod -Uri $LoginUrl -Method Post -Body $Body -ContentType "application/json"
    $TokenObj = $LoginResponse.data.token
    $Token = $TokenObj.access_token
    Write-Host "✅ 登录成功，Token: $Token"
}
catch {
    Write-Host "❌ 登录失败: $_"
    exit
}

$Headers = @{
    "Authorization" = "Bearer $Token"
}

# 2. 验证加密版本字段 (P1-9)
Write-Host "`n🔍 验证 B站账号列表 API (Checking for encryption_version)..."
try {
    $AccResponse = Invoke-RestMethod -Uri $AccountUrl -Method Get -Headers $Headers
    $Accounts = $AccResponse.data

    if ($Accounts.Count -gt 0) {
        $Account = $Accounts[0]
        if ($Account.PSObject.Properties.Match("encryption_version").Count -gt 0) {
            Write-Host "✅ 发现 'encryption_version' 字段: $($Account.encryption_version)"
        }
        else {
            Write-Host "❌ 未发现 'encryption_version' 字段!"
        }
    }
    else {
        Write-Host "⚠️ 没有绑定的账号，无法验证字段 (但 API 调用成功)"
    }
}
catch {
    Write-Host "❌ API 调用失败: $_"
}

# 3. 验证 CORS 限制 (P2-1)
# 注意：这需要在 config.toml 中配置 CORSAllowedOrigins 才能验证为了 Forbidden
Write-Host "`n🔍 验证 CORS (Attempting strictly disallowed origin)..."
try {
    $CorsHeaders = $Headers.Clone()
    $CorsHeaders["Origin"] = "http://evil-site.com"
    
    # 使用 Invoke-WebRequest 以捕获 403 错误
    $Response = Invoke-WebRequest -Uri $AccountUrl -Method Get -Headers $CorsHeaders -ErrorAction Stop
    Write-Host "⚠️ 请求成功 (这在开发模式且未配置白名单时是预期的，如果配置了白名单则应失败)"
}
catch {
    if ($_.Exception.Response.StatusCode -eq [System.Net.HttpStatusCode]::Forbidden) {
        Write-Host "✅ CORS 拦截生效: 403 Forbidden"
    }
    else {
        Write-Host "❌ 发生非预期错误: $_"
    }
}
