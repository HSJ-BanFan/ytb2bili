@echo off
REM 快速构建并启动脚本（修复手动上传按钮）

echo 🔧 修复手动上传按钮显示问题
echo.

REM 1. 构建前端
echo 1️⃣  构建前端...
cd web
call npm run build
if %errorlevel% neq 0 (
    echo    ❌ 前端构建失败
    pause
    exit /b 1
)
cd ..
echo    ✅ 前端构建完成
echo.

REM 2. 构建后端
echo 2️⃣  构建后端...
go build -o ytb2bili.exe .
if %errorlevel% neq 0 (
    echo    ❌ 后端构建失败
    pause
    exit /b 1
)
echo    ✅ 后端构建完成
echo.

echo 🎉 构建成功！
echo.
echo 📝 下一步操作：
echo    1. 停止当前运行的服务器（Ctrl+C）
echo    2. 运行: ytb2bili.exe
echo    3. 清除浏览器缓存（Ctrl+Shift+R）
echo    4. 使用 Free 账户测试手动上传功能
echo.
echo ✅ 修复内容：
echo    - 状态 205 现在会显示"立即上传"按钮
echo    - Free 用户可以手动上传视频
echo.
pause
