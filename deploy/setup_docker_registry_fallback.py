#!/usr/bin/env python3
"""
配置 Docker 节点，使其在本地找不到镜像时自动从 Registry 拉取
通过创建 docker pull 包装脚本实现
"""

import os
import sys
import yaml
import argparse
import subprocess
import time
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

# 获取脚本所在目录
SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent

class RegistryFallbackConfigurator:
    def __init__(self, vm_config_path: str, registry_node_id: int = 0):
        """初始化配置器"""
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
        
        # 计算 Registry 地址
        iarnet_config = self.vm_config['vm_types']['iarnet']
        ip_suffix = iarnet_config['ip_start'] + registry_node_id
        self.registry_ip = f"{iarnet_config['ip_base']}.{ip_suffix}"
        self.registry_port = 5000
        self.registry_url = f"{self.registry_ip}:{self.registry_port}"
    
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
    
    def install_docker_pull_wrapper(self, ssh_cmd: list, node_id: int = None, node_info: dict = None) -> bool:
        """安装 docker pull 包装脚本，实现自动从 Registry 拉取"""
        if node_info:
            node_prefix = f"[节点 {node_id}] {node_info.get('hostname', '')} ({node_info.get('ip', '')}) "
        elif node_id is not None:
            node_prefix = f"[节点 {node_id}] "
        else:
            node_prefix = ""
        
        print(f"{node_prefix}安装 Docker pull 包装脚本...")
        
        # 创建包装脚本 - 更智能的版本
        wrapper_script = f'''#!/bin/bash
# Docker pull 包装脚本 - 自动从 Registry 拉取镜像
# 如果本地找不到镜像，自动尝试从 Registry 拉取

REGISTRY_URL="{self.registry_url}"

# 获取所有参数（镜像名）
IMAGE_NAME="$@"

# 如果镜像名包含 registry URL，直接使用
if [[ "$IMAGE_NAME" == *"$REGISTRY_URL"* ]]; then
    docker pull "$IMAGE_NAME"
    exit $?
fi

# 如果镜像名已经是完整路径（包含 /），检查是否是已知的命名空间
if [[ "$IMAGE_NAME" == *"/"* ]]; then
    # 提取命名空间和镜像名
    NAMESPACE="${{IMAGE_NAME%%/*}}"
    IMAGE="${{IMAGE_NAME#*/}}"
    
    # 如果是 components 命名空间，自动添加 Registry 前缀
    if [[ "$NAMESPACE" == "components" ]]; then
        REGISTRY_IMAGE="$REGISTRY_URL/$IMAGE_NAME"
        echo "从 Registry 拉取 component 镜像: $REGISTRY_IMAGE"
        if docker pull "$REGISTRY_IMAGE"; then
            # 创建本地别名
            docker tag "$REGISTRY_IMAGE" "$IMAGE_NAME"
            exit 0
        fi
    fi
    
    # 其他情况，先尝试本地，失败后尝试 Registry
    if docker pull "$IMAGE_NAME" 2>/dev/null; then
        exit 0
    fi
    
    # 尝试从 Registry 拉取
    REGISTRY_IMAGE="$REGISTRY_URL/$IMAGE_NAME"
    echo "本地镜像不存在，尝试从 Registry 拉取: $REGISTRY_IMAGE"
    if docker pull "$REGISTRY_IMAGE"; then
        docker tag "$REGISTRY_IMAGE" "$IMAGE_NAME"
        exit 0
    fi
else
    # 简单镜像名（如 iarnet:latest）
    # 先尝试本地
    if docker pull "$IMAGE_NAME" 2>/dev/null; then
        exit 0
    fi
    
    # 尝试从 Registry 拉取（常见镜像）
    # 检查是否是已知的镜像名
    case "$IMAGE_NAME" in
        iarnet*)
            REGISTRY_IMAGE="$REGISTRY_URL/$IMAGE_NAME"
            echo "从 Registry 拉取 iarnet 镜像: $REGISTRY_IMAGE"
            if docker pull "$REGISTRY_IMAGE"; then
                docker tag "$REGISTRY_IMAGE" "$IMAGE_NAME"
                exit 0
            fi
            ;;
        *)
            # 对于其他镜像，也尝试从 Registry 拉取
            REGISTRY_IMAGE="$REGISTRY_URL/$IMAGE_NAME"
            if docker pull "$REGISTRY_IMAGE" 2>/dev/null; then
                docker tag "$REGISTRY_IMAGE" "$IMAGE_NAME"
                exit 0
            fi
            ;;
    esac
fi

# 如果都失败，执行原始 docker pull（让 Docker 显示原始错误）
docker pull "$IMAGE_NAME"
exit $?
'''
        
        # 创建包装脚本文件
        script_content = wrapper_script
        
        # 使用 base64 编码传递脚本
        import base64
        script_encoded = base64.b64encode(script_content.encode('utf-8')).decode('utf-8')
        
        # 在远程节点上创建包装脚本
        install_cmd = ' '.join(ssh_cmd) + f' "echo {script_encoded} | base64 -d | sudo tee /usr/local/bin/docker-pull-wrapper >/dev/null && sudo chmod +x /usr/local/bin/docker-pull-wrapper"'
        
        try:
            result = subprocess.run(install_cmd, shell=True, check=True, timeout=30, capture_output=True, text=True)
            
            # 创建 docker pull 别名（使用 bashrc）
            alias_cmd = ' '.join(ssh_cmd) + f' "echo \'alias docker-pull-original=\"$(which docker) pull\"\' | sudo tee -a /etc/bash.bashrc >/dev/null && echo \'alias docker-pull=\"docker-pull-wrapper\"\' | sudo tee -a /etc/bash.bashrc >/dev/null"'
            subprocess.run(alias_cmd, shell=True, check=False, timeout=10, capture_output=True)
            
            print(f"{node_prefix}✓ 包装脚本安装成功")
            return True
        except subprocess.CalledProcessError as e:
            print(f"{node_prefix}✗ 包装脚本安装失败: {e}")
            return False
    
    def configure_docker_daemon_fallback(self, ssh_cmd: list, node_id: int = None, node_info: dict = None) -> bool:
        """配置 Docker daemon 使用 Registry 作为镜像代理（更优雅的方法）"""
        if node_info:
            node_prefix = f"[节点 {node_id}] {node_info.get('hostname', '')} ({node_info.get('ip', '')}) "
        elif node_id is not None:
            node_prefix = f"[节点 {node_id}] "
        else:
            node_prefix = ""
        
        print(f"{node_prefix}配置 Docker daemon 镜像代理...")
        
        # 使用 Python 脚本配置 daemon.json
        import base64
        python_script = f'''
import json
import sys

registry_url = "{self.registry_url}"

# 读取现有配置
try:
    with open("/etc/docker/daemon.json", "r") as f:
        config = json.load(f)
except (FileNotFoundError, json.JSONDecodeError):
    config = {{}}

# 配置 registry-mirrors（用于加速，但不支持自动 fallback）
# 配置 insecure-registries（如果还没有）
if "insecure-registries" not in config:
    config["insecure-registries"] = []

if registry_url not in config["insecure-registries"]:
    config["insecure-registries"].append(registry_url)

# 写入配置
with open("/etc/docker/daemon.json", "w") as f:
    json.dump(config, f, indent=2)

print("OK")
'''
        
        script_encoded = base64.b64encode(python_script.encode('utf-8')).decode('utf-8')
        config_cmd = ' '.join(ssh_cmd) + f' "echo {script_encoded} | base64 -d | sudo python3 && sudo systemctl restart docker"'
        
        try:
            result = subprocess.run(config_cmd, shell=True, check=True, timeout=30, capture_output=True, text=True)
            time.sleep(2)  # 等待 Docker 重启
            print(f"{node_prefix}✓ Docker daemon 配置成功")
            return True
        except subprocess.CalledProcessError as e:
            print(f"{node_prefix}✗ Docker daemon 配置失败: {e}")
            return False
    
    def configure_node(self, node_id: int, node_type: str = 'docker') -> bool:
        """配置单个节点"""
        # 获取节点配置
        if node_type == 'docker':
            node_config = self.vm_config['vm_types']['docker']
        elif node_type == 'iarnet':
            node_config = self.vm_config['vm_types']['iarnet']
        else:
            return False
        
        ip_suffix = node_config['ip_start'] + node_id
        node_ip = f"{node_config['ip_base']}.{ip_suffix}"
        hostname = f"{node_config['hostname_prefix']}-{node_id+1:02d}"
        
        node_info = {
            'hostname': hostname,
            'ip': node_ip
        }
        
        # 检查连通性
        if not self.check_node_connectivity(node_ip):
            print(f"[节点 {node_id}] {hostname} ({node_ip}) ⚠ 无法连接，跳过")
            return False
        
        ssh_cmd = [
            'ssh',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f"{self.user}@{node_ip}"
        ]
        
        # 检查 Docker
        if not self.check_docker_installed(ssh_cmd):
            print(f"[节点 {node_id}] {hostname} ({node_ip}) ⚠ Docker 未安装，跳过")
            return False
        
        # 配置 Docker daemon
        if not self.configure_docker_daemon_fallback(ssh_cmd, node_id, node_info):
            return False
        
        # 安装包装脚本
        if not self.install_docker_pull_wrapper(ssh_cmd, node_id, node_info):
            return False
        
        return True
    
    def configure_nodes(self, node_type: str = 'docker', node_ids: list = None):
        """配置多个节点"""
        print(f"\n配置 {node_type} 节点以支持自动从 Registry 拉取镜像...")
        print(f"Registry URL: {self.registry_url}")
        print("=" * 60)
        
        # 获取节点列表
        if node_type == 'docker':
            node_config = self.vm_config['vm_types']['docker']
        elif node_type == 'iarnet':
            node_config = self.vm_config['vm_types']['iarnet']
        else:
            print(f"错误: 不支持的节点类型: {node_type}")
            return
        
        if node_ids is None:
            node_ids = list(range(node_config['count']))
        
        success_count = 0
        for node_id in node_ids:
            if self.configure_node(node_id, node_type):
                success_count += 1
        
        print("\n" + "=" * 60)
        print(f"配置完成: {success_count}/{len(node_ids)} 个节点成功")
        print("=" * 60)
        
        print(f"\n使用说明:")
        print(f"  配置后，可以使用以下方式拉取镜像:")
        print(f"  1. 直接使用镜像名（会自动从 Registry 拉取）:")
        print(f"     docker pull iarnet:latest")
        print(f"     docker pull components/my-component:latest")
        print(f"  2. 或者显式指定 Registry:")
        print(f"     docker pull {self.registry_url}/iarnet:latest")

