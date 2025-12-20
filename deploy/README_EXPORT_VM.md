# 虚拟机导出为镜像使用指南

## 概述

`export_vm_to_image.py` 脚本可以将已经安装好 Docker 的虚拟机导出为镜像文件，这样就不需要在 virt-customize 环境中安装 Docker，避免了网络问题。

## 使用场景

- 你已经有一个运行中的虚拟机，并且已经成功安装了 Docker
- 你想将这个虚拟机作为模板，用于创建其他虚拟机
- 你想避免在 virt-customize 环境中安装 Docker 时遇到的网络问题

## 使用方法

### 基本用法

```bash
# 导出虚拟机为镜像
sudo python3 deploy/export_vm_to_image.py --vm-name <虚拟机名称>

# 指定输出路径
sudo python3 deploy/export_vm_to_image.py \
    --vm-name ubuntu-vm-01 \
    --output /var/lib/libvirt/images/ubuntu-22.04-cloud-docker.qcow2
```

### 完整示例（包含清理）

```bash
# 如果虚拟机正在运行，可以指定 IP 进行清理
sudo python3 deploy/export_vm_to_image.py \
    --vm-name ubuntu-vm-01 \
    --vm-ip 192.168.100.11 \
    --vm-user ubuntu \
    --output /var/lib/libvirt/images/ubuntu-22.04-cloud-docker.qcow2
```

### 跳过清理

```bash
# 如果不想清理虚拟机中的临时文件
sudo python3 deploy/export_vm_to_image.py \
    --vm-name ubuntu-vm-01 \
    --no-cleanup
```

## 工作流程

1. **检查虚拟机** - 验证虚拟机是否存在
2. **清理虚拟机**（可选）- 清理临时文件、日志、命令历史等
3. **关闭虚拟机**（如果正在运行）- 确保数据一致性
4. **导出磁盘** - 复制或转换虚拟机磁盘为镜像文件
5. **验证镜像** - 检查镜像文件是否成功创建

## 清理内容

脚本会自动清理以下内容（如果启用清理）：

- APT 缓存和包列表
- 临时文件（/tmp, /var/tmp）
- 日志文件（保留最近的内容）
- 命令历史
- Cloud-init 数据
- Docker 临时数据和日志

## 查看虚拟机列表

如果不确定虚拟机名称，可以查看：

```bash
# 查看所有虚拟机
virsh list --all

# 查看虚拟机详细信息
virsh dominfo <虚拟机名称>
```

## 示例场景

### 场景 1: 从运行中的虚拟机导出

```bash
# 1. 查看虚拟机状态
virsh list --all

# 2. 导出虚拟机（脚本会自动处理关闭）
sudo python3 deploy/export_vm_to_image.py \
    --vm-name ubuntu-vm-01 \
    --vm-ip 192.168.100.11 \
    --vm-user ubuntu
```

### 场景 2: 从已关闭的虚拟机导出

```bash
# 如果虚拟机已经关闭，直接导出
sudo python3 deploy/export_vm_to_image.py \
    --vm-name ubuntu-vm-01 \
    --output /var/lib/libvirt/images/my-docker-image.qcow2
```

### 场景 3: 导出后使用镜像

```bash
# 1. 导出虚拟机
sudo python3 deploy/export_vm_to_image.py \
    --vm-name ubuntu-vm-01 \
    --output /var/lib/libvirt/images/ubuntu-22.04-cloud-docker.qcow2

# 2. 更新 vm-config.yaml
# base_image: "/var/lib/libvirt/images/ubuntu-22.04-cloud-docker.qcow2"

# 3. 使用新镜像创建虚拟机
python3 deploy/create_vms.py
```

## 注意事项

1. **权限要求** - 需要 root 权限（使用 sudo）
2. **磁盘空间** - 确保有足够的磁盘空间存储镜像文件
3. **虚拟机状态** - 如果虚拟机正在运行，脚本会询问是否关闭
4. **数据一致性** - 建议在导出前关闭虚拟机，确保数据一致性
5. **网络配置** - 导出的镜像会保留网络配置，新虚拟机可能需要重新配置

## 故障排查

### 虚拟机不存在

```bash
# 检查虚拟机名称是否正确
virsh list --all

# 检查虚拟机状态
virsh dominfo <虚拟机名称>
```

### 无法获取磁盘路径

```bash
# 手动查看虚拟机磁盘
virsh domblklist <虚拟机名称>
```

### 磁盘空间不足

```bash
# 检查可用空间
df -h /var/lib/libvirt/images

# 清理旧镜像
sudo rm /var/lib/libvirt/images/old-image.qcow2
```

### 导出速度慢

- 使用 `qemu-img convert` 可以优化镜像大小
- 如果磁盘很大，导出可能需要较长时间
- 可以考虑先清理虚拟机，减少镜像大小

## 与 build_docker_base_image.py 对比

| 特性 | build_docker_base_image.py | export_vm_to_image.py |
|------|---------------------------|----------------------|
| 安装方式 | virt-customize 在镜像中安装 | 从已安装的虚拟机导出 |
| 网络要求 | 需要网络访问（可能失败） | 不需要网络（已安装） |
| 适用场景 | 从基础镜像开始 | 已有安装好的虚拟机 |
| 速度 | 较慢（需要安装） | 较快（直接复制） |
| 可靠性 | 可能遇到网络问题 | 更可靠 |

## 最佳实践

1. **准备虚拟机** - 在导出前，确保虚拟机中 Docker 已正确安装和配置
2. **清理虚拟机** - 使用 `--vm-ip` 参数启用清理，减少镜像大小
3. **关闭虚拟机** - 导出前关闭虚拟机，确保数据一致性
4. **验证镜像** - 导出后可以创建测试虚拟机验证镜像是否正常
5. **备份原镜像** - 导出前备份原始镜像，以防需要回退

