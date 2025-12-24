#!/usr/bin/env python3
"""
清理 SSH known_hosts 中的虚拟机条目
重新创建虚拟机后，SSH主机密钥会改变，需要清理旧的密钥
"""

import os
import sys
import yaml
import subprocess
import argparse
from typing import List, Set


class SSHKnownHostsCleaner:
    def __init__(self, config_path: str):
        """初始化SSH known_hosts清理器"""
        with open(config_path, 'r', encoding='utf-8') as f:
            self.config = yaml.safe_load(f)
        
        self.vm_types = self.config['vm_types']
        self.known_hosts_path = os.path.expanduser('~/.ssh/known_hosts')
    
    def get_all_vm_ips(self) -> Set[str]:
        """获取所有虚拟机的IP地址"""
        ips = set()
        
        # iarnet-global节点
        if 'iarnet_global' in self.vm_types:
            global_config = self.vm_types['iarnet_global']
            for i in range(global_config['count']):
                ip_suffix = global_config['ip_start'] + i
                ip_address = f"{global_config['ip_base']}.{ip_suffix}"
                ips.add(ip_address)
        
        # iarnet节点
        if 'iarnet' in self.vm_types:
            iarnet_config = self.vm_types['iarnet']
            for i in range(iarnet_config['count']):
                ip_suffix = iarnet_config['ip_start'] + i
                ip_address = f"{iarnet_config['ip_base']}.{ip_suffix}"
                ips.add(ip_address)
        
        # Docker节点
        if 'docker' in self.vm_types:
            docker_config = self.vm_types['docker']
            for i in range(docker_config['count']):
                ip_suffix = docker_config['ip_start'] + i
                ip_address = f"{docker_config['ip_base']}.{ip_suffix}"
                ips.add(ip_address)
        
        # K8s集群节点
        if 'k8s_clusters' in self.vm_types:
            k8s_config = self.vm_types['k8s_clusters']
            master_config = k8s_config['master']
            worker_config = k8s_config['worker']
            
            for cluster_id in range(1, k8s_config['count'] + 1):
                master_ip_suffix = master_config['ip_start'] + (cluster_id - 1) * master_config['ip_step']
                master_ip = f"{master_config['ip_base']}.{master_ip_suffix}"
                ips.add(master_ip)
                
                for worker_id in range(1, worker_config['count_per_cluster'] + 1):
                    worker_ip_suffix = master_ip_suffix + worker_id
                    worker_ip = f"{master_config['ip_base']}.{worker_ip_suffix}"
                    ips.add(worker_ip)
        
        return ips
    
    def cleanup_ip(self, ip_address: str, verbose: bool = True) -> bool:
        """从known_hosts中删除指定IP的条目"""
        if not os.path.exists(self.known_hosts_path):
            if verbose:
                print(f"  known_hosts文件不存在: {self.known_hosts_path}")
            return True
        
        try:
            # 使用ssh-keygen删除
            cmd = ['ssh-keygen', '-f', self.known_hosts_path, '-R', ip_address]
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=5
            )
            
            if result.returncode == 0:
                if verbose:
                    print(f"  ✓ 已清理: {ip_address}")
                return True
            else:
                # 如果IP不在known_hosts中，ssh-keygen会返回非0，但这不是错误
                if 'not found' in result.stderr.lower() or 'not found' in result.stdout.lower():
                    if verbose:
                        print(f"  - 未找到: {ip_address}（可能已清理）")
                    return True
                else:
                    if verbose:
                        print(f"  ✗ 清理失败: {ip_address} - {result.stderr}")
                    return False
        except Exception as e:
            if verbose:
                print(f"  ✗ 清理失败: {ip_address} - {e}")
            return False
    
    def cleanup_all(self, verbose: bool = True) -> int:
        """清理所有虚拟机的SSH密钥"""
        ips = self.get_all_vm_ips()
        
        if verbose:
            print(f"\n清理 {len(ips)} 个虚拟机的SSH密钥...")
            print(f"known_hosts文件: {self.known_hosts_path}")
            print("-" * 60)
        
        cleaned = 0
        for ip in sorted(ips):
            if self.cleanup_ip(ip, verbose):
                cleaned += 1
        
        if verbose:
            print("-" * 60)
            print(f"清理完成: {cleaned}/{len(ips)} 个IP地址")
        
        return cleaned
    
    def cleanup_by_type(self, vm_type: str, verbose: bool = True) -> int:
        """按类型清理虚拟机的SSH密钥"""
        ips = set()
        
        if vm_type == 'iarnet_global' and 'iarnet_global' in self.vm_types:
            global_config = self.vm_types['iarnet_global']
            for i in range(global_config['count']):
                ip_suffix = global_config['ip_start'] + i
                ip_address = f"{global_config['ip_base']}.{ip_suffix}"
                ips.add(ip_address)
        elif vm_type == 'iarnet' and 'iarnet' in self.vm_types:
            iarnet_config = self.vm_types['iarnet']
            for i in range(iarnet_config['count']):
                ip_suffix = iarnet_config['ip_start'] + i
                ip_address = f"{iarnet_config['ip_base']}.{ip_suffix}"
                ips.add(ip_address)
        elif vm_type == 'docker' and 'docker' in self.vm_types:
            docker_config = self.vm_types['docker']
            for i in range(docker_config['count']):
                ip_suffix = docker_config['ip_start'] + i
                ip_address = f"{docker_config['ip_base']}.{ip_suffix}"
                ips.add(ip_address)
        elif vm_type == 'k8s' and 'k8s_clusters' in self.vm_types:
            k8s_config = self.vm_types['k8s_clusters']
            master_config = k8s_config['master']
            worker_config = k8s_config['worker']
            
            for cluster_id in range(1, k8s_config['count'] + 1):
                master_ip_suffix = master_config['ip_start'] + (cluster_id - 1) * master_config['ip_step']
                master_ip = f"{master_config['ip_base']}.{master_ip_suffix}"
                ips.add(master_ip)
                
                for worker_id in range(1, worker_config['count_per_cluster'] + 1):
                    worker_ip_suffix = master_ip_suffix + worker_id
                    worker_ip = f"{master_config['ip_base']}.{worker_ip_suffix}"
                    ips.add(worker_ip)
        
        if verbose:
            print(f"\n清理 {vm_type} 类型的 {len(ips)} 个虚拟机的SSH密钥...")
            print(f"known_hosts文件: {self.known_hosts_path}")
            print("-" * 60)
        
        cleaned = 0
        for ip in sorted(ips):
            if self.cleanup_ip(ip, verbose):
                cleaned += 1
        
        if verbose:
            print("-" * 60)
            print(f"清理完成: {cleaned}/{len(ips)} 个IP地址")
        
        return cleaned


