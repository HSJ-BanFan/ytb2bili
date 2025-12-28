#!/bin/bash
# 快速集成进度日志系统的辅助脚本

set -e

echo "================================================"
echo "  进度日志系统快速集成工具"
echo "================================================"
echo

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查是否在项目根目录
if [ ! -f "main.go" ] || [ ! -d "pkg/logger" ]; then
    echo "❌ 错误: 请在项目根目录运行此脚本"
    exit 1
fi

echo -e "${GREEN}✓${NC} 检测到项目根目录"
echo

# 步骤 1: 检查必要的文件
echo "📋 检查必要的文件..."
files=(
    "pkg/logger/progress_logger.go"
    "pkg/logger/smart_logger.go"
    "pkg/logger/compact_progress.go"
)

all_exist=true
for file in "${files[@]}"; do
    if [ -f "$file" ]; then
        echo -e "  ${GREEN}✓${NC} $file"
    else
        echo -e "  ${YELLOW}✗${NC} $file (缺失)"
        all_exist=false
    fi
done

if [ "$all_exist" = false ]; then
    echo
    echo "❌ 缺少必要的文件，请确保已经创建了所有日志模块"
    exit 1
fi

echo
echo -e "${GREEN}✓${NC} 所有必要的文件都已就位"
echo

# 步骤 2: 检查 main.go 是否已初始化
echo "🔍 检查 main.go 初始化状态..."
if grep -q "InitProgressLogManager" main.go; then
    echo -e "  ${GREEN}✓${NC} main.go 已包含进度日志管理器初始化"
else
    echo -e "  ${YELLOW}⚠${NC} main.go 需要添加初始化代码"
    echo
    echo "请在 main.go 的 fx.Provide 部分（日志模块之后）添加:"
    echo
    echo "  // 初始化进度日志管理器"
    echo "  fx.Invoke(func(lg *zap.SugaredLogger) {"
    echo "      logger.InitProgressLogManager(lg)"
    echo "  }),"
    echo
    read -p "是否自动添加? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # 查找插入位置（在日志模块之后）
        awk '
            /fx\.Provide\(func\(config \*types\.AppConfig\) \(\*zap\.SugaredLogger, error\) \{/ {
                print
                getline
                print
                getline
                print
                print "\t\t// 初始化进度日志管理器"
                print "\t\tfx.Invoke(func(lg *zap.SugaredLogger) {"
                print "\t\t\tlogger.InitProgressLogManager(lg)"
                print "\t\t}),"
                next
            }
            { print }
        ' main.go > main.go.tmp && mv main.go.tmp main.go
        echo -e "${GREEN}✓${NC} 已添加初始化代码"
    fi
fi

echo

# 步骤 3: 创建备份
echo "💾 创建备份..."
backup_dir="backup_before_progress_logging_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$backup_dir"
cp -r internal/chain_task/handlers/down_load_video.go "$backup_dir/" 2>/dev/null || true
cp -r internal/chain_task/chain_task_handler.go "$backup_dir/" 2>/dev/null || true
echo -e "  ${GREEN}✓${NC} 备份创建在: $backup_dir"

echo

# 步骤 4: 提供集成选项
echo "📝 集成选项:"
echo
echo "1. 仅添加智能日志到定时任务 (推荐先做，风险低)"
echo "2. 添加紧凑进度条到下载视频 (需要更多测试)"
echo "3. 同时执行 1 和 2"
echo "4. 跳过，稍后手动集成"
echo
read -p "请选择 (1-4): " choice

case $choice in
    1)
        echo
        echo "添加智能日志到定时任务..."
        # 这里可以添加自动修改代码的逻辑
        echo "请参考 docs/PROGRESS_LOGGING_GUIDE.md 手动添加 SmartLogger"
        ;;
    2)
        echo
        echo "添加紧凑进度条到下载视频..."
        echo "请参考 docs/compact_progress_example.go 手动集成"
        ;;
    3)
        echo
        echo "同时执行两个集成..."
        echo "请参考文档手动集成"
        ;;
    4)
        echo
        echo "跳过自动集成"
        ;;
    *)
        echo "无效选择"
        exit 1
        ;;
esac

echo
echo "================================================"
echo -e "${GREEN}✓ 集成准备完成!${NC}"
echo "================================================"
echo
echo "下一步:"
echo "1. 阅读 docs/PROGRESS_LOGGING_GUIDE.md"
echo "2. 查看 docs/compact_progress_example.go"
echo "3. 根据需要修改代码"
echo "4. 测试新功能"
echo
echo "如需回滚，备份位于: $backup_dir"
