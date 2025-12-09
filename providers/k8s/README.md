# Kubernetes Provider

iarnet 的 Kubernetes 资源提供者，通过 gRPC 接口管理 Kubernetes 集群中的 Pod 资源。

## 功能特性

- **Pod 生命周期管理**: 创建、监控和删除 Pod
- **资源限制**: 支持 CPU、内存、GPU 资源限制
- **实时监控**: 通过 metrics-server 获取实时资源使用情况
- **健康检查**: 自动检测与控制平面的连接状态
- **RBAC 支持**: 完善的权限控制

## 快速开始

### 在集群外运行

```bash
# 构建
go build -o k8s-provider ./cmd/main.go

# 运行（使用默认 kubeconfig）
./k8s-provider --config config.yaml
```

### 在集群内运行（推荐）

```bash
# 构建镜像
./build.sh

# 部署到 Kubernetes
kubectl apply -f k8s-deployment.yaml
```

## 配置说明

```yaml
server:
  port: 50052  # gRPC 服务端口

kubernetes:
  kubeconfig: ""  # kubeconfig 路径，留空使用 in-cluster 配置
  namespace: "default"  # 部署 Pod 的命名空间
  in_cluster: true  # 是否使用 in-cluster 配置

resource:
  cpu: 8000  # CPU 容量（millicores）
  memory: "8Gi"  # 内存容量
  gpu: 4  # GPU 数量

resource_tags:
  - cpu
  - memory
  - gpu
```

## 与 Docker Provider 的区别

| 特性 | Docker Provider | Kubernetes Provider |
|------|-----------------|---------------------|
| 运行时 | Docker 容器 | Kubernetes Pod |
| 资源隔离 | Docker cgroups | Kubernetes ResourceQuota |
| 网络 | Docker 网络 | Kubernetes Service/DNS |
| 实时监控 | Docker Stats API | metrics-server |
| GPU 支持 | nvidia-docker | nvidia device plugin |

## API 接口

与 Docker Provider 完全一致的 gRPC 接口：

- `Connect`: 控制端连接
- `Disconnect`: 断开连接
- `GetCapacity`: 获取资源容量
- `GetAvailable`: 获取可用资源
- `Deploy`: 部署 Pod
- `HealthCheck`: 健康检查
- `GetRealTimeUsage`: 获取实时资源使用

kubectl get pods -n kube-system -l k8s-app=metrics-server -o wide && echo "---" && kubectl describe pod -n kube-system -l k8s-app=metrics-server | tail -30

# 删除 default 命名空间下所有 iarnet 管理的 Pod
kubectl delete pods -l iarnet.managed=true -n default

# 或者删除所有命名空间下的
kubectl delete pods -l iarnet.managed=true --all-namespaces

# 查看所有 iarnet 管理的 Pod
kubectl get pods -l iarnet.managed=true -n default

# 查看特定 provider 部署的 Pod
kubectl get pods -l iarnet.provider_id=<your-provider-id> -n default

# 强制删除（如果 Pod 卡住）
kubectl delete pods -l iarnet.managed=true -n default --force --grace-period=0

kind load docker-image iarnet/component:python_3.11-latest --name test