def main():
    parser = argparse.ArgumentParser(
        description='清理SSH known_hosts中的虚拟机条目',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  %(prog)s                          # 清理所有虚拟机的SSH密钥
  %(prog)s --type iarnet            # 只清理 iarnet 节点的SSH密钥
  %(prog)s --type docker            # 只清理 docker 节点的SSH密钥
  %(prog)s --type k8s               # 只清理 k8s 集群的SSH密钥
  %(prog)s --ip 192.168.100.11      # 清理指定IP的SSH密钥
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
        help='清理类型 (默认: all)'
    )
    
    parser.add_argument(
        '--ip',
        help='清理指定IP地址的SSH密钥'
    )
    
    parser.add_argument(
        '--quiet', '-q',
        action='store_true',
        help='静默模式（不显示详细信息）'
    )
    
    args = parser.parse_args()
    
    try:
        cleaner = SSHKnownHostsCleaner(args.config)
        
        if args.ip:
            # 清理指定IP
            cleaner.cleanup_ip(args.ip, verbose=not args.quiet)
        elif args.type == 'all':
            # 清理所有
            cleaner.cleanup_all(verbose=not args.quiet)
        else:
            # 按类型清理
            cleaner.cleanup_by_type(args.type, verbose=not args.quiet)
        
    except FileNotFoundError as e:
        print(f"错误: 配置文件不存在: {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()

