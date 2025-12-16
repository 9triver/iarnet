# 虚拟机批量创建脚本

使用 libvirt + qemu 批量创建虚拟机的工具脚本。

## 快速开始

### 一键安装依赖

```bash
cd deploy
./install_dependencies.sh
```

安装完成后，**请重新登录**以使libvirt组权限生效。

### 手动安装依赖

如果一键安装脚本不适用，可以手动安装：

```bash
# 安装系统依赖（包括Python包）
sudo apt-get update
sudo apt-get install -y \
    libvirt-dev \
    qemu-kvm \
    qemu-utils \
    genisoimage \
    python3-pip \
    python3-dev \
    libvirt-daemon-system \
    libvirt-clients \
    python3-yaml \
    python3-libvirt

# 如果需要特定版本的Python依赖，可以使用pip（Ubuntu 24.04+需要--break-system-packages）
pip3 install --user --break-system-packages -r requirements.txt

# 将用户添加到libvirt组（需要重新登录生效）
sudo usermod -aG libvirt $USER
```

**注意**: Ubuntu 24.04+ 引入了 `externally-managed-environment` 限制。脚本会优先使用apt安装的Python包（`python3-yaml`和`python3-libvirt`），这样可以避免该限制。如果apt包版本不满足要求，脚本会回退到使用pip安装。

3. **准备基础镜像**:
   - 下载 Ubuntu 22.04 Cloud Image
   - 放置到 `/var/lib/libvirt/images/ubuntu-22.04-cloud.qcow2`
   - 或修改 `vm-config.yaml` 中的 `base_image` 路径

4. **准备SSH密钥**:
   - 确保 `~/.ssh/id_rsa.pub` 存在
   - 或修改 `vm-config.yaml` 中的 `ssh_key_path`

5. **创建libvirt网络**:
```bash
# 创建 iarnet-network（如果不存在）
sudo virsh net-define <network-xml>
sudo virsh net-start iarnet-network
sudo virsh net-autostart iarnet-network
```

## 使用方法

### 创建所有虚拟机
```bash
python3 create_vms.py
```

### 只创建特定类型的虚拟机
```bash
# 只创建 iarnet 节点
python3 create_vms.py --type iarnet

# 只创建 Docker 节点
python3 create_vms.py --type docker

# 只创建 K8s 集群
python3 create_vms.py --type k8s
```

### 指定配置文件
```bash
python3 create_vms.py --config /path/to/vm-config.yaml
```

## 配置说明

配置文件 `vm-config.yaml` 包含以下配置：

- **global**: 全局配置（基础镜像路径、网络名称、SSH密钥等）
- **vm_types**: 虚拟机类型配置
  - **iarnet**: iarnet节点配置（20台）
  - **docker**: Docker节点配置（60台）
  - **k8s_clusters**: K8s集群配置（40个集群，每个集群1 master + 2 worker）
- **k8s_pod_cidrs**: K8s Pod网络CIDR分配（40个）

## 虚拟机命名规则

- **iarnet节点**: `vm-iarnet-01` 到 `vm-iarnet-20`
- **Docker节点**: `vm-docker-01` 到 `vm-docker-60`
- **K8s Master**: `vm-k8s-cluster-01-master` 到 `vm-k8s-cluster-40-master`
- **K8s Worker**: `vm-k8s-cluster-01-worker-1` 到 `vm-k8s-cluster-40-worker-2`

## IP地址分配

- **iarnet节点**: 192.168.100.10 - 192.168.100.29
- **Docker节点**: 192.168.100.50 - 192.168.100.109
- **K8s集群**: 192.168.100.220 开始，每个集群占用3个IP

## 注意事项

1. 脚本会检查虚拟机是否已存在，如果存在则跳过创建
2. 磁盘镜像会从基础镜像复制创建，首次创建可能需要较长时间
3. 确保有足够的磁盘空间（每台虚拟机需要20-30GB）
4. 确保libvirt网络已正确配置并启动
5. 虚拟机创建后需要等待cloud-init完成初始化才能使用

## 故障排查

1. **权限错误**: 
   - libvirt连接错误: 确保用户已加入libvirt组并重新登录
   - 磁盘创建权限错误: `/var/lib/libvirt/images` 目录需要写入权限
     ```bash
     # 解决方案1: 使用sudo运行脚本（推荐）
     sudo python3 create_vms.py
     
     # 解决方案2: 配置免密sudo（适合自动化）
     sudo visudo
     # 添加: your_username ALL=(ALL) NOPASSWD: /usr/bin/qemu-img
     
     # 解决方案3: 修改目录权限（需要root）
     sudo chmod 775 /var/lib/libvirt/images
     sudo chgrp libvirt /var/lib/libvirt/images
     ```

