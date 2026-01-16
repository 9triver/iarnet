#!/bin/bash
# 推送镜像到 Harbor 私有仓库（用于离线部署）

set -e

# 配置 Harbor 地址
HARBOR_HOST="${HARBOR_HOST:-localhost}"
HARBOR_PORT="${HARBOR_PORT:-80}"
HARBOR_PROJECT="${HARBOR_PROJECT:-iarnet}"
HARBOR_USER="${HARBOR_USER:-admin}"
HARBOR_PASS="${HARBOR_PASS:-Harbor12345}"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 需要推送的镜像列表
IMAGES=(
    "iarnet:latest"
    "iarnet-global:latest"
    "iarnet/provider:docker"
    "iarnet/runner:python_3.11-latest"
    "iarnet/component:python_3.11-latest"
)

# 登录 Harbor
login_harbor() {
    echo_info "登录 Harbor: ${HARBOR_HOST}:${HARBOR_PORT}"
    
    if [ -n "$HARBOR_PASS" ]; then
        echo "$HARBOR_PASS" | docker login "${HARBOR_HOST}:${HARBOR_PORT}" -u "$HARBOR_USER" --password-stdin
    else
        docker login "${HARBOR_HOST}:${HARBOR_PORT}" -u "$HARBOR_USER"
    fi
    
    if [ $? -eq 0 ]; then
        echo_info "登录成功"
    else
        echo_error "登录失败"
        exit 1
    fi
}

# 推送镜像
push_image() {
    local image=$1
    local harbor_image="${HARBOR_HOST}:${HARBOR_PORT}/${HARBOR_PROJECT}/${image}"
    
    echo_info "推送: $image -> $harbor_image"
    
    # 标记镜像
    docker tag "$image" "$harbor_image"
    
    # 推送镜像
    docker push "$harbor_image"
    
    if [ $? -eq 0 ]; then
        echo_info "  ✓ 推送成功"
    else
        echo_error "  ✗ 推送失败"
        return 1
    fi
}

# 主函数
main() {
    echo_info "=== 推送镜像到 Harbor ==="
    echo ""
    echo_info "Harbor 地址: ${HARBOR_HOST}:${HARBOR_PORT}"
    echo_info "项目名称: ${HARBOR_PROJECT}"
    echo ""
    
    # 检查参数
    if [ "$1" == "--help" ] || [ "$1" == "-h" ]; then
        echo "用法: $0 [HARBOR_HOST] [HARBOR_PORT] [HARBOR_PROJECT]"
        echo ""
        echo "环境变量:"
        echo "  HARBOR_HOST      - Harbor 主机地址（默认: localhost）"
        echo "  HARBOR_PORT      - Harbor 端口（默认: 80）"
        echo "  HARBOR_PROJECT   - Harbor 项目名称（默认: iarnet）"
        echo "  HARBOR_USER      - Harbor 用户名（默认: admin）"
        echo "  HARBOR_PASS      - Harbor 密码（默认: Harbor12345）"
        echo ""
        echo "示例:"
        echo "  HARBOR_HOST=192.168.1.100 HARBOR_PORT=8080 $0"
        exit 0
    fi
    
    # 使用命令行参数覆盖环境变量
    if [ -n "$1" ]; then
        HARBOR_HOST="$1"
    fi
    if [ -n "$2" ]; then
        HARBOR_PORT="$2"
    fi
    if [ -n "$3" ]; then
        HARBOR_PROJECT="$3"
    fi
    
    # 登录
    login_harbor
    
    # 推送所有镜像
    local failed=0
    for img in "${IMAGES[@]}"; do
        if ! docker image inspect "$img" &>/dev/null; then
            echo_warn "镜像不存在，跳过: $img"
            continue
        fi
        
        if ! push_image "$img"; then
            failed=$((failed + 1))
        fi
        echo ""
    done
    
    # 总结
    echo_info "=== 推送完成 ==="
    if [ $failed -eq 0 ]; then
        echo_info "所有镜像推送成功！"
        echo ""
        echo_info "在目标服务器上拉取镜像:"
        echo "  docker pull ${HARBOR_HOST}:${HARBOR_PORT}/${HARBOR_PROJECT}/iarnet:latest"
    else
        echo_error "有 $failed 个镜像推送失败"
        exit 1
    fi
}

main "$@"

