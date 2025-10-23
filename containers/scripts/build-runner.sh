#!/bin/bash

# 构建 Runner 镜像的脚本
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTAINERS_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== 构建 IARNet Runner 镜像 ==="

# 构建 runner 镜像
echo "构建 Runner 镜像..."
cd "$CONTAINERS_DIR/images/runner"
docker build -t iarnet/runner:latest .

echo "=== Runner 镜像构建完成 ==="
docker images | grep iarnet/runner