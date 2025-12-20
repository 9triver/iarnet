#!/usr/bin/env python3
"""
将运行中的虚拟机导出为镜像
适用于已经安装好 Docker 的虚拟机
"""

import os
import sys
import subprocess
import argparse
import shutil
from pathlib import Path

class VMExporter:
    def __init__(self, vm_name: str, output_image: str = None):
        """初始化虚拟机导出器"""
        self.vm_name = vm_name
        self.output_image = output_image or self._generate_output_name()
        
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
        
        # 检查是否以 root 权限运行（某些操作需要 root）
        if os.geteuid() != 0:
            print("警告: 某些操作可能需要 root 权限")
    
    def _generate_output_name(self) -> str:
        """生成输出镜像名称"""
        # 默认保存到 /var/lib/libvirt/images/
        default_dir = '/var/lib/libvirt/images'
        if os.path.exists(default_dir) and os.access(default_dir, os.W_OK):
            output_name = f"{self.vm_name}-exported.qcow2"
            return os.path.join(default_dir, output_name)
        else:
            # 保存到当前目录
            return f"{self.vm_name}-exported.qcow2"
    
    def check_vm_exists(self) -> bool:
        """检查虚拟机是否存在"""
        try:
            result = subprocess.run(
                ['virsh', 'dominfo', self.vm_name],
                capture_output=True,
                text=True,
                check=True
            )
            return True
        except subprocess.CalledProcessError:
            return False
    
    def get_vm_disk_path(self) -> str:
        """获取虚拟机的磁盘路径"""
        try:
            result = subprocess.run(
                ['virsh', 'domblklist', self.vm_name],
                capture_output=True,
                text=True,
                check=True
            )
            # 解析输出，找到磁盘路径
            for line in result.stdout.split('\n'):
                if line.strip() and not line.startswith('Target') and not line.startswith('---'):
                    parts = line.split()
                    if len(parts) >= 2:
                        disk_path = parts[1]
                        if disk_path.startswith('/'):
                            return disk_path
            return None
        except subprocess.CalledProcessError as e:
            print(f"  ✗ 无法获取虚拟机磁盘路径: {e}")
            return None
    
    def cleanup_vm(self, ssh_cmd: list, remove_iarnet: bool = True) -> bool:
        """清理虚拟机中的临时文件和敏感信息
        
        Args:
            ssh_cmd: SSH 命令列表
            remove_iarnet: 是否移除 iarnet 相关内容
        """
        print("  清理虚拟机中的临时文件和 iarnet 相关内容...")
        
        cleanup_script = '''#!/bin/bash
set -e

# ========== 停止并清理 iarnet 相关服务 ==========
if [ "''' + ('true' if remove_iarnet else 'false') + '''" = "true" ]; then
    echo "停止 iarnet 相关服务..."
    
    # 停止 iarnet 进程
    pkill -f iarnet 2>/dev/null || true
    pkill -f "npm start" 2>/dev/null || true
    sleep 2
    
    # 停止 systemd 服务（如果存在）
    systemctl stop iarnet 2>/dev/null || true
    systemctl stop iarnet-backend 2>/dev/null || true
    systemctl stop iarnet-frontend 2>/dev/null || true
    systemctl disable iarnet 2>/dev/null || true
    systemctl disable iarnet-backend 2>/dev/null || true
    systemctl disable iarnet-frontend 2>/dev/null || true
    
    # 删除 systemd 服务文件
    rm -f /etc/systemd/system/iarnet.service 2>/dev/null || true
    rm -f /etc/systemd/system/iarnet-backend.service 2>/dev/null || true
    rm -f /etc/systemd/system/iarnet-frontend.service 2>/dev/null || true
    rm -f /etc/systemd/system/multi-user.target.wants/iarnet.service 2>/dev/null || true
    rm -f /etc/systemd/system/multi-user.target.wants/iarnet-backend.service 2>/dev/null || true
    rm -f /etc/systemd/system/multi-user.target.wants/iarnet-frontend.service 2>/dev/null || true
    
    # 重新加载 systemd
    systemctl daemon-reload 2>/dev/null || true
    
    # 删除 iarnet 目录和文件
    rm -rf ~/iarnet 2>/dev/null || true
    rm -rf /opt/iarnet 2>/dev/null || true
    rm -rf /usr/local/iarnet 2>/dev/null || true
    
    # 清理 iarnet 相关的配置文件
    rm -f ~/.iarnet* 2>/dev/null || true
    rm -f /etc/iarnet* 2>/dev/null || true
    
    # 清理 crontab 中的 iarnet 任务
    crontab -l 2>/dev/null | grep -v iarnet | crontab - 2>/dev/null || true
    
    # 清理自动启动脚本
    rm -f ~/.bashrc.iarnet 2>/dev/null || true
    rm -f /etc/profile.d/iarnet.sh 2>/dev/null || true
    rm -f /etc/rc.local.iarnet 2>/dev/null || true
    
    # 清理 /etc/rc.local 中的 iarnet 相关内容（如果存在）
    if [ -f /etc/rc.local ]; then
        sed -i '/iarnet/d' /etc/rc.local 2>/dev/null || true
    fi
    
    # 清理 Docker 中的 iarnet 容器和镜像（如果使用 Docker 部署）
    docker stop $(docker ps -q --filter "name=iarnet") 2>/dev/null || true
    docker rm $(docker ps -aq --filter "name=iarnet") 2>/dev/null || true
    docker rmi $(docker images -q iarnet) 2>/dev/null || true
    
    echo "  ✓ iarnet 相关内容已清理"
fi

# ========== 清理通用临时文件 ==========
echo "清理临时文件..."

# 清理 apt 缓存
apt-get clean 2>/dev/null || true
rm -rf /var/lib/apt/lists/* 2>/dev/null || true

# 清理临时文件
rm -rf /tmp/* /var/tmp/* 2>/dev/null || true

# 清理日志（保留最近的一些）
find /var/log -type f -name "*.log" -exec truncate -s 0 {} \\; 2>/dev/null || true
journalctl --vacuum-time=1d 2>/dev/null || true

# 清理命令历史
if [ -f ~/.bash_history ]; then
    > ~/.bash_history
fi
if [ -f /root/.bash_history ]; then
    > /root/.bash_history
fi

# 清理 SSH 主机密钥（可选，如果希望每次启动生成新的）
# rm -f /etc/ssh/ssh_host_*_key* 2>/dev/null || true

# 清理 cloud-init 数据（如果使用 cloud-init）
cloud-init clean 2>/dev/null || true
rm -rf /var/lib/cloud/instances/* 2>/dev/null || true

# 清理 Docker 日志和临时数据（但保留 Docker 本身）
docker system prune -f 2>/dev/null || true
rm -rf /var/lib/docker/tmp/* 2>/dev/null || true

# 清理网络配置（可选）
# rm -f /etc/netplan/*.yaml.bak 2>/dev/null || true

echo "清理完成"
'''
        
        # 将清理脚本编码为 base64，通过 SSH 执行
        import base64
        script_b64 = base64.b64encode(cleanup_script.encode('utf-8')).decode('utf-8')
        
        # 创建远程执行命令
        remote_cmd = f'echo {script_b64} | base64 -d | bash'
        full_cmd = ' '.join(ssh_cmd) + f' "{remote_cmd}"'
        
        try:
            result = subprocess.run(
                full_cmd,
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
    
    def export_vm(self, cleanup: bool = True, vm_ip: str = None, vm_user: str = 'ubuntu', remove_iarnet: bool = True) -> bool:
        """导出虚拟机为镜像"""
        print(f"导出虚拟机为镜像...")
        print(f"  虚拟机名称: {self.vm_name}")
        print(f"  输出镜像: {self.output_image}")
        print("=" * 60)
        
        # 检查虚拟机是否存在
        if not self.check_vm_exists():
            print(f"  ✗ 虚拟机不存在: {self.vm_name}")
            print("\n可用的虚拟机列表:")
            try:
                result = subprocess.run(
                    ['virsh', 'list', '--all'],
                    capture_output=True,
                    text=True,
                    check=True
                )
                print(result.stdout)
            except:
                pass
            return False
        
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
        
        # 如果虚拟机正在运行，可以选择关闭或直接导出
        if vm_state == "running":
            if cleanup and vm_ip:
                print("\n1. 清理虚拟机...")
                ssh_cmd = [
                    'ssh',
                    '-o', 'StrictHostKeyChecking=no',
                    '-o', 'UserKnownHostsFile=/dev/null',
                    '-o', 'ConnectTimeout=5',
                    f"{vm_user}@{vm_ip}"
                ]
                self.cleanup_vm(ssh_cmd, remove_iarnet=remove_iarnet)
            
            response = input("\n虚拟机正在运行，是否关闭后导出？(y/n，默认: y): ")
            if response.lower() != 'n':
                print("  关闭虚拟机...")
                try:
                    subprocess.run(
                        ['virsh', 'shutdown', self.vm_name],
                        check=True,
                        timeout=60
                    )
                    # 等待虚拟机完全关闭
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
        print(f"  磁盘路径: {disk_path}")
        
        # 检查磁盘文件是否存在
        if not os.path.exists(disk_path):
            print(f"  ✗ 磁盘文件不存在: {disk_path}")
            return False
        
        # 如果输出镜像已存在，询问是否覆盖
        if os.path.exists(self.output_image):
            response = input(f"\n输出镜像已存在: {self.output_image}\n是否覆盖？(y/n): ")
            if response.lower() != 'y':
                print("已取消")
                return False
            os.remove(self.output_image)
        
        # 复制磁盘镜像
        print(f"\n3. 复制磁盘镜像...")
        print(f"  从: {disk_path}")
        print(f"  到: {self.output_image}")
        print("  这可能需要几分钟，请耐心等待...")
        
        try:
            # 使用 qemu-img 转换（如果可用）或直接复制
            if shutil.which('qemu-img'):
                # 使用 qemu-img convert 可以优化镜像
                print("  使用 qemu-img 转换镜像（优化中）...")
                subprocess.run(
                    ['qemu-img', 'convert', '-O', 'qcow2', '-c', disk_path, self.output_image],
                    check=True
                )
            else:
                # 直接复制
                print("  直接复制镜像文件...")
                shutil.copy2(disk_path, self.output_image)
            
            print("  ✓ 镜像复制成功")
        except Exception as e:
            print(f"  ✗ 镜像复制失败: {e}")
            return False
        
        # 验证镜像
        print("\n4. 验证镜像...")
        if os.path.exists(self.output_image):
            size_gb = os.path.getsize(self.output_image) / (1024**3)
            print(f"  ✓ 镜像文件存在")
            print(f"  镜像大小: {size_gb:.2f} GB")
        else:
            print("  ✗ 镜像文件不存在")
            return False
        
        print("\n" + "=" * 60)
        print(f"✓ 虚拟机导出成功: {self.output_image}")
        print(f"\n使用方法:")
        print(f"  1. 更新 vm-config.yaml 中的 base_image 路径:")
        print(f"     base_image: \"{self.output_image}\"")
        print(f"  2. 重新创建虚拟机:")
        print(f"     python3 deploy/create_vms.py")
        print("=" * 60)
        
        return True

def main():
    parser = argparse.ArgumentParser(description='将运行中的虚拟机导出为镜像')
    parser.add_argument(
        '--vm-name', '-n',
        type=str,
        required=True,
        help='虚拟机名称（virsh 中的名称）'
    )
    parser.add_argument(
        '--output', '-o',
        type=str,
        default=None,
        help='输出镜像路径 (默认: /var/lib/libvirt/images/<vm-name>-exported.qcow2)'
    )
    parser.add_argument(
        '--no-cleanup',
        action='store_true',
        help='不清理虚拟机中的临时文件'
    )
    parser.add_argument(
        '--vm-ip',
        type=str,
        default=None,
        help='虚拟机 IP 地址（用于清理，如果虚拟机正在运行）'
    )
    parser.add_argument(
        '--vm-user',
        type=str,
        default='ubuntu',
        help='虚拟机 SSH 用户名 (默认: ubuntu)'
    )
    parser.add_argument(
        '--keep-iarnet',
        action='store_true',
        help='保留 iarnet 相关内容（默认会清理）'
    )
    
    args = parser.parse_args()
    
    try:
        exporter = VMExporter(args.vm_name, args.output)
        if exporter.export_vm(
            cleanup=not args.no_cleanup,
            vm_ip=args.vm_ip,
            vm_user=args.vm_user,
            remove_iarnet=not args.keep_iarnet
        ):
            print("\n✓ 导出成功！")
            sys.exit(0)
        else:
            print("\n✗ 导出失败")
            sys.exit(1)
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()

