#!/usr/bin/env bash

# Runner Docker 镜像构建脚本
# 使用方法: ./build.sh [环境] [镜像标签]
#
# 示例：
#   ./build.sh python_3.11 v1.0.0
#   ./build.sh python_3.11 latest

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 获取参数
ENVIRONMENT=${1:-python_3.11}
TAG=${2:-latest}

# 项目名称（镜像前缀，可自定义）
IMAGE_NAME="iarnet/runner"

# 选择镜像标签
FULL_TAG="${IMAGE_NAME}:${ENVIRONMENT}-${TAG}"

echo -e "${YELLOW}开始构建 Runner Docker 镜像...${NC}"
echo -e "${YELLOW}环境:   ${ENVIRONMENT}${NC}"
echo -e "${YELLOW}镜像:   ${FULL_TAG}${NC}"

# 切换到项目根目录进行构建（因为需要访问跨目录依赖）
PROJECT_ROOT="../../.."
cd "$PROJECT_ROOT"

echo -e "${YELLOW}开始 Docker 构建...${NC}"

# 构建镜像（使用新的 runner/python Dockerfile）
if [ "$ENVIRONMENT" = "python_3.11" ]; then
  docker build \
    --target python_3.11 \
    -t "${FULL_TAG}" \
    -f containers/runner/python/Dockerfile .
else
  docker build \
    --build-arg BUILD_ENV="${ENVIRONMENT}" \
    -t "${FULL_TAG}" \
    -f containers/runner/python/Dockerfile .
fi

if [ $? -eq 0 ]; then
  echo -e "${GREEN}✔ Docker 镜像构建成功!${NC}"
  echo -e "${GREEN}镜像标签: ${FULL_TAG}${NC}"

  echo -e "${YELLOW}镜像信息:${NC}"
  docker images "${FULL_TAG}"
else
  echo -e "${RED}✘ Docker 镜像构建失败!${NC}"
  exit 1
fi