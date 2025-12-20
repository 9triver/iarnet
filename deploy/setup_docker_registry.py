#!/usr/bin/env python3
"""
在指定节点上部署 Docker Registry
用于共享 Docker 镜像，避免在每个节点重复上传
"""

import os
import sys
import yaml
import argparse
import subprocess
import time
from pathlib import Path

# 获取脚本所在目录
SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent

class DockerRegistryManager:
    def __init__(self, vm_config_path: str, registry_node_id: int = None):
        """初始化 Registry 管理器"""
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
        
        # 确定 Registry 节点（默认使用第一个 iarnet 节点）
        if registry_node_id is None:
            iarnet_config = self.vm_config['vm_types']['iarnet']
            registry_node_id = 0
        
        # 计算 Registry 节点信息
        iarnet_config = self.vm_config['vm_types']['iarnet']
        ip_suffix = iarnet_config['ip_start'] + registry_node_id
        self.registry_ip = f"{iarnet_config['ip_base']}.{ip_suffix}"
        self.registry_hostname = f"{iarnet_config['hostname_prefix']}-{registry_node_id+1:02d}"
        self.registry_port = 5000  # Docker Registry 默认端口
        self.registry_url = f"http://{self.registry_ip}:{self.registry_port}"
    
    def check_docker_installed(self, ssh_cmd: list) -> bool:
        """检查 Docker 是否已安装"""
        check_cmd = ' '.join(ssh_cmd) + ' "docker --version >/dev/null 2>&1 && echo OK || echo NOT_INSTALLED"'
        try:
            result = subprocess.run(check_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            return 'OK' in result.stdout
        except:
            return False
    
    def install_docker(self, ssh_cmd: list) -> bool:
        """安装 Docker"""
        print("  安装 Docker...")
        install_cmd = ' '.join(ssh_cmd) + ' "curl -fsSL https://get.docker.com | sudo sh"'
        try:
            result = subprocess.run(install_cmd, shell=True, check=True, timeout=300, capture_output=True, text=True)
            print("  ✓ Docker 安装成功")
            # 将用户添加到 docker 组
            add_user_cmd = ' '.join(ssh_cmd) + f' "sudo usermod -aG docker {self.user}"'
            subprocess.run(add_user_cmd, shell=True, check=False, timeout=10, capture_output=True)
            return True
        except subprocess.CalledProcessError as e:
            print(f"  ✗ Docker 安装失败: {e}")
            return False
    
    def check_registry_running(self, ssh_cmd: list) -> bool:
        """检查 Registry 是否正在运行"""
        check_cmd = ' '.join(ssh_cmd) + ' "docker ps | grep registry:2 >/dev/null && echo RUNNING || echo NOT_RUNNING"'
        try:
            result = subprocess.run(check_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            return 'RUNNING' in result.stdout
        except:
            return False
    
    def deploy_registry(self) -> bool:
        """部署 Docker Registry"""
        print(f"\n部署 Docker Registry 到节点: {self.registry_hostname} ({self.registry_ip})")
        print("=" * 60)
        
        # 检查连通性
        try:
            result = subprocess.run(
                ['ping', '-c', '1', '-W', '2', self.registry_ip],
                capture_output=True,
                timeout=5
            )
            if result.returncode != 0:
                print(f"✗ 无法连接到节点 {self.registry_ip}")
                return False
        except:
            print(f"✗ 无法连接到节点 {self.registry_ip}")
            return False
        
        # 构建 SSH 命令
        ssh_cmd = [
            'ssh',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f"{self.user}@{self.registry_ip}"
        ]
        
        # 1. 检查 Docker 是否安装
        print("\n1. 检查 Docker...")
        if not self.check_docker_installed(ssh_cmd):
            print("  Docker 未安装，开始安装...")
            if not self.install_docker(ssh_cmd):
                return False
        else:
            print("  ✓ Docker 已安装")
        
        # 2. 检查 Registry 是否已运行
        print("\n2. 检查 Registry 状态...")
        if self.check_registry_running(ssh_cmd):
            print(f"  ✓ Registry 已在运行: {self.registry_url}")
            return True
        
        # 3. 创建 Registry 数据目录
        print("\n3. 创建 Registry 数据目录...")
        mkdir_cmd = ' '.join(ssh_cmd) + ' "mkdir -p ~/docker-registry/data"'
        try:
            subprocess.run(mkdir_cmd, shell=True, check=True, timeout=10, capture_output=True)
            print("  ✓ 目录创建成功")
        except subprocess.CalledProcessError as e:
            print(f"  ✗ 目录创建失败: {e}")
            return False
        
        # 4. 启动 Registry 容器
        print("\n4. 启动 Registry 容器...")
        # 使用 host 网络模式，方便其他节点访问
        run_registry_cmd = ' '.join(ssh_cmd) + f' "docker run -d --name docker-registry --restart=unless-stopped --network host -v ~/docker-registry/data:/var/lib/registry registry:2"'
        try:
            result = subprocess.run(run_registry_cmd, shell=True, check=True, timeout=30, capture_output=True, text=True)
            print("  ✓ Registry 容器启动成功")
            
            # 等待 Registry 启动
            print("  等待 Registry 就绪...")
            for i in range(10):
                time.sleep(2)
                check_cmd = ' '.join(ssh_cmd) + f' "curl -s http://localhost:{self.registry_port}/v2/ >/dev/null && echo READY || echo NOT_READY"'
                check_result = subprocess.run(check_cmd, shell=True, check=False, timeout=5, capture_output=True, text=True)
                if 'READY' in check_result.stdout:
                    print(f"  ✓ Registry 已就绪: {self.registry_url}")
                    break
                if i == 9:
                    print("  ⚠ Registry 启动可能有问题，但继续...")
        except subprocess.CalledProcessError as e:
            print(f"  ✗ Registry 启动失败: {e}")
            if e.stderr:
                print(f"  错误信息: {e.stderr[:300]}")
            return False
        
        print("\n" + "=" * 60)
        print(f"✓ Docker Registry 部署完成")
        print(f"Registry URL: {self.registry_url}")
        print(f"\n其他节点可以通过以下方式访问:")
        print(f"  {self.registry_url}")
        print(f"\n配置 Docker daemon（如果需要）:")
        print(f"  在 /etc/docker/daemon.json 中添加:")
        print(f'  {{"insecure-registries": ["{self.registry_ip}:{self.registry_port}"]}}')
        print("=" * 60)
        
        return True
    
    def configure_nodes_for_registry(self, node_ids: list = None, node_type: str = 'docker') -> bool:
        """配置其他节点以使用 Registry（配置 insecure-registries）
        
        Args:
            node_ids: 节点ID列表，如果为None则配置所有指定类型的节点
            node_type: 节点类型，'docker' 或 'iarnet'
        """
        print(f"\nRegistry 信息:")
        print(f"  运行节点: {self.registry_hostname} ({self.registry_ip})")
        print(f"  Registry URL: {self.registry_url}")
        
        # 获取所有需要配置的节点
        if node_ids is None:
            if node_type == 'docker':
                # 配置所有 Docker provider 节点
                node_config = self.vm_config['vm_types']['docker']
                node_ids = list(range(node_config['count']))
                node_type_name = "Docker provider"
            elif node_type == 'iarnet':
                # 配置所有 iarnet 节点
                node_config = self.vm_config['vm_types']['iarnet']
                node_ids = list(range(node_config['count']))
                node_type_name = "iarnet"
            else:
                print(f"错误: 不支持的节点类型: {node_type}")
                return False
        else:
            # 根据 node_type 获取配置
            if node_type == 'docker':
                node_config = self.vm_config['vm_types']['docker']
                node_type_name = "Docker provider"
            elif node_type == 'iarnet':
                node_config = self.vm_config['vm_types']['iarnet']
                node_type_name = "iarnet"
            else:
                print(f"错误: 不支持的节点类型: {node_type}")
                return False
        
        print(f"\n配置 {node_type_name} 节点以使用 Registry...")
        print("=" * 60)
        
        success_count = 0
        for node_id in node_ids:
            # 计算节点 IP
            ip_suffix = node_config['ip_start'] + node_id
            node_ip = f"{node_config['ip_base']}.{ip_suffix}"
            hostname = f"{node_config['hostname_prefix']}-{node_id+1:02d}"
            
            ssh_cmd = [
                'ssh',
                '-o', 'StrictHostKeyChecking=no',
                '-o', 'UserKnownHostsFile=/dev/null',
                '-o', 'ConnectTimeout=5',
                f"{self.user}@{node_ip}"
            ]
            
            # 检查 Docker 是否安装
            if not self.check_docker_installed(ssh_cmd):
                print(f"[节点 {node_id}] {hostname} ({node_ip}) Docker 未安装，跳过")
                continue
            
            # 配置 insecure-registries
            print(f"[节点 {node_id}] {hostname} ({node_ip}) 配置 insecure-registries...")
            
            # 使用 Python 脚本在远程执行，避免引号转义问题
            import base64
            registry_url = f"{self.registry_ip}:{self.registry_port}"
            python_script = f'''
import json
import sys

registry_url = "{registry_url}"

# 读取现有配置
try:
    with open("/etc/docker/daemon.json", "r") as f:
        config = json.load(f)
except (FileNotFoundError, json.JSONDecodeError):
    config = {{}}

# 添加 insecure-registries
if "insecure-registries" not in config:
    config["insecure-registries"] = []

if registry_url not in config["insecure-registries"]:
    config["insecure-registries"].append(registry_url)

# 写入配置
with open("/etc/docker/daemon.json", "w") as f:
    json.dump(config, f, indent=2)

print("OK")
'''
            
            # 使用 base64 编码传递 Python 脚本
            script_encoded = base64.b64encode(python_script.encode('utf-8')).decode('utf-8')
            
            # 在远程执行 Python 脚本
            config_cmd = ' '.join(ssh_cmd) + f' "echo {script_encoded} | base64 -d | sudo python3 && sudo systemctl restart docker"'
            try:
                result = subprocess.run(config_cmd, shell=True, check=True, timeout=30, capture_output=True, text=True)
                time.sleep(2)  # 等待 Docker 重启
                print(f"[节点 {node_id}] {hostname} ({node_ip}) ✓ 配置成功")
                success_count += 1
            except subprocess.CalledProcessError as e:
                print(f"[节点 {node_id}] {hostname} ({node_ip}) ✗ 配置失败: {e}")
                if e.stderr:
                    print(f"[节点 {node_id}] {hostname} ({node_ip})   错误信息: {e.stderr[:200]}")
        
        print(f"\n配置完成: {success_count}/{len(node_ids)} 个节点成功")
        return success_count > 0

def main():
    parser = argparse.ArgumentParser(description='部署 Docker Registry 用于共享镜像')
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--registry-node', '-n',
        type=int,
        default=0,
        help='Registry 部署在哪个 iarnet 节点上 (默认: 0)'
    )
    parser.add_argument(
        '--configure-nodes', '-c',
        type=str,
        choices=['docker', 'iarnet', 'all'],
        help='配置节点以使用 Registry: docker (Docker provider节点), iarnet (iarnet节点), all (所有节点)'
    )
    
    args = parser.parse_args()
    
    try:
        manager = DockerRegistryManager(args.vm_config, args.registry_node)
        
        if manager.deploy_registry():
            if args.configure_nodes:
                if args.configure_nodes == 'all':
                    # 先配置 iarnet 节点
                    print("\n" + "=" * 60)
                    manager.configure_nodes_for_registry(node_type='iarnet')
                    # 再配置 docker provider 节点
                    print("\n" + "=" * 60)
                    manager.configure_nodes_for_registry(node_type='docker')
                else:
                    manager.configure_nodes_for_registry(node_type=args.configure_nodes)
            
            print(f"\nRegistry 信息:")
            print(f"  URL: {manager.registry_url}")
            print(f"  节点: {manager.registry_hostname} ({manager.registry_ip})")
            print(f"\n使用示例:")
            print(f"  # 标记镜像")
            print(f"  docker tag <image> {manager.registry_ip}:{manager.registry_port}/<image>")
            print(f"  # 推送镜像")
            print(f"  docker push {manager.registry_ip}:{manager.registry_port}/<image>")
            print(f"  # 拉取镜像")
            print(f"  docker pull {manager.registry_ip}:{manager.registry_port}/<image>")
        else:
            print("Registry 部署失败")
            sys.exit(1)
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()

