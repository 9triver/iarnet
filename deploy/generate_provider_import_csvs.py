#!/usr/bin/env python3
"""
为 10 个 iarnet 节点生成 provider 批量导入 CSV 文件
将 100 个资源节点（60 个 docker + 40 个 k8s）随机分配给 10 个 iarnet 节点
"""

import os
import sys
import yaml
import argparse
import random
import csv
from pathlib import Path
from collections import defaultdict

# 获取脚本所在目录和项目根目录
SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent

# Provider 名称模板（体现这是提供资源的设备）
PROVIDER_NAME_TEMPLATES = [
    "计算节点-{id:02d}",
    "存储节点-{id:02d}",
    "GPU节点-{id:02d}",
    "AI计算节点-{id:02d}",
    "边缘计算节点-{id:02d}",
    "高性能计算节点-{id:02d}",
    "容器计算节点-{id:02d}",
    "云原生节点-{id:02d}",
    "资源池节点-{id:02d}",
    "智能计算节点-{id:02d}",
    "分布式计算节点-{id:02d}",
    "弹性计算节点-{id:02d}",
    "混合云节点-{id:02d}",
    "超融合节点-{id:02d}",
    "虚拟化节点-{id:02d}",
]

def generate_provider_name(provider_type: str, global_index: int, used_names: set) -> str:
    """生成唯一的 provider 名称"""
    # 根据类型选择不同的模板前缀
    if provider_type == "docker":
        templates = ["容器节点-{id:03d}", "Docker节点-{id:03d}", "容器计算节点-{id:03d}", 
                     "容器资源节点-{id:03d}", "容器服务节点-{id:03d}", "容器引擎节点-{id:03d}",
                     "容器化节点-{id:03d}", "容器平台节点-{id:03d}"]
    elif provider_type == "k8s":
        templates = ["K8s节点-{id:03d}", "Kubernetes节点-{id:03d}", "集群节点-{id:03d}",
                     "K8s计算节点-{id:03d}", "K8s资源节点-{id:03d}", "K8s集群节点-{id:03d}",
                     "容器编排节点-{id:03d}", "云原生节点-{id:03d}"]
    else:
        templates = [t.replace("{id:02d}", "{id:03d}") for t in PROVIDER_NAME_TEMPLATES]
    
    # 尝试生成唯一名称（最多尝试 20 次）
    for _ in range(20):
        template = random.choice(templates)
        name = template.format(id=global_index + 1)
        if name not in used_names:
            used_names.add(name)
            return name
    
    # 如果都重复，使用带类型前缀的唯一编号
    name = f"{provider_type}-节点-{global_index+1:03d}"
    used_names.add(name)
    return name

def generate_random_allocation(docker_count: int, k8s_count: int, iarnet_count: int, seed: int = None):
    """生成随机分配方案（非完全平均）"""
    if seed is not None:
        random.seed(seed)
    
    # 创建所有 provider 列表
    all_providers = []
    used_names = set()  # 用于确保名称唯一
    global_index = 0
    
    # Docker providers (60个)
    for i in range(docker_count):
        all_providers.append({
            'type': 'docker',
            'index': i,
            'name': generate_provider_name('docker', global_index, used_names)
        })
        global_index += 1
    
    # K8s providers (40个)
    for i in range(k8s_count):
        all_providers.append({
            'type': 'k8s',
            'index': i,
            'name': generate_provider_name('k8s', global_index, used_names)
        })
        global_index += 1
    
    # 随机打乱
    random.shuffle(all_providers)
    
    # 非完全平均分配：每个 iarnet 节点分配 8-12 个 provider
    # 确保总和为 100
    allocation = []
    remaining = len(all_providers)
    
    for i in range(iarnet_count):
        if i == iarnet_count - 1:
            # 最后一个节点分配剩余的所有 provider
            count = remaining
        else:
            # 随机分配 8-12 个（但不超过剩余数量）
            min_count = max(8, remaining - (iarnet_count - i - 1) * 12)
            max_count = min(12, remaining - (iarnet_count - i - 1) * 8)
            count = random.randint(min_count, max_count)
        
        allocation.append(count)
        remaining -= count
    
    # 分配 provider 到各个 iarnet 节点
    iarnet_assignments = defaultdict(list)
    provider_idx = 0
    
    for iarnet_id in range(iarnet_count):
        for _ in range(allocation[iarnet_id]):
            if provider_idx < len(all_providers):
                iarnet_assignments[iarnet_id].append(all_providers[provider_idx])
                provider_idx += 1
    
    return iarnet_assignments, allocation

