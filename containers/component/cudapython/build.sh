#!/bin/bash

# Python Component Docker 镜像构建脚本
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

echo -e "${YELLOW}开始构建 Python Component Docker 镜像...${NC}"

# 检查是否在正确的目录
if [ ! -f "Dockerfile" ]; then
    echo -e "${RED}错误: 在当前目录找不到 Dockerfile${NC}"
    echo "请确保在 containers/component/python 目录下运行此脚本"
    exit 1
fi

# 检查必要文件
if [ ! -f "main.py" ]; then
    echo -e "${RED}错误: 找不到 main.py${NC}"
    exit 1
fi

if [ ! -d "proto" ]; then
    echo -e "${RED}错误: 找不到 proto 目录${NC}"
    echo "请确保已生成 proto 文件"
    exit 1
fi

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo -e "${YELLOW}当前构建目录: $(pwd)${NC}"
echo -e "${YELLOW}构建镜像标签: $IMAGE_TAG${NC}"

# 检测代理是否可用（可选）
PROXY_HOST="172.17.0.1"
PROXY_PORT="7897"
USE_PROXY=false

# 检查代理是否可用（静默检测，避免错误输出）
if command -v nc >/dev/null 2>&1; then
    if nc -z -w 1 "$PROXY_HOST" "$PROXY_PORT" 2>/dev/null; then
        USE_PROXY=true
        echo -e "${GREEN}检测到可用代理: http://${PROXY_HOST}:${PROXY_PORT}${NC}"
    fi
elif command -v timeout >/dev/null 2>&1; then
    # 使用 timeout 和 bash 内置的 TCP 连接测试
    if timeout 1 bash -c "echo > /dev/tcp/${PROXY_HOST}/${PROXY_PORT}" 2>/dev/null; then
        USE_PROXY=true
        echo -e "${GREEN}检测到可用代理: http://${PROXY_HOST}:${PROXY_PORT}${NC}"
    fi
fi

if [ "$USE_PROXY" = false ]; then
    echo -e "${YELLOW}代理 ${PROXY_HOST}:${PROXY_PORT} 不可用或未检测到，将不使用代理${NC}"
fi

# 构建 Docker 镜像
echo -e "${YELLOW}开始 Docker 构建...${NC}"
if [ "$USE_PROXY" = true ]; then
    docker build --build-arg HTTP_PROXY="http://${PROXY_HOST}:${PROXY_PORT}" \
                 --build-arg HTTPS_PROXY="http://${PROXY_HOST}:${PROXY_PORT}" \
                 --build-arg NO_PROXY="localhost,127.0.0.1" \
                 -t "$IMAGE_TAG" .
else
    # 不使用代理
    docker build -t "$IMAGE_TAG" .
fi

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Docker 镜像构建成功!${NC}"
    echo -e "${GREEN}镜像标签: $IMAGE_TAG${NC}"
    
    # 显示镜像信息
    echo -e "${YELLOW}镜像信息:${NC}"
    docker images "$IMAGE_TAG"
    
    echo -e "${YELLOW}运行示例:${NC}"
    echo "docker run -e ZMQ_ADDR=tcp://localhost:5555 -e STORE_ADDR=localhost:50051 $IMAGE_TAG"
else
    echo -e "${RED}❌ Docker 镜像构建失败!${NC}"
    exit 1
fi

