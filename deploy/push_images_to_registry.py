#!/usr/bin/env python3
"""
推送 Docker 镜像到共享 Registry
支持推送 iarnet 镜像和 component 镜像
"""

import os
import sys
import yaml
import argparse
import subprocess
from pathlib import Path

# 获取脚本所在目录
SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent

class ImagePusher:
    def __init__(self, vm_config_path: str, registry_node_id: int = 0):
        """初始化镜像推送器"""
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
        
        # 计算 Registry 地址
        iarnet_config = self.vm_config['vm_types']['iarnet']
        ip_suffix = iarnet_config['ip_start'] + registry_node_id
        self.registry_ip = f"{iarnet_config['ip_base']}.{ip_suffix}"
        self.registry_port = 5000
        self.registry_url = f"{self.registry_ip}:{self.registry_port}"
    
    def check_image_exists(self, image_name: str) -> bool:
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
    
    def build_iarnet_image(self, dockerfile_path: Path, tag: str = None) -> bool:
        """构建 iarnet 镜像"""
        if tag is None:
            tag = f"iarnet:latest"
        
        print(f"构建 iarnet 镜像: {tag}...")
        print(f"  Dockerfile: {dockerfile_path}")
        
        if not dockerfile_path.exists():
            print(f"  ✗ Dockerfile 不存在: {dockerfile_path}")
            return False
        
        build_cmd = [
            'docker', 'build',
            '-t', tag,
            '-f', str(dockerfile_path),
            str(PROJECT_ROOT)
        ]
        
        try:
            result = subprocess.run(
                build_cmd,
                check=True,
                capture_output=False  # 显示构建输出
            )
            print(f"  ✓ 镜像构建成功: {tag}")
            return True
        except subprocess.CalledProcessError as e:
            print(f"  ✗ 镜像构建失败: {e}")
            return False
    
    def tag_image(self, source_image: str, target_image: str) -> bool:
        """标记镜像"""
        print(f"标记镜像: {source_image} -> {target_image}...")
        try:
            subprocess.run(
                ['docker', 'tag', source_image, target_image],
                check=True,
                capture_output=True
            )
            print(f"  ✓ 镜像标记成功")
            return True
        except subprocess.CalledProcessError as e:
            print(f"  ✗ 镜像标记失败: {e}")
            return False
    
    def push_image(self, image_name: str) -> bool:
        """推送镜像到 Registry"""
        print(f"推送镜像: {image_name}...")
        try:
            result = subprocess.run(
                ['docker', 'push', image_name],
                check=True,
                capture_output=False  # 显示推送进度
            )
            print(f"  ✓ 镜像推送成功: {image_name}")
            return True
        except subprocess.CalledProcessError as e:
            print(f"  ✗ 镜像推送失败: {e}")
            return False
    
    def push_iarnet_image(self, local_image: str = None, remote_tag: str = None) -> bool:
        """推送 iarnet 镜像"""
        if local_image is None:
            local_image = "iarnet:latest"
        
        if remote_tag is None:
            remote_tag = f"{self.registry_url}/iarnet:latest"
        
        # 检查本地镜像是否存在
        if not self.check_image_exists(local_image):
            print(f"本地镜像不存在: {local_image}")
            print("请先构建镜像或指定已存在的镜像")
            return False
        
        # 标记镜像
        if not self.tag_image(local_image, remote_tag):
            return False
        
        # 推送镜像
        if not self.push_image(remote_tag):
            return False
        
        return True
    
    def push_component_image(self, local_image: str, component_name: str = None, tag: str = "latest") -> bool:
        """推送 component 镜像"""
        if component_name is None:
            # 从镜像名提取 component 名称
            if '/' in local_image:
                component_name = local_image.split('/')[-1].split(':')[0]
            else:
                component_name = local_image.split(':')[0]
        
        # 检查本地镜像是否存在
        if not self.check_image_exists(local_image):
            print(f"本地镜像不存在: {local_image}")
            return False
        
        remote_tag = f"{self.registry_url}/components/{component_name}:{tag}"
        
        # 标记镜像
        if not self.tag_image(local_image, remote_tag):
            return False
        
        # 推送镜像
        if not self.push_image(remote_tag):
            return False
        
        return True
    
    def list_registry_images(self) -> bool:
        """列出 Registry 中的镜像"""
        print(f"列出 Registry 中的镜像: {self.registry_url}...")
        try:
            # 使用 Registry API 列出镜像
            result = subprocess.run(
                ['curl', '-s', f'http://{self.registry_url}/v2/_catalog'],
                capture_output=True,
                text=True,
                check=True
            )
            print(f"Registry 镜像列表:")
            print(result.stdout)
            return True
        except subprocess.CalledProcessError as e:
            print(f"  ✗ 无法列出镜像: {e}")
            return False

def main():
    parser = argparse.ArgumentParser(description='推送 Docker 镜像到共享 Registry')
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
        '--build-iarnet', '-b',
        action='store_true',
        help='构建并推送 iarnet 镜像'
    )
    parser.add_argument(
        '--push-iarnet', '-p',
        type=str,
        default=None,
        help='推送已存在的 iarnet 镜像（指定镜像名，如 iarnet:latest）'
    )
    parser.add_argument(
        '--push-component', '-c',
        type=str,
        nargs=2,
        metavar=('IMAGE', 'NAME'),
        help='推送 component 镜像（镜像名 组件名）'
    )
    parser.add_argument(
        '--list', '-l',
        action='store_true',
        help='列出 Registry 中的镜像'
    )
    parser.add_argument(
        '--dockerfile',
        type=str,
        default=str(PROJECT_ROOT / 'Dockerfile'),
        help='iarnet Dockerfile 路径 (默认: Dockerfile)'
    )
    
    args = parser.parse_args()
    
    try:
        pusher = ImagePusher(args.vm_config, args.registry_node)
        
        if args.list:
            pusher.list_registry_images()
        elif args.build_iarnet:
            # 构建并推送 iarnet 镜像
            dockerfile_path = Path(args.dockerfile)
            if not dockerfile_path.is_absolute():
                dockerfile_path = PROJECT_ROOT / dockerfile_path
            
            if pusher.build_iarnet_image(dockerfile_path):
                pusher.push_iarnet_image()
        elif args.push_iarnet:
            # 推送已存在的 iarnet 镜像
            pusher.push_iarnet_image(args.push_iarnet)
        elif args.push_component:
            # 推送 component 镜像
            image_name, component_name = args.push_component
            pusher.push_component_image(image_name, component_name)
        else:
            parser.print_help()
            print("\n示例:")
            print("  # 构建并推送 iarnet 镜像")
            print("  python3 push_images_to_registry.py --build-iarnet")
            print("\n  # 推送已存在的 iarnet 镜像")
            print("  python3 push_images_to_registry.py --push-iarnet iarnet:latest")
            print("\n  # 推送 component 镜像")
            print("  python3 push_images_to_registry.py --push-component my-component:latest my-component")
            print("\n  # 列出 Registry 中的镜像")
            print("  python3 push_images_to_registry.py --list")
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()

