#!/usr/bin/env python3
"""
SSH连接到虚拟机的便捷脚本
支持通过hostname或IP地址连接虚拟机
"""

import os
import sys
import yaml
import subprocess
import argparse
from typing import Dict, List, Optional

class VMSSH:
    def __init__(self, config_path: str):
        """初始化VM SSH连接器"""
        with open(config_path, 'r', encoding='utf-8') as f:
            self.config = yaml.safe_load(f)
        
        self.global_config = self.config['global']
        self.vm_types = self.config['vm_types']
        self.user = self.global_config['user']
        
        # 构建虚拟机信息映射表
        self.vm_info = {}
        self._build_vm_mapping()
    
    def _build_vm_mapping(self):
        """构建虚拟机hostname到IP的映射"""
        # iarnet节点
        iarnet_config = self.vm_types['iarnet']
        for i in range(iarnet_config['count']):
            ip_suffix = iarnet_config['ip_start'] + i
            ip_address = f"{iarnet_config['ip_base']}.{ip_suffix}"
            hostname = f"{iarnet_config['hostname_prefix']}-{i+1:02d}"
            self.vm_info[hostname] = {
                'ip': ip_address,
                'type': 'iarnet',
                'index': i + 1
            }
        
        # Docker节点
        docker_config = self.vm_types['docker']
        for i in range(docker_config['count']):
            ip_suffix = docker_config['ip_start'] + i
            ip_address = f"{docker_config['ip_base']}.{ip_suffix}"
            hostname = f"{docker_config['hostname_prefix']}-{i+1:02d}"
            self.vm_info[hostname] = {
                'ip': ip_address,
                'type': 'docker',
                'index': i + 1
            }
        
        # K8s集群节点
        k8s_config = self.vm_types['k8s_clusters']
        master_config = k8s_config['master']
        worker_config = k8s_config['worker']
        
        for cluster_id in range(1, k8s_config['count'] + 1):
            # Master节点
            master_ip_suffix = master_config['ip_start'] + (cluster_id - 1) * master_config['ip_step']
            master_ip = f"{master_config['ip_base']}.{master_ip_suffix}"
            master_hostname = f"{master_config['hostname_prefix']}-{cluster_id:02d}{master_config['hostname_suffix']}"
            self.vm_info[master_hostname] = {
                'ip': master_ip,
                'type': 'k8s-master',
                'cluster_id': cluster_id,
                'index': cluster_id
            }
            
            # Worker节点
            for worker_id in range(1, worker_config['count_per_cluster'] + 1):
                worker_ip_suffix = master_ip_suffix + worker_id
                worker_ip = f"{master_config['ip_base']}.{worker_ip_suffix}"
                worker_hostname = f"{worker_config['hostname_prefix']}-{cluster_id:02d}{worker_config['hostname_suffix']}-{worker_id}"
                self.vm_info[worker_hostname] = {
                    'ip': worker_ip,
                    'type': 'k8s-worker',
                    'cluster_id': cluster_id,
                    'worker_id': worker_id
                }
    
    def find_vm(self, identifier: str) -> Optional[Dict]:
        """根据hostname或IP查找虚拟机信息"""
        # 直接匹配hostname
        if identifier in self.vm_info:
            return self.vm_info[identifier]
        
        # 匹配IP地址
        for hostname, info in self.vm_info.items():
            if info['ip'] == identifier:
                return info
        
        # 尝试模糊匹配hostname
        matching = []
        for hostname, info in self.vm_info.items():
            if identifier.lower() in hostname.lower():
                matching.append((hostname, info))
        
        if len(matching) == 1:
            return matching[0][1]
        elif len(matching) > 1:
            print(f"找到多个匹配的虚拟机:")
            for hostname, info in matching:
                print(f"  {hostname} ({info['ip']})")
            return None
        
        return None
    
    def list_vms(self, vm_type: Optional[str] = None):
        """列出所有虚拟机"""
        print("=" * 80)
        print(f"{'Hostname':<30} {'IP Address':<20} {'Type':<15} {'Info':<20}")
        print("=" * 80)
        
        for hostname in sorted(self.vm_info.keys()):
            info = self.vm_info[hostname]
            
            if vm_type and info['type'] != vm_type:
                continue
            
            info_str = ""
            if info['type'] == 'k8s-master':
                info_str = f"Cluster {info['cluster_id']}"
            elif info['type'] == 'k8s-worker':
                info_str = f"Cluster {info['cluster_id']}, Worker {info['worker_id']}"
            else:
                info_str = f"#{info['index']}"
            
            print(f"{hostname:<30} {info['ip']:<20} {info['type']:<15} {info_str:<20}")
        
        print("=" * 80)
        print(f"总计: {len([v for v in self.vm_info.values() if not vm_type or v['type'] == vm_type])} 台虚拟机")
    
    def ssh_connect(self, identifier: str, command: Optional[str] = None, 
                    port: int = 22, extra_args: List[str] = None):
        """SSH连接到虚拟机"""
        vm_info = self.find_vm(identifier)
        
        if vm_info is None:
            # 如果找不到，尝试直接作为IP地址连接
            if self._is_valid_ip(identifier):
                ip_address = identifier
                hostname = identifier
            else:
                print(f"错误: 找不到虚拟机 '{identifier}'")
                print(f"\n可用的虚拟机类型:")
                print(f"  - iarnet: {len([v for v in self.vm_info.values() if v['type'] == 'iarnet'])} 台")
                print(f"  - docker: {len([v for v in self.vm_info.values() if v['type'] == 'docker'])} 台")
                print(f"  - k8s-master: {len([v for v in self.vm_info.values() if v['type'] == 'k8s-master'])} 台")
                print(f"  - k8s-worker: {len([v for v in self.vm_info.values() if v['type'] == 'k8s-worker'])} 台")
                print(f"\n使用 'python3 ssh_vm.py --list' 查看所有虚拟机")
                return False
        else:
            ip_address = vm_info['ip']
            hostname = identifier
        
        # 构建SSH命令
        ssh_cmd = ['ssh']
        
        # 添加SSH选项
        ssh_cmd.extend([
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5'
        ])
        
        # 添加端口
        if port != 22:
            ssh_cmd.extend(['-p', str(port)])
        
        # 添加额外参数
        if extra_args:
            ssh_cmd.extend(extra_args)
        
        # 添加用户和主机
        ssh_cmd.append(f"{self.user}@{ip_address}")
        
        # 添加要执行的命令
        if command:
            ssh_cmd.append(command)
        
        # 显示连接信息
        print(f"连接到: {hostname} ({ip_address})")
        print(f"用户: {self.user}")
        print(f"命令: {' '.join(ssh_cmd)}")
        print("-" * 80)
        
        # 执行SSH连接
        try:
            os.execvp('ssh', ssh_cmd)
        except Exception as e:
            print(f"SSH连接失败: {e}")
            return False
    
    def _is_valid_ip(self, ip_str: str) -> bool:
        """简单验证IP地址格式"""
        parts = ip_str.split('.')
        if len(parts) != 4:
            return False
        try:
            return all(0 <= int(part) <= 255 for part in parts)
        except ValueError:
            return False