2. **网络错误**: 
   - libvirt网络未启动: 检查 `virsh net-list`，确保网络已启动
   - 虚拟机无法访问（No route to host）:
     ```bash
     # 检查虚拟机网络状态
     ./check_vm_network.sh <vm-name>
     
     # 检查虚拟机是否获得IP地址
     virsh console <vm-name>
     # 登录后执行: ip addr show
     
     # 如果IP地址未配置，检查cloud-init状态
     virsh console <vm-name>
     # 登录后执行: cloud-init status
     # 如果cloud-init未完成，等待几分钟后重试
     
     # 手动应用网络配置
     virsh console <vm-name>
     # 登录后执行:
     sudo netplan apply
     sudo systemctl restart systemd-networkd
     ```
   - 虚拟机网络配置错误: 如果看到 `Failed to start Wait for Network to be Configured` 错误
     ```bash
     # 方法1: 删除并重新创建虚拟机（推荐）
     virsh undefine <vm-name>
     sudo rm /var/lib/libvirt/images/<vm-name>.qcow2
     python3 create_vms.py
     
     # 方法2: 通过控制台手动修复
     virsh console <vm-name>
     # 登录后执行:
     sudo nano /etc/netplan/50-cloud-init.yaml
     sudo netplan apply
     
     # 方法3: 使用修复脚本
     ./fix_vm_network.sh <vm-name>
     ```

3. **磁盘空间不足**: 检查 `/var/lib/libvirt/images/` 目录空间

4. **镜像不存在**: 检查基础镜像路径是否正确

5. **IP地址超出范围**: 检查配置文件中的IP分配是否合理

## 创建网络

在创建虚拟机之前，需要先创建libvirt网络：

```bash
sudo ./create_network.sh
```

或者手动创建：

```bash
sudo virsh net-define <network-xml>
sudo virsh net-start iarnet-network
sudo virsh net-autostart iarnet-network
```

## 删除虚拟机

使用删除脚本批量删除虚拟机：

```bash
# 删除所有虚拟机（不删除磁盘）
python3 delete_vms.py

# 删除所有虚拟机（包括磁盘镜像）
python3 delete_vms.py --delete-disk

# 只删除特定类型的虚拟机
python3 delete_vms.py --type iarnet --delete-disk

# 跳过确认提示
python3 delete_vms.py --yes --delete-disk
```

## SSH连接到虚拟机

使用便捷的SSH连接脚本：

```bash
# 列出所有虚拟机
python3 ssh_vm.py --list

# 列出特定类型的虚拟机
python3 ssh_vm.py --list --type iarnet
python3 ssh_vm.py --list --type docker
python3 ssh_vm.py --list --type k8s-master

# 通过hostname连接
python3 ssh_vm.py vm-iarnet-01
python3 ssh_vm.py vm-docker-05
python3 ssh_vm.py vm-k8s-cluster-01-master
python3 ssh_vm.py vm-k8s-cluster-01-worker-1

# 通过IP地址连接
python3 ssh_vm.py 192.168.100.10

# 执行命令而不进入交互式shell
python3 ssh_vm.py vm-iarnet-01 "ls -la"
python3 ssh_vm.py vm-iarnet-01 "sudo systemctl status"

# 使用自定义SSH端口
python3 ssh_vm.py vm-iarnet-01 --port 2222

# 使用自定义用户
python3 ssh_vm.py vm-iarnet-01 --user root
```

## 管理虚拟机

```bash
# 查看所有虚拟机
virsh list --all

# 启动虚拟机
virsh start vm-iarnet-01

# 关闭虚拟机
virsh shutdown vm-iarnet-01

# 查看虚拟机信息
virsh dominfo vm-iarnet-01

# 查看虚拟机控制台
virsh console vm-iarnet-01
```

## 部署 iarnet 到节点

### 快速部署（推荐）

使用完整部署脚本，一键完成所有步骤：

```bash
# 从项目根目录运行
cd /home/zhangyx/iarnet

# 完整部署到节点 0-10（生成配置 + 安装依赖 + 本地构建 + 上传部署 + 启动服务）
./deploy/deploy_iarnet_complete.sh 0-10

# 或者分步执行
./deploy/deploy_iarnet_complete.sh 0-10 true true true
# 参数说明: 节点范围 安装依赖 本地构建 重启服务
```

### 分步部署

如果需要更多控制，可以分步执行：

### 分步部署

#### 步骤1: 生成配置文件

为每个节点生成独立的配置文件：

```bash
# 为节点 0-10 生成配置文件
python3 deploy/generate_iarnet_configs.py --nodes 0-10

# 为单个节点生成配置
python3 deploy/generate_iarnet_configs.py --nodes 0

# 为指定节点生成配置
python3 deploy/generate_iarnet_configs.py --nodes 0,1,2,5,10
```

