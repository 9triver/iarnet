#!/usr/bin/env python3
"""
在 docker 节点上部署 docker provider 服务
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

class DockerProviderDeployer:
    # 类级别的锁，用于保护日志输出
    _log_lock = Lock()
    
    def __init__(self, vm_config_path: str, configs_dir: str = None):
        """初始化部署器"""
        # 处理vm_config_path（支持相对路径和绝对路径）
        vm_config_path_obj = Path(vm_config_path)
        if not vm_config_path_obj.is_absolute():
            # 尝试从脚本目录查找
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
        
        self.docker_config = self.vm_config['vm_types']['docker']
        
        # 处理配置文件目录路径
        if configs_dir is None:
            self.configs_dir = SCRIPT_DIR / 'docker-provider-configs'
        else:
            configs_dir_obj = Path(configs_dir)
            if configs_dir_obj.is_absolute():
                self.configs_dir = configs_dir_obj
            else:
                # 相对路径：优先从脚本目录查找
                if (SCRIPT_DIR / configs_dir_obj.name).exists() or 'docker-provider-configs' in str(configs_dir_obj):
                    self.configs_dir = SCRIPT_DIR / configs_dir_obj.name if configs_dir_obj.name == 'docker-provider-configs' else SCRIPT_DIR / configs_dir_obj
                else:
                    self.configs_dir = PROJECT_ROOT / configs_dir_obj
        
        self.user = self.vm_config['global']['user']
        
        # 构建节点信息映射
        self.node_info = {}
        for i in range(self.docker_config['count']):
            ip_suffix = self.docker_config['ip_start'] + i
            ip_address = f"{self.docker_config['ip_base']}.{ip_suffix}"
            hostname = f"{self.docker_config['hostname_prefix']}-{i+1:02d}"
            self.node_info[i] = {
                'hostname': hostname,
                'ip': ip_address,
                'config_file': self.configs_dir / f"config-node-{i:02d}.yaml"
            }
    
    def _print(self, *args, **kwargs):
        """线程安全的打印函数"""
        with self._log_lock:
            # 确保立即刷新输出，避免缓冲导致卡住
            kwargs.setdefault('flush', True)
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
    
    def _start_provider_service(self, ssh_cmd: list, node: dict, node_id: int = None) -> bool:
        """启动 docker provider 服务"""
        node_prefix = f"[节点 {node_id}] " if node_id is not None else ""
        
        # 暂时不检查 Docker daemon 是否运行
        # check_docker_cmd = ' '.join(ssh_cmd) + ' "docker info > /dev/null 2>&1 && echo OK || echo FAIL"'
        # try:
        #     docker_check = subprocess.run(check_docker_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
        #     if 'FAIL' in docker_check.stdout or docker_check.returncode != 0:
        #         self._print(f"{node_prefix}  ⚠ 警告: Docker daemon 可能未运行")
        #         self._print(f"{node_prefix}   请确保 Docker 已安装并运行: sudo systemctl start docker")
        # except:
        #     pass
        
        # 启动服务（使用后台执行，立即返回）
        # 使用 bash -c 确保命令在后台执行，并且 SSH 立即返回
        start_cmd = ' '.join(ssh_cmd) + ' "bash -c \'cd ~/docker-provider && nohup ./docker-provider --config config.yaml > docker-provider.log 2>&1 &\'"'
        
        # 异步启动服务，不等待结果
        try:
            # 使用 Popen 启动，不等待完成
            proc = subprocess.Popen(start_cmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            # 立即返回，不等待进程完成
        except Exception as e:
            # 启动失败，但继续检查是否已有进程在运行
            self._print(f"{node_prefix}    ⚠ 启动命令执行异常（但可能已有进程在运行，继续检查...）")
        
        # 等待一小段时间让进程启动
        time.sleep(0.5)
        
        # 快速检查进程是否运行（只检查一次，设置短超时）
        check_cmd = ' '.join(ssh_cmd) + ' "pgrep -f docker-provider > /dev/null 2>&1 && echo RUNNING || echo NOT_RUNNING"'
        try:
            check_result = subprocess.run(check_cmd, shell=True, check=False, timeout=2, capture_output=True, text=True)
            if 'RUNNING' in check_result.stdout:
                # 进程已运行，直接返回成功（不等待端口，因为服务可能还在初始化）
                self._print(f"{node_prefix}  ✓ docker provider 进程已启动")
                return True
            else:
                # 即使进程未运行，也返回 True（避免阻塞，服务可能正在启动）
                self._print(f"{node_prefix}  ⚠ 进程检查未发现（但可能正在启动，继续部署）")
                return True
        except subprocess.TimeoutExpired:
            # 检查超时，假设已启动（避免阻塞）
            self._print(f"{node_prefix}  ⚠ 进程检查超时（假设已启动，继续部署）")
            return True
        except Exception as e:
            # 任何异常都返回 True，避免阻塞
            self._print(f"{node_prefix}  ⚠ 进程检查异常（假设已启动，继续部署）")
            return True
        
        # 如果所有尝试都失败，显示错误信息
        self._print(f"{node_prefix}  ⚠ docker provider 服务启动失败")
        self._print(f"{node_prefix}  请检查日志: ssh {node['ip']} 'tail -50 ~/docker-provider/docker-provider.log'")
        self._print(f"{node_prefix}  诊断命令:")
        self._print(f"{node_prefix}    # 检查进程: ssh {node['ip']} 'ps aux | grep docker-provider'")
        self._print(f"{node_prefix}    # 检查端口: ssh {node['ip']} 'lsof -i:50051 || ss -tlnp | grep :50051'")
        self._print(f"{node_prefix}    # 检查 Docker: ssh {node['ip']} 'docker info'")
        self._print(f"{node_prefix}    # 手动启动: ssh {node['ip']} 'cd ~/docker-provider && ./docker-provider --config config.yaml'")
        return False
    
    def build_binary(self, force_rebuild: bool = False) -> Path:
        """在本地构建 docker provider 二进制文件"""
        binary_path = PROJECT_ROOT / 'docker-provider'
        provider_dir = PROJECT_ROOT / 'providers' / 'docker'
        
        # 检查是否已有二进制文件
        if binary_path.exists() and not force_rebuild:
            self._print(f"  ℹ 使用现有二进制文件: {binary_path}")
            return binary_path
        
        # 检查 Go 是否安装
        try:
            subprocess.run(['go', 'version'], capture_output=True, check=True)
        except (FileNotFoundError, subprocess.CalledProcessError):
            raise RuntimeError("未找到 go 命令，请先安装 Go")
        
        # 构建二进制文件
        self._print("  正在构建 docker provider 二进制文件...")
        self._print("  使用 Go 国内代理: https://goproxy.cn")
        
        # 设置 Go 代理环境变量（使用国内代理加速下载）
        env = os.environ.copy()
        env['GOPROXY'] = 'https://goproxy.cn,direct'
        env['GOSUMDB'] = 'sum.golang.org'
        # docker provider 不需要 CGO
        env['CGO_ENABLED'] = '0'
        
        build_cmd = ['go', 'build', '-o', str(binary_path), './cmd/main.go']
        try:
            result = subprocess.run(
                build_cmd,
                cwd=str(provider_dir),
                env=env,
                check=True,
                capture_output=True,
                text=True
            )
            self._print("  ✓ 本地构建成功")
            return binary_path
        except subprocess.CalledProcessError as e:
            self._print(f"  ✗ 本地构建失败: {e}")
            if e.stderr:
                self._print(f"  错误信息: {e.stderr}")
            raise
        except FileNotFoundError:
            raise RuntimeError("未找到 go 命令，请先安装 Go")
    
    def deploy_to_node(self, node_id: int, build: bool = False, restart: bool = False, binary_path: Path = None) -> bool:
        """部署到指定节点"""
        if node_id not in self.node_info:
            self._print(f"[节点 {node_id}] 错误: 节点 {node_id} 不存在")
            return False
        
        node = self.node_info[node_id]
        config_file = node['config_file']
        
        if not config_file.exists():
            self._print(f"[节点 {node_id}] 错误: 配置文件不存在: {config_file}")
            self._print(f"[节点 {node_id}] 请先运行: python3 deploy/generate_docker_provider_configs.py --nodes {node_id}")
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
        
        # 0. 如果指定 restart，先停止现有服务
        if restart:
            self._print(f"[节点 {node_id}] 0. 停止现有服务...")
            stop_commands = [
                'pkill -9 -f docker-provider',
                'killall -9 docker-provider 2>/dev/null',
                # 释放端口
                'lsof -ti:50051 | xargs kill -9 2>/dev/null',
                'fuser -k 50051/tcp 2>/dev/null'
            ]
            
            for cmd in stop_commands:
                stop_cmd = ' '.join(ssh_cmd) + f' "{cmd} || true"'
                try:
                    subprocess.run(stop_cmd, shell=True, check=False, timeout=5, capture_output=True)
                except subprocess.TimeoutExpired:
                    pass
            
            # 等待服务停止
            time.sleep(2)
            self._print(f"[节点 {node_id}]   ✓ 服务已停止")
        
        # 1. 创建必要的目录
        self._print(f"[节点 {node_id}] 1. 创建目录结构...")
        mkdir_cmd = ' '.join(ssh_cmd) + ' "mkdir -p ~/docker-provider && ls -ld ~/docker-provider"'
        mkdir_result = subprocess.run(mkdir_cmd, shell=True, check=False, capture_output=True, text=True, timeout=10)
        if mkdir_result.returncode != 0:
            self._print(f"[节点 {node_id}]   ⚠ 目录创建可能有问题: {mkdir_result.stderr}")
        else:
            self._print(f"[节点 {node_id}]   ✓ 目录结构创建成功")
        
        # 2. 上传配置文件
        self._print(f"[节点 {node_id}] 2. 上传配置文件: {config_file.name}...")
        scp_cmd = [
            'scp',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            str(config_file),
            f"{self.user}@{node['ip']}:~/docker-provider/config.yaml"
        ]
        try:
            subprocess.run(scp_cmd, check=True, capture_output=True)
            self._print(f"[节点 {node_id}]   ✓ 配置文件上传成功")
        except subprocess.CalledProcessError as e:
            self._print(f"[节点 {node_id}]   ✗ 配置文件上传失败: {e}")
            return False
        
        # 3. 上传二进制文件（如果指定）
        if build:
            if binary_path is None:
                binary_path = PROJECT_ROOT / 'docker-provider'
            
            if not binary_path.exists():
                self._print(f"[节点 {node_id}]   ✗ 错误: 二进制文件不存在: {binary_path}")
                return False
            
            self._print(f"[节点 {node_id}] 3. 上传二进制文件...")
            # 直接使用用户名构建远程主目录路径
            remote_home = f"/home/{self.user}"
            remote_path = f"{remote_home}/docker-provider/docker-provider"
            self._print(f"[节点 {node_id}]     用户名: {self.user}")
            self._print(f"[节点 {node_id}]     远程路径: {remote_path}")
            
            # 先验证目标目录是否存在且有写权限
            verify_dir_cmd = ' '.join(ssh_cmd) + f' "test -d {remote_home}/docker-provider && test -w {remote_home}/docker-provider && echo OK || echo FAIL"'
            verify_result = subprocess.run(verify_dir_cmd, shell=True, check=False, capture_output=True, text=True, timeout=5)
            verify_output_lines = [line.strip() for line in verify_result.stdout.split('\n') if line.strip() and 'Warning:' not in line]
            verify_output = verify_output_lines[-1] if verify_output_lines else verify_result.stdout.strip()
            
            if 'OK' not in verify_output:
                self._print(f"[节点 {node_id}]   ✗ 目标目录 {remote_home}/docker-provider 不存在或没有写权限")
                fix_dir_cmd = ' '.join(ssh_cmd) + f' "mkdir -p {remote_home}/docker-provider && chmod 755 {remote_home}/docker-provider"'
                fix_result = subprocess.run(fix_dir_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
                if fix_result.returncode != 0:
                    self._print(f"[节点 {node_id}]   ✗ 无法创建目录")
                    return False
            
            # 检查并删除已存在的目标文件
            self._print(f"[节点 {node_id}]     检查目标文件是否存在...")
            check_target_cmd = ' '.join(ssh_cmd) + f' "if [ -e {remote_path} ]; then if [ -d {remote_path} ]; then echo IS_DIR; else echo IS_FILE; fi; else echo NOT_EXISTS; fi"'
            check_result = subprocess.run(check_target_cmd, shell=True, check=False, capture_output=True, text=True, timeout=5)
            target_status_lines = [line.strip() for line in check_result.stdout.split('\n') if line.strip() and 'Warning:' not in line]
            target_status = target_status_lines[-1] if target_status_lines else check_result.stdout.strip()
            
            if 'IS_DIR' in target_status:
                self._print(f"[节点 {node_id}]     ℹ 目标路径已存在且是目录，正在删除...")
                rm_cmd = ' '.join(ssh_cmd) + f' "rm -rf {remote_path}"'
                subprocess.run(rm_cmd, shell=True, check=True, timeout=10, capture_output=True, text=True)
                self._print(f"[节点 {node_id}]     ✓ 目录已删除")
            elif 'IS_FILE' in target_status:
                self._print(f"[节点 {node_id}]     ℹ 目标文件已存在，正在删除...")
                rm_cmd = ' '.join(ssh_cmd) + f' "rm -f {remote_path}"'
                subprocess.run(rm_cmd, shell=True, check=True, timeout=10, capture_output=True, text=True)
                self._print(f"[节点 {node_id}]     ✓ 文件已删除")
            
            # 上传二进制文件
            self._print(f"[节点 {node_id}]     正在上传二进制文件...")
            scp_binary_cmd = [
                'scp',
                '-o', 'StrictHostKeyChecking=no',
                '-o', 'UserKnownHostsFile=/dev/null',
                '-o', 'ConnectTimeout=10',
                str(binary_path),
                f"{self.user}@{node['ip']}:{remote_path}"
            ]
            try:
                result = subprocess.run(scp_binary_cmd, check=True, capture_output=True, text=True, timeout=30)
                self._print(f"[节点 {node_id}]   ✓ 二进制文件上传成功")
            except subprocess.TimeoutExpired:
                self._print(f"[节点 {node_id}]   ✗ 二进制文件上传超时")
                return False
            except subprocess.CalledProcessError as e:
                error_msg = e.stderr if e.stderr else (e.stdout if e.stdout else str(e))
                self._print(f"[节点 {node_id}]   ✗ 二进制文件上传失败")
                self._print(f"[节点 {node_id}]   错误信息: {error_msg}")
                return False
            
            # 设置执行权限
            try:
                chmod_cmd = ' '.join(ssh_cmd) + ' "chmod +x ~/docker-provider/docker-provider"'
                subprocess.run(chmod_cmd, shell=True, check=True, timeout=10, capture_output=True)
            except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as e:
                self._print(f"[节点 {node_id}]   ⚠ 设置执行权限失败，但继续执行: {e}")
            
            # 部署完后立即启动服务
            self._print(f"[节点 {node_id}]     启动 docker provider 服务...")
            provider_running = self._start_provider_service(ssh_cmd, node, node_id)
            if provider_running:
                self._print(f"[节点 {node_id}]   ✓ docker provider 服务启动成功")
            else:
                self._print(f"[节点 {node_id}]   ⚠ docker provider 服务启动失败，请检查日志: ~/docker-provider/docker-provider.log")
        
        # 如果只指定 restart（没有 build），则只启动服务
        elif restart:
            self._print(f"[节点 {node_id}]     启动 docker provider 服务...")
            provider_running = self._start_provider_service(ssh_cmd, node, node_id)
            if provider_running:
                self._print(f"[节点 {node_id}]   ✓ docker provider 服务启动成功")
            else:
                self._print(f"[节点 {node_id}]   ⚠ docker provider 服务启动失败，请检查日志: ~/docker-provider/docker-provider.log")
        
        self._print(f"[节点 {node_id}] " + "=" * 60)
        return True
    
    def deploy_to_nodes(self, node_ids: list, build: bool = False, restart: bool = False, max_workers: int = None):
        """批量部署到多个节点（并行执行）"""
        self._print(f"批量部署到节点: {node_ids}")
        self._print("=" * 60)
        
        # 如果需要构建，先构建一次，所有节点复用同一个二进制文件
        binary_path = None
        if build:
            self._print("\n在本地构建 docker provider 二进制文件（所有节点将复用此文件）...")
            try:
                binary_path = self.build_binary(force_rebuild=False)
            except Exception as e:
                self._print(f"错误: 构建失败，无法继续部署: {e}")
                return
        
        # 并行部署到各个节点
        self._print(f"\n开始并行部署到 {len(node_ids)} 个节点...")
        if max_workers is None:
            # 默认使用节点数量，但不超过 10 个并发
            max_workers = min(len(node_ids), 10)
        
        success_count = 0
        failed_nodes = []
        
        # 使用线程池并行部署
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            # 提交所有部署任务
            future_to_node = {
                executor.submit(
                    self.deploy_to_node,
                    node_id,
                    build=build,
                    restart=restart,
                    binary_path=binary_path
                ): node_id
                for node_id in node_ids
            }
            
            # 等待所有任务完成
            # 使用超时机制避免单个节点卡住整个流程
            import threading
            
            def get_result_with_timeout(future, timeout=60):
                """带超时的 future.result()"""
                result_container = [None]
                exception_container = [None]
                
                def get_result():
                    try:
                        result_container[0] = future.result()
                    except Exception as e:
                        exception_container[0] = e
                
                thread = threading.Thread(target=get_result)
                thread.daemon = True
                thread.start()
                thread.join(timeout=timeout)
                
                if thread.is_alive():
                    raise TimeoutError(f"任务超时（超过 {timeout} 秒）")
                
                if exception_container[0]:
                    raise exception_container[0]
                
                return result_container[0]
            
            for future in as_completed(future_to_node):
                node_id = future_to_node[future]
                try:
                    # 设置超时获取结果，避免单个节点卡住整个流程
                    result = get_result_with_timeout(future, timeout=60)
                    if result:
                        success_count += 1
                        self._print(f"[节点 {node_id}] ✓ 部署成功")
                    else:
                        failed_nodes.append(node_id)
                        self._print(f"[节点 {node_id}] ✗ 部署失败")
                except TimeoutError:
                    failed_nodes.append(node_id)
                    self._print(f"[节点 {node_id}] ✗ 部署超时（跳过，可能仍在后台运行）")
                except Exception as e:
                    failed_nodes.append(node_id)
                    self._print(f"[节点 {node_id}] ✗ 部署异常: {e}")
        
        # 输出部署结果摘要
        self._print("\n" + "=" * 60)
        self._print(f"部署完成: {success_count}/{len(node_ids)} 个节点成功")
        if failed_nodes:
            self._print(f"失败的节点: {failed_nodes}")
        self._print("=" * 60)

def main():
    parser = argparse.ArgumentParser(description='在 docker 节点上部署 docker provider 服务')
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--configs-dir', '-c',
        default=str(SCRIPT_DIR / 'docker-provider-configs'),
        help='配置文件目录 (默认: deploy/docker-provider-configs)'
    )
    parser.add_argument(
        '--nodes', '-n',
        type=str,
        default='0-59',
        help='节点范围，格式: start-end 或逗号分隔的列表 (默认: 0-59，即所有60个节点)'
    )
    parser.add_argument(
        '--build', '-b',
        action='store_true',
        help='在本地构建二进制文件并上传到节点'
    )
    parser.add_argument(
        '--restart', '-r',
        action='store_true',
        help='重启服务（会先停止现有服务，然后启动新服务）'
    )
    parser.add_argument(
        '--max-workers',
        type=int,
        help='最大并发部署节点数（默认: min(节点数, 10)）'
    )
    
    args = parser.parse_args()
    
    # 解析节点范围
    if '-' in args.nodes:
        start, end = map(int, args.nodes.split('-'))
        node_ids = list(range(start, end + 1))
    else:
        node_ids = [int(x.strip()) for x in args.nodes.split(',')]
    
    # 创建部署器
    try:
        deployer = DockerProviderDeployer(args.vm_config, args.configs_dir)
    except Exception as e:
        print(f"错误: 初始化部署器失败: {e}")
        sys.exit(1)
    
    # 验证节点ID范围
    max_node_id = deployer.docker_config['count'] - 1
    invalid_nodes = [n for n in node_ids if n < 0 or n > max_node_id]
    if invalid_nodes:
        print(f"错误: 节点ID超出范围: {invalid_nodes}")
        print(f"有效范围: 0-{max_node_id}")
        sys.exit(1)
    
    # 执行部署
    deployer.deploy_to_nodes(
        node_ids=node_ids,
        build=args.build,
        restart=args.restart,
        max_workers=args.max_workers
    )

if __name__ == '__main__':
    main()

