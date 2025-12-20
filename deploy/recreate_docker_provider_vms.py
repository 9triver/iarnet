#!/usr/bin/env python3
"""
使用安装好 Docker 的镜像重新创建所有 docker provider 虚拟机
并同步 component 镜像到这些节点
"""

import os
import sys
import yaml
import argparse
import subprocess
import time
from pathlib import Path

# 导入现有的模块
SCRIPT_DIR = Path(__file__).parent.absolute()
sys.path.insert(0, str(SCRIPT_DIR))

from delete_vms import VMDeleter
from create_vms import VMBuilder
from sync_images_to_nodes import ImageSyncer

class DockerProviderVMRecreator:
    def __init__(self, config_path: str):
        """初始化 docker provider 虚拟机重新创建器"""
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
        self.docker_config = self.vm_types.get('docker', {})
        
        if not self.docker_config:
            raise ValueError("配置文件中未找到 docker 配置")
        
        self.config_path = str(config_path_obj)
    
    def get_docker_vm_list(self) -> list:
        """获取所有 docker provider 虚拟机的 hostname 列表"""
        vm_list = []
        count = self.docker_config.get('count', 0)
        hostname_prefix = self.docker_config.get('hostname_prefix', 'vm-docker')
        
        for i in range(count):
            hostname = f"{hostname_prefix}-{i+1:02d}"
            vm_list.append(hostname)
        
        return vm_list
    
    def delete_docker_vms(self, delete_disk: bool = True) -> bool:
        """删除所有 docker provider 虚拟机"""
        print("=" * 60)
        print("步骤 1: 删除现有的 docker provider 虚拟机")
        print("=" * 60)
        
        vm_list = self.get_docker_vm_list()
        print(f"\n找到 {len(vm_list)} 个 docker provider 虚拟机:")
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
    
    def create_docker_vms(self) -> bool:
        """创建所有 docker provider 虚拟机"""
        print("\n" + "=" * 60)
        print("步骤 2: 使用安装好 Docker 的镜像创建 docker provider 虚拟机")
        print("=" * 60)
        
        # 检查基础镜像是否存在
        base_image = os.path.expanduser(self.config['global']['base_image'])
        if not os.path.exists(base_image):
            print(f"\n✗ 基础镜像不存在: {base_image}")
            print(f"请确保镜像已导出或路径正确")
            return False
        
        print(f"\n使用基础镜像: {base_image}")
        print(f"镜像大小: {os.path.getsize(base_image) / (1024**3):.2f} GB")
        
        count = self.docker_config.get('count', 0)
        print(f"\n将创建 {count} 个 docker provider 虚拟机")
        
        try:
            builder = VMBuilder(self.config_path)
            results = builder.create_docker_vms()
            
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
    
    def wait_for_vms_ready(self, wait_time: int = 60) -> bool:
        """等待虚拟机启动并完成 cloud-init 初始化"""
        print("\n" + "=" * 60)
        print("步骤 3: 等待虚拟机启动并完成初始化")
        print("=" * 60)
        
        vm_list = self.get_docker_vm_list()
        print(f"\n等待 {len(vm_list)} 个虚拟机启动...")
        print(f"预计等待时间: {wait_time} 秒")
        
        # 等待一段时间让虚拟机启动
        for i in range(wait_time // 10):
            time.sleep(10)
            if (i + 1) % 3 == 0:
                print(f"  已等待 {(i + 1) * 10} 秒...")
        
        print("\n等待完成，虚拟机应该已经启动")
        return True
    
    def cleanup_iarnet_dirs(self) -> bool:
        """清理所有 docker provider 节点上的 iarnet 目录"""
        print("\n" + "=" * 60)
        print("步骤 3.5: 清理虚拟机上的 iarnet 目录")
        print("=" * 60)
        
        try:
            from cleanup_iarnet_dirs import IarnetDirCleaner
            cleaner = IarnetDirCleaner(self.config_path)
            success = cleaner.cleanup_docker_nodes(max_workers=10)
            
            if success:
                print("\n✓ 所有节点的 iarnet 目录已清理")
            else:
                print("\n⚠ 部分节点清理失败，请检查日志")
            
            return success
        except Exception as e:
            print(f"\n✗ 清理过程出错: {e}")
            import traceback
            traceback.print_exc()
            return False
    
    def sync_component_images(self, component_images: list = None) -> bool:
        """同步 component 镜像到 docker provider 节点"""
        print("\n" + "=" * 60)
        print("步骤 4: 同步 component 镜像到 docker provider 节点")
        print("=" * 60)
        
        if not component_images:
            # 默认同步常见的 component 镜像
            component_images = ['iarnet/component:python_3.11-latest']
            print(f"\n未指定 component 镜像，使用默认镜像: {component_images}")
        else:
            print(f"\n将同步以下 component 镜像: {component_images}")
        
        try:
            syncer = ImageSyncer(self.config_path)
            
            # 同步到所有 docker provider 节点
            success = syncer.sync_multiple_images(
                component_images,
                node_type='docker',
                node_ids=None,  # 同步到所有节点
                max_workers=10,
                force=True  # 总是同步，确保使用最新版本
            )
            
            if success:
                print("\n✓ 所有 component 镜像同步完成")
            else:
                print("\n⚠ 部分镜像同步失败，请检查日志")
            
            return success
        except Exception as e:
            print(f"\n✗ 同步镜像过程出错: {e}")
            import traceback
            traceback.print_exc()
            return False
    
    def recreate_all(self, delete_disk: bool = True, wait_time: int = 60, 
                     component_images: list = None, skip_sync: bool = False) -> bool:
        """重新创建所有 docker provider 虚拟机并同步镜像"""
        print("=" * 60)
        print("重新创建所有 docker provider 虚拟机")
        print("=" * 60)
        
        # 显示配置信息
        print(f"\n配置信息:")
        print(f"  虚拟机数量: {self.docker_config.get('count', 0)}")
        print(f"  CPU: {self.docker_config.get('cpu', 'N/A')}")
        print(f"  内存: {self.docker_config.get('memory', 'N/A')} MB")
        print(f"  磁盘: {self.docker_config.get('disk', 'N/A')} GB")
        print(f"  IP 范围: {self.docker_config.get('ip_base', 'N/A')}.{self.docker_config.get('ip_start', 'N/A')} - {self.docker_config.get('ip_base', 'N/A')}.{self.docker_config.get('ip_start', 0) + self.docker_config.get('count', 0) - 1}")
        print(f"  基础镜像: {self.config['global']['base_image']}")
        
        # 确认操作
        print("\n" + "!" * 60)
        print("警告: 此操作将:")
        print("  1. 删除所有现有的 docker provider 虚拟机")
        if delete_disk:
            print("  2. 删除所有 docker provider 虚拟机的磁盘文件")
        print("  3. 使用安装好 Docker 的镜像重新创建所有 docker provider 虚拟机")
        if not skip_sync:
            print("  4. 同步 component 镜像到所有 docker provider 节点")
        print("!" * 60)
        
        response = input("\n是否继续？(yes/no): ")
        if response.lower() not in ['yes', 'y']:
            print("已取消")
            return False
        
        # 步骤1: 删除现有虚拟机
        if not self.delete_docker_vms(delete_disk=delete_disk):
            print("\n✗ 删除虚拟机失败，停止操作")
            return False
        
        # 等待一下，确保删除完成
        print("\n等待 3 秒以确保删除完成...")
        time.sleep(3)
        
        # 步骤2: 创建新虚拟机
        if not self.create_docker_vms():
            print("\n✗ 创建虚拟机失败")
            return False
        
        # 步骤3: 等待虚拟机启动
        if not self.wait_for_vms_ready(wait_time=wait_time):
            print("\n⚠ 等待虚拟机启动完成，但继续执行...")
        
        # 步骤3.5: 清理 iarnet 目录（从基础镜像中继承的数据）
        if not self.cleanup_iarnet_dirs():
            print("\n⚠ 清理 iarnet 目录失败，但继续执行...")
        
        # 步骤4: 同步 component 镜像
        if not skip_sync:
            if not self.sync_component_images(component_images=component_images):
                print("\n⚠ 同步镜像失败，但虚拟机已创建完成")
                return False
        
        print("\n" + "=" * 60)
        print("✓ 所有 docker provider 虚拟机重新创建完成！")
        if not skip_sync:
            print("✓ Component 镜像已同步到所有节点")
        print("=" * 60)
        
        return True

def main():
    parser = argparse.ArgumentParser(description='使用安装好 Docker 的镜像重新创建所有 docker provider 虚拟机并同步 component 镜像')
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
    parser.add_argument(
        '--skip-sync',
        action='store_true',
        help='跳过同步镜像步骤'
    )
    parser.add_argument(
        '--wait-time', '-w',
        type=int,
        default=60,
        help='等待虚拟机启动的时间（秒，默认: 60）'
    )
    parser.add_argument(
        '--component-images', '-i',
        type=str,
        nargs='+',
        default=None,
        help='要同步的 component 镜像列表（默认: iarnet/component:python_3.11-latest）'
    )
    
    args = parser.parse_args()
    
    try:
        recreator = DockerProviderVMRecreator(args.config)
        
        if args.skip_delete and args.skip_create:
            print("错误: 不能同时指定 --skip-delete 和 --skip-create")
            sys.exit(1)
        
        if args.skip_create:
            # 只删除
            recreator.delete_docker_vms(delete_disk=not args.keep_disk)
        elif args.skip_delete:
            # 只创建和同步
            if not recreator.create_docker_vms():
                sys.exit(1)
            if not args.skip_sync:
                recreator.wait_for_vms_ready(wait_time=args.wait_time)
                recreator.sync_component_images(component_images=args.component_images)
        else:
            # 完整流程：删除 + 创建 + 同步
            recreator.recreate_all(
                delete_disk=not args.keep_disk,
                wait_time=args.wait_time,
                component_images=args.component_images,
                skip_sync=args.skip_sync
            )
        
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)

if __name__ == '__main__':
    main()

