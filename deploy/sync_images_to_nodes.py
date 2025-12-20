#!/usr/bin/env python3
"""
将 Docker 镜像同步到各个节点
支持将本地镜像直接推送到节点，避免使用 Registry
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

class ImageSyncer:
    def __init__(self, vm_config_path: str):
        """初始化镜像同步器"""
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
    
    def check_docker_installed(self, ssh_cmd: list) -> bool:
        """检查 Docker 是否已安装"""
        check_cmd = ' '.join(ssh_cmd) + ' "docker --version >/dev/null 2>&1 && echo OK || echo NOT_INSTALLED"'
        try:
            result = subprocess.run(check_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            return 'OK' in result.stdout
        except:
            return False
    
    def check_image_exists_local(self, image_name: str) -> bool:
        """检查本地镜像是否存在"""
        try:
            result = subprocess.run(
                ['docker', 'images', '-q', image_name],
                capture_output=True,
                text=True,
                check=True
            )
            return bool(result.stdout.strip())
        except:
            return False
    
    def check_image_exists_remote(self, ssh_cmd: list, image_name: str) -> bool:
        """检查远程节点上镜像是否存在"""
        check_cmd = ' '.join(ssh_cmd) + f' "docker images -q {image_name} 2>/dev/null | head -1"'
        try:
            result = subprocess.run(check_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            return bool(result.stdout.strip())
        except:
            return False
    
    def save_image_to_tar(self, image_name: str, output_path: str) -> bool:
        """将镜像保存为 tar 文件"""
        try:
            subprocess.run(
                ['docker', 'save', '-o', output_path, image_name],
                check=True,
                capture_output=True,
                text=True
            )
            return True
        except subprocess.CalledProcessError as e:
            self._print(f"  ✗ 镜像保存失败: {e}")
            return False
    
    def sync_image_to_node(self, node_id: int, node_info: dict, image_name: str, 
                          image_path: Path = None, skip_if_exists: bool = True) -> bool:
        """同步镜像到单个节点"""
        node_prefix = f"[节点 {node_id}] {node_info['hostname']} ({node_info['ip']}) "
        
        # 检查连通性
        if not self.check_node_connectivity(node_info['ip']):
            self._print(f"{node_prefix}⚠ 无法连接，跳过")
            return False
        
        ssh_cmd = [
            'ssh',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f"{self.user}@{node_info['ip']}"
        ]
        
        # 检查 Docker
        if not self.check_docker_installed(ssh_cmd):
            self._print(f"{node_prefix}⚠ Docker 未安装，跳过")
            return False
        
        # 检查镜像是否已存在
        if skip_if_exists:
            if self.check_image_exists_remote(ssh_cmd, image_name):
                self._print(f"{node_prefix}✓ 镜像已存在: {image_name}")
                return True
        
        # 如果提供了镜像文件路径，直接传输
        if image_path and image_path.exists():
            self._print(f"{node_prefix}传输镜像文件: {image_path.name}...")
            
            # 使用 scp 传输
            scp_cmd = [
                'scp',
                '-o', 'StrictHostKeyChecking=no',
                '-o', 'UserKnownHostsFile=/dev/null',
                str(image_path),
                f"{self.user}@{node_info['ip']}:~/image.tar"
            ]
            
            try:
                subprocess.run(scp_cmd, check=True, capture_output=True, timeout=300)
                self._print(f"{node_prefix}  ✓ 文件传输成功")
            except subprocess.CalledProcessError as e:
                self._print(f"{node_prefix}  ✗ 文件传输失败: {e}")
                return False
            except subprocess.TimeoutExpired:
                self._print(f"{node_prefix}  ✗ 文件传输超时")
                return False
            
            # 在远程节点上加载镜像
            self._print(f"{node_prefix}加载镜像...")
            load_cmd = ' '.join(ssh_cmd) + ' "docker load -i ~/image.tar && rm ~/image.tar"'
            try:
                result = subprocess.run(load_cmd, shell=True, check=True, timeout=120, capture_output=True, text=True)
                self._print(f"{node_prefix}  ✓ 镜像加载成功")
                return True
            except subprocess.CalledProcessError as e:
                self._print(f"{node_prefix}  ✗ 镜像加载失败: {e}")
                # 清理临时文件
                cleanup_cmd = ' '.join(ssh_cmd) + ' "rm -f ~/image.tar"'
                subprocess.run(cleanup_cmd, shell=True, check=False, timeout=5, capture_output=True)
                return False
        else:
            # 如果没有提供文件路径，需要先从本地保存
            self._print(f"{node_prefix}⚠ 未提供镜像文件路径，跳过")
            return False
    
    def sync_image_to_nodes(self, image_name: str, node_type: str = 'docker', 
                           node_ids: list = None, max_workers: int = 10, force: bool = False) -> bool:
        """同步镜像到多个节点"""
        self._print(f"\n同步镜像到节点: {image_name}")
        self._print("=" * 60)
        
        # 检查本地镜像是否存在
        if not self.check_image_exists_local(image_name):
            self._print(f"✗ 本地镜像不存在: {image_name}")
            self._print(f"请先构建或拉取镜像: docker build/pull {image_name}")
            return False
        
        # 获取节点列表
        if node_type == 'docker':
            node_config = self.vm_config['vm_types']['docker']
        elif node_type == 'iarnet':
            node_config = self.vm_config['vm_types']['iarnet']
        else:
            self._print(f"错误: 不支持的节点类型: {node_type}")
            return False
        
        if node_ids is None:
            node_ids = list(range(node_config['count']))
        
        # 构建节点信息
        node_info_list = []
        for node_id in node_ids:
            ip_suffix = node_config['ip_start'] + node_id
            ip_address = f"{node_config['ip_base']}.{ip_suffix}"
            hostname = f"{node_config['hostname_prefix']}-{node_id+1:02d}"
            node_info_list.append({
                'id': node_id,
                'hostname': hostname,
                'ip': ip_address
            })
        
        # 先保存镜像为 tar 文件（所有节点共享同一个文件）
        self._print(f"\n1. 保存镜像为 tar 文件...")
        import tempfile
        with tempfile.NamedTemporaryFile(suffix='.tar', delete=False) as tmp_file:
            image_tar_path = Path(tmp_file.name)
        
        try:
            self._print(f"  保存镜像: {image_name} -> {image_tar_path.name}...")
            if not self.save_image_to_tar(image_name, str(image_tar_path)):
                return False
            self._print(f"  ✓ 镜像保存成功 ({image_tar_path.stat().st_size / (1024**2):.1f} MB)")
        except Exception as e:
            self._print(f"  ✗ 镜像保存失败: {e}")
            return False
        
        # 并行同步到各个节点
        self._print(f"\n2. 同步镜像到 {len(node_info_list)} 个节点...")
        success_count = 0
        failed_nodes = []
        
        try:
            with ThreadPoolExecutor(max_workers=max_workers) as executor:
                future_to_node = {
                    executor.submit(
                        self.sync_image_to_node,
                        node_info['id'],
                        node_info,
                        image_name,
                        image_tar_path,
                        skip_if_exists=not force
                    ): node_info
                    for node_info in node_info_list
                }
                
                for future in as_completed(future_to_node):
                    node_info = future_to_node[future]
                    try:
                        result = future.result()
                        if result:
                            success_count += 1
                        else:
                            failed_nodes.append(node_info['id'])
                    except Exception as e:
                        failed_nodes.append(node_info['id'])
                        self._print(f"[节点 {node_info['id']}] ✗ 同步异常: {e}")
        finally:
            # 清理临时文件
            if image_tar_path.exists():
                image_tar_path.unlink()
        
        # 输出结果
        self._print("\n" + "=" * 60)
        self._print(f"同步完成: {success_count}/{len(node_info_list)} 个节点成功")
        if failed_nodes:
            self._print(f"失败的节点: {failed_nodes}")
        self._print("=" * 60)
        
        return success_count > 0
    
    def sync_iarnet_image(self, local_image: str = None, node_type: str = 'docker', 
                         node_ids: list = None) -> bool:
        """同步 iarnet 镜像到节点"""
        if local_image is None:
            local_image = "iarnet:latest"
        
        return self.sync_image_to_nodes(local_image, node_type, node_ids)
    
    def sync_component_image(self, image_name: str, node_type: str = 'docker',
                            node_ids: list = None) -> bool:
        """同步 component 镜像到节点"""
        return self.sync_image_to_nodes(image_name, node_type, node_ids)

    def sync_multiple_images(self, image_names: list, node_type: str = 'docker',
                            node_ids: list = None, max_workers: int = 10, force: bool = False) -> bool:
        """批量同步多个镜像"""
        self._print(f"\n批量同步 {len(image_names)} 个镜像到节点...")
        self._print("=" * 60)
        
        success_count = 0
        for i, image_name in enumerate(image_names, 1):
            self._print(f"\n[{i}/{len(image_names)}] 同步镜像: {image_name}")
            if self.sync_image_to_nodes(image_name, node_type, node_ids, max_workers, force):
                success_count += 1
            else:
                self._print(f"  ⚠ 镜像 {image_name} 同步失败，继续下一个...")
        
        self._print("\n" + "=" * 60)
        self._print(f"批量同步完成: {success_count}/{len(image_names)} 个镜像成功")
        self._print("=" * 60)
        
        return success_count == len(image_names)

def main():
    parser = argparse.ArgumentParser(description='将 Docker 镜像同步到各个节点')
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--image', '-i',
        type=str,
        default=None,
        help='要同步的镜像名（如 iarnet:latest 或 my-component:latest）'
    )
    parser.add_argument(
        '--images', '-I',
        type=str,
        nargs='+',
        default=None,
        help='要同步的多个镜像名（用空格分隔）'
    )
    parser.add_argument(
        '--node-type', '-t',
        type=str,
        choices=['docker', 'iarnet', 'all'],
        default='docker',
        help='要同步到的节点类型 (默认: docker)'
    )
    parser.add_argument(
        '--nodes', '-n',
        type=str,
        default=None,
        help='指定节点ID列表，格式: 0,1,2 或 0-10 (默认: 所有节点)'
    )
    parser.add_argument(
        '--max-workers', '-w',
        type=int,
        default=10,
        help='最大并发数 (默认: 10)'
    )
    parser.add_argument(
        '--force', '-f',
        action='store_true',
        help='强制同步，即使节点上已存在镜像'
    )
    
    args = parser.parse_args()
    
    # 检查参数
    if not args.image and not args.images:
        parser.print_help()
        print("\n错误: 请指定要同步的镜像 (--image 或 --images)")
        sys.exit(1)
    
    try:
        syncer = ImageSyncer(args.vm_config)
        
        # 解析节点列表
        node_ids = None
        if args.nodes:
            if '-' in args.nodes:
                start, end = map(int, args.nodes.split('-'))
                node_ids = list(range(start, end + 1))
            else:
                node_ids = [int(x.strip()) for x in args.nodes.split(',')]
        
        # 确定要同步的镜像列表
        if args.images:
            image_list = args.images
        else:
            image_list = [args.image]
        
        # 同步镜像
        if args.node_type == 'all':
            # 先同步到 iarnet 节点
            syncer.sync_multiple_images(image_list, 'iarnet', node_ids, args.max_workers, args.force)
            # 再同步到 docker 节点
            syncer.sync_multiple_images(image_list, 'docker', node_ids, args.max_workers, args.force)
        else:
            syncer.sync_multiple_images(image_list, args.node_type, node_ids, args.max_workers, args.force)
        
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()

