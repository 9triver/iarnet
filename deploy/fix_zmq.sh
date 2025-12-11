#!/bin/bash
# ZeroMQ 快速修复脚本
# 用于修复 "error attaching zsock" 错误

echo "=========================================="
echo "ZeroMQ 快速修复工具"
echo "=========================================="
echo ""

echo "1. 更新包列表..."
sudo apt-get update

echo ""
echo "2. 安装/重新安装 ZeroMQ 运行时库..."
sudo apt-get install -y --reinstall \
    libzmq5 \
    libczmq4 \
    libsodium23

echo ""
echo "3. 更新动态链接库缓存..."
sudo ldconfig

echo ""
echo "4. 验证库安装..."
dpkg -l | grep -E "(libzmq5|libczmq4|libsodium23)"

echo ""
echo "5. 验证库文件..."
ldconfig -p | grep -E "(libzmq|libczmq|libsodium)" || echo "警告: 库未在缓存中"

echo ""
echo "=========================================="
echo "修复完成！"
echo "=========================================="
echo ""
echo "如果问题仍然存在，请："
echo "1. 检查日志: tail -50 ~/iarnet/iarnet.log"
echo "2. 运行诊断: bash deploy/diagnose_zmq.sh"
echo "3. 重启服务: pkill -f iarnet && cd ~/iarnet && ./iarnet --config=config.yaml"