def main():
    parser = argparse.ArgumentParser(description='配置节点以支持自动从 Registry 拉取镜像')
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--registry-node', '-n',
        type=int,
        default=0,
        help='Registry 节点 ID (默认: 0)'
    )
    parser.add_argument(
        '--node-type', '-t',
        type=str,
        choices=['docker', 'iarnet', 'all'],
        default='docker',
        help='要配置的节点类型 (默认: docker)'
    )
    parser.add_argument(
        '--nodes', '-N',
        type=str,
        default=None,
        help='指定节点ID列表，格式: 0,1,2 或 0-10 (默认: 所有节点)'
    )
    
    args = parser.parse_args()
    
    try:
        configurator = RegistryFallbackConfigurator(args.vm_config, args.registry_node)
        
        # 解析节点列表
        node_ids = None
        if args.nodes:
            if '-' in args.nodes:
                start, end = map(int, args.nodes.split('-'))
                node_ids = list(range(start, end + 1))
            else:
                node_ids = [int(x.strip()) for x in args.nodes.split(',')]
        
        if args.node_type == 'all':
            configurator.configure_nodes('iarnet', node_ids)
            print("\n")
            configurator.configure_nodes('docker', node_ids)
        else:
            configurator.configure_nodes(args.node_type, node_ids)
        
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()

