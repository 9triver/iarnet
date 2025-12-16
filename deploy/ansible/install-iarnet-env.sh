#!/bin/bash
# 使用 Ansible 批量安装 iarnet 环境的便捷脚本
# 包括：Go、gRPC、ZeroMQ 等依赖

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

# 检查 inventory 文件是否存在或是否需要更新
INVENTORY_FILE="$ANSIBLE_DIR/inventory.ini"
NEED_REGENERATE=false

if [ ! -f "$INVENTORY_FILE" ]; then
    echo "警告: inventory 文件不存在，正在生成..."
    NEED_REGENERATE=true
elif ! grep -q "^\[iarnet_nodes\]" "$INVENTORY_FILE"; then
    echo "警告: inventory 文件中缺少 iarnet_nodes 组，正在重新生成..."
    NEED_REGENERATE=true
fi

if [ "$NEED_REGENERATE" = true ]; then
    cd "$DEPLOY_DIR"
    if [ -f "generate_ansible_inventory.py" ]; then
        # 生成包含所有类型的 inventory（不指定 --type，生成所有）
        python3 generate_ansible_inventory.py --output "$INVENTORY_FILE"
        echo "✓ inventory 文件已生成"
    else
        echo "错误: 找不到 generate_ansible_inventory.py"
        echo "请手动创建 inventory 文件或运行:"
        echo "  python3 deploy/generate_ansible_inventory.py --output deploy/ansible/inventory.ini"
        exit 1
    fi
    echo ""
fi

# 验证 iarnet_nodes 组是否存在
if ! grep -q "^\[iarnet_nodes\]" "$INVENTORY_FILE"; then
    echo "错误: inventory 文件中仍然缺少 iarnet_nodes 组"
    echo "请检查 generate_ansible_inventory.py 脚本"
    exit 1
fi

# 进入 Ansible 目录
cd "$ANSIBLE_DIR"

# 检查 playbook 是否存在
PLAYBOOK_FILE="playbooks/install-iarnet-deps.yml"
if [ ! -f "$PLAYBOOK_FILE" ]; then
    echo "错误: Playbook 文件不存在: $PLAYBOOK_FILE"
    exit 1
fi

# 运行 playbook
echo "开始批量安装 iarnet 环境..."
echo "=================================="
echo ""
echo "将安装以下组件:"
echo "  - Go 语言环境"
echo "  - Git"
echo "  - gRPC 相关库 (libprotobuf-dev, protobuf-compiler)"
echo "  - ZeroMQ 库 (libzmq3-dev)"
echo "  - 其他运行时依赖"
echo ""
echo "注意: 需要输入虚拟机的 sudo 密码"
echo "=================================="
echo ""

ansible-playbook -i "$INVENTORY_FILE" "$PLAYBOOK_FILE" "$@"

echo ""
echo "=================================="
echo "安装完成！"
echo ""
echo "验证安装:"
echo "  ansible iarnet_nodes -m shell -a 'go version'"
echo "  ansible iarnet_nodes -m shell -a 'git --version'"
echo "  ansible iarnet_nodes -m shell -a 'protoc --version'"
echo "  ansible iarnet_nodes -m shell -a 'pkg-config --modversion libzmq'"
echo ""
echo "或使用 SSH 脚本验证:"
echo "  python3 deploy/ssh_vm.py vm-iarnet-01 \"go version\""
echo "  python3 deploy/ssh_vm.py vm-iarnet-01 \"git --version\""

