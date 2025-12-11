#!/usr/bin/env python3
"""
为每个 iarnet 节点生成独立的配置文件
"""

import os
import sys
import yaml
import argparse
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
    
    # 更新host地址
    config['host'] = node_ip
    
    # 更新节点名称
    config['resource']['name'] = f"node.{node_id}"
    config['resource']['description'] = f"{hostname} - iarnet node {node_id}"
    
    # 更新peer_listen_addr（如果需要）
    if 'peer_listen_addr' in config:
        # 保持端口不变，只更新IP
        port = config['peer_listen_addr'].split(':')[-1] if ':' in config['peer_listen_addr'] else '50051'
        config['peer_listen_addr'] = f":{port}"
    
    # 更新initial_peers（排除自己）
    # 注意：initial_peers 应该使用 discovery 端口，而不是 resource peer_port
    if 'initial_peers' in config:
        initial_peers = []
        # 获取 discovery 端口（默认 50005）
        discovery_port = config.get('transport', {}).get('rpc', {}).get('discovery', {}).get('port', 50005)
        if discovery_port == 0:
            discovery_port = 50005  # 如果为 0，使用默认值
        
        for i in range(vm_config['count']):
            if i != node_id:
                peer_ip_suffix = vm_config['ip_start'] + i
                peer_ip = f"{vm_config['ip_base']}.{peer_ip_suffix}"
                initial_peers.append(f"{peer_ip}:{discovery_port}")
        config['initial_peers'] = initial_peers
    
    # 更新数据目录（使用节点特定的目录）
    if 'data_dir' in config:
        config['data_dir'] = f"./data/node_{node_id}"
    
    # 更新数据库路径
    if 'database' in config:
        if 'application_db_path' in config['database']:
            config['database']['application_db_path'] = f"./data/node_{node_id}/application.db"
        if 'resource_provider_db_path' in config['database']:
            config['database']['resource_provider_db_path'] = f"./data/node_{node_id}/resource_provider.db"
        if 'resource_logger_db_path' in config['database']:
            config['database']['resource_logger_db_path'] = f"./data/node_{node_id}/resource_logger.db"
    
    # 更新日志目录
    if 'logging' in config:
        if 'data_dir' in config['logging']:
            config['logging']['data_dir'] = f"./data/node_{node_id}/logs"
        if 'db_path' in config['logging']:
            config['logging']['db_path'] = f"./data/node_{node_id}/logs.db"
    
    return config

def main():
    parser = argparse.ArgumentParser(description='为 iarnet 节点生成独立的配置文件')
    parser.add_argument(
        '--base-config', '-b',
        default=str(PROJECT_ROOT / 'config.yaml'),
        help='基础配置文件路径 (默认: 项目根目录/config.yaml)'
    )
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--output-dir', '-o',
        default=str(SCRIPT_DIR / 'iarnet-configs'),
        help='输出目录 (默认: deploy/iarnet-configs)'
    )
    parser.add_argument(
        '--nodes', '-n',
        type=str,
        default='0-10',
        help='节点范围，格式: start-end 或逗号分隔的列表 (默认: 0-10)'
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
    
    iarnet_config = vm_config_data['vm_types']['iarnet']
    
    # 验证节点ID范围
    max_node_id = iarnet_config['count'] - 1
    invalid_nodes = [n for n in node_ids if n < 0 or n > max_node_id]
    if invalid_nodes:
        print(f"错误: 节点ID超出范围: {invalid_nodes}")
        print(f"有效范围: 0-{max_node_id}")
        sys.exit(1)
    
    # 创建输出目录
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    
    print(f"为节点 {node_ids} 生成配置文件...")
    print(f"输出目录: {output_dir}")
    print("=" * 60)
    
    # 为每个节点生成配置
    for node_id in node_ids:
        config = generate_config_for_node(node_id, base_config, iarnet_config)
        
        # 保存配置文件
        config_file = output_dir / f"config-node-{node_id:02d}.yaml"
        with open(config_file, 'w', encoding='utf-8') as f:
            yaml.dump(config, f, default_flow_style=False, allow_unicode=True, sort_keys=False)
        
        ip_suffix = iarnet_config['ip_start'] + node_id
        node_ip = f"{iarnet_config['ip_base']}.{ip_suffix}"
        hostname = f"{iarnet_config['hostname_prefix']}-{node_id+1:02d}"
        
        print(f"✓ 节点 {node_id:2d}: {hostname} ({node_ip}) -> {config_file}")
    
    print("=" * 60)
    print(f"完成！共生成 {len(node_ids)} 个配置文件")
    print(f"\n配置文件位置: {output_dir}")
    print(f"\n使用方式:")
    print(f"  # 部署到节点0:")
    print(f"  python3 deploy_iarnet.py --node 0")
    print(f"  # 或批量部署:")
    print(f"  python3 deploy_iarnet.py --nodes 0-10")

if __name__ == '__main__':
    main()

