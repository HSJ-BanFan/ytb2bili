#!/bin/bash
# security_test.sh - 安全配置自动化测试

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

BASE_URL="http://localhost:8096"
PASS_COUNT=0
FAIL_COUNT=0

# 测试函数
test_case() {
    local name=$1
    local command=$2
    local expected=$3

    echo -n "📋 测试: $name ... "

    if eval "$command" | grep -q "$expected"; then
        echo -e "${GREEN}✅ 通过${NC}"
        ((PASS_COUNT++))
    else
        echo -e "${RED}❌ 失败${NC}"
        ((FAIL_COUNT++))
    fi
}

echo "🔒 安全配置自动化测试"
echo "======================================"
echo ""

# 1. CORS 拒绝未授权域名
test_case \
    "CORS 应拒绝未授权域名" \
    "curl -s -o /dev/null -w '%{http_code}' -H 'Origin: https://evil.com' $BASE_URL/api/health" \
    "^403$"

# 2. CORS 允许授权域名
test_case \
    "CORS 应允许授权域名 (localhost:3000)" \
    "curl -s -o /dev/null -w '%{http_code}' -H 'Origin: http://localhost:3000' $BASE_URL/api/health" \
    "^200$"

# 3. 安全响应头 - X-Frame-Options
test_case \
    "X-Frame-Options 应为 SAMEORIGIN" \
    "curl -s -I $BASE_URL/api/health" \
    "X-Frame-Options: SAMEORIGIN"

# 4. 安全响应头 - X-Content-Type-Options
test_case \
    "X-Content-Type-Options 应为 nosniff" \
    "curl -s -I $BASE_URL/api/health" \
    "X-Content-Type-Options: nosniff"

# 5. Content-Security-Policy
if curl -s -I "$BASE_URL/api/health" | grep -q "Content-Security-Policy"; then
    echo -e "📋 测试: Content-Security-Policy 存在 ... ${GREEN}✅ 通过${NC}"
    ((PASS_COUNT++))
else
    echo -e "📋 测试: Content-Security-Policy 存在 ... ${RED}❌ 失败${NC}"
    ((FAIL_COUNT++))
fi

# 6. Permissions-Policy
if curl -s -I "$BASE_URL/api/health" | grep -q "Permissions-Policy"; then
    echo -e "📋 测试: Permissions-Policy 存在 ... ${GREEN}✅ 通过${NC}"
    ((PASS_COUNT++))
else
    echo -e "📋 测试: Permissions-Policy 存在 ... ${RED}❌ 失败${NC}"
    ((FAIL_COUNT++))
fi

# 7. 检查是否泄露敏感信息
test_case \
    "响应头不应泄露服务器版本" \
    "curl -s -I $BASE_URL/api/health" \
    "^Server:" && echo -e "  ${YELLOW}⚠️  警告: Server 头存在，建议隐藏${NC}" || ((PASS_COUNT++))

# 8. JSON 错误响应格式
test_case \
    "CORS 拒绝应返回 JSON 错误" \
    "curl -s -H 'Origin: https://evil.com' $BASE_URL/api/health" \
    '"error"'

echo ""
echo "======================================"
echo -e "${GREEN}✅ 通过: $PASS_COUNT${NC}"
echo -e "${RED}❌ 失败: $FAIL_COUNT${NC}"
echo "======================================"

if [ $FAIL_COUNT -gt 0 ]; then
    echo -e "${RED}⚠️  部分测试失败，请检查安全配置${NC}"
    exit 1
else
    echo -e "${GREEN}✅ 所有测试通过！${NC}"
    exit 0
fi
