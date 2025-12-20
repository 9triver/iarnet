#!/bin/bash
#
# K8s 集群节点依赖安装脚本
# 适用于 Ubuntu 22.04 LTS
# 
# 使用方法:
#   sudo bash install_k8s_dependencies.sh [master|worker]
#
# 注意: Master 和 Worker 节点需要安装相同的依赖
# 区别在于 Master 节点需要初始化集群，Worker 节点需要加入集群

set -e

NODE_TYPE=${1:-"worker"}  # 默认为 worker

echo "=========================================="
echo "K8s 节点依赖安装脚本"
echo "节点类型: $NODE_TYPE"
echo "=========================================="

# 检查是否为 root
if [ "$EUID" -ne 0 ]; then 
    echo "错误: 请使用 sudo 运行此脚本"
    exit 1
fi

# 检查操作系统
if [ ! -f /etc/os-release ]; then
    echo "错误: 无法检测操作系统"
    exit 1
fi

. /etc/os-release
if [ "$ID" != "ubuntu" ]; then
    echo "警告: 此脚本专为 Ubuntu 设计，当前系统: $ID"
fi

echo ""
echo "步骤 1: 配置系统参数"
echo "----------------------------------------"

# 1.1 禁用 swap
echo "1.1 禁用 swap..."
swapoff -a
sed -i '/ swap / s/^/#/' /etc/fstab
echo "  ✓ Swap 已禁用"

# 1.2 加载内核模块
echo "1.2 加载内核模块..."
modprobe overlay
modprobe br_netfilter

# 确保模块在启动时自动加载
cat > /etc/modules-load.d/k8s.conf << 'EOF'
overlay
br_netfilter
EOF
echo "  ✓ 内核模块配置完成"

# 1.3 配置 sysctl 参数
echo "1.3 配置 sysctl 参数..."
cat > /etc/sysctl.d/k8s.conf << 'EOF'
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

# 应用 sysctl 参数（忽略不支持的参数）
sysctl --system 2>/dev/null || {
    # 如果 --system 失败，手动应用支持的参数
    sysctl -w net.bridge.bridge-nf-call-iptables=1 2>/dev/null || true
    sysctl -w net.bridge.bridge-nf-call-ip6tables=1 2>/dev/null || true
    sysctl -w net.ipv4.ip_forward=1 2>/dev/null || true
}
echo "  ✓ Sysctl 参数配置完成"

echo ""
echo "步骤 2: 安装容器运行时 (containerd)"
echo "----------------------------------------"

# 2.0 清理可能存在的旧 Kubernetes 配置（避免干扰）
echo "  清理旧配置..."
rm -f /etc/apt/sources.list.d/kubernetes.list
rm -f /etc/apt/keyrings/kubernetes-apt-keyring.gpg

# 2.1 更新包列表
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq

# 2.2 安装基础依赖
apt-get install -y -qq ca-certificates curl gnupg lsb-release apt-transport-https

# 2.3 配置 Docker/containerd 仓库
mkdir -p /etc/apt/keyrings

# 添加 Docker GPG 密钥（使用官方源）
echo "  添加 Docker 官方 GPG 密钥..."
# 先删除可能存在的旧文件
rm -f /etc/apt/keyrings/docker.gpg
# 下载并添加 GPG 密钥
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --batch --no-tty --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

# 设置 Docker 仓库（使用官方源）
UBUNTU_CODENAME=$(lsb_release -cs)
cat > /etc/apt/sources.list.d/docker.list << EOF
deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $UBUNTU_CODENAME stable
EOF

# 2.4 更新包列表并安装 containerd
apt-get update -qq
apt-get install -y -qq containerd.io

# 2.5 配置 containerd 使用 systemd cgroup driver
mkdir -p /etc/containerd
containerd config default | sed 's/SystemdCgroup = false/SystemdCgroup = true/' > /etc/containerd/config.toml

# 重启 containerd
systemctl restart containerd
systemctl enable containerd

echo "  ✓ Containerd 安装完成"

echo ""
echo "步骤 3: 安装 Kubernetes 组件"
echo "----------------------------------------"

# 3.1 添加 Kubernetes GPG 密钥（使用官方源）
echo "  配置 Kubernetes GPG 密钥（使用官方源）..."

# 先删除可能存在的旧配置
rm -f /etc/apt/sources.list.d/kubernetes.list
rm -f /etc/apt/keyrings/kubernetes-apt-keyring.gpg