def main():
    parser = argparse.ArgumentParser(description='为 iarnet 节点生成 provider 批量导入 CSV 文件')
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--output-dir', '-o',
        default=str(SCRIPT_DIR / 'provider-import-csvs'),
        help='输出目录 (默认: deploy/provider-import-csvs)'
    )
    parser.add_argument(
        '--iarnet-count',
        type=int,
        default=10,
        help='iarnet 节点数量 (默认: 10)'
    )
    parser.add_argument(
        '--seed',
        type=int,
        help='随机种子（用于生成可重复的分配方案）'
    )
    
    args = parser.parse_args()
    
    # 读取虚拟机配置
    vm_config_path = Path(args.vm_config)
    if not vm_config_path.is_absolute():
        vm_config_path = SCRIPT_DIR / vm_config_path
    
    if not vm_config_path.exists():
        print(f"错误: 虚拟机配置文件不存在: {vm_config_path}")
        sys.exit(1)
    
    with open(vm_config_path, 'r', encoding='utf-8') as f:
        vm_config = yaml.safe_load(f)
    
    # 获取配置信息
    iarnet_config = vm_config['vm_types']['iarnet']
    docker_config = vm_config['vm_types']['docker']
    k8s_config = vm_config['vm_types']['k8s_clusters']
    
    docker_count = docker_config['count']
    k8s_count = k8s_config['count']
    iarnet_count = args.iarnet_count
    
    # 验证 iarnet 节点数量
    if iarnet_count > iarnet_config['count']:
        print(f"警告: 指定的 iarnet 节点数量 ({iarnet_count}) 超过配置中的数量 ({iarnet_config['count']})")
        print(f"将使用配置中的数量: {iarnet_config['count']}")
        iarnet_count = iarnet_config['count']
    
    print(f"生成 provider 分配方案...")
    print(f"  Docker providers: {docker_count}")
    print(f"  K8s providers: {k8s_count}")
    print(f"  Iarnet nodes: {iarnet_count}")
    print(f"  总计: {docker_count + k8s_count} 个 provider")
    
    # 生成随机分配方案
    iarnet_assignments, allocation = generate_random_allocation(
        docker_count, k8s_count, iarnet_count, args.seed
    )
    
    # 创建输出目录
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    
    print(f"\n分配方案:")
    for iarnet_id in range(iarnet_count):
        docker_providers = [p for p in iarnet_assignments[iarnet_id] if p['type'] == 'docker']
        k8s_providers = [p for p in iarnet_assignments[iarnet_id] if p['type'] == 'k8s']
        print(f"  Iarnet 节点 {iarnet_id:2d}: {allocation[iarnet_id]:2d} 个 provider "
              f"(Docker: {len(docker_providers)}, K8s: {len(k8s_providers)})")
    
    print(f"\n生成 CSV 文件...")
    print("=" * 60)
    
    # 为每个 iarnet 节点生成 CSV 文件
    for iarnet_id in range(iarnet_count):
        # 计算 iarnet 节点 IP
        ip_suffix = iarnet_config['ip_start'] + iarnet_id
        iarnet_ip = f"{iarnet_config['ip_base']}.{ip_suffix}"
        iarnet_hostname = f"{iarnet_config['hostname_prefix']}-{iarnet_id+1:02d}"
        
        # 生成 CSV 文件
        csv_file = output_dir / f"iarnet-{iarnet_id:02d}-providers.csv"
        
        with open(csv_file, 'w', encoding='utf-8', newline='') as f:
            writer = csv.writer(f)
            
            # CSV 格式：节点名称,地址:端口（首行就是数据，不是表头）
            for provider in iarnet_assignments[iarnet_id]:
                if provider['type'] == 'docker':
                    # Docker provider
                    provider_ip_suffix = docker_config['ip_start'] + provider['index']
                    provider_ip = f"{docker_config['ip_base']}.{provider_ip_suffix}"
                    provider_port = 50051  # Docker provider 默认端口
                else:  # k8s
                    # K8s provider
                    cluster_id = provider['index']
                    provider_ip_suffix = k8s_config['master']['ip_start'] + cluster_id * k8s_config['master']['ip_step']
                    provider_ip = f"{k8s_config['master']['ip_base']}.{provider_ip_suffix}"
                    provider_port = k8s_config['master']['provider_port_base'] + cluster_id
                
                # 写入 CSV 行：节点名称,地址:端口
                writer.writerow([provider['name'], f"{provider_ip}:{provider_port}"])
        
        docker_count_in_file = len([p for p in iarnet_assignments[iarnet_id] if p['type'] == 'docker'])
        k8s_count_in_file = len([p for p in iarnet_assignments[iarnet_id] if p['type'] == 'k8s'])
        
        print(f"✓ Iarnet 节点 {iarnet_id:2d} ({iarnet_hostname}): {csv_file}")
        print(f"   包含 {allocation[iarnet_id]} 个 provider "
              f"(Docker: {docker_count_in_file}, K8s: {k8s_count_in_file})")
    
    print("=" * 60)
    print(f"完成！共生成 {iarnet_count} 个 CSV 文件")
    print(f"\nCSV 文件位置: {output_dir}")
    print(f"\n使用方式:")
    print(f"  1. 登录到各个 iarnet 节点的前端界面")
    print(f"  2. 进入资源管理页面")
    print(f"  3. 使用批量导入功能，上传对应的 CSV 文件")
    print(f"\n例如，为 Iarnet 节点 0 导入:")
    print(f"  上传文件: {output_dir / 'iarnet-00-providers.csv'}")

if __name__ == '__main__':
    main()

