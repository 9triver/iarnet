#!/usr/bin/env python3
"""
在 k8s master 节点上部署 k8s provider 服务
支持为每个集群的 master 节点使用独立的配置文件
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

class K8sProviderDeployer:
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
        
        k8s_config = self.vm_config['vm_types']['k8s_clusters']
        self.master_config = k8s_config['master']
        self.cluster_count = k8s_config['count']
        
        # 处理配置文件目录路径
        if configs_dir is None:
            self.configs_dir = SCRIPT_DIR / 'k8s-provider-configs'
        else:
            configs_dir_obj = Path(configs_dir)
            if configs_dir_obj.is_absolute():
                self.configs_dir = configs_dir_obj
            else:
                # 相对路径：优先从脚本目录查找
                if (SCRIPT_DIR / configs_dir_obj.name).exists() or 'k8s-provider-configs' in str(configs_dir_obj):
                    self.configs_dir = SCRIPT_DIR / configs_dir_obj.name if configs_dir_obj.name == 'k8s-provider-configs' else SCRIPT_DIR / configs_dir_obj
                else:
                    self.configs_dir = PROJECT_ROOT / configs_dir_obj
        
        self.user = self.vm_config['global']['user']
        
        # 构建集群信息映射
        self.cluster_info = {}
        for cluster_id in range(self.cluster_count):
            ip_suffix = self.master_config['ip_start'] + cluster_id * self.master_config['ip_step']
            ip_address = f"{self.master_config['ip_base']}.{ip_suffix}"
            hostname = f"{self.master_config['hostname_prefix']}-{cluster_id+1:02d}{self.master_config['hostname_suffix']}"
            provider_port = self.master_config['provider_port_base'] + cluster_id
            self.cluster_info[cluster_id] = {
                'hostname': hostname,
                'ip': ip_address,
                'port': provider_port,
                'config_file': self.configs_dir / f"config-cluster-{cluster_id:02d}.yaml"
            }
    
    def _print(self, *args, **kwargs):
        """线程安全的打印函数"""
        with self._log_lock:
            # 确保立即刷新输出，避免缓冲导致卡住
            kwargs.setdefault('flush', True)
            print(*args, **kwargs)
    
    def check_cluster_connectivity(self, cluster_id: int) -> bool:
        """检查集群 master 节点连通性"""
        cluster = self.cluster_info[cluster_id]
        try:
            result = subprocess.run(
                ['ping', '-c', '1', '-W', '2', cluster['ip']],
                capture_output=True,
                timeout=5
            )
            return result.returncode == 0
        except:
            return False
    
    def _start_provider_service(self, ssh_cmd: list, cluster: dict, cluster_id: int = None) -> bool:
        """启动 k8s provider 服务"""
        cluster_prefix = f"[集群 {cluster_id}] " if cluster_id is not None else ""
        
        # 启动服务（使用后台执行，立即返回）
        # 使用 bash -c 确保命令在后台执行，并且 SSH 立即返回
        start_cmd = ' '.join(ssh_cmd) + ' "bash -c \'cd ~/k8s-provider && nohup ./k8s-provider --config config.yaml > k8s-provider.log 2>&1 &\'"'
        
        # 异步启动服务，不等待结果
        try:
            # 使用 Popen 启动，不等待完成
            proc = subprocess.Popen(start_cmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            # 立即返回，不等待进程完成
        except Exception as e:
            # 启动失败，但继续检查是否已有进程在运行
            self._print(f"{cluster_prefix}    ⚠ 启动命令执行异常（但可能已有进程在运行，继续检查...）")
        
        # 等待一小段时间让进程启动
        time.sleep(0.5)
        
        # 快速检查进程是否运行（只检查一次，设置短超时）
        check_cmd = ' '.join(ssh_cmd) + ' "pgrep -f k8s-provider > /dev/null 2>&1 && echo RUNNING || echo NOT_RUNNING"'
        try:
            check_result = subprocess.run(check_cmd, shell=True, check=False, timeout=2, capture_output=True, text=True)
            if 'RUNNING' in check_result.stdout:
                # 进程已运行，直接返回成功（不等待端口，因为服务可能还在初始化）
                self._print(f"{cluster_prefix}  ✓ k8s provider 进程已启动")
                return True
            else:
                # 即使进程未运行，也返回 True（避免阻塞，服务可能正在启动）
                self._print(f"{cluster_prefix}  ⚠ 进程检查未发现（但可能正在启动，继续部署）")
                return True
        except subprocess.TimeoutExpired:
            # 检查超时，假设已启动（避免阻塞）
            self._print(f"{cluster_prefix}  ⚠ 进程检查超时（假设已启动，继续部署）")
            return True
        except Exception as e:
            # 任何异常都返回 True，避免阻塞
            self._print(f"{cluster_prefix}  ⚠ 进程检查异常（假设已启动，继续部署）")
            return True
    
    def build_binary(self, force_rebuild: bool = False) -> Path:
        """在本地构建 k8s provider 二进制文件"""
        binary_path = PROJECT_ROOT / 'k8s-provider'
        provider_dir = PROJECT_ROOT / 'providers' / 'k8s'
        
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
        self._print("  正在构建 k8s provider 二进制文件...")
        self._print("  使用 Go 国内代理: https://goproxy.cn")
        
        # 设置 Go 代理环境变量（使用国内代理加速下载）
        env = os.environ.copy()
        env['GOPROXY'] = 'https://goproxy.cn,direct'
        env['GOSUMDB'] = 'sum.golang.org'
        # k8s provider 不需要 CGO
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
    
    def deploy_to_cluster(self, cluster_id: int, build: bool = False, restart: bool = False, binary_path: Path = None) -> bool:
        """部署到指定集群的 master 节点"""
        if cluster_id not in self.cluster_info:
            self._print(f"[集群 {cluster_id}] 错误: 集群 {cluster_id} 不存在")
            return False
        
        cluster = self.cluster_info[cluster_id]
        config_file = cluster['config_file']
        
        if not config_file.exists():
            self._print(f"[集群 {cluster_id}] 错误: 配置文件不存在: {config_file}")
            self._print(f"[集群 {cluster_id}] 请先运行: python3 deploy/generate_k8s_provider_configs.py --clusters {cluster_id}")
            return False
        
        self._print(f"\n[集群 {cluster_id}] 部署到节点: {cluster['hostname']} ({cluster['ip']})")
        self._print(f"[集群 {cluster_id}] " + "=" * 60)
        
        # 检查连通性
        if not self.check_cluster_connectivity(cluster_id):
            self._print(f"[集群 {cluster_id}] 警告: 无法连接到集群 {cluster_id} ({cluster['ip']})")
            self._print(f"[集群 {cluster_id}] 请确保虚拟机已启动且网络正常")
            return False
        
        # 构建SSH命令
        ssh_cmd = [
            'ssh',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f"{self.user}@{cluster['ip']}"
        ]
        
        # 0. 如果指定 restart，先停止现有服务
        if restart:
            self._print(f"[集群 {cluster_id}] 0. 停止现有服务...")
            # 读取配置文件获取端口
            with open(config_file, 'r', encoding='utf-8') as f:
                config_data = yaml.safe_load(f)
                provider_port = config_data.get('server', {}).get('port', cluster['port'])
            
            stop_commands = [
                'pkill -9 -f k8s-provider',
                'killall -9 k8s-provider 2>/dev/null',
                # 释放端口
                f'lsof -ti:{provider_port} | xargs kill -9 2>/dev/null',
                f'fuser -k {provider_port}/tcp 2>/dev/null'
            ]
            
            for cmd in stop_commands:
                stop_cmd = ' '.join(ssh_cmd) + f' "{cmd} || true"'
                try:
                    subprocess.run(stop_cmd, shell=True, check=False, timeout=5, capture_output=True)
                except subprocess.TimeoutExpired:
                    pass
            
            # 等待服务停止
            time.sleep(2)
            self._print(f"[集群 {cluster_id}]   ✓ 服务已停止")
        
        # 1. 创建必要的目录
        self._print(f"[集群 {cluster_id}] 1. 创建目录结构...")
        mkdir_cmd = ' '.join(ssh_cmd) + ' "mkdir -p ~/k8s-provider && ls -ld ~/k8s-provider"'
        mkdir_result = subprocess.run(mkdir_cmd, shell=True, check=False, capture_output=True, text=True, timeout=10)
        if mkdir_result.returncode != 0:
            self._print(f"[集群 {cluster_id}]   ⚠ 目录创建可能有问题: {mkdir_result.stderr}")
        else:
            self._print(f"[集群 {cluster_id}]   ✓ 目录结构创建成功")
        
        # 2. 上传配置文件
        self._print(f"[集群 {cluster_id}] 2. 上传配置文件: {config_file.name}...")
        scp_cmd = [
            'scp',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            str(config_file),
            f"{self.user}@{cluster['ip']}:~/k8s-provider/config.yaml"
        ]
        try:
            subprocess.run(scp_cmd, check=True, capture_output=True)
            self._print(f"[集群 {cluster_id}]   ✓ 配置文件上传成功")
        except subprocess.CalledProcessError as e:
            self._print(f"[集群 {cluster_id}]   ✗ 配置文件上传失败: {e}")
            return False
        
        # 3. 上传二进制文件（如果指定）
        if build:
            if binary_path is None:
                binary_path = PROJECT_ROOT / 'k8s-provider'
            
            if not binary_path.exists():
                self._print(f"[集群 {cluster_id}]   ✗ 错误: 二进制文件不存在: {binary_path}")
                return False
            
            self._print(f"[集群 {cluster_id}] 3. 上传二进制文件...")
            # 直接使用用户名构建远程主目录路径
            remote_home = f"/home/{self.user}"
            remote_path = f"{remote_home}/k8s-provider/k8s-provider"
            self._print(f"[集群 {cluster_id}]     用户名: {self.user}")
            self._print(f"[集群 {cluster_id}]     远程路径: {remote_path}")
            
            # 先验证目标目录是否存在且有写权限
            verify_dir_cmd = ' '.join(ssh_cmd) + f' "test -d {remote_home}/k8s-provider && test -w {remote_home}/k8s-provider && echo OK || echo FAIL"'
            verify_result = subprocess.run(verify_dir_cmd, shell=True, check=False, capture_output=True, text=True, timeout=5)
            verify_output_lines = [line.strip() for line in verify_result.stdout.split('\n') if line.strip() and 'Warning:' not in line]
            verify_output = verify_output_lines[-1] if verify_output_lines else verify_result.stdout.strip()
            
            if 'OK' not in verify_output:
                self._print(f"[集群 {cluster_id}]   ✗ 目标目录 {remote_home}/k8s-provider 不存在或没有写权限")
                fix_dir_cmd = ' '.join(ssh_cmd) + f' "mkdir -p {remote_home}/k8s-provider && chmod 755 {remote_home}/k8s-provider"'
                fix_result = subprocess.run(fix_dir_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
                if fix_result.returncode != 0:
                    self._print(f"[集群 {cluster_id}]   ✗ 无法创建目录")
                    return False
            
            # 检查并删除已存在的目标文件
            self._print(f"[集群 {cluster_id}]     检查目标文件是否存在...")
            check_target_cmd = ' '.join(ssh_cmd) + f' "if [ -e {remote_path} ]; then if [ -d {remote_path} ]; then echo IS_DIR; else echo IS_FILE; fi; else echo NOT_EXISTS; fi"'
            check_result = subprocess.run(check_target_cmd, shell=True, check=False, capture_output=True, text=True, timeout=5)
            target_status_lines = [line.strip() for line in check_result.stdout.split('\n') if line.strip() and 'Warning:' not in line]
            target_status = target_status_lines[-1] if target_status_lines else check_result.stdout.strip()
            
            if 'IS_DIR' in target_status:
                self._print(f"[集群 {cluster_id}]     ℹ 目标路径已存在且是目录，正在删除...")
                rm_cmd = ' '.join(ssh_cmd) + f' "rm -rf {remote_path}"'
                subprocess.run(rm_cmd, shell=True, check=True, timeout=10, capture_output=True, text=True)
                self._print(f"[集群 {cluster_id}]     ✓ 目录已删除")
            elif 'IS_FILE' in target_status:
                self._print(f"[集群 {cluster_id}]     ℹ 目标文件已存在，正在删除...")
                rm_cmd = ' '.join(ssh_cmd) + f' "rm -f {remote_path}"'
                subprocess.run(rm_cmd, shell=True, check=True, timeout=10, capture_output=True, text=True)
                self._print(f"[集群 {cluster_id}]     ✓ 文件已删除")
            
            # 上传二进制文件
            self._print(f"[集群 {cluster_id}]     正在上传二进制文件...")
            scp_binary_cmd = [
                'scp',
                '-o', 'StrictHostKeyChecking=no',
                '-o', 'UserKnownHostsFile=/dev/null',
                '-o', 'ConnectTimeout=10',
                str(binary_path),
                f"{self.user}@{cluster['ip']}:{remote_path}"
            ]
            try:
                result = subprocess.run(scp_binary_cmd, check=True, capture_output=True, text=True, timeout=30)
                self._print(f"[集群 {cluster_id}]   ✓ 二进制文件上传成功")
            except subprocess.TimeoutExpired:
                self._print(f"[集群 {cluster_id}]   ✗ 二进制文件上传超时")
                return False
            except subprocess.CalledProcessError as e:
                error_msg = e.stderr if e.stderr else (e.stdout if e.stdout else str(e))
                self._print(f"[集群 {cluster_id}]   ✗ 二进制文件上传失败")
                self._print(f"[集群 {cluster_id}]   错误信息: {error_msg}")
                return False
            
            # 设置执行权限
            try:
                chmod_cmd = ' '.join(ssh_cmd) + ' "chmod +x ~/k8s-provider/k8s-provider"'
                subprocess.run(chmod_cmd, shell=True, check=True, timeout=10, capture_output=True)
            except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as e:
                self._print(f"[集群 {cluster_id}]   ⚠ 设置执行权限失败，但继续执行: {e}")
            
            # 部署完后立即启动服务
            self._print(f"[集群 {cluster_id}]     启动 k8s provider 服务...")
            provider_running = self._start_provider_service(ssh_cmd, cluster, cluster_id)
            if provider_running:
                self._print(f"[集群 {cluster_id}]   ✓ k8s provider 服务启动成功")
            else:
                self._print(f"[集群 {cluster_id}]   ⚠ k8s provider 服务启动失败，请检查日志: ~/k8s-provider/k8s-provider.log")
        
        # 如果只指定 restart（没有 build），则只启动服务
        elif restart:
            self._print(f"[集群 {cluster_id}]     启动 k8s provider 服务...")
            provider_running = self._start_provider_service(ssh_cmd, cluster, cluster_id)
            if provider_running:
                self._print(f"[集群 {cluster_id}]   ✓ k8s provider 服务启动成功")
            else:
                self._print(f"[集群 {cluster_id}]   ⚠ k8s provider 服务启动失败，请检查日志: ~/k8s-provider/k8s-provider.log")
        
        self._print(f"[集群 {cluster_id}] " + "=" * 60)
        return True
    
    def deploy_to_clusters(self, cluster_ids: list, build: bool = False, restart: bool = False, max_workers: int = None):
        """批量部署到多个集群（并行执行）"""
        self._print(f"批量部署到集群: {cluster_ids}")
        self._print("=" * 60)
        
        # 如果需要构建，先构建一次，所有集群复用同一个二进制文件
        binary_path = None
        if build:
            self._print("\n在本地构建 k8s provider 二进制文件（所有集群将复用此文件）...")
            try:
                binary_path = self.build_binary(force_rebuild=False)
            except Exception as e:
                self._print(f"错误: 构建失败，无法继续部署: {e}")
                return
        
        # 并行部署到各个集群
        self._print(f"\n开始并行部署到 {len(cluster_ids)} 个集群...")
        if max_workers is None:
            # 默认使用集群数量，但不超过 10 个并发
            max_workers = min(len(cluster_ids), 10)
        
        success_count = 0
        failed_clusters = []
        
        # 使用线程池并行部署
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            # 提交所有部署任务
            future_to_cluster = {
                executor.submit(
                    self.deploy_to_cluster,
                    cluster_id,
                    build=build,
                    restart=restart,
                    binary_path=binary_path
                ): cluster_id
                for cluster_id in cluster_ids
            }
            
            # 等待所有任务完成
            # 使用超时机制避免单个集群卡住整个流程
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
            
            for future in as_completed(future_to_cluster):
                cluster_id = future_to_cluster[future]
                try:
                    # 设置超时获取结果，避免单个集群卡住整个流程
                    result = get_result_with_timeout(future, timeout=60)
                    if result:
                        success_count += 1
                        self._print(f"[集群 {cluster_id}] ✓ 部署成功")
                    else:
                        failed_clusters.append(cluster_id)
                        self._print(f"[集群 {cluster_id}] ✗ 部署失败")
                except TimeoutError:
                    failed_clusters.append(cluster_id)
                    self._print(f"[集群 {cluster_id}] ✗ 部署超时（跳过，可能仍在后台运行）")
                except Exception as e:
                    failed_clusters.append(cluster_id)
                    self._print(f"[集群 {cluster_id}] ✗ 部署异常: {e}")
        
        # 输出部署结果摘要
        self._print("\n" + "=" * 60)
        self._print(f"部署完成: {success_count}/{len(cluster_ids)} 个集群成功")
        if failed_clusters:
            self._print(f"失败的集群: {failed_clusters}")
        self._print("=" * 60)

def main():
    parser = argparse.ArgumentParser(description='在 k8s master 节点上部署 k8s provider 服务')
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--configs-dir', '-c',
        default=str(SCRIPT_DIR / 'k8s-provider-configs'),
        help='配置文件目录 (默认: deploy/k8s-provider-configs)'
    )
    parser.add_argument(
        '--clusters', '-n',
        type=str,
        default='0-39',
        help='集群范围，格式: start-end 或逗号分隔的列表 (默认: 0-39，即所有40个集群)'
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
        help='最大并发部署集群数（默认: min(集群数, 10)）'
    )
    
    args = parser.parse_args()
    
    # 解析集群范围
    if '-' in args.clusters:
        start, end = map(int, args.clusters.split('-'))
        cluster_ids = list(range(start, end + 1))
    else:
        cluster_ids = [int(x.strip()) for x in args.clusters.split(',')]
    
    # 创建部署器
    try:
        deployer = K8sProviderDeployer(args.vm_config, args.configs_dir)
    except Exception as e:
        print(f"错误: 初始化部署器失败: {e}")
        sys.exit(1)
    
    # 验证集群ID范围
    max_cluster_id = deployer.cluster_count - 1
    invalid_clusters = [c for c in cluster_ids if c < 0 or c > max_cluster_id]
    if invalid_clusters:
        print(f"错误: 集群ID超出范围: {invalid_clusters}")
        print(f"有效范围: 0-{max_cluster_id}")
        sys.exit(1)
    
    # 执行部署
    deployer.deploy_to_clusters(
        cluster_ids=cluster_ids,
        build=args.build,
        restart=args.restart,
        max_workers=args.max_workers
    )

if __name__ == '__main__':
    main()

