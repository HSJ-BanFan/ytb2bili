#!/bin/sh
set -e

# Docker 容器启动脚本
# 用于在运行时注入环境变量配置

echo "Starting ytb2bili frontend..."

# 如果提供了 BACKEND_URL 环境变量，创建运行时配置文件
if [ -n "$BACKEND_URL" ]; then
    echo "Configuring backend URL: $BACKEND_URL"
    cat > /usr/share/nginx/html/env-config.js <<EOF
// 运行时环境配置（由 Docker 容器启动时生成）
window.ENV = {
    BACKEND_URL: '$BACKEND_URL',
    API_BASE_URL: '/api/v1'
};
EOF
else
    echo "Using default backend configuration"
fi

# 启动 Nginx
echo "Starting Nginx..."
exec nginx -g 'daemon off;'
