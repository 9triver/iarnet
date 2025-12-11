#!/bin/bash
# 删除并重新创建单个虚拟机（使用现有脚本）

VM_NAME="$1"

if [ -z "$VM_NAME" ]; then
    echo "用法: $0 <虚拟机名称>"
    echo "示例: $0 vm-iarnet-06"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

echo "=========================================="
echo "删除并重新创建虚拟机: $VM_NAME"
echo "=========================================="

# 步骤1: 删除虚拟机（如果存在）
echo ""
echo "步骤1: 删除虚拟机（如果存在）..."

# 使用Python脚本删除（需要先实现单个删除功能，或直接使用virsh）
if virsh dominfo "$VM_NAME" &>/dev/null; then
    echo "  找到虚拟机，正在删除..."
    
    # 如果正在运行，先关闭
    if virsh dominfo "$VM_NAME" | grep -q "State:.*running"; then
        echo "  关闭虚拟机..."
        virsh destroy "$VM_NAME"
    fi
    
    # 删除虚拟机定义
    echo "  删除虚拟机定义..."
    virsh undefine "$VM_NAME"
    
    # 删除磁盘
    DISK_PATH="/var/lib/libvirt/images/${VM_NAME}.qcow2"
    if [ -f "$DISK_PATH" ]; then
        echo "  删除磁盘: $DISK_PATH"
        sudo rm -f "$DISK_PATH"
    fi
    
    echo "  ✓ 虚拟机删除完成"
else
    echo "  虚拟机不存在，跳过删除步骤"
fi

# 步骤2: 重新创建虚拟机
echo ""
echo "步骤2: 重新创建虚拟机..."
echo "  使用 create_vms.py 脚本创建（会跳过已存在的虚拟机）"

# 确定虚拟机类型
if [[ "$VM_NAME" =~ ^vm-iarnet- ]]; then
    python3 deploy/create_vms.py --type iarnet
elif [[ "$VM_NAME" =~ ^vm-docker- ]]; then
    python3 deploy/create_vms.py --type docker
elif [[ "$VM_NAME" =~ ^vm-k8s-cluster- ]]; then
    python3 deploy/create_vms.py --type k8s
else
    echo "错误: 无法识别虚拟机类型"
    exit 1
fi

echo ""
echo "=========================================="
echo "完成！"
echo "=========================================="
echo ""
echo "验证虚拟机:"
echo "  virsh dominfo $VM_NAME"
echo "  virsh start $VM_NAME"
echo ""
echo "等待几分钟后检查网络:"
echo "  python3 deploy/ssh_vm.py $VM_NAME \"ip addr show\""

