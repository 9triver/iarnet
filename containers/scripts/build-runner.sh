#!/bin/bash

# 构建 Runner 镜像的脚本 - 使用多阶段构建
# 使用方法: ./build-runner.sh

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}构建 IARNet Runner 镜像（多阶段构建）...${NC}"

# 切换到项目根目录
PROJECT_ROOT="../../"
cd "$PROJECT_ROOT"

echo -e "${YELLOW}当前构建目录: $(pwd)${NC}"

# 使用统一的多阶段 Dockerfile
DOCKERFILE="iarnet/containers/images/base/python.Dockerfile"

# 构建 Runner 镜像（会自动构建依赖的阶段）
echo -e "${YELLOW}构建 Runner 镜像...${NC}"
docker build --target runner -f "$DOCKERFILE" -t iarnet/runner .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Runner 镜像构建成功!${NC}"
    echo -e "${YELLOW}镜像信息:${NC}"
    docker images | grep iarnet/runner
    
    echo -e "${YELLOW}镜像详细信息:${NC}"
    docker inspect iarnet/runner --format='{{.Config.Cmd}}'
else
    echo -e "${RED}❌ Runner 镜像构建失败!${NC}"
    exit 1
fi