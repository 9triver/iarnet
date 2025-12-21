# K8s 集群初始化脚本使用说明

## 概述

`init_k8s_clusters.py` 是一个自动化脚本，用于一键初始化所有或指定的 Kubernetes 集群。脚本会自动完成以下操作：

1. **Master 节点初始化**
   - 执行 `kubeadm init` 初始化集群
   - 配置 `kubectl` 访问权限
   - 安装网络插件（Flannel）
   - 获取 worker 节点加入命令

2. **Worker 节点加入**
   - 在每个 worker 节点上执行 `kubeadm join`
   - 验证节点成功加入集群

3. **集群状态验证**
   - 检查所有节点状态
   - 输出集群初始化结果

## 前置条件

### 1. 虚拟机已创建并运行

确保所有 K8s 节点（master 和 worker）虚拟机已经创建并正常运行：

```bash
# 检查虚拟机状态
virsh list | grep k8s-cluster

# 确保所有节点可以通过 SSH 连接
ssh ubuntu@<master-ip>
ssh ubuntu@<worker-ip>
```

### 2. K8s 依赖已安装

确保所有节点上已安装 K8s 依赖（containerd, kubeadm, kubelet, kubectl）：

```bash
# 如果尚未安装，请先运行安装脚本
# 参考: install_k8s_dependencies.sh 或 K8S_DEPENDENCIES.md
```

### 3. 配置文件准备

确保 `vm-config.yaml` 配置文件存在且正确配置了：
- K8s 集群数量
- Master 和 Worker 节点的 IP 地址
- SSH 密钥路径
- Pod CIDR（可选，脚本会自动生成）

### 4. SSH 密钥配置

确保可以通过 SSH 密钥无密码登录到所有节点：

```bash
# 检查 SSH 密钥是否存在
ls -la ~/.ssh/id_rsa ~/.ssh/id_ed25519

# 测试 SSH 连接
ssh -i ~/.ssh/id_rsa ubuntu@<node-ip>
```

## 使用方法

### 基本用法

#### 1. 初始化所有集群

```bash
cd /home/zhangyx/iarnet/deploy
python3 init_k8s_clusters.py
```

这会初始化 `vm-config.yaml` 中配置的所有 K8s 集群。

#### 2. 初始化指定范围的集群

```bash
# 初始化集群 1-10（注意：这里使用的是 1-based 索引）
python3 init_k8s_clusters.py --clusters 1-10
```

#### 3. 初始化指定的几个集群

```bash
# 初始化集群 1, 2, 3
python3 init_k8s_clusters.py --clusters 1,2,3
```

#### 4. 指定配置文件

```bash
# 使用自定义配置文件
python3 init_k8s_clusters.py --vm-config /path/to/custom-vm-config.yaml
```

#### 5. 调整并发数

```bash
# 同时初始化 5 个集群（默认是 3）
python3 init_k8s_clusters.py --max-workers 5
```

**注意**：不建议设置过大的并发数，因为：
- 每个集群初始化需要较长时间（5-10 分钟）
- 过多并发可能导致资源竞争
- 建议值：3-5

### 完整参数说明

```bash
python3 init_k8s_clusters.py [选项]

选项:
  --vm-config PATH     虚拟机配置文件路径（默认: vm-config.yaml）
  --clusters RANGE     集群范围，格式: start-end 或逗号分隔的列表
                       例如: 1-10 或 1,2,3,5-8
                       注意：使用 1-based 索引（集群编号从 1 开始）
  --max-workers N      最大并发线程数（默认: 3）
  -h, --help           显示帮助信息
```

## 执行流程

脚本执行时会按以下流程进行：

```
1. 读取配置文件
   └─> 解析集群配置、IP 地址、SSH 密钥等

2. 并行初始化多个集群（根据 max-workers 设置）
   └─> 对每个集群：
       ├─> 检查 master 节点连通性
       ├─> 执行 kubeadm init
       ├─> 配置 kubectl
       ├─> 安装 Flannel 网络插件
       ├─> 获取 join 命令
       ├─> 等待 master 节点就绪
       ├─> Worker 节点依次加入集群
       └─> 验证集群状态

3. 输出初始化结果
   └─> 显示成功/失败的集群列表
```

## 输出示例

### 正常执行输出

```
开始初始化 40 个 K8s 集群...
============================================================

[集群 01] 开始初始化集群...
[集群 01] Master: vm-k8s-cluster-01-master (192.168.100.110)
[集群 01] Workers: 2 个
[集群 01] Pod CIDR: 10.240.0.0/16
[集群 01] 初始化 master 节点: vm-k8s-cluster-01-master (192.168.100.110)
[集群 01]   执行 kubeadm init...
[集群 01]   ✓ kubeadm init 成功
[集群 01]   配置 kubectl...
[集群 01]   ✓ kubectl 配置完成
[集群 01]   安装网络插件 (Flannel)...
[集群 01]   ✓ Flannel 安装成功
[集群 01]   等待 master 节点就绪...
[集群 01]   ✓ Master 节点已就绪
[集群 01] Worker vm-k8s-cluster-01-worker-1 加入集群...
[集群 01] Worker vm-k8s-cluster-01-worker-1 ✓ 加入集群成功
[集群 01] Worker vm-k8s-cluster-01-worker-2 加入集群...
[集群 01] Worker vm-k8s-cluster-01-worker-2 ✓ 加入集群成功
[集群 01]   验证集群状态...
[集群 01]   集群节点状态:
[集群 01]     NAME                          STATUS   ROLES           AGE   VERSION
[集群 01]     vm-k8s-cluster-01-master      Ready    control-plane   2m    v1.28.0
[集群 01]     vm-k8s-cluster-01-worker-1   Ready    <none>          1m    v1.28.0
[集群 01]     vm-k8s-cluster-01-worker-2   Ready    <none>          1m    v1.28.0
[集群 01] ✓ 集群初始化完成（2/2 workers 已加入）

============================================================
集群初始化完成
  成功: 40 个集群
  失败: 0 个集群
```

