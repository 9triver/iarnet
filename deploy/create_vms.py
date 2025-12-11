#!/usr/bin/env python3
"""
批量创建虚拟机脚本
使用 libvirt + qemu 根据 vm-config.yaml 配置批量创建虚拟机
"""

import os
import sys
import yaml
import shutil
import subprocess
import libvirt
import xml.etree.ElementTree as ET
from pathlib import Path
from typing import Dict, List, Any
import argparse
import time
from concurrent.futures import ThreadPoolExecutor, as_completed

class VMBuilder:
    def __init__(self, config_path: str):
        """初始化VM构建器"""
        with open(config_path, 'r', encoding='utf-8') as f:
            self.config = yaml.safe_load(f)
        
        self.global_config = self.config['global']
        self.vm_types = self.config['vm_types']
        self.k8s_pod_cidrs = self.config.get('k8s_pod_cidrs', [])
        
        # 展开路径
        self.base_image = os.path.expanduser(self.global_config['base_image'])
        
        # 读取密码配置（可选）
        self.password = self.global_config.get('password')
        self.password_hash = self.global_config.get('password_hash')
        
        # 处理SSH密钥路径：如果使用sudo运行，尝试使用原始用户的SSH密钥
        ssh_key_config = self.global_config['ssh_key_path']
        self.ssh_key_path = os.path.expanduser(ssh_key_config)
        
        # 如果使用sudo运行且SSH密钥不存在，尝试使用SUDO_USER的密钥
        if not os.path.exists(self.ssh_key_path) and os.environ.get('SUDO_USER'):
            original_user = os.environ['SUDO_USER']
            # 替换~为原始用户的home目录
            if ssh_key_config.startswith('~'):
                original_home = os.path.expanduser(f'~{original_user}')
                self.ssh_key_path = ssh_key_config.replace('~', original_home, 1)
            else:
                # 如果路径包含当前用户home，替换为原始用户home
                current_home = os.path.expanduser('~')
                if self.ssh_key_path.startswith(current_home):
                    original_home = os.path.expanduser(f'~{original_user}')
                    self.ssh_key_path = self.ssh_key_path.replace(current_home, original_home, 1)
        
        # 如果还是不存在，尝试常见的SSH密钥位置
        if not os.path.exists(self.ssh_key_path):
            # 尝试当前用户（如果使用sudo，尝试SUDO_USER）
            test_user = os.environ.get('SUDO_USER') or os.environ.get('USER', '')
            if test_user:
                test_paths = [
                    os.path.expanduser(f'~{test_user}/.ssh/id_rsa.pub'),
                    os.path.expanduser(f'~{test_user}/.ssh/id_ed25519.pub'),
                    os.path.expanduser('~/.ssh/id_rsa.pub'),
                    os.path.expanduser('~/.ssh/id_ed25519.pub'),
                ]
                for test_path in test_paths:
                    if os.path.exists(test_path):
                        self.ssh_key_path = test_path
                        print(f"使用找到的SSH密钥: {self.ssh_key_path}")
                        break
        
        self.network_name = self.global_config['network_name']
        self.user = self.global_config['user']
        
        # 验证基础镜像是否存在
        if not os.path.exists(self.base_image):
            raise FileNotFoundError(f"基础镜像不存在: {self.base_image}")
        
        # 验证SSH密钥是否存在
        if not os.path.exists(self.ssh_key_path):
            raise FileNotFoundError(
                f"SSH公钥不存在: {self.ssh_key_path}\n"
                f"请确保SSH密钥存在，或修改配置文件中的ssh_key_path设置。\n"
                f"如果使用sudo运行，请确保原始用户的SSH密钥可访问。"
            )
        
        # 连接libvirt
        try:
            self.conn = libvirt.open('qemu:///system')
            if self.conn is None:
                raise Exception("无法连接到libvirt守护进程")
        except Exception as e:
            raise Exception(f"连接libvirt失败: {e}")
    
    def __del__(self):
        """清理libvirt连接"""
        if hasattr(self, 'conn') and self.conn:
            self.conn.close()
    
    def read_ssh_key(self) -> str:
        """读取SSH公钥"""
        with open(self.ssh_key_path, 'r') as f:
            return f.read().strip()
    
    def create_cloud_init_iso(self, hostname: str, ip_address: str, 
                               ssh_key: str, user_data: str = None,
                               gateway: str = None, dns: List[str] = None) -> str:
        """创建cloud-init ISO镜像"""
        iso_dir = Path(f"/tmp/cloud-init-{hostname}")
        iso_dir.mkdir(exist_ok=True)
        
        # 创建meta-data
        meta_data = f"""instance-id: {hostname}
local-hostname: {hostname}
"""
        
        # 解析IP地址和子网
        ip_parts = ip_address.split('.')
        ip_base = '.'.join(ip_parts[:3])
        
        # 默认网关和DNS
        if gateway is None:
            gateway = f"{ip_base}.1"
        if dns is None:
            dns = ["8.8.8.8", "8.8.4.4"]
        
        # 创建user-data
        if user_data is None:
            # 构建用户配置
            user_config_lines = [
                f"  - name: {self.user}",
                "    sudo: ALL=(ALL) NOPASSWD:ALL",
                "    shell: /bin/bash",
                f"    ssh_authorized_keys:",
                f"      - {ssh_key}"
            ]
            
            # 添加密码配置（如果配置了）
            if self.password_hash:
                user_config_lines.append(f"    passwd: {self.password_hash}")
            elif self.password:
                user_config_lines.append(f"    plain_text_passwd: {self.password}")
                user_config_lines.append("    lock_passwd: false")
            
            user_config = "\n".join(user_config_lines)
            
            user_data = f"""#cloud-config
hostname: {hostname}
users:
{user_config}
manage_etc_hosts: true
# 确保网络配置应用
runcmd:
  - netplan apply || true
"""
        
        # 创建network-config（独立的网络配置文件）
        # 使用match匹配所有以太网接口，兼容eth0、ens3等不同命名
        # Ubuntu 22.04 cloud image默认使用ens3接口，但我们也支持其他命名
        network_config = f"""version: 2
ethernets:
  eth0:
    match:
      name: en*
    dhcp4: false
    addresses:
      - {ip_address}/24
    routes:
      - to: default
        via: {gateway}
    nameservers:
      addresses: {dns}
"""
        
        # 写入文件
        meta_data_path = iso_dir / "meta-data"
        user_data_path = iso_dir / "user-data"
        network_config_path = iso_dir / "network-config"
        
        meta_data_path.write_text(meta_data)
        user_data_path.write_text(user_data)
        network_config_path.write_text(network_config)
        
        # 验证文件已创建
        if not meta_data_path.exists() or not user_data_path.exists() or not network_config_path.exists():
            raise Exception("cloud-init配置文件创建失败")
        
        # 创建ISO镜像
        iso_path = f"/tmp/{hostname}-cloud-init.iso"
        
        # 如果ISO已存在，先删除
        if os.path.exists(iso_path):
            os.remove(iso_path)
        
        # 使用绝对路径执行genisoimage
        cmd = [
            'genisoimage', '-output', iso_path,
            '-volid', 'cidata', '-joliet', '-rock',
            str(user_data_path.absolute()),
            str(meta_data_path.absolute()),
            str(network_config_path.absolute())
        ]
        
        try:
            result = subprocess.run(
                cmd, 
                check=True, 
                capture_output=True, 
                text=True,
                cwd=str(iso_dir)
            )
        except subprocess.CalledProcessError as e:
            error_msg = e.stderr if e.stderr else str(e)
            raise Exception(f"创建cloud-init ISO失败: {error_msg}\n命令: {' '.join(cmd)}")
        
        # 验证ISO文件已创建
        if not os.path.exists(iso_path):
            raise Exception(f"ISO文件创建失败: {iso_path}")
        
        # 清理临时目录
        shutil.rmtree(iso_dir)
        
        return iso_path
    
    def create_disk_image(self, hostname: str, disk_size_gb: int) -> str:
        """从基础镜像创建虚拟机磁盘"""
        disk_path = f"/var/lib/libvirt/images/{hostname}.qcow2"
        
        # 如果磁盘已存在，跳过创建
        if os.path.exists(disk_path):
            print(f"  磁盘已存在: {disk_path}")
            return disk_path
        
        # 检查目录权限，如果没有写入权限则使用sudo
        disk_dir = os.path.dirname(disk_path)
        use_sudo = not os.access(disk_dir, os.W_OK)
        
        # 使用qemu-img创建磁盘（从基础镜像复制）
        cmd = [
            'qemu-img', 'create', '-f', 'qcow2',
            '-b', self.base_image,
            '-F', 'qcow2',
            disk_path,
            f'{disk_size_gb}G'
        ]
        
        # 如果没有写入权限，使用sudo
        if use_sudo:
            cmd = ['sudo'] + cmd
            print(f"  注意: 需要sudo权限创建磁盘镜像")
        
        try:
            result = subprocess.run(cmd, check=True, capture_output=True, text=True)
            print(f"  创建磁盘: {disk_path} ({disk_size_gb}GB)")
        except subprocess.CalledProcessError as e:
            error_msg = e.stderr if isinstance(e.stderr, str) else (e.stderr.decode('utf-8') if e.stderr else str(e))
            if use_sudo:
                # 检查是否是sudo密码提示
                if 'password' in error_msg.lower() or 'sudo' in error_msg.lower():
                    raise Exception(
                        f"创建磁盘失败: 需要sudo权限。\n"
                        f"解决方案:\n"
                        f"  1. 使用sudo运行脚本: sudo python3 create_vms.py\n"
                        f"  2. 或者配置免密sudo (推荐):\n"
                        f"     sudo visudo\n"
                        f"     添加: {os.environ.get('USER', 'your_username')} ALL=(ALL) NOPASSWD: /usr/bin/qemu-img\n"
                        f"  3. 或者修改目录权限:\n"
                        f"     sudo chmod 775 /var/lib/libvirt/images\n"
                        f"     sudo chgrp libvirt /var/lib/libvirt/images"
                    )
            raise Exception(f"创建磁盘失败: {error_msg}")
        
        return disk_path
    
    def create_network_config(self, ip_address: str) -> str:
        """创建网络配置XML（用于静态IP）"""
        # 注意：cloud-init通常通过user-data配置网络
        # 这里返回一个简单的网络接口配置
        return f"""
<interface type='network'>
  <source network='{self.network_name}'/>
  <model type='virtio'/>
</interface>
"""
    
    def generate_vm_xml(self, hostname: str, cpu: int, memory_mb: int,
                       disk_path: str, cloud_init_iso: str, 
                       ip_address: str) -> str:
        """生成虚拟机XML配置"""
        # 内存转换为KB
        memory_kb = memory_mb * 1024
        
        xml = f"""<domain type='kvm'>
  <name>{hostname}</name>
  <memory unit='KiB'>{memory_kb}</memory>
  <currentMemory unit='KiB'>{memory_kb}</currentMemory>
  <vcpu placement='static'>{cpu}</vcpu>
  <os>
    <type arch='x86_64' machine='pc-q35-8.0'>hvm</type>
    <boot dev='hd'/>
  </os>
  <features>
    <acpi/>
    <apic/>
  </features>
  <cpu mode='host-passthrough' check='none'/>
  <clock offset='utc'/>
  <on_poweroff>destroy</on_poweroff>
  <on_reboot>restart</on_reboot>
  <on_crash>destroy</on_crash>
  <devices>
    <emulator>/usr/bin/qemu-system-x86_64</emulator>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='{disk_path}'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <disk type='file' device='cdrom'>
      <driver name='qemu' type='raw'/>
      <source file='{cloud_init_iso}'/>
      <target dev='sda' bus='sata'/>
      <readonly/>
    </disk>
    {self.create_network_config(ip_address)}
    <console type='pty'>
      <target type='serial' port='0'/>
    </console>
    <graphics type='vnc' port='-1' autoport='yes' listen='0.0.0.0'/>
  </devices>
</domain>"""
        return xml
    
    def create_vm(self, hostname: str, cpu: int, memory_mb: int,
                  disk_size_gb: int, ip_address: str, 
                  user_data: str = None) -> bool:
        """创建单个虚拟机"""
        try:
            # 验证IP地址格式
            ip_parts = ip_address.split('.')
            if len(ip_parts) != 4:
                raise ValueError(f"无效的IP地址格式: {ip_address}")
            
            ip_suffix = int(ip_parts[3])
            if ip_suffix < 1 or ip_suffix > 254:
                raise ValueError(f"IP地址后缀超出范围: {ip_suffix} (有效范围: 1-254)")
            
            # 检查虚拟机是否已存在
            try:
                existing_dom = self.conn.lookupByName(hostname)
                print(f"  虚拟机已存在: {hostname}，跳过创建")
                return True
            except libvirt.libvirtError:
                pass  # 虚拟机不存在，继续创建
            
            print(f"创建虚拟机: {hostname} (IP: {ip_address})")
            
            # 创建磁盘
            disk_path = self.create_disk_image(hostname, disk_size_gb)
            
            # 读取SSH密钥
            ssh_key = self.read_ssh_key()
            
            # 创建cloud-init ISO
            cloud_init_iso = self.create_cloud_init_iso(
                hostname, ip_address, ssh_key, user_data
            )
            
            # 生成XML配置
            xml = self.generate_vm_xml(
                hostname, cpu, memory_mb, disk_path, 
                cloud_init_iso, ip_address
            )
            
            # 创建虚拟机
            dom = self.conn.defineXML(xml)
            dom.create()
            
            print(f"  ✓ 虚拟机创建成功: {hostname}")
            return True
            
        except Exception as e:
            print(f"  ✗ 创建虚拟机失败 {hostname}: {e}")
            import traceback
            if '--debug' in sys.argv or '-d' in sys.argv:
                traceback.print_exc()
            return False
    
    def create_iarnet_vms(self) -> List[bool]:
        """创建iarnet节点虚拟机"""
        iarnet_config = self.vm_types['iarnet']
        results = []
        
        print(f"\n创建 {iarnet_config['count']} 个 iarnet 节点...")
        
        for i in range(iarnet_config['count']):
            ip_suffix = iarnet_config['ip_start'] + i
            ip_address = f"{iarnet_config['ip_base']}.{ip_suffix}"
            hostname = f"{iarnet_config['hostname_prefix']}-{i+1:02d}"
            
            result = self.create_vm(
                hostname=hostname,
                cpu=iarnet_config['cpu'],
                memory_mb=iarnet_config['memory'],
                disk_size_gb=iarnet_config['disk'],
                ip_address=ip_address
            )
            results.append(result)
        
        return results
    
    def create_docker_vms(self) -> List[bool]:
        """创建Docker节点虚拟机"""
        docker_config = self.vm_types['docker']
        results = []
        
        print(f"\n创建 {docker_config['count']} 个 Docker 节点...")
        
        for i in range(docker_config['count']):
            ip_suffix = docker_config['ip_start'] + i
            ip_address = f"{docker_config['ip_base']}.{ip_suffix}"
            hostname = f"{docker_config['hostname_prefix']}-{i+1:02d}"
            
            result = self.create_vm(
                hostname=hostname,
                cpu=docker_config['cpu'],
                memory_mb=docker_config['memory'],
                disk_size_gb=docker_config['disk'],
                ip_address=ip_address
            )
            results.append(result)
        
        return results
    
    def validate_ip_suffix(self, ip_suffix: int, hostname: str) -> bool:
        """验证IP地址后缀是否在有效范围内（1-254）"""
        if ip_suffix < 1 or ip_suffix > 254:
            print(f"  ✗ IP地址超出范围: {ip_suffix} (有效范围: 1-254)")
            print(f"    虚拟机: {hostname}")
            return False
        return True
    
    def create_k8s_cluster_vms(self) -> List[bool]:
        """创建K8s集群虚拟机"""
        k8s_config = self.vm_types['k8s_clusters']
        master_config = k8s_config['master']
        worker_config = k8s_config['worker']
        results = []
        
        cluster_count = k8s_config['count']
        print(f"\n创建 {cluster_count} 个 K8s 集群（每个集群1个master + {worker_config['count_per_cluster']}个worker）...")
        
        # 预先验证IP地址范围
        max_master_ip = master_config['ip_start'] + (cluster_count - 1) * master_config['ip_step']
        max_worker_ip = max_master_ip + worker_config['count_per_cluster']
        
        if max_master_ip > 254 or max_worker_ip > 254:
            print(f"\n错误: IP地址配置超出范围！")
            print(f"  需要 {cluster_count} 个集群，每个集群 {master_config['ip_step']} 个IP")
            print(f"  从 {master_config['ip_start']} 开始，最后一个IP将是 {max_worker_ip}")
            print(f"  但IP地址后缀必须在 1-254 范围内")
            print(f"\n建议:")
            print(f"  1. 减少集群数量")
            print(f"  2. 或者调整 ip_start 和 ip_step 配置")
            print(f"  3. 或者使用不同的IP段（如192.168.101.x）")
            return []
        
        for cluster_id in range(1, cluster_count + 1):
            print(f"\n创建集群 {cluster_id}/{cluster_count}:")
            
            # 计算master IP
            master_ip_suffix = master_config['ip_start'] + (cluster_id - 1) * master_config['ip_step']
            master_ip = f"{master_config['ip_base']}.{master_ip_suffix}"
            master_hostname = f"{master_config['hostname_prefix']}-{cluster_id:02d}{master_config['hostname_suffix']}"
            
            # 验证master IP
            if not self.validate_ip_suffix(master_ip_suffix, master_hostname):
                results.append(False)
                continue
            
            # 创建master节点
            print(f"  创建master节点: {master_hostname} (IP: {master_ip})")
            result = self.create_vm(
                hostname=master_hostname,
                cpu=master_config['cpu'],
                memory_mb=master_config['memory'],
                disk_size_gb=master_config['disk'],
                ip_address=master_ip
            )
            results.append(result)
            
            # 创建worker节点
            for worker_id in range(1, worker_config['count_per_cluster'] + 1):
                worker_ip_suffix = master_ip_suffix + worker_id
                worker_ip = f"{master_config['ip_base']}.{worker_ip_suffix}"
                worker_hostname = f"{worker_config['hostname_prefix']}-{cluster_id:02d}{worker_config['hostname_suffix']}-{worker_id}"
                
                # 验证worker IP
                if not self.validate_ip_suffix(worker_ip_suffix, worker_hostname):
                    results.append(False)
                    continue
                
                print(f"  创建worker节点: {worker_hostname} (IP: {worker_ip})")
                result = self.create_vm(
                    hostname=worker_hostname,
                    cpu=worker_config['cpu'],
                    memory_mb=worker_config['memory'],
                    disk_size_gb=worker_config['disk'],
                    ip_address=worker_ip
                )
                results.append(result)
        
        return results
    
    def create_all_vms(self, parallel: bool = False, max_workers: int = 5):
        """创建所有虚拟机"""
        print("=" * 60)
        print("开始批量创建虚拟机")
        print("=" * 60)
        
        all_results = []
        
        # 创建iarnet节点
        iarnet_results = self.create_iarnet_vms()
        all_results.extend(iarnet_results)
        
        # 创建Docker节点
        docker_results = self.create_docker_vms()
        all_results.extend(docker_results)
        
        # 创建K8s集群
        k8s_results = self.create_k8s_cluster_vms()
        all_results.extend(k8s_results)
        
        # 统计结果
        total = len(all_results)
        success = sum(all_results)
        failed = total - success
        
        print("\n" + "=" * 60)
        print("创建完成统计:")
        print(f"  总计: {total} 台虚拟机")
        print(f"  成功: {success} 台")
        print(f"  失败: {failed} 台")
        print("=" * 60)
        
        return success == total


def main():
    parser = argparse.ArgumentParser(description='批量创建虚拟机')
    parser.add_argument(
        '--config', '-c',
        default='./vm-config.yaml',
        help='配置文件路径 (默认: ./vm-config.yaml)'
    )
    parser.add_argument(
        '--type', '-t',
        choices=['all', 'iarnet', 'docker', 'k8s'],
        default='all',
        help='创建类型 (默认: all)'
    )
    parser.add_argument(
        '--parallel', '-p',
        action='store_true',
        help='并行创建（暂未实现）'
    )
    
    args = parser.parse_args()
    
    try:
        builder = VMBuilder(args.config)
        
        if args.type == 'all':
            builder.create_all_vms(parallel=args.parallel)
        elif args.type == 'iarnet':
            builder.create_iarnet_vms()
        elif args.type == 'docker':
            builder.create_docker_vms()
        elif args.type == 'k8s':
            builder.create_k8s_cluster_vms()
        
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()

