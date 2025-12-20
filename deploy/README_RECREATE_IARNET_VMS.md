# 重新创建 iarnet 虚拟机使用指南

## 概述

`recreate_iarnet_vms.py` 脚本用于使用新导出的镜像重新创建所有 iarnet 虚拟机。脚本会先删除现有的 iarnet 虚拟机，然后使用新镜像重新创建它们。

## 使用场景

- 你已经导出了一个包含 Docker 的新镜像（使用 `export_vm_to_image.py`）
- 你想使用这个新镜像重新创建所有 iarnet 虚拟机
- 你想确保所有 iarnet 虚拟机都使用相同的基础镜像

## 前置条件

1. **已导出新镜像** - 确保已经使用 `export_vm_to_image.py` 导出了新镜像
2. **更新配置文件** - 确保 `vm-config.yaml` 中的 `base_image` 路径指向新镜像
3. **权限要求** - 需要 root 权限（使用 sudo）

## 使用方法

### 基本用法

```bash
# 使用新镜像重新创建所有 iarnet 虚拟机
sudo python3 deploy/recreate_iarnet_vms.py
```

### 指定配置文件

```bash
# 使用自定义配置文件
sudo python3 deploy/recreate_iarnet_vms.py --config /path/to/vm-config.yaml
```

### 保留磁盘文件

```bash
# 删除虚拟机但保留磁盘文件（不推荐，可能导致冲突）
sudo python3 deploy/recreate_iarnet_vms.py --keep-disk
```

### 只删除不创建

```bash
# 只删除现有虚拟机，不创建新虚拟机（用于测试）
sudo python3 deploy/recreate_iarnet_vms.py --skip-create
```

### 只创建不删除

```bash
# 跳过删除步骤，直接创建（如果虚拟机已不存在）
sudo python3 deploy/recreate_iarnet_vms.py --skip-delete
```

## 工作流程

1. **读取配置** - 从 `vm-config.yaml` 读取 iarnet 虚拟机配置
2. **显示配置信息** - 显示将要创建的虚拟机数量和配置
3. **确认操作** - 要求用户确认（输入 yes）
4. **删除现有虚拟机** - 停止并删除所有现有的 iarnet 虚拟机
5. **删除磁盘文件** - 删除虚拟机的磁盘文件（除非指定 `--keep-disk`）
6. **创建新虚拟机** - 使用新镜像创建所有 iarnet 虚拟机
7. **验证结果** - 显示创建结果统计

## 完整示例

### 步骤 1: 导出新镜像

```bash
# 从 iarnet-01 导出镜像（已清理 iarnet 相关内容）
sudo python3 deploy/export_vm_to_image.py \
    --vm-name vm-iarnet-01 \
    --vm-ip 192.168.100.10 \
    --vm-user ubuntu \
    --output /var/lib/libvirt/images/ubuntu-22.04-cloud-docker.qcow2
```

### 步骤 2: 更新配置文件

编辑 `vm-config.yaml`，确保 `base_image` 指向新镜像：

```yaml
global:
  base_image: "/var/lib/libvirt/images/ubuntu-22.04-cloud-docker.qcow2"
```

### 步骤 3: 重新创建所有 iarnet 虚拟机

```bash
# 重新创建所有 iarnet 虚拟机
sudo python3 deploy/recreate_iarnet_vms.py
```

## 注意事项

1. **数据备份** - 重新创建会删除所有现有虚拟机，请确保重要数据已备份
2. **网络配置** - 新创建的虚拟机会使用相同的 IP 地址和 hostname
3. **服务部署** - 重新创建后需要重新部署 iarnet 服务
4. **磁盘空间** - 确保有足够的磁盘空间存储新虚拟机的磁盘文件

## 故障排查

### 权限错误

```bash
# 确保使用 sudo 运行
sudo python3 deploy/recreate_iarnet_vms.py
```

### 镜像不存在

```bash
# 检查镜像路径是否正确
ls -lh /var/lib/libvirt/images/ubuntu-22.04-cloud-docker.qcow2

# 检查 vm-config.yaml 中的 base_image 路径
grep base_image deploy/vm-config.yaml
```

### 虚拟机删除失败

```bash
# 手动检查虚拟机状态
virsh list --all | grep iarnet

# 手动删除虚拟机（如果需要）
virsh destroy vm-iarnet-01
virsh undefine vm-iarnet-01
```

### 创建失败

```bash
# 检查 libvirt 连接
virsh list

# 检查网络配置
virsh net-list

# 检查磁盘空间
df -h /var/lib/libvirt/images
```

## 与手动操作对比

### 手动操作（繁琐）

```bash
# 1. 逐个删除虚拟机
for i in {1..10}; do
    virsh destroy vm-iarnet-$(printf "%02d" $i)
    virsh undefine vm-iarnet-$(printf "%02d" $i)
    rm -f /var/lib/libvirt/images/vm-iarnet-$(printf "%02d" $i).qcow2
done

# 2. 使用 create_vms.py 重新创建
sudo python3 deploy/create_vms.py --type iarnet
```

### 使用脚本（简单）

```bash
# 一条命令完成所有操作
sudo python3 deploy/recreate_iarnet_vms.py
```

## 最佳实践

1. **测试环境** - 先在测试环境验证脚本功能
2. **备份配置** - 备份 `vm-config.yaml` 和重要配置文件
3. **分批操作** - 如果虚拟机数量很多，可以考虑分批操作
4. **验证结果** - 创建后验证虚拟机是否正常启动
5. **重新部署** - 创建完成后重新部署 iarnet 服务

## 相关脚本

- `export_vm_to_image.py` - 导出虚拟机为镜像
- `create_vms.py` - 创建虚拟机
- `delete_vms.py` - 删除虚拟机
- `recreate_vm.py` - 重新创建单个虚拟机

