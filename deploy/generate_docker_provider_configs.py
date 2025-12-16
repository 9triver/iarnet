#!/usr/bin/env python3
"""
为每个 Docker Provider 节点生成独立的配置文件
"""

import os
import sys
import yaml
import argparse
import random
from pathlib import Path

# 获取脚本所在目录和项目根目录
SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent

def generate_config_for_node(node_id: int, base_config: dict, vm_config: dict) -> dict:
    """为指定节点生成配置文件"""
    config = base_config.copy()
    
    # 计算节点IP地址
    ip_suffix = vm_config['ip_start'] + node_id
    node_ip = f"{vm_config['ip_base']}.{ip_suffix}"
    hostname = f"{vm_config['hostname_prefix']}-{node_id+1:02d}"
    
    # 为每个节点随机生成 resource_tags
    # 使用节点ID作为随机种子，确保每次生成的标签一致
    random.seed(node_id)
    
    # cpu 和 memory 是必须的标签
    required_tags = ['cpu', 'memory']
    
    # 可选的标签列表（gpu 和 camera）
    optional_tags = ['gpu', 'camera']
    
    # 随机选择可选标签（0-2个）
    num_optional = random.randint(0, len(optional_tags))
    selected_optional = random.sample(optional_tags, num_optional) if num_optional > 0 else []
    
    # 合并必须标签和可选标签
    selected_tags = required_tags + selected_optional
    
    # 对标签进行排序，确保输出一致（cpu, memory 在前，然后是 gpu, camera）
    selected_tags.sort()
    
    # 更新配置中的 resource_tags
    config['resource_tags'] = selected_tags
    
    # 设置 CPU 和 Memory（使用 vm_config 中的值）
    if 'resource' not in config:
        config['resource'] = {}
    
    # CPU: vm_config 中的 cpu 是核心数，需要转换为 millicores (1 core = 1000 millicores)
    cpu_cores = vm_config.get('cpu', 1)
    config['resource']['cpu'] = cpu_cores * 1000
    
    # Memory: vm_config 中的 memory 是 MB，转换为 "XXXMi" 格式
    memory_mb = vm_config.get('memory', 1024)
    config['resource']['memory'] = f"{memory_mb}Mi"
    
    # GPU: 如果 resource_tags 中包含 'gpu'，则随机生成 1-8 的值；否则为 0
    if 'gpu' in selected_tags:
        # 使用节点ID作为随机种子，确保每次生成的GPU数量一致
        random.seed(node_id + 1000)  # 使用不同的种子避免与 tags 选择冲突
        config['resource']['gpu'] = random.randint(1, 8)
    else:
        config['resource']['gpu'] = 0
    
    # 确保 allow_connection_failure 为 true
    if 'docker' not in config:
        config['docker'] = {}
    config['docker']['allow_connection_failure'] = True
    
    return config

def main():
    parser = argparse.ArgumentParser(description='为 Docker Provider 节点生成独立的配置文件')
    parser.add_argument(
        '--base-config', '-b',
        default=str(PROJECT_ROOT / 'providers' / 'docker' / 'config.yaml'),
        help='基础配置文件路径 (默认: providers/docker/config.yaml)'
    )
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--output-dir', '-o',
        default=str(SCRIPT_DIR / 'docker-provider-configs'),
        help='输出目录 (默认: deploy/docker-provider-configs)'
    )
    parser.add_argument(
        '--nodes', '-n',
        type=str,
        default='0-59',
        help='节点范围，格式: start-end 或逗号分隔的列表 (默认: 0-59，即所有60个节点)'
    )
    
    args = parser.parse_args()
    
    # 解析节点范围
    if '-' in args.nodes:
        start, end = map(int, args.nodes.split('-'))
        node_ids = list(range(start, end + 1))
    else:
        node_ids = [int(x.strip()) for x in args.nodes.split(',')]
    
    # 解析路径（支持相对路径和绝对路径）
    base_config_path = Path(args.base_config)
    if not base_config_path.is_absolute():
        base_config_path = PROJECT_ROOT / base_config_path
    
    vm_config_path = Path(args.vm_config)
    if not vm_config_path.is_absolute():
        vm_config_path = PROJECT_ROOT / vm_config_path if 'deploy' in str(vm_config_path) else SCRIPT_DIR / vm_config_path
    
    # 读取基础配置
    if not base_config_path.exists():
        print(f"错误: 基础配置文件不存在: {base_config_path}")
        sys.exit(1)
    
    with open(base_config_path, 'r', encoding='utf-8') as f:
        base_config = yaml.safe_load(f)
    
    # 读取虚拟机配置
    if not vm_config_path.exists():
        print(f"错误: 虚拟机配置文件不存在: {vm_config_path}")
        sys.exit(1)
    
    with open(vm_config_path, 'r', encoding='utf-8') as f:
        vm_config_data = yaml.safe_load(f)
    
    docker_config = vm_config_data['vm_types']['docker']
    
    # 验证节点ID范围
    max_node_id = docker_config['count'] - 1
    invalid_nodes = [n for n in node_ids if n < 0 or n > max_node_id]
    if invalid_nodes:
        print(f"错误: 节点ID超出范围: {invalid_nodes}")
        print(f"有效范围: 0-{max_node_id}")
        sys.exit(1)
    
    # 创建输出目录
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    
    print(f"为 Docker Provider 节点 {node_ids} 生成配置文件...")
    print(f"输出目录: {output_dir}")
    print("=" * 60)
    
    # 为每个节点生成配置
    for node_id in node_ids:
        config = generate_config_for_node(node_id, base_config, docker_config)
        
        # 保存配置文件
        config_file = output_dir / f"config-node-{node_id:02d}.yaml"
        with open(config_file, 'w', encoding='utf-8') as f:
            yaml.dump(config, f, default_flow_style=False, allow_unicode=True, sort_keys=False)
        
        ip_suffix = docker_config['ip_start'] + node_id
        node_ip = f"{docker_config['ip_base']}.{ip_suffix}"
        hostname = f"{docker_config['hostname_prefix']}-{node_id+1:02d}"
        
        print(f"✓ 节点 {node_id:2d}: {hostname} ({node_ip}) -> {config_file}")
    
    print("=" * 60)
    print(f"完成！共生成 {len(node_ids)} 个配置文件")
    print(f"\n配置文件位置: {output_dir}")
    print(f"\n使用方式:")
    print(f"  # 部署到节点0:")
    print(f"  python3 deploy_docker_provider.py --node 0")
    print(f"  # 或批量部署:")
    print(f"  python3 deploy_docker_provider.py --nodes 0-59")

if __name__ == '__main__':
    main()

