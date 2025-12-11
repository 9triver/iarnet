#!/bin/bash
# 一键部署 iarnet 到多个节点
# 自动生成配置文件并部署到指定节点

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

NODES="${1:-0-10}"
BUILD="${2:-false}"
RESTART="${3:-true}"

echo "=========================================="
echo "部署 iarnet 到节点: $NODES"
echo "=========================================="

# 步骤1: 生成配置文件
echo ""
echo "步骤1: 生成配置文件..."
python3 deploy/generate_iarnet_configs.py --nodes "$NODES"

if [ $? -ne 0 ]; then
    echo "错误: 配置文件生成失败"
    exit 1
fi

# 步骤2: 部署到节点
echo ""
echo "步骤2: 部署到节点..."
DEPLOY_ARGS="--nodes $NODES"

if [ "$BUILD" = "true" ] || [ "$BUILD" = "yes" ]; then
    DEPLOY_ARGS="$DEPLOY_ARGS --build"
fi

if [ "$RESTART" = "true" ] || [ "$RESTART" = "yes" ]; then
    DEPLOY_ARGS="$DEPLOY_ARGS --restart"
fi

python3 deploy/deploy_iarnet.py $DEPLOY_ARGS

echo ""
echo "=========================================="
echo "部署完成！"
echo "=========================================="

