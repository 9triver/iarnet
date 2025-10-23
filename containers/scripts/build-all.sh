#!/bin/bash

# 构建所有容器镜像的脚本
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTAINERS_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$(dirname "$CONTAINERS_DIR")")"

echo "=== 构建 IARNet 容器镜像 ==="
echo "项目根目录: $PROJECT_ROOT"
echo "容器目录: $CONTAINERS_DIR"

# 构建基础镜像
echo "构建 Python 基础镜像..."
cd "$CONTAINERS_DIR"
docker build -f images/base/python.Dockerfile -t iarnet/python-base:latest .

# 构建 component 镜像
echo "构建 Component 镜像..."
cd "$PROJECT_ROOT"
docker build -f iarnet/containers/images/component/Dockerfile -t iarnet/component:latest .

# 构建 runner 镜像
echo "构建 Runner 镜像..."
cd "$CONTAINERS_DIR/images/runner"
docker build -t iarnet/runner:latest .

echo "=== 所有镜像构建完成 ==="
docker images | grep iarnet