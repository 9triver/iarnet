#!/usr/bin/env bash
set -e

# ===============================
# 使用说明
# ===============================
# ./build.sh [环境] [镜像标签]
#  - 环境：可选值 dev | prod | test (默认 prod)
#  - 镜像标签：镜像版本号 (默认 latest)
#
# 示例：
#   ./build.sh dev v1.0.0
#   ./build.sh prod latest
# ===============================

# 获取参数
ENVIRONMENT=${1:-python_3.11}
TAG=${2:-latest}

# 项目名称（镜像前缀，可自定义）
IMAGE_NAME="iarnet/runner"

# 选择镜像标签
FULL_TAG="${IMAGE_NAME}:${ENVIRONMENT}-${TAG}"

echo "============================================"
echo "🚀 开始构建 Docker 镜像"
echo "👉 环境:   ${ENVIRONMENT}"
echo "👉 镜像:   ${FULL_TAG}"
echo "============================================"

# 切换到项目根目录进行构建（因为需要访问跨目录依赖）
PROJECT_ROOT="../../.."
cd "$PROJECT_ROOT"

# 构建镜像（使用新的 runner/python Dockerfile）
if [ "$ENVIRONMENT" = "python_3.11" ]; then
  docker build \
    --target python_3.11 \
    -t ${FULL_TAG} \
    -f containers/runner/python/Dockerfile .
else
  docker build \
    --build-arg BUILD_ENV=${ENVIRONMENT} \
    -t ${FULL_TAG} \
    -f containers/runner/python/Dockerfile .
fi

echo "✅ 构建完成: ${FULL_TAG}"