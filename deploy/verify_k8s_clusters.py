#!/usr/bin/env python3
"""
验证所有 K8s 集群的初始化状态
通过在每个 master 节点上执行 kubectl get nodes 来检查集群状态
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

class K8sClusterVerifier:
    _log_lock = Lock()
    
    def __init__(self, vm_config_path: str):
        """初始化集群验证器"""
        vm_config_path_obj = Path(vm_config_path)
        if not vm_config_path_obj.is_absolute():
            if (SCRIPT_DIR / vm_config_path_obj.name).exists():
                vm_config_path_obj = SCRIPT_DIR / vm_config_path_obj.name
            else:
                vm_config_path_obj = Path(vm_config_path)
        
        if not vm_config_path_obj.exists():
            raise FileNotFoundError(f"配置文件不存在: {vm_config_path_obj}")
        
        with open(vm_config_path_obj, 'r', encoding='utf-8') as f:
            self.vm_config = yaml.safe_load(f)
        
        k8s_config = self.vm_config['vm_types']['k8s_clusters']
        self.master_config = k8s_config['master']
        self.worker_config = k8s_config['worker']
        self.cluster_count = k8s_config['count']
        self.user = self.vm_config['global']['user']
        
        # 处理 SSH 密钥路径
        ssh_key_config = self.vm_config['global'].get('ssh_key_path', '~/.ssh/id_rsa.pub')
        ssh_key_path = os.path.expanduser(ssh_key_config)
        if ssh_key_path.endswith('.pub'):
            self.ssh_key_path = ssh_key_path[:-4]
        else:
            self.ssh_key_path = ssh_key_path
        
        if not os.path.exists(self.ssh_key_path):
            default_keys = [
                os.path.expanduser('~/.ssh/id_rsa'),
                os.path.expanduser('~/.ssh/id_ed25519'),
            ]
            for key_path in default_keys:
                if os.path.exists(key_path):
                    self.ssh_key_path = key_path
                    break
        
        # 构建集群信息
        self.cluster_info = {}
        for cluster_id in range(self.cluster_count):
            master_ip_suffix = self.master_config['ip_start'] + cluster_id * self.master_config['ip_step']
            master_ip = f"{self.master_config['ip_base']}.{master_ip_suffix}"
            master_hostname = f"{self.master_config['hostname_prefix']}-{cluster_id+1:02d}{self.master_config['hostname_suffix']}"
            
            # Worker 节点信息
            workers = []
            for worker_id in range(1, self.worker_config['count_per_cluster'] + 1):
                worker_ip_suffix = master_ip_suffix + worker_id
                worker_ip = f"{self.master_config['ip_base']}.{worker_ip_suffix}"
                worker_hostname = f"{self.worker_config['hostname_prefix']}-{cluster_id+1:02d}-{self.worker_config['hostname_suffix']}-{worker_id}"
                workers.append({
                    'hostname': worker_hostname,
                    'ip': worker_ip
                })
            
            self.cluster_info[cluster_id] = {
                'master': {
                    'hostname': master_hostname,
                    'ip': master_ip
                },
                'workers': workers
            }
    
    def _print(self, *args, **kwargs):
        """线程安全的打印函数"""
        with self._log_lock:
            kwargs.setdefault('flush', True)
            print(*args, **kwargs)
    
    def _build_ssh_cmd(self, ip: str) -> list:
        """构建 SSH 命令"""
        return [
            'ssh',
            '-i', self.ssh_key_path,
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f'{self.user}@{ip}'
        ]
    
    def check_node_connectivity(self, ip: str) -> bool:
        """检查节点连通性"""
        try:
            result = subprocess.run(
                ['ping', '-c', '1', '-W', '2', ip],
                capture_output=True,
                timeout=5
            )
            return result.returncode == 0
        except:
            return False
    
    def verify_cluster(self, cluster_id: int) -> dict:
        """验证单个集群的状态"""
        cluster = self.cluster_info[cluster_id]
        master = cluster['master']
        cluster_prefix = f"[集群 {cluster_id+1:02d}]"
        
        result = {
            'cluster_id': cluster_id,
            'master_ip': master['ip'],
            'master_hostname': master['hostname'],
            'status': 'unknown',
            'nodes': [],
            'error': None
        }
        
        self._print(f"\n{cluster_prefix} 验证集群状态...")
        self._print(f"{cluster_prefix} Master: {master['hostname']} ({master['ip']})")
        
        # 检查连通性
        if not self.check_node_connectivity(master['ip']):
            result['status'] = 'unreachable'
            result['error'] = '无法连接到 master 节点'
            self._print(f"{cluster_prefix} ✗ 无法连接到 master 节点")
            return result
        
        ssh_cmd = self._build_ssh_cmd(master['ip'])
        
        # 1. 确保 kubectl 已配置（参考 init 中的配置步骤）
        self._print(f"{cluster_prefix}   检查 kubectl 配置...")
        check_kubectl_cmd = '''if [ ! -f /home/ubuntu/.kube/config ]; then
  mkdir -p /home/ubuntu/.kube
  sudo cp -f /etc/kubernetes/admin.conf /home/ubuntu/.kube/config
  sudo chown $(id -u):$(id -g) /home/ubuntu/.kube/config
  chmod 600 /home/ubuntu/.kube/config
fi
if [ ! -f /root/.kube/config ]; then
  sudo mkdir -p /root/.kube
  sudo cp -f /etc/kubernetes/admin.conf /root/.kube/config
  sudo chown root:root /root/.kube/config
  sudo chmod 600 /root/.kube/config
fi
echo "KUBECTL_CONFIGURED"'''
        
        check_kubectl_ssh_cmd = ' '.join(ssh_cmd) + f' "{check_kubectl_cmd}"'
        
        try:
            kubectl_result = subprocess.run(
                check_kubectl_ssh_cmd,
                shell=True,
                check=False,
                timeout=30,
                capture_output=True,
                text=True
            )
            if 'KUBECTL_CONFIGURED' in kubectl_result.stdout:
                self._print(f"{cluster_prefix}   ✓ kubectl 配置完成")
            else:
                self._print(f"{cluster_prefix}   ⚠ kubectl 配置可能失败，但继续...")
        except:
            self._print(f"{cluster_prefix}   ⚠ kubectl 配置检查异常，但继续...")
        
        # 2. 执行 kubectl get nodes
        self._print(f"{cluster_prefix}   获取节点状态...")
        get_nodes_cmd = 'sudo KUBECONFIG=/home/ubuntu/.kube/config kubectl get nodes -o wide'
        get_nodes_ssh_cmd = ' '.join(ssh_cmd) + f' "{get_nodes_cmd}"'
        
        try:
            result_cmd = subprocess.run(
                get_nodes_ssh_cmd,
                shell=True,
                check=False,
                timeout=30,
                capture_output=True,
                text=True
            )
            
            if result_cmd.returncode == 0:
                # 解析节点信息
                lines = result_cmd.stdout.strip().split('\n')
                if len(lines) > 1:  # 有表头
                    for line in lines[1:]:  # 跳过表头
                        if line.strip():
                            parts = line.split()
                            if len(parts) >= 2:
                                node_name = parts[0]
                                node_status = parts[1]
                                node_info = {
                                    'name': node_name,
                                    'status': node_status,
                                    'raw': line.strip()
                                }
                                result['nodes'].append(node_info)
                
                # 统计节点状态
                expected_nodes = 1 + len(cluster['workers'])  # 1 master + workers
                ready_nodes = sum(1 for n in result['nodes'] if 'Ready' in n.get('status', ''))
                
                if len(result['nodes']) == expected_nodes and ready_nodes == expected_nodes:
                    result['status'] = 'healthy'
                    self._print(f"{cluster_prefix} ✓ 集群健康（{ready_nodes}/{expected_nodes} 节点就绪）")
                elif len(result['nodes']) == expected_nodes:
                    result['status'] = 'partial'
                    self._print(f"{cluster_prefix} ⚠ 集群部分就绪（{ready_nodes}/{expected_nodes} 节点就绪）")
                else:
                    result['status'] = 'incomplete'
                    self._print(f"{cluster_prefix} ⚠ 集群不完整（{len(result['nodes'])}/{expected_nodes} 节点）")
                
                # 显示节点详情
                self._print(f"{cluster_prefix}   节点列表:")
                for node in result['nodes']:
                    status_icon = "✓" if "Ready" in node['status'] else "⚠"
                    self._print(f"{cluster_prefix}     {status_icon} {node['name']}: {node['status']}")
                
            else:
                error_msg = (result_cmd.stderr or result_cmd.stdout or '')[:200]
                error_lines = [line for line in error_msg.split('\n') 
                              if line.strip() and 'Warning:' not in line and 'Permanently added' not in line]
                result['status'] = 'error'
                result['error'] = error_lines[0] if error_lines else 'kubectl get nodes 失败'
                self._print(f"{cluster_prefix} ✗ 获取节点状态失败")
                if error_lines:
                    self._print(f"{cluster_prefix}     错误: {error_lines[0]}")
                    
        except subprocess.TimeoutExpired:
            result['status'] = 'timeout'
            result['error'] = '获取节点状态超时'
            self._print(f"{cluster_prefix} ✗ 获取节点状态超时")
        except Exception as e:
            result['status'] = 'error'
            result['error'] = str(e)
            self._print(f"{cluster_prefix} ✗ 获取节点状态异常: {e}")
        
        return result
    
    def verify_all_clusters(self, cluster_ids: list = None, max_workers: int = 5) -> dict:
        """验证所有或指定集群
        
        Args:
            cluster_ids: 要验证的集群 ID 列表，如果为 None 则验证所有集群
            max_workers: 最大并发线程数
        
        Returns:
            dict: 验证结果
        """
        if cluster_ids is None:
            cluster_ids = list(range(self.cluster_count))
        
        self._print(f"开始验证 {len(cluster_ids)} 个 K8s 集群...")
        self._print("=" * 60)
        
        results = {}
        
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            future_to_cluster = {
                executor.submit(self.verify_cluster, cluster_id): cluster_id
                for cluster_id in cluster_ids
            }
            
            for future in as_completed(future_to_cluster):
                cluster_id = future_to_cluster[future]
                try:
                    result = future.result()
                    results[cluster_id] = result
                except Exception as e:
                    self._print(f"[集群 {cluster_id+1:02d}] ✗ 验证异常: {e}")
                    results[cluster_id] = {
                        'cluster_id': cluster_id,
                        'status': 'error',
                        'error': str(e)
                    }
        
        # 输出总结
        self._print("\n" + "=" * 60)
        self._print("集群验证完成")
        
        healthy_count = sum(1 for r in results.values() if r.get('status') == 'healthy')
        partial_count = sum(1 for r in results.values() if r.get('status') == 'partial')
        incomplete_count = sum(1 for r in results.values() if r.get('status') == 'incomplete')
        error_count = sum(1 for r in results.values() if r.get('status') in ['error', 'timeout', 'unreachable'])
        
        self._print(f"  健康: {healthy_count} 个集群")
        self._print(f"  部分就绪: {partial_count} 个集群")
        self._print(f"  不完整: {incomplete_count} 个集群")
        self._print(f"  错误/超时: {error_count} 个集群")
        
        if error_count > 0:
            self._print(f"\n有问题的集群:")
            for cluster_id, result in sorted(results.items()):
                if result.get('status') in ['error', 'timeout', 'unreachable']:
                    self._print(f"  集群 {cluster_id+1:02d} ({result.get('master_ip', 'N/A')}): {result.get('error', '未知错误')}")
        
        return {
            'results': results,
            'summary': {
                'total': len(results),
                'healthy': healthy_count,
                'partial': partial_count,
                'incomplete': incomplete_count,
                'error': error_count
            }
        }


def parse_cluster_range(cluster_range: str) -> list:
    """解析集群范围字符串"""
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
        description='验证 K8s 集群初始化状态',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 验证所有集群
  python3 verify_k8s_clusters.py

  # 验证指定范围的集群
  python3 verify_k8s_clusters.py --clusters 1-10

  # 验证指定集群
  python3 verify_k8s_clusters.py --clusters 1,2,3

  # 指定配置文件
  python3 verify_k8s_clusters.py --vm-config /path/to/vm-config.yaml

  # 调整并发数
  python3 verify_k8s_clusters.py --max-workers 10
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
        default=5,
        help='最大并发线程数（默认: 5）'
    )
    
    args = parser.parse_args()
    
    try:
        verifier = K8sClusterVerifier(args.vm_config)
        
        cluster_ids = None
        if args.clusters:
            # 转换为 0-based 索引
            cluster_ids_raw = parse_cluster_range(args.clusters)
            cluster_ids = [cid - 1 for cid in cluster_ids_raw]  # 转换为 0-based
            print(f"将验证以下集群: {cluster_ids_raw}")
        
        results = verifier.verify_all_clusters(cluster_ids=cluster_ids, max_workers=args.max_workers)
        
        # 根据结果设置退出码
        if results['summary']['error'] > 0 or results['summary']['incomplete'] > 0:
            sys.exit(1)
        else:
            sys.exit(0)
            
    except KeyboardInterrupt:
        print("\n\n操作被用户中断")
        sys.exit(130)
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()

