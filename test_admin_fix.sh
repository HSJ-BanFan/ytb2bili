#!/bin/bash

# 测试管理员权限修复的脚本
# 使用前请确保服务器已启动

echo "🧪 测试管理员权限修复"
echo "=========================="
echo ""

# 服务器地址
SERVER="http://localhost:8096"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 步骤1: 测试登录
echo "1️⃣  测试登录..."
LOGIN_RESPONSE=$(curl -s -X POST "$SERVER/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"mei","password":"123456"}')

TOKEN=$(echo $LOGIN_RESPONSE | grep -o '"token":"[^"]*' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
    echo -e "${RED}❌ 登录失败${NC}"
    echo "响应: $LOGIN_RESPONSE"
    exit 1
fi

echo -e "${GREEN}✅ 登录成功${NC}"
echo "Token: ${TOKEN:0:20}..."
echo ""

# 步骤2: 测试获取配置（应该成功）
echo "2️⃣  测试获取配置（GET请求）..."
GET_RESPONSE=$(curl -s -X GET "$SERVER/api/v1/config/gemini" \
  -H "Authorization: Bearer $TOKEN")

if echo "$GET_RESPONSE" | grep -q '"code":200'; then
    echo -e "${GREEN}✅ GET请求成功${NC}"
else
    echo -e "${RED}❌ GET请求失败${NC}"
    echo "响应: $GET_RESPONSE"
fi
echo ""

# 步骤3: 测试修改配置（管理员应该成功）
echo "3️⃣  测试修改配置（PUT请求 - 需要管理员权限）..."
PUT_RESPONSE=$(curl -s -X PUT "$SERVER/api/v1/config/gemini" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "api_key": "test-key-for-validation",
    "model": "gemini-2.0-flash-exp",
    "timeout": 120,
    "max_tokens": 8000
  }')

if echo "$PUT_RESPONSE" | grep -q '"code":200'; then
    echo -e "${GREEN}✅ PUT请求成功 - 管理员权限正常${NC}"
elif echo "$PUT_RESPONSE" | grep -q '"code":403'; then
    echo -e "${RED}❌ PUT请求被拒绝 - 权限不足（可能不是管理员）${NC}"
    echo "响应: $PUT_RESPONSE"
elif echo "$PUT_RESPONSE" | grep -q '"code":401'; then
    echo -e "${RED}❌ PUT请求被拒绝 - 未认证${NC}"
    echo "响应: $PUT_RESPONSE"
else
    echo -e "${YELLOW}⚠️  PUT请求返回未知响应${NC}"
    echo "响应: $PUT_RESPONSE"
fi
echo ""

# 步骤4: 检查用户角色
echo "4️⃣  检查当前用户角色..."
DECODED_TOKEN=$(echo -n "$TOKEN" | cut -d. -f2 | base64 -d 2>/dev/null || echo "")
echo "JWT Payload (可能包含role信息):"
echo "$DECODED_TOKEN" | grep -o '"role":"[^"]*"' || echo "  (role字段可能不在JWT中)"
echo ""

# 步骤5: 测试无Token访问（应该失败）
echo "5️⃣  测试无Token访问（应该失败）..."
NO_TOKEN_RESPONSE=$(curl -s -X GET "$SERVER/api/v1/config/gemini")

if echo "$NO_TOKEN_RESPONSE" | grep -q '"code":401'; then
    echo -e "${GREEN}✅ 无Token访问被正确拒绝${NC}"
else
    echo -e "${RED}❌ 无Token访问未被拦截（安全问题！）${NC}"
    echo "响应: $NO_TOKEN_RESPONSE"
fi
echo ""

echo "=========================="
echo -e "${GREEN}测试完成！${NC}"
echo ""
echo "📋 修复验证清单："
echo "  [ ] 能成功登录"
echo "  [ ] GET请求返回配置"
echo "  [ ] PUT请求修改成功（需要是admin角色）"
echo "  [ ] 无Token访问被拒绝"
echo ""
echo "💡 如果PUT请求失败（403），请检查数据库中mei用户的role字段是否为'admin'"
echo "   可以使用以下SQL手动修复:"
echo "   UPDATE cw_users SET role = 'admin' WHERE username = 'mei';"
