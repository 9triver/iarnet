#!/bin/bash
# 完整的 iarnet 部署脚本
# 包括：生成配置 -> 安装依赖 -> 本地构建 -> 上传部署 -> 启动服务

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

NODES="${1:-0-10}"
INSTALL_DEPS="${2:-true}"
BUILD="${3:-true}"
RESTART="${4:-true}"

echo "=========================================="
echo "完整部署 iarnet 到节点: $NODES"
echo "=========================================="

# 步骤1: 生成配置文件
echo ""
echo "步骤1: 生成配置文件..."
python3 deploy/generate_iarnet_configs.py --nodes "$NODES"

if [ $? -ne 0 ]; then
    echo "错误: 配置文件生成失败"
    exit 1
fi

# 步骤2: 安装依赖（使用 Ansible）
if [ "$INSTALL_DEPS" = "true" ] || [ "$INSTALL_DEPS" = "yes" ]; then
    echo ""
    echo "步骤2: 使用 Ansible 安装依赖..."
    echo "注意: 需要输入 sudo 密码"
    python3 deploy/deploy_iarnet.py --nodes "$NODES" --install-deps
fi

# 步骤3: 构建并部署
echo ""
echo "步骤3: 构建并部署..."
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
echo ""
echo "验证部署:"
echo "  python3 deploy/ssh_vm.py vm-iarnet-01 \"ps aux | grep iarnet\""
echo "  python3 deploy/ssh_vm.py vm-iarnet-01 \"tail -20 ~/iarnet/iarnet.log\""

