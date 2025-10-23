#!/bin/bash

# 构建 Component 镜像的脚本
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTAINERS_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$(dirname "$CONTAINERS_DIR")")"

echo "=== 构建 IARNet Component 镜像 ==="
echo "项目根目录: $PROJECT_ROOT"

# 构建 Python 基础镜像（如果不存在）
if ! docker images | grep -q "iarnet/python-base"; then
    echo "构建 Python 基础镜像..."
    cd "$CONTAINERS_DIR"
    docker build -f images/base/python.Dockerfile -t iarnet/python-base:latest .
fi

# 构建 component 镜像
echo "构建 Component 镜像..."
cd "$PROJECT_ROOT"
docker build -f iarnet/containers/images/component/Dockerfile -t iarnet/component:latest .

echo "=== Component 镜像构建完成 ==="
docker images | grep iarnet/component