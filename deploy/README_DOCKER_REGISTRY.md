# Docker Registry 使用指南

本文档说明如何搭建和使用共享的 Docker Registry，以便所有 provider 节点可以共享 Docker 镜像。

## 架构说明

- **Registry 节点**: 部署在第一个 iarnet 节点上（默认 vm-iarnet-01，IP: 192.168.100.10）
- **Registry 端口**: 5000
- **Registry URL**: `http://192.168.100.10:5000`
- **存储位置**: `~/docker-registry/data`（在 Registry 节点上）

## 快速开始

### 1. 部署 Registry

```bash
# 在第一个 iarnet 节点上部署 Registry
python3 deploy/setup_docker_registry.py

# 同时配置所有 Docker provider 节点
python3 deploy/setup_docker_registry.py --configure-nodes
```

### 2. 构建并推送 iarnet 镜像

```bash
# 在本地构建并推送 iarnet 镜像
python3 deploy/push_images_to_registry.py --build-iarnet
```

### 3. 推送 component 镜像

```bash
# 推送已存在的 component 镜像
python3 deploy/push_images_to_registry.py --push-component <镜像名> <组件名>

# 示例：推送名为 my-component:latest 的镜像
python3 deploy/push_images_to_registry.py --push-component my-component:latest my-component
```

### 4. 查看 Registry 中的镜像

```bash
python3 deploy/push_images_to_registry.py --list
```

## 详细说明

### 部署 Registry

`setup_docker_registry.py` 脚本会：
1. 检查目标节点的 Docker 是否安装（未安装则自动安装）
2. 创建 Registry 数据目录
3. 启动 Docker Registry 容器（使用 host 网络模式）
4. 可选：配置所有 Docker provider 节点以使用 Registry

**参数说明：**
- `--vm-config`: 虚拟机配置文件路径（默认: deploy/vm-config.yaml）
- `--registry-node`: Registry 部署在哪个 iarnet 节点上（默认: 0，即第一个节点）
- `--configure-nodes`: 同时配置所有 Docker provider 节点

### 推送镜像

`push_images_to_registry.py` 脚本支持：
1. 构建并推送 iarnet 镜像
2. 推送已存在的镜像（iarnet 或 component）
3. 列出 Registry 中的镜像

**常用命令：**

```bash
# 构建并推送 iarnet 镜像
python3 deploy/push_images_to_registry.py --build-iarnet

# 推送已存在的 iarnet 镜像
python3 deploy/push_images_to_registry.py --push-iarnet iarnet:latest

# 推送 component 镜像
python3 deploy/push_images_to_registry.py --push-component component-image:latest component-name

# 列出所有镜像
python3 deploy/push_images_to_registry.py --list
```

### 在 Provider 节点使用镜像

配置完成后，provider 节点可以直接从 Registry 拉取镜像：

```bash
# 在 provider 节点上拉取镜像
docker pull 192.168.100.10:5000/iarnet:latest
docker pull 192.168.100.10:5000/components/my-component:latest
```

## 手动操作

### 手动配置节点使用 Registry

如果某个节点需要手动配置，可以在节点上执行：

```bash
# 编辑 Docker daemon 配置
sudo nano /etc/docker/daemon.json
```

添加以下内容：

```json
{
  "insecure-registries": ["192.168.100.10:5000"]
}
```

然后重启 Docker：

```bash
sudo systemctl restart docker
```

### 手动推送镜像

```bash
# 标记镜像
docker tag <本地镜像> 192.168.100.10:5000/<镜像名>:<标签>

# 推送镜像
docker push 192.168.100.10:5000/<镜像名>:<标签>

# 示例：推送 iarnet 镜像
docker tag iarnet:latest 192.168.100.10:5000/iarnet:latest
docker push 192.168.100.10:5000/iarnet:latest
```

### 手动拉取镜像

```bash
# 拉取镜像
docker pull 192.168.100.10:5000/<镜像名>:<标签>

# 示例
docker pull 192.168.100.10:5000/iarnet:latest
docker pull 192.168.100.10:5000/components/my-component:latest
```

## 镜像命名规范

- **iarnet 镜像**: `192.168.100.10:5000/iarnet:latest`
- **component 镜像**: `192.168.100.10:5000/components/<组件名>:<标签>`

## 故障排查

### Registry 无法访问

1. 检查 Registry 容器是否运行：
   ```bash
   ssh ubuntu@192.168.100.10 "docker ps | grep registry"
   ```

2. 检查 Registry 是否响应：
   ```bash
   curl http://192.168.100.10:5000/v2/
   ```

3. 检查防火墙设置（如果启用）

### 节点无法拉取镜像

1. 检查节点是否配置了 `insecure-registries`：
   ```bash
   cat /etc/docker/daemon.json
   ```

2. 检查 Docker 是否重启：
   ```bash
   sudo systemctl restart docker
   ```

3. 检查网络连通性：
   ```bash
   ping 192.168.100.10
   curl http://192.168.100.10:5000/v2/
   ```

### 镜像推送失败

1. 检查镜像是否存在：
   ```bash
   docker images | grep <镜像名>
   ```

2. 检查 Registry 是否可访问：
   ```bash
   curl http://192.168.100.10:5000/v2/
   ```

3. 检查本地 Docker 是否配置了 `insecure-registries`

## 注意事项

1. **安全性**: 当前 Registry 使用 HTTP（不安全），仅适用于内网环境
2. **存储**: Registry 数据存储在 `~/docker-registry/data`，注意磁盘空间
3. **性能**: Registry 使用 host 网络模式，性能较好
4. **备份**: 建议定期备份 Registry 数据目录

## 扩展功能

### 启用认证（可选）

如果需要启用认证，可以修改 `setup_docker_registry.py`，添加认证配置。

### 使用 HTTPS（可选）

如果需要使用 HTTPS，需要配置证书，并修改 Registry 启动参数。