mkdir -p /etc/apt/keyrings

# 使用官方 Kubernetes 源
echo "  添加官方 Kubernetes GPG 密钥..."
# 先删除可能存在的旧文件
rm -f /etc/apt/keyrings/kubernetes-apt-keyring.gpg
if curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key | gpg --batch --no-tty --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg 2>/dev/null; then
    chmod a+r /etc/apt/keyrings/kubernetes-apt-keyring.gpg
    echo "  ✓ 官方 GPG 密钥配置成功"
    cat > /etc/apt/sources.list.d/kubernetes.list << 'EOF'
deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /
EOF
else
    # 备用方案：使用 apt-key（Ubuntu 22.04 仍支持）
    echo "  尝试使用 apt-key 添加官方密钥..."
    curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key | apt-key add - 2>/dev/null
    cat > /etc/apt/sources.list.d/kubernetes.list << 'EOF'
deb https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /
EOF
    echo "  ✓ 使用 apt-key 方式配置成功"
fi

# 3.3 更新包列表并安装 K8s 组件
K8S_VERSION="1.28.0-00"
apt-get update -qq

apt-get install -y -qq kubelet=$K8S_VERSION kubeadm=$K8S_VERSION kubectl=$K8S_VERSION || {
    echo "  指定版本安装失败，尝试安装最新版本..."
    apt-get install -y -qq kubelet kubeadm kubectl
}

# 3.4 锁定版本，防止自动升级
apt-mark hold kubelet kubeadm kubectl

# 3.5 配置 kubelet 使用 systemd cgroup driver
mkdir -p /var/lib/kubelet
cat > /var/lib/kubelet/config.yaml << 'EOF'
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
cgroupDriver: systemd
EOF

# 3.6 启用 kubelet（但不启动，需要先初始化集群）
systemctl enable kubelet

echo "  ✓ Kubernetes 组件安装完成"
echo "    已安装: kubelet, kubeadm, kubectl"

echo ""
echo "步骤 4: 验证安装"
echo "----------------------------------------"

# 检查 containerd
if systemctl is-active --quiet containerd; then
    echo "  ✓ Containerd 运行正常"
else
    echo "  ✗ Containerd 未运行"
fi

# 检查 kubelet
if systemctl is-enabled --quiet kubelet; then
    echo "  ✓ Kubelet 已启用"
else
    echo "  ✗ Kubelet 未启用"
fi

# 显示版本信息
echo ""
echo "已安装版本:"
kubelet --version | sed 's/^/  /'
kubeadm version -o short | sed 's/^/  /'
kubectl version --client --short 2>/dev/null | sed 's/^/  /' || echo "  kubectl: 已安装"

echo ""
echo "=========================================="
echo "依赖安装完成！"
echo "=========================================="
echo ""
echo "下一步操作:"
echo ""

if [ "$NODE_TYPE" = "master" ]; then
    echo "【Master 节点】"
    echo "1. 初始化集群:"
    echo "   sudo kubeadm init \\"
    echo "     --pod-network-cidr=10.244.0.0/16 \\"
    echo "     --apiserver-advertise-address=<MASTER_IP> \\"
    echo "     --control-plane-endpoint=<MASTER_IP>:6443"
    echo ""
    echo "2. 配置 kubectl:"
    echo "   mkdir -p \$HOME/.kube"
    echo "   sudo cp -i /etc/kubernetes/admin.conf \$HOME/.kube/config"
    echo "   sudo chown \$(id -u):\$(id -g) \$HOME/.kube/config"
    echo ""
    echo "3. 安装网络插件 (Flannel):"
    echo "   kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml"
    echo ""
    echo "4. 获取 join 命令（用于 worker 节点加入）:"
    echo "   kubeadm token create --print-join-command"
else
    echo "【Worker 节点】"
    echo "1. 等待 Master 节点初始化完成"
    echo "2. 在 Master 节点上获取 join 命令:"
    echo "   kubeadm token create --print-join-command"
    echo "3. 在 Worker 节点上执行 join 命令（需要 sudo）"
fi

echo ""
echo "注意事项:"
echo "- 确保所有节点的 /etc/hosts 包含所有节点的 IP 和主机名映射"
echo "- 确保防火墙允许必要的端口（6443, 10250, 10259, 10257 等）"
echo "- Master 节点需要至少 2GB 内存和 2 CPU 核心"
echo "- Worker 节点需要至少 1GB 内存和 1 CPU 核心"

