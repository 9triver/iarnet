#!/usr/bin/env python3
"""
修复所有 K8s master 节点上镜像名称的 docker.io 前缀问题
将 docker.io/xxx 重新标记为 xxx
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
PROJECT_ROOT = SCRIPT_DIR.parent

class K8sImageNameFixer:
    def __init__(self, vm_config_path: str):
        """初始化修复器"""
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
    
    def fix_image_names_on_cluster(self, cluster_id: int, cluster_info: dict, 
                                   image_pattern: str = None) -> bool:
        """修复指定集群 master 节点上的镜像名称"""
        node_prefix = f"[K8s集群 {cluster_id}] {cluster_info['hostname']} ({cluster_info['ip']}) "
        
        # 检查连通性
        if not self.check_node_connectivity(cluster_info['ip']):
            self._print(f"{node_prefix}⚠ 无法连接，跳过")
            return False
        
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
                return False
        except:
            self._print(f"{node_prefix}⚠ 无法检查 containerd，跳过")
            return False
        
        # 查找所有带有 docker.io/ 前缀的镜像
        self._print(f"{node_prefix}查找需要修复的镜像...")
        list_cmd = ' '.join(ssh_cmd) + ' "sudo ctr -n k8s.io images ls | grep -E \'^docker\\.io/\' || echo NO_IMAGES"'
        try:
            list_result = subprocess.run(list_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            if 'NO_IMAGES' in list_result.stdout or not list_result.stdout.strip():
                self._print(f"{node_prefix}  ✓ 没有需要修复的镜像")
                return True
            
            # 解析镜像列表
            images_to_fix = []
            for line in list_result.stdout.strip().split('\n'):
                if line.strip() and 'docker.io/' in line:
                    # 提取镜像名称（第一列）
                    parts = line.split()
                    if parts:
                        old_name = parts[0]
                        # 去掉 docker.io/ 前缀
                        if old_name.startswith('docker.io/'):
                            new_name = old_name[len('docker.io/'):]
                            # 如果指定了镜像模式，只处理匹配的镜像
                            if image_pattern is None or image_pattern in old_name:
                                images_to_fix.append((old_name, new_name))
            
            if not images_to_fix:
                self._print(f"{node_prefix}  ✓ 没有需要修复的镜像（已过滤）")
                return True
            
            self._print(f"{node_prefix}  找到 {len(images_to_fix)} 个需要修复的镜像")
            
            # 修复每个镜像
            fixed_count = 0
            for old_name, new_name in images_to_fix:
                self._print(f"{node_prefix}    修复: {old_name} -> {new_name}")
                
                # 使用 ctr tag 重新标记镜像
                tag_cmd = ' '.join(ssh_cmd) + f' "sudo ctr -n k8s.io images tag {old_name} {new_name} 2>&1"'
                try:
                    tag_result = subprocess.run(tag_cmd, shell=True, check=True, timeout=30, capture_output=True, text=True)
                    self._print(f"{node_prefix}      ✓ 已重新标记")
                    fixed_count += 1
                    
                    # 可选：删除旧的带前缀的镜像引用（只删除标签，不删除实际镜像数据）
                    # 注意：这不会删除镜像数据，只是删除标签引用
                    untag_cmd = ' '.join(ssh_cmd) + f' "sudo ctr -n k8s.io images remove {old_name} 2>&1 || true"'
                    subprocess.run(untag_cmd, shell=True, check=False, timeout=10, capture_output=True)
                    
                except subprocess.CalledProcessError as e:
                    error_msg = e.stderr if hasattr(e, 'stderr') and e.stderr else (e.stdout if hasattr(e, 'stdout') else str(e))
                    # 如果镜像已经存在，也算成功
                    if 'already exists' in str(error_msg).lower() or 'already tagged' in str(error_msg).lower():
                        self._print(f"{node_prefix}      ℹ 镜像已存在，跳过")
                        fixed_count += 1
                    else:
                        self._print(f"{node_prefix}      ✗ 修复失败: {error_msg[:100]}")
            
            if fixed_count == len(images_to_fix):
                self._print(f"{node_prefix}  ✓ 所有镜像修复成功 ({fixed_count}/{len(images_to_fix)})")
                return True
            else:
                self._print(f"{node_prefix}  ⚠ 部分镜像修复失败 ({fixed_count}/{len(images_to_fix)})")
                return False
                
        except Exception as e:
            self._print(f"{node_prefix}  ✗ 修复过程出错: {e}")
            return False
    
    def fix_all_clusters(self, image_pattern: str = None, max_workers: int = 10) -> dict:
        """修复所有 K8s 集群的镜像名称"""
        self._print(f"\n修复所有 K8s master 节点上的镜像名称（去掉 docker.io/ 前缀）")
        if image_pattern:
            self._print(f"只处理匹配的镜像: {image_pattern}")
        self._print("=" * 80)
        
        # 获取 k8s 集群列表（只修复 master 节点）
        k8s_config = self.vm_config['vm_types']['k8s_clusters']
        master_config = k8s_config['master']
        k8s_clusters = []
        for cluster_id in range(k8s_config['count']):
            ip_suffix = master_config['ip_start'] + cluster_id * master_config['ip_step']
            ip_address = f"{master_config['ip_base']}.{ip_suffix}"
            hostname = f"{master_config['hostname_prefix']}-{cluster_id+1:02d}{master_config['hostname_suffix']}"
            k8s_clusters.append({
                'id': cluster_id,
                'hostname': hostname,
                'ip': ip_address
            })
        
        # 并行修复所有集群
        self._print(f"\n修复 {len(k8s_clusters)} 个 K8s 集群的镜像名称...")
        success_count = 0
        failed_clusters = []
        
        try:
            with ThreadPoolExecutor(max_workers=max_workers) as executor:
                future_to_cluster = {
                    executor.submit(
                        self.fix_image_names_on_cluster,
                        cluster['id'],
                        cluster,
                        image_pattern
                    ): cluster
                    for cluster in k8s_clusters
                }
                
                for future in as_completed(future_to_cluster):
                    cluster = future_to_cluster[future]
                    try:
                        result = future.result()
                        if result:
                            success_count += 1
                        else:
                            failed_clusters.append(cluster['id'])
                    except Exception as e:
                        failed_clusters.append(cluster['id'])
                        self._print(f"[K8s集群 {cluster['id']}] ✗ 修复异常: {e}")
        except Exception as e:
            self._print(f"✗ 修复过程出错: {e}")
        
        # 输出结果
        self._print("\n" + "=" * 80)
        self._print(f"修复完成:")
        self._print(f"  成功: {success_count}/{len(k8s_clusters)} 个集群")
        if failed_clusters:
            self._print(f"  失败: {failed_clusters}")
        self._print("=" * 80)
        
        return {
            'success': success_count,
            'total': len(k8s_clusters),
            'failed': failed_clusters
        }

def main():
    parser = argparse.ArgumentParser(
        description='修复所有 K8s master 节点上镜像名称的 docker.io 前缀问题',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 修复所有集群的所有镜像
  python3 fix_k8s_image_names.py
  
  # 只修复特定镜像（如 component 镜像）
  python3 fix_k8s_image_names.py --image-pattern component
  
  # 指定配置文件
  python3 fix_k8s_image_names.py --vm-config vm-config.yaml
  
  # 调整并发数
  python3 fix_k8s_image_names.py --max-workers 20
        """
    )
    
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    
    parser.add_argument(
        '--image-pattern', '-i',
        type=str,
        help='只修复匹配的镜像名称（如 component）'
    )
    
    parser.add_argument(
        '--max-workers', '-w',
        type=int,
        default=10,
        help='最大并发数 (默认: 10)'
    )
    
    args = parser.parse_args()
    
    try:
        fixer = K8sImageNameFixer(args.vm_config)
        result = fixer.fix_all_clusters(image_pattern=args.image_pattern, max_workers=args.max_workers)
        
        if result['success'] == result['total']:
            print("\n✓ 所有镜像修复完成！")
            sys.exit(0)
        else:
            print(f"\n⚠ 部分镜像修复失败: {result['success']}/{result['total']}")
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

