#!/usr/bin/env python3
"""
构建预装 Kubernetes 依赖的 Ubuntu 基础镜像
使用 virt-customize 在现有 Ubuntu 镜像中安装 kubeadm, kubelet, kubectl 和 containerd
"""

import os
import sys
import subprocess
import shutil
import argparse
from pathlib import Path

class K8sBaseImageBuilder:
    def __init__(self, source_image: str, output_image: str = None, k8s_version: str = "1.28.0"):
        """初始化镜像构建器
        
        Args:
            source_image: 源镜像路径（应该已经包含 Docker）
            output_image: 输出镜像路径（如果为 None，自动生成）
            k8s_version: Kubernetes 版本（格式：1.28.0）
        """
        self.source_image = os.path.expanduser(source_image)
        self.k8s_version = k8s_version
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
                "  sudo python3 deploy/build_k8s_base_image.py"
            )
    
    def _generate_output_name(self) -> str:
        """生成输出镜像名称"""
        source_path = Path(self.source_image)
        # 在源镜像同目录下创建新镜像
        output_name = source_path.stem + '-k8s' + source_path.suffix
        return str(source_path.parent / output_name)
    
    def build(self, user: str = 'ubuntu') -> bool:
        """构建预装 K8s 依赖的镜像
        
        Args:
            user: 要添加到 docker 组的用户
        """
        print(f"构建预装 Kubernetes 依赖的 Ubuntu 镜像...")
        print(f"  源镜像: {self.source_image}")
        print(f"  输出镜像: {self.output_image}")
        print(f"  Kubernetes 版本: {self.k8s_version}")
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
        
        # 使用 virt-customize 安装 K8s 依赖
        print("\n2. 在镜像中安装 Kubernetes 依赖...")
        print("  这可能需要几分钟，请耐心等待...")
        
        # 构建 virt-customize 命令
        # 使用 --run 执行安装脚本
        install_script = f'''#!/bin/bash
set -e

# 更新包列表
export DEBIAN_FRONTEND=noninteractive

# 配置 DNS（virt-customize 环境中可能没有 DNS）
# 先尝试从主机复制 resolv.conf，如果失败则创建新的
if [ -f /etc/resolv.conf ]; then
    # 备份现有配置
    cp /etc/resolv.conf /etc/resolv.conf.bak 2>/dev/null || true
fi

# 创建 DNS 配置（使用多个 DNS 服务器）
cat > /etc/resolv.conf << 'DNSEOF'
nameserver 8.8.8.8
nameserver 8.8.4.4
nameserver 114.114.114.114
nameserver 223.5.5.5
nameserver 1.1.1.1
DNSEOF

# 配置阿里云 Ubuntu 镜像源（加速下载）
if [ -f /etc/apt/sources.list ]; then
    cp /etc/apt/sources.list /etc/apt/sources.list.bak
fi

# 获取 Ubuntu 版本代号（使用 eval 确保变量正确展开）
UBUNTU_CODENAME=$(lsb_release -cs 2>/dev/null || echo "jammy")
# 如果 lsb_release 不可用，尝试从 /etc/os-release 读取
if [ "$UBUNTU_CODENAME" = "" ] || [ "$UBUNTU_CODENAME" = "jammy" ]; then
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        if [ "$VERSION_CODENAME" != "" ]; then
            UBUNTU_CODENAME="$VERSION_CODENAME"
        fi
    fi
fi

# 使用 eval 或直接展开变量来写入 sources.list
cat > /etc/apt/sources.list << EOF
deb http://mirrors.aliyun.com/ubuntu/ $UBUNTU_CODENAME main restricted universe multiverse
deb http://mirrors.aliyun.com/ubuntu/ $UBUNTU_CODENAME-security main restricted universe multiverse
deb http://mirrors.aliyun.com/ubuntu/ $UBUNTU_CODENAME-updates main restricted universe multiverse
deb http://mirrors.aliyun.com/ubuntu/ $UBUNTU_CODENAME-backports main restricted universe multiverse
EOF

# 测试 DNS 解析和网络连接
echo "测试网络连接..."
DNS_WORKING=false
if command -v nslookup >/dev/null 2>&1; then
    if nslookup mirrors.aliyun.com >/dev/null 2>&1; then
        DNS_WORKING=true
    fi
elif command -v host >/dev/null 2>&1; then
    if host mirrors.aliyun.com >/dev/null 2>&1; then
        DNS_WORKING=true
    fi
fi

if [ "$DNS_WORKING" = false ]; then
    echo "警告: DNS 解析失败，virt-customize 环境中可能无法访问网络"
    echo "提示: 如果网络完全不可用，建议使用 cloud-init 在首次启动时安装依赖"
    echo "继续尝试安装（可能会失败）..."
fi

# 更新包列表（重试机制，增加重试次数）
MAX_RETRIES=5
RETRY_DELAY=3
for i in $(seq 1 $MAX_RETRIES); do
    echo "尝试更新包列表 (第 $i 次)..."
    if apt-get update -qq 2>&1 | grep -q "Could not resolve\|Failed to fetch\|Temporary failure"; then
        if [ $i -eq $MAX_RETRIES ]; then
            echo "错误: 网络连接失败，无法继续安装"
            echo "提示: virt-customize 环境中可能无法访问网络"
            echo "建议: 使用 --copy-in 预先复制必要的包文件，或使用 cloud-init 在首次启动时安装"
            exit 1
        fi
        echo "等待 $RETRY_DELAY 秒后重试..."
        sleep $RETRY_DELAY
    else
        echo "包列表更新成功"
        break
    fi
done

# 安装必要的依赖
echo "安装基础依赖..."
apt-get install -y -qq ca-certificates curl gnupg lsb-release apt-transport-https || {{
    echo "错误: 无法安装基础依赖，可能是网络问题"
    exit 1
}}

# ============================================
# 1. 安装 containerd（K8s 1.24+ 默认使用 containerd）
# ============================================
echo "安装 containerd..."

# 创建 keyrings 目录
mkdir -p /etc/apt/keyrings

# 添加 containerd GPG 密钥
CONTAINERD_GPG_SUCCESS=false
for url in "https://download.docker.com/linux/ubuntu/gpg" "https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg"; do
    if curl -fsSL "$url" 2>/dev/null | gpg --batch --no-tty --dearmor -o /etc/apt/keyrings/containerd.gpg 2>/dev/null; then
        CONTAINERD_GPG_SUCCESS=true
        break
    fi
done

if [ "$CONTAINERD_GPG_SUCCESS" = false ]; then
    echo "警告: 无法获取 containerd GPG 密钥，尝试继续..."
fi

chmod a+r /etc/apt/keyrings/containerd.gpg 2>/dev/null || true

# 设置 containerd 仓库（使用变量展开）
cat > /etc/apt/sources.list.d/containerd.list << EOF
deb [arch=amd64 signed-by=/etc/apt/keyrings/containerd.gpg] https://mirrors.aliyun.com/docker-ce/linux/ubuntu $UBUNTU_CODENAME stable
EOF

# 更新包列表
apt-get update -qq || true

# 安装 containerd
apt-get install -y -qq --no-install-recommends containerd.io || {{
    echo "警告: containerd 安装失败，但继续..."
}}

# 配置 containerd 使用 systemd cgroup driver
mkdir -p /etc/containerd
containerd config default 2>/dev/null | sed 's/SystemdCgroup = false/SystemdCgroup = true/' > /etc/containerd/config.toml || {{
    # 如果 containerd 命令不可用，创建基本配置
    cat > /etc/containerd/config.toml << 'CONTAINERDEOF'
version = 2
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
    SystemdCgroup = true
CONTAINERDEOF
}}

# ============================================
# 2. 安装 Kubernetes 组件（kubeadm, kubelet, kubectl）
# ============================================
echo "安装 Kubernetes 组件..."

# 添加 Kubernetes GPG 密钥
K8S_GPG_SUCCESS=false
for url in "https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key" "https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg"; do
    if curl -fsSL "$url" 2>/dev/null | gpg --batch --no-tty --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg 2>/dev/null; then
        K8S_GPG_SUCCESS=true
        break
    fi
done

# 如果官方源失败，尝试使用阿里云镜像
if [ "$K8S_GPG_SUCCESS" = false ]; then
    echo "尝试使用阿里云 Kubernetes 镜像源..."
    curl -fsSL https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg 2>/dev/null | gpg --batch --no-tty --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg 2>/dev/null || true
fi

chmod a+r /etc/apt/keyrings/kubernetes-apt-keyring.gpg 2>/dev/null || true

# 设置 Kubernetes 仓库（使用阿里云镜像）
cat > /etc/apt/sources.list.d/kubernetes.list << EOF
deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://mirrors.aliyun.com/kubernetes/apt/ kubernetes-xenial main
EOF

# 更新包列表
apt-get update -qq || true

# 安装指定版本的 kubeadm, kubelet, kubectl
K8S_VERSION="{self.k8s_version}-00"
apt-get install -y -qq --no-install-recommends \
    kubelet=${{K8S_VERSION}} \
    kubeadm=${{K8S_VERSION}} \
    kubectl=${{K8S_VERSION}} || {{
    echo "错误: Kubernetes 组件安装失败"
    # 如果指定版本安装失败，尝试安装最新版本
    echo "尝试安装最新版本..."
    apt-get install -y -qq --no-install-recommends kubelet kubeadm kubectl || {{
        echo "错误: Kubernetes 组件安装完全失败"
        exit 1
    }}
}}

# 锁定 kubelet, kubeadm, kubectl 版本，防止自动升级
apt-mark hold kubelet kubeadm kubectl || true

# ============================================
# 3. 配置系统参数（K8s 要求）
# ============================================
echo "配置系统参数..."

# 禁用 swap（K8s 要求）
# 注释掉 /etc/fstab 中的 swap 行
sed -i '/ swap / s/^/#/' /etc/fstab || true

# 配置内核参数（加载 br_netfilter 模块）
cat > /etc/modules-load.d/k8s.conf << 'EOF'
overlay
br_netfilter
EOF

# 配置 sysctl 参数
cat > /etc/sysctl.d/k8s.conf << 'EOF'
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

# 注意：这些模块和 sysctl 参数需要在运行时加载/应用
# 在镜像构建时无法完全生效，需要在首次启动时执行

# ============================================
# 4. 创建首次启动时的初始化脚本
# ============================================
echo "创建首次启动初始化脚本..."

# 创建 systemd 服务，在首次启动时加载内核模块和应用 sysctl
cat > /etc/systemd/system/k8s-init.service << 'EOF'
[Unit]
Description=Kubernetes Initialization
After=network.target
Before=kubelet.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/bash -c 'modprobe overlay && modprobe br_netfilter && sysctl --system'

[Install]
WantedBy=multi-user.target
EOF

# 启用服务
systemctl enable k8s-init.service || true

# ============================================
# 5. 配置 kubelet 使用 systemd cgroup driver
# ============================================
echo "配置 kubelet..."

# 创建 kubelet 配置目录
mkdir -p /var/lib/kubelet

# 配置 kubelet 使用 systemd cgroup driver
cat > /var/lib/kubelet/config.yaml << 'EOF'
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
cgroupDriver: systemd
EOF

# ============================================
# 6. 清理
# ============================================
apt-get clean
rm -rf /var/lib/apt/lists/*
rm -f /etc/apt/sources.list.bak

echo "Kubernetes 依赖安装完成"
echo "注意: 以下操作需要在首次启动时执行:"
echo "  - 加载内核模块 (overlay, br_netfilter)"
echo "  - 应用 sysctl 参数"
echo "  - 禁用 swap (如果已启用)"
'''
        
        # 将脚本写入临时文件
        import tempfile
        with tempfile.NamedTemporaryFile(mode='w', suffix='.sh', delete=False) as f:
            f.write(install_script)
            script_path = f.name
        
        try:
            # 使用 virt-customize 运行安装脚本
            virt_customize_cmd = [
                'virt-customize',
                '-a', self.output_image,
                '--run', script_path,
                '--selinux-relabel',
                '-v',
                '-x'
            ]
            
            print("  执行 virt-customize（这可能需要几分钟，请耐心等待）...")
            print("  提示: 如果网络较慢，可能需要 10-15 分钟")
            
            result = subprocess.run(
                virt_customize_cmd,
                capture_output=False,  # 直接输出到终端
                text=True
            )
            
            if result.returncode != 0:
                print(f"\n  ✗ virt-customize 执行失败（返回码: {result.returncode}）")
                return False
            
            print(f"\n  ✓ Kubernetes 依赖安装成功")
            
        except subprocess.TimeoutExpired:
            print("\n  ✗ virt-customize 执行超时")
            return False
        except Exception as e:
            print(f"\n  ✗ virt-customize 执行异常: {e}")
            return False
        finally:
            # 清理临时脚本
            if os.path.exists(script_path):
                os.remove(script_path)
        
        # 优化镜像大小
        print("\n3. 优化镜像大小...")
        try:
            # 使用 qemu-img 转换并压缩镜像
            temp_image = self.output_image + '.tmp'
            subprocess.run(
                ['qemu-img', 'convert', '-O', 'qcow2', '-c', self.output_image, temp_image],
                check=True,
                capture_output=True
            )
            os.replace(temp_image, self.output_image)
            print("  ✓ 镜像优化完成")
        except Exception as e:
            print(f"  ⚠ 镜像优化失败（可忽略）: {e}")
        
        print("\n" + "=" * 60)
        print("✓ 镜像构建完成！")
        print(f"  输出镜像: {self.output_image}")
        print("\n下一步:")
        print("  1. 使用此镜像创建 K8s 集群虚拟机")
        print("  2. 在 master 节点上运行: kubeadm init")
        print("  3. 在 worker 节点上运行: kubeadm join")
        print("=" * 60)
        
        return True


