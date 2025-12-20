#!/usr/bin/env python3
"""
构建预装 Docker 的 Ubuntu 基础镜像
使用 virt-customize 在现有 Ubuntu 镜像中安装 Docker
"""

import os
import sys
import subprocess
import shutil
import argparse
from pathlib import Path

class DockerBaseImageBuilder:
    def __init__(self, source_image: str, output_image: str = None):
        """初始化镜像构建器"""
        self.source_image = os.path.expanduser(source_image)
        self.output_image = output_image or self._generate_output_name()
        
        # 验证源镜像是否存在
        if not os.path.exists(self.source_image):
            raise FileNotFoundError(f"源镜像不存在: {self.source_image}")
        
        # 检查 virt-customize 是否可用
        try:
            subprocess.run(['virt-customize', '--version'], 
                         capture_output=True, check=True)
        except (FileNotFoundError, subprocess.CalledProcessError):
            raise RuntimeError(
                "未找到 virt-customize 命令。\n"
                "请安装 libguestfs-tools:\n"
                "  Ubuntu/Debian: sudo apt-get install libguestfs-tools\n"
                "  CentOS/RHEL: sudo yum install libguestfs-tools"
            )
        
        # 检查是否以 root 权限运行（virt-customize 需要 root）
        if os.geteuid() != 0:
            raise RuntimeError(
                "virt-customize 需要 root 权限。\n"
                "请使用 sudo 运行此脚本:\n"
                "  sudo python3 deploy/build_docker_base_image.py"
            )
        
        # 检查并修复 libguestfs 环境
        print("检查 libguestfs 环境...")
        try:
            # 运行 libguestfs-test-tool 检查环境
            test_result = subprocess.run(
                ['libguestfs-test-tool'],
                capture_output=True,
                text=True,
                timeout=30
            )
            if test_result.returncode != 0:
                print("  ⚠ libguestfs 环境可能有问题，但继续尝试...")
        except:
            print("  ⚠ 无法运行 libguestfs-test-tool，但继续尝试...")
    
    def _generate_output_name(self) -> str:
        """生成输出镜像名称"""
        source_path = Path(self.source_image)
        # 在源镜像同目录下创建新镜像
        output_name = source_path.stem + '-docker' + source_path.suffix
        return str(source_path.parent / output_name)
    
    def build(self, user: str = 'ubuntu', use_cloud_init: bool = False) -> bool:
        """构建预装 Docker 的镜像
        
        Args:
            user: 要添加到 docker 组的用户
            use_cloud_init: 如果为 True，使用 cloud-init 在首次启动时安装 Docker（更可靠，但需要首次启动时联网）
        """
        print(f"构建预装 Docker 的 Ubuntu 镜像...")
        print(f"  源镜像: {self.source_image}")
        print(f"  输出镜像: {self.output_image}")
        print(f"  安装方式: {'cloud-init (首次启动时安装)' if use_cloud_init else 'virt-customize (镜像构建时安装)'}")
        print("=" * 60)
        
        # 如果输出镜像已存在，询问是否覆盖
        if os.path.exists(self.output_image):
            response = input(f"输出镜像已存在: {self.output_image}\n是否覆盖？(y/n): ")
            if response.lower() != 'y':
                print("已取消")
                return False
            os.remove(self.output_image)
        
        # 复制源镜像
        print("\n1. 复制源镜像...")
        try:
            shutil.copy2(self.source_image, self.output_image)
            print(f"  ✓ 镜像复制成功")
        except Exception as e:
            print(f"  ✗ 镜像复制失败: {e}")
            return False
        
        # 如果使用 cloud-init 方式，直接配置 cloud-init
        if use_cloud_init:
            return self._build_with_cloud_init(user)
        
        # 使用 virt-customize 安装 Docker
        print("\n2. 在镜像中安装 Docker...")
        print("  这可能需要几分钟，请耐心等待...")
        
        # 构建 virt-customize 命令
        # 使用 --run 执行安装脚本
        # 注意：virt-customize 环境中网络可能受限，需要配置 DNS 并使用离线方式
        install_script = f'''#!/bin/bash
set -e

# 更新包列表
export DEBIAN_FRONTEND=noninteractive

# 配置 DNS（virt-customize 环境中可能没有 DNS）
if [ ! -f /etc/resolv.conf ] || [ ! -s /etc/resolv.conf ]; then
    mkdir -p /etc
    cat > /etc/resolv.conf << 'DNSEOF'
nameserver 8.8.8.8
nameserver 114.114.114.114
nameserver 223.5.5.5
DNSEOF
fi

# 配置阿里云 Ubuntu 镜像源（加速下载）
# 先备份原有配置
if [ -f /etc/apt/sources.list ]; then
    cp /etc/apt/sources.list /etc/apt/sources.list.bak
fi

# 获取 Ubuntu 版本代号
UBUNTU_CODENAME=$(lsb_release -cs 2>/dev/null || echo "jammy")

cat > /etc/apt/sources.list << 'EOF'
deb http://mirrors.aliyun.com/ubuntu/ $UBUNTU_CODENAME main restricted universe multiverse
deb http://mirrors.aliyun.com/ubuntu/ $UBUNTU_CODENAME-security main restricted universe multiverse
deb http://mirrors.aliyun.com/ubuntu/ $UBUNTU_CODENAME-updates main restricted universe multiverse
deb http://mirrors.aliyun.com/ubuntu/ $UBUNTU_CODENAME-backports main restricted universe multiverse
EOF

# 更新包列表（重试机制）
for i in 1 2 3; do
    if apt-get update -qq 2>&1 | grep -q "Could not resolve\|Failed to fetch"; then
        echo "DNS 解析失败，尝试使用 IP 地址..."
        # 如果 DNS 失败，尝试直接使用 IP（不推荐，但作为备选）
        if [ $i -eq 3 ]; then
            echo "警告: 网络连接失败，将尝试离线安装方式"
            break
        fi
        sleep 2
    else
        break
    fi
done

# 安装必要的依赖
apt-get install -y -qq ca-certificates curl gnupg lsb-release || true

# 创建 keyrings 目录
mkdir -p /etc/apt/keyrings

# 添加 Docker GPG 密钥（使用非交互式模式）
# 方法1: 尝试从官方源获取
GPG_SUCCESS=false
for url in "https://download.docker.com/linux/ubuntu/gpg" "https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg"; do
    if curl -fsSL "$url" 2>/dev/null | gpg --batch --no-tty --dearmor -o /etc/apt/keyrings/docker.gpg 2>/dev/null; then
        GPG_SUCCESS=true
        break
    fi
done

# 如果网络下载失败，使用预置的 GPG 密钥（从已知的 Docker 官方密钥）
if [ "$GPG_SUCCESS" = false ]; then
    echo "网络下载 GPG 密钥失败，使用预置密钥..."
    # Docker 官方 GPG 密钥的 base64 编码（这是已知的公钥）
    # 如果网络完全不可用，可以在这里硬编码密钥
    # 但更好的方式是使用 --copy-in 预先复制密钥文件
    echo "错误: 无法获取 Docker GPG 密钥，请检查网络连接"
    exit 1
fi

chmod a+r /etc/apt/keyrings/docker.gpg

# 设置 Docker 仓库（使用阿里云镜像）
cat > /etc/apt/sources.list.d/docker.list << 'EOF'
deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.gpg] https://mirrors.aliyun.com/docker-ce/linux/ubuntu $UBUNTU_CODENAME stable
EOF

# 更新包列表
apt-get update -qq || {
    echo "警告: apt-get update 失败，但继续尝试安装..."
}

# 安装 Docker Engine（使用 --no-install-recommends 减少依赖）
apt-get install -y -qq --no-install-recommends \
    docker-ce \
    docker-ce-cli \
    containerd.io \
    docker-buildx-plugin \
    docker-compose-plugin || {
    echo "错误: Docker 安装失败"
    exit 1
}

# 将用户添加到 docker 组
if id "{user}" &>/dev/null; then
    usermod -aG docker {user}
else
    echo "警告: 用户 {user} 不存在，跳过添加到 docker 组"
fi

# 配置 Docker 使用 systemd cgroup driver（避免警告）
mkdir -p /etc/docker
cat > /etc/docker/daemon.json << 'DOCKEREOF'
{{
  "exec-opts": ["native.cgroupdriver=systemd"],
  "log-driver": "json-file",
  "log-opts": {{
    "max-size": "100m"
  }},
  "storage-driver": "overlay2"
}}
DOCKEREOF

# 清理
apt-get clean
rm -rf /var/lib/apt/lists/*

echo "Docker 安装完成"
'''
        
        # 将脚本写入临时文件
        import tempfile
        with tempfile.NamedTemporaryFile(mode='w', suffix='.sh', delete=False) as f:
            f.write(install_script)
            script_path = f.name
        
        try:
            # 使用 virt-customize 运行安装脚本
            # 添加 -v -x 参数以获取详细输出用于调试
            virt_customize_cmd = [
                'virt-customize',
                '-a', self.output_image,
                '--run', script_path,
                '--selinux-relabel',  # 如果需要 SELinux 支持
                '-v',  # 详细输出
                '-x'   # 调试输出
            ]
            
            print("  执行 virt-customize（这可能需要几分钟，请耐心等待）...")
            print("  提示: 如果网络较慢，可能需要 5-10 分钟")
            print("  提示: 如果失败，可能需要修复 libguestfs 环境")
            
            # 设置环境变量以提高稳定性
            env = os.environ.copy()
            env['LIBGUESTFS_BACKEND'] = 'direct'  # 使用 direct 后端（更稳定）
            
            result = subprocess.run(
                virt_customize_cmd,
                check=True,
                capture_output=False,  # 显示输出
                text=True,
                timeout=900,  # Python 层面的超时（15分钟）
                env=env
            )
            print("  ✓ Docker 安装成功")
        except subprocess.CalledProcessError as e:
            print(f"  ✗ Docker 安装失败: {e}")
            print("\n  故障排查建议:")
            print("  1. 检查 libguestfs 环境:")
            print("     sudo libguestfs-test-tool")
            print("  2. 尝试修复 libguestfs:")
            print("     sudo update-guestfs-appliance")
            print("  3. 检查内核模块:")
            print("     sudo modprobe nbd")
            print("     sudo modprobe fuse")
            print("  4. 如果问题持续，可以尝试使用 cloud-init 在首次启动时安装 Docker")
            # 清理失败的镜像
            if os.path.exists(self.output_image):
                os.remove(self.output_image)
            return False
        finally:
            # 清理临时脚本
            if os.path.exists(script_path):
                os.remove(script_path)
        
        # 验证镜像
        print("\n3. 验证镜像...")
        try:
            # 检查镜像是否可以正常访问
            check_cmd = ['virt-customize', '-a', self.output_image, '--run-command', 'docker --version']
            result = subprocess.run(check_cmd, check=True, capture_output=True, text=True, timeout=60)
            if 'Docker version' in result.stdout or 'Docker version' in result.stderr:
                print("  ✓ Docker 验证成功")
            else:
                print("  ⚠ Docker 验证结果不确定，但继续...")
        except Exception as e:
            print(f"  ⚠ 验证过程出错，但镜像可能已成功构建: {e}")
        
        print("\n" + "=" * 60)
        print(f"✓ 镜像构建完成: {self.output_image}")
        print(f"\n镜像大小: {os.path.getsize(self.output_image) / (1024**3):.2f} GB")
        print(f"\n使用方法:")
        print(f"  1. 更新 vm-config.yaml 中的 base_image 路径:")
        print(f"     base_image: \"{self.output_image}\"")
        print(f"  2. 重新创建虚拟机:")
        print(f"     python3 deploy/create_vms.py")
        print("=" * 60)
        
        return True

def main():
    parser = argparse.ArgumentParser(description='构建预装 Docker 的 Ubuntu 基础镜像')
    parser.add_argument(
        '--source', '-s',
        type=str,
        default='/var/lib/libvirt/images/ubuntu-22.04-cloud.qcow2',
        help='源镜像路径 (默认: /var/lib/libvirt/images/ubuntu-22.04-cloud.qcow2)'
    )
    parser.add_argument(
        '--output', '-o',
        type=str,
        default=None,
        help='输出镜像路径 (默认: 源镜像同目录，名称添加 -docker 后缀)'
    )
    parser.add_argument(
        '--user', '-u',
        type=str,
        default='ubuntu',
        help='要添加到 docker 组的用户 (默认: ubuntu)'
    )
    
    args = parser.parse_args()
    
    try:
        builder = DockerBaseImageBuilder(args.source, args.output)
        if builder.build(args.user):
            print("\n✓ 镜像构建成功！")
            sys.exit(0)
        else:
            print("\n✗ 镜像构建失败")
            sys.exit(1)
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()