配置文件会生成在 `deploy/iarnet-configs/` 目录下，每个节点一个文件：
- `config-node-00.yaml` - 节点0的配置
- `config-node-01.yaml` - 节点1的配置
- ...

#### 步骤2: 安装依赖（使用 Ansible）

在虚拟机上安装 Go、gRPC、ZeroMQ、Node.js、Docker 等依赖：

```bash
# 安装依赖到节点 0-10
python3 deploy/deploy_iarnet.py --nodes 0-10 --install-deps

# 注意: 需要输入 sudo 密码
```

或者手动使用 Ansible：

```bash
cd deploy/ansible
ansible-playbook -i inventory.ini playbooks/install-iarnet-deps.yml --become --ask-become-pass
```

**安装的依赖包括**：
- **Go 语言环境**: Go 1.25.0（优先使用国内镜像源）
- **gRPC 相关库**: libprotobuf-dev, protobuf-compiler, libgrpc-dev 等
- **ZeroMQ 库**: libzmq3-dev, libczmq-dev（开发库）和 libzmq5, libczmq4（运行时库）
- **Node.js 和 npm**: Node.js 20（用于前端项目）
- **Docker**: Docker Engine（可选，用于容器化部署）
- **其他运行时依赖**: ca-certificates, libssl-dev, libsqlite3-dev 等

#### 步骤3: 部署到节点

```bash
# 只上传配置文件（不构建、不重启）
python3 deploy/deploy_iarnet.py --nodes 0-10

# 本地构建二进制文件并上传（推荐）
python3 deploy/deploy_iarnet.py --nodes 0-10 --build

# 部署并重启服务
python3 deploy/deploy_iarnet.py --nodes 0-10 --restart

# 完整部署（本地构建 + 上传 + 重启）
python3 deploy/deploy_iarnet.py --nodes 0-10 --build --restart

# 完整流程（安装依赖 + 构建 + 重启）
python3 deploy/deploy_iarnet.py --nodes 0-10 --install-deps --build --restart

# 部署到单个节点
python3 deploy/deploy_iarnet.py --node 0 --build --restart

# 部署后端和前端（本地构建前端，然后部署）
python3 deploy/deploy_iarnet.py --nodes 0-10 --build --frontend --restart

# 完整流程（安装依赖 + 构建后端 + 构建前端 + 部署 + 启动）
python3 deploy/deploy_iarnet.py --nodes 0-10 --install-deps --build --frontend --restart
```

**部署流程说明**：
- `--build`: 在本地构建后端二进制文件，然后上传到所有节点（只构建一次，所有节点复用）
- `--frontend`: 在本地构建前端项目（Next.js），然后部署到所有节点（只构建一次，所有节点复用）
- `--restart`: 重启服务（停止现有服务，然后启动后端，后端启动成功后立即启动前端）

**自动启动逻辑**：
- 部署后端（`--build`）后，**自动启动后端服务**
- 部署前端（`--frontend`）后，如果后端正在运行，**自动启动前端服务**
- 使用 `--restart` 时，先停止所有服务，然后启动后端，后端启动成功后立即启动前端

### 配置文件说明

每个节点的配置文件包含：
- **host**: 节点的IP地址
- **resource.name**: 节点名称（node.0, node.1, ...）
- **initial_peers**: 其他节点的地址列表（自动生成）
- **data_dir**: 节点特定的数据目录
- **database**: 节点特定的数据库路径
- **logging**: 节点特定的日志配置

### 部署流程

完整部署流程包括以下步骤：

1. **生成配置文件**: 为每个节点生成独立的配置文件
2. **安装依赖**（可选，使用 `--install-deps`）: 使用 Ansible 在虚拟机上安装：
   - Go 语言环境
   - gRPC 相关库（libprotobuf-dev, protobuf-compiler等）
   - ZeroMQ 库（libzmq3-dev, libzmq5, libczmq4等，包括运行时库）
   - Node.js 和 npm（用于前端）
   - Docker（可选）
   - 其他运行时依赖
3. **本地构建**（可选，使用 `--build` 和 `--frontend`）: 
   - **后端**: 在本地构建 iarnet 二进制文件（只构建一次，所有节点复用）
   - **前端**: 在本地构建 Next.js 前端项目（只构建一次，所有节点复用）
4. **上传文件**: 
   - 上传配置文件到 `~/iarnet/config.yaml`
   - 上传二进制文件到 `~/iarnet/iarnet`（如果使用 `--build`）
   - 上传前端构建产物到 `~/iarnet/web/`（如果使用 `--frontend`）
