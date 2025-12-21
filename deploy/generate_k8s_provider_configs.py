#!/usr/bin/env python3
"""
为每个 K8s Master 节点生成独立的 K8s Provider 配置文件
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

def generate_config_for_cluster(cluster_id: int, base_config: dict, master_config: dict, worker_config: dict) -> dict:
    """为指定集群的 master 节点生成配置文件"""
    config = base_config.copy()
    
    # 计算 master 节点IP地址
    # 每个集群占用 ip_step 个IP，master 是第一个
    ip_suffix = master_config['ip_start'] + cluster_id * master_config['ip_step']
    node_ip = f"{master_config['ip_base']}.{ip_suffix}"
    hostname = f"{master_config['hostname_prefix']}-{cluster_id+1:02d}{master_config['hostname_suffix']}"
    
    # 设置 gRPC 服务端口（每个集群使用不同的端口）
    provider_port = master_config['provider_port_base'] + cluster_id
    config['server']['port'] = provider_port
    
    # 为每个节点随机生成 resource_tags
    # 使用集群ID作为随机种子，确保每次生成的标签一致
    random.seed(cluster_id)
    
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
    
    # 设置 CPU 和 Memory（使用整个集群的总资源：master + workers）
    if 'resource' not in config:
        config['resource'] = {}
    
    # 计算整个集群的总资源
    # Master 资源
    master_cpu = master_config.get('cpu', 2)
    master_memory = master_config.get('memory', 2048)
    
    # Worker 资源（每个集群有 count_per_cluster 个 worker）
    worker_count = worker_config.get('count_per_cluster', 2)
    worker_cpu = worker_config.get('cpu', 1)
    worker_memory = worker_config.get('memory', 1024)
    
    # 总资源 = master + workers
    total_cpu = master_cpu + worker_cpu * worker_count
    total_memory = master_memory + worker_memory * worker_count
    
    # CPU: 转换为 millicores (1 core = 1000 millicores)
    config['resource']['cpu'] = total_cpu * 1000
    
    # Memory: 转换为 "XXXMi" 格式
    config['resource']['memory'] = f"{total_memory}Mi"
    
    # GPU: 如果 resource_tags 中包含 'gpu'，则随机生成 1-8 的值；否则为 0
    if 'gpu' in selected_tags:
        # 使用集群ID作为随机种子，确保每次生成的GPU数量一致
        random.seed(cluster_id + 1000)  # 使用不同的种子避免与 tags 选择冲突
        config['resource']['gpu'] = random.randint(1, 8)
    else:
        config['resource']['gpu'] = 0
    
    # 设置 Kubernetes 配置
    if 'kubernetes' not in config:
        config['kubernetes'] = {}
    
    # provider 使用 sudo 运行，以 root 用户执行，所以使用 root 的 kubeconfig 文件
    config['kubernetes']['kubeconfig'] = "/root/.kube/config"
    
    # 不使用 in-cluster 配置（因为 provider 不是在 Pod 中运行）
    config['kubernetes']['in_cluster'] = False
    
    # 命名空间使用 default
    config['kubernetes']['namespace'] = "default"
    
    # 确保 allow_connection_failure 为 true
    config['kubernetes']['allow_connection_failure'] = True
    
    print(f"  生成集群 {cluster_id} 配置: {hostname} ({node_ip}), port: {provider_port}, "
          f"资源: CPU={total_cpu}核, Memory={total_memory}MB, tags: {selected_tags}")
    return config

def main():
    parser = argparse.ArgumentParser(description='为 K8s Master 节点生成独立的 K8s Provider 配置文件')
    parser.add_argument(
        '--base-config', '-b',
        default=str(PROJECT_ROOT / 'providers' / 'k8s' / 'config.yaml'),
        help='基础配置文件路径 (默认: providers/k8s/config.yaml)'
    )
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--output-dir', '-o',
        default=str(SCRIPT_DIR / 'k8s-provider-configs'),
        help='输出目录 (默认: deploy/k8s-provider-configs)'
    )
    parser.add_argument(
        '--clusters', '-c',
        type=str,
        default='0-39',
        help='集群范围，格式: start-end 或逗号分隔的列表 (默认: 0-39，即所有40个集群)'
    )
    
    args = parser.parse_args()
    
    # 解析集群范围
    if '-' in args.clusters:
        start, end = map(int, args.clusters.split('-'))
        cluster_ids = list(range(start, end + 1))
    else:
        cluster_ids = [int(x.strip()) for x in args.clusters.split(',')]
    
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
    
    k8s_config = vm_config_data['vm_types']['k8s_clusters']
    master_config = k8s_config['master']
    worker_config = k8s_config['worker']
    cluster_count = k8s_config['count']
    
    # 验证集群ID范围
    max_cluster_id = cluster_count - 1
    invalid_clusters = [c for c in cluster_ids if c < 0 or c > max_cluster_id]
    if invalid_clusters:
        print(f"错误: 集群ID超出范围: {invalid_clusters}")
        print(f"有效范围: 0-{max_cluster_id}")
        sys.exit(1)
    
    # 创建输出目录
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    
    print(f"为 K8s Master 节点（集群 {cluster_ids}）生成配置文件...")
    print(f"输出目录: {output_dir}")
    print("=" * 60)
    
    # 为每个集群的 master 节点生成配置
    for cluster_id in cluster_ids:
        config = generate_config_for_cluster(cluster_id, base_config, master_config, worker_config)
        
        # 保存配置文件
        config_file = output_dir / f"config-cluster-{cluster_id:02d}.yaml"
        with open(config_file, 'w', encoding='utf-8') as f:
            yaml.dump(config, f, default_flow_style=False, allow_unicode=True, sort_keys=False)
        
        # 计算节点信息用于显示
        ip_suffix = master_config['ip_start'] + cluster_id * master_config['ip_step']
        node_ip = f"{master_config['ip_base']}.{ip_suffix}"
        hostname = f"{master_config['hostname_prefix']}-{cluster_id+1:02d}{master_config['hostname_suffix']}"
        provider_port = master_config['provider_port_base'] + cluster_id
        
        print(f"✓ 集群 {cluster_id:2d}: {hostname} ({node_ip}) -> {config_file} (port: {provider_port})")
    
    print("=" * 60)
    print(f"完成！共生成 {len(cluster_ids)} 个配置文件")
    print(f"\n配置文件位置: {output_dir}")
    print(f"\n使用方式:")
    print(f"  # 部署到集群0的 master 节点:")
    print(f"  python3 deploy_k8s_provider.py --cluster 0")
    print(f"  # 或批量部署:")
    print(f"  python3 deploy_k8s_provider.py --clusters 0-{max_cluster_id}")

if __name__ == '__main__':
    main()

