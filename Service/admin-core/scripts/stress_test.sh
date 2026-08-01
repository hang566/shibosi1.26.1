#!/bin/bash
# ============================================
# 市舶司 v2.0 性能压测脚本
# 目标: 单机4C8G ≥ 2000 QPS，P99 < 50ms
# 使用: wrk 或 vegeta
# ============================================

API_BASE="http://localhost:8080"
TOKEN=""  # 运行前先获取Token

# ============================================
# 1. 获取JWT Token（压测前执行）
# ============================================
get_token() {
    TOKEN=$(curl -s -X POST "${API_BASE}/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin123"}' \
        | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
    echo "Token: ${TOKEN:0:20}..."
}

# ============================================
# 2. wrk 压测脚本
# ============================================
# 安装: brew install wrk (macOS) / apt install wrk (Linux)

run_wrk_health() {
    echo "=== 压测: 健康检查接口 (预期: > 5000 QPS) ==="
    wrk -t4 -c100 -d30s --latency "${API_BASE}/api/health"
}

run_wrk_auth_profile() {
    echo "=== 压测: 鉴权接口 - 获取用户信息 (预期: ≥ 2000 QPS, P99 < 50ms) ==="
    wrk -t4 -c100 -d30s --latency \
        -H "Authorization: Bearer ${TOKEN}" \
        "${API_BASE}/api/v1/auth/profile"
}

run_wrk_dashboard() {
    echo "=== 压测: 仪表盘数据 (预期: ≥ 1500 QPS) ==="
    wrk -t4 -c100 -d30s --latency \
        -H "Authorization: Bearer ${TOKEN}" \
        "${API_BASE}/api/v1/admin/dashboard"
}

run_wrk_user_list() {
    echo "=== 压测: 用户列表 (预期: ≥ 1000 QPS) ==="
    wrk -t4 -c100 -d30s --latency \
        -H "Authorization: Bearer ${TOKEN}" \
        "${API_BASE}/api/v1/admin/users?page=1&page_size=20"
}

run_wrk_login() {
    echo "=== 压测: 登录接口 (预期: ≥ 500 QPS) ==="
    wrk -t4 -c50 -d30s --latency \
        -s <(cat <<'SCRIPT'
wrk.method = "POST"
wrk.body   = '{"username":"admin","password":"admin123"}'
wrk.headers["Content-Type"] = "application/json"
SCRIPT
) "${API_BASE}/api/v1/auth/login"
}

# ============================================
# 3. vegeta 压测脚本（更精确的延迟分布）
# ============================================
# 安装: go install github.com/tsenart/vegeta/v12@latest

run_vegeta_profile() {
    echo "=== Vegeta: 鉴权接口 P99延迟测试 ==="
    echo "GET ${API_BASE}/api/v1/auth/profile" | \
    vegeta attack -rate=2000 -duration=30s \
        -header "Authorization: Bearer ${TOKEN}" \
        -format=http | \
    vegeta report -type=text
}

run_vegeta_burst() {
    echo "=== Vegeta: 突发流量测试 (3000 QPS 持续10秒) ==="
    echo "GET ${API_BASE}/api/v1/health" | \
    vegeta attack -rate=3000 -duration=10s | \
    vegeta report -type=text
}

run_vegeta_ramp() {
    echo "=== Vegeta: 阶梯加压测试 (500 -> 5000 QPS) ==="
    echo "GET ${API_BASE}/api/v1/auth/profile" | \
    vegeta attack -rate=500/1s -duration=5s \
        -header "Authorization: Bearer ${TOKEN}" \
        -format=http > /tmp/ramp.bin && \
    echo "GET ${API_BASE}/api/v1/auth/profile" | \
    vegeta attack -rate=1000/1s -duration=5s \
        -header "Authorization: Bearer ${TOKEN}" \
        -format=http >> /tmp/ramp.bin && \
    echo "GET ${API_BASE}/api/v1/auth/profile" | \
    vegeta attack -rate=2000/1s -duration=5s \
        -header "Authorization: Bearer ${TOKEN}" \
        -format=http >> /tmp/ramp.bin && \
    echo "GET ${API_BASE}/api/v1/auth/profile" | \
    vegeta attack -rate=5000/1s -duration=5s \
        -header "Authorization: Bearer ${TOKEN}" \
        -format=http >> /tmp/ramp.bin && \
    vegeta report -type=text /tmp/ramp.bin
}

# ============================================
# 4. 瓶颈分析（压测后执行）
# ============================================
analyze_bottleneck() {
    echo "=== 瓶颈分析检查清单 ==="
    echo "1. 检查Go runtime: GODEBUG=gctrace=1"
    echo "2. 检查数据库连接池: SHOW PROCESSLIST;"
    echo "3. 检查Redis连接: redis-cli INFO clients"
    echo "4. 检查Nginx连接数: ss -s"
    echo "5. 检查文件描述符: ulimit -n"
    echo "6. 检查CPU/内存: top -p \$(pgrep shibosi-admin)"
    echo ""
    echo "=== 常见优化方案 ==="
    echo "- 连接池不足: 增大 MaxOpenConns / Redis PoolSize"
    echo "- GC压力大: 使用 sync.Pool 复用对象，减少内存分配"
    echo "- 数据库慢查询: 添加索引，启用慢查询日志"
    echo "- 序列化瓶颈: 使用 json-iterator 或 easyjson 替换标准库"
    echo "- 网络瓶颈: 启用 HTTP/2，调整内核参数"
}

# ============================================
# 5. 主流程
# ============================================
main() {
    echo "=========================================="
    echo "  市舶司 v2.0 性能压测"
    echo "  目标: 鉴权接口 ≥ 2000 QPS, P99 < 50ms"
    echo "=========================================="
    get_token

    if [ -z "$TOKEN" ]; then
        echo "获取Token失败，请确认服务已启动"
        exit 1
    fi

    echo ""
    echo "开始压测..."
    echo ""

    # 健康检查
    run_wrk_health

    echo ""
    echo "---"

    # 鉴权接口（核心指标）
    run_wrk_auth_profile

    echo ""
    echo "---"

    # 仪表盘
    run_wrk_dashboard

    echo ""
    echo "---"

    # 用户列表
    run_wrk_user_list

    echo ""
    echo "---"

    # 登录接口
    run_wrk_login

    echo ""
    echo "=========================================="
    echo "压测完成！请检查QPS和P99延迟是否达标"
    echo "=========================================="

    analyze_bottleneck
}

main "$@"