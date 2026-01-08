#!/bin/bash

# 清理 dind-* 与 iarnet-* 容器内部的所有 Docker 容器
# 使用方法：./cleanup-containers.sh

set -euo pipefail

YELLOW='\033[1;33m'
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

get_target_host() {
    case $1 in
        dind-1) echo "tcp://localhost:23751" ;;
        dind-2) echo "tcp://localhost:23752" ;;
        dind-3) echo "tcp://localhost:23753" ;;
        iarnet-1) echo "tcp://localhost:23761" ;;
        iarnet-2) echo "tcp://localhost:23762" ;;
        iarnet-3) echo "tcp://localhost:23763" ;;
        *) echo "" ;;
    esac
}

TARGETS=("dind-1" "dind-2" "dind-3" "iarnet-1" "iarnet-2" "iarnet-3")

for target in "${TARGETS[@]}"; do
    host=$(get_target_host "$target")
    if [ -z "$host" ]; then
        echo -e "${RED}未知目标 ${target}，跳过${NC}"
        continue
    fi

    echo -e "${YELLOW}清理 ${target} (${host}) 中的容器...${NC}"

    ids=$(docker -H "$host" ps -aq || true)
    if [ -z "$ids" ]; then
        echo -e "${YELLOW}${target} 无需清理${NC}"
        continue
    fi

    echo "$ids" | xargs -r docker -H "$host" rm -f
    echo -e "${GREEN}${target} 容器已全部删除${NC}"
done

echo -e "${GREEN}所有目标清理完成${NC}"

