#!/usr/bin/env python3
"""
修复 K8s 镜像的循环 backing file 问题
使用底层方法直接修改 qcow2 元数据，移除 backing file 引用
"""

import struct
import sys
import os

def fix_qcow2_backing_file(image_path, output_path):
    """修复 qcow2 镜像的循环 backing file"""
    
    # qcow2 文件头结构（前 72 字节）
    # offset 0x38 (56): backing_file_offset (8 bytes, big-endian)
    # offset 0x40 (64): backing_file_size (4 bytes, big-endian)
    
    with open(image_path, 'rb') as f:
        header = bytearray(f.read(72))
    
    # 检查魔数（前4字节应该是 'QFI\xfb'）
    magic = header[0:4]
    if magic != b'QFI\xfb':
        raise ValueError("不是有效的 qcow2 文件")
    
    # 读取 backing_file_offset (offset 0x38, 8 bytes, big-endian)
    backing_file_offset = struct.unpack('>Q', header[56:64])[0]
    
    # 读取 backing_file_size (offset 0x40, 4 bytes, big-endian)
    backing_file_size = struct.unpack('>I', header[64:68])[0]
    
    print(f"当前 backing file offset: {backing_file_offset}")
    print(f"当前 backing file size: {backing_file_size}")
    
    if backing_file_offset == 0:
        print("镜像没有 backing file，无需修复")
        return False
    
    # 将 backing_file_offset 和 backing_file_size 设置为 0
    header[56:64] = struct.pack('>Q', 0)  # backing_file_offset = 0
    header[64:68] = struct.pack('>I', 0)   # backing_file_size = 0
    
    # 同时需要清除 backing file format（如果存在）
    # offset 0x44 (68): backing_file_format (4 bytes)
    # 通常设置为 0 表示没有 backing file
    if len(header) >= 72:
        header[68:72] = b'\x00\x00\x00\x00'  # backing_file_format = 0
    
    # 写入修复后的文件
    with open(image_path, 'r+b') as f:
        f.seek(0)
        f.write(header)
        f.flush()
        os.fsync(f.fileno())
    
    print("✓ backing file 引用已移除")
    return True


def main():
    if len(sys.argv) < 2:
        print("使用方法: sudo python3 fix_k8s_image_circular_backing.py <镜像路径>")
        print("示例: sudo python3 fix_k8s_image_circular_backing.py /var/lib/libvirt/images/ubuntu-22.04-cloud-k8s.qcow2")
        sys.exit(1)
    
    image_path = sys.argv[1]
    
    if not os.path.exists(image_path):
        print(f"错误: 镜像文件不存在: {image_path}")
        sys.exit(1)
    
    # 备份原镜像
    backup_path = f"{image_path}.backup.$(date +%Y%m%d_%H%M%S)"
    print(f"备份原镜像到: {backup_path}")
    import shutil
    import time
    backup_path = f"{image_path}.backup.{int(time.time())}"
    shutil.copy2(image_path, backup_path)
    print(f"✓ 备份完成")
    
    # 修复镜像
    print(f"\n修复镜像: {image_path}")
    try:
        if fix_qcow2_backing_file(image_path, image_path):
            print("\n✓ 修复完成！")
            print(f"备份文件: {backup_path}")
            print("\n验证修复结果:")
            import subprocess
            result = subprocess.run(
                ['qemu-img', 'info', image_path],
                capture_output=True,
                text=True
            )
            print(result.stdout)
        else:
            print("\n镜像无需修复")
    except Exception as e:
        print(f"\n✗ 修复失败: {e}")
        print(f"已恢复备份: {backup_path}")
        shutil.copy2(backup_path, image_path)
        sys.exit(1)


if __name__ == '__main__':
    main()

