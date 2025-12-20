# K8s 集群节点依赖安装指南

本文档说明在 Ubuntu 22.04 LTS 虚拟机上安装 K8s 集群所需依赖的详细步骤。

## 一、依赖概览

### Master 和 Worker 节点需要安装相同的依赖：

1. **系统配置**
   - 禁用 swap
   - 加载内核模块（overlay, br_netfilter）
   - 配置 sysctl 参数

2. **容器运行时**
   - containerd（K8s 1.24+ 默认使用）

3. **Kubernetes 组件**
   - kubelet（节点代理）
   - kubeadm（集群管理工具）
   - kubectl（命令行工具）

### Master 和 Worker 的区别：

- **Master 节点**：需要执行 `kubeadm init` 初始化集群
- **Worker 节点**：需要执行 `kubeadm join` 加入集群

## 二、快速安装

### 方法 1: 使用自动化脚本（推荐）

```bash
# 在 Master 节点上
sudo bash install_k8s_dependencies.sh master

# 在 Worker 节点上
sudo bash install_k8s_dependencies.sh worker
```

### 方法 2: 手动执行命令

参考下面的详细步骤。

## 三、详细安装步骤

### 步骤 1: 配置系统参数

```bash
# 1.1 禁用 swap
sudo swapoff -a
sudo sed -i '/ swap / s/^/#/' /etc/fstab

# 1.2 加载内核模块
sudo modprobe overlay
sudo modprobe br_netfilter

# 确保模块在启动时自动加载
cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

# 1.3 配置 sysctl 参数
cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

# 应用 sysctl 参数
sudo sysctl --system
```

### 步骤 2: 安装 containerd

```bash
# 2.1 更新包列表
sudo apt-get update

# 2.2 安装基础依赖
sudo apt-get install -y ca-certificates curl gnupg lsb-release apt-transport-https

# 2.3 配置 Docker/containerd 仓库
sudo mkdir -p /etc/apt/keyrings

# 添加 Docker GPG 密钥（使用阿里云镜像）
curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg | \
    sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg

sudo chmod a+r /etc/apt/keyrings/docker.gpg

# 设置 Docker 仓库
UBUNTU_CODENAME=$(lsb_release -cs)
echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.gpg] \
    https://mirrors.aliyun.com/docker-ce/linux/ubuntu $UBUNTU_CODENAME stable" | \
    sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 2.4 安装 containerd
sudo apt-get update
sudo apt-get install -y containerd.io

# 2.5 配置 containerd 使用 systemd cgroup driver
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml > /dev/null
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml

# 重启 containerd
sudo systemctl restart containerd
sudo systemctl enable containerd
```

### 步骤 3: 安装 Kubernetes 组件

```bash
# 3.1 添加 Kubernetes GPG 密钥（使用阿里云镜像）
curl -fsSL https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg | \
    sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

sudo chmod a+r /etc/apt/keyrings/kubernetes-apt-keyring.gpg

# 3.2 设置 Kubernetes 仓库
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] \
    https://mirrors.aliyun.com/kubernetes/apt/ kubernetes-xenial main" | \
    sudo tee /etc/apt/sources.list.d/kubernetes.list

# 3.3 安装 K8s 组件（指定版本 1.28.0）
sudo apt-get update
sudo apt-get install -y kubelet=1.28.0-00 kubeadm=1.28.0-00 kubectl=1.28.0-00

# 锁定版本，防止自动升级
sudo apt-mark hold kubelet kubeadm kubectl

# 3.4 配置 kubelet 使用 systemd cgroup driver
sudo mkdir -p /var/lib/kubelet
cat <<EOF | sudo tee /var/lib/kubelet/config.yaml
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
cgroupDriver: systemd
EOF

# 3.5 启用 kubelet（但不启动，需要先初始化集群）
sudo systemctl enable kubelet
```

### 步骤 4: 验证安装

```bash
# 检查 containerd
sudo systemctl status containerd

# 检查 kubelet
sudo systemctl status kubelet

# 查看版本
kubelet --version
kubeadm version -o short
kubectl version --client --short
```

## 四、Master 节点初始化

在所有节点安装完依赖后，在 Master 节点上执行：

```bash
# 1. 初始化集群
sudo kubeadm init \
  --pod-network-cidr=10.244.0.0/16 \
  --apiserver-advertise-address=<MASTER_IP> \
  --control-plane-endpoint=<MASTER_IP>:6443

# 2. 配置 kubectl（普通用户）
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config

# 3. 安装网络插件（Flannel）
kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml

# 4. 获取 join 命令（用于 worker 节点加入）
kubeadm token create --print-join-command
```

## 五、Worker 节点加入集群

在 Worker 节点上执行 Master 节点输出的 join 命令：

```bash
# 示例（实际命令从 Master 节点获取）
sudo kubeadm join <MASTER_IP>:6443 \
  --token <TOKEN> \
  --discovery-token-ca-cert-hash sha256:<HASH>
```

## 六、验证集群

在 Master 节点上执行：

```bash
# 查看节点状态
kubectl get nodes

# 查看所有 Pod 状态
kubectl get pods --all-namespaces
```

## 七、依赖清单总结

### 系统配置
- [x] 禁用 swap
- [x] 加载内核模块（overlay, br_netfilter）
- [x] 配置 sysctl 参数

### 容器运行时
- [x] containerd
- [x] containerd 配置（systemd cgroup driver）

### Kubernetes 组件
- [x] kubelet (1.28.0)
- [x] kubeadm (1.28.0)
- [x] kubectl (1.28.0)

### 网络插件（集群初始化后安装）
- [ ] Flannel 或 Calico

## 八、常见问题

### 1. DNS 解析失败

如果无法访问阿里云镜像，可以尝试：
- 使用官方源（速度可能较慢）
- 配置代理
- 使用其他国内镜像源（如清华、中科大）

### 2. containerd 未启动

```bash
sudo systemctl restart containerd
sudo systemctl enable containerd
```

### 3. kubelet 未启动

kubelet 在集群初始化前不会启动，这是正常的。初始化集群后会自动启动。

### 4. 端口被占用

确保以下端口未被占用：
- Master: 6443, 2379-2380, 10250, 10259, 10257
- Worker: 10250

## 九、导出镜像

安装完依赖后，可以使用 `export_vm_to_image.py` 脚本导出虚拟机为镜像：

```bash
# 导出 Master 节点镜像
sudo python3 deploy/export_vm_to_image.py \
  --vm-name vm-k8s-cluster-01-master \
  --output /var/lib/libvirt/images/ubuntu-22.04-cloud-docker-k8s-master.qcow2 \
  --cleanup-iarnet

# 导出 Worker 节点镜像
sudo python3 deploy/export_vm_to_image.py \
  --vm-name vm-k8s-cluster-01-worker-1 \
  --output /var/lib/libvirt/images/ubuntu-22.04-cloud-docker-k8s-worker.qcow2 \
  --cleanup-iarnet
```

注意：导出镜像前，建议：
1. 不要初始化集群（避免包含集群特定配置）
2. 清理临时文件和日志
3. 确保 kubelet 已禁用（未初始化前）

