#!/usr/bin/env python3
"""
批量删除虚拟机脚本
根据 vm-config.yaml 配置批量删除虚拟机
"""

import os
import sys
import yaml
import libvirt
import argparse
import subprocess
from pathlib import Path

class VMDeleter:
    def __init__(self, config_path: str):
        """初始化VM删除器"""
        with open(config_path, 'r', encoding='utf-8') as f:
            self.config = yaml.safe_load(f)
        
        self.vm_types = self.config['vm_types']
        
        # 连接libvirt
        try:
            self.conn = libvirt.open('qemu:///system')
            if self.conn is None:
                raise Exception("无法连接到libvirt守护进程")
        except Exception as e:
            raise Exception(f"连接libvirt失败: {e}")
    
    def __del__(self):
        """清理libvirt连接"""
        if hasattr(self, 'conn') and self.conn:
            self.conn.close()
    
    def delete_vm(self, hostname: str, delete_disk: bool = False) -> bool:
        """删除单个虚拟机"""
        try:
            # 查找虚拟机
            try:
                dom = self.conn.lookupByName(hostname)
            except libvirt.libvirtError:
                print(f"  虚拟机不存在: {hostname}")
                return True
            
            # 如果正在运行，先关闭
            if dom.isActive():
                print(f"  关闭虚拟机: {hostname}")
                dom.destroy()
            
            # 删除虚拟机定义
            print(f"  删除虚拟机: {hostname}")
            dom.undefine()
            
            # 删除磁盘（可选）
            if delete_disk:
                disk_path = f"/var/lib/libvirt/images/{hostname}.qcow2"
                if os.path.exists(disk_path):
                    print(f"  删除磁盘: {disk_path}")
                    deleted = False
                    
                    # 方法1: 尝试直接删除（如果有权限）
                    try:
                        if os.access(disk_path, os.W_OK):
                            os.remove(disk_path)
                            print(f"  ✓ 磁盘删除成功")
                            deleted = True
                    except (PermissionError, OSError) as e:
                        pass  # 权限不足，继续尝试sudo
                    
                    # 方法2: 使用sudo删除（如果没有权限或直接删除失败）
                    if not deleted:
                        try:
                            result = subprocess.run(
                                ['sudo', 'rm', '-f', disk_path], 
                                check=True, 
                                capture_output=True,
                                text=True,
                                timeout=10
                            )
                            # 验证文件是否真的被删除
                            if not os.path.exists(disk_path):
                                print(f"  ✓ 磁盘删除成功（使用sudo）")
                                deleted = True
                            else:
                                print(f"  ⚠ 删除命令执行成功，但文件仍存在")
                        except subprocess.CalledProcessError as e:
                            error_msg = e.stderr if e.stderr else str(e)
                            print(f"  ✗ 磁盘删除失败: {error_msg}")
                            print(f"  请手动删除: sudo rm -f {disk_path}")
                        except subprocess.TimeoutExpired:
                            print(f"  ✗ 磁盘删除超时")
                            print(f"  请手动删除: sudo rm -f {disk_path}")
                        except Exception as e:
                            print(f"  ✗ 磁盘删除失败: {e}")
                            print(f"  请手动删除: sudo rm -f {disk_path}")
                    
                    # 最终验证
                    if deleted and os.path.exists(disk_path):
                        print(f"  ⚠ 警告: 磁盘文件仍存在，可能需要手动删除: sudo rm -f {disk_path}")
                else:
                    print(f"  磁盘不存在: {disk_path}（可能已删除）")
            
            print(f"  ✓ 删除成功: {hostname}")
            return True
            
        except Exception as e:
            print(f"  ✗ 删除失败 {hostname}: {e}")
            return False
    
    def delete_iarnet_vms(self, delete_disk: bool = False) -> int:
        """删除iarnet节点虚拟机"""
        iarnet_config = self.vm_types['iarnet']
        count = 0
        
        print(f"\n删除 {iarnet_config['count']} 个 iarnet 节点...")
        
        for i in range(iarnet_config['count']):
            hostname = f"{iarnet_config['hostname_prefix']}-{i+1:02d}"
            if self.delete_vm(hostname, delete_disk):
                count += 1
        
        return count
    
    def delete_docker_vms(self, delete_disk: bool = False) -> int:
        """删除Docker节点虚拟机"""
        docker_config = self.vm_types['docker']
        count = 0
        
        print(f"\n删除 {docker_config['count']} 个 Docker 节点...")
        
        for i in range(docker_config['count']):
            hostname = f"{docker_config['hostname_prefix']}-{i+1:02d}"
            if self.delete_vm(hostname, delete_disk):
                count += 1
        
        return count
    
    def delete_k8s_cluster_vms(self, delete_disk: bool = False) -> int:
        """删除K8s集群虚拟机"""
        k8s_config = self.vm_types['k8s_clusters']
        master_config = k8s_config['master']
        worker_config = k8s_config['worker']
        count = 0
        
        cluster_count = k8s_config['count']
        print(f"\n删除 {cluster_count} 个 K8s 集群...")
        
        for cluster_id in range(1, cluster_count + 1):
            # 删除master节点
            master_hostname = f"{master_config['hostname_prefix']}-{cluster_id:02d}{master_config['hostname_suffix']}"
            if self.delete_vm(master_hostname, delete_disk):
                count += 1
            
            # 删除worker节点
            for worker_id in range(1, worker_config['count_per_cluster'] + 1):
                worker_hostname = f"{worker_config['hostname_prefix']}-{cluster_id:02d}{worker_config['hostname_suffix']}-{worker_id}"
                if self.delete_vm(worker_hostname, delete_disk):
                    count += 1
        
        return count
    
    def delete_all_vms(self, delete_disk: bool = False):
        """删除所有虚拟机"""
        print("=" * 60)
        print("开始批量删除虚拟机")
        print("=" * 60)
        
        total = 0
        
        # 删除iarnet节点
        total += self.delete_iarnet_vms(delete_disk)
        
        # 删除Docker节点
        total += self.delete_docker_vms(delete_disk)
        
        # 删除K8s集群
        total += self.delete_k8s_cluster_vms(delete_disk)
        
        print("\n" + "=" * 60)
        print(f"删除完成，共删除 {total} 台虚拟机")
        print("=" * 60)


def main():
    parser = argparse.ArgumentParser(description='批量删除虚拟机')
    parser.add_argument(
        '--config', '-c',
        default='./vm-config.yaml',
        help='配置文件路径 (默认: ./vm-config.yaml)'
    )
    parser.add_argument(
        '--type', '-t',
        choices=['all', 'iarnet', 'docker', 'k8s'],
        default='all',
        help='删除类型 (默认: all)'
    )
    parser.add_argument(
        '--delete-disk', '-d',
        action='store_true',
        help='同时删除磁盘镜像'
    )
    parser.add_argument(
        '--yes', '-y',
        action='store_true',
        help='跳过确认提示'
    )
    
    args = parser.parse_args()
    
    # 确认提示
    if not args.yes:
        response = input("确定要删除虚拟机吗？(yes/no): ")
        if response.lower() != 'yes':
            print("取消删除")
            return
    
    try:
        deleter = VMDeleter(args.config)
        
        if args.type == 'all':
            deleter.delete_all_vms(delete_disk=args.delete_disk)
        elif args.type == 'iarnet':
            deleter.delete_iarnet_vms(delete_disk=args.delete_disk)
        elif args.type == 'docker':
            deleter.delete_docker_vms(delete_disk=args.delete_disk)
        elif args.type == 'k8s':
            deleter.delete_k8s_cluster_vms(delete_disk=args.delete_disk)
        
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()

