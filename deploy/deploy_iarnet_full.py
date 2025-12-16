#!/usr/bin/env python3
"""
完整的 iarnet 部署脚本
包括代码上传、依赖安装、构建和启动服务
"""

import os
import sys
import yaml
import argparse
import subprocess
import tarfile
from pathlib import Path

class IarnetFullDeployer:
    def __init__(self, vm_config_path: str, configs_dir: str = 'deploy/iarnet-configs', 
                 source_dir: str = '.'):
        """初始化完整部署器"""
        with open(vm_config_path, 'r', encoding='utf-8') as f:
            self.vm_config = yaml.safe_load(f)
        
        self.iarnet_config = self.vm_config['vm_types']['iarnet']
        self.configs_dir = Path(configs_dir)
        self.source_dir = Path(source_dir)
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
    
    def ssh_exec(self, node_id: int, command: str, check: bool = True) -> bool:
        """在节点上执行SSH命令"""
        node = self.node_info[node_id]
        ssh_cmd = [
            'ssh',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f"{self.user}@{node['ip']}",
            command
        ]
        try:
            result = subprocess.run(ssh_cmd, check=check, capture_output=True, text=True)
            return result.returncode == 0
        except subprocess.CalledProcessError:
            return False
    
    def scp_upload(self, node_id: int, local_path: str, remote_path: str) -> bool:
        """上传文件到节点"""
        node = self.node_info[node_id]
        scp_cmd = [
            'scp',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            local_path,
            f"{self.user}@{node['ip']}:{remote_path}"
        ]
        try:
            subprocess.run(scp_cmd, check=True, capture_output=True)
            return True
        except subprocess.CalledProcessError:
            return False
    
    def prepare_source_package(self) -> Path:
        """准备源代码包（排除不必要的文件）"""
        print("准备源代码包...")
        package_path = Path('/tmp/iarnet-source.tar.gz')
        
        exclude_patterns = [
            '.git',
            '__pycache__',
            '*.pyc',
            'node_modules',
            '.next',
            'data',
            'workspaces',
            'deploy/iarnet-configs',
            '*.log'
        ]
        
        with tarfile.open(package_path, 'w:gz') as tar:
            for root, dirs, files in os.walk(self.source_dir):
                # 排除目录
                dirs[:] = [d for d in dirs if not any(
                    pattern in d for pattern in exclude_patterns
                )]
                
                for file in files:
                    if any(pattern in file for pattern in exclude_patterns):
                        continue
                    
                    file_path = Path(root) / file
                    arcname = file_path.relative_to(self.source_dir)
                    tar.add(file_path, arcname=arcname)
        
        print(f"  ✓ 源代码包已创建: {package_path}")
        return package_path
    
    def deploy_to_node(self, node_id: int, upload_source: bool = True, 
                      install_deps: bool = True, build: bool = True, 
                      restart: bool = True) -> bool:
        """完整部署到指定节点"""
        if node_id not in self.node_info:
            print(f"错误: 节点 {node_id} 不存在")
            return False
        
        node = self.node_info[node_id]
        config_file = node['config_file']
        
        if not config_file.exists():
            print(f"错误: 配置文件不存在: {config_file}")
            print(f"请先运行: python3 generate_iarnet_configs.py --nodes {node_id}")
            return False
        
        print(f"\n部署到节点 {node_id}: {node['hostname']} ({node['ip']})")
        print("=" * 60)
        
        # 检查连通性
        if not self.check_node_connectivity(node_id):
            print(f"错误: 无法连接到节点 {node_id} ({node['ip']})")
            return False
        
        # 1. 创建目录结构
        print("1. 创建目录结构...")
        if not self.ssh_exec(node_id, "mkdir -p ~/iarnet/{data,workspaces,logs}"):
            print("  ✗ 目录创建失败")
            return False
        print("  ✓ 目录创建成功")
        
        # 2. 上传源代码
        if upload_source:
            print("2. 上传源代码...")
            package_path = self.prepare_source_package()
            if not self.scp_upload(node_id, str(package_path), "~/iarnet-source.tar.gz"):
                print("  ✗ 源代码上传失败")
                return False
            
            # 解压源代码
            if not self.ssh_exec(node_id, "cd ~ && rm -rf iarnet/src && mkdir -p iarnet/src && tar -xzf iarnet-source.tar.gz -C iarnet/src"):
                print("  ✗ 源代码解压失败")
                return False
            print("  ✓ 源代码上传成功")
        
        # 3. 上传配置文件
        print("3. 上传配置文件...")
        if not self.scp_upload(node_id, str(config_file), "~/iarnet/config.yaml"):
            print("  ✗ 配置文件上传失败")
            return False
        print("  ✓ 配置文件上传成功")
        
        # 4. 安装依赖
        if install_deps:
            print("4. 安装依赖...")
            install_cmd = """
                cd ~/iarnet/src && \
                sudo apt-get update && \
                sudo apt-get install -y golang-go docker.io docker-compose || true && \
                sudo systemctl start docker || true && \
                sudo systemctl enable docker || true
            """
            if not self.ssh_exec(node_id, install_cmd, check=False):
                print("  ⚠ 依赖安装可能有问题，继续...")
            else:
                print("  ✓ 依赖安装成功")
        
        # 5. 构建
        if build:
            print("5. 构建 iarnet...")
            build_cmd = "cd ~/iarnet/src && go mod download && go build -o ~/iarnet/iarnet cmd/main.go"
            if not self.ssh_exec(node_id, build_cmd):
                print("  ✗ 构建失败")
                return False
            print("  ✓ 构建成功")
        
        # 6. 启动服务
        if restart:
            print("6. 启动服务...")
            # 停止现有服务
            self.ssh_exec(node_id, "pkill -f 'iarnet.*config.yaml' || true", check=False)
            
            # 启动服务
            start_cmd = "cd ~/iarnet && nohup ./iarnet --config=config.yaml > iarnet.log 2>&1 &"
            if not self.ssh_exec(node_id, start_cmd):
                print("  ✗ 服务启动失败")
                return False
            
            # 等待一下，检查服务是否启动
            import time
            time.sleep(2)
            if self.ssh_exec(node_id, "pgrep -f 'iarnet.*config.yaml' > /dev/null", check=False):
                print("  ✓ 服务启动成功")
            else:
                print("  ⚠ 服务可能未正常启动，请检查日志: ~/iarnet/iarnet.log")
        
        print("=" * 60)
        print(f"✓ 节点 {node_id} 部署完成")
        return True
    
    def deploy_to_nodes(self, node_ids: list, **kwargs):
        """批量部署到多个节点"""
        print(f"批量部署到节点: {node_ids}")
        print("=" * 60)
        
        success_count = 0
        for node_id in node_ids:
            if self.deploy_to_node(node_id, **kwargs):
                success_count += 1
        
        print("\n" + "=" * 60)
        print(f"部署完成: {success_count}/{len(node_ids)} 个节点成功")
        print("=" * 60)

