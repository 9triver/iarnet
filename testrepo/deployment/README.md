# 本地快速部署调试环境

## 1 概述

本部署环境包含以下组件：
- **dind-1/dind-2/dind-3**: Docker-in-Docker 容器，为 Provider 提供独立的 Docker 环境
- **provider-docker-1/provider-docker-2/provider-docker-3**: Docker Provider 服务
- **iarnet-global**: 全局注册中心
- **iarnet-1/iarnet-2/iarnet-3**: iarnet 节点实例（支持 Gossip 节点发现）

## 2 镜像打包

- **iarnet**: 
  ```sh
  cd ../iarnet && ./build.sh
  ```

- **iarnet/runner:python**: 
  ```sh
  cd ../iarnet/containers/images/runner && ./build.sh python_3.11 latest
  ```

- **iarnet/component**: 
  ```sh
  cd ../iarnet/containers/component/python && ./build.sh
  ```

- **iarnet-global**: 
  ```sh
  cd ../iarnet-global && ./build.sh
  ```

- **iarnet/provider:docker**: 
  ```sh
  cd ../iarnet/providers/docker && ./build.sh
  ```

## 3 启动步骤

### 3.1 启动 Providers

```sh
# start
docker-compose up -d provider-docker-1 provider-docker-2 provider-docker-3

# stop
docker-compose stop provider-docker-1 provider-docker-2 provider-docker-3

# remove
docker-compose rm provider-docker-1 provider-docker-2 provider-docker-3
```

### 3.2 启动 iarnet-global

```sh
# start
docker-compose up -d iarnet-global

# stop
docker-compose stop iarnet-global

# remove
docker-compose rm iarnet-global
```

### 3.3 启动 iarnet 节点

```sh
# start
docker-compose up -d iarnet-1 iarnet-2 iarnet-3

# stop
docker-compose stop iarnet-1 iarnet-2 iarnet-3

# remove
docker-compose rm iarnet-1 iarnet-2 iarnet-3
```

**注意**：各类组件按序启动之间可能需要等待片刻，待其依赖组件就绪，如若出现 iarnet 没有连上 global 的情景，单独重启相应 iarnet 节点的容器即可。

## 4 同步镜像

### 背景说明

由于本部署环境使用了 Docker-in-Docker (dind) 架构，每个 dind 容器和 iarnet 节点都拥有独立的 Docker 环境。在宿主机上构建的 `iarnet/component` 和 `iarnet/runner` 镜像需要同步到这些独立的 Docker 环境中，否则在运行任务时会出现找不到镜像的错误。

- **component 镜像**需要同步到 `dind-1`、`dind-2`、`dind-3`，供 Provider 使用
- **runner 镜像**需要同步到 `iarnet-1`、`iarnet-2`、`iarnet-3`，供任务执行使用

### 使用方法

在 `deployment` 目录下运行同步脚本：

```sh
cd deployment && ./sync-images-to-dind.sh
```

脚本会自动：
1. 查找宿主机上所有 `iarnet/component` 和 `iarnet/runner` 镜像
2. 将 component 镜像同步到所有 dind 容器
3. 将 runner 镜像同步到所有 iarnet 节点
4. 显示同步结果和各节点的镜像列表

**注意**：每次在宿主机上重新构建 component 或 runner 镜像后，都需要重新运行此脚本进行同步。

## 5 各节点控制台地址
### iarnet-1
- 前端: http://localhost:3000

### iarnet-2
- 前端: http://localhost:3001

### iarnet-3
- 前端: http://localhost:3003

### iarnet-global
- 前端: http://localhost:3002


## 6 清除 dind 中的容器

### 背景说明

在开发和测试过程中，dind 容器和 iarnet 节点内部会运行大量的任务容器（如 component 容器、runner 容器等）。

### 使用方法

在 `deployment` 目录下运行清除脚本：

```sh
cd deployment && ./cleanup-containers.sh
```

脚本会自动清理以下目标中的所有容器：
- `dind-1`、`dind-2`、`dind-3`：Provider 使用的 Docker 环境
- `iarnet-1`、`iarnet-2`、`iarnet-3`：iarnet 节点内部的 Docker 环境
