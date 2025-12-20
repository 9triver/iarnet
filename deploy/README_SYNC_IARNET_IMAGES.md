# 同步 iarnet 镜像到虚拟机使用指南

## 概述

`sync_iarnet_images.py` 是一个便捷脚本，专门用于将本地的 iarnet 和 runner 镜像同步到 iarnet 虚拟机的 Docker 引擎中。该脚本基于 `sync_images_to_nodes.py`，提供了更简单的接口来同步这两个常用镜像。

## 功能特性

- **自动检测镜像** - 自动检查本地镜像是否存在
- **批量同步** - 同时同步 iarnet 和 runner 镜像
- **灵活选择** - 支持只同步其中一个镜像
- **并行传输** - 支持并行同步到多个节点
- **智能跳过** - 自动跳过已存在的镜像（除非使用 `--force`）

## 使用方法

### 基本用法

```bash
# 同步默认镜像到所有 iarnet 虚拟机
# 默认镜像: iarnet:latest 和 iarnet/runner:python_3.11-latest
python3 deploy/sync_iarnet_images.py
```

### 指定自定义镜像名

```bash
# 使用自定义镜像名
python3 deploy/sync_iarnet_images.py \
    --iarnet-image my-iarnet:v1.0 \
    --runner-image my-runner:latest
```

### 只同步特定镜像

```bash
# 只同步 iarnet 镜像
python3 deploy/sync_iarnet_images.py --iarnet-only

# 只同步 runner 镜像
python3 deploy/sync_iarnet_images.py --runner-only
```

### 同步到指定节点

```bash
# 同步到节点 0-5
python3 deploy/sync_iarnet_images.py --nodes 0-5

# 同步到特定节点
python3 deploy/sync_iarnet_images.py --nodes 0,1,2,5
```

### 强制同步

```bash
# 强制同步，即使节点上已存在镜像
python3 deploy/sync_iarnet_images.py --force
```

### 只检查不同步

```bash
# 只检查本地镜像是否存在，不进行同步
python3 deploy/sync_iarnet_images.py --check-only
```

## 完整示例

### 场景 1: 首次部署

```bash
# 1. 确保本地有镜像（构建或拉取）
docker build -t iarnet:latest .
docker build -t iarnet/runner:python_3.11-latest ./runner

# 2. 同步到所有 iarnet 虚拟机
python3 deploy/sync_iarnet_images.py
```

### 场景 2: 更新镜像

```bash
# 1. 重新构建镜像
docker build -t iarnet:latest .
docker build -t iarnet/runner:python_3.11-latest ./runner

# 2. 强制同步到所有节点（覆盖旧镜像）
python3 deploy/sync_iarnet_images.py --force
```

### 场景 3: 只更新 runner 镜像

```bash
# 1. 重新构建 runner 镜像
docker build -t iarnet/runner:python_3.11-latest ./runner

# 2. 只同步 runner 镜像
python3 deploy/sync_iarnet_images.py --runner-only --force
```

### 场景 4: 同步到部分节点

```bash
# 只同步到前 5 个节点
python3 deploy/sync_iarnet_images.py --nodes 0-4
```

## 工作流程

1. **检查本地镜像** - 验证镜像是否存在于本地
2. **保存镜像** - 将镜像保存为 tar 文件
3. **并行传输** - 使用 scp 并行传输到各个节点
4. **加载镜像** - 在节点上使用 `docker load` 加载镜像
5. **清理** - 删除临时文件

## 镜像名称说明

### 默认镜像名

- **iarnet 镜像**: `iarnet:latest`
- **runner 镜像**: `iarnet/runner:python_3.11-latest`

这些默认值基于配置文件中的常见用法。如果使用不同的镜像名，可以通过参数指定。

### 从配置文件获取 runner 镜像名

runner 镜像名通常定义在 `iarnet-configs/config-node-*.yaml` 中：

```yaml
application:
  runner_images:
    python:3.11-latest: iarnet/runner:python_3.11-latest
```

如果使用不同的镜像名，请使用 `--runner-image` 参数指定。

## 参数说明

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--iarnet-image` | iarnet 镜像名 | `iarnet:latest` |
| `--runner-image` | runner 镜像名 | `iarnet/runner:python_3.11-latest` |
| `--iarnet-only` | 只同步 iarnet 镜像 | 否 |
| `--runner-only` | 只同步 runner 镜像 | 否 |
| `--nodes` | 指定节点ID列表 | 所有节点 |
| `--max-workers` | 最大并发数 | 10 |
| `--force` | 强制同步 | 否 |
| `--check-only` | 只检查不同步 | 否 |

## 故障排查

### 镜像不存在

```bash
# 检查本地镜像
docker images | grep -E "iarnet|runner"

# 如果不存在，先构建或拉取
docker build -t iarnet:latest .
docker build -t iarnet/runner:python_3.11-latest ./runner
```

### 同步失败

```bash
# 检查节点连通性
ping 192.168.100.10

# 检查 Docker 是否运行
ssh ubuntu@192.168.100.10 "docker --version"

# 检查磁盘空间
ssh ubuntu@192.168.100.10 "df -h"
```

### 网络问题

```bash
# 检查 SSH 连接
ssh ubuntu@192.168.100.10 "echo OK"

# 检查防火墙
sudo ufw status
```

## 性能优化

- **并发数调整** - 使用 `--max-workers` 调整并发数（默认 10）
- **分批同步** - 如果节点很多，可以分批同步：`--nodes 0-9` 然后 `--nodes 10-19`
- **网络优化** - 脚本会自动复用同一个 tar 文件，避免重复保存

## 与 sync_images_to_nodes.py 的关系

`sync_iarnet_images.py` 是基于 `sync_images_to_nodes.py` 的便捷脚本，专门用于同步 iarnet 相关镜像。两者的关系：

- **sync_images_to_nodes.py** - 通用镜像同步脚本，支持任意镜像和节点类型
- **sync_iarnet_images.py** - 便捷脚本，专门用于 iarnet 和 runner 镜像，提供更简单的接口

如果需要同步其他镜像或到其他节点类型，可以使用 `sync_images_to_nodes.py`。

## 最佳实践

1. **首次部署** - 先构建镜像，然后同步到所有节点
2. **更新镜像** - 使用 `--force` 强制同步，确保所有节点使用新镜像
3. **分批更新** - 如果节点很多，可以分批更新以减少网络压力
4. **验证结果** - 同步后验证镜像是否成功加载

## 相关脚本

- `sync_images_to_nodes.py` - 通用镜像同步脚本
- `deploy_iarnet.py` - iarnet 服务部署脚本
- `export_vm_to_image.py` - 导出虚拟机为镜像

