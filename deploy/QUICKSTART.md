# 快速使用指南

## 步骤1: 安装依赖

```bash
cd deploy
./install_dependencies.sh
```

**重要**: 安装完成后需要重新登录以使libvirt组权限生效。

## 步骤2: 准备基础镜像

下载Ubuntu 22.04 Cloud Image：

```bash
# 创建镜像目录
sudo mkdir -p /var/lib/libvirt/images

# 下载Ubuntu 22.04 Cloud Image
cd /var/lib/libvirt/images
sudo wget https://cloud-images.ubuntu.com/releases/22.04/release/ubuntu-22.04-server-cloudimg-amd64.img

# 重命名为配置文件中指定的名称
sudo mv ubuntu-22.04-server-cloudimg-amd64.img ubuntu-22.04-cloud.qcow2
```

## 步骤3: 创建libvirt网络

```bash
sudo ./create_network.sh
```

## 步骤4: 创建虚拟机

```bash
# 创建所有虚拟机（200台）
python3 create_vms.py

# 或者只创建特定类型
python3 create_vms.py --type iarnet    # 只创建20个iarnet节点
python3 create_vms.py --type docker   # 只创建60个docker节点
python3 create_vms.py --type k8s      # 只创建120个k8s节点（40个集群）
```

## 步骤5: 验证虚拟机

```bash
# 查看所有虚拟机状态
virsh list --all

# 查看特定虚拟机信息
virsh dominfo vm-iarnet-01

# 连接到虚拟机控制台
virsh console vm-iarnet-01
```

## 步骤6: 访问虚拟机

虚拟机创建后，cloud-init会自动配置网络和SSH密钥。等待几分钟后即可通过SSH访问：

```bash
# SSH连接到虚拟机（使用配置的用户名）
ssh ubuntu@192.168.100.10  # iarnet节点示例
ssh ubuntu@192.168.100.50  # docker节点示例
ssh ubuntu@192.168.100.220 # k8s master节点示例
```

## 删除虚拟机

如果需要删除所有虚拟机：

```bash
# 删除所有虚拟机（保留磁盘）
python3 delete_vms.py

# 删除所有虚拟机（包括磁盘镜像）
python3 delete_vms.py --delete-disk --yes
```

## 常见问题

### 1. 权限错误
```
错误: 无法连接到libvirt守护进程
```
**解决**: 确保已运行 `sudo usermod -aG libvirt $USER` 并重新登录

### 2. 基础镜像不存在
```
错误: 基础镜像不存在: /var/lib/libvirt/images/ubuntu-22.04-cloud.qcow2
```
**解决**: 按照步骤2下载基础镜像

### 3. 网络不存在
```
错误: 网络 'iarnet-network' 不存在
```
**解决**: 运行 `sudo ./create_network.sh` 创建网络

### 4. 磁盘空间不足
**解决**: 每台虚拟机需要20-30GB空间，确保有足够的磁盘空间

## 虚拟机清单

创建完成后，你将拥有：

- **20个iarnet节点**: vm-iarnet-01 到 vm-iarnet-20
  - IP: 192.168.100.10 - 192.168.100.29
  
- **60个Docker节点**: vm-docker-01 到 vm-docker-60
  - IP: 192.168.100.50 - 192.168.100.109
  
- **40个K8s集群**（每个集群1 master + 2 worker = 120台）:
  - Master: vm-k8s-cluster-01-master 到 vm-k8s-cluster-40-master
  - Worker: vm-k8s-cluster-01-worker-1/2 到 vm-k8s-cluster-40-worker-1/2
  - IP: 192.168.100.220 开始，每个集群占用3个IP

**总计**: 200台虚拟机

