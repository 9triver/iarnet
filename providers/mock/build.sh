#!/bin/bash

# Mock Provider 镜像构建脚本
# 使用方法: ./build.sh [tag_name]
# 注意：需要在 iarnet 项目根目录下运行此脚本

set -e

# 默认镜像标签
DEFAULT_TAG="iarnet-mock-provider:latest"
IMAGE_TAG="${1:-$DEFAULT_TAG}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}开始构建 Mock Provider 镜像...${NC}"

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# 切换到项目根目录
cd "$PROJECT_ROOT"

echo -e "${YELLOW}项目根目录: $PROJECT_ROOT${NC}"
echo -e "${YELLOW}构建镜像标签: $IMAGE_TAG${NC}"

# 检查 Dockerfile 是否存在
if [ ! -f "$SCRIPT_DIR/Dockerfile" ]; then
    echo -e "${RED}错误: 找不到 Dockerfile${NC}"
    echo "路径: $SCRIPT_DIR/Dockerfile"
    exit 1
fi

# 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}错误: Docker daemon 未运行${NC}"
    exit 1
fi

# 构建 Docker 镜像
# 使用项目根目录作为构建上下文，因为需要访问整个项目代码
echo -e "${YELLOW}开始 Docker 构建...${NC}"
echo -e "${YELLOW}构建上下文: $PROJECT_ROOT${NC}"
echo -e "${YELLOW}Dockerfile: $SCRIPT_DIR/Dockerfile${NC}"

docker build \
    -f "$SCRIPT_DIR/Dockerfile" \
    -t "$IMAGE_TAG" \
    "$PROJECT_ROOT"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Docker 镜像构建成功!${NC}"
    echo -e "${GREEN}镜像标签: $IMAGE_TAG${NC}"
    
    # 显示镜像信息
    echo -e "${YELLOW}镜像信息:${NC}"
    docker images "$IMAGE_TAG"
    
    echo ""
    echo -e "${YELLOW}运行示例:${NC}"
    echo "docker run -d --name mock-provider \\"
    echo "  -v \$(pwd)/providers/mock/config.yaml:/app/config.yaml \\"
    echo "  -p 50051:50051 \\"
    echo "  $IMAGE_TAG"
    echo ""
    echo -e "${YELLOW}注意:${NC}"
    echo -e "${YELLOW}  - gRPC 服务端口: 50051${NC}"
    echo -e "${YELLOW}  - 可以通过挂载自定义配置文件覆盖默认配置${NC}"
    echo -e "${YELLOW}  - Mock provider 用于实验和测试，模拟资源提供者行为${NC}"
else
    echo -e "${RED}❌ Docker 镜像构建失败!${NC}"
    exit 1
fi

