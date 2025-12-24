#!/usr/bin/env python3
"""
重新创建所有虚拟机脚本
先删除现有虚拟机，然后重新创建
"""

import os
import sys
import argparse
import time
from delete_vms import VMDeleter
from create_vms import VMBuilder
from cleanup_ssh_known_hosts import SSHKnownHostsCleaner


class VMRecreator:
    def __init__(self, config_path: str):
        """初始化VM重新创建器"""
        self.config_path = config_path
        self.deleter = VMDeleter(config_path)
        self.builder = VMBuilder(config_path)
        self.ssh_cleaner = SSHKnownHostsCleaner(config_path)
    
    def recreate_iarnet_vms(self, delete_disk: bool = True) -> bool:
        """重新创建iarnet节点虚拟机"""
        print("\n" + "=" * 60)
        print("重新创建 iarnet 节点虚拟机")
        print("=" * 60)
        
        # 删除现有虚拟机
        print("\n步骤 1/2: 删除现有 iarnet 节点...")
        self.deleter.delete_iarnet_vms(delete_disk=delete_disk)
        
        # 等待一下确保删除完成
        time.sleep(2)
        
        # 重新创建虚拟机
        print("\n步骤 2/2: 创建新的 iarnet 节点...")
        results = self.builder.create_iarnet_vms()
        
        success = all(results)
        total = len(results)
        success_count = sum(results)
        
        # 清理SSH密钥
        print("\n清理SSH known_hosts...")
        try:
            self.ssh_cleaner.cleanup_by_type('iarnet_global', verbose=False)
        except Exception:
            pass
        
        print(f"\n重新创建完成: {success_count}/{total} 台虚拟机成功")
        return success
    
    def recreate_docker_vms(self, delete_disk: bool = True) -> bool:
        """重新创建Docker节点虚拟机"""
        print("\n" + "=" * 60)
        print("重新创建 Docker 节点虚拟机")
        print("=" * 60)
        
        # 删除现有虚拟机
        print("\n步骤 1/2: 删除现有 Docker 节点...")
        self.deleter.delete_docker_vms(delete_disk=delete_disk)
        
        # 等待一下确保删除完成
        time.sleep(2)
        
        # 重新创建虚拟机
        print("\n步骤 2/2: 创建新的 Docker 节点...")
        results = self.builder.create_docker_vms()
        
        success = all(results)
        total = len(results)
        success_count = sum(results)
        
        # 清理SSH密钥
        print("\n清理SSH known_hosts...")
        try:
            self.ssh_cleaner.cleanup_by_type('iarnet', verbose=False)
        except Exception:
            pass
        
        print(f"\n重新创建完成: {success_count}/{total} 台虚拟机成功")
        return success
    
    def recreate_k8s_cluster_vms(self, delete_disk: bool = True) -> bool:
        """重新创建K8s集群虚拟机"""
        print("\n" + "=" * 60)
        print("重新创建 K8s 集群虚拟机")
        print("=" * 60)
        
        # 删除现有虚拟机
        print("\n步骤 1/2: 删除现有 K8s 集群...")
        self.deleter.delete_k8s_cluster_vms(delete_disk=delete_disk)
        
        # 等待一下确保删除完成
        time.sleep(2)
        
        # 重新创建虚拟机
        print("\n步骤 2/2: 创建新的 K8s 集群...")
        results = self.builder.create_k8s_cluster_vms()
        
        success = all(results)
        total = len(results)
        success_count = sum(results)
        
        # 清理SSH密钥
        print("\n清理SSH known_hosts...")
        try:
            self.ssh_cleaner.cleanup_by_type('docker', verbose=False)
        except Exception:
            pass
        
        print(f"\n重新创建完成: {success_count}/{total} 台虚拟机成功")
        return success
    
    def recreate_iarnet_global_vms(self, delete_disk: bool = True) -> bool:
        """重新创建iarnet-global节点虚拟机"""
        print("\n" + "=" * 60)
        print("重新创建 iarnet-global 节点虚拟机")
        print("=" * 60)
        
        # 检查配置中是否有iarnet_global
        if 'iarnet_global' not in self.deleter.vm_types:
            print("配置中没有 iarnet-global 节点，跳过")
            return True
        
        global_config = self.deleter.vm_types['iarnet_global']
        
        # 删除现有虚拟机
        print("\n步骤 1/2: 删除现有 iarnet-global 节点...")
        count = 0
        for i in range(global_config['count']):
            hostname = f"{global_config['hostname_prefix']}-{i+1:02d}"
            if self.deleter.delete_vm(hostname, delete_disk):
                count += 1
        
        # 等待一下确保删除完成
        time.sleep(2)
        
        # 重新创建虚拟机
        print("\n步骤 2/2: 创建新的 iarnet-global 节点...")
        results = self.builder.create_iarnet_global_vms()
        
        if not results:
            print("没有 iarnet-global 节点配置")
            return True
        
        success = all(results)
        total = len(results)
        success_count = sum(results)
        
        # 清理SSH密钥
        print("\n清理SSH known_hosts...")
        try:
            self.ssh_cleaner.cleanup_by_type('k8s', verbose=False)
        except Exception:
            pass
        
        print(f"\n重新创建完成: {success_count}/{total} 台虚拟机成功")
        return success
    
    def recreate_all_vms(self, delete_disk: bool = True) -> bool:
        """重新创建所有虚拟机"""
        print("\n" + "=" * 80)
        print("重新创建所有虚拟机")
        print("=" * 80)
        print(f"配置: {self.config_path}")
        print(f"删除磁盘: {'是' if delete_disk else '否'}")
        print("=" * 80)
        
        all_success = True
        
        # 重新创建iarnet-global节点
        try:
            if not self.recreate_iarnet_global_vms(delete_disk):
                all_success = False
        except Exception as e:
            print(f"重新创建 iarnet-global 节点时出错: {e}")
            all_success = False
        
        # 重新创建iarnet节点
        try:
            if not self.recreate_iarnet_vms(delete_disk):
                all_success = False
        except Exception as e:
            print(f"重新创建 iarnet 节点时出错: {e}")
            all_success = False
        
        # 重新创建Docker节点
        try:
            if not self.recreate_docker_vms(delete_disk):
                all_success = False
        except Exception as e:
            print(f"重新创建 Docker 节点时出错: {e}")
            all_success = False
        
        # 重新创建K8s集群
        try:
            if not self.recreate_k8s_cluster_vms(delete_disk):
                all_success = False
        except Exception as e:
            print(f"重新创建 K8s 集群时出错: {e}")
            all_success = False
        
        # 清理SSH known_hosts
        print("\n" + "=" * 80)
        print("清理SSH known_hosts中的旧密钥...")
        print("=" * 80)
        try:
            self.ssh_cleaner.cleanup_all(verbose=True)
        except Exception as e:
            print(f"警告: 清理SSH known_hosts时出错: {e}")
            print("可以手动运行: python3 cleanup_ssh_known_hosts.py")
        
        print("\n" + "=" * 80)
        if all_success:
            print("✓ 所有虚拟机重新创建成功！")
        else:
            print("✗ 部分虚拟机重新创建失败，请检查上面的错误信息")
        print("=" * 80)
        
        return all_success


