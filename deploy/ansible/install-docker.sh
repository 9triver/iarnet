#!/bin/bash
# 使用 Ansible 批量安装 Docker 引擎的便捷脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(dirname "$SCRIPT_DIR")"
ANSIBLE_DIR="$SCRIPT_DIR"

# 检查 Ansible 是否安装
if ! command -v ansible-playbook &> /dev/null; then
    echo "错误: Ansible 未安装"
    echo ""
    echo "安装方法:"
    echo "  Ubuntu/Debian:"
    echo "    sudo apt-get update"
    echo "    sudo apt-get install -y ansible"
    echo ""
    echo "  或使用 pip:"
    echo "    pip3 install ansible"
    exit 1
fi

# 检查 inventory 文件是否存在
INVENTORY_FILE="$ANSIBLE_DIR/inventory.ini"
if [ ! -f "$INVENTORY_FILE" ]; then
    echo "警告: inventory 文件不存在，正在生成..."
    cd "$DEPLOY_DIR"
    python3 generate_ansible_inventory.py --output "$INVENTORY_FILE" --type docker
    echo ""
fi

# 进入 Ansible 目录
cd "$ANSIBLE_DIR"

# 运行 playbook
echo "开始批量安装 Docker 引擎..."
echo "=================================="
ansible-playbook playbooks/install-docker.yml "$@"

echo ""
echo "=================================="
echo "安装完成！"
echo ""
echo "验证安装:"
echo "  ansible docker -m shell -a 'docker --version'"
echo "  ansible docker -m shell -a 'docker ps'"

