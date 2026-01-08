#!/bin/bash

# 自动同步 component 镜像到 dind-*、runner 镜像到 iarnet-* 的脚本
# 使用方法: 
#   ./sync-images-to-dind.sh          # 同步所有镜像（component 和 runner）
#   ./sync-images-to-dind.sh --runner-only  # 仅同步 runner 镜像到 iarnet
#   ./sync-images-to-dind.sh -r        # 同上（简短形式）

set -euo pipefail

# 解析命令行参数
SYNC_RUNNER_ONLY=false
if [[ "$#" -gt 0 ]]; then
    case "$1" in
        --runner-only|-r)
            SYNC_RUNNER_ONLY=true
            ;;
        --help|-h)
            echo "用法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  --runner-only, -r    仅同步 runner 镜像到 iarnet（跳过 component 镜像）"
            echo "  --help, -h           显示此帮助信息"
            exit 0
            ;;
        *)
            echo "错误: 未知选项 '$1'"
            echo "使用 --help 查看帮助信息"
            exit 1
            ;;
    esac
fi

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 获取目标容器对应的 Docker Host
get_target_host() {
    local name=$1
    case $name in
        dind-1) echo "tcp://localhost:23751" ;;
        dind-2) echo "tcp://localhost:23752" ;;
        dind-3) echo "tcp://localhost:23753" ;;
        iarnet-1) echo "tcp://localhost:23761" ;;
        iarnet-2) echo "tcp://localhost:23762" ;;
        iarnet-3) echo "tcp://localhost:23763" ;;
        *)
            echo ""
            ;;
    esac
}

# 同步镜像到指定的 Docker 节点（dind 或 iarnet）
sync_image_to_target() {
    local target=$1
    local image_name=$2
    local target_host
    target_host=$(get_target_host "$target")

    if [ -z "$target_host" ]; then
        echo -e "${RED}未知目标: ${target}${NC}"
        return 1
    fi

    echo -e "${YELLOW}同步镜像 ${image_name} 到 ${target} (${target_host})...${NC}"

    # 检查镜像是否存在
    if ! docker image inspect "$image_name" > /dev/null 2>&1; then
        echo -e "${RED}警告: 镜像 ${image_name} 在宿主机上不存在，跳过${NC}"
        return 1
    fi

    # 导出并导入
    docker save "$image_name" | docker -H "$target_host" load

    echo -e "${GREEN}✅ 镜像 ${image_name} 已同步到 ${target}${NC}"
    return 0
}

DIND_TARGETS=("dind-1" "dind-2" "dind-3")
IARNET_TARGETS=("iarnet-1" "iarnet-2" "iarnet-3")

if [ "$SYNC_RUNNER_ONLY" = true ]; then
    echo -e "${YELLOW}开始同步 runner -> iarnet 镜像（跳过 component）...${NC}"
else
echo -e "${YELLOW}开始同步 component -> dind、runner -> iarnet 镜像...${NC}"
fi
echo ""

# 查找所有需要的镜像
if [ "$SYNC_RUNNER_ONLY" = false ]; then
echo -e "${YELLOW}查找 component/runner 镜像...${NC}"
COMPONENT_IMAGES=$(docker images --format "{{.Repository}}:{{.Tag}}" | grep "^iarnet/component" || true)
else
    echo -e "${YELLOW}查找 runner 镜像...${NC}"
    COMPONENT_IMAGES=""
fi
RUNNER_IMAGES=$(docker images --format "{{.Repository}}:{{.Tag}}" | grep "^iarnet/runner" || true)

if [ -z "$COMPONENT_IMAGES" ] && [ -z "$RUNNER_IMAGES" ]; then
    echo -e "${RED}错误: 未找到任何 iarnet/component 或 iarnet/runner 镜像${NC}"
    echo -e "${YELLOW}请确保已在宿主机上构建了这些镜像${NC}"
    exit 1
fi

# 显示找到的镜像
if [ "$SYNC_RUNNER_ONLY" = false ]; then
if [ -n "$COMPONENT_IMAGES" ]; then
    echo -e "${GREEN}组件镜像:${NC}"
    echo "$COMPONENT_IMAGES" | while read -r img; do
        echo -e "  - ${img}"
    done
else
    echo -e "${YELLOW}未找到 component 镜像，跳过同步到 dind${NC}"
    fi
fi

if [ -n "$RUNNER_IMAGES" ]; then
    echo -e "${GREEN}Runner 镜像:${NC}"
    echo "$RUNNER_IMAGES" | while read -r img; do
        echo -e "  - ${img}"
    done
else
    echo -e "${YELLOW}未找到 runner 镜像，跳过同步到 iarnet${NC}"
fi

echo ""

if [ "$SYNC_RUNNER_ONLY" = false ] && [ -n "$COMPONENT_IMAGES" ]; then
    for target in "${DIND_TARGETS[@]}"; do
        echo -e "${YELLOW}========== 同步 component 到 ${target} ==========${NC}"
        echo "$COMPONENT_IMAGES" | while read -r img; do
            sync_image_to_target "$target" "$img"
        done
        echo ""
    done
fi

if [ -n "$RUNNER_IMAGES" ]; then
    for target in "${IARNET_TARGETS[@]}"; do
        echo -e "${YELLOW}========== 同步 runner 到 ${target} ==========${NC}"
        echo "$RUNNER_IMAGES" | while read -r img; do
            sync_image_to_target "$target" "$img"
        done
        echo ""
    done
fi

echo ""
echo -e "${GREEN}✅ 所有镜像同步完成!${NC}"
echo ""

# 显示各节点的镜像列表
if [ "$SYNC_RUNNER_ONLY" = false ]; then
for target in "${DIND_TARGETS[@]}" "${IARNET_TARGETS[@]}"; do
    host=$(get_target_host "$target")
    echo -e "${YELLOW}${target} 中的 component/runner 镜像:${NC}"
    docker -H "$host" images | grep -E "(REPOSITORY|iarnet/(component|runner))" || echo "无"
    echo ""
done
else
    for target in "${IARNET_TARGETS[@]}"; do
        host=$(get_target_host "$target")
        echo -e "${YELLOW}${target} 中的 runner 镜像:${NC}"
        docker -H "$host" images | grep -E "(REPOSITORY|iarnet/runner)" || echo "无"
        echo ""
    done
fi

