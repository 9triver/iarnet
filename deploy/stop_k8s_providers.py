#!/usr/bin/env python3
"""
停止所有 K8s Provider 节点上的 provider 进程
"""

import os
import sys
import yaml
import argparse
import subprocess
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
from threading import Lock

# 获取脚本所在目录
SCRIPT_DIR = Path(__file__).parent.absolute()

class K8sProviderStopper:
    def __init__(self, config_path: str):
        """初始化 Provider 停止器"""
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
        
        k8s_config = self.config['vm_types']['k8s_clusters']
        self.master_config = k8s_config['master']
        self.cluster_count = k8s_config['count']
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
    
    def check_cluster_connectivity(self, cluster_id: int, ip: str) -> bool:
        """检查集群 master 节点连通性"""
        try:
            result = subprocess.run(
                ['ping', '-c', '1', '-W', '2', ip],
                capture_output=True,
                timeout=5
            )
            return result.returncode == 0
        except:
            return False
    
    def stop_provider_process(self, cluster_id: int) -> bool:
        """停止指定集群 master 节点上的 provider 进程"""
        ip_suffix = self.master_config['ip_start'] + cluster_id * self.master_config['ip_step']
        ip_address = f"{self.master_config['ip_base']}.{ip_suffix}"
        hostname = f"{self.master_config['hostname_prefix']}-{cluster_id+1:02d}{self.master_config['hostname_suffix']}"
        
        cluster_prefix = f"[集群 {cluster_id+1:02d}] {hostname}"
        
        # 检查连通性
        if not self.check_cluster_connectivity(cluster_id, ip_address):
            self._print(f"{cluster_prefix} ✗ 无法连接到节点 ({ip_address})")
            return False
        
        # 构建 SSH 命令（包含 SSH 密钥）
        ssh_cmd = [
            'ssh',
            '-i', self.ssh_key_path,
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f'{self.user}@{ip_address}'
        ]
        
        # 停止 provider 进程的命令
        # 使用多种方式确保进程被停止，按顺序执行
        self._print(f"{cluster_prefix} 停止 k8s provider 进程...")
        
        # 首先获取进程 PID（用于调试）
        # 使用更精确的匹配，查找实际运行的 k8s-provider 进程
        get_pid_cmd = ' '.join(ssh_cmd) + ' "ps aux | grep -v grep | grep -E \"k8s-provider.*config\" | awk \'{print $2}\' | head -1"'
        try:
            pid_result = subprocess.run(
                get_pid_cmd,
                shell=True,
                check=False,
                timeout=5,
                capture_output=True,
                text=True
            )
            pid = pid_result.stdout.strip()
            if pid and pid.isdigit():
                self._print(f"{cluster_prefix}   发现进程 PID: {pid}")
        except:
            pass
        
        # 执行停止命令（使用更强的信号）
        # 使用更精确的匹配，匹配包含 config 的 k8s-provider 进程（实际运行的进程）
        stop_commands = [
            'pkill -9 -f "k8s-provider.*config"',  # 强制杀死（匹配实际运行的进程）
            'pkill -9 -f k8s-provider',  # 备用：匹配所有 k8s-provider
            'killall -9 k8s-provider 2>/dev/null',  # 备用方式
            'pgrep -f "k8s-provider.*config" | xargs -r kill -9 2>/dev/null',  # 通过 PID 杀死
        ]
        
        for cmd in stop_commands:
            stop_cmd = ' '.join(ssh_cmd) + f' "{cmd} 2>&1 || true"'
            try:
                result = subprocess.run(
                    stop_cmd,
                    shell=True,
                    check=False,
                    timeout=10,
                    capture_output=True,
                    text=True
                )
                # 输出命令执行结果（用于调试）
                if result.stdout.strip() or result.stderr.strip():
                    output = (result.stdout + result.stderr).strip()
                    if output and 'No such process' not in output:
                        self._print(f"{cluster_prefix}   执行: {cmd[:50]}... 输出: {output[:100]}")
            except subprocess.TimeoutExpired:
                self._print(f"{cluster_prefix} ⚠ 停止命令超时: {cmd[:50]}...")
            except Exception as e:
                self._print(f"{cluster_prefix} ⚠ 停止命令执行异常: {e}")
        
        # 等待一小段时间让进程完全终止
        import time
        time.sleep(1)
        
                # 检查是否还有进程在运行（多次检查确保准确）
        max_checks = 3
        stopped = False
        for i in range(max_checks):
            # 使用更精确的检查：直接检查进程文件路径，排除 grep 和脚本本身
            # 查找 ~/k8s-provider/k8s-provider 进程
            check_cmd = ' '.join(ssh_cmd) + ' "ps aux | grep -v grep | grep -E \"k8s-provider.*config\" | grep -v \"stop_k8s_providers\" >/dev/null 2>&1 && echo RUNNING || echo STOPPED"'
            try:
                check_result = subprocess.run(
                    check_cmd,
                    shell=True,
                    check=False,
                    timeout=5,
                    capture_output=True,
                    text=True
                )
                
                full_output = (check_result.stdout or '') + (check_result.stderr or '')
                
                if 'STOPPED' in check_result.stdout:
                    stopped = True
                    break
                elif 'RUNNING' in check_result.stdout:
                    # 如果还在运行，再尝试一次强制杀死
                    if i < max_checks - 1:
                        self._print(f"{cluster_prefix}   进程仍在运行，再次尝试停止...")
                        # 使用更精确的 kill 命令，直接通过进程路径匹配
                        final_kill_cmd = ' '.join(ssh_cmd) + ' "pkill -9 -f \"k8s-provider.*config\" 2>&1 || true"'
                        subprocess.run(final_kill_cmd, shell=True, check=False, timeout=5, capture_output=True)
                        time.sleep(1)  # 增加等待时间
            except subprocess.TimeoutExpired:
                self._print(f"{cluster_prefix} ⚠ 检查命令超时")
            except Exception as e:
                self._print(f"{cluster_prefix} ⚠ 检查命令执行异常: {e}")
        
        if stopped:
            self._print(f"{cluster_prefix} ✓ k8s provider 进程已停止")
            return True
        else:
            # 最后一次详细检查（排除 grep 命令本身）
            # 使用更精确的检查：查找实际的 k8s-provider 二进制文件进程
            final_check_cmd = ' '.join(ssh_cmd) + ' "ps aux | grep -v grep | grep k8s-provider || echo NO_PROCESS"'
            try:
                final_result = subprocess.run(
                    final_check_cmd,
                    shell=True,
                    check=False,
                    timeout=5,
                    capture_output=True,
                    text=True
                )
                
                output = final_result.stdout.strip()
                
                # 过滤掉 grep 命令本身和空行
                lines = [line for line in output.split('\n') if line.strip() and 'grep' not in line]
                
                if 'NO_PROCESS' in output or not lines:
                    self._print(f"{cluster_prefix} ✓ k8s provider 进程已停止（最终确认）")
                    return True
                else:
                    # 显示仍在运行的进程信息（排除 grep）
                    process_info = '\n'.join(lines)[:200]
                    self._print(f"{cluster_prefix} ✗ k8s provider 进程仍在运行")
                    self._print(f"{cluster_prefix}   进程信息: {process_info}")
                    return False
            except:
                self._print(f"{cluster_prefix} ✗ 无法确认进程状态")
                return False
    
    def stop_all_providers(self, cluster_ids: list = None, max_workers: int = 10) -> dict:
        """停止所有或指定集群的 provider 进程
        
        Args:
            cluster_ids: 要停止的集群 ID 列表，如果为 None 则停止所有集群
            max_workers: 最大并发线程数
        
        Returns:
            dict: {'success': [...], 'failed': [...]}
        """
        if cluster_ids is None:
            cluster_ids = list(range(self.cluster_count))
        
        self._print(f"开始停止 {len(cluster_ids)} 个集群的 k8s provider 进程...")
        self._print("=" * 60)
        
        success_clusters = []
        failed_clusters = []
        
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            # 提交所有任务
            future_to_cluster = {
                executor.submit(self.stop_provider_process, cluster_id): cluster_id
                for cluster_id in cluster_ids
            }
            
            # 收集结果
            for future in as_completed(future_to_cluster):
                cluster_id = future_to_cluster[future]
                try:
                    success = future.result()
                    if success:
                        success_clusters.append(cluster_id)
                    else:
                        failed_clusters.append(cluster_id)
                except Exception as e:
                    self._print(f"[集群 {cluster_id+1:02d}] ✗ 处理异常: {e}")
                    failed_clusters.append(cluster_id)
        
        # 输出总结
        self._print("\n" + "=" * 60)
        self._print("停止操作完成")
        self._print(f"  成功: {len(success_clusters)} 个集群")
        self._print(f"  失败: {len(failed_clusters)} 个集群")
        
        if failed_clusters:
            self._print(f"\n失败的集群: {[c+1 for c in failed_clusters]}")
        
        return {
            'success': success_clusters,
            'failed': failed_clusters
        }


