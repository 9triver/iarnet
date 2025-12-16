#!/bin/bash
# 修复虚拟机的网络接口配置
# 用于修复已创建的虚拟机网络接口未正确连接到libvirt网络的问题

VM_NAME="$1"

if [ -z "$VM_NAME" ]; then
    echo "用法: $0 <虚拟机名称>"
    echo "示例: $0 vm-iarnet-01"
    exit 1
fi

echo "修复虚拟机网络接口: $VM_NAME"
echo "=========================================="

# 检查虚拟机是否存在
if ! sudo virsh dominfo "$VM_NAME" &>/dev/null; then
    echo "错误: 虚拟机 $VM_NAME 不存在"
    exit 1
fi

# 检查网络是否存在
NETWORK_NAME="iarnet-network"
if ! sudo virsh net-info "$NETWORK_NAME" &>/dev/null; then
    echo "错误: libvirt网络 $NETWORK_NAME 不存在"
    echo "请先创建网络: sudo ./create_network.sh"
    exit 1
fi

# 检查网络是否激活
if ! sudo virsh net-info "$NETWORK_NAME" | grep -q "Active:.*yes"; then
    echo "启动网络 $NETWORK_NAME..."
    sudo virsh net-start "$NETWORK_NAME"
fi

# 获取虚拟机XML
VM_XML=$(sudo virsh dumpxml "$VM_NAME")

# 检查是否已有网络接口
if echo "$VM_XML" | grep -q "<interface type='network'>"; then
    echo "虚拟机已有网络接口配置"
    echo ""
    echo "检查网络接口状态:"
    sudo virsh domiflist "$VM_NAME"
    echo ""
    echo "如果接口未连接，尝试以下方法:"
    echo "1. 重启虚拟机: sudo virsh reboot $VM_NAME"
    echo "2. 或者重新定义虚拟机网络接口"
else
    echo "虚拟机缺少网络接口配置，正在添加..."
    
    # 创建临时XML文件添加网络接口
    TMP_XML=$(mktemp)
    sudo virsh dumpxml "$VM_NAME" > "$TMP_XML"
    
    # 在devices部分添加网络接口（在最后一个disk之后）
    # 使用sed在</devices>之前插入网络配置
    sed -i '/<\/devices>/i\
    <interface type="network">\
      <source network="'"$NETWORK_NAME"'"/>\
      <model type="virtio"/>\
    </interface>' "$TMP_XML"
    
    # 重新定义虚拟机
    echo "重新定义虚拟机..."
    sudo virsh define "$TMP_XML"
    
    # 如果虚拟机正在运行，需要重启
    if sudo virsh dominfo "$VM_NAME" | grep -q "State:.*running"; then
        echo "虚拟机正在运行，需要重启以应用网络配置"
        echo "重启虚拟机: sudo virsh reboot $VM_NAME"
    else
        echo "虚拟机未运行，可以直接启动: sudo virsh start $VM_NAME"
    fi
    
    rm -f "$TMP_XML"
fi

echo ""
echo "完成！"
echo ""
echo "验证步骤:"
echo "1. 检查网络接口: sudo virsh domiflist $VM_NAME"
echo "2. 检查虚拟机IP: sudo virsh domifaddr $VM_NAME"
echo "3. 如果IP未分配，等待cloud-init完成或通过控制台检查"

