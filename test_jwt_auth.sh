#!/bin/bash

echo "=================================="
echo "JWT 认证系统调试脚本"
echo "=================================="
echo ""

API_BASE="http://localhost:8096/api/v1"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass_count=0
fail_count=0

# 测试函数
test_api() {
    local name="$1"
    local method="$2"
    local endpoint="$3"
    local token="$4"
    local data="$5"
    local expected_code="$6"

    echo -n "测试: $name ... "

    headers="-H \"Content-Type: application/json\""
    if [ -n "$token" ]; then
        headers="$headers -H \"Authorization: Bearer $token\""
    fi

    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "\n%{http_code}" -X GET "$API_BASE$endpoint" $headers 2>/dev/null)
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$API_BASE$endpoint" $headers -d "$data" 2>/dev/null)
    fi

    actual_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)

    if [ "$actual_code" = "$expected_code" ]; then
        echo -e "${GREEN}✅ PASS${NC} (HTTP $actual_code)"
        ((pass_count++))
        return 0
    else
        echo -e "${RED}❌ FAIL${NC} (期望: $expected_code, 实际: $actual_code)"
        echo "   响应: $body"
        ((fail_count++))
        return 1
    fi
}

echo "=================================="
echo "步骤 1: 测试登录"
echo "=================================="

login_response=$(curl -s -X POST "$API_BASE/user/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"mei","password":"123456"}')

echo "登录响应: $login_response"

# 提取 token
TOKEN=$(echo $login_response | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
    echo -e "${RED}❌ 登录失败，无法获取 Token${NC}"
    exit 1
fi

echo -e "${GREEN}✅ 登录成功${NC}"
echo "Token: ${TOKEN:0:50}..."
echo ""

echo "=================================="
echo "步骤 2: 测试认证 API（带 Token）"
echo "=================================="

test_api "获取配置 (GET)" \
    "GET" \
    "/config/download" \
    "$TOKEN" \
    "" \
    "200"

test_api "更新配置 (PUT)" \
    "PUT" \
    "/config/download" \
    "$TOKEN" \
    "{\"auto_upload_enabled\":true,\"video_upload_delay\":10}" \
    "200"

test_api "获取 OpenAI 配置 (GET)" \
    "GET" \
    "/config/openai-compatible" \
    "$TOKEN" \
    "" \
    "200"

test_api "获取 Gemini 配置 (GET)" \
    "GET" \
    "/config/gemini" \
    "$TOKEN" \
    "" \
    "200"

test_api "获取账号列表 (GET)" \
    "GET" \
    "/auth/accounts" \
    "$TOKEN" \
    "" \
    "200"

echo ""
echo "=================================="
echo "步骤 3: 测试未认证访问（不带 Token）"
echo "=================================="

test_api "获取配置 (无 Token)" \
    "GET" \
    "/config/download" \
    "" \
    "" \
    "401"

test_api "更新配置 (无 Token)" \
    "PUT" \
    "/config/download" \
    "" \
    "{\"auto_upload_enabled\":true}" \
    "401"

test_api "获取账号列表 (无 Token)" \
    "GET" \
    "/auth/accounts" \
    "" \
    "" \
    "401"

echo ""
echo "=================================="
echo "步骤 4: 测试管理员权限"
echo "=================================="

# 检查用户角色
user_info=$(curl -s "$API_BASE/user/me" -H "Authorization: Bearer $TOKEN")
echo "用户信息: $user_info"
role=$(echo $user_info | grep -o '"role":"[^"]*' | cut -d'"' -f4)

if [ "$role" = "admin" ]; then
    echo -e "${GREEN}✅ 用户角色: admin${NC}"
else
    echo -e "${YELLOW}⚠️  用户角色: $role (不是 admin)${NC}"
fi

echo ""
echo "=================================="
echo "测试总结"
echo "=================================="
echo -e "${GREEN}通过: $pass_count${NC}"
echo -e "${RED}失败: $fail_count${NC}"
echo ""

if [ $fail_count -eq 0 ]; then
    echo -e "${GREEN}🎉 所有测试通过！JWT 认证系统工作正常${NC}"
    exit 0
else
    echo -e "${RED}❌ 有测试失败，需要调试${NC}"
    exit 1
fi
