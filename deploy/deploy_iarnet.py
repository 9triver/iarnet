#!/usr/bin/env python3
"""
在 iarnet 节点上部署 iarnet 服务
支持为每个节点使用独立的配置文件
"""

import os
import sys
import yaml
import argparse
import subprocess
import time
import tempfile
import tarfile
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
from threading import Lock

# 获取脚本所在目录和项目根目录
SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent

class IarnetDeployer:
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
        
        self.iarnet_config = self.vm_config['vm_types']['iarnet']
        
        # 处理配置文件目录路径
        if configs_dir is None:
            self.configs_dir = SCRIPT_DIR / 'iarnet-configs'
        else:
            configs_dir_obj = Path(configs_dir)
            if configs_dir_obj.is_absolute():
                self.configs_dir = configs_dir_obj
            else:
                # 相对路径：优先从脚本目录查找
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
    
    def _start_backend_service(self, ssh_cmd: list, node: dict, node_id: int = None) -> bool:
        """启动后端服务"""
        # 先检查 ZeroMQ 运行时库
        check_lib_cmd = ' '.join(ssh_cmd) + ' "ldconfig -p | grep -E \"(libzmq|libczmq)\" > /dev/null && echo OK || echo MISSING"'
        try:
            lib_check = subprocess.run(check_lib_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
            if 'MISSING' in lib_check.stdout:
                node_prefix = f"[节点 {node_id}] " if node_id is not None else ""
                self._print(f"{node_prefix}    ⚠ 警告: ZeroMQ 运行时库可能未正确安装，尝试更新库缓存...")
                update_cache_cmd = ' '.join(ssh_cmd) + ' "sudo ldconfig"'
                subprocess.run(update_cache_cmd, shell=True, check=False, timeout=10, capture_output=True)
        except:
            pass
        
        start_backend_cmd = ' '.join(ssh_cmd) + ' "cd ~/iarnet && nohup ./iarnet --config=config.yaml > iarnet.log 2>&1 & sleep 2 && pgrep -f iarnet > /dev/null && echo started || echo failed"'
        for attempt in range(3):
            try:
                result = subprocess.run(start_backend_cmd, shell=True, check=True, timeout=30, capture_output=True, text=True)
                if 'started' in result.stdout:
                    # 等待一下，检查日志看是否有错误
                    time.sleep(2)
                    # 检查日志中是否有 ZeroMQ 错误
                    check_log_cmd = ' '.join(ssh_cmd) + ' "tail -30 ~/iarnet/iarnet.log 2>/dev/null | grep -i \"zmq\\|zeromq\\|error attaching\" || echo NO_ERROR"'
                    log_check = subprocess.run(check_log_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
                    if 'error attaching' in log_check.stdout.lower() or 'zmq' in log_check.stdout.lower():
                        node_prefix = f"[节点 {node_id}] " if node_id is not None else ""
                        self._print(f"{node_prefix}    ⚠ 检测到 ZeroMQ 错误，请检查日志: ~/iarnet/iarnet.log")
                        self._print(f"{node_prefix}    建议运行诊断脚本: python3 deploy/ssh_vm.py {node['hostname']} 'bash -s' < deploy/diagnose_zmq.sh")
                    
                    # 再次验证服务是否真的在运行
                    verify_cmd = ' '.join(ssh_cmd) + ' "pgrep -f iarnet > /dev/null && echo running || echo not_running"'
                    verify_result = subprocess.run(verify_cmd, shell=True, check=True, timeout=5, capture_output=True, text=True)
                    if 'running' in verify_result.stdout:
                        return True
                elif attempt < 2:
                    time.sleep(3)
            except (subprocess.TimeoutExpired, subprocess.CalledProcessError):
                if attempt < 2:
                    time.sleep(3)
        return False
    
    def _start_frontend_service(self, ssh_cmd: list, node: dict, node_id: int = None) -> bool:
        """启动前端服务"""
        node_prefix = f"[节点 {node_id}] " if node_id is not None else ""
        self._print(f"{node_prefix}    停止旧的前端服务并释放端口...")
        
        # 使用多种方法停止前端进程
        stop_commands = [
            'pkill -9 -f "next start"',
            'pkill -9 -f "node.*next"',
            'killall -9 node 2>/dev/null',
            'ps aux | grep -E "next|node.*3000" | grep -v grep | awk \'{print $2}\' | xargs kill -9 2>/dev/null'
        ]
        
        for cmd in stop_commands:
            stop_cmd = ' '.join(ssh_cmd) + f' "{cmd} || true"'
            try:
                subprocess.run(stop_cmd, shell=True, check=False, timeout=5, capture_output=True)
            except subprocess.TimeoutExpired:
                pass
        
        # 使用多种方法释放3000端口
        port_release_commands = [
            # 方法1: 使用 lsof
            'lsof -ti:3000 | xargs kill -9 2>/dev/null',
            # 方法2: 使用 fuser（如果可用）
            'fuser -k 3000/tcp 2>/dev/null',
            # 方法3: 使用 netstat + kill
            'netstat -tlnp 2>/dev/null | grep :3000 | awk \'{print $7}\' | cut -d/ -f1 | xargs kill -9 2>/dev/null',
            # 方法4: 使用 ss + kill
            'ss -tlnp | grep :3000 | grep -oP \'pid=\\K\\d+\' | xargs kill -9 2>/dev/null'
        ]
        
        for cmd in port_release_commands:
            release_cmd = ' '.join(ssh_cmd) + f' "{cmd} || true"'
            try:
                subprocess.run(release_cmd, shell=True, check=False, timeout=5, capture_output=True)
            except subprocess.TimeoutExpired:
                pass
        
        # 等待端口释放（增加等待时间，确保 TIME_WAIT 状态结束）
        self._print(f"{node_prefix}    等待端口完全释放...")
        for wait_attempt in range(5):
            time.sleep(2)
            # 检查端口是否真的空闲（包括 TIME_WAIT 状态）
            check_port_cmd = ' '.join(ssh_cmd) + ' "(lsof -i:3000 2>/dev/null || ss -tlnp | grep :3000 || netstat -tlnp 2>/dev/null | grep :3000 || echo PORT_FREE) | head -1"'
            try:
                check_result = subprocess.run(check_port_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
                output = check_result.stdout.strip()
                if 'PORT_FREE' in output or not output:
                    self._print(f"{node_prefix}    ✓ 端口3000已释放")
                    break
                else:
                    # 检查是否是 TIME_WAIT 状态
                    if 'TIME_WAIT' in output or 'TIME-WAIT' in output:
                        self._print(f"{node_prefix}    端口处于 TIME_WAIT 状态，继续等待... ({wait_attempt + 1}/5)")
                    else:
                        self._print(f"{node_prefix}    ⚠ 端口可能仍被占用: {output[:80]}")
                        # 再次尝试强制释放
                        force_release_cmd = ' '.join(ssh_cmd) + ' "for pid in $(lsof -ti:3000 2>/dev/null); do kill -9 $pid 2>/dev/null; done"'
                        subprocess.run(force_release_cmd, shell=True, check=False, timeout=5, capture_output=True)
            except subprocess.TimeoutExpired:
                pass
        
        # 最后检查一次，如果还是被占用，尝试使用不同的方法
        final_check_cmd = ' '.join(ssh_cmd) + ' "timeout 1 bash -c \'</dev/tcp/localhost/3000\' 2>/dev/null && echo PORT_IN_USE || echo PORT_FREE"'
        try:
            final_check = subprocess.run(final_check_cmd, shell=True, check=False, timeout=3, capture_output=True, text=True)
            if 'PORT_IN_USE' in final_check.stdout:
                self._print(f"{node_prefix}    ⚠ 警告: 端口3000仍在使用中，尝试强制清理...")
                # 使用更激进的方法
                aggressive_cleanup = ' '.join(ssh_cmd) + ' "pkill -9 node; pkill -9 next; sleep 2; lsof -ti:3000 2>/dev/null | xargs kill -9 2>/dev/null || true"'
                subprocess.run(aggressive_cleanup, shell=True, check=False, timeout=10, capture_output=True)
                time.sleep(3)
        except:
            pass
        
        backend_url = f"http://{node['ip']}:8083"
        
        # 启动前端服务（使用 PORT 和 HOSTNAME 环境变量）
        # HOSTNAME=0.0.0.0 确保只绑定 IPv4，避免 IPv6 的 :::3000 问题
        self._print(f"{node_prefix}    启动前端服务...")
        start_frontend_cmd = ' '.join(ssh_cmd) + f' "cd ~/iarnet/web && rm -f ../web.log && PORT=3000 HOSTNAME=0.0.0.0 BACKEND_URL={backend_url} nohup npm start > ../web.log 2>&1 &"'
        
        for attempt in range(3):
            try:
                # 先启动服务（增加超时时间到30秒）
                subprocess.run(start_frontend_cmd, shell=True, check=False, timeout=30, capture_output=True)
                
                # 等待服务启动（增加等待时间到10秒，给 Next.js 更多时间启动）
                self._print(f"{node_prefix}    等待前端服务启动（尝试 {attempt + 1}/3）...")
                time.sleep(10)
                
                # 检查进程是否在运行（增加超时时间到10秒）
                check_process_cmd = ' '.join(ssh_cmd) + ' "pgrep -f \"next start\" > /dev/null && echo running || echo not_running"'
                process_check = subprocess.run(check_process_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
                
                if 'running' in process_check.stdout:
                    # 检查端口是否真的在监听（增加超时时间到10秒）
                    check_port_listen_cmd = ' '.join(ssh_cmd) + ' "timeout 5 bash -c \'</dev/tcp/localhost/3000\' 2>/dev/null && echo PORT_LISTENING || echo PORT_NOT_LISTENING"'
                    port_check = subprocess.run(check_port_listen_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
                    
                    if 'PORT_LISTENING' in port_check.stdout:
                        self._print(f"{node_prefix}  ✓ 前端服务启动成功")
                        self._print(f"{node_prefix}    前端访问地址: http://{node['ip']}:3000")
                        return True
                    else:
                        # 检查日志看是否有错误（增加超时时间到10秒）
                        check_log_cmd = ' '.join(ssh_cmd) + ' "tail -20 ~/iarnet/web.log 2>/dev/null | tail -5"'
                        log_check = subprocess.run(check_log_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
                        if 'EADDRINUSE' in log_check.stdout:
                            self._print(f"{node_prefix}    ⚠ 端口仍被占用（尝试 {attempt + 1}/3），清理后重试...")
                            # 再次清理
                            cleanup_cmd = ' '.join(ssh_cmd) + ' "pkill -9 -f next; pkill -9 node; sleep 2; lsof -ti:3000 2>/dev/null | xargs kill -9 2>/dev/null || true; sleep 2"'
                            subprocess.run(cleanup_cmd, shell=True, check=False, timeout=15, capture_output=True)
                            if attempt < 2:
                                continue
                
                if attempt < 2:
                    self._print(f"{node_prefix}    ⚠ 前端服务未正常启动（尝试 {attempt + 1}/3），等待后重试...")
                    time.sleep(8)  # 增加重试等待时间
                    
            except subprocess.TimeoutExpired:
                if attempt < 2:
                    self._print(f"{node_prefix}    ⚠ 启动检查超时（尝试 {attempt + 1}/3），重试...")
                    time.sleep(5)
        
        # 如果所有尝试都失败，显示错误信息和诊断命令
        self._print(f"{node_prefix}  ⚠ 前端服务启动失败")
        self._print(f"{node_prefix}  请检查日志: ssh {node['ip']} 'tail -50 ~/iarnet/web.log'")
        self._print(f"{node_prefix}  诊断命令:")
        self._print(f"{node_prefix}    # 检查进程: ssh {node['ip']} 'ps aux | grep next'")
        self._print(f"{node_prefix}    # 检查端口: ssh {node['ip']} 'lsof -i:3000 || ss -tlnp | grep :3000'")
        self._print(f"{node_prefix}    # 手动启动: ssh {node['ip']} 'cd ~/iarnet/web && PORT=3000 HOSTNAME=0.0.0.0 BACKEND_URL={backend_url} npm start'")
        return False
    
    def check_build_dependencies(self) -> bool:
        """检查构建所需的系统依赖"""
        missing_deps = []
        
        # 先检查 pkg-config（其他检查依赖它）
        pkg_config_available = False
        try:
            subprocess.run(['pkg-config', '--version'], 
                         capture_output=True, check=True)
            pkg_config_available = True
        except (FileNotFoundError, subprocess.CalledProcessError):
            missing_deps.append('pkg-config')
        
        # 如果 pkg-config 可用，检查各个库
        if pkg_config_available:
            # 检查 libzmq
            try:
                subprocess.run(['pkg-config', '--exists', 'libzmq'],
                              capture_output=True, check=True)
            except subprocess.CalledProcessError:
                missing_deps.append('libzmq3-dev')
            
            # 检查 libczmq
            try:
                subprocess.run(['pkg-config', '--exists', 'libczmq'],
                              capture_output=True, check=True)
            except subprocess.CalledProcessError:
                missing_deps.append('libczmq-dev')
            
            # 检查 libsodium（ZeroMQ 的加密依赖）
            try:
                subprocess.run(['pkg-config', '--exists', 'libsodium'],
                              capture_output=True, check=True)
            except subprocess.CalledProcessError:
                missing_deps.append('libsodium-dev')
        
        # 检查 gcc（CGO 需要）
        try:
            subprocess.run(['gcc', '--version'], 
                         capture_output=True, check=True)
        except (FileNotFoundError, subprocess.CalledProcessError):
            missing_deps.append('build-essential')
        
        if missing_deps:
            print("  ✗ 缺少构建依赖:")
            for dep in missing_deps:
                print(f"    - {dep}")
            print("\n  请安装缺失的依赖:")
            print(f"    sudo apt-get update")
            print(f"    sudo apt-get install -y {' '.join(missing_deps)}")
            return False
        
        return True
    
    def build_binary(self, force_rebuild: bool = False) -> Path:
        """在本地构建 iarnet 二进制文件"""
        binary_path = PROJECT_ROOT / 'iarnet'
        
        # 检查是否已有二进制文件
        if binary_path.exists() and not force_rebuild:
            print(f"  ℹ 使用现有二进制文件: {binary_path}")
            return binary_path
        
        # 检查构建依赖
        print("  检查构建依赖...")
        if not self.check_build_dependencies():
            raise RuntimeError("缺少必要的构建依赖，请先安装依赖后再试")
        print("  ✓ 构建依赖检查通过")
        
        # 构建二进制文件
        print("  正在构建 iarnet 二进制文件...")
        print("  使用 Go 国内代理: https://goproxy.cn")
        
        # 设置 Go 代理环境变量（使用国内代理加速下载）
        env = os.environ.copy()
        env['GOPROXY'] = 'https://goproxy.cn,direct'
        env['GOSUMDB'] = 'sum.golang.org'
        # 启用 CGO（goczmq 需要）
        env['CGO_ENABLED'] = '1'
        
        build_cmd = ['go', 'build', '-o', str(binary_path), 'cmd/main.go']
        try:
            result = subprocess.run(
                build_cmd,
                cwd=str(PROJECT_ROOT),
                env=env,
                check=True,
                capture_output=True,
                text=True
            )
            print("  ✓ 本地构建成功")
            return binary_path
        except subprocess.CalledProcessError as e:
            print(f"  ✗ 本地构建失败: {e}")
            if e.stderr:
                print(f"  错误信息: {e.stderr}")
                # 检查是否是依赖问题
                if 'libzmq' in e.stderr or 'libczmq' in e.stderr or 'libsodium' in e.stderr:
                    print("\n  提示: 如果错误与 ZeroMQ 相关，请确保已安装:")
                    print("    sudo apt-get install -y libzmq3-dev libczmq-dev libsodium-dev pkg-config build-essential")
            raise
        except FileNotFoundError:
            print("  ✗ 错误: 未找到 go 命令，请先安装 Go")
            raise
    
    def build_frontend(self, force_rebuild: bool = False) -> Path:
        """在本地构建前端项目（生产版本）"""
        web_dir = PROJECT_ROOT / 'web'
        build_dir = web_dir / '.next'
        
        # 检查是否已构建
        if build_dir.exists() and not force_rebuild:
            print(f"  ℹ 使用现有前端构建产物: {build_dir}")
            # 验证构建产物是否完整
            if not (build_dir / 'standalone').exists() and not (build_dir / 'static').exists():
                print("  ⚠ 构建产物可能不完整，强制重新构建...")
                force_rebuild = True
        
        # 检查 Node.js 和 npm
        try:
            subprocess.run(['node', '--version'], capture_output=True, check=True)
            subprocess.run(['npm', '--version'], capture_output=True, check=True)
        except (FileNotFoundError, subprocess.CalledProcessError):
            raise RuntimeError("未找到 Node.js 或 npm，请先安装 Node.js")
        
        # 如果需要重新构建
        if force_rebuild or not build_dir.exists():
            # 构建前端
            print("  正在构建前端项目（生产版本）...")
            
            # 安装依赖
            print("    安装 npm 依赖...")
            install_cmd = ['npm', 'install', '--legacy-peer-deps']
            try:
                result = subprocess.run(
                    install_cmd,
                    cwd=str(web_dir),
                    check=True,
                    capture_output=True,
                    text=True
                )
                print("    ✓ 依赖安装成功")
            except subprocess.CalledProcessError as e:
                print(f"    ✗ 依赖安装失败: {e}")
                if e.stderr:
                    print(f"    错误信息: {e.stderr}")
                raise
            
            # 构建生产版本
            print("    构建生产版本...")
            build_cmd = ['npm', 'run', 'build']
            try:
                result = subprocess.run(
                    build_cmd,
                    cwd=str(web_dir),
                    check=True,
                    capture_output=True,
                    text=True
                )
                print("    ✓ 前端构建成功")
            except subprocess.CalledProcessError as e:
                print(f"    ✗ 前端构建失败: {e}")
                if e.stderr:
                    print(f"    错误信息: {e.stderr}")
                raise
        
        # 验证构建产物
        if not build_dir.exists():
            raise RuntimeError("前端构建失败：未找到构建产物 .next 目录")
        
        return web_dir
    
    def _print(self, *args, **kwargs):
        """线程安全的打印函数"""
        with self._log_lock:
            print(*args, **kwargs)
    
    def deploy_to_node(self, node_id: int, build: bool = False, restart: bool = False, binary_path: Path = None, deploy_frontend: bool = False, frontend_dir: Path = None) -> bool:
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
        
        # 0. 如果指定 restart，先停止所有服务
        if restart:
            self._print(f"[节点 {node_id}] 0. 停止现有服务...")
            stop_commands = [
                'pkill -9 -f iarnet',
                'pkill -9 -f "next start"',
                'pkill -9 -f "node.*next"',
                'killall -9 node 2>/dev/null',
                # 释放端口
                'lsof -ti:3000 | xargs kill -9 2>/dev/null',
                'lsof -ti:8083 | xargs kill -9 2>/dev/null',
                'fuser -k 3000/tcp 2>/dev/null',
                'fuser -k 8083/tcp 2>/dev/null'
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
        # 先确保主目录存在，再创建子目录
        mkdir_cmd = ' '.join(ssh_cmd) + ' "mkdir -p ~/iarnet && mkdir -p ~/iarnet/data ~/iarnet/workspaces ~/iarnet/web && ls -ld ~/iarnet"'
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
            f"{self.user}@{node['ip']}:~/iarnet/config.yaml"
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
                binary_path = PROJECT_ROOT / 'iarnet'
            
            if not binary_path.exists():
                self._print(f"[节点 {node_id}]   ✗ 错误: 二进制文件不存在: {binary_path}")
                return False
            
            self._print(f"[节点 {node_id}] 3. 上传二进制文件...")
            # 直接使用用户名构建远程主目录路径（更可靠）
            # SSH 警告信息可能混入输出，直接构建路径更安全
            remote_home = f"/home/{self.user}"
            remote_path = f"{remote_home}/iarnet/iarnet"
            self._print(f"[节点 {node_id}]     用户名: {self.user}")
            self._print(f"[节点 {node_id}]     远程路径: {remote_path}")
            
            # 先验证目标目录是否存在且有写权限（使用绝对路径）
            verify_dir_cmd = ' '.join(ssh_cmd) + f' "test -d {remote_home}/iarnet && test -w {remote_home}/iarnet && echo OK || echo FAIL"'
            verify_result = subprocess.run(verify_dir_cmd, shell=True, check=False, capture_output=True, text=True, timeout=5)
            # 清理警告信息
            verify_output_lines = [line.strip() for line in verify_result.stdout.split('\n') if line.strip() and 'Warning:' not in line]
            verify_output = verify_output_lines[-1] if verify_output_lines else verify_result.stdout.strip()
            
            if 'OK' not in verify_output:
                self._print(f"[节点 {node_id}]   ✗ 目标目录 {remote_home}/iarnet 不存在或没有写权限")
                # 尝试重新创建目录
                fix_dir_cmd = ' '.join(ssh_cmd) + f' "mkdir -p {remote_home}/iarnet && chmod 755 {remote_home}/iarnet"'
                fix_result = subprocess.run(fix_dir_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
                if fix_result.returncode != 0:
                    self._print(f"[节点 {node_id}]   ✗ 无法创建目录")
                    return False
            
            # 检查并删除已存在的目标文件（无论是文件还是目录，使用绝对路径）
            self._print(f"[节点 {node_id}]     检查目标文件是否存在...")
            check_target_cmd = ' '.join(ssh_cmd) + f' "if [ -e {remote_path} ]; then if [ -d {remote_path} ]; then echo IS_DIR; else echo IS_FILE; fi; else echo NOT_EXISTS; fi"'
            check_result = subprocess.run(check_target_cmd, shell=True, check=False, capture_output=True, text=True, timeout=5)
            target_status_lines = [line.strip() for line in check_result.stdout.split('\n') if line.strip() and 'Warning:' not in line]
            target_status = target_status_lines[-1] if target_status_lines else check_result.stdout.strip()
            
            if 'IS_DIR' in target_status:
                self._print(f"[节点 {node_id}]     ℹ 目标路径已存在且是目录，正在删除...")
                rm_cmd = ' '.join(ssh_cmd) + f' "rm -rf {remote_path}"'
                rm_result = subprocess.run(rm_cmd, shell=True, check=True, timeout=10, capture_output=True, text=True)
                self._print(f"[节点 {node_id}]     ✓ 目录已删除")
            elif 'IS_FILE' in target_status:
                self._print(f"[节点 {node_id}]     ℹ 目标文件已存在，正在删除...")
                rm_cmd = ' '.join(ssh_cmd) + f' "rm -f {remote_path}"'
                rm_result = subprocess.run(rm_cmd, shell=True, check=True, timeout=10, capture_output=True, text=True)
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
                self._print(f"[节点 {node_id}]   请检查网络连接或手动上传: scp {binary_path} {self.user}@{node['ip']}:{remote_path}")
                return False
            except subprocess.CalledProcessError as e:
                error_msg = e.stderr if e.stderr else (e.stdout if e.stdout else str(e))
                self._print(f"[节点 {node_id}]   ✗ 二进制文件上传失败")
                self._print(f"[节点 {node_id}]   错误信息: {error_msg}")
                # 诊断问题
                self._print(f"[节点 {node_id}]   诊断信息:")
                # 检查目录权限和文件状态
                check_cmd = ' '.join(ssh_cmd) + f' "ls -ld {remote_home}/iarnet && ls -la {remote_home}/iarnet/ | head -5 && df -h {remote_home}/iarnet"'
                diag_result = subprocess.run(check_cmd, shell=True, check=False, capture_output=True, text=True, timeout=5)
                if diag_result.stdout:
                    self._print(f"[节点 {node_id}]     目录信息: {diag_result.stdout[:400]}")
                if diag_result.stderr:
                    self._print(f"[节点 {node_id}]     错误: {diag_result.stderr[:200]}")
                return False
            
            # 设置执行权限
            try:
                chmod_cmd = ' '.join(ssh_cmd) + ' "chmod +x ~/iarnet/iarnet"'
                subprocess.run(chmod_cmd, shell=True, check=True, timeout=10, capture_output=True)
                self._print(f"[节点 {node_id}]   ✓ 二进制文件上传成功")
            except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as e:
                self._print(f"[节点 {node_id}]   ⚠ 设置执行权限失败，但继续执行: {e}")
            
            # 部署完后端后立即启动后端服务
            self._print(f"[节点 {node_id}]     启动后端服务...")
            backend_running = self._start_backend_service(ssh_cmd, node, node_id)
            if backend_running:
                self._print(f"[节点 {node_id}]   ✓ 后端服务启动成功")
            else:
                self._print(f"[节点 {node_id}]   ⚠ 后端服务启动失败，请检查日志: ~/iarnet/iarnet.log")
        
        # 4. 部署前端（如果指定）
        if deploy_frontend:
            if frontend_dir is None:
                frontend_dir = PROJECT_ROOT / 'web'
            
            if not frontend_dir.exists():
                self._print(f"[节点 {node_id}]   ✗ 错误: 前端目录不存在: {frontend_dir}")
                return False
            
            # 检查本地是否已构建（必须）
            build_dir = frontend_dir / '.next'
            if not build_dir.exists():
                self._print(f"[节点 {node_id}]   ✗ 错误: 前端未在本地构建，请先构建前端")
                self._print(f"[节点 {node_id}]   运行: cd {frontend_dir} && npm install --legacy-peer-deps && npm run build")
                return False
            
            step_num = "4" if build else "3"
            self._print(f"[节点 {node_id}] {step_num}. 部署前端项目（使用本地构建产物）...")
            
            # 检查是否可以使用 rsync（更高效）
            use_rsync = False
            try:
                subprocess.run(['rsync', '--version'], capture_output=True, check=True)
                use_rsync = True
            except (FileNotFoundError, subprocess.CalledProcessError):
                self._print(f"[节点 {node_id}]     提示: 未找到 rsync，将使用 tar+scp 上传文件")
            
            if use_rsync:
                # 使用 rsync 上传前端文件（只上传必要文件）
                # 包括：.next（构建产物）、public（静态资源）、package.json、next.config.js 等
                rsync_cmd = [
                    'rsync',
                    '-avz',
                    '--delete',
                    '--include', '.next/',
                    '--include', '.next/**',
                    '--include', 'public/',
                    '--include', 'public/**',
                    '--include', 'package.json',
                    '--include', 'package-lock.json',
                    '--include', 'next.config.js',
                    '--include', 'next.config.mjs',
                    '--exclude', '*',
                    '--exclude', 'node_modules',
                    '--exclude', '.git',
                    '--exclude', '.next/cache',
                    f"{str(frontend_dir)}/",
                    f"{self.user}@{node['ip']}:~/iarnet/web/"
                ]
                try:
                    subprocess.run(rsync_cmd, check=True, capture_output=True, text=True)
                    self._print(f"[节点 {node_id}]   ✓ 前端文件上传成功（使用 rsync）")
                except subprocess.CalledProcessError as e:
                    self._print(f"[节点 {node_id}]   ⚠ rsync 上传失败，尝试使用 tar+scp 方式...")
                    use_rsync = False
            
            if not use_rsync:
                # 使用 tar+scp 方式上传（只打包必要文件）
                self._print(f"[节点 {node_id}]     打包前端文件（仅构建产物和必要文件）...")
                
                with tempfile.NamedTemporaryFile(suffix='.tar.gz', delete=False) as tmp_file:
                    tmp_tar = tmp_file.name
                    try:
                        with tarfile.open(tmp_tar, 'w:gz') as tar:
                            # 添加构建产物目录
                            if build_dir.exists():
                                tar.add(build_dir, arcname='web/.next')
                            
                            # 添加 public 目录（静态资源）
                            public_dir = frontend_dir / 'public'
                            if public_dir.exists():
                                tar.add(public_dir, arcname='web/public')
                            
                            # 添加必要的配置文件
                            for config_file in ['package.json', 'package-lock.json', 'next.config.js', 'next.config.mjs']:
                                config_path = frontend_dir / config_file
                                if config_path.exists():
                                    tar.add(config_path, arcname=f'web/{config_file}')
                        
                        self._print(f"[节点 {node_id}]     上传前端文件包...")
                        scp_tar_cmd = [
                            'scp',
                            '-o', 'StrictHostKeyChecking=no',
                            '-o', 'UserKnownHostsFile=/dev/null',
                            tmp_tar,
                            f"{self.user}@{node['ip']}:~/iarnet/web.tar.gz"
                        ]
                        subprocess.run(scp_tar_cmd, check=True, capture_output=True, timeout=120)
                        
                        # 在远程解压
                        extract_cmd = ' '.join(ssh_cmd) + ' "cd ~/iarnet && rm -rf web && mkdir -p web && tar -xzf web.tar.gz && rm web.tar.gz"'
                        subprocess.run(extract_cmd, shell=True, check=True, timeout=60)
                        self._print(f"[节点 {node_id}]   ✓ 前端文件上传成功（使用 tar+scp）")
                    finally:
                        if os.path.exists(tmp_tar):
                            os.unlink(tmp_tar)
            
            # 在远程服务器上安装生产依赖（只需要运行时依赖）
            self._print(f"[节点 {node_id}]     在远程服务器上安装生产依赖...")
            install_cmd = ' '.join(ssh_cmd) + ' "cd ~/iarnet/web && npm install --legacy-peer-deps --production"'
            install_result = subprocess.run(install_cmd, shell=True, check=False, capture_output=True, text=True, timeout=300)
            if install_result.returncode == 0:
                self._print(f"[节点 {node_id}]     ✓ 生产依赖安装成功")
            else:
                self._print(f"[节点 {node_id}]     ⚠ 依赖安装可能有问题，但继续...")
                if install_result.stderr:
                    self._print(f"[节点 {node_id}]     错误信息: {install_result.stderr[:300]}")
            
            self._print(f"[节点 {node_id}]     ℹ 使用本地构建产物，无需远程构建")
            
            # 部署完前端后，如果后端已运行，立即启动前端
            self._print(f"[节点 {node_id}]     检查后端服务状态...")
            verify_backend_cmd = ' '.join(ssh_cmd) + ' "pgrep -f iarnet > /dev/null && echo running || echo not_running"'
            backend_running = False
            try:
                verify_result = subprocess.run(verify_backend_cmd, shell=True, check=True, timeout=5, capture_output=True, text=True)
                backend_running = 'running' in verify_result.stdout
            except:
                pass
            
            if backend_running:
                self._print(f"[节点 {node_id}]     后端服务正在运行，启动前端服务...")
                self._start_frontend_service(ssh_cmd, node, node_id)
            else:
                self._print(f"[节点 {node_id}]     ⚠ 后端服务未运行，前端将在后端启动后自动启动")
        
        # 如果指定了 restart 但没有部署后端和前端，需要启动服务
        if restart and not build and not deploy_frontend:
            self._print(f"[节点 {node_id}] 4. 启动服务...")
            # 启动后端服务
            self._print(f"[节点 {node_id}]     启动后端服务...")
            backend_running = self._start_backend_service(ssh_cmd, node, node_id)
            if backend_running:
                self._print(f"[节点 {node_id}]   ✓ 后端服务启动成功")
                
                # 检查是否有前端，如果有则启动前端
                check_frontend_cmd = ' '.join(ssh_cmd) + ' "test -d ~/iarnet/web && test -f ~/iarnet/web/package.json && echo EXISTS || echo NOT_EXISTS"'
                try:
                    check_result = subprocess.run(check_frontend_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
                    if 'EXISTS' in check_result.stdout:
                        self._print(f"[节点 {node_id}]     启动前端服务...")
                        self._start_frontend_service(ssh_cmd, node, node_id)
                except:
                    pass
            else:
                self._print(f"[节点 {node_id}]   ⚠ 后端服务启动失败，请检查日志: ~/iarnet/iarnet.log")
        
        self._print(f"[节点 {node_id}] " + "=" * 60)
        self._print(f"[节点 {node_id}] ✓ 节点 {node_id} 部署完成")
        return True
    
    def install_deps_with_ansible(self, node_ids: list) -> bool:
        """使用 Ansible 安装依赖"""
        print("\n使用 Ansible 安装依赖...")
        print("=" * 60)
        
        # 检查 ansible 是否安装
        try:
            subprocess.run(['ansible-playbook', '--version'], 
                         capture_output=True, check=True)
        except (FileNotFoundError, subprocess.CalledProcessError):
            print("错误: 未找到 ansible-playbook，请先安装 Ansible")
            print("安装方法: sudo apt-get install ansible 或 pip3 install ansible")
            return False
        
        # 生成临时 inventory 文件
        inventory_file = SCRIPT_DIR / 'ansible' / 'inventory-temp.ini'
        with open(inventory_file, 'w') as f:
            f.write("[iarnet_nodes]\n")
            for node_id in node_ids:
                if node_id in self.node_info:
                    node = self.node_info[node_id]
                    f.write(f"{node['hostname']} ansible_host={node['ip']} ansible_user={self.user}\n")
        
        # 运行 ansible playbook
        playbook_path = SCRIPT_DIR / 'ansible' / 'playbooks' / 'install-iarnet-deps.yml'
        if not playbook_path.exists():
            print(f"错误: Ansible playbook 不存在: {playbook_path}")
            return False
        
        ansible_cmd = [
            'ansible-playbook',
            '-i', str(inventory_file),
            str(playbook_path),
            '--become',
            '--become-method=sudo',
            '--ask-become-pass'
        ]
        
        try:
            result = subprocess.run(ansible_cmd, check=True)
            print("  ✓ 依赖安装成功")
            return True
        except subprocess.CalledProcessError as e:
            print(f"  ✗ 依赖安装失败: {e}")
            return False
        finally:
            # 清理临时文件
            if inventory_file.exists():
                inventory_file.unlink()
    
    def deploy_to_nodes(self, node_ids: list, build: bool = False, restart: bool = False, install_deps: bool = False, deploy_frontend: bool = False, max_workers: int = None):
        """批量部署到多个节点（并行执行）"""
        print(f"批量部署到节点: {node_ids}")
        print("=" * 60)
        
        # 先安装依赖（如果指定）
        if install_deps:
            if not self.install_deps_with_ansible(node_ids):
                print("警告: 依赖安装失败，但继续部署流程")
        
        # 如果需要构建，先构建一次，所有节点复用同一个二进制文件
        binary_path = None
        if build:
            print("\n在本地构建 iarnet 二进制文件（所有节点将复用此文件）...")
            try:
                binary_path = self.build_binary(force_rebuild=False)
            except Exception as e:
                print(f"错误: 构建失败，无法继续部署")
                return
        
        # 如果需要部署前端，必须在本地构建一次，所有节点复用同一个构建产物
        frontend_dir = None
        if deploy_frontend:
            print("\n在本地构建前端项目（所有节点将复用此构建产物）...")
            try:
                # 强制构建，确保有最新的构建产物
                frontend_dir = self.build_frontend(force_rebuild=False)
                # 验证构建产物存在
                build_dir = frontend_dir / '.next'
                if not build_dir.exists():
                    raise RuntimeError("前端构建失败：未找到构建产物")
                print(f"  ✓ 前端构建完成，构建产物: {build_dir}")
            except Exception as e:
                print(f"错误: 前端构建失败，无法继续部署: {e}")
                return
        
        # 并行部署到各个节点
        print(f"\n开始并行部署到 {len(node_ids)} 个节点...")
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
                    binary_path=binary_path,
                    deploy_frontend=deploy_frontend,
                    frontend_dir=frontend_dir
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
                        with self._log_lock:
                            print(f"[节点 {node_id}] ✓ 部署成功")
                    else:
                        failed_nodes.append(node_id)
                        with self._log_lock:
                            print(f"[节点 {node_id}] ✗ 部署失败")
                except Exception as e:
                    failed_nodes.append(node_id)
                    with self._log_lock:
                        print(f"[节点 {node_id}] ✗ 部署异常: {e}")
        
        # 输出部署结果摘要
        print("\n" + "=" * 60)
        print(f"部署完成: {success_count}/{len(node_ids)} 个节点成功")
        if failed_nodes:
            print(f"失败的节点: {failed_nodes}")
        if deploy_frontend:
            print("\n前端访问地址:")
            for node_id in node_ids:
                if node_id in self.node_info:
                    node = self.node_info[node_id]
                    print(f"  节点 {node_id} ({node['hostname']}): http://{node['ip']}:3000")
        print("=" * 60)

def main():
    parser = argparse.ArgumentParser(description='在 iarnet 节点上部署 iarnet 服务')
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
        help='部署到多个节点，格式: start-end 或逗号分隔的列表 (例如: 0-10 或 0,1,2)'
    )
    parser.add_argument(
        '--build', '-b',
        action='store_true',
        help='在本地构建 iarnet 二进制文件并上传到节点'
    )
    parser.add_argument(
        '--install-deps', '-i',
        action='store_true',
        help='使用 Ansible 安装节点依赖（Go、gRPC、ZeroMQ等）'
    )
    parser.add_argument(
        '--restart', '-r',
        action='store_true',
        help='重启 iarnet 服务'
    )
    parser.add_argument(
        '--frontend', '-f',
        action='store_true',
        help='部署前端项目（Next.js）'
    )
    parser.add_argument(
        '--max-workers',
        type=int,
        default=None,
        help='并行部署的最大线程数（默认: min(节点数, 10)）'
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
            build=args.build,
            restart=args.restart,
            install_deps=args.install_deps,
            deploy_frontend=args.frontend,
            max_workers=args.max_workers
        )
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()

