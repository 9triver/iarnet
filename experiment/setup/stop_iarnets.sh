#!/bin/bash

# 停止所有 Iarnet 服务脚本
# 使用方法: ./stop_iarnets.sh

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  停止所有 Iarnet 服务${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 定义所有 iarnet 服务名称
IARNETS=(
    "iarnet-1"
    "iarnet-2"
    "iarnet-3"
    "iarnet-4"
    "iarnet-5"
    "iarnet-6"
    "iarnet-7"
)

# 停止所有 iarnet
docker-compose stop "${IARNETS[@]}"

if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✅ 所有 Iarnet 服务已停止!${NC}"
else
    echo -e "${RED}❌ Iarnet 服务停止失败!${NC}"
    exit 1
fi

