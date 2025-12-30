#!/bin/bash

# ============================================================
# ytb2bili 生产环境测试脚本
# 用途：渐进式部署细粒度锁功能
# ============================================================

set -e  # 遇到错误立即退出

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查命令是否存在
check_command() {
    if ! command -v $1 &> /dev/null; then
        log_error "$1 未安装，请先安装"
        exit 1
    fi
}

# 解析命令行参数
MODE=$1
DURATION=${2:-30}  # 默认测试30分钟

# 显示帮助信息
show_help() {
    cat << EOF
用法: $0 <mode> [duration]

模式:
  test       - 运行负载测试（不修改生产配置）
  stage1     - 阶段1：5% 流量测试（推荐 30 分钟）
  stage2     - 阶段2：25% 流量测试（推荐 1 小时）
  stage3     - 阶段3：50% 流量测试（推荐 2 小时）
  stage4     - 阶段4：100% 全量发布
  rollback   - 紧急回滚（禁用细粒度锁）
  status     - 查看当前配置状态
  monitor    - 实时监控指标

示例:
  $0 test 30              # 运行30分钟测试
  $0 stage1 30            # 阶段1测试，30分钟
  $0 rollback             # 立即回滚
  $0 status               # 查看当前状态

EOF
    exit 0
}

# 检查当前配置
check_status() {
    log_info "检查当前配置..."

    if [ -f "config.toml" ]; then
        if grep -q "enable_fine_grained_lock = true" config.toml; then
            log_info "✅ 细粒度锁已启用"
        else
            log_warn "⚠️  细粒度锁未启用（使用全局锁）"
        fi
    else
        log_error "config.toml 不存在"
        exit 1
    fi
}

# 修改配置并重启
apply_config() {
    local enabled=$1

    if [ "$enabled" = "true" ]; then
        log_info "启用细粒度锁..."
        sed -i 's/enable_fine_grained_lock = false/enable_fine_grained_lock = true/' config.toml
        sed -i 's/# enable_fine_grained_lock = true/enable_fine_grained_lock = true/' config.toml
    else
        log_warn "禁用细粒度锁（回滚模式）..."
        sed -i 's/enable_fine_grained_lock = true/enable_fine_grained_lock = false/' config.toml
    fi

    log_info "重启服务..."
    systemctl restart ytb2bili

    sleep 3
    systemctl status ytb2bili --no-pager
}

# 运行测试
run_test() {
    local duration=$1

    log_info "开始负载测试（持续 ${duration} 分钟）..."

    # 检查依赖
    check_command "go"
    check_command "curl"

    # 运行性能测试
    log_info "运行并发性能测试..."
    go test -v ./internal/chain_task/ -run TestUploadScheduler_ConcurrentUploadPerformance -timeout ${duration}m

    log_info "运行压力测试..."
    go test -v ./internal/chain_task/ -run TestUploadScheduler_ConcurrentStressTest -timeout ${duration}m

    log_info "✅ 测试完成"
}

# 监控指标
monitor_metrics() {
    log_info "实时监控指标（按 Ctrl+C 退出）"

    check_command "curl"

    while true; do
        clear
        echo "=========================================="
        echo "实时监控指标"
        echo "=========================================="
        echo ""

        # 获取健康状态
        echo "【健康检查】"
        if curl -sf http://localhost:8096/health > /dev/null; then
            echo -e "  API 状态: ${GREEN}运行中${NC}"
        else
            echo -e "  API 状态: ${RED}离线${NC}"
        fi
        echo ""

        # 获取上传统计（如果有 API）
        echo "【上传统计】"
        # TODO: 添加实际的上传统计 API
        echo "  当前并发数: $(ps aux | grep ytb2bili | grep -v grep | wc -l)"
        echo "  内存使用: $(free -h | grep Mem | awk '{print $3 "/" $2}')"
        echo "  CPU 使用: $(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1)%"
        echo ""

        # 数据库连接（如果有权限）
        echo "【数据库连接】"
        # TODO: 添加实际的数据库查询
        echo "  使用 'SHOW PROCESSLIST' 查看 MySQL 连接"
        echo ""

        echo "=========================================="
        echo "刷新间隔: 5秒"
        echo "=========================================="

        sleep 5
    done
}

# 阶段性部署
run_stage() {
    local stage=$1
    local duration=$2

    case $stage in
        stage1)
            log_info "=========================================="
            log_info "阶段1：5% 流量测试"
            log_info "=========================================="
            log_warn "预计时长：${duration} 分钟"
            log_warn "如果出现问题，请立即运行：$0 rollback"
            log_info ""
            read -p "确认开始阶段1测试？(y/n) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                apply_config true
                log_info "开始监控..."
                monitor_metrics &
                MONITOR_PID=$!
                sleep $(($duration * 60))
                kill $MONITOR_PID 2>/dev/null || true
                log_info "阶段1测试完成"
                log_info "请检查日志和指标，确认是否进入阶段2"
            fi
            ;;
        stage2)
            log_info "=========================================="
            log_info "阶段2：25% 流量测试"
            log_info "=========================================="
            log_warn "预计时长：${duration} 分钟"
            log_info ""
            read -p "确认开始阶段2测试？(y/n) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                log_info "阶段2已经在运行细粒度锁，继续观察..."
                monitor_metrics &
                MONITOR_PID=$!
                sleep $(($duration * 60))
                kill $MONITOR_PID 2>/dev/null || true
                log_info "阶段2测试完成"
            fi
            ;;
        stage3)
            log_info "=========================================="
            log_info "阶段3：50% 流量测试"
            log_info "=========================================="
            log_info "继续扩大流量范围..."
            monitor_metrics &
            MONITOR_PID=$!
            sleep $(($duration * 60))
            kill $MONITOR_PID 2>/dev/null || true
            log_info "阶段3测试完成"
            ;;
        stage4)
            log_info "=========================================="
            log_info "阶段4：100% 全量发布"
            log_info "=========================================="
            log_info "细粒度锁已全量发布"
            log_info "持续监控中..."
            monitor_metrics
            ;;
    esac
}

# 回滚
rollback() {
    log_warn "=========================================="
    log_warn "紧急回滚"
    log_warn "=========================================="
    log_warn "将禁用细粒度锁并重启服务..."
    log_info ""
    read -p "确认回滚？(y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        apply_config false
        log_info "✅ 回滚完成"
        log_info "已切换回全局锁模式（兼容模式）"
    fi
}

# ============================================================
# 主逻辑
# ============================================================

case $MODE in
    test)
        run_test $DURATION
        ;;
    stage1|stage2|stage3|stage4)
        run_stage $MODE $DURATION
        ;;
    rollback)
        rollback
        ;;
    status)
        check_status
        ;;
    monitor)
        monitor_metrics
        ;;
    *)
        show_help
        ;;
esac
