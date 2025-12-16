#!/bin/bash
# 创建 libvirt 网络配置脚本

NETWORK_NAME="iarnet-network"
NETWORK_BRIDGE="virbr100"
NETWORK_IP="192.168.100.1"
NETWORK_NETMASK="255.255.255.0"
NETWORK_DHCP_START="192.168.100.2"
NETWORK_DHCP_END="192.168.100.254"

# 检查网络是否已存在
if virsh net-info "$NETWORK_NAME" &>/dev/null; then
    echo "网络 $NETWORK_NAME 已存在"
    virsh net-info "$NETWORK_NAME"
    exit 0
fi

# 创建网络XML配置
cat > /tmp/${NETWORK_NAME}.xml <<EOF
<network>
  <name>${NETWORK_NAME}</name>
  <bridge name="${NETWORK_BRIDGE}"/>
  <forward mode="nat"/>
  <ip address="${NETWORK_IP}" netmask="${NETWORK_NETMASK}">
    <dhcp>
      <range start="${NETWORK_DHCP_START}" end="${NETWORK_DHCP_END}"/>
    </dhcp>
  </ip>
</network>
EOF

# 定义并启动网络
echo "创建网络 $NETWORK_NAME..."
virsh net-define /tmp/${NETWORK_NAME}.xml
virsh net-start "$NETWORK_NAME"
virsh net-autostart "$NETWORK_NAME"

echo "网络 $NETWORK_NAME 创建成功！"
virsh net-info "$NETWORK_NAME"

# 清理临时文件
rm -f /tmp/${NETWORK_NAME}.xml

