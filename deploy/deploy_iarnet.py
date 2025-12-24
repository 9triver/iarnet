#!/usr/bin/env python3
"""
在 iarnet 节点上部署 iarnet 服务（使用 Docker 容器）
支持为每个节点使用独立的配置文件
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

# 获取脚本所在目录和项目根目录
SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent

class IarnetDeployer:
    """iarnet Docker 容器部署器"""
    
    _log_lock = Lock()
    
    def __init__(self, vm_config_path: str, configs_dir: str = None):
        """初始化部署器"""
        # 处理vm_config_path（支持相对路径和绝对路径）
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
        
        self.iarnet_config = self.vm_config['vm_types']['iarnet']
        
        # 处理配置文件目录路径
        if configs_dir is None:
            self.configs_dir = SCRIPT_DIR / 'iarnet-configs'
        else:
            configs_dir_obj = Path(configs_dir)
            if configs_dir_obj.is_absolute():
                self.configs_dir = configs_dir_obj
            else:
                if (SCRIPT_DIR / configs_dir_obj.name).exists() or 'iarnet-configs' in str(configs_dir_obj):
                    self.configs_dir = SCRIPT_DIR / configs_dir_obj.name if configs_dir_obj.name == 'iarnet-configs' else SCRIPT_DIR / configs_dir_obj
                else:
                    self.configs_dir = PROJECT_ROOT / configs_dir_obj
        
        self.user = self.vm_config['global']['user']
        
        # 构建节点信息映射
        self.node_info = {}
        for i in range(self.iarnet_config['count']):
            ip_suffix = self.iarnet_config['ip_start'] + i
            ip_address = f"{self.iarnet_config['ip_base']}.{ip_suffix}"
            hostname = f"{self.iarnet_config['hostname_prefix']}-{i+1:02d}"
            self.node_info[i] = {
                'hostname': hostname,
                'ip': ip_address,
                'config_file': self.configs_dir / f"config-node-{i:02d}.yaml"
            }
    
    def _print(self, *args, **kwargs):
        """线程安全的打印函数"""
        with self._log_lock:
            print(*args, **kwargs)
    
    def check_node_connectivity(self, node_id: int) -> bool:
        """检查节点连通性"""
        node = self.node_info[node_id]
        try:
            result = subprocess.run(
                ['ping', '-c', '1', '-W', '2', node['ip']],
                capture_output=True,
                timeout=5
            )
            return result.returncode == 0
        except:
            return False
    
    def check_docker_installed(self, ssh_cmd: list) -> bool:
        """检查 Docker 是否已安装"""
        check_cmd = ' '.join(ssh_cmd) + ' "docker --version >/dev/null 2>&1 && echo OK || echo FAIL"'
        try:
            result = subprocess.run(check_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
            return 'OK' in result.stdout
        except:
            return False
    
    def check_image_exists(self, ssh_cmd: list, image_name: str) -> bool:
        """检查镜像是否存在于远程节点"""
        check_cmd = ' '.join(ssh_cmd) + f' "docker images -q {image_name} | head -1"'
        try:
            result = subprocess.run(check_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
            return bool(result.stdout.strip())
        except:
            return False
    
    def stop_existing_container(self, ssh_cmd: list, container_name: str, node_id: int = None) -> bool:
        """停止并删除现有的容器"""
        node_prefix = f"[节点 {node_id}] " if node_id is not None else ""
        
        # 检查容器是否存在
        check_cmd = ' '.join(ssh_cmd) + f' "docker ps -a --filter name=^{container_name}$ --format \'{{{{.Names}}}}\' | head -1"'
        try:
            result = subprocess.run(check_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
            if not result.stdout.strip():
                return True  # 容器不存在，无需停止
        except:
            pass
        
        # 停止容器
        self._print(f"{node_prefix}    停止现有容器: {container_name}")
        stop_cmd = ' '.join(ssh_cmd) + f' "docker stop {container_name} >/dev/null 2>&1 || true"'
        subprocess.run(stop_cmd, shell=True, check=False, timeout=60, capture_output=True)
        
        # 删除容器
        self._print(f"{node_prefix}    删除现有容器: {container_name}")
        rm_cmd = ' '.join(ssh_cmd) + f' "docker rm {container_name} >/dev/null 2>&1 || true"'
        subprocess.run(rm_cmd, shell=True, check=False, timeout=60, capture_output=True)
        
        return True
    
    def upload_config_file(self, ssh_cmd: list, config_file: Path, node_id: int = None) -> bool:
        """上传配置文件到虚拟机"""
        node_prefix = f"[节点 {node_id}] " if node_id is not None else ""
        
        if not config_file.exists():
            self._print(f"{node_prefix}    ✗ 配置文件不存在: {config_file}")
            return False
        
        # 确保目标目录存在
        ensure_dir_cmd = ' '.join(ssh_cmd) + ' "mkdir -p ~/iarnet"'
        subprocess.run(ensure_dir_cmd, shell=True, check=False, timeout=5, capture_output=True)
        
        # 使用 scp 上传配置文件
        self._print(f"{node_prefix}    上传配置文件: {config_file.name}")
        scp_cmd = [
            'scp',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            str(config_file),
            f"{self.user}@{ssh_cmd[-1].split('@')[1]}:~/iarnet/config.yaml"
        ]
        
        try:
            subprocess.run(scp_cmd, check=True, capture_output=True, timeout=60)
            self._print(f"{node_prefix}    ✓ 配置文件上传成功")
            return True
        except subprocess.CalledProcessError as e:
            self._print(f"{node_prefix}    ✗ 配置文件上传失败: {e}")
            return False
    
    def ensure_directories(self, ssh_cmd: list, node_id: int = None) -> bool:
        """确保必要的目录存在"""
        node_prefix = f"[节点 {node_id}] " if node_id is not None else ""
        
        # 创建必要的目录
        dirs = [
            '~/iarnet/data',
            '~/iarnet/workspaces'
        ]
        
        for dir_path in dirs:
            mkdir_cmd = ' '.join(ssh_cmd) + f' "mkdir -p {dir_path}"'
            try:
                subprocess.run(mkdir_cmd, shell=True, check=True, timeout=5, capture_output=True)
            except subprocess.CalledProcessError:
                self._print(f"{node_prefix}    ⚠ 创建目录失败: {dir_path}")
                return False
        
        # 创建日志文件（如果不存在）
        touch_log_cmd = ' '.join(ssh_cmd) + ' "touch ~/iarnet/iarnet.log && chmod 666 ~/iarnet/iarnet.log || true"'
        subprocess.run(touch_log_cmd, shell=True, check=False, timeout=5, capture_output=True)
        
        return True
    
    def start_container(self, ssh_cmd: list, node: dict, node_id: int = None, image_name: str = "iarnet:latest") -> bool:
        """启动 iarnet Docker 容器"""
        node_prefix = f"[节点 {node_id}] " if node_id is not None else ""
        container_name = f"iarnet-{node['hostname']}"
        
        # 检查镜像是否存在
        if not self.check_image_exists(ssh_cmd, image_name):
            self._print(f"{node_prefix}    ✗ 镜像不存在: {image_name}")
            self._print(f"{node_prefix}    请先同步镜像到节点: python3 deploy/sync_iarnet_images.py --nodes {node_id}")
            return False
        
        # 停止现有容器
        self.stop_existing_container(ssh_cmd, container_name, node_id)
        
        # 确保目录存在
        if not self.ensure_directories(ssh_cmd, node_id):
            return False
        
        # 获取虚拟机上的 HOME 目录绝对路径（通过 SSH 执行）
        get_home_cmd = ' '.join(ssh_cmd) + ' "echo $HOME"'
        try:
            home_result = subprocess.run(get_home_cmd, shell=True, check=True, timeout=5, capture_output=True, text=True)
            home_path = home_result.stdout.strip()
            if not home_path:
                self._print(f"{node_prefix}    ✗ 无法获取 HOME 目录路径")
                return False
            # 验证路径是虚拟机上的路径（不是本地路径）
            if 'zhangyx' in home_path and self.user != 'zhangyx':
                self._print(f"{node_prefix}    ⚠ 警告: 获取到的路径可能不正确: {home_path}")
                # 使用配置的用户名构建路径
                home_path = f'/home/{self.user}'
                self._print(f"{node_prefix}    使用配置的用户路径: {home_path}")
        except subprocess.CalledProcessError:
            self._print(f"{node_prefix}    ✗ 无法获取 HOME 目录路径")
            return False
        
        # 验证配置文件在虚拟机上是否存在
        check_config_cmd = ' '.join(ssh_cmd) + f' "test -f {home_path}/iarnet/config.yaml && echo EXISTS || echo NOT_EXISTS"'
        try:
            config_check = subprocess.run(check_config_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
            if 'NOT_EXISTS' in config_check.stdout:
                self._print(f"{node_prefix}    ✗ 配置文件不存在: {home_path}/iarnet/config.yaml")
                self._print(f"{node_prefix}    请确保已上传配置文件")
                return False
        except:
            pass  # 如果检查失败，继续执行（可能权限问题）
        
        # 清空日志文件（每次重启容器时清除旧日志）
        # 确保文件存在且有正确的权限（容器内需要可写）
        log_file_path = f'{home_path}/iarnet/iarnet.log'
        self._print(f"{node_prefix}    清空日志文件: {log_file_path}")
        # 使用 truncate 或 echo -n 清空文件，确保文件存在且有写权限
        clear_log_cmd = ' '.join(ssh_cmd) + f' "truncate -s 0 {log_file_path} 2>/dev/null || echo -n > {log_file_path} 2>/dev/null || true"'
        subprocess.run(clear_log_cmd, shell=True, check=False, timeout=5, capture_output=True)
        # 确保文件有正确的权限（容器内可写）
        chmod_log_cmd = ' '.join(ssh_cmd) + f' "chmod 666 {log_file_path} 2>/dev/null || true"'
        subprocess.run(chmod_log_cmd, shell=True, check=False, timeout=5, capture_output=True)
        
        # 构建 docker run 命令
        # 使用 host 网络模式，使容器与虚拟机共享网络
        # 挂载 docker socket，使容器可以使用主机的 Docker
        # 挂载配置文件、数据目录和日志文件
        # 使用绝对路径避免路径展开问题
        docker_run_cmd_parts = [
            'docker', 'run',
            '--name', container_name,
            '--network', 'host',  # 使用主机网络
            '--restart', 'unless-stopped',
            '-v', '/var/run/docker.sock:/var/run/docker.sock',  # 挂载 Docker socket
            '-v', f'{home_path}/iarnet/config.yaml:/app/config.yaml:ro',  # 只读挂载配置文件
            '-v', f'{home_path}/iarnet/data:/app/data',  # 数据目录
            '-v', f'{home_path}/iarnet/workspaces:/app/workspaces',  # 工作空间目录
            '-v', f'{home_path}/iarnet/iarnet.log:/app/iarnet.log',  # 日志文件
            '-e', f'BACKEND_URL=http://{node["ip"]}:8083',
            '-e', 'DOCKER_HOST=unix:///var/run/docker.sock',
            '-e', 'USE_HOST_DOCKER=1',  # 使用主机 Docker
            '-d',  # 后台运行
            image_name
        ]
        
        # 执行 docker run 命令
        # 使用 shlex.quote 来正确转义每个参数
        self._print(f"{node_prefix}    启动容器: {container_name}")
        import shlex
        quoted_parts = [shlex.quote(part) for part in docker_run_cmd_parts]
        docker_run_cmd_str = ' '.join(quoted_parts)
        run_cmd = ' '.join(ssh_cmd) + f' "{docker_run_cmd_str}"'
        
        try:
            result = subprocess.run(run_cmd, shell=True, check=False, timeout=30, capture_output=True, text=True)
            
            if result.returncode != 0:
                self._print(f"{node_prefix}    ✗ 启动容器失败: {result.stderr[:300]}")
                return False
            
            container_id = result.stdout.strip()
            if container_id:
                self._print(f"{node_prefix}    ✓ 容器已启动: {container_id[:12]}")
            else:
                self._print(f"{node_prefix}    ⚠ 容器启动但未返回 ID")
            
            # 等待容器启动
            time.sleep(3)
            
            # 检查容器状态
            status_cmd = ' '.join(ssh_cmd) + f' "docker ps --filter name=^{container_name}$ --format \'{{{{.Status}}}}\' | head -1"'
            status_result = subprocess.run(status_cmd, shell=True, check=False, timeout=30, capture_output=True, text=True)
            
            if status_result.stdout.strip():
                self._print(f"{node_prefix}    ✓ 容器运行中: {status_result.stdout.strip()}")
                
                # 检查服务是否就绪（检查端口 8083）
                for attempt in range(5):
                    time.sleep(2)
                    check_port_cmd = ' '.join(ssh_cmd) + ' "timeout 3 bash -c \'</dev/tcp/localhost/8083\' 2>/dev/null && echo PORT_OK || echo PORT_NOT_READY"'
                    port_result = subprocess.run(check_port_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
                    
                    if 'PORT_OK' in port_result.stdout:
                        self._print(f"{node_prefix}    ✓ 后端服务已就绪 (http://{node['ip']}:8083)")
                        self._print(f"{node_prefix}    ✓ 前端服务已就绪 (http://{node['ip']}:3000)")
                        return True
                
                # 如果端口检查失败，查看日志
                log_cmd = ' '.join(ssh_cmd) + f' "docker logs --tail 20 {container_name} 2>&1 | tail -10"'
                log_result = subprocess.run(log_cmd, shell=True, check=False, timeout=30, capture_output=True, text=True)
                if log_result.stdout:
                    self._print(f"{node_prefix}    容器日志: {log_result.stdout.strip()}")
                
                self._print(f"{node_prefix}    ⚠ 服务可能未完全就绪，请检查日志")
                return True  # 容器已启动，即使服务未就绪也返回成功
            else:
                # 容器可能已停止，查看日志
                log_cmd = ' '.join(ssh_cmd) + f' "docker logs --tail 30 {container_name} 2>&1"'
                log_result = subprocess.run(log_cmd, shell=True, check=False, timeout=30, capture_output=True, text=True)
                if log_result.stdout:
                    self._print(f"{node_prefix}    容器日志: {log_result.stdout.strip()[:500]}")
                
                self._print(f"{node_prefix}    ✗ 容器未运行")
                return False
                
        except subprocess.TimeoutExpired:
            self._print(f"{node_prefix}    ✗ 启动容器超时")
            return False
        except Exception as e:
            self._print(f"{node_prefix}    ✗ 启动容器异常: {e}")
            return False
    
    def deploy_to_node(self, node_id: int, restart: bool = False, image_name: str = "iarnet:latest") -> bool:
        """部署到指定节点"""
        if node_id not in self.node_info:
            self._print(f"[节点 {node_id}] 错误: 节点 {node_id} 不存在")
            return False
        
        node = self.node_info[node_id]
        config_file = node['config_file']
        
        if not config_file.exists():
            self._print(f"[节点 {node_id}] 错误: 配置文件不存在: {config_file}")
            self._print(f"[节点 {node_id}] 请先运行: python3 generate_iarnet_configs.py --nodes {node_id}")
            return False
        
        self._print(f"\n[节点 {node_id}] 部署到节点: {node['hostname']} ({node['ip']})")
        self._print(f"[节点 {node_id}] " + "=" * 60)
        
        # 检查连通性
        if not self.check_node_connectivity(node_id):
            self._print(f"[节点 {node_id}] 警告: 无法连接到节点 {node_id} ({node['ip']})")
            self._print(f"[节点 {node_id}] 请确保虚拟机已启动且网络正常")
            return False
        
        # 构建SSH命令
        ssh_cmd = [
            'ssh',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f"{self.user}@{node['ip']}"
        ]
        
        # 检查 Docker 是否安装
        if not self.check_docker_installed(ssh_cmd):
            self._print(f"[节点 {node_id}] 错误: Docker 未安装")
            self._print(f"[节点 {node_id}] 请先安装 Docker 或使用已安装 Docker 的基础镜像")
            return False
        
        # 如果指定 restart，先停止现有容器
        if restart:
            container_name = f"iarnet-{node['hostname']}"
            self._print(f"[节点 {node_id}] 停止现有容器...")
            self.stop_existing_container(ssh_cmd, container_name, node_id)
        
        # 上传配置文件
        self._print(f"[节点 {node_id}] 1. 上传配置文件...")
        if not self.upload_config_file(ssh_cmd, config_file, node_id):
            return False
        
        # 启动容器
        self._print(f"[节点 {node_id}] 2. 启动 Docker 容器...")
        if not self.start_container(ssh_cmd, node, node_id, image_name):
            return False
        
        self._print(f"[节点 {node_id}] ✓ 部署完成")
        self._print(f"[节点 {node_id}] 访问地址:")
        self._print(f"[节点 {node_id}]   后端: http://{node['ip']}:8083")
        self._print(f"[节点 {node_id}]   前端: http://{node['ip']}:3000")
        self._print(f"[节点 {node_id}]   日志: ssh {node['ip']} 'tail -f ~/iarnet/iarnet.log'")
        
        return True
    
    def deploy_to_nodes(self, node_ids: list, restart: bool = False, image_name: str = "iarnet:latest", max_workers: int = None):
        """批量部署到多个节点（并行执行）"""
        self._print(f"批量部署到节点: {node_ids}")
        self._print("=" * 60)
        
        if max_workers is None:
            max_workers = min(len(node_ids), 10)
        
        success_count = 0
        failed_nodes = []
        
        # 使用线程池并行部署
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            future_to_node = {
                executor.submit(
                    self.deploy_to_node,
                    node_id,
                    restart=restart,
                    image_name=image_name
                ): node_id
                for node_id in node_ids
            }
            
            # 等待所有任务完成
            for future in as_completed(future_to_node):
                node_id = future_to_node[future]
                try:
                    result = future.result()
                    if result:
                        success_count += 1
                    else:
                        failed_nodes.append(node_id)
                except Exception as e:
                    failed_nodes.append(node_id)
                    self._print(f"[节点 {node_id}] ✗ 部署异常: {e}")
        
        # 输出部署结果摘要
        self._print("\n" + "=" * 60)
        self._print(f"部署完成: {success_count}/{len(node_ids)} 个节点成功")
        if failed_nodes:
            self._print(f"失败的节点: {failed_nodes}")
        self._print("\n访问地址:")
        for node_id in node_ids:
            if node_id in self.node_info and node_id not in failed_nodes:
                node = self.node_info[node_id]
                self._print(f"  节点 {node_id} ({node['hostname']}):")
                self._print(f"    后端: http://{node['ip']}:8083")
                self._print(f"    前端: http://{node['ip']}:3000")
        self._print("=" * 60)

def main():
    parser = argparse.ArgumentParser(description='在 iarnet 节点上部署 iarnet 服务（使用 Docker 容器）')
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--configs-dir', '-c',
        default=str(SCRIPT_DIR / 'iarnet-configs'),
        help='配置文件目录 (默认: deploy/iarnet-configs)'
    )
    parser.add_argument(
        '--node', '-n',
        type=int,
        help='部署到单个节点'
    )
    parser.add_argument(
        '--nodes', '-N',
        type=str,
        help='部署到多个节点，格式: start-end 或逗号分隔的列表 (例如: 0-9 或 0,1,2)'
    )
    parser.add_argument(
        '--restart', '-r',
        action='store_true',
        help='重启 iarnet 服务（停止现有容器后重新启动）'
    )
    parser.add_argument(
        '--image', '-i',
        type=str,
        default='iarnet:latest',
        help='使用的 Docker 镜像名称 (默认: iarnet:latest)'
    )
    parser.add_argument(
        '--max-workers', '-w',
        type=int,
        default=None,
        help='并行部署的最大线程数 (默认: min(节点数, 10))'
    )
    
    args = parser.parse_args()
    
    # 确定要部署的节点
    if args.node is not None:
        node_ids = [args.node]
    elif args.nodes:
        if '-' in args.nodes:
            start, end = map(int, args.nodes.split('-'))
            node_ids = list(range(start, end + 1))
        else:
            node_ids = [int(x.strip()) for x in args.nodes.split(',')]
    else:
        parser.print_help()
        print("\n错误: 请指定要部署的节点 (--node 或 --nodes)")
        sys.exit(1)
    
    try:
        deployer = IarnetDeployer(args.vm_config, args.configs_dir)
        deployer.deploy_to_nodes(
            node_ids,
            restart=args.restart,
            image_name=args.image,
            max_workers=args.max_workers
        )
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)

if __name__ == '__main__':
    main()
