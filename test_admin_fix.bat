@echo off
setlocal enabledelayedexpansion

REM 测试管理员权限修复的脚本 (Windows)
REM 使用前请确保服务器已启动

echo 🧪 测试管理员权限修复
echo ==========================
echo.

REM 服务器地址
set SERVER=http://localhost:8096

REM 步骤1: 测试登录
echo 1️⃣  测试登录...
curl -s -X POST "%SERVER%/api/v1/auth/login" -H "Content-Type: application/json" -d "{\"username\":\"mei\",\"password\":\"123456\"}" > login_response.json

REM 提取token (使用PowerShell)
for /f "tokens=*" %%i in ('powershell -Command "(Get-Content login_response.json | ConvertFrom-Json).data.token"') do set TOKEN=%%i

if "%TOKEN%"=="" (
    echo ❌ 登录失败
    type login_response.json
    pause
    exit /b 1
)

echo ✅ 登录成功
echo Token: %TOKEN:~0,20%...
echo.

REM 步骤2: 测试获取配置（应该成功）
echo 2️⃣  测试获取配置（GET请求）...
curl -s -X GET "%SERVER%/api/v1/config/gemini" -H "Authorization: Bearer %TOKEN%" > get_response.json

findstr /C:"\"code\":200" get_response.json >nul
if %errorlevel%==0 (
    echo ✅ GET请求成功
) else (
    echo ❌ GET请求失败
    type get_response.json
)
echo.

REM 步骤3: 测试修改配置（管理员应该成功）
echo 3️⃣  测试修改配置（PUT请求 - 需要管理员权限）...
curl -s -X PUT "%SERVER%/api/v1/config/gemini" -H "Authorization: Bearer %TOKEN%" -H "Content-Type: application/json" -d "{\"enabled\":true,\"api_key\":\"test-key-for-validation\",\"model\":\"gemini-2.0-flash-exp\",\"timeout\":120,\"max_tokens\":8000}" > put_response.json

findstr /C:"\"code\":200" put_response.json >nul
if %errorlevel%==0 (
    echo ✅ PUT请求成功 - 管理员权限正常
) else (
    findstr /C:"\"code\":403" put_response.json >nul
    if %errorlevel%==0 (
        echo ❌ PUT请求被拒绝 - 权限不足（可能不是管理员）
        type put_response.json
    ) else (
        findstr /C:"\"code\":401" put_response.json >nul
        if %errorlevel%==0 (
            echo ❌ PUT请求被拒绝 - 未认证
            type put_response.json
        ) else (
            echo ⚠️  PUT请求返回未知响应
            type put_response.json
        )
    )
)
echo.

REM 步骤4: 检查用户角色
echo 4️⃣  检查当前用户角色...
echo JWT Payload (解码后):
powershell -Command "$token = '%TOKEN%'; $parts = $token -split '\.'; $payload = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($parts[1] + '=' * (4 - $parts[1].Length %% 4))); Write-Output $payload | ConvertFrom-Json | ConvertTo-Json"
echo.

REM 步骤5: 测试无Token访问（应该失败）
echo 5️⃣  测试无Token访问（应该失败）...
curl -s -X GET "%SERVER%/api/v1/config/gemini" > no_token_response.json

findstr /C:"\"code\":401" no_token_response.json >nul
if %errorlevel%==0 (
    echo ✅ 无Token访问被正确拒绝
) else (
    echo ❌ 无Token访问未被拦截（安全问题！）
    type no_token_response.json
)
echo.

echo ==========================
echo ✅ 测试完成！
echo.
echo 📋 修复验证清单：
echo   [ ] 能成功登录
echo   [ ] GET请求返回配置
echo   [ ] PUT请求修改成功（需要是admin角色）
echo   [ ] 无Token访问被拒绝
echo.
echo 💡 如果PUT请求失败（403），请检查数据库中mei用户的role字段是否为'admin'
echo.

REM 清理临时文件
del login_response.json get_response.json put_response.json no_token_response.json 2>nul

pause
