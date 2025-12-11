# Ansible 批量管理工具

使用 Ansible 批量管理虚拟机（安装软件、配置服务等）。

## 快速开始

### 1. 安装 Ansible

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y ansible

# 或使用 pip
pip3 install ansible
```

### 2. 生成 Inventory

从 `vm-config.yaml` 生成 Ansible inventory 文件：

```bash
cd /home/zhangyx/iarnet/deploy
python3 generate_ansible_inventory.py
```

这会生成 `ansible/inventory.ini` 文件，包含所有虚拟机的配置。

### 3. 批量安装 Docker

```bash
cd ansible
./install-docker.sh
```

或者直接使用 ansible-playbook：

```bash
cd ansible
ansible-playbook playbooks/install-docker.yml
```

## Inventory 文件说明

生成的 inventory 文件包含以下组：

- **docker**: Docker 节点（60台，vm-docker-01 到 vm-docker-60）
- **iarnet**: iarnet 节点（20台，vm-iarnet-01 到 vm-iarnet-20）
- **k8s_master**: K8s Master 节点（40台）
- **k8s_worker**: K8s Worker 节点（80台）
- **k8s_cluster**: 所有 K8s 节点（包含 master 和 worker）

## 常用操作

### 检查连接

```bash
# 检查所有 docker 节点
ansible docker -m ping

# 检查特定节点
ansible vm-docker-01 -m ping
```

### 执行命令

```bash
# 在所有 docker 节点上执行命令
ansible docker -m shell -a "uptime"
ansible docker -m shell -a "docker --version"
ansible docker -m shell -a "docker ps"

# 在特定节点上执行
ansible vm-docker-01 -m shell -a "docker info"
```

### 管理服务

```bash
# 重启所有 docker 节点的 Docker 服务
ansible docker -m systemd -a "name=docker state=restarted" --become

# 检查服务状态
ansible docker -m systemd -a "name=docker" --become
```

### 限制执行范围

```bash
# 只在前10个 docker 节点上执行
ansible-playbook playbooks/install-docker.yml --limit docker[0:9]

# 只在特定节点上执行
ansible-playbook playbooks/install-docker.yml --limit vm-docker-01,vm-docker-02
```

## Playbook 说明

### install-docker.yml

批量安装 Docker 引擎的 playbook，功能包括：

- ✅ 自动检测已安装的节点并跳过
- ✅ 安装 Docker CE、Docker CLI、containerd
- ✅ 安装 Docker Buildx 和 Docker Compose 插件
- ✅ 启动并启用 Docker 服务
- ✅ 将用户添加到 docker 组
- ✅ 验证安装结果

## 自定义 Playbook

可以在 `playbooks/` 目录下创建新的 playbook，例如：

```yaml
# playbooks/custom-task.yml
---
- name: 自定义任务
  hosts: docker
  become: yes
  tasks:
    - name: 执行自定义命令
      shell: echo "Hello from Ansible"
```

运行自定义 playbook：

```bash
ansible-playbook playbooks/custom-task.yml
```

## 配置文件说明

- **ansible.cfg**: Ansible 主配置文件
  - 设置默认 inventory 路径
  - 配置 SSH 连接参数
  - 设置并发数和超时时间

- **inventory.ini**: 主机清单文件（自动生成）
  - 包含所有虚拟机的 IP 和主机名
  - 按类型分组

## 故障排查

### 连接失败

1. 检查虚拟机是否运行：
   ```bash
   virsh list --all
   ```

2. 检查网络连接：
   ```bash
   ping 192.168.100.50  # docker 节点的第一个 IP
   ```

3. 检查 SSH 连接：
   ```bash
   ssh ubuntu@192.168.100.50
   ```

### 权限问题

如果遇到权限问题，确保：
- 用户有 sudo 权限
- 可以免密 sudo（或使用 `--ask-become-pass`）

### 重新生成 Inventory

如果虚拟机配置发生变化，重新生成 inventory：

```bash
python3 generate_ansible_inventory.py --output ./ansible/inventory.ini
```

