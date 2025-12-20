#!/bin/bash
# 在虚拟机上升级 ZMQ 库版本到与本地相同
# 使用方法：./upgrade_vm_zmq.sh <vm_ip> [vm_user]

set -e

VM_IP="${1:-192.168.100.11}"
VM_USER="${2:-ubuntu}"

echo "=== 在虚拟机上升级 ZMQ 库版本 ==="
echo ""
echo "目标版本（与本地一致）："
echo "  libzmq3-dev: 4.3.5-1build2"
echo "  libczmq-dev: 4.2.1-2build1"
echo ""
echo "虚拟机: ${VM_USER}@${VM_IP}"
echo ""

read -p "是否继续？(y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "已取消"
    exit 0
fi

echo ""
echo "连接到虚拟机并升级 ZMQ 库..."

ssh "${VM_USER}@${VM_IP}" << 'EOF'
    echo "当前版本："
    dpkg -l | grep -E "(libzmq3-dev|libczmq-dev)" | grep "^ii" || echo "未找到"
    echo ""
    
    echo "更新包列表..."
    sudo apt-get update
    
    echo "升级 ZMQ 库..."
    sudo apt-get install -y libzmq3-dev libzmq5 libczmq-dev libczmq4
    
    echo ""
    echo "升级后版本："
    dpkg -l | grep -E "(libzmq3-dev|libczmq-dev|libzmq5|libczmq4)" | grep "^ii"
    
    echo ""
    echo "更新库缓存..."
    sudo ldconfig
    
    echo ""
    echo "✓ 升级完成！"
EOF

echo ""
echo "✓ 虚拟机 ZMQ 库已升级"
echo ""
echo "建议："
echo "  1. 重新编译本地二进制文件：python3 deploy/deploy_iarnet.py --build"
echo "  2. 重新部署到虚拟机：python3 deploy/deploy_iarnet.py --nodes <node_id> --build --restart"