def main():
    parser = argparse.ArgumentParser(
        description='SSH连接到虚拟机',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  %(prog)s vm-iarnet-01              # 连接到iarnet节点1
  %(prog)s vm-docker-05               # 连接到docker节点5
  %(prog)s vm-k8s-cluster-01-master  # 连接到K8s集群1的master节点
  %(prog)s 192.168.100.10             # 通过IP地址连接
  %(prog)s --list                     # 列出所有虚拟机
  %(prog)s --list --type iarnet       # 只列出iarnet节点
  %(prog)s vm-iarnet-01 "ls -la"      # 执行命令而不进入交互式shell
        """
    )
    
    parser.add_argument(
        'vm',
        nargs='?',
        help='虚拟机hostname或IP地址'
    )
    
    parser.add_argument(
        'command',
        nargs='?',
        help='要执行的命令（可选，如果不提供则进入交互式shell）'
    )
    
    parser.add_argument(
        '--config', '-c',
        default='./vm-config.yaml',
        help='配置文件路径 (默认: ./vm-config.yaml)'
    )
    
    parser.add_argument(
        '--list', '-l',
        action='store_true',
        help='列出所有虚拟机'
    )
    
    parser.add_argument(
        '--type', '-t',
        choices=['iarnet', 'docker', 'k8s-master', 'k8s-worker'],
        help='过滤虚拟机类型（与--list一起使用）'
    )
    
    parser.add_argument(
        '--port', '-p',
        type=int,
        default=22,
        help='SSH端口 (默认: 22)'
    )
    
    parser.add_argument(
        '--user', '-u',
        help='SSH用户名（覆盖配置文件中的设置）'
    )
    
    parser.add_argument(
        '--ssh-args',
        help='额外的SSH参数（用引号括起来，例如: "-v -o LogLevel=DEBUG"）'
    )
    
    args = parser.parse_args()
    
    try:
        ssh_client = VMSSH(args.config)
        
        # 如果指定了用户，覆盖配置
        if args.user:
            ssh_client.user = args.user
        
        # 列出虚拟机
        if args.list:
            ssh_client.list_vms(vm_type=args.type)
            return
        
        # 需要指定虚拟机
        if not args.vm:
            parser.print_help()
            print("\n错误: 请指定要连接的虚拟机")
            print("使用 'python3 ssh_vm.py --list' 查看所有可用的虚拟机")
            sys.exit(1)
        
        # 解析额外SSH参数
        extra_args = []
        if args.ssh_args:
            import shlex
            extra_args = shlex.split(args.ssh_args)
        
        # 连接虚拟机
        success = ssh_client.ssh_connect(
            args.vm,
            command=args.command,
            port=args.port,
            extra_args=extra_args if extra_args else None
        )
        
        if not success:
            sys.exit(1)
            
    except FileNotFoundError as e:
        print(f"错误: 配置文件不存在: {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()

