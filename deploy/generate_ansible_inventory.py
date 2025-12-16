#!/usr/bin/env python3
"""
从 vm-config.yaml 生成 Ansible inventory 文件
"""

import yaml
import argparse
import sys
from pathlib import Path


def generate_inventory(config_path: str, output_path: str, vm_type: str = None):
    """从配置文件生成 Ansible inventory"""
    
    # 读取配置文件
    with open(config_path, 'r', encoding='utf-8') as f:
        config = yaml.safe_load(f)
    
    global_config = config['global']
    vm_types = config['vm_types']
    user = global_config['user']
    
    # 生成 inventory 内容
    inventory_lines = []
    
    # 添加全局变量
    inventory_lines.append("[all:vars]")
    inventory_lines.append(f"ansible_user={user}")
    inventory_lines.append("ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'")
    inventory_lines.append("ansible_python_interpreter=/usr/bin/python3")
    inventory_lines.append("")
    
    # 生成 docker 节点组
    if not vm_type or vm_type == 'docker':
        docker_config = vm_types['docker']
        inventory_lines.append("[docker]")
        for i in range(docker_config['count']):
            ip_suffix = docker_config['ip_start'] + i
            ip_address = f"{docker_config['ip_base']}.{ip_suffix}"
            hostname = f"{docker_config['hostname_prefix']}-{i+1:02d}"
            inventory_lines.append(f"{hostname} ansible_host={ip_address}")
        inventory_lines.append("")
    
    # 生成 iarnet 节点组
    if not vm_type or vm_type == 'iarnet':
        iarnet_config = vm_types['iarnet']
        inventory_lines.append("[iarnet_nodes]")
        for i in range(iarnet_config['count']):
            ip_suffix = iarnet_config['ip_start'] + i
            ip_address = f"{iarnet_config['ip_base']}.{ip_suffix}"
            hostname = f"{iarnet_config['hostname_prefix']}-{i+1:02d}"
            inventory_lines.append(f"{hostname} ansible_host={ip_address}")
        inventory_lines.append("")
        
        # 同时添加 iarnet 别名组（向后兼容）
        inventory_lines.append("[iarnet:children]")
        inventory_lines.append("iarnet_nodes")
        inventory_lines.append("")
    
    # 生成 k8s 节点组
    if not vm_type or vm_type == 'k8s':
        k8s_config = vm_types['k8s_clusters']
        master_config = k8s_config['master']
        worker_config = k8s_config['worker']
        
        # K8s master 节点
        inventory_lines.append("[k8s_master]")
        for cluster_id in range(1, k8s_config['count'] + 1):
            master_ip_suffix = master_config['ip_start'] + (cluster_id - 1) * master_config['ip_step']
            master_ip = f"{master_config['ip_base']}.{master_ip_suffix}"
            master_hostname = f"{master_config['hostname_prefix']}-{cluster_id:02d}{master_config['hostname_suffix']}"
            inventory_lines.append(f"{master_hostname} ansible_host={master_ip} cluster_id={cluster_id}")
        inventory_lines.append("")
        
        # K8s worker 节点
        inventory_lines.append("[k8s_worker]")
        for cluster_id in range(1, k8s_config['count'] + 1):
            master_ip_suffix = master_config['ip_start'] + (cluster_id - 1) * master_config['ip_step']
            for worker_id in range(1, worker_config['count_per_cluster'] + 1):
                worker_ip_suffix = master_ip_suffix + worker_id
                worker_ip = f"{master_config['ip_base']}.{worker_ip_suffix}"
                worker_hostname = f"{worker_config['hostname_prefix']}-{cluster_id:02d}{worker_config['hostname_suffix']}-{worker_id}"
                inventory_lines.append(f"{worker_hostname} ansible_host={worker_ip} cluster_id={cluster_id} worker_id={worker_id}")
        inventory_lines.append("")
        
        # K8s 集群组（包含 master 和 worker）
        inventory_lines.append("[k8s_cluster:children]")
        inventory_lines.append("k8s_master")
        inventory_lines.append("k8s_worker")
        inventory_lines.append("")
    
    # 写入文件
    output_file = Path(output_path)
    output_file.parent.mkdir(parents=True, exist_ok=True)
    
    with open(output_file, 'w', encoding='utf-8') as f:
        f.write('\n'.join(inventory_lines))
    
    print(f"✓ Ansible inventory 已生成: {output_path}")
    print(f"  包含 {len([l for l in inventory_lines if l and not l.startswith('[')])} 台虚拟机")
    
    return output_path


def main():
    parser = argparse.ArgumentParser(
        description='从 vm-config.yaml 生成 Ansible inventory 文件',
        formatter_class=argparse.RawDescriptionHelpFormatter
    )
    
    parser.add_argument(
        '--config', '-c',
        default='./vm-config.yaml',
        help='配置文件路径 (默认: ./vm-config.yaml)'
    )
    
    parser.add_argument(
        '--output', '-o',
        default='./ansible/inventory.ini',
        help='输出 inventory 文件路径 (默认: ./ansible/inventory.ini)'
    )
    
    parser.add_argument(
        '--type', '-t',
        choices=['docker', 'iarnet', 'k8s'],
        help='只生成特定类型的节点组'
    )
    
    args = parser.parse_args()
    
    try:
        generate_inventory(args.config, args.output, args.type)
    except FileNotFoundError as e:
        print(f"错误: 文件不存在: {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()