def main():
    parser = argparse.ArgumentParser(
        description='构建预装 Kubernetes 依赖的 Ubuntu 基础镜像',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 使用默认配置构建镜像
  sudo python3 build_k8s_base_image.py --source /var/lib/libvirt/images/ubuntu-22.04-cloud-docker.qcow2

  # 指定输出路径和 K8s 版本
  sudo python3 build_k8s_base_image.py \\
    --source /var/lib/libvirt/images/ubuntu-22.04-cloud-docker.qcow2 \\
    --output /var/lib/libvirt/images/ubuntu-22.04-cloud-docker-k8s.qcow2 \\
    --k8s-version 1.28.0
        """
    )
    
    parser.add_argument(
        '--source',
        required=True,
        help='源镜像路径（应该已经包含 Docker）'
    )
    
    parser.add_argument(
        '--output',
        help='输出镜像路径（如果未指定，自动生成）'
    )
    
    parser.add_argument(
        '--k8s-version',
        default='1.28.0',
        help='Kubernetes 版本（默认: 1.28.0）'
    )
    
    parser.add_argument(
        '--user',
        default='ubuntu',
        help='用户名（默认: ubuntu）'
    )
    
    args = parser.parse_args()
    
    try:
        builder = K8sBaseImageBuilder(
            source_image=args.source,
            output_image=args.output,
            k8s_version=args.k8s_version
        )
        
        success = builder.build(user=args.user)
        sys.exit(0 if success else 1)
        
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()

