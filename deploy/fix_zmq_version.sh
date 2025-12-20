#!/bin/bash
# 修复 ZMQ 版本不一致问题
# 注意：由于本地仓库只有 4.3.5 版本，降级方案不可行
# 建议使用 upgrade_vm_zmq.sh 在虚拟机上升级到与本地相同的版本

echo "=== ZMQ 版本不一致问题 ==="
echo ""
echo "当前情况："
echo "  虚拟机版本：libzmq3-dev 4.3.4-2, libczmq-dev 4.2.1-1"
echo "  本地版本：  libzmq3-dev 4.3.5-1build2, libczmq-dev 4.2.1-2build1"
echo ""
echo "⚠ 本地仓库中没有 4.3.4-2 版本，无法降级"
echo ""
echo "推荐方案：在虚拟机上升级到与本地相同的版本"
echo ""
echo "使用方法："
echo "  ./upgrade_vm_zmq.sh <vm_ip> [vm_user]"
echo ""
echo "例如："
echo "  ./upgrade_vm_zmq.sh 192.168.100.11 ubuntu"
echo ""
echo "或者手动在虚拟机上执行："
echo "  ssh ubuntu@<vm_ip> 'sudo apt-get update && sudo apt-get install -y libzmq3-dev libzmq5 libczmq-dev libczmq4 && sudo ldconfig'"

