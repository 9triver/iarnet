# 构建预装 Docker 的 Ubuntu 镜像指南

## 概述

为了避免在创建虚拟机后再安装 Docker 时受网络影响而失败，可以预先构建一个包含 Docker 的 Ubuntu 基础镜像。

## 前置要求

### 1. 安装 libguestfs-tools

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y libguestfs-tools

# CentOS/RHEL
sudo yum install -y libguestfs-tools
```

### 2. 准备源镜像

确保已有 Ubuntu cloud 镜像：
```bash
# 检查源镜像是否存在
ls -lh /var/lib/libvirt/images/ubuntu-22.04-cloud.qcow2
```

如果不存在，可以从 Ubuntu 官网下载：
```bash
# 下载 Ubuntu 22.04 cloud 镜像
cd /var/lib/libvirt/images
wget https://cloud-images.ubuntu.com/releases/22.04/release/ubuntu-22.04-server-cloudimg-amd64.img
mv ubuntu-22.04-server-cloudimg-amd64.img ubuntu-22.04-cloud.qcow2
```

## 构建镜像

### 基本用法

```bash
# 使用默认设置构建镜像
python3 deploy/build_docker_base_image.py
```

这会：
- 使用 `/var/lib/libvirt/images/ubuntu-22.04-cloud.qcow2` 作为源镜像
- 生成 `/var/lib/libvirt/images/ubuntu-22.04-cloud-docker.qcow2` 作为输出镜像
- 在镜像中预装 Docker Engine
- 将 `ubuntu` 用户添加到 docker 组

### 自定义选项

```bash
# 指定源镜像和输出镜像
python3 deploy/build_docker_base_image.py \
    --source /path/to/source.qcow2 \
    --output /path/to/output-docker.qcow2

# 指定要添加到 docker 组的用户
python3 deploy/build_docker_base_image.py --user ubuntu
```

## 镜像特性

构建的镜像包含：

1. **Docker Engine** - 最新稳定版
2. **Docker CLI** - Docker 命令行工具
3. **containerd** - 容器运行时
4. **Docker Compose** - Docker Compose 插件
5. **Docker Buildx** - 构建扩展插件
6. **Docker 配置** - 已配置 systemd cgroup driver
7. **用户配置** - 指定用户已添加到 docker 组

## 使用国内镜像源

脚本已自动配置使用阿里云镜像源，以提高下载速度和成功率：
- Ubuntu 软件包：使用 `mirrors.aliyun.com`
- Docker 仓库：使用 `mirrors.aliyun.com/docker-ce`

## 更新虚拟机配置

构建完成后，更新 `vm-config.yaml`：

```yaml
global:
  base_image: "/var/lib/libvirt/images/ubuntu-22.04-cloud-docker.qcow2"
  # ... 其他配置
```

## 重新创建虚拟机

```bash
# 删除现有虚拟机（如果需要）
python3 deploy/delete_vms.py

# 使用新镜像创建虚拟机
python3 deploy/create_vms.py
```

## 验证

创建虚拟机后，可以验证 Docker 是否已安装：

```bash
# SSH 到任意虚拟机
ssh ubuntu@192.168.100.10

# 检查 Docker 版本
docker --version

# 检查 Docker 服务状态
sudo systemctl status docker

# 测试 Docker（不需要 sudo，因为用户已在 docker 组中）
docker run hello-world
```

## 故障排查

### 1. virt-customize 命令不存在

```bash
sudo apt-get install -y libguestfs-tools
```

### 2. 构建过程很慢

- 这是正常的，安装 Docker 需要下载大量包
- 使用国内镜像源可以加速
- 预计需要 5-10 分钟

### 3. 构建失败

- 检查网络连接
- 检查磁盘空间（镜像可能占用 2-3GB）
- 查看详细错误信息

### 4. 权限问题

```bash
# 确保有权限访问镜像目录
sudo chmod 755 /var/lib/libvirt/images
```

## 优势

使用预装 Docker 的镜像有以下优势：

1. **避免网络问题** - 在构建镜像时一次性安装，不受虚拟机创建时的网络影响
2. **加快部署速度** - 创建虚拟机后无需等待 Docker 安装
3. **提高成功率** - 减少因网络问题导致的部署失败
4. **统一环境** - 所有虚拟机使用相同版本的 Docker

## 注意事项

1. **镜像大小** - 预装 Docker 后，镜像大小会增加约 500MB-1GB
2. **构建时间** - 首次构建需要 5-10 分钟（取决于网络速度）
3. **一次性工作** - 构建一次后，可以重复使用该镜像创建所有虚拟机
4. **更新 Docker** - 如果需要更新 Docker 版本，需要重新构建镜像

