#!/bin/bash
# 优化 Docker 镜像大小（用于减少离线部署传输时间）
# 使用 docker-squash 等工具压缩镜像

set -e

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

# 检查 docker-squash 是否安装
check_docker_squash() {
    if ! command -v docker-squash &> /dev/null; then
        echo_warn "docker-squash 未安装"
        echo_info "安装方法:"
        echo "  pip install docker-squash"
        echo "  或者使用: pip3 install docker-squash"
        return 1
    fi
    return 0
}

# 分析镜像大小
analyze_image() {
    local image=$1
    echo_info "分析镜像: $image"
    
    # 显示镜像大小
    local size=$(docker image inspect "$image" --format='{{.Size}}' 2>/dev/null || echo "0")
    local size_human=$(numfmt --to=iec-i --suffix=B "$size" 2>/dev/null || echo "$size bytes")
    echo_info "  当前大小: $size_human"
    
    # 显示镜像层数
    local layers=$(docker image inspect "$image" --format='{{len .RootFS.Layers}}' 2>/dev/null || echo "0")
    echo_info "  层数: $layers"
    
    # 显示各层大小（前5层）
    echo_info "  主要层:"
    docker history "$image" --format "{{.Size}}\t{{.CreatedBy}}" --no-trunc | head -5 | while read line; do
        echo "    $line"
    done
}

# 压缩镜像（使用 docker-squash）
squash_image() {
    local image=$1
    local squashed_tag="${image}-squashed"
    
    echo_info "压缩镜像: $image -> $squashed_tag"
    
    if ! check_docker_squash; then
        echo_error "无法压缩: docker-squash 未安装"
        return 1
    fi
    
    # 使用 docker-squash 压缩
    docker-squash -t "$squashed_tag" "$image"
    
    # 显示压缩后大小
    local original_size=$(docker image inspect "$image" --format='{{.Size}}' 2>/dev/null || echo "0")
    local squashed_size=$(docker image inspect "$squashed_tag" --format='{{.Size}}' 2>/dev/null || echo "0")
    
    local original_human=$(numfmt --to=iec-i --suffix=B "$original_size" 2>/dev/null || echo "$original_size bytes")
    local squashed_human=$(numfmt --to=iec-i --suffix=B "$squashed_size" 2>/dev/null || echo "$squashed_size bytes")
    
    echo_info "  原始大小: $original_human"
    echo_info "  压缩后: $squashed_human"
    
    if [ $original_size -gt 0 ] && [ $squashed_size -gt 0 ]; then
        local saved=$((original_size - squashed_size))
        local saved_human=$(numfmt --to=iec-i --suffix=B "$saved" 2>/dev/null || echo "$saved bytes")
        local ratio=$(echo "scale=1; $squashed_size * 100 / $original_size" | bc 2>/dev/null || echo "N/A")
        echo_info "  节省: $saved_human (压缩率: ${ratio}%)"
    fi
    
    echo_warn "压缩后的镜像标签为: $squashed_tag"
    echo_warn "如需使用，请重新标记: docker tag $squashed_tag $image"
}

# 清理未使用的镜像和构建缓存
cleanup_docker() {
    echo_info "清理 Docker 未使用的资源..."
    
    # 清理未使用的镜像
    docker image prune -f
    
    # 清理构建缓存（可选，谨慎使用）
    echo_warn "是否清理构建缓存？这可能会影响后续构建速度 (y/N)"
    read -r response
    if [[ "$response" =~ ^[Yy]$ ]]; then
        docker builder prune -f
        echo_info "构建缓存已清理"
    fi
}

# 主函数
main() {
    echo_info "=== Docker 镜像优化工具 ==="
    echo ""
    
    IMAGES=(
        "iarnet:latest"
        "iarnet-global:latest"
        "iarnet/provider:docker"
    )
    
    case "${1:-analyze}" in
        analyze)
            echo_info "分析镜像大小..."
            for img in "${IMAGES[@]}"; do
                if docker image inspect "$img" &>/dev/null; then
                    analyze_image "$img"
                    echo ""
                else
                    echo_warn "镜像不存在: $img"
                fi
            done
            ;;
        squash)
            if [ -z "$2" ]; then
                echo_error "请指定要压缩的镜像"
                echo "用法: $0 squash <image:tag>"
                exit 1
            fi
            squash_image "$2"
            ;;
        cleanup)
            cleanup_docker
            ;;
        *)
            echo "用法: $0 [analyze|squash|cleanup]"
            echo ""
            echo "命令:"
            echo "  analyze  - 分析镜像大小（默认）"
            echo "  squash <image> - 压缩指定镜像"
            echo "  cleanup  - 清理未使用的 Docker 资源"
            exit 1
            ;;
    esac
}

main "$@"

