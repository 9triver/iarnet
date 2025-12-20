#!/bin/bash
#
# 在 K8s 节点上执行依赖安装脚本的便捷脚本
# 使用方法:
#   bash run_k8s_dependencies_install.sh <vm-name> [master|worker]
#
# 示例:
#   bash run_k8s_dependencies_install.sh vm-k8s-cluster-01-master master
#

set -e

VM_NAME=${1:-""}
NODE_TYPE=${2:-"master"}

if [ -z "$VM_NAME" ]; then
    echo "错误: 请指定虚拟机名称"
    echo "使用方法: bash run_k8s_dependencies_install.sh <vm-name> [master|worker]"
    echo "示例: bash run_k8s_dependencies_install.sh vm-k8s-cluster-01-master master"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_SCRIPT="$SCRIPT_DIR/install_k8s_dependencies.sh"

if [ ! -f "$INSTALL_SCRIPT" ]; then
    echo "错误: 找不到安装脚本: $INSTALL_SCRIPT"
    exit 1
fi

# 使用 ssh_vm.py 获取虚拟机信息并执行
echo "正在连接到虚拟机: $VM_NAME"
echo "节点类型: $NODE_TYPE"
echo ""

# 方法1: 使用 ssh_vm.py（如果可用）
if command -v python3 >/dev/null 2>&1 && [ -f "$SCRIPT_DIR/ssh_vm.py" ]; then
    echo "使用方法1: 通过 ssh_vm.py 执行"
    python3 "$SCRIPT_DIR/ssh_vm.py" "$VM_NAME" << EOF
sudo bash -s < "$INSTALL_SCRIPT" $NODE_TYPE
EOF
else
    # 方法2: 直接使用 SSH（需要配置 SSH 密钥）
    echo "使用方法2: 直接使用 SSH"
    echo "请手动执行以下命令:"
    echo ""
    echo "1. 复制脚本到虚拟机:"
    echo "   scp $INSTALL_SCRIPT ubuntu@<VM_IP>:/tmp/install_k8s_dependencies.sh"
    echo ""
    echo "2. SSH 到虚拟机并执行:"
    echo "   ssh ubuntu@<VM_IP>"
    echo "   sudo bash /tmp/install_k8s_dependencies.sh $NODE_TYPE"
    echo ""
    echo "或者使用一行命令:"
    echo "   cat $INSTALL_SCRIPT | ssh ubuntu@<VM_IP> 'sudo bash -s' $NODE_TYPE"
fi