5. **自动启动服务**: 
   - 部署后端后**自动启动后端服务**
   - 部署前端后，如果后端正在运行，**自动启动前端服务**
   - 使用 `--restart` 时，先停止所有服务，然后启动后端，后端启动成功后立即启动前端

**重要说明**: 
- **后端构建**: `--build` 参数会在**本地**构建二进制文件，然后上传到虚拟机。不再在虚拟机上构建，避免虚拟机缺少 Go 环境的问题。
- **前端构建**: `--frontend` 参数会在**本地**构建前端项目（`npm run build`），然后只上传构建产物（`.next` 目录、`public` 目录、`package.json` 等）到虚拟机。在虚拟机上只安装生产依赖（`npm install --production`），不进行构建。
- **启动流程**: 
  - 部署后端 → 立即启动后端
  - 部署前端 → 检查后端状态 → 如果后端运行中，立即启动前端
  - 使用 `--restart` → 停止服务 → 启动后端 → 启动前端（如果部署了前端）
- **超时设置**: 后端启动超时 30 秒，前端启动超时 40 秒，确保服务有足够时间启动
- 使用 `--install-deps` 可以一次性安装所有依赖（包括前端依赖）

### 检查部署状态

```bash
# SSH到节点检查后端服务状态
python3 deploy/ssh_vm.py vm-iarnet-01 "ps aux | grep iarnet"

# SSH到节点检查前端服务状态
python3 deploy/ssh_vm.py vm-iarnet-01 "ps aux | grep 'next start'"

# 检查后端日志
python3 deploy/ssh_vm.py vm-iarnet-01 "tail -f ~/iarnet/iarnet.log"

# 检查前端日志
python3 deploy/ssh_vm.py vm-iarnet-01 "tail -f ~/iarnet/web.log"

# 检查配置文件
python3 deploy/ssh_vm.py vm-iarnet-01 "cat ~/iarnet/config.yaml"

# 检查前端访问（前端默认端口 3000）
curl http://192.168.100.10:3000

# 检查后端访问（后端默认端口 8083）
curl http://192.168.100.10:8083
```

## 使用 Ansible 批量管理

项目提供了 Ansible 配置，可以批量管理虚拟机（安装软件、配置服务等）。

### 安装 Ansible

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y ansible

# 或使用 pip
pip3 install ansible
```

### 生成 Ansible Inventory

从 `vm-config.yaml` 自动生成 Ansible inventory 文件：

```bash
# 生成所有节点的 inventory
python3 generate_ansible_inventory.py

# 只生成 docker 节点的 inventory
python3 generate_ansible_inventory.py --type docker

# 指定输出路径
python3 generate_ansible_inventory.py --output ./ansible/inventory.ini
```

生成的 inventory 文件包含以下组：
- `docker`: Docker 节点（60台）
- `iarnet`: iarnet 节点（20台）
- `k8s_master`: K8s Master 节点（40台）
- `k8s_worker`: K8s Worker 节点（80台）
- `k8s_cluster`: 所有 K8s 节点（包含 master 和 worker）

### 批量安装 Docker 引擎

使用 Ansible 批量安装 Docker 引擎到所有 docker 节点：

```bash
# 方法1: 使用便捷脚本（推荐）
cd ansible
./install-docker.sh

# 方法2: 直接使用 ansible-playbook
cd ansible
ansible-playbook playbooks/install-docker.yml

# 只安装到特定节点
ansible-playbook playbooks/install-docker.yml --limit docker[0:9]  # 只安装前10个节点

# 检查安装状态
ansible docker -m shell -a "docker --version"
ansible docker -m shell -a "docker ps"
```

### 其他 Ansible 操作示例

```bash
cd ansible

# 检查所有 docker 节点的连接
ansible docker -m ping

# 在所有 docker 节点上执行命令
ansible docker -m shell -a "uptime"
ansible docker -m shell -a "df -h"

# 重启所有 docker 节点的 Docker 服务
ansible docker -m systemd -a "name=docker state=restarted" --become

# 查看特定节点的信息
ansible vm-docker-01 -m setup
```

### Ansible Playbook 说明

- **install-docker.yml**: 安装 Docker 引擎
  - 自动检测已安装的节点并跳过
  - 安装 Docker CE、Docker CLI、containerd
  - 启动并启用 Docker 服务
  - 将用户添加到 docker 组
  - 验证安装结果

### 自定义 Ansible Playbook

可以在 `ansible/playbooks/` 目录下创建新的 playbook 文件，例如：

```yaml
# ansible/playbooks/custom-task.yml
---
- name: 自定义任务
  hosts: docker
  become: yes
  tasks:
    - name: 执行自定义命令
      shell: echo "Hello from Ansible"
```

然后运行：
```bash
cd ansible
ansible-playbook playbooks/custom-task.yml
```


