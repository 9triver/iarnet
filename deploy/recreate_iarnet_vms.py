#!/usr/bin/env python3
"""
使用新镜像重新创建所有 iarnet 虚拟机
会先删除现有的 iarnet 虚拟机，然后使用新镜像重新创建
"""

import os
import sys
import yaml
import argparse
import subprocess
from pathlib import Path

# 导入现有的模块
SCRIPT_DIR = Path(__file__).parent.absolute()
sys.path.insert(0, str(SCRIPT_DIR))

from delete_vms import VMDeleter
from create_vms import VMBuilder

class IarnetVMRecreator:
    def __init__(self, config_path: str):
        """初始化 iarnet 虚拟机重新创建器"""
        config_path_obj = Path(config_path)
        if not config_path_obj.is_absolute():
            if (SCRIPT_DIR / config_path_obj.name).exists():
                config_path_obj = SCRIPT_DIR / config_path_obj.name
            else:
                config_path_obj = Path(config_path)
        
        if not config_path_obj.exists():
            raise FileNotFoundError(f"配置文件不存在: {config_path_obj}")
        
        with open(config_path_obj, 'r', encoding='utf-8') as f:
            self.config = yaml.safe_load(f)
        
        self.vm_types = self.config['vm_types']
        self.iarnet_config = self.vm_types.get('iarnet', {})
        
        if not self.iarnet_config:
            raise ValueError("配置文件中未找到 iarnet 配置")
        
        self.config_path = str(config_path_obj)
    
    def get_iarnet_vm_list(self) -> list:
        """获取所有 iarnet 虚拟机的 hostname 列表"""
        vm_list = []
        count = self.iarnet_config.get('count', 0)
        hostname_prefix = self.iarnet_config.get('hostname_prefix', 'vm-iarnet')
        
        for i in range(count):
            hostname = f"{hostname_prefix}-{i+1:02d}"
            vm_list.append(hostname)
        
        return vm_list
    
    def delete_iarnet_vms(self, delete_disk: bool = True) -> bool:
        """删除所有 iarnet 虚拟机"""
        print("=" * 60)
        print("步骤 1: 删除现有的 iarnet 虚拟机")
        print("=" * 60)
        
        vm_list = self.get_iarnet_vm_list()
        print(f"\n找到 {len(vm_list)} 个 iarnet 虚拟机:")
        for vm in vm_list:
            print(f"  - {vm}")
        
        if not vm_list:
            print("  没有找到需要删除的虚拟机")
            return True
        
        try:
            deleter = VMDeleter(self.config_path)
            success_count = 0
            failed_vms = []
            
            for hostname in vm_list:
                print(f"\n删除虚拟机: {hostname}")
                try:
                    if deleter.delete_vm(hostname, delete_disk=delete_disk):
                        print(f"  ✓ {hostname} 删除成功")
                        success_count += 1
                    else:
                        print(f"  ⚠ {hostname} 删除失败或不存在")
                        failed_vms.append(hostname)
                except Exception as e:
                    print(f"  ✗ {hostname} 删除出错: {e}")
                    failed_vms.append(hostname)
            
            print("\n" + "-" * 60)
            print(f"删除完成: {success_count}/{len(vm_list)} 个虚拟机成功删除")
            if failed_vms:
                print(f"失败的虚拟机: {failed_vms}")
            print("-" * 60)
            
            return success_count > 0
        except Exception as e:
            error_msg = str(e)
            if 'Permission denied' in error_msg or 'Failed to connect' in error_msg:
                print(f"\n✗ 权限错误: {error_msg}")
                print(f"\n请使用 sudo 运行:")
                print(f"  sudo python3 {__file__}")
                return False
            print(f"\n✗ 删除过程出错: {e}")
            return False
    
    def create_iarnet_vms(self) -> bool:
        """创建所有 iarnet 虚拟机"""
        print("\n" + "=" * 60)
        print("步骤 2: 使用新镜像创建 iarnet 虚拟机")
        print("=" * 60)
        
        # 检查基础镜像是否存在
        base_image = os.path.expanduser(self.config['global']['base_image'])
        if not os.path.exists(base_image):
            print(f"\n✗ 基础镜像不存在: {base_image}")
            print(f"请确保镜像已导出或路径正确")
            return False
        
        print(f"\n使用基础镜像: {base_image}")
        print(f"镜像大小: {os.path.getsize(base_image) / (1024**3):.2f} GB")
        
        count = self.iarnet_config.get('count', 0)
        print(f"\n将创建 {count} 个 iarnet 虚拟机")
        
        try:
            builder = VMBuilder(self.config_path)
            results = builder.create_iarnet_vms()
            
            success_count = sum(results)
            total_count = len(results)
            
            print("\n" + "-" * 60)
            print(f"创建完成: {success_count}/{total_count} 个虚拟机成功创建")
            if success_count < total_count:
                failed_count = total_count - success_count
                print(f"失败的虚拟机数量: {failed_count}")
            print("-" * 60)
            
            return success_count == total_count
        except Exception as e:
            error_msg = str(e)
            if 'Permission denied' in error_msg or 'Failed to connect' in error_msg:
                print(f"\n✗ 权限错误: {error_msg}")
                print(f"\n请使用 sudo 运行:")
                print(f"  sudo python3 {__file__}")
                return False
            print(f"\n✗ 创建过程出错: {e}")
            return False
    
    def recreate_all(self, delete_disk: bool = True) -> bool:
        """重新创建所有 iarnet 虚拟机"""
        print("=" * 60)
        print("重新创建所有 iarnet 虚拟机")
        print("=" * 60)
        
        # 显示配置信息
        print(f"\n配置信息:")
        print(f"  虚拟机数量: {self.iarnet_config.get('count', 0)}")
        print(f"  CPU: {self.iarnet_config.get('cpu', 'N/A')}")
        print(f"  内存: {self.iarnet_config.get('memory', 'N/A')} MB")
        print(f"  磁盘: {self.iarnet_config.get('disk', 'N/A')} GB")
        print(f"  IP 范围: {self.iarnet_config.get('ip_base', 'N/A')}.{self.iarnet_config.get('ip_start', 'N/A')} - {self.iarnet_config.get('ip_base', 'N/A')}.{self.iarnet_config.get('ip_start', 0) + self.iarnet_config.get('count', 0) - 1}")
        print(f"  基础镜像: {self.config['global']['base_image']}")
        
        # 确认操作
        print("\n" + "!" * 60)
        print("警告: 此操作将:")
        print("  1. 删除所有现有的 iarnet 虚拟机")
        if delete_disk:
            print("  2. 删除所有 iarnet 虚拟机的磁盘文件")
        print("  3. 使用新镜像重新创建所有 iarnet 虚拟机")
        print("!" * 60)
        
        response = input("\n是否继续？(yes/no): ")
        if response.lower() not in ['yes', 'y']:
            print("已取消")
            return False
        
        # 步骤1: 删除现有虚拟机
        if not self.delete_iarnet_vms(delete_disk=delete_disk):
            print("\n✗ 删除虚拟机失败，停止操作")
            return False
        
        # 等待一下，确保删除完成
        print("\n等待 3 秒以确保删除完成...")
        import time
        time.sleep(3)
        
        # 步骤2: 创建新虚拟机
        if not self.create_iarnet_vms():
            print("\n✗ 创建虚拟机失败")
            return False
        
        print("\n" + "=" * 60)
        print("✓ 所有 iarnet 虚拟机重新创建完成！")
        print("=" * 60)
        
        return True

def main():
    parser = argparse.ArgumentParser(description='使用新镜像重新创建所有 iarnet 虚拟机')
    parser.add_argument(
        '--config', '-c',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--keep-disk',
        action='store_true',
        help='保留磁盘文件（默认会删除）'
    )
    parser.add_argument(
        '--skip-delete',
        action='store_true',
        help='跳过删除步骤，直接创建（如果虚拟机已不存在）'
    )
    parser.add_argument(
        '--skip-create',
        action='store_true',
        help='只删除，不创建（用于测试）'
    )
    
    args = parser.parse_args()
    
    try:
        recreator = IarnetVMRecreator(args.config)
        
        if args.skip_delete and args.skip_create:
            print("错误: 不能同时指定 --skip-delete 和 --skip-create")
            sys.exit(1)
        
        if args.skip_create:
            # 只删除
            recreator.delete_iarnet_vms(delete_disk=not args.keep_disk)
        elif args.skip_delete:
            # 只创建
            recreator.create_iarnet_vms()
        else:
            # 完整流程：删除 + 创建
            recreator.recreate_all(delete_disk=not args.keep_disk)
        
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()

