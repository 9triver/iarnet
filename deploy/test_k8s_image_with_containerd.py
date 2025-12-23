#!/usr/bin/env python3
"""
在 K8s master 节点上使用 containerd 创建容器并获取日志，验证镜像是否可以正常运行
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
PROJECT_ROOT = SCRIPT_DIR.parent

class ContainerdContainerTester:
    def __init__(self, vm_config_path: str):
        """初始化测试器"""
        # 处理vm_config_path
        vm_config_path_obj = Path(vm_config_path)
        if not vm_config_path_obj.is_absolute():
            if (SCRIPT_DIR / vm_config_path_obj.name).exists():
                vm_config_path_obj = SCRIPT_DIR / vm_config_path_obj.name
            elif (PROJECT_ROOT / vm_config_path).exists():
                vm_config_path_obj = PROJECT_ROOT / vm_config_path
            else:
                vm_config_path_obj = Path(vm_config_path)
        
        if not vm_config_path_obj.exists():
            raise FileNotFoundError(f"虚拟机配置文件不存在: {vm_config_path_obj}")
        
        with open(vm_config_path_obj, 'r', encoding='utf-8') as f:
            self.vm_config = yaml.safe_load(f)
        
        self.user = self.vm_config['global']['user']
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
    
    def test_image_on_cluster(self, cluster_id: int, cluster_info: dict, 
                              image_name: str, command: str = None, 
                              container_name: str = None) -> dict:
        """在指定集群的 master 节点上测试镜像"""
        node_prefix = f"[K8s集群 {cluster_id}] {cluster_info['hostname']} ({cluster_info['ip']}) "
        
        # 检查连通性
        if not self.check_node_connectivity(cluster_info['ip']):
            self._print(f"{node_prefix}⚠ 无法连接，跳过")
            return {'success': False, 'error': '无法连接'}
        
        ssh_cmd = [
            'ssh',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f"{self.user}@{cluster_info['ip']}"
        ]
        
        # 检查 containerd
        check_containerd_cmd = ' '.join(ssh_cmd) + ' "sudo ctr version >/dev/null 2>&1 && echo OK || echo NOT_INSTALLED"'
        try:
            result = subprocess.run(check_containerd_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            if 'OK' not in result.stdout:
                self._print(f"{node_prefix}⚠ containerd 未安装，跳过")
                return {'success': False, 'error': 'containerd 未安装'}
        except:
            self._print(f"{node_prefix}⚠ 无法检查 containerd，跳过")
            return {'success': False, 'error': '无法检查 containerd'}
        
        # 检查镜像是否存在
        self._print(f"{node_prefix}检查镜像是否存在: {image_name}")
        check_image_cmd = ' '.join(ssh_cmd) + f' "sudo ctr -n k8s.io images ls | grep -E \\"^{image_name}\\\\s|\\\\s{image_name}\\\\s|\\\\s{image_name}$\\" && echo EXISTS || echo NOT_FOUND"'
        check_image_result = subprocess.run(check_image_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
        
        if 'NOT_FOUND' in check_image_result.stdout:
            self._print(f"{node_prefix}  ✗ 镜像不存在: {image_name}")
            return {'success': False, 'error': f'镜像不存在: {image_name}'}
        
        self._print(f"{node_prefix}  ✓ 镜像存在")
        
        # 生成容器名称
        if container_name is None:
            container_name = f"test-{image_name.replace('/', '-').replace(':', '-')}-{cluster_id}"
            # 清理容器名称中的特殊字符
            container_name = ''.join(c if c.isalnum() or c in '-_' else '-' for c in container_name)
        
        # 清理可能存在的旧容器
        self._print(f"{node_prefix}清理旧容器（如果存在）...")
        cleanup_cmd = ' '.join(ssh_cmd) + f' "sudo ctr -n k8s.io containers delete {container_name} 2>/dev/null || sudo ctr -n k8s.io tasks kill {container_name} 2>/dev/null || true"'
        subprocess.run(cleanup_cmd, shell=True, check=False, timeout=10, capture_output=True)
        
        # 创建容器
        self._print(f"{node_prefix}创建容器: {container_name}")
        if command is None:
            # 默认命令：运行一个简单的测试命令
            command = "echo 'Container is running successfully' && sleep 10"
        
        # 使用 ctr 创建容器
        # ctr run 命令格式: ctr run [flags] <image-ref> <container-id> <command>
        # 注意：如果需要设置环境变量，使用 --env 参数
        # 示例：--env KEY=value --env KEY2=value2
        create_cmd = ' '.join(ssh_cmd) + f' "sudo ctr -n k8s.io run --rm -d {image_name} {container_name} sh -c \\"{command}\\" 2>&1"'
        
        try:
            create_result = subprocess.run(create_cmd, shell=True, check=False, timeout=30, capture_output=True, text=True)
            if create_result.returncode != 0:
                error_msg = create_result.stderr if create_result.stderr else create_result.stdout
                self._print(f"{node_prefix}  ✗ 容器创建失败: {error_msg[:200]}")
                return {'success': False, 'error': f'容器创建失败: {error_msg[:100]}'}
            
            self._print(f"{node_prefix}  ✓ 容器已创建")
            
            # 等待容器运行一段时间
            time.sleep(2)
            
            # 检查容器状态
            self._print(f"{node_prefix}检查容器状态...")
            status_cmd = ' '.join(ssh_cmd) + f' "sudo ctr -n k8s.io tasks ls | grep {container_name} || echo NOT_RUNNING"'
            status_result = subprocess.run(status_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            
            if 'NOT_RUNNING' in status_result.stdout:
                self._print(f"{node_prefix}  ⚠ 容器未运行")
            else:
                self._print(f"{node_prefix}  ✓ 容器正在运行")
            
            # 获取容器日志
            self._print(f"{node_prefix}获取容器日志...")
            # 使用 ctr tasks logs 获取日志
            logs_cmd = ' '.join(ssh_cmd) + f' "sudo ctr -n k8s.io tasks logs {container_name} 2>&1 | head -50 || echo NO_LOGS"'
            logs_result = subprocess.run(logs_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            
            if 'NO_LOGS' not in logs_result.stdout and logs_result.stdout.strip():
                self._print(f"{node_prefix}  容器日志:")
                for line in logs_result.stdout.strip().split('\n')[:20]:  # 只显示前20行
                    if line.strip():
                        self._print(f"{node_prefix}    {line.strip()}")
            else:
                self._print(f"{node_prefix}  ⚠ 无法获取日志或日志为空")
            
            # 等待容器完成（如果命令会结束）
            time.sleep(3)
            
            # 检查容器最终状态
            final_status_cmd = ' '.join(ssh_cmd) + f' "sudo ctr -n k8s.io tasks ls | grep {container_name} || echo COMPLETED"'
            final_status_result = subprocess.run(final_status_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            
            if 'COMPLETED' in final_status_result.stdout:
                self._print(f"{node_prefix}  ✓ 容器已成功完成")
                return {'success': True, 'status': 'completed', 'logs': logs_result.stdout}
            else:
                self._print(f"{node_prefix}  ℹ 容器仍在运行")
                return {'success': True, 'status': 'running', 'logs': logs_result.stdout}
            
        except subprocess.TimeoutExpired:
            self._print(f"{node_prefix}  ✗ 操作超时")
            return {'success': False, 'error': '操作超时'}
        except Exception as e:
            self._print(f"{node_prefix}  ✗ 测试过程出错: {e}")
            return {'success': False, 'error': str(e)}
        finally:
            # 清理容器
            self._print(f"{node_prefix}清理测试容器...")
            cleanup_cmd = ' '.join(ssh_cmd) + f' "sudo ctr -n k8s.io containers delete {container_name} 2>/dev/null || sudo ctr -n k8s.io tasks kill {container_name} 2>/dev/null || true"'
            subprocess.run(cleanup_cmd, shell=True, check=False, timeout=10, capture_output=True)
    
    def test_all_clusters(self, image_name: str, command: str = None, 
                         cluster_ids: list = None, max_workers: int = 10) -> dict:
        """在所有或指定的 K8s 集群上测试镜像"""
        self._print(f"\n测试镜像在所有 K8s master 节点上是否可以正常运行")
        self._print(f"镜像: {image_name}")
        if command:
            self._print(f"命令: {command}")
        self._print("=" * 80)
        
        # 获取 k8s 集群列表
        k8s_config = self.vm_config['vm_types']['k8s_clusters']
        master_config = k8s_config['master']
        
        if cluster_ids is None:
            cluster_ids = list(range(k8s_config['count']))
        
        k8s_clusters = []
        for cluster_id in cluster_ids:
            ip_suffix = master_config['ip_start'] + cluster_id * master_config['ip_step']
            ip_address = f"{master_config['ip_base']}.{ip_suffix}"
            hostname = f"{master_config['hostname_prefix']}-{cluster_id+1:02d}{master_config['hostname_suffix']}"
            k8s_clusters.append({
                'id': cluster_id,
                'hostname': hostname,
                'ip': ip_address
            })
        
        # 并行测试所有集群
        self._print(f"\n测试 {len(k8s_clusters)} 个 K8s 集群...")
        success_count = 0
        failed_clusters = []
        results = {}
        
        try:
            with ThreadPoolExecutor(max_workers=max_workers) as executor:
                future_to_cluster = {
                    executor.submit(
                        self.test_image_on_cluster,
                        cluster['id'],
                        cluster,
                        image_name,
                        command
                    ): cluster
                    for cluster in k8s_clusters
                }
                
                for future in as_completed(future_to_cluster):
                    cluster = future_to_cluster[future]
                    try:
                        result = future.result()
                        results[cluster['id']] = result
                        if result.get('success'):
                            success_count += 1
                        else:
                            failed_clusters.append(cluster['id'])
                    except Exception as e:
                        failed_clusters.append(cluster['id'])
                        results[cluster['id']] = {'success': False, 'error': str(e)}
                        self._print(f"[K8s集群 {cluster['id']}] ✗ 测试异常: {e}")
        except Exception as e:
            self._print(f"✗ 测试过程出错: {e}")
        
        # 输出结果
        self._print("\n" + "=" * 80)
        self._print(f"测试完成:")
        self._print(f"  成功: {success_count}/{len(k8s_clusters)} 个集群")
        if failed_clusters:
            self._print(f"  失败: {failed_clusters}")
        self._print("=" * 80)
        
        return {
            'success': success_count,
            'total': len(k8s_clusters),
            'failed': failed_clusters,
            'results': results
        }

def main():
    parser = argparse.ArgumentParser(
        description='在 K8s master 节点上使用 containerd 创建容器并获取日志，验证镜像是否可以正常运行',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 测试镜像在所有集群上
  python3 test_k8s_image_with_containerd.py --image iarnet/component:python_3.11-latest
  
  # 测试镜像并运行自定义命令
  python3 test_k8s_image_with_containerd.py --image iarnet/component:python_3.11-latest --command "python3 --version"
  
  # 只测试指定集群
  python3 test_k8s_image_with_containerd.py --image iarnet/component:python_3.11-latest --clusters 0,1,2
  
  # 指定容器名称
  python3 test_k8s_image_with_containerd.py --image iarnet/component:python_3.11-latest --container-name my-test-container
        """
    )
    
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    
    parser.add_argument(
        '--image', '-i',
        type=str,
        required=True,
        help='要测试的镜像名称（如 iarnet/component:python_3.11-latest）'
    )
    
    parser.add_argument(
        '--command', '-c',
        type=str,
        help='容器内要执行的命令（默认: echo 测试消息 && sleep 10）'
    )
    
    parser.add_argument(
        '--container-name',
        type=str,
        help='容器名称（默认自动生成）'
    )
    
    parser.add_argument(
        '--clusters',
        type=str,
        help='要测试的集群ID列表，格式: 0,1,2 或 0-10（默认: 所有集群）'
    )
    
    parser.add_argument(
        '--max-workers', '-w',
        type=int,
        default=10,
        help='最大并发数 (默认: 10)'
    )
    
    args = parser.parse_args()
    
    # 解析集群ID列表
    cluster_ids = None
    if args.clusters:
        if '-' in args.clusters:
            start, end = map(int, args.clusters.split('-'))
            cluster_ids = list(range(start, end + 1))
        else:
            cluster_ids = [int(x.strip()) for x in args.clusters.split(',')]
    
    try:
        tester = ContainerdContainerTester(args.vm_config)
        result = tester.test_all_clusters(
            image_name=args.image,
            command=args.command,
            cluster_ids=cluster_ids,
            max_workers=args.max_workers
        )
        
        if result['success'] == result['total']:
            print("\n✓ 所有测试通过！")
            sys.exit(0)
        else:
            print(f"\n⚠ 部分测试失败: {result['success']}/{result['total']}")
            sys.exit(1)
        
    except KeyboardInterrupt:
        print("\n\n用户中断")
        sys.exit(1)
    except Exception as e:
        print(f"\n错误: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)

if __name__ == '__main__':
    main()

