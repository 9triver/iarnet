#!/bin/bash
# 删除并重新创建单个虚拟机

VM_NAME="$1"

if [ -z "$VM_NAME" ]; then
    echo "用法: $0 <虚拟机名称>"
    echo "示例: $0 vm-iarnet-06"
    exit 1
fi

echo "=========================================="
echo "删除并重新创建虚拟机: $VM_NAME"
echo "=========================================="

# 检查虚拟机是否存在
if ! sudo virsh dominfo "$VM_NAME" &>/dev/null; then
    echo "警告: 虚拟机 $VM_NAME 不存在，将直接创建"
else
    echo ""
    echo "步骤1: 删除虚拟机..."
    
    # 如果正在运行，先关闭
    if sudo virsh dominfo "$VM_NAME" | grep -q "State:.*running"; then
        echo "  关闭虚拟机..."
        sudo virsh destroy "$VM_NAME"
    fi
    
    # 删除虚拟机定义
    echo "  删除虚拟机定义..."
    sudo virsh undefine "$VM_NAME"
    
    # 删除磁盘
    DISK_PATH="/var/lib/libvirt/images/${VM_NAME}.qcow2"
    if [ -f "$DISK_PATH" ]; then
        echo "  删除磁盘: $DISK_PATH"
        sudo rm -f "$DISK_PATH"
    fi
    
    echo "  ✓ 虚拟机删除完成"
fi

# 确定虚拟机类型和节点ID
if [[ "$VM_NAME" =~ ^vm-iarnet-([0-9]+)$ ]]; then
    NODE_ID=$((10#${BASH_REMATCH[1]} - 1))  # 转换为0-based索引
    VM_TYPE="iarnet"
    echo ""
    echo "步骤2: 重新创建虚拟机..."
    echo "  节点ID: $NODE_ID"
    echo "  类型: $VM_TYPE"
    
    # 使用Python脚本重新创建
    cd "$(dirname "$0")/.."
    python3 deploy/create_vms.py --type iarnet 2>&1 | grep -A 20 "创建.*$VM_NAME\|创建虚拟机: $VM_NAME" || {
        echo "  注意: 脚本会创建所有iarnet节点，已存在的会跳过"
        echo "  如果 $VM_NAME 未创建，请检查配置"
    }
    
elif [[ "$VM_NAME" =~ ^vm-docker-([0-9]+)$ ]]; then
    NODE_ID=$((10#${BASH_REMATCH[1]} - 1))
    VM_TYPE="docker"
    echo ""
    echo "步骤2: 重新创建虚拟机..."
    echo "  节点ID: $NODE_ID"
    echo "  类型: $VM_TYPE"
    
    cd "$(dirname "$0")/.."
    python3 deploy/create_vms.py --type docker 2>&1 | grep -A 20 "创建.*$VM_NAME\|创建虚拟机: $VM_NAME" || {
        echo "  注意: 脚本会创建所有docker节点，已存在的会跳过"
    }
    
elif [[ "$VM_NAME" =~ ^vm-k8s-cluster-([0-9]+)-(master|worker-[0-9]+)$ ]]; then
    CLUSTER_ID=${BASH_REMATCH[1]}
    VM_TYPE="k8s"
    echo ""
    echo "步骤2: 重新创建虚拟机..."
    echo "  集群ID: $CLUSTER_ID"
    echo "  类型: $VM_TYPE"
    
    cd "$(dirname "$0")/.."
    python3 deploy/create_vms.py --type k8s 2>&1 | grep -A 20 "创建.*$VM_NAME\|创建虚拟机: $VM_NAME" || {
        echo "  注意: 脚本会创建所有k8s节点，已存在的会跳过"
    }
else
    echo "错误: 无法识别虚拟机类型: $VM_NAME"
    echo "支持的格式:"
    echo "  vm-iarnet-XX"
    echo "  vm-docker-XX"
    echo "  vm-k8s-cluster-XX-master"
    echo "  vm-k8s-cluster-XX-worker-X"
    exit 1
fi

echo ""
echo "=========================================="
echo "完成！"
echo "=========================================="
echo ""
echo "验证虚拟机:"
echo "  sudo virsh dominfo $VM_NAME"
echo "  sudo virsh start $VM_NAME"
echo ""
echo "等待几分钟后检查网络:"
echo "  python3 deploy/ssh_vm.py $VM_NAME \"ip addr show\""

