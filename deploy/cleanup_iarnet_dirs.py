#!/usr/bin/env python3
"""
清理所有 iarnet 虚拟机上的 iarnet 目录
用于清除从基础镜像中继承的 iarnet-01 数据
"""

import os
import sys
import yaml
import argparse
import subprocess
import time
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
from threading import Lock

# 获取脚本所在目录
SCRIPT_DIR = Path(__file__).parent.absolute()

class IarnetDirCleaner:
    def __init__(self, config_path: str):
        """初始化清理器"""
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
        self.user = self.config['global']['user']
        
        # 处理 SSH 密钥路径
        ssh_key_config = self.config['global'].get('ssh_key_path', '~/.ssh/id_rsa.pub')
        ssh_key_path = os.path.expanduser(ssh_key_config)
        # 将公钥路径转换为私钥路径（去掉 .pub 后缀）
        if ssh_key_path.endswith('.pub'):
            self.ssh_key_path = ssh_key_path[:-4]  # 去掉 .pub
        else:
            self.ssh_key_path = ssh_key_path
        
        # 如果私钥文件不存在，尝试常见的默认位置
        if not os.path.exists(self.ssh_key_path):
            default_keys = [
                os.path.expanduser('~/.ssh/id_rsa'),
                os.path.expanduser('~/.ssh/id_ed25519'),
                os.path.expanduser('~/.ssh/id_ecdsa'),
            ]
            for key_path in default_keys:
                if os.path.exists(key_path):
                    self.ssh_key_path = key_path
                    break
            else:
                # 如果都找不到，使用配置的路径（可能后续会失败，但至少尝试）
                self.ssh_key_path = ssh_key_path
        
        self._log_lock = Lock()
    
    def _print(self, *args, **kwargs):
        """线程安全的打印函数"""
        with self._log_lock:
            kwargs.setdefault('flush', True)
            print(*args, **kwargs)
    
    def check_node_connectivity(self, node_ip: str) -> bool:
        """检查节点连通性"""
        try:
            result = subprocess.run(
                ['ping', '-c', '1', '-W', '2', node_ip],
                capture_output=True,
                timeout=5
            )
            return result.returncode == 0
        except:
            return False
    
    def cleanup_node(self, node_id: int, node_ip: str, hostname: str, node_type: str = 'iarnet') -> bool:
        """清理单个节点的 iarnet 目录"""
        node_prefix = f"[{node_type}节点 {node_id}] {hostname}"
        
        # 检查连通性
        if not self.check_node_connectivity(node_ip):
            self._print(f"{node_prefix} ✗ 无法连接到节点 ({node_ip})")
            return False
        
        # 构建 SSH 命令（包含 SSH 密钥）
        ssh_cmd = [
            'ssh',
            '-i', self.ssh_key_path,
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f"{self.user}@{node_ip}"
        ]
        
        # 清理 iarnet 目录的命令
        # 使用 sudo 删除，如果目录不存在也不报错
        cleanup_cmd = ' '.join(ssh_cmd) + ' "sudo rm -rf ~/iarnet 2>/dev/null; test ! -d ~/iarnet && echo CLEANUP_SUCCESS || echo CLEANUP_FAILED"'
        
        try:
            self._print(f"{node_prefix} 清理 iarnet 目录...")
            result = subprocess.run(
                cleanup_cmd,
                shell=True,
                check=False,
                timeout=30,
                capture_output=True,
                text=True
            )
            
            # 合并 stdout 和 stderr
            full_output = (result.stdout or '') + (result.stderr or '')
            
            # 检查输出中是否有成功标记
            if 'CLEANUP_SUCCESS' in full_output:
                self._print(f"{node_prefix} ✓ 清理成功")
                return True
            
            # 如果返回码为 0，即使没有成功标记，也验证目录是否真的不存在
            if result.returncode == 0:
                # 再次检查目录是否存在
                check_cmd = ' '.join(ssh_cmd) + ' "test ! -d ~/iarnet && echo NOT_EXISTS || echo EXISTS"'
                check_result = subprocess.run(
                    check_cmd,
                    shell=True,
                    check=False,
                    timeout=5,
                    capture_output=True,
                    text=True
                )
                
                if 'NOT_EXISTS' in check_result.stdout:
                    self._print(f"{node_prefix} ✓ 清理成功（目录已不存在）")
                    return True
                else:
                    self._print(f"{node_prefix} ✗ 清理失败：目录仍存在")
                    return False
            
            # 返回码非 0，提取错误信息
            error_lines = []
            for line in full_output.split('\n'):
                line = line.strip()
                if line and 'CLEANUP' not in line and 'Permanently added' not in line and 'Warning:' not in line:
                    error_lines.append(line)
            
            if error_lines:
                error_msg = ' '.join(error_lines[:2])
                self._print(f"{node_prefix} ✗ 清理失败: {error_msg}")
            else:
                self._print(f"{node_prefix} ✗ 清理失败: SSH 命令返回码 {result.returncode}")
            
            return False
                
        except subprocess.TimeoutExpired:
            self._print(f"{node_prefix} ✗ 清理超时")
            return False
        except Exception as e:
            self._print(f"{node_prefix} ✗ 清理异常: {e}")
            return False
    
    def cleanup_iarnet_nodes(self, max_workers: int = 10) -> bool:
        """清理所有 iarnet 节点的 iarnet 目录"""
        iarnet_config = self.vm_types.get('iarnet', {})
        if not iarnet_config:
            self._print("配置文件中未找到 iarnet 配置")
            return False
        
        count = iarnet_config.get('count', 0)
        ip_base = iarnet_config.get('ip_base', '192.168.100')
        ip_start = iarnet_config.get('ip_start', 10)
        hostname_prefix = iarnet_config.get('hostname_prefix', 'vm-iarnet')
        
        self._print("=" * 60)
        self._print(f"清理 {count} 个 iarnet 节点的 iarnet 目录")
        self._print("=" * 60)
        
        # 准备节点列表
        nodes = []
        for i in range(count):
            node_id = i
            ip_suffix = ip_start + i
            ip_address = f"{ip_base}.{ip_suffix}"
            hostname = f"{hostname_prefix}-{i+1:02d}"
            nodes.append((node_id, ip_address, hostname))
        
        if not nodes:
            self._print("没有找到需要清理的节点")
            return True
        
        # 并行清理
        success_count = 0
        failed_nodes = []
        
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            future_to_node = {
                executor.submit(self.cleanup_node, node_id, ip, hostname, 'iarnet'): (node_id, hostname)
                for node_id, ip, hostname in nodes
            }
            
            for future in as_completed(future_to_node):
                node_id, hostname = future_to_node[future]
                try:
                    result = future.result()
                    if result:
                        success_count += 1
                    else:
                        failed_nodes.append((node_id, hostname))
                except Exception as e:
                    failed_nodes.append((node_id, hostname))
                    self._print(f"[iarnet节点 {node_id}] {hostname} ✗ 清理异常: {e}")
        
        # 输出结果
        self._print("\n" + "=" * 60)
        self._print(f"清理完成: {success_count}/{len(nodes)} 个节点成功")
        if failed_nodes:
            self._print(f"失败的节点: {[f'{hostname}({node_id})' for node_id, hostname in failed_nodes]}")
        self._print("=" * 60)
        
        return success_count == len(nodes)
    
    def cleanup_docker_nodes(self, max_workers: int = 10) -> bool:
        """清理所有 docker provider 节点的 iarnet 目录"""
        docker_config = self.vm_types.get('docker', {})
        if not docker_config:
            self._print("配置文件中未找到 docker 配置")
            return False
        
        count = docker_config.get('count', 0)
        ip_base = docker_config.get('ip_base', '192.168.100')
        ip_start = docker_config.get('ip_start', 50)
        hostname_prefix = docker_config.get('hostname_prefix', 'vm-docker')
        
        self._print("=" * 60)
        self._print(f"清理 {count} 个 docker provider 节点的 iarnet 目录")
        self._print("=" * 60)
        
        # 准备节点列表
        nodes = []
        for i in range(count):
            node_id = i
            ip_suffix = ip_start + i
            ip_address = f"{ip_base}.{ip_suffix}"
            hostname = f"{hostname_prefix}-{i+1:02d}"
            nodes.append((node_id, ip_address, hostname))
        
        if not nodes:
            self._print("没有找到需要清理的节点")
            return True
        
        # 并行清理
        success_count = 0
        failed_nodes = []
        
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            future_to_node = {
                executor.submit(self.cleanup_node, node_id, ip, hostname, 'docker'): (node_id, hostname)
                for node_id, ip, hostname in nodes
            }
            
            for future in as_completed(future_to_node):
                node_id, hostname = future_to_node[future]
                try:
                    result = future.result()
                    if result:
                        success_count += 1
                    else:
                        failed_nodes.append((node_id, hostname))
                except Exception as e:
                    failed_nodes.append((node_id, hostname))
                    self._print(f"[docker节点 {node_id}] {hostname} ✗ 清理异常: {e}")
        
        # 输出结果
        self._print("\n" + "=" * 60)
        self._print(f"清理完成: {success_count}/{len(nodes)} 个节点成功")
        if failed_nodes:
            self._print(f"失败的节点: {[f'{hostname}({node_id})' for node_id, hostname in failed_nodes]}")
        self._print("=" * 60)
        
        return success_count == len(nodes)
    
    def cleanup_all_nodes(self, max_workers: int = 10) -> bool:
        """清理所有节点的 iarnet 目录"""
        self._print("=" * 60)
        self._print("清理所有节点的 iarnet 目录")
        self._print("=" * 60)
        
        iarnet_success = self.cleanup_iarnet_nodes(max_workers=max_workers)
        docker_success = self.cleanup_docker_nodes(max_workers=max_workers)
        
        return iarnet_success and docker_success

def main():
    parser = argparse.ArgumentParser(description='清理所有虚拟机上的 iarnet 目录')
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--node-type', '-t',
        type=str,
        choices=['iarnet', 'docker', 'all'],
        default='all',
        help='要清理的节点类型 (默认: all)'
    )
    parser.add_argument(
        '--max-workers', '-w',
        type=int,
        default=10,
        help='最大并发数 (默认: 10)'
    )
    
    args = parser.parse_args()
    
    try:
        cleaner = IarnetDirCleaner(args.vm_config)
        
        if args.node_type == 'iarnet':
            cleaner.cleanup_iarnet_nodes(max_workers=args.max_workers)
        elif args.node_type == 'docker':
            cleaner.cleanup_docker_nodes(max_workers=args.max_workers)
        else:
            cleaner.cleanup_all_nodes(max_workers=args.max_workers)
        
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)

if __name__ == '__main__':
    main()

