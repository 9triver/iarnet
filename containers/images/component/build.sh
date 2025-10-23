#!/bin/bash

# Component Docker 镜像构建脚本
# 使用方法: ./build.sh [tag_name]

set -e

# 默认镜像标签
DEFAULT_TAG="iarnet/component:python_3.11-latest"
IMAGE_TAG="${1:-$DEFAULT_TAG}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}开始构建 Component Docker 镜像...${NC}"

# 检查是否在正确的目录
if [ ! -f "Dockerfile" ]; then
    echo -e "${RED}错误: 在当前目录找不到 Dockerfile${NC}"
    echo "请确保在 component 目录下运行此脚本"
    exit 1
fi

# 检查是否在项目根目录有必要的依赖
if [ ! -d "../../../ignis" ]; then
    echo -e "${RED}错误: 找不到 ignis 依赖目录${NC}"
    echo "请确保项目结构完整"
    exit 1
fi

# 切换到项目根目录进行构建（因为需要访问跨目录依赖）
PROJECT_ROOT="../../../"
cd "$PROJECT_ROOT"

echo -e "${YELLOW}当前构建目录: $(pwd)${NC}"
echo -e "${YELLOW}构建镜像标签: $IMAGE_TAG${NC}"

# 构建 Docker 镜像
echo -e "${YELLOW}开始 Docker 构建...${NC}"
docker build -f containers/images/component/Dockerfile -t "$IMAGE_TAG" .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Docker 镜像构建成功!${NC}"
    echo -e "${GREEN}镜像标签: $IMAGE_TAG${NC}"
    
    # 显示镜像信息
    echo -e "${YELLOW}镜像信息:${NC}"
    docker images "$IMAGE_TAG"
    
    echo -e "${YELLOW}运行示例:${NC}"
    echo "docker run -e APP_ID=test -e IGNIG_ADDR=localhost:8080 -e FUNC_NAME=test $IMAGE_TAG"
else
    echo -e "${RED}❌ Docker 镜像构建失败!${NC}"
    exit 1
fi