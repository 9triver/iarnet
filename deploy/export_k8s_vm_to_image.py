#!/usr/bin/env python3
"""
将运行中的 K8s 虚拟机导出为镜像
专门用于导出已安装 K8s 依赖的虚拟机，但未初始化集群
注意：此镜像不包含 Docker，仅用于创建 K8s 节点
"""

import os
import sys
import subprocess
import argparse
import shutil
import yaml
from pathlib import Path

# 获取脚本所在目录
SCRIPT_DIR = Path(__file__).parent.absolute()

class K8sVMExporter:
    def __init__(self, vm_name: str, output_image: str = None, vm_config_path: str = None):
        """初始化 K8s 虚拟机导出器"""
        self.vm_name = vm_name
        self.output_image = output_image or self._generate_output_name()
        
        # 从配置文件获取虚拟机信息
        if vm_config_path:
            config_path = Path(vm_config_path)
            if not config_path.is_absolute():
                config_path = SCRIPT_DIR / config_path
        else:
            config_path = SCRIPT_DIR / 'vm-config.yaml'
        
        if config_path.exists():
            with open(config_path, 'r', encoding='utf-8') as f:
                self.config = yaml.safe_load(f)
            self.vm_user = self.config['global'].get('user', 'ubuntu')
        else:
            self.vm_user = 'ubuntu'
        
        # 检查 virsh 是否可用
        try:
            subprocess.run(['virsh', '--version'], 
                         capture_output=True, check=True)
        except (FileNotFoundError, subprocess.CalledProcessError):
            raise RuntimeError(
                "未找到 virsh 命令。\n"
                "请安装 libvirt:\n"
                "  Ubuntu/Debian: sudo apt-get install libvirt-clients\n"
                "  CentOS/RHEL: sudo yum install libvirt-client"
            )
        
        # 检查是否以 root 权限运行
        if os.geteuid() != 0:
            raise RuntimeError(
                "此脚本需要 root 权限。\n"
                "请使用 sudo 运行:\n"
                "  sudo python3 export_k8s_vm_to_image.py --vm-name <vm-name>"
            )
    
    def _generate_output_name(self) -> str:
        """生成输出镜像名称"""
        default_dir = '/var/lib/libvirt/images'
        if os.path.exists(default_dir) and os.access(default_dir, os.W_OK):
            output_name = "ubuntu-22.04-cloud-k8s-v1.qcow2"
            return os.path.join(default_dir, output_name)
        else:
            return "ubuntu-22.04-cloud-k8s-v1.qcow2"
    
    def check_vm_exists(self) -> bool:
        """检查虚拟机是否存在"""
        try:
            subprocess.run(
                ['virsh', 'dominfo', self.vm_name],
                capture_output=True,
                check=True
            )
            return True
        except subprocess.CalledProcessError:
            return False
    
    def get_vm_ip(self) -> str:
        """从配置文件获取虚拟机 IP"""
        try:
            k8s_config = self.config['vm_types']['k8s_clusters']
            master_config = k8s_config['master']
            
            # 从虚拟机名称提取集群编号
            # 例如: vm-k8s-cluster-01-master -> 集群 1
            if 'vm-k8s-cluster-' in self.vm_name and '-master' in self.vm_name:
                cluster_num_str = self.vm_name.replace('vm-k8s-cluster-', '').replace('-master', '')
                try:
                    cluster_id = int(cluster_num_str) - 1  # 转换为 0-based
                    ip_suffix = master_config['ip_start'] + cluster_id * master_config['ip_step']
                    ip_address = f"{master_config['ip_base']}.{ip_suffix}"
                    return ip_address
                except ValueError:
                    pass
        except:
            pass
        return None
    
    def get_vm_disk_path(self) -> str:
        """获取虚拟机的磁盘路径"""
        try:
            result = subprocess.run(
                ['virsh', 'domblklist', self.vm_name],
                capture_output=True,
                text=True,
                check=True
            )
            for line in result.stdout.split('\n'):
                if line.strip() and not line.startswith('Target') and not line.startswith('---'):
                    parts = line.split()
                    if len(parts) >= 2:
                        disk_path = parts[1]
                        if disk_path.startswith('/'):
                            return disk_path
            return None
        except subprocess.CalledProcessError:
            return None
    
    def cleanup_k8s_config(self, ssh_cmd: list) -> bool:
        """清理 K8s 集群配置（避免新虚拟机自动加入集群）"""
        print("  清理 K8s 集群配置...")
        
        cleanup_script = '''#!/bin/bash
set -e

echo "清理 K8s 集群配置..."

# 1. 停止 kubelet（如果正在运行）
systemctl stop kubelet 2>/dev/null || true

# 2. 清理 .kube 目录和配置文件
rm -rf ~/.kube 2>/dev/null || true
rm -rf /root/.kube 2>/dev/null || true

# 3. 清理 /etc/kubernetes/ 目录（集群初始化后的配置）
rm -rf /etc/kubernetes/* 2>/dev/null || true

# 4. 清理 kubeadm 的配置和状态
rm -rf /var/lib/kubelet/* 2>/dev/null || true
# 保留 /var/lib/kubelet/config.yaml（这是依赖安装时创建的）

# 5. 清理 kubeadm 的 token 和证书
rm -rf /etc/kubernetes/pki 2>/dev/null || true
rm -rf /var/lib/etcd 2>/dev/null || true

# 6. 清理 containerd 中的 K8s 相关容器和镜像
crictl ps -aq 2>/dev/null | xargs -r crictl rm 2>/dev/null || true
crictl images -q 2>/dev/null | xargs -r crictl rmi 2>/dev/null || true

# 7. 清理 K8s provider 相关（如果存在）
pkill -f k8s-provider 2>/dev/null || true
rm -rf ~/k8s-provider 2>/dev/null || true

# 8. 清理日志
journalctl --vacuum-time=1d 2>/dev/null || true
rm -rf /var/log/pods 2>/dev/null || true
rm -rf /var/log/containers 2>/dev/null || true

# 9. 清理临时文件
apt-get clean 2>/dev/null || true
rm -rf /var/lib/apt/lists/* 2>/dev/null || true
rm -rf /tmp/* /var/tmp/* 2>/dev/null || true

# 10. 清理命令历史
> ~/.bash_history 2>/dev/null || true
> /root/.bash_history 2>/dev/null || true

# 11. 清理 cloud-init 数据
cloud-init clean 2>/dev/null || true
rm -rf /var/lib/cloud/instances/* 2>/dev/null || true

echo "  ✓ K8s 配置清理完成"
echo "  注意: K8s 依赖组件（kubelet, kubeadm, kubectl, containerd）已保留"
'''
        
        try:
            # 将脚本通过 SSH 执行
            full_cmd = ' '.join(ssh_cmd) + ' "bash -s"'
            result = subprocess.run(
                full_cmd,
                input=cleanup_script,
                shell=True,
                check=True,
                timeout=300,
                capture_output=True,
                text=True
            )
            print("  ✓ 清理完成")
            return True
        except subprocess.CalledProcessError as e:
            print(f"  ⚠ 清理过程有警告: {e.stderr if e.stderr else '未知错误'}")
            return True  # 清理失败不影响导出
        except subprocess.TimeoutExpired:
            print("  ⚠ 清理超时，但继续导出")
            return True
    
    def export_vm(self, cleanup: bool = True, vm_ip: str = None) -> bool:
        """导出虚拟机为镜像"""
        print(f"导出 K8s 虚拟机为镜像...")
        print(f"  虚拟机名称: {self.vm_name}")
        print(f"  输出镜像: {self.output_image}")
        print("=" * 60)
        
        # 检查虚拟机是否存在
        if not self.check_vm_exists():
            print(f"  ✗ 虚拟机不存在: {self.vm_name}")
            return False
        
        # 获取虚拟机 IP（如果未提供）
        if not vm_ip:
            vm_ip = self.get_vm_ip()
            if vm_ip:
                print(f"  从配置文件获取 IP: {vm_ip}")
        
        # 获取虚拟机状态
        try:
            result = subprocess.run(
                ['virsh', 'domstate', self.vm_name],
                capture_output=True,
                text=True,
                check=True
            )
            vm_state = result.stdout.strip()
            print(f"  虚拟机状态: {vm_state}")
        except:
            vm_state = "unknown"
        
        # 如果虚拟机正在运行，清理并关闭
        if vm_state == "running":
            if cleanup and vm_ip:
                print("\n1. 清理 K8s 配置...")
                ssh_cmd = [
                    'ssh',
                    '-o', 'StrictHostKeyChecking=no',
                    '-o', 'UserKnownHostsFile=/dev/null',
                    '-o', 'ConnectTimeout=5',
                    f"{self.vm_user}@{vm_ip}"
                ]
                self.cleanup_k8s_config(ssh_cmd)
            
            response = input("\n虚拟机正在运行，是否关闭后导出？(y/n，默认: y): ")
            if response.lower() != 'n':
                print("  关闭虚拟机...")
                try:
                    subprocess.run(
                        ['virsh', 'shutdown', self.vm_name],
                        check=True,
                        timeout=60
                    )
                    print("  等待虚拟机关闭...")
                    subprocess.run(
                        ['virsh', 'wait', '--domain', self.vm_name, '--state', 'shutoff'],
                        check=True,
                        timeout=300
                    )
                    print("  ✓ 虚拟机已关闭")
                except subprocess.TimeoutExpired:
                    print("  ⚠ 等待关闭超时，尝试强制关闭...")
                    subprocess.run(['virsh', 'destroy', self.vm_name], check=False)
                except Exception as e:
                    print(f"  ⚠ 关闭虚拟机时出错: {e}")
                    response = input("  是否强制关闭并继续？(y/n): ")
                    if response.lower() == 'y':
                        subprocess.run(['virsh', 'destroy', self.vm_name], check=False)
                    else:
                        return False
        
        # 获取磁盘路径
        print("\n2. 获取虚拟机磁盘路径...")
        disk_path = self.get_vm_disk_path()
        if not disk_path:
            print("  ✗ 无法获取虚拟机磁盘路径")
            return False
        print(f"  ✓ 磁盘路径: {disk_path}")
        
        # 如果输出镜像已存在，询问是否覆盖
        if os.path.exists(self.output_image):
            response = input(f"\n输出镜像已存在: {self.output_image}\n是否覆盖？(y/n): ")
            if response.lower() != 'y':
                print("已取消")
                return False
            os.remove(self.output_image)
        
        # 复制磁盘镜像
        print("\n3. 复制磁盘镜像...")
        try:
            shutil.copy2(disk_path, self.output_image)
            print(f"  ✓ 镜像复制成功")
        except Exception as e:
            print(f"  ✗ 镜像复制失败: {e}")
            return False
        
        # 优化镜像大小并移除 backing file（避免循环引用）
        print("\n4. 优化镜像大小并移除 backing file...")
        try:
            temp_image = self.output_image + '.tmp'
            # 使用 -B none 或 -B "" 来移除 backing file，避免循环引用
            # -c 启用压缩，-O qcow2 指定输出格式
            subprocess.run(
                ['qemu-img', 'convert', '-O', 'qcow2', '-c', '-B', 'none', self.output_image, temp_image],
                check=True,
                capture_output=True,
                timeout=3600  # 1小时超时（大镜像转换可能需要较长时间）
            )
            # 验证转换后的镜像
            verify_result = subprocess.run(
                ['qemu-img', 'info', temp_image],
                capture_output=True,
                text=True,
                check=True
            )
            # 检查是否还有 backing file（不应该有）
            if 'backing file' in verify_result.stdout.lower():
                print("  ⚠ 警告: 转换后的镜像仍有 backing file，可能存在问题")
            os.replace(temp_image, self.output_image)
            print("  ✓ 镜像优化完成（已移除 backing file）")
        except subprocess.TimeoutExpired:
            print(f"  ⚠ 镜像优化超时（镜像可能很大），但原始镜像已保存")
            if os.path.exists(temp_image):
                os.remove(temp_image)
        except Exception as e:
            print(f"  ⚠ 镜像优化失败（可忽略）: {e}")
            if os.path.exists(temp_image):
                os.remove(temp_image)
        
        print("\n" + "=" * 60)
        print("✓ 镜像导出完成！")
        print(f"  输出镜像: {self.output_image}")
        print("\n下一步:")
        print("  1. 更新 vm-config.yaml 中的 k8s_clusters 配置")
        print("  2. 将 base_image 设置为: " + self.output_image)
        print("  3. 使用此镜像创建新的 K8s 节点虚拟机")
        print("=" * 60)
        
        return True


