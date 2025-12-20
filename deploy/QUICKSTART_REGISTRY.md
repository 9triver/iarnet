# Docker Registry 快速使用指南

## 第一步：部署 Registry 并配置 iarnet 节点

```bash
# 部署 Registry 并配置所有 iarnet 节点
python3 deploy/setup_docker_registry.py --configure-nodes iarnet
```

这会：
1. 在第一个 iarnet 节点（vm-iarnet-01, 192.168.100.10）上部署 Docker Registry
2. 配置所有 iarnet 节点以访问 Registry

## 第二步：构建并推送 iarnet 镜像

```bash
# 构建并推送 iarnet 镜像到 Registry
python3 deploy/push_images_to_registry.py --build-iarnet
```

这会：
1. 在本地构建 iarnet Docker 镜像
2. 标记镜像为 `192.168.100.10:5000/iarnet:latest`
3. 推送到 Registry

## 完整流程示例

```bash
# 1. 部署 Registry 并配置 iarnet 节点
python3 deploy/setup_docker_registry.py --configure-nodes iarnet

# 2. 构建并推送 iarnet 镜像
python3 deploy/push_images_to_registry.py --build-iarnet

# 3. 验证镜像已推送
python3 deploy/push_images_to_registry.py --list
```

## 在其他节点上使用镜像

配置完成后，所有 iarnet 节点都可以从 Registry 拉取镜像：

```bash
# 在任意 iarnet 节点上
docker pull 192.168.100.10:5000/iarnet:latest
```

## 配置所有节点（可选）

如果需要同时配置 Docker provider 节点和 iarnet 节点：

```bash
python3 deploy/setup_docker_registry.py --configure-nodes all
```

## 单独配置节点类型

```bash
# 只配置 iarnet 节点
python3 deploy/setup_docker_registry.py --configure-nodes iarnet

# 只配置 Docker provider 节点
python3 deploy/setup_docker_registry.py --configure-nodes docker
```

