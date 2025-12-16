#!/bin/bash
# 检查虚拟机网络状态诊断脚本

VM_NAME="$1"

if [ -z "$VM_NAME" ]; then
    echo "用法: $0 <虚拟机名称>"
    echo "示例: $0 vm-iarnet-01"
    exit 1
fi

echo "=========================================="
echo "检查虚拟机网络状态: $VM_NAME"
echo "=========================================="

# 检查虚拟机是否存在
if ! virsh dominfo "$VM_NAME" &>/dev/null; then
    echo "错误: 虚拟机 $VM_NAME 不存在"
    exit 1
fi

# 检查虚拟机状态
echo ""
echo "1. 虚拟机状态:"
sudo virsh dominfo "$VM_NAME" | grep -E "State|CPU|Memory"

# 检查虚拟机是否运行
if ! sudo virsh dominfo "$VM_NAME" | grep -q "State:.*running"; then
    echo ""
    echo "警告: 虚拟机未运行，请先启动:"
    echo "  sudo virsh start $VM_NAME"
    exit 1
fi

# 获取虚拟机IP地址（从配置文件或virsh）
echo ""
echo "2. 检查libvirt网络接口:"
sudo virsh domifaddr "$VM_NAME" 2>/dev/null || echo "  无法获取网络接口信息（可能接口未连接或未分配IP）"

# 检查网络接口列表
echo ""
echo "3. 检查虚拟机网络接口列表:"
sudo virsh domiflist "$VM_NAME" || echo "  无法获取网络接口列表"

# 检查网络配置
echo ""
echo "4. 检查libvirt网络状态:"
NETWORK=$(sudo virsh domiflist "$VM_NAME" 2>/dev/null | grep -oP 'network \K\S+' | head -1)
if [ -n "$NETWORK" ]; then
    echo "  网络名称: $NETWORK"
    sudo virsh net-info "$NETWORK" 2>/dev/null | grep -E "Name|UUID|Active|Persistent|Bridge" || echo "  网络信息获取失败"
    
    # 检查网络是否激活
    if sudo virsh net-info "$NETWORK" 2>/dev/null | grep -q "Active:.*no"; then
        echo "  警告: 网络未激活！"
        echo "  启动网络: sudo virsh net-start $NETWORK"
    fi
else
    echo "  未找到网络接口 - 虚拟机可能没有连接到libvirt网络"
    echo "  修复方法: sudo ./fix_vm_network_interface.sh $VM_NAME"
fi

# 检查网络桥接
echo ""
echo "5. 检查网络桥接:"
BRIDGE=$(sudo virsh net-info "$NETWORK" 2>/dev/null | grep "Bridge:" | awk '{print $2}')
if [ -n "$BRIDGE" ]; then
    echo "  桥接名称: $BRIDGE"
    ip addr show "$BRIDGE" 2>/dev/null | grep -E "inet|state" || echo "    桥接不存在或未配置"
else
    echo "  无法获取桥接信息"
fi

# 尝试通过控制台检查虚拟机内部网络
echo ""
echo "6. 虚拟机内部网络状态（需要通过控制台查看）:"
echo "  连接控制台: sudo virsh console $VM_NAME"
echo "  登录后执行以下命令检查网络:"
echo "    ip addr show"
echo "    ip route show"
echo "    ping -c 2 192.168.100.1"
echo "    systemctl status systemd-networkd"
echo "    systemctl status networking"
echo "    cat /etc/netplan/*.yaml"

# 检查防火墙
echo ""
echo "7. 检查主机防火墙:"
if command -v ufw &>/dev/null; then
    echo "  UFW状态:"
    sudo ufw status | head -5
fi

if command -v firewall-cmd &>/dev/null; then
    echo "  firewalld状态:"
    sudo firewall-cmd --list-all-zones | head -10
fi

# 提供诊断建议
echo ""
echo "=========================================="
echo "诊断建议:"
echo "=========================================="
echo "1. 如果网络接口未连接，修复网络接口:"
echo "   sudo ./fix_vm_network_interface.sh $VM_NAME"
echo ""
echo "2. 如果网络未激活，启动网络:"
echo "   sudo virsh net-start iarnet-network"
echo ""
echo "3. 检查虚拟机是否获得IP地址:"
echo "   sudo virsh console $VM_NAME"
echo "   # 登录后执行: ip addr show"
echo ""
echo "4. 检查网络配置是否正确:"
echo "   sudo virsh console $VM_NAME"
echo "   # 登录后执行: cat /etc/netplan/*.yaml"
echo ""
echo "5. 如果IP地址未配置，手动应用网络配置:"
echo "   sudo virsh console $VM_NAME"
echo "   # 登录后执行: sudo netplan apply"
echo ""
echo "6. 检查cloud-init是否完成:"
echo "   sudo virsh console $VM_NAME"
echo "   # 登录后执行: cloud-init status"
echo ""
echo "7. 如果网络配置有问题，可能需要重新创建虚拟机:"
echo "   sudo virsh undefine $VM_NAME"
echo "   sudo rm /var/lib/libvirt/images/${VM_NAME}.qcow2"
echo "   sudo python3 create_vms.py"