def main():
    parser = argparse.ArgumentParser(
        description='导出 K8s 虚拟机为镜像',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 导出 K8s master 节点
  sudo python3 export_k8s_vm_to_image.py --vm-name vm-k8s-cluster-01-master

  # 指定输出路径
  sudo python3 export_k8s_vm_to_image.py \\
    --vm-name vm-k8s-cluster-01-master \\
    --output /var/lib/libvirt/images/ubuntu-22.04-cloud-k8s-v1.qcow2

  # 不清理配置直接导出
  sudo python3 export_k8s_vm_to_image.py \\
    --vm-name vm-k8s-cluster-01-master \\
    --no-cleanup
        """
    )
    
    parser.add_argument(
        '--vm-name',
        required=True,
        help='虚拟机名称（例如: vm-k8s-cluster-01-master）'
    )
    
    parser.add_argument(
        '--output',
        help='输出镜像路径（如果未指定，自动生成）'
    )
    
    parser.add_argument(
        '--vm-config',
        default='vm-config.yaml',
        help='虚拟机配置文件路径（默认: vm-config.yaml）'
    )
    
    parser.add_argument(
        '--vm-ip',
        help='虚拟机 IP 地址（如果未指定，从配置文件获取）'
    )
    
    parser.add_argument(
        '--no-cleanup',
        action='store_true',
        help='不清理 K8s 配置（不推荐）'
    )
    
    args = parser.parse_args()
    
    try:
        exporter = K8sVMExporter(
            vm_name=args.vm_name,
            output_image=args.output,
            vm_config_path=args.vm_config
        )
        
        success = exporter.export_vm(
            cleanup=not args.no_cleanup,
            vm_ip=args.vm_ip
        )
        
        sys.exit(0 if success else 1)
        
    except KeyboardInterrupt:
        print("\n\n操作被用户中断")
        sys.exit(130)
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()

