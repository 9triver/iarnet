#!/bin/bash

# 构建所有 Docker 镜像的脚本 - 使用多阶段构建
# 使用方法: ./build-all.sh

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}开始构建所有 IARNet Docker 镜像（多阶段构建）...${NC}"

# 切换到项目根目录
PROJECT_ROOT="../../"
cd "$PROJECT_ROOT"

echo -e "${YELLOW}当前构建目录: $(pwd)${NC}"

# 使用统一的多阶段 Dockerfile
DOCKERFILE="iarnet/containers/images/base/python.Dockerfile"

# 1. 构建 Python 基础镜像
echo -e "${YELLOW}1. 构建 Python 基础镜像...${NC}"
docker build --target python-base -f "$DOCKERFILE" -t iarnet/python-base .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Python 基础镜像构建成功!${NC}"
else
    echo -e "${RED}❌ Python 基础镜像构建失败!${NC}"
    exit 1
fi

# 2. 构建 Component 镜像
echo -e "${YELLOW}2. 构建 Component 镜像...${NC}"
docker build --target component -f "$DOCKERFILE" -t iarnet/component .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Component 镜像构建成功!${NC}"
else
    echo -e "${RED}❌ Component 镜像构建失败!${NC}"
    exit 1
fi

# 3. 构建 Runner 镜像
echo -e "${YELLOW}3. 构建 Runner 镜像...${NC}"
docker build --target runner -f "$DOCKERFILE" -t iarnet/runner .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Runner 镜像构建成功!${NC}"
else
    echo -e "${RED}❌ Runner 镜像构建失败!${NC}"
    exit 1
fi

echo -e "${GREEN}🎉 所有镜像构建完成!${NC}"

# 显示构建的镜像
echo -e "${YELLOW}构建的镜像列表:${NC}"
docker images | grep iarnet

echo -e "${YELLOW}镜像大小对比:${NC}"
docker images --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}" | grep -E "(REPOSITORY|iarnet)"