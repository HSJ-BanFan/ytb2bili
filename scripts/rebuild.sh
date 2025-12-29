#!/bin/bash
# 快速重新构建并启动脚本

echo "🔧 开始重新构建..."
echo ""

# 1. 清理旧的构建文件
echo "1️⃣  清理旧文件..."
rm -rf web/.next
rm -f ytb2bili.exe
echo "   ✅ 清理完成"
echo ""

# 2. 构建前端
echo "2️⃣  构建前端..."
cd web
npm run build
if [ $? -ne 0 ]; then
    echo "   ❌ 前端构建失败"
    exit 1
fi
cd ..
echo "   ✅ 前端构建完成"
echo ""

# 3. 构建后端（自动嵌入前端）
echo "3️⃣  构建后端..."
go build -o ytb2bili.exe .
if [ $? -ne 0 ]; then
    echo "   ❌ 后端构建失败"
    exit 1
fi
echo "   ✅ 后端构建完成"
echo ""

echo "🎉 构建成功！"
echo ""
echo "📝 下一步操作："
echo "   1. 停止当前运行的服务器（Ctrl+C）"
echo "   2. 运行: ./ytb2bili.exe"
echo "   3. 清除浏览器缓存（Ctrl+Shift+R）"
echo ""
