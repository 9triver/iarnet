#!/bin/bash

# Docker Provider 镜像构建脚本
# 使用方法: ./build.sh [tag_name]

set -e

# 默认镜像标签
DEFAULT_TAG="iarnet/provider:docker"
IMAGE_TAG="${1:-$DEFAULT_TAG}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}开始构建 Docker Provider 镜像（使用 socket 映射方式）...${NC}"

# 检查是否在正确的目录
if [ ! -f "Dockerfile" ]; then
    echo -e "${RED}错误: 在当前目录找不到 Dockerfile${NC}"
    echo "请确保在 providers/docker 目录下运行此脚本"
    exit 1
fi

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 获取项目根目录（向上两级：providers/docker -> providers -> iarnet）
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo -e "${YELLOW}当前构建目录: $SCRIPT_DIR${NC}"
echo -e "${YELLOW}项目根目录: $PROJECT_ROOT${NC}"
echo -e "${YELLOW}构建镜像标签: $IMAGE_TAG${NC}"

# 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}错误: Docker daemon 未运行${NC}"
    exit 1
fi

# 构建 Docker 镜像（需要从项目根目录构建，因为 Dockerfile 中有跨目录依赖）
echo -e "${YELLOW}开始 Docker 构建...${NC}"
cd "$PROJECT_ROOT"
docker build -t "$IMAGE_TAG" -f "$SCRIPT_DIR/Dockerfile" .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Docker 镜像构建成功!${NC}"
    echo -e "${GREEN}镜像标签: $IMAGE_TAG${NC}"
    
    # 显示镜像信息
    echo -e "${YELLOW}镜像信息:${NC}"
    docker images "$IMAGE_TAG"
    
    echo -e "${YELLOW}运行示例:${NC}"
    echo "docker run -d --name docker-provider \\"
    echo "  -v /var/run/docker.sock:/var/run/docker.sock \\"
    echo "  -p 50051:50051 \\"
    echo "  $IMAGE_TAG"
    echo ""
    echo -e "${YELLOW}注意:${NC}"
    echo -e "${YELLOW}  - 需要挂载宿主机的 Docker socket: -v /var/run/docker.sock:/var/run/docker.sock${NC}"
    echo -e "${YELLOW}  - docker provider 将使用宿主机的 Docker daemon${NC}"
    echo -e "${YELLOW}  - 不需要 --privileged 标志${NC}"
else
    echo -e "${RED}❌ Docker 镜像构建失败!${NC}"
    exit 1
fi