### 部分失败输出

```
============================================================
集群初始化完成
  成功: 38 个集群
  失败: 2 个集群

失败的集群: [3, 15]
```

## 故障排查

### 1. SSH 连接失败

**错误信息**：
```
[集群 XX] ✗ 无法连接到 master 节点
```

**解决方法**：
- 检查虚拟机是否运行：`virsh list | grep k8s-cluster`
- 检查网络连通性：`ping <node-ip>`
- 检查 SSH 密钥配置：`ssh -i ~/.ssh/id_rsa ubuntu@<node-ip>`
- 确认 `vm-config.yaml` 中的 IP 地址和 SSH 密钥路径正确

### 2. kubeadm init 失败

**错误信息**：
```
[集群 XX] ✗ kubeadm init 失败
[集群 XX]   错误: ...
```

**解决方法**：
- 检查节点上是否已安装 K8s 依赖
- 检查节点是否已有集群配置（如果之前初始化过，需要先重置）：
  ```bash
  ssh ubuntu@<master-ip>
  sudo kubeadm reset --force
  ```
- 检查节点资源是否充足（CPU、内存、磁盘）
- 查看详细错误信息，根据错误提示处理

### 3. Worker 节点无法加入

**错误信息**：
```
[集群 XX] Worker xxx ✗ 加入集群失败
```

**解决方法**：
- 检查 master 节点是否已成功初始化
- 检查 worker 节点与 master 节点的网络连通性
- 检查 join 命令是否有效（token 可能过期）：
  ```bash
  ssh ubuntu@<master-ip>
  kubeadm token create --print-join-command
  ```
- 检查 worker 节点上是否已安装 K8s 依赖
- 如果 worker 节点之前尝试加入过，需要先重置：
  ```bash
  ssh ubuntu@<worker-ip>
  sudo kubeadm reset --force
  ```

### 4. Flannel 安装失败

**错误信息**：
```
[集群 XX] ⚠ Flannel 安装可能失败: ...
```

**解决方法**：
- 检查 master 节点网络连接（需要访问 GitHub）
- 手动安装 Flannel：
  ```bash
  ssh ubuntu@<master-ip>
  kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
  ```
- 检查 Pod CIDR 配置是否正确

### 5. 集群状态验证失败

**错误信息**：
```
[集群 XX] ⚠ 集群部分初始化（1/2 workers 已加入）
```

**解决方法**：
- 等待一段时间后再次检查（节点加入需要时间）
- 手动检查节点状态：
  ```bash
  ssh ubuntu@<master-ip>
  kubectl get nodes
  kubectl get pods --all-namespaces
  ```
- 检查未加入的 worker 节点日志：
  ```bash
  ssh ubuntu@<worker-ip>
  sudo journalctl -u kubelet -n 50
  ```

## 手动验证集群

初始化完成后，可以手动验证集群状态：

```bash
# 连接到 master 节点
ssh ubuntu@<master-ip>

# 查看节点状态
kubectl get nodes

# 查看所有 Pod 状态
kubectl get pods --all-namespaces

# 查看集群信息
kubectl cluster-info
```

## 重新初始化集群

如果集群初始化失败，需要先重置节点：

### Master 节点重置

```bash
ssh ubuntu@<master-ip>
sudo kubeadm reset --force
sudo rm -rf /etc/cni/net.d
sudo rm -rf /var/lib/etcd
sudo rm -rf ~/.kube
```

### Worker 节点重置

```bash
ssh ubuntu@<worker-ip>
sudo kubeadm reset --force
```

然后重新运行初始化脚本。

## 注意事项

1. **初始化时间**：每个集群初始化大约需要 5-10 分钟，请耐心等待

2. **并发控制**：不建议设置过大的 `--max-workers`，建议值：3-5

3. **网络要求**：
   - 所有节点之间网络互通
   - Master 节点需要访问互联网（下载 Flannel 镜像）

4. **资源要求**：
   - Master 节点：至少 2 CPU、2GB 内存
   - Worker 节点：至少 1 CPU、1GB 内存

5. **Pod CIDR**：
   - 如果配置文件中未指定 `k8s_pod_cidrs`，脚本会自动生成
   - 自动生成的 CIDR 格式：`10.{240+cluster_id}.0.0/16`
   - 确保不同集群的 Pod CIDR 不冲突

6. **重复初始化**：
   - 如果节点已经初始化过，需要先执行 `kubeadm reset` 重置
   - 脚本不会自动检测和重置已初始化的节点

## 相关文档

- [K8s 依赖安装说明](K8S_DEPENDENCIES.md)
- [K8s Provider 部署说明](../providers/k8s/README.md)
- [虚拟机创建脚本](create_vms.py)
- [虚拟机配置文件](vm-config.yaml)

