# K8s 集群环境搭建指南

本文档说明如何搭建 K8s 集群环境，包括依赖分析和镜像构建。

## 一、K8s 集群架构

根据 `vm-config.yaml` 配置：
- **40 个 K8s 集群**
- **每个集群包含**：
  - 1 个 Master 节点
  - 2 个 Worker 节点
- **总计**：120 台虚拟机（40 master + 80 worker）

## 二、依赖分析

### 2.1 必需组件

#### 1. **容器运行时（Container Runtime）**
- **containerd**（推荐，K8s 1.24+ 默认）
  - 版本：最新稳定版
  - 用途：运行容器
  - 配置：使用 systemd cgroup driver

- **Docker**（可选，如果使用 Docker 作为运行时）
  - 已在基础镜像中安装
  - 需要配置为使用 systemd cgroup driver

#### 2. **Kubernetes 核心组件**
- **kubeadm**
  - 版本：1.28.0（可配置）
  - 用途：初始化集群和管理节点

- **kubelet**
  - 版本：1.28.0（与 kubeadm 版本一致）
  - 用途：节点代理，管理 Pod 和容器

- **kubectl**
  - 版本：1.28.0（与 kubeadm 版本一致）
  - 用途：命令行工具，与集群交互

#### 3. **系统配置**

##### 内核模块
- `overlay`：用于 overlay 文件系统
- `br_netfilter`：用于网络桥接和过滤

##### 内核参数（sysctl）
```bash
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
```

##### Swap 禁用
- K8s 要求禁用 swap
- 在 `/etc/fstab` 中注释掉 swap 行

##### Cgroup 驱动
- 使用 `systemd` cgroup driver（与 Docker/containerd 一致）

#### 4. **网络插件（可选，在集群初始化后安装）**
- **Flannel**（推荐，简单易用）
- **Calico**（功能更强大，支持网络策略）
- **Weave Net**（其他选择）

### 2.2 端口要求

#### Master 节点
- **6443**：Kubernetes API server
- **2379-2380**：etcd server client API（如果使用外部 etcd）
- **10250**：Kubelet API
- **10259**：kube-scheduler
- **10257**：kube-controller-manager

#### Worker 节点
- **10250**：Kubelet API
- **30000-32767**：NodePort Services（可选）

### 2.3 系统要求

- **操作系统**：Ubuntu 22.04 LTS
- **CPU**：至少 2 核（Master），1 核（Worker）
- **内存**：至少 2GB（Master），1GB（Worker）
- **磁盘**：至少 20GB（Master），20GB（Worker）

## 三、镜像构建

### 3.1 构建 K8s 基础镜像

使用 `build_k8s_base_image.py` 脚本构建预装 K8s 依赖的镜像：

```bash
# 需要 root 权限
sudo python3 deploy/build_k8s_base_image.py \
  --source /var/lib/libvirt/images/ubuntu-22.04-cloud-docker.qcow2 \
  --output /var/lib/libvirt/images/ubuntu-22.04-cloud-docker-k8s.qcow2 \
  --k8s-version 1.28.0
```

**脚本功能**：
1. 复制源镜像（应已包含 Docker）
2. 安装 containerd
3. 安装 kubeadm, kubelet, kubectl（指定版本）
4. 配置系统参数（内核模块、sysctl、swap）
5. 创建首次启动初始化脚本
6. 配置 kubelet 使用 systemd cgroup driver
7. 优化镜像大小

**注意事项**：
- 需要网络连接（下载包和 GPG 密钥）
- 使用阿里云镜像源加速下载
- 首次启动时会自动加载内核模块和应用 sysctl 参数

### 3.2 更新 vm-config.yaml

构建完成后，更新 `vm-config.yaml` 中的基础镜像路径：

```yaml
global:
  base_image: "/var/lib/libvirt/images/ubuntu-22.04-cloud-docker-k8s.qcow2"
```

## 四、集群初始化

### 4.1 Master 节点初始化

在 Master 节点上执行：

```bash
# 1. 加载内核模块和应用 sysctl（如果未自动执行）
sudo modprobe overlay
sudo modprobe br_netfilter
sudo sysctl --system

# 2. 禁用 swap（如果已启用）
sudo swapoff -a
sudo sed -i '/ swap / s/^/#/' /etc/fstab

# 3. 初始化集群
sudo kubeadm init \
  --pod-network-cidr=10.244.0.0/16 \
  --apiserver-advertise-address=<MASTER_IP> \
  --control-plane-endpoint=<MASTER_IP>:6443

# 4. 配置 kubectl（普通用户）
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config

# 5. 安装网络插件（Flannel）
kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
```

### 4.2 Worker 节点加入集群

在 Worker 节点上执行：

```bash
# 1. 加载内核模块和应用 sysctl（如果未自动执行）
sudo modprobe overlay
sudo modprobe br_netfilter
sudo sysctl --system

# 2. 禁用 swap（如果已启用）
sudo swapoff -a
sudo sed -i '/ swap / s/^/#/' /etc/fstab

# 3. 使用 Master 节点输出的 join 命令加入集群
sudo kubeadm join <MASTER_IP>:6443 \
  --token <TOKEN> \
  --discovery-token-ca-cert-hash sha256:<HASH>
```

### 4.3 验证集群

在 Master 节点上执行：

```bash
# 查看节点状态
kubectl get nodes

# 查看所有 Pod 状态
kubectl get pods --all-namespaces
```

## 五、自动化脚本

### 5.1 集群初始化脚本（待实现）

计划创建以下脚本：
- `init_k8s_cluster.py`：自动化初始化 K8s 集群
  - 初始化 Master 节点
  - 配置 Worker 节点加入集群
  - 安装网络插件
  - 验证集群状态

## 六、常见问题

### 6.1 containerd 未启动

```bash
sudo systemctl enable containerd
sudo systemctl start containerd
```

### 6.2 kubelet 未启动

```bash
sudo systemctl enable kubelet
sudo systemctl start kubelet
```

### 6.3 网络插件未就绪

检查 Flannel Pod 状态：
```bash
kubectl get pods -n kube-flannel
```

### 6.4 节点 NotReady

检查 kubelet 日志：
```bash
sudo journalctl -u kubelet -f
```

## 七、参考资源

- [Kubernetes 官方文档 - 安装 kubeadm](https://kubernetes.io/zh-cn/docs/setup/production-environment/tools/kubeadm/install-kubeadm/)
- [Kubernetes 官方文档 - 使用 kubeadm 创建集群](https://kubernetes.io/zh-cn/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/)
- [Flannel 官方文档](https://github.com/flannel-io/flannel)

