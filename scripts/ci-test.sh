#!/bin/bash

# CI/CD 本地测试脚本
# 模拟 GitHub Actions 的测试流程

set -e  # 遇到错误立即退出

echo "========================================="
echo "🚀 CI/CD Local Test"
echo "========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# ========================================
# Step 1: 代码格式化检查
# ========================================
echo -e "${YELLOW}📝 Step 1: Checking code format...${NC}"
if [ -n "$(gofmt -l .)" ]; then
    echo -e "${RED}❌ Code is not formatted. Run 'go fmt ./...'${NC}"
    gofmt -d .
    exit 1
else
    echo -e "${GREEN}✅ Code is properly formatted${NC}"
fi
echo ""

# ========================================
# Step 2: 依赖验证
# ========================================
echo -e "${YELLOW}📦 Step 2: Verifying dependencies...${NC}"
go mod verify
echo -e "${GREEN}✅ Dependencies verified${NC}"
echo ""

# ========================================
# Step 3: Lint 检查（如果安装了 golangci-lint）
# ========================================
echo -e "${YELLOW}🔍 Step 3: Running linter...${NC}"
if command -v golangci-lint &> /dev/null; then
    golangci-lint run --timeout=5m
    echo -e "${GREEN}✅ Linter passed${NC}"
else
    echo -e "${YELLOW}⚠️  golangci-lint not found. Skipping...${NC}"
    echo "   Install: https://golangci-lint.run/usage/install/"
fi
echo ""

# ========================================
# Step 4: 单元测试
# ========================================
echo -e "${YELLOW}🧪 Step 4: Running unit tests...${NC}"
echo "-------------------------------------------"
go test -v -short ./... -exclude-dir tests/integration
echo -e "${GREEN}✅ Unit tests passed${NC}"
echo ""

# ========================================
# Step 5: 单元测试覆盖率
# ========================================
echo -e "${YELLOW}📊 Step 5: Unit test coverage...${NC}"
go test -race -coverprofile=coverage_unit.out -covermode=atomic ./...
go tool cover -func=coverage_unit.out | grep total
echo -e "${GREEN}✅ Coverage report generated${NC}"
echo ""

# ========================================
# Step 6: 集成测试
# ========================================
echo -e "${YELLOW}🧪 Step 6: Running integration tests...${NC}"
echo "-------------------------------------------"
go test -v -race -coverprofile=coverage_integration.out -covermode=atomic ./tests/integration/...
echo -e "${GREEN}✅ Integration tests passed${NC}"
echo ""

# ========================================
# Step 7: 合并覆盖率报告
# ========================================
echo -e "${YELLOW}📊 Step 7: Merging coverage reports...${NC}"
echo "mode: atomic" > coverage.out
grep -h "^.*:[0-9]*:.*" coverage_unit.out coverage_integration.out | sort -u >> coverage.out

TOTAL_COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
echo -e "${GREEN}✅ Total Coverage: $TOTAL_COVERAGE${NC}"
echo ""

# ========================================
# Step 8: 生成 HTML 报告
# ========================================
echo -e "${YELLOW}📄 Step 8: Generating HTML coverage report...${NC}"
go tool cover -html=coverage.out -o coverage.html
echo -e "${GREEN}✅ HTML report generated: coverage.html${NC}"
echo ""

# ========================================
# 完成
# ========================================
echo "========================================="
echo -e "${GREEN}🎉 All tests passed!${NC}"
echo "========================================="
echo ""
echo "📊 Coverage Summary:"
echo "   - Unit Tests: $(go tool cover -func=coverage_unit.out | grep total | awk '{print $3}')"
echo "   - Integration Tests: $(go tool cover -func=coverage_integration.out | grep total | awk '{print $3}')"
echo "   - Total: $TOTAL_COVERAGE"
echo ""
echo "📄 View HTML report:"
echo "   open coverage.html"
echo ""
