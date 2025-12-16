#!/bin/bash
# ZeroMQ 诊断脚本
# 用于排查 "error attaching zsock" 错误

echo "=========================================="
echo "ZeroMQ 诊断工具"
echo "=========================================="
echo ""

echo "1. 检查已安装的 ZeroMQ 相关包:"
dpkg -l | grep -E "(libzmq|libczmq|libsodium)" || echo "未找到相关包"
echo ""

echo "2. 检查库文件是否存在:"
find /usr/lib /usr/lib/x86_64-linux-gnu -name "libzmq.so*" 2>/dev/null | head -5
find /usr/lib /usr/lib/x86_64-linux-gnu -name "libczmq.so*" 2>/dev/null | head -5
find /usr/lib /usr/lib/x86_64-linux-gnu -name "libsodium.so*" 2>/dev/null | head -5
echo ""

echo "3. 检查动态链接库缓存:"
ldconfig -p | grep -E "(libzmq|libczmq|libsodium)" || echo "库未在缓存中"
echo ""

echo "4. 检查库依赖关系:"
if [ -f /usr/lib/x86_64-linux-gnu/libzmq.so.5 ]; then
    echo "libzmq.so.5 依赖:"
    ldd /usr/lib/x86_64-linux-gnu/libzmq.so.5
elif [ -f /usr/lib/libzmq.so.5 ]; then
    echo "libzmq.so.5 依赖:"
    ldd /usr/lib/libzmq.so.5
else
    echo "libzmq.so.5 未找到"
fi
echo ""

if [ -f /usr/lib/x86_64-linux-gnu/libczmq.so.4 ]; then
    echo "libczmq.so.4 依赖:"
    ldd /usr/lib/x86_64-linux-gnu/libczmq.so.4
elif [ -f /usr/lib/libczmq.so.4 ]; then
    echo "libczmq.so.4 依赖:"
    ldd /usr/lib/libczmq.so.4
else
    echo "libczmq.so.4 未找到"
fi
echo ""

echo "5. 测试库加载（使用 Python）:"
python3 -c "
import ctypes
import sys

libs_to_test = ['libzmq.so.5', 'libzmq.so', 'libczmq.so.4', 'libczmq.so']
for lib_name in libs_to_test:
    try:
        lib = ctypes.CDLL(lib_name)
        print(f'✓ {lib_name} 加载成功')
    except OSError as e:
        print(f'✗ {lib_name} 加载失败: {e}')
" 2>&1
echo ""

echo "6. 检查环境变量:"
echo "LD_LIBRARY_PATH: ${LD_LIBRARY_PATH:-未设置}"
echo ""

echo "7. 建议的修复命令:"
echo "sudo apt-get update"
echo "sudo apt-get install -y libzmq5 libczmq4 libsodium23"
echo "sudo ldconfig"
echo ""