def parse_cluster_range(cluster_range: str) -> list:
    """解析集群范围字符串
    
    支持格式:
    - "0-39" -> [0, 1, ..., 39]
    - "0,1,2" -> [0, 1, 2]
    - "0-5,10-15" -> [0,1,2,3,4,5,10,11,12,13,14,15]
    """
    cluster_ids = []
    parts = cluster_range.split(',')
    for part in parts:
        part = part.strip()
        if '-' in part:
            start, end = part.split('-', 1)
            cluster_ids.extend(range(int(start), int(end) + 1))
        else:
            cluster_ids.append(int(part))
    return sorted(set(cluster_ids))


def main():
    parser = argparse.ArgumentParser(
        description='停止所有 K8s Provider 节点上的 provider 进程',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 停止所有集群的 provider
  python3 stop_k8s_providers.py

  # 停止指定范围的集群
  python3 stop_k8s_providers.py --clusters 0-9

  # 停止指定集群
  python3 stop_k8s_providers.py --clusters 0,1,2

  # 指定配置文件
  python3 stop_k8s_providers.py --vm-config /path/to/vm-config.yaml
        """
    )
    
    parser.add_argument(
        '--vm-config',
        default='vm-config.yaml',
        help='虚拟机配置文件路径（默认: vm-config.yaml）'
    )
    
    parser.add_argument(
        '--clusters',
        help='集群范围，格式: start-end 或逗号分隔的列表 (默认: 所有集群)'
    )
    
    parser.add_argument(
        '--max-workers',
        type=int,
        default=10,
        help='最大并发线程数（默认: 10）'
    )
    
    args = parser.parse_args()
    
    try:
        stopper = K8sProviderStopper(args.vm_config)
        
        cluster_ids = None
        if args.clusters:
            cluster_ids = parse_cluster_range(args.clusters)
            print(f"将停止以下集群的 provider: {[c+1 for c in cluster_ids]}")
        
        results = stopper.stop_all_providers(cluster_ids=cluster_ids, max_workers=args.max_workers)
        
        # 根据结果设置退出码
        if results['failed']:
            sys.exit(1)
        else:
            sys.exit(0)
            
    except KeyboardInterrupt:
        print("\n\n操作被用户中断")
        sys.exit(130)
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()

