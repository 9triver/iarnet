#!/usr/bin/env python3
"""
停止并删除所有 iarnet 虚拟机中的 iarnet 容器
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

class IarnetContainerStopper:
    def __init__(self, config_path: str):
        """初始化容器停止器"""
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
    
    def check_docker_installed(self, ssh_cmd: list) -> bool:
        """检查 Docker 是否安装"""
        check_cmd = ' '.join(ssh_cmd) + ' "docker --version >/dev/null 2>&1 && echo DOCKER_OK || echo DOCKER_NOT_FOUND"'
        try:
            result = subprocess.run(check_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
            return 'DOCKER_OK' in result.stdout
        except:
            return False
    
    def stop_and_remove_containers(self, node_id: int, node_ip: str, hostname: str) -> bool:
        """停止并删除单个节点上的 iarnet 容器"""
        node_prefix = f"[iarnet节点 {node_id}] {hostname}"
        
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
        
        # 检查 Docker 是否安装
        if not self.check_docker_installed(ssh_cmd):
            self._print(f"{node_prefix} ⚠ Docker 未安装，跳过")
            return True  # 不算错误，只是跳过
        
        # 停止并删除所有容器（使用 xargs 避免命令替换问题）
        try:
            # 先获取所有容器 ID
            get_containers_cmd = ' '.join(ssh_cmd) + ' "docker ps -aq 2>/dev/null"'
            get_result = subprocess.run(
                get_containers_cmd,
                shell=True,
                check=False,
                timeout=10,
                capture_output=True,
                text=True
            )
            
            container_ids = [cid.strip() for cid in get_result.stdout.strip().split('\n') if cid.strip()]
            
            if not container_ids:
                self._print(f"{node_prefix} ✓ 节点上无容器")
                return True
            
            self._print(f"{node_prefix} 找到 {len(container_ids)} 个容器，开始删除...")
            
            # 使用 xargs 强制删除所有容器（会先停止再删除）
            rm_all_cmd = ' '.join(ssh_cmd) + ' "docker ps -aq | xargs -r docker rm -f 2>/dev/null || true"'
            rm_result = subprocess.run(
                rm_all_cmd,
                shell=True,
                check=False,
                timeout=60,
                capture_output=True,
                text=True
            )
            
            # 验证是否还有容器残留
            check_cmd = ' '.join(ssh_cmd) + ' "docker ps -aq 2>/dev/null | wc -l"'
            check_result = subprocess.run(
                check_cmd,
                shell=True,
                check=False,
                timeout=5,
                capture_output=True,
                text=True
            )
            
            remaining = int(check_result.stdout.strip()) if check_result.stdout.strip().isdigit() else -1
            
            if remaining == 0:
                self._print(f"{node_prefix} ✓ 所有容器已删除")
                return True
            else:
                self._print(f"{node_prefix} ⚠ 仍有 {remaining} 个容器残留")
                return False
                
        except subprocess.TimeoutExpired:
            self._print(f"{node_prefix} ✗ 操作超时")
            return False
        except Exception as e:
            self._print(f"{node_prefix} ✗ 操作异常: {e}")
            return False
    
    def stop_all_containers(self, max_workers: int = 10) -> bool:
        """停止并删除所有 iarnet 节点上的所有容器"""
        iarnet_config = self.vm_types.get('iarnet', {})
        if not iarnet_config:
            self._print("配置文件中未找到 iarnet 配置")
            return False
        
        count = iarnet_config.get('count', 0)
        ip_base = iarnet_config.get('ip_base', '192.168.100')
        ip_start = iarnet_config.get('ip_start', 10)
        hostname_prefix = iarnet_config.get('hostname_prefix', 'vm-iarnet')
        
        self._print("=" * 60)
        self._print(f"停止并删除 {count} 个 iarnet 节点上的所有容器")
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
            self._print("没有找到需要处理的节点")
            return True
        
        # 确认操作（如果 skip_confirm 为 False）
        if not getattr(self, 'skip_confirm', False):
            self._print(f"\n将处理以下 {len(nodes)} 个节点:")
            for node_id, ip, hostname in nodes[:10]:  # 只显示前10个
                self._print(f"  - {hostname} ({ip})")
            if len(nodes) > 10:
                self._print(f"  ... 还有 {len(nodes) - 10} 个节点")
            
            response = input("\n是否继续？(yes/no): ")
            if response.lower() not in ['yes', 'y']:
                self._print("已取消")
                return False
        
        # 并行处理
        success_count = 0
        failed_nodes = []
        skipped_nodes = []
        
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            future_to_node = {
                executor.submit(self.stop_and_remove_containers, node_id, ip, hostname): (node_id, hostname)
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
                    self._print(f"[iarnet节点 {node_id}] {hostname} ✗ 处理异常: {e}")
        
        # 输出结果
        self._print("\n" + "=" * 60)
        self._print(f"处理完成: {success_count}/{len(nodes)} 个节点成功")
        if failed_nodes:
            self._print(f"失败的节点: {[f'{hostname}({node_id})' for node_id, hostname in failed_nodes]}")
        self._print("=" * 60)
        
        return success_count == len(nodes)

def main():
    parser = argparse.ArgumentParser(description='停止并删除所有 iarnet 虚拟机中的所有容器')
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--max-workers', '-w',
        type=int,
        default=10,
        help='最大并发数 (默认: 10)'
    )
    parser.add_argument(
        '--yes', '-y',
        action='store_true',
        help='跳过确认提示，直接执行'
    )
    
    args = parser.parse_args()
    
    try:
        stopper = IarnetContainerStopper(args.vm_config)
        
        # 如果指定了 --yes，跳过确认
        if args.yes:
            stopper.skip_confirm = True
        
        stopper.stop_all_containers(max_workers=args.max_workers)
        
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)

if __name__ == '__main__':
    main()

