#!/usr/bin/env python3
"""
删除并重新创建单个虚拟机
"""

import sys
import argparse
from delete_vms import VMDeleter
from create_vms import VMBuilder

def get_node_id_from_hostname(hostname: str, vm_type: str) -> int:
    """从hostname提取节点ID"""
    if vm_type == 'iarnet':
        # vm-iarnet-06 -> 5 (0-based)
        if hostname.startswith('vm-iarnet-'):
            num = int(hostname.replace('vm-iarnet-', ''))
            return num - 1
    elif vm_type == 'docker':
        if hostname.startswith('vm-docker-'):
            num = int(hostname.replace('vm-docker-', ''))
            return num - 1
    elif vm_type == 'k8s':
        # vm-k8s-cluster-01-master -> cluster_id=1
        if 'master' in hostname:
            parts = hostname.split('-')
            cluster_id = int(parts[2])
            return cluster_id - 1
    return None

def recreate_vm(hostname: str, config_path: str = './vm-config.yaml', delete_disk: bool = True):
    """删除并重新创建虚拟机"""
    print("=" * 60)
    print(f"删除并重新创建虚拟机: {hostname}")
    print("=" * 60)
    
    # 确定虚拟机类型
    vm_type = None
    if hostname.startswith('vm-iarnet-'):
        vm_type = 'iarnet'
    elif hostname.startswith('vm-docker-'):
        vm_type = 'docker'
    elif hostname.startswith('vm-k8s-cluster-'):
        vm_type = 'k8s'
    else:
        print(f"错误: 无法识别虚拟机类型: {hostname}")
        return False
    
    # 步骤1: 删除虚拟机
    print(f"\n步骤1: 删除虚拟机 {hostname}...")
    try:
        deleter = VMDeleter(config_path)
        if deleter.delete_vm(hostname, delete_disk=delete_disk):
            print("  ✓ 删除成功")
        else:
            print("  ⚠ 删除失败或虚拟机不存在")
    except Exception as e:
        error_msg = str(e)
        if 'Permission denied' in error_msg or 'Failed to connect' in error_msg:
            print(f"  ✗ 权限错误: {error_msg}")
            print(f"\n请使用 sudo 运行:")
            print(f"  sudo python3 {__file__} {hostname}")
            return False
        print(f"  ✗ 删除出错: {e}")
        return False
    
    # 步骤2: 重新创建虚拟机
    print(f"\n步骤2: 重新创建虚拟机 {hostname}...")
    try:
        builder = VMBuilder(config_path)
        
        if vm_type == 'iarnet':
            # 获取节点ID
            node_id = get_node_id_from_hostname(hostname, 'iarnet')
            if node_id is None:
                print(f"  错误: 无法从hostname提取节点ID")
                return False
            
            # 获取节点配置
            iarnet_config = builder.vm_types['iarnet']
            ip_suffix = iarnet_config['ip_start'] + node_id
            ip_address = f"{iarnet_config['ip_base']}.{ip_suffix}"
            
            # 创建虚拟机
            result = builder.create_vm(
                hostname=hostname,
                cpu=iarnet_config['cpu'],
                memory_mb=iarnet_config['memory'],
                disk_size_gb=iarnet_config['disk'],
                ip_address=ip_address
            )
            
        elif vm_type == 'docker':
            node_id = get_node_id_from_hostname(hostname, 'docker')
            if node_id is None:
                print(f"  错误: 无法从hostname提取节点ID")
                return False
            
            docker_config = builder.vm_types['docker']
            ip_suffix = docker_config['ip_start'] + node_id
            ip_address = f"{docker_config['ip_base']}.{ip_suffix}"
            
            result = builder.create_vm(
                hostname=hostname,
                cpu=docker_config['cpu'],
                memory_mb=docker_config['memory'],
                disk_size_gb=docker_config['disk'],
                ip_address=ip_address
            )
            
        elif vm_type == 'k8s':
            # K8s节点需要特殊处理
            print("  错误: K8s节点需要指定集群ID，请使用 create_vms.py --type k8s")
            return False
        
        if result:
            print("  ✓ 创建成功")
            print("\n" + "=" * 60)
            print("完成！")
            print("=" * 60)
            print(f"\n验证虚拟机:")
            print(f"  virsh dominfo {hostname}")
            print(f"  virsh start {hostname}")
            print(f"\n等待几分钟后检查网络:")
            print(f"  python3 deploy/ssh_vm.py {hostname} \"ip addr show\"")
            return True
        else:
            print("  ✗ 创建失败")
            return False
            
    except Exception as e:
        print(f"  ✗ 创建出错: {e}")
        import traceback
        traceback.print_exc()
        return False

def main():
    parser = argparse.ArgumentParser(description='删除并重新创建单个虚拟机')
    parser.add_argument(
        'hostname',
        help='虚拟机hostname (例如: vm-iarnet-06)'
    )
    parser.add_argument(
        '--config', '-c',
        default='./vm-config.yaml',
        help='配置文件路径 (默认: ./vm-config.yaml)'
    )
    parser.add_argument(
        '--no-delete-disk', '-n',
        action='store_true',
        help='不删除磁盘镜像（保留磁盘）'
    )
    
    args = parser.parse_args()
    
    delete_disk = not args.no_delete_disk
    
    success = recreate_vm(args.hostname, args.config, delete_disk)
    sys.exit(0 if success else 1)

if __name__ == '__main__':
    main()

