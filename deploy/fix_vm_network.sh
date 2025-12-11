#!/bin/bash
# 修复虚拟机的网络配置问题
# 用于修复已创建的虚拟机中cloud-init网络配置错误

VM_NAME="$1"

if [ -z "$VM_NAME" ]; then
    echo "用法: $0 <虚拟机名称>"
    echo "示例: $0 vm-iarnet-01"
    exit 1
fi

echo "修复虚拟机网络配置: $VM_NAME"
echo "=========================================="

# 检查虚拟机是否存在
if ! virsh dominfo "$VM_NAME" &>/dev/null; then
    echo "错误: 虚拟机 $VM_NAME 不存在"
    exit 1
fi

# 检查虚拟机是否运行
if virsh dominfo "$VM_NAME" | grep -q "State:.*running"; then
    echo "虚拟机正在运行，可以通过SSH连接后手动修复"
    echo ""
    echo "或者停止虚拟机后重新创建cloud-init ISO:"
    echo "  virsh shutdown $VM_NAME"
    echo "  # 然后重新运行 create_vms.py（会跳过已存在的虚拟机）"
    exit 0
fi

echo "注意: 此脚本需要虚拟机支持cloud-init重新初始化"
echo "建议: 删除虚拟机后重新创建，或通过SSH手动修复网络配置"
echo ""
echo "手动修复步骤:"
echo "1. 启动虚拟机: virsh start $VM_NAME"
echo "2. 等待启动后，通过控制台连接: virsh console $VM_NAME"
echo "3. 登录后检查网络配置:"
echo "   sudo cat /etc/netplan/50-cloud-init.yaml"
echo "4. 如果配置错误，编辑文件:"
echo "   sudo nano /etc/netplan/50-cloud-init.yaml"
echo "5. 应用配置: sudo netplan apply"
echo ""
echo "或者删除并重新创建虚拟机:"
echo "  virsh undefine $VM_NAME"
echo "  sudo rm /var/lib/libvirt/images/${VM_NAME}.qcow2"
echo "  python3 create_vms.py --type <type>"

