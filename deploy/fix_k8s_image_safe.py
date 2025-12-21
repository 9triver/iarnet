#!/usr/bin/env python3
"""
安全修复 K8s 镜像的循环 backing file 问题
使用 qemu-img rebase 命令，但先手动修复文件头以避免循环引用
"""

import subprocess
import sys
import os
import shutil
import time

def fix_qcow2_backing_file_safe(image_path):
    """安全修复 qcow2 镜像的循环 backing file"""
    
    # 备份原镜像
    backup_path = f"{image_path}.backup.{int(time.time())}"
    print(f"1. 备份原镜像到: {backup_path}")
    shutil.copy2(image_path, backup_path)
    print(f"   ✓ 备份完成")
    
    # 方法1: 尝试使用 qemu-img rebase 移除 backing file
    print(f"\n2. 尝试使用 qemu-img rebase 移除 backing file...")
    try:
        # 先尝试 rebase 到空（移除 backing file）
        result = subprocess.run(
            ['qemu-img', 'rebase', '-u', '-b', '', image_path],
            capture_output=True,
            text=True,
            timeout=60
        )
        if result.returncode == 0:
            print("   ✓ rebase 成功")
            return True, backup_path
        else:
            print(f"   ✗ rebase 失败: {result.stderr}")
    except subprocess.TimeoutExpired:
        print("   ✗ rebase 超时")
    except Exception as e:
        print(f"   ✗ rebase 异常: {e}")
    
    # 方法2: 如果 rebase 失败，尝试从原始基础镜像重新创建
    print(f"\n3. rebase 失败，尝试从原始基础镜像重新创建...")
    base_image = "/var/lib/libvirt/images/ubuntu-22.04-cloud.qcow2"
    
    if not os.path.exists(base_image):
        print(f"   ✗ 原始基础镜像不存在: {base_image}")
        print(f"   请手动指定基础镜像路径")
        return False, backup_path
    
    print(f"   使用基础镜像: {base_image}")
    temp_image = f"{image_path}.temp_recreate"
    
    try:
        # 获取原镜像的虚拟大小
        info_result = subprocess.run(
            ['qemu-img', 'info', backup_path],
            capture_output=True,
            text=True,
            timeout=10
        )
        
        # 从备份中提取大小（如果可能）
        size = "30G"  # 默认大小
        if 'virtual size' in info_result.stdout:
            for line in info_result.stdout.split('\n'):
                if 'virtual size' in line:
                    # 提取大小，例如 "30 GiB" -> "30G"
                    import re
                    match = re.search(r'(\d+(?:\.\d+)?)\s*Gi?B', line)
                    if match:
                        size = f"{int(float(match.group(1)))}G"
                        break
        
        print(f"   创建新镜像（大小: {size}）...")
        # 从基础镜像创建新镜像（不使用 backing file，直接复制）
        result = subprocess.run(
            ['qemu-img', 'create', '-f', 'qcow2', '-b', base_image, '-F', 'qcow2', temp_image, size],
            capture_output=True,
            text=True,
            timeout=60
        )
        
        if result.returncode != 0:
            print(f"   ✗ 创建失败: {result.stderr}")
            return False, backup_path
        
        # 使用 qemu-img convert 将备份镜像的数据复制到新镜像
        # 但由于循环引用，这可能也会失败
        print(f"   注意: 由于循环引用，无法直接转换数据")
        print(f"   建议: 使用备份镜像重新创建虚拟机并重新配置")
        
        # 删除临时文件
        if os.path.exists(temp_image):
            os.remove(temp_image)
        
        return False, backup_path
        
    except Exception as e:
        print(f"   ✗ 重新创建失败: {e}")
        if os.path.exists(temp_image):
            os.remove(temp_image)
        return False, backup_path


def main():
    if len(sys.argv) < 2:
        print("使用方法: sudo python3 fix_k8s_image_safe.py <镜像路径>")
        print("示例: sudo python3 fix_k8s_image_safe.py /var/lib/libvirt/images/ubuntu-22.04-cloud-k8s.qcow2")
        sys.exit(1)
    
    image_path = sys.argv[1]
    
    if not os.path.exists(image_path):
        print(f"错误: 镜像文件不存在: {image_path}")
        sys.exit(1)
    
    print("=" * 60)
    print("安全修复 K8s 镜像的循环 backing file")
    print("=" * 60)
    print(f"镜像路径: {image_path}")
    print()
    
    success, backup_path = fix_qcow2_backing_file_safe(image_path)
    
    if success:
        print("\n" + "=" * 60)
        print("✓ 修复完成！")
        print("=" * 60)
        print(f"备份文件: {backup_path}")
        print("\n验证修复结果:")
        result = subprocess.run(
            ['qemu-img', 'info', image_path],
            capture_output=True,
            text=True
        )
        print(result.stdout)
    else:
        print("\n" + "=" * 60)
        print("⚠ 自动修复失败")
        print("=" * 60)
        print(f"备份文件: {backup_path}")
        print("\n建议方案:")
        print("1. 恢复备份镜像:")
        print(f"   sudo cp {backup_path} {image_path}")
        print("\n2. 从原始基础镜像重新创建虚拟机:")
        print("   - 使用 ubuntu-22.04-cloud.qcow2 作为基础镜像")
        print("   - 在虚拟机上安装 K8s 依赖")
        print("   - 拉取 K8s 镜像")
        print("   - 使用修复后的 export_k8s_vm_to_image.py 重新导出")


if __name__ == '__main__':
    main()

