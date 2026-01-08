#!/bin/bash

# 启动所有 Provider 服务脚本
# 使用方法: ./start_providers.sh

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
echo -e "${BLUE}  启动所有 Provider 服务${NC}"
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

# 定义所有 provider 服务名称
PROVIDERS=(
    "provider-i1-p1"
    "provider-i1-p2"
    "provider-i1-p3"
    "provider-i2-p1"
    "provider-i2-p2"
    "provider-i2-p3"
    "provider-i3-p1"
    "provider-i3-p2"
    "provider-i3-p3"
    "provider-i4-p1"
    "provider-i4-p2"
    "provider-i4-p3"
    "provider-i5-p1"
    "provider-i5-p2"
    "provider-i5-p3"
    "provider-i6-p1"
    "provider-i6-p2"
    "provider-i6-p3"
    "provider-i7-p1"
    "provider-i7-p2"
    "provider-i7-p3"
    "provider-i8-p1"
    "provider-i8-p2"
    "provider-i8-p3"
)

echo -e "${YELLOW}准备启动 ${#PROVIDERS[@]} 个 Provider 服务...${NC}"
echo ""

# 启动所有 provider
docker-compose up -d "${PROVIDERS[@]}"

if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✅ 所有 Provider 服务启动成功!${NC}"
    echo ""
    echo -e "${YELLOW}Provider 服务状态:${NC}"
    docker-compose ps "${PROVIDERS[@]}"
    echo ""
    echo -e "${YELLOW}查看日志:${NC}"
    echo "  docker-compose logs -f provider-i1-p1"
    echo ""
    echo -e "${YELLOW}停止所有 Provider:${NC}"
    echo "  docker-compose stop ${PROVIDERS[@]}"
else
    echo -e "${RED}❌ Provider 服务启动失败!${NC}"
    exit 1
fi

