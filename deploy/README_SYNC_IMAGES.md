# 镜像同步使用指南

## 概述

`sync_images_to_nodes.py` 脚本可以将本地 Docker 镜像直接同步到各个节点，避免使用 Registry。这对于需要将镜像分发到多个节点的场景非常有用。

## 使用方法

### 基本用法

```bash
# 同步单个镜像到所有 Docker provider 节点
python3 deploy/sync_images_to_nodes.py --image iarnet:latest

# 同步到 iarnet 节点
python3 deploy/sync_images_to_nodes.py --image iarnet:latest --node-type iarnet

# 同步到所有节点（docker + iarnet）
python3 deploy/sync_images_to_nodes.py --image iarnet:latest --node-type all
```

### 同步多个镜像

```bash
# 同步多个镜像
python3 deploy/sync_images_to_nodes.py --images iarnet:latest component1:latest component2:latest
```

### 指定节点范围

```bash
# 同步到指定节点
python3 deploy/sync_images_to_nodes.py --image iarnet:latest --nodes 0-10

# 同步到特定节点
python3 deploy/sync_images_to_nodes.py --image iarnet:latest --nodes 0,1,2,5
```

### 强制同步

```bash
# 强制同步，即使节点上已存在镜像
python3 deploy/sync_images_to_nodes.py --image iarnet:latest --force
```

## 工作流程

1. **检查本地镜像** - 验证镜像是否存在于本地
2. **保存镜像** - 将镜像保存为 tar 文件
3. **并行传输** - 使用 scp 并行传输到各个节点
4. **加载镜像** - 在节点上使用 `docker load` 加载镜像
5. **清理** - 删除临时文件

## 特性

- **智能检查** - 自动检查节点上是否已存在镜像，避免重复传输
- **并行传输** - 支持并行同步到多个节点，提高效率
- **断点续传** - 如果节点上已存在镜像，自动跳过
- **错误处理** - 完善的错误处理和日志输出

## 示例场景

### 场景 1: 同步 iarnet 镜像到所有 Docker provider 节点

```bash
# 1. 确保本地有 iarnet 镜像（从 Registry 拉取或本地构建）
docker pull 192.168.100.10:5000/iarnet:latest
# 或者
docker tag iarnet:latest 192.168.100.10:5000/iarnet:latest

# 2. 同步到所有 Docker provider 节点
python3 deploy/sync_images_to_nodes.py --image 192.168.100.10:5000/iarnet:latest --node-type docker
```

### 场景 2: 同步多个 component 镜像

```bash
# 同步多个 component 镜像
python3 deploy/sync_images_to_nodes.py \
    --images \
    component1:latest \
    component2:v1.0 \
    component3:latest \
    --node-type docker
```

### 场景 3: 从 Registry 拉取后同步

```bash
# 1. 从 Registry 拉取镜像到本地
docker pull 192.168.100.10:5000/iarnet:latest

# 2. 同步到所有节点
python3 deploy/sync_images_to_nodes.py \
    --image 192.168.100.10:5000/iarnet:latest \
    --node-type all
```

## 性能优化

- **并发数调整** - 使用 `--max-workers` 调整并发数（默认 10）
- **网络优化** - 脚本会自动复用同一个 tar 文件，避免重复保存
- **智能跳过** - 如果节点上已存在相同镜像，自动跳过传输

## 故障排查

### 镜像不存在

```bash
# 检查本地镜像
docker images | grep iarnet

# 如果不存在，先从 Registry 拉取
docker pull 192.168.100.10:5000/iarnet:latest
```

### 传输失败

- 检查网络连通性
- 检查节点磁盘空间
- 检查 SSH 连接

### 加载失败

- 检查节点上 Docker 是否正常运行
- 检查镜像文件是否完整传输

## 与 Registry 方案对比

| 特性 | Registry 方案 | 直接同步方案 |
|------|--------------|-------------|
| 网络要求 | 需要所有节点访问 Registry | 只需要从本地到各节点 |
| 镜像存储 | 集中存储在 Registry | 分散存储在各节点 |
| 更新速度 | 快（只需推送一次） | 较慢（需要传输到各节点） |
| 网络依赖 | 高（依赖 Registry 节点） | 低（直接传输） |
| 适用场景 | 频繁更新、大量节点 | 一次性部署、节点较少 |

## 最佳实践

1. **首次部署** - 使用直接同步方案，避免 Registry 依赖
2. **频繁更新** - 使用 Registry 方案，只需推送一次
3. **混合使用** - 首次使用直接同步，后续更新使用 Registry