def main():
    parser = argparse.ArgumentParser(
        description='重新创建虚拟机（先删除后创建）',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  %(prog)s                          # 重新创建所有虚拟机（保留磁盘）
  %(prog)s --delete-disk             # 重新创建所有虚拟机（删除磁盘）
  %(prog)s --type iarnet             # 只重新创建 iarnet 节点
  %(prog)s --type docker --delete-disk  # 重新创建 docker 节点并删除磁盘
  %(prog)s --type k8s                # 只重新创建 K8s 集群
  %(prog)s --yes --delete-disk       # 跳过确认，删除磁盘并重新创建
        """
    )
    
    parser.add_argument(
        '--config', '-c',
        default='./vm-config.yaml',
        help='配置文件路径 (默认: ./vm-config.yaml)'
    )
    
    parser.add_argument(
        '--type', '-t',
        choices=['all', 'iarnet_global', 'iarnet', 'docker', 'k8s'],
        default='all',
        help='重新创建类型 (默认: all)'
    )
    
    parser.add_argument(
        '--delete-disk', '-d',
        action='store_true',
        help='删除磁盘镜像（默认: 保留磁盘）'
    )
    
    parser.add_argument(
        '--yes', '-y',
        action='store_true',
        help='跳过确认提示'
    )
    
    args = parser.parse_args()
    
    # 确认提示
    if not args.yes:
        vm_type_name = {
            'all': '所有虚拟机',
            'iarnet_global': 'iarnet-global 节点',
            'iarnet': 'iarnet 节点',
            'docker': 'Docker 节点',
            'k8s': 'K8s 集群'
        }.get(args.type, args.type)
        
        disk_action = "删除磁盘并" if args.delete_disk else ""
        print(f"\n警告: 将{disk_action}重新创建 {vm_type_name}")
        print("此操作将:")
        print("  1. 删除现有的虚拟机（如果存在）")
        if args.delete_disk:
            print("  2. 删除虚拟机磁盘镜像")
        print("  3. 重新创建虚拟机")
        print()
        
        response = input("确定要继续吗？(yes/no): ")
        if response.lower() != 'yes':
            print("取消操作")
            return
    
    try:
        recreator = VMRecreator(args.config)
        
        if args.type == 'all':
            success = recreator.recreate_all_vms(delete_disk=args.delete_disk)
        elif args.type == 'iarnet_global':
            success = recreator.recreate_iarnet_global_vms(delete_disk=args.delete_disk)
        elif args.type == 'iarnet':
            success = recreator.recreate_iarnet_vms(delete_disk=args.delete_disk)
        elif args.type == 'docker':
            success = recreator.recreate_docker_vms(delete_disk=args.delete_disk)
        elif args.type == 'k8s':
            success = recreator.recreate_k8s_cluster_vms(delete_disk=args.delete_disk)
        
        if not success:
            sys.exit(1)
            
    except KeyboardInterrupt:
        print("\n\n操作被用户中断")
        sys.exit(1)
    except Exception as e:
        print(f"\n错误: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()

