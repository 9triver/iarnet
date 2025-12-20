#!/usr/bin/env python3
"""
将本地 iarnet 和 runner 镜像同步到 iarnet 虚拟机的 Docker 引擎
便捷脚本，专门用于同步 iarnet 相关镜像
"""

import os
import sys
import argparse
from pathlib import Path

# 获取脚本所在目录
SCRIPT_DIR = Path(__file__).parent.absolute()

# 导入现有的镜像同步器
from sync_images_to_nodes import ImageSyncer

def main():
    parser = argparse.ArgumentParser(
        description='将本地 iarnet 和 runner 镜像同步到 iarnet 虚拟机的 Docker 引擎',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 同步默认镜像 (iarnet:latest 和 iarnet/runner:python_3.11-latest)
  python3 deploy/sync_iarnet_images.py

  # 指定自定义镜像名
  python3 deploy/sync_iarnet_images.py \\
      --iarnet-image my-iarnet:v1.0 \\
      --runner-image my-runner:latest

  # 只同步 iarnet 镜像
  python3 deploy/sync_iarnet_images.py --iarnet-only

  # 只同步 runner 镜像
  python3 deploy/sync_iarnet_images.py --runner-only

  # 同步到指定节点
  python3 deploy/sync_iarnet_images.py --nodes 0-5

  # 强制同步（即使已存在）
  python3 deploy/sync_iarnet_images.py --force
        """
    )
    
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--iarnet-image', '-i',
        type=str,
        default='iarnet:latest',
        help='iarnet 镜像名 (默认: iarnet:latest)'
    )
    parser.add_argument(
        '--runner-image', '-r',
        type=str,
        default='iarnet/runner:python_3.11-latest',
        help='runner 镜像名 (默认: iarnet/runner:python_3.11-latest)'
    )
    parser.add_argument(
        '--iarnet-only',
        action='store_true',
        help='只同步 iarnet 镜像'
    )
    parser.add_argument(
        '--runner-only',
        action='store_true',
        help='只同步 runner 镜像'
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
    parser.add_argument(
        '--check-only',
        action='store_true',
        help='只检查镜像是否存在，不进行同步'
    )
    
    args = parser.parse_args()
    
    # 确定要同步的镜像列表
    images_to_sync = []
    
    if args.iarnet_only:
        images_to_sync = [args.iarnet_image]
    elif args.runner_only:
        images_to_sync = [args.runner_image]
    else:
        images_to_sync = [args.iarnet_image, args.runner_image]
    
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
        
        # 检查镜像是否存在
        print("=" * 60)
        print("检查本地镜像...")
        print("=" * 60)
        
        missing_images = []
        for image_name in images_to_sync:
            if syncer.check_image_exists_local(image_name):
                print(f"  ✓ {image_name} - 存在")
            else:
                print(f"  ✗ {image_name} - 不存在")
                missing_images.append(image_name)
        
        if missing_images:
            print(f"\n错误: 以下镜像在本地不存在:")
            for img in missing_images:
                print(f"  - {img}")
            print(f"\n请先构建或拉取这些镜像:")
            for img in missing_images:
                print(f"  docker build/pull {img}")
            sys.exit(1)
        
        if args.check_only:
            print("\n✓ 所有镜像都存在，检查完成")
            sys.exit(0)
        
        # 同步镜像到 iarnet 节点
        print("\n" + "=" * 60)
        print("开始同步镜像到 iarnet 虚拟机...")
        print("=" * 60)
        
        success = syncer.sync_multiple_images(
            images_to_sync,
            node_type='iarnet',
            node_ids=node_ids,
            max_workers=args.max_workers,
            force=args.force
        )
        
        if success:
            print("\n✓ 所有镜像同步成功！")
            sys.exit(0)
        else:
            print("\n✗ 部分镜像同步失败")
            sys.exit(1)
        
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()