def main():
    parser = argparse.ArgumentParser(description='完整部署 iarnet 到节点')
    parser.add_argument(
        '--vm-config', '-v',
        default='deploy/vm-config.yaml',
        help='虚拟机配置文件路径'
    )
    parser.add_argument(
        '--configs-dir', '-c',
        default='deploy/iarnet-configs',
        help='配置文件目录'
    )
    parser.add_argument(
        '--source-dir', '-s',
        default='.',
        help='源代码目录'
    )
    parser.add_argument(
        '--node', '-n',
        type=int,
        help='部署到单个节点'
    )
    parser.add_argument(
        '--nodes', '-N',
        type=str,
        help='部署到多个节点，格式: start-end 或逗号分隔的列表'
    )
    parser.add_argument(
        '--no-upload-source',
        action='store_true',
        help='不上传源代码（假设已存在）'
    )
    parser.add_argument(
        '--no-install-deps',
        action='store_true',
        help='不安装依赖'
    )
    parser.add_argument(
        '--no-build',
        action='store_true',
        help='不构建二进制文件'
    )
    parser.add_argument(
        '--no-restart',
        action='store_true',
        help='不重启服务'
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
        deployer = IarnetFullDeployer(
            args.vm_config, 
            args.configs_dir,
            args.source_dir
        )
        deployer.deploy_to_nodes(
            node_ids,
            upload_source=not args.no_upload_source,
            install_deps=not args.no_install_deps,
            build=not args.no_build,
            restart=not args.no_restart
        )
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)

if __name__ == '__main__':
    main()

