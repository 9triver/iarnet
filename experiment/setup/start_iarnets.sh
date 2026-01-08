#!/bin/bash

# 启动所有 Iarnet 服务脚本
# 使用方法: ./start_iarnets.sh

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
echo -e "${BLUE}  启动所有 Iarnet 服务${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检查 docker-compose.yaml 是否存在
if [ ! -f "docker-compose.yaml" ]; then
    echo -e "${RED}错误: 找不到 docker-compose.yaml${NC}"
    exit 1
fi

# 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}错误: Docker daemon 未运行${NC}"
    exit 1
fi

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

echo -e "${YELLOW}准备启动 ${#IARNETS[@]} 个 Iarnet 服务...${NC}"
echo ""

# 启动所有 iarnet
docker-compose up -d "${IARNETS[@]}"

if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✅ 所有 Iarnet 服务启动成功!${NC}"
    echo ""
    echo -e "${YELLOW}Iarnet 服务状态:${NC}"
    docker-compose ps "${IARNETS[@]}"
    echo ""
    echo -e "${YELLOW}前端访问地址:${NC}"
    echo "  iarnet-1: http://localhost:3001"
    echo "  iarnet-2: http://localhost:3002"
    echo "  iarnet-3: http://localhost:3003"
    echo "  iarnet-4: http://localhost:3004"
    echo "  iarnet-5: http://localhost:3005"
    echo "  iarnet-6: http://localhost:3006"
    echo "  iarnet-7: http://localhost:3007"
    echo ""
    echo -e "${YELLOW}查看日志:${NC}"
    echo "  docker-compose logs -f iarnet-1"
    echo ""
    echo -e "${YELLOW}停止所有 Iarnet:${NC}"
    echo "  docker-compose stop ${IARNETS[@]}"
else
    echo -e "${RED}❌ Iarnet 服务启动失败!${NC}"
    exit 1
fi

