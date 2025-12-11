#!/bin/bash
# 安装虚拟机创建脚本所需的所有依赖

set -e

echo "=========================================="
echo "安装虚拟机创建脚本依赖"
echo "=========================================="

# 检查是否为root用户
if [ "$EUID" -eq 0 ]; then 
   echo "请不要使用root用户运行此脚本"
   exit 1
fi

# 更新包列表
echo "更新包列表..."
sudo apt-get update

# 安装系统依赖
echo "安装系统依赖..."
sudo apt-get install -y \
    libvirt-dev \
    qemu-kvm \
    qemu-utils \
    genisoimage \
    python3-pip \
    python3-dev \
    libvirt-daemon-system \
    libvirt-clients \
    bridge-utils \
    python3-yaml \
    python3-libvirt

# 检查libvirt服务状态
echo "检查libvirt服务..."
if ! systemctl is-active --quiet libvirtd; then
    echo "启动libvirt服务..."
    sudo systemctl start libvirtd
    sudo systemctl enable libvirtd
fi

# 将用户添加到libvirt组
echo "将用户添加到libvirt组..."
sudo usermod -aG libvirt $USER

# Python依赖已通过apt安装（python3-yaml和python3-libvirt）
# 如果需要特定版本，可以使用pip安装
echo "检查Python依赖..."
python3 -c "import yaml; import libvirt" 2>/dev/null && {
    echo "Python依赖已就绪"
} || {
    echo "警告: Python依赖未正确安装，尝试使用pip安装..."
    pip3 install --user --break-system-packages -r requirements.txt || {
        echo "尝试使用sudo安装..."
        sudo pip3 install --break-system-packages -r requirements.txt
    }
}

# 检查基础镜像是否存在
BASE_IMAGE="/var/lib/libvirt/images/ubuntu-22.04-cloud.qcow2"
if [ ! -f "$BASE_IMAGE" ]; then
    echo ""
    echo "警告: 基础镜像不存在: $BASE_IMAGE"
    echo "请下载Ubuntu 22.04 Cloud Image并放置到该路径"
    echo "下载地址: https://cloud-images.ubuntu.com/releases/22.04/release/"
    echo ""
fi

# 检查SSH密钥是否存在
SSH_KEY="$HOME/.ssh/id_rsa.pub"
if [ ! -f "$SSH_KEY" ]; then
    echo ""
    echo "警告: SSH公钥不存在: $SSH_KEY"
    echo "是否要生成新的SSH密钥对？(y/n)"
    read -r response
    if [[ "$response" =~ ^([yY][eE][sS]|[yY])$ ]]; then
        ssh-keygen -t rsa -b 4096 -f "$HOME/.ssh/id_rsa" -N ""
        echo "SSH密钥已生成"
    fi
fi

echo ""
echo "=========================================="
echo "安装完成！"
echo "=========================================="
echo ""
echo "重要提示:"
echo "1. 请重新登录以使libvirt组权限生效"
echo "2. 确保基础镜像已下载: $BASE_IMAGE"
echo "3. 运行 ./create_network.sh 创建libvirt网络"
echo "4. 然后运行 python3 create_vms.py 创建虚拟机"
echo ""

