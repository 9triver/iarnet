# IARNet 算力网络资源管理与应用运行平台设计文档

## 1. 项目概述

### 1.1 项目背景

IARNet（Intelligent Application Resource Network）是一个基于Go语言开发的算力网络资源管理与应用运行平台。随着云计算和容器化技术的快速发展，企业面临着分布式算力资源管理的挑战，需要一个统一的平台来管理和调度各种计算资源。

该项目旨在解决以下问题：
- 分散的算力资源难以统一管理
- 容器化应用部署复杂
- 资源利用率监控不足
- 缺乏智能的资源调度机制
- 多环境部署配置复杂

### 1.2 目标与价值

**核心目标：**
- 提供统一的算力资源管理平台
- 支持容器化应用的快速部署和运行
- 实现智能的资源调度和配额控制
- 提供实时的资源监控和状态管理
- 支持多种部署模式和环境

**业务价值：**
- **降低运维成本**：统一管理减少人工干预
- **提高资源利用率**：智能调度优化资源分配
- **加速应用部署**：简化容器化应用的部署流程
- **增强可观测性**：实时监控和告警机制
- **提升扩展性**：支持水平扩展和多云部署

### 1.3 系统整体功能

**核心功能模块：**

1. **容器运行管理**
   - 支持Docker和Kubernetes两种运行时
   - 容器生命周期管理（启动、停止、重启）
   - 容器镜像管理和版本控制

2. **资源配额控制**
   - CPU、内存、GPU、存储资源限制
   - 实时资源使用监控
   - 资源超限保护机制

3. **节点发现与管理**
   - 基于gRPC的peer-to-peer发现机制
   - 动态节点加入和离开
   - 节点健康状态监控

4. **Web管理界面**
   - 算力资源接入管理
   - 应用部署和管理
   - 运行状态监控和可视化

5. **API服务**
   - RESTful API接口
   - 容器操作API
   - 资源查询API
   - 状态监控API

## 2. 系统总体架构

### 2.1 系统组成

系统采用三层架构设计：

```
┌─────────────────────────────────────────────────────────────┐
│                    Web前端管理界面                          │
│  ┌─────────────┬─────────────────┬─────────────────────────┐ │
│  │ 资源接入管理 │   应用管理页面   │     运行状态监控页面     │ │
│  └─────────────┴─────────────────┴─────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                                │
                                │ HTTP API
                                ▼
┌─────────────────────────────────────────────────────────────┐
│                      IARNet 后台服务                        │
│  ┌─────────────┬─────────────────┬─────────────────────────┐ │
│  │  HTTP服务器  │   资源管理器     │      容器运行器         │ │
│  │             │                │  ┌─────────┬─────────────┐ │ │
│  │             │                │  │Standalone│ Kubernetes │ │ │
│  │             │                │  │ Runner  │   Runner   │ │ │
│  │             │                │  └─────────┴─────────────┘ │ │
│  └─────────────┴─────────────────┴─────────────────────────┘ │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              Peer Discovery (gRPC)                     │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                                │
                                │ gRPC Gossip
                                ▼
┌─────────────────────────────────────────────────────────────┐
│                        算力资源层                           │
│  ┌─────────────┬─────────────────┬─────────────────────────┐ │
│  │Docker环境    │ Kubernetes集群   │      其他Peer节点       │ │
│  │             │                │                         │ │
│  └─────────────┴─────────────────┴─────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

**前端层（Web管理界面）：**
- 基于Next.js + React的现代化Web应用
- 提供直观的用户界面和交互体验
- 支持响应式设计，适配不同设备

**后端服务层（IARNet核心服务）：**
- 基于Go语言的高性能服务
- 提供RESTful API和gRPC服务
- 实现核心业务逻辑和资源管理

**资源层（底层计算资源）：**
- Docker容器运行时环境
- Kubernetes集群
- 分布式peer节点

### 2.2 部署模式

系统支持两种主要部署模式：

#### 2.2.1 Standalone模式

**特点：**
- 单节点部署，使用Docker作为容器运行时
- 适合小规模环境和开发测试
- 部署简单，资源需求较低

**架构：**
```
┌─────────────────────────────────────┐
│           IARNet Service            │
│  ┌─────────────┬─────────────────┐   │
│  │ HTTP Server │ Docker Runner   │   │
│  └─────────────┴─────────────────┘   │
└─────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│           Docker Engine             │
│  ┌─────────┬─────────┬─────────────┐ │
│  │Container│Container│  Container  │ │
│  │    A    │    B    │      C      │ │
│  └─────────┴─────────┴─────────────┘ │
└─────────────────────────────────────┘
```

#### 2.2.2 Kubernetes模式

**特点：**
- 集群部署，使用Kubernetes作为容器编排平台
- 适合生产环境和大规模部署
- 支持高可用和自动扩缩容

**架构：**
```
┌─────────────────────────────────────┐
│        Kubernetes Cluster          │
│  ┌─────────────────────────────────┐ │
│  │         IARNet Pods             │ │
│  │  ┌─────────┬─────────────────┐   │ │
│  │  │HTTP API │ K8s Runner      │   │ │
│  │  └─────────┴─────────────────┘   │ │
│  └─────────────────────────────────┘ │
│  ┌─────────────────────────────────┐ │
│  │        Application Pods         │ │
│  │  ┌─────────┬─────────┬───────┐   │ │
│  │  │  Pod A  │  Pod B  │ Pod C │   │ │
│  │  └─────────┴─────────┴───────┘   │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

### 2.3 技术选型

#### 2.3.1 后端技术栈

**核心语言和框架：**
- **Go 1.21+**：主要编程语言，高性能、并发友好
- **gorilla/mux**：HTTP路由和中间件
- **gRPC**：高性能RPC框架，用于节点间通信
- **Protocol Buffers**：数据序列化格式

**容器和编排：**
- **docker/go-docker**：Docker API客户端
- **k8s.io/client-go**：Kubernetes官方Go客户端
- **k8s.io/api**：Kubernetes API对象定义

**其他依赖：**
- **sirupsen/logrus**：结构化日志
- **gopkg.in/yaml.v2**：YAML配置解析

#### 2.3.2 前端技术栈

**核心框架：**
- **Next.js 15**：React全栈框架
- **React 19**：用户界面库
- **TypeScript**：类型安全的JavaScript

**UI和样式：**
- **Tailwind CSS**：原子化CSS框架
- **shadcn/ui**：现代化UI组件库
- **Lucide React**：图标库

**状态管理和工具：**
- **Zustand**：轻量级状态管理
- **React Hook Form**：表单处理
- **Zod**：数据验证

### 2.4 架构图

#### 2.4.1 前后端交互架构

```
┌─────────────────────────────────────┐
│            Browser                  │
│  ┌─────────────────────────────────┐ │
│  │         Next.js App             │ │
│  │  ┌─────────┬─────────────────┐   │ │
│  │  │Pages    │ Components      │   │ │
│  │  │         │ (shadcn/ui)     │   │ │
│  │  └─────────┴─────────────────┘   │ │
│  │  ┌─────────────────────────────┐ │ │
│  │  │     State Management        │ │ │
│  │  │       (Zustand)             │ │ │
│  │  └─────────────────────────────┘ │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
                  │
                  │ HTTP/WebSocket
                  ▼
┌─────────────────────────────────────┐
│         IARNet Backend              │
│  ┌─────────────────────────────────┐ │
│  │        HTTP Server              │ │
│  │     (gorilla/mux)               │ │
│  └─────────────────────────────────┘ │
│  ┌─────────────────────────────────┐ │
│  │      Business Logic             │ │
│  │  ┌─────────┬─────────────────┐   │ │
│  │  │Resource │ Container       │   │ │
│  │  │Manager  │ Runner          │   │ │
│  │  └─────────┴─────────────────┘   │ │
│  └─────────────────────────────────┘ │
│  ┌─────────────────────────────────┐ │
│  │       gRPC Server               │ │
│  │    (Peer Discovery)             │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

#### 2.4.2 资源层关系架构

```
┌─────────────────────────────────────┐
│          IARNet Cluster             │
│  ┌─────────┬─────────┬─────────────┐ │
│  │ Node A  │ Node B  │   Node C    │ │
│  │         │         │             │ │
│  └─────────┴─────────┴─────────────┘ │
└─────────────────────────────────────┘
                  │
                  │ gRPC Gossip Protocol
                  ▼
┌─────────────────────────────────────┐
│        Resource Discovery           │
│  ┌─────────────────────────────────┐ │
│  │     Peer Information            │ │
│  │  ┌─────────┬─────────────────┐   │ │
│  │  │Node List│ Health Status   │   │ │
│  │  │         │ Resource Usage  │   │ │
│  │  └─────────┴─────────────────┘   │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│       Container Orchestration       │
│  ┌─────────────┬─────────────────┐   │
│  │   Docker    │   Kubernetes    │   │
│  │   Engine    │    Cluster      │   │
│  └─────────────┴─────────────────┘   │
└─────────────────────────────────────┘
```

## 3. 后端服务设计（Go实现）

### 3.1 功能概述

后端服务是IARNet平台的核心，负责处理所有的业务逻辑和资源管理。主要功能包括：

#### 3.1.1 容器运行管理

**功能描述：**
- 支持Docker和Kubernetes两种容器运行时
- 提供统一的容器生命周期管理接口
- 支持容器镜像拉取、启动、停止、重启等操作

**核心特性：**
- 运行时抽象：通过Runner接口统一不同运行时的操作
- 容器规格定义：支持CPU、内存、GPU等资源规格
- 错误处理：完善的错误处理和重试机制

#### 3.1.2 资源配额控制

**功能描述：**
- 实现CPU、内存、GPU、存储等资源的配额管理
- 支持资源使用情况的实时监控
- 提供资源超限保护机制

**核心特性：**
- 资源限制：设置节点总体资源限制
- 使用跟踪：实时跟踪资源分配和使用情况
- 超限保护：拒绝超过资源限制的请求

#### 3.1.3 资源监控

**功能描述：**
- 收集容器和节点的资源使用数据
- 提供实时的监控指标
- 支持历史数据查询和分析

**核心特性：**
- 多维度监控：CPU、内存、网络、存储等
- 实时数据：低延迟的数据收集和更新
- 数据聚合：支持节点级和集群级数据聚合

#### 3.1.4 节点发现（Peer Discovery）

**功能描述：**
- 基于gRPC的分布式节点发现机制
- 支持动态节点加入和离开
- 实现gossip协议进行节点信息传播

**核心特性：**
- 自动发现：新节点自动加入集群
- 故障检测：检测和处理节点故障
- 信息同步：节点间信息的一致性保证

#### 3.1.5 HTTP API

**功能描述：**
- 提供RESTful API接口
- 支持容器操作、资源查询、状态监控等功能
- 实现API认证和授权机制

**核心特性：**
- 标准化接口：遵循REST设计原则
- 错误处理：统一的错误响应格式
- 文档化：完整的API文档

### 3.2 模块划分

#### 3.2.1 API层（gorilla/mux提供REST API）

**职责：**
- HTTP请求路由和处理
- 请求参数验证和解析
- 响应格式化和错误处理
- 中间件支持（认证、日志、CORS等）

**核心组件：**
```go
type Server struct {
    router *mux.Router
    runner runner.Runner
    resMgr *resource.ResourceManager
    ctx    context.Context
    cancel context.CancelFunc
}

func NewServer(r runner.Runner, rm *resource.ResourceManager) *Server {
    ctx, cancel := context.WithCancel(context.Background())
    s := &Server{
        router: mux.NewRouter(), 
        runner: r, 
        resMgr: rm, 
        ctx: ctx, 
        cancel: cancel
    }
    s.router.HandleFunc("/run", s.handleRun).Methods("POST")
    return s
}
```

#### 3.2.2 运行时管理层（Docker/Kubernetes集成）

**职责：**
- 抽象不同的容器运行时
- 提供统一的容器操作接口
- 处理运行时特定的逻辑

**核心接口：**
```go
type Runner interface {
    Run(ctx context.Context, spec ContainerSpec) error
    Stop(containerID string) error
    GetUsage() resource.ResourceUsage
}

type ContainerSpec struct {
    Image   string
    Command []string
    CPU     float64
    Memory  float64
    GPU     float64
}
```

**实现类：**
- `StandaloneRunner`：Docker运行时实现
- `K8sRunner`：Kubernetes运行时实现

#### 3.2.3 资源监控层（Docker Stats API/Pod Requests汇总）

**职责：**
- 收集容器和节点的资源使用数据
- 实现不同运行时的监控逻辑
- 提供统一的监控数据接口

**核心组件：**
```go
type ResourceManager struct {
    limits  ResourceUsage
    current ResourceUsage
    mu      sync.Mutex
}

func (rm *ResourceManager) CanAllocate(req ResourceUsage) bool
func (rm *ResourceManager) Allocate(req ResourceUsage)
func (rm *ResourceManager) Deallocate(req ResourceUsage)
```

#### 3.2.4 Peer Discovery层（gRPC Gossip）

**职责：**
- 实现节点间的服务发现
- 维护集群节点信息
- 处理节点加入和离开事件

**核心组件：**
```go
type PeerManager struct {
    peers map[string]struct{}
    mu    sync.Mutex
}

func (pm *PeerManager) GetPeers() []string
func (pm *PeerManager) AddPeers(newPeers []string)
func (pm *PeerManager) StartGossip(ctx context.Context)
```

**gRPC服务定义：**
```protobuf
service PeerService {
  rpc ExchangePeers(ExchangeRequest) returns (ExchangeResponse);
}

message ExchangeRequest {
  repeated string known_peers = 1;
}

message ExchangeResponse {
  repeated string known_peers = 1;
}
```

#### 3.2.5 调度与运行层（容器/Pod启动、停止）

**职责：**
- 实现容器的调度逻辑
- 处理容器的生命周期管理
- 协调资源分配和释放

**核心流程：**
1. 接收容器启动请求
2. 检查资源可用性
3. 选择合适的运行时
4. 启动容器
5. 更新资源使用情况

### 3.3 关键功能流程

#### 3.3.1 容器启动流程

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Client    │    │ HTTP Server │    │   Runner    │
└─────────────┘    └─────────────┘    └─────────────┘
        │                   │                   │
        │ POST /run         │                   │
        ├──────────────────►│                   │
        │                   │                   │
        │                   │ Validate Request  │
        │                   ├──────────────────►│
        │                   │                   │
        │                   │ Check Resources   │
        │                   ├──────────────────►│
        │                   │                   │
        │                   │ Start Container   │
        │                   ├──────────────────►│
        │                   │                   │
        │                   │ Update Usage      │
        │                   ├──────────────────►│
        │                   │                   │
        │ 202 Accepted      │                   │
        │◄──────────────────┤                   │
        │                   │                   │
```

**详细步骤：**

1. **请求接收**：HTTP服务器接收POST /run请求
2. **参数验证**：验证容器规格参数的有效性
3. **资源检查**：检查当前资源是否满足容器需求
4. **容器启动**：调用相应的Runner启动容器
5. **资源更新**：更新资源使用情况
6. **响应返回**：返回启动结果给客户端

#### 3.3.2 资源配额检查流程

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Request   │    │ Resource    │    │   Result    │
│             │    │ Manager     │    │             │
└─────────────┘    └─────────────┘    └─────────────┘
        │                   │                   │
        │ Check Resources   │                   │
        ├──────────────────►│                   │
        │                   │                   │
        │                   │ Get Current Usage │
        │                   ├──────────────────►│
        │                   │                   │
        │                   │ Calculate Total   │
        │                   ├──────────────────►│
        │                   │                   │
        │                   │ Compare Limits    │
        │                   ├──────────────────►│
        │                   │                   │
        │ Allow/Deny        │                   │
        │◄──────────────────┤                   │
        │                   │                   │
```

**检查逻辑：**
```go
func (rm *ResourceManager) CanAllocate(req ResourceUsage) bool {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    
    if rm.current.CPU+req.CPU > rm.limits.CPU ||
       rm.current.Memory+req.Memory > rm.limits.Memory ||
       rm.current.GPU+req.GPU > rm.limits.GPU {
        return false
    }
    return true
}
```

#### 3.3.3 节点发现流程

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Node A    │    │   Node B    │    │   Node C    │
└─────────────┘    └─────────────┘    └─────────────┘
        │                   │                   │
        │ Exchange Peers    │                   │
        ├──────────────────►│                   │
        │                   │                   │
        │ Known Peers List  │                   │
        │◄──────────────────┤                   │
        │                   │                   │
        │                   │ Exchange Peers    │
        │                   ├──────────────────►│
        │                   │                   │
        │                   │ Known Peers List  │
        │                   │◄──────────────────┤
        │                   │                   │
        │ Update Local List │                   │
        ├──────────────────►│                   │
        │                   │                   │
```

**Gossip实现：**
```go
func (pm *PeerManager) gossipOnce() {
    known := pm.GetPeers()
    for _, peerAddr := range known {
        conn, err := grpc.Dial(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
        if err != nil {
            continue
        }
        client := proto.NewPeerServiceClient(conn)
        resp, err := client.ExchangePeers(context.Background(), &proto.ExchangeRequest{KnownPeers: known})
        if err != nil {
            conn.Close()
            continue
        }
        pm.AddPeers(resp.KnownPeers)
        conn.Close()
    }
}
```

### 3.4 接口设计（API Spec）

#### 3.4.1 POST /run（启动应用容器）

**功能描述：**
启动一个新的容器实例

**请求格式：**
```http
POST /run
Content-Type: application/json

{
  "image": "nginx:latest",
  "command": ["nginx", "-g", "daemon off;"],
  "cpu": 1.0,
  "memory": 0.5,
  "gpu": 0
}
```

**请求参数：**
- `image` (string, required): 容器镜像名称
- `command` (array, optional): 容器启动命令
- `cpu` (float, required): CPU需求（核心数）
- `memory` (float, required): 内存需求（GB）
- `gpu` (float, optional): GPU需求（卡数）

**响应格式：**
```http
HTTP/1.1 202 Accepted
Content-Type: application/json

{
  "status": "accepted",
  "container_id": "abc123",
  "message": "Container started successfully"
}
```

**错误响应：**
```http
HTTP/1.1 503 Service Unavailable
Content-Type: application/json

{
  "error": "Resource limit exceeded",
  "details": "Insufficient CPU resources"
}
```

#### 3.4.2 GET /resources（查看资源使用情况）

**功能描述：**
获取当前节点的资源使用情况

**请求格式：**
```http
GET /resources
```

**响应格式：**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "limits": {
    "cpu": 8.0,
    "memory": 16.0,
    "gpu": 4.0
  },
  "current": {
    "cpu": 3.5,
    "memory": 8.2,
    "gpu": 1.0
  },
  "available": {
    "cpu": 4.5,
    "memory": 7.8,
    "gpu": 3.0
  },
  "utilization": {
    "cpu": 43.75,
    "memory": 51.25,
    "gpu": 25.0
  }
}
```

#### 3.4.3 GET /status（查看应用运行状态）

**功能描述：**
获取当前运行的容器状态信息

**请求格式：**
```http
GET /status
```

**响应格式：**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "containers": [
    {
      "id": "abc123",
      "image": "nginx:latest",
      "status": "running",
      "created_at": "2024-01-15T10:30:00Z",
      "resources": {
        "cpu": 1.0,
        "memory": 0.5,
        "gpu": 0
      }
    }
  ],
  "total_containers": 1,
  "node_info": {
    "id": "node-001",
    "address": "192.168.1.100:50051",
    "last_updated": "2024-01-15T10:35:00Z"
  }
}
```

### 3.5 运行模式说明

#### 3.5.1 Standalone模式假设

**环境假设：**
- Docker Engine已安装并运行
- 具有足够的权限操作Docker
- 网络连接正常，可以拉取镜像

**特点：**
- 单节点部署，配置简单
- 直接使用Docker API操作容器
- 资源监控基于Docker Stats API
- 适合开发测试和小规模部署

**配置示例：**
```yaml
mode: "standalone"
listen_addr: ":8080"
peer_listen_addr: ":50051"
resource_limits:
  cpu: "4"
  memory: "8Gi"
  gpu: "1"
```

#### 3.5.2 K8s模式假设

**环境假设：**
- 运行在Kubernetes集群中
- 具有创建和管理Pod的RBAC权限
- 集群网络配置正确

**特点：**
- 集群部署，支持高可用
- 使用Kubernetes API操作Pod
- 资源监控基于Pod资源请求汇总
- 适合生产环境和大规模部署

**RBAC配置：**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: iarnet-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["create", "delete", "get", "list", "watch"]
```

#### 3.5.3 差异点

**容器启动逻辑差异：**

*Standalone模式：*
```go
func (r *StandaloneRunner) Run(ctx context.Context, spec ContainerSpec) error {
    resp, err := r.docker.ContainerCreate(ctx, &container.Config{
        Image: spec.Image,
        Cmd:   spec.Command,
    }, &container.HostConfig{
        Resources: container.Resources{
            CPUQuota: int64(spec.CPU * 100000),
            Memory:   int64(spec.Memory * 1024 * 1024 * 1024),
        },
    }, nil, nil, "")
    if err != nil {
        return err
    }
    return r.docker.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
}
```

*K8s模式：*
```go
func (r *K8sRunner) Run(ctx context.Context, spec ContainerSpec) error {
    pod := &v1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            GenerateName: "iarnet-pod-",
        },
        Spec: v1.PodSpec{
            Containers: []v1.Container{{
                Name:    "container",
                Image:   spec.Image,
                Command: spec.Command,
                Resources: v1.ResourceRequirements{
                    Requests: v1.ResourceList{
                        v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%f", spec.CPU)),
                        v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%fGi", spec.Memory)),
                    },
                },
            }},
        },
    }
    _, err := r.clientset.CoreV1().Pods(r.namespace).Create(ctx, pod, metav1.CreateOptions{})
    return err
}
```

**资源监控实现差异：**

*Standalone模式：*
- 使用Docker Stats API获取实时资源使用情况
- 直接读取容器的CPU、内存使用率
- 监控粒度为容器级别

*K8s模式：*
- 汇总Pod的资源请求作为使用量
- 通过Kubernetes Metrics API获取实际使用情况
- 监控粒度为Pod级别

## 4. 前端Web管理工具设计

### 4.1 功能概述

前端Web管理工具是IARNet平台的用户界面，提供直观的可视化管理功能。基于Next.js + React技术栈构建，采用现代化的设计理念和用户体验。

**核心功能：**

#### 4.1.1 资源接入管理

**功能描述：**
- 添加和管理算力资源节点
- 配置资源连接参数
- 监控资源连接状态
- 查看资源使用情况

**主要特性：**
- 支持多种资源类型（Kubernetes、Docker、VM）
- 实时连接状态监控
- 资源使用率可视化
- 批量资源操作

#### 4.1.2 应用管理

**功能描述：**
- 从Git仓库导入应用
- 配置应用部署参数
- 管理应用生命周期
- 监控应用运行状态

**主要特性：**
- Git集成（GitHub、GitLab、Bitbucket）
- 可视化应用卡片
- 一键部署和停止
- 应用配置管理

#### 4.1.3 运行状态管理

**功能描述：**
- 实时监控应用运行状态
- 查看资源使用情况
- 管理应用实例
- 查看运行日志

**主要特性：**
- 实时状态更新
- 资源使用图表
- 日志查看和搜索
- 性能指标监控

### 4.2 页面设计

#### 4.2.1 算力资源接入管理页面

**页面布局：**
```
┌─────────────────────────────────────────────────────────────┐
│                        页面标题                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │总资源数  │  │在线资源  │  │CPU核心   │  │    总存储       │ │
│  │   12    │  │   10    │  │  256    │  │    2.5TB       │ │
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐ │
│  │                   资源列表                              │ │
│  │ ┌─────┬─────────┬─────┬─────┬─────────┬─────────────┐   │ │
│  │ │名称 │  类型   │状态 │CPU  │  内存   │    操作     │   │ │
│  │ ├─────┼─────────┼─────┼─────┼─────────┼─────────────┤   │ │
│  │ │集群A│Kubernetes│连接 │32核 │ 128GB   │编辑│删除│刷新│   │ │
│  │ │集群B│ Docker  │离线 │16核 │  64GB   │编辑│删除│刷新│   │ │
│  │ └─────┴─────────┴─────┴─────┴─────────┴─────────────┘   │ │
│  └─────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐                                        │
│  │   添加资源      │                                        │
│  └─────────────────┘                                        │
└─────────────────────────────────────────────────────────────┘
```

**表单设计：输入API Server URL和Token**

```typescript
interface ResourceFormData {
  name: string          // 资源名称
  type: 'kubernetes' | 'docker' | 'vm'  // 资源类型
  url: string           // API Server URL
  token: string         // 认证Token
  description?: string  // 描述信息
}

// 表单验证规则
const resourceSchema = z.object({
  name: z.string().min(1, '资源名称不能为空'),
  type: z.enum(['kubernetes', 'docker', 'vm']),
  url: z.string().url('请输入有效的URL'),
  token: z.string().min(1, 'Token不能为空'),
  description: z.string().optional(),
})
```

**添加资源对话框：**
```tsx
function AddResourceDialog() {
  const form = useForm<ResourceFormData>({
    resolver: zodResolver(resourceSchema),
    defaultValues: {
      name: '',
      type: 'kubernetes',
      url: '',
      token: '',
      description: '',
    },
  })

  const onSubmit = async (data: ResourceFormData) => {
    try {
      await api.addResource(data)
      toast.success('资源添加成功')
      form.reset()
    } catch (error) {
      toast.error('资源添加失败')
    }
  }

  return (
    <Dialog>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>添加算力资源</DialogTitle>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)}>
            <FormField name="name" label="资源名称" />
            <FormField name="type" label="资源类型" type="select" />
            <FormField name="url" label="API Server URL" />
            <FormField name="token" label="认证Token" type="password" />
            <FormField name="description" label="描述" type="textarea" />
            <DialogFooter>
              <Button type="submit">添加资源</Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
```

**资源列表展示：**
```tsx
function ResourceList() {
  const { resources, loading } = useResources()

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>名称</TableHead>
          <TableHead>类型</TableHead>
          <TableHead>状态</TableHead>
          <TableHead>CPU</TableHead>
          <TableHead>内存</TableHead>
          <TableHead>操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {resources.map((resource) => (
          <TableRow key={resource.id}>
            <TableCell>{resource.name}</TableCell>
            <TableCell>
              <Badge variant={getTypeVariant(resource.type)}>
                {resource.type}
              </Badge>
            </TableCell>
            <TableCell>
              <StatusBadge status={resource.status} />
            </TableCell>
            <TableCell>{resource.cpu.total}核</TableCell>
            <TableCell>{resource.memory.total}GB</TableCell>
            <TableCell>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="sm">
                    <MoreHorizontal className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent>
                  <DropdownMenuItem onClick={() => editResource(resource.id)}>
                    编辑
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => deleteResource(resource.id)}>
                    删除
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => refreshResource(resource.id)}>
                    刷新
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
```

#### 4.2.2 应用管理页面

**页面布局：**
```
┌─────────────────────────────────────────────────────────────┐
│                      应用管理                               │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │应用总数  │  │运行中   │  │已停止   │  │    错误         │ │
│  │   25    │  │   18    │  │   5     │  │     2          │ │
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐                  │
│  │   快速导入      │  │   高级配置      │                  │
│  └─────────────────┘  └─────────────────┘                  │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐ │
│  │                   应用卡片网格                          │ │
│  │ ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐ │ │
│  │ │    App A    │ │    App B    │ │       App C         │ │ │
│  │ │   Web应用   │ │   API服务   │ │      数据库         │ │ │
│  │ │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌─────────────────┐ │ │ │
│  │ │ │  运行   │ │ │ │  停止   │ │ │ │     部署        │ │ │ │
│  │ │ └─────────┘ │ │ └─────────┘ │ │ └─────────────────┘ │ │ │
│  │ └─────────────┘ └─────────────┘ └─────────────────────┘ │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

**应用导入：Git URL输入框**

```tsx
function QuickImportDialog() {
  const [gitUrl, setGitUrl] = useState('')
  const [loading, setLoading] = useState(false)

  const handleImport = async () => {
    if (!gitUrl) return
    
    setLoading(true)
    try {
      const appInfo = await api.parseGitRepository(gitUrl)
      // 自动填充应用信息
      await api.createApplication({
        name: appInfo.name,
        gitUrl: gitUrl,
        branch: 'main',
        type: appInfo.detectedType,
      })
      toast.success('应用导入成功')
      setGitUrl('')
    } catch (error) {
      toast.error('应用导入失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>快速导入应用</DialogTitle>
          <DialogDescription>
            输入Git仓库URL，系统将自动解析应用信息
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div>
            <Label htmlFor="git-url">Git仓库URL</Label>
            <Input
              id="git-url"
              placeholder="https://github.com/username/repo.git"
              value={gitUrl}
              onChange={(e) => setGitUrl(e.target.value)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button 
            onClick={handleImport} 
            disabled={!gitUrl || loading}
          >
            {loading ? '导入中...' : '导入应用'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

**应用卡片：运行/停止按钮**

```tsx
function ApplicationCard({ app }: { app: Application }) {
  const [isDeploying, setIsDeploying] = useState(false)
  
  const handleDeploy = async () => {
    setIsDeploying(true)
    try {
      await api.deployApplication(app.id)
      toast.success('应用部署成功')
    } catch (error) {
      toast.error('应用部署失败')
    } finally {
      setIsDeploying(false)
    }
  }

  const handleStop = async () => {
    try {
      await api.stopApplication(app.id)
      toast.success('应用已停止')
    } catch (error) {
      toast.error('停止应用失败')
    }
  }

  return (
    <Card className="w-full max-w-sm">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-lg">{app.name}</CardTitle>
          <Badge variant={getStatusVariant(app.status)}>
            {app.status}
          </Badge>
        </div>
        <CardDescription>{app.description}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="space-y-2">
          <div className="flex items-center text-sm text-muted-foreground">
            <GitBranch className="mr-2 h-4 w-4" />
            {app.branch}
          </div>
          <div className="flex items-center text-sm text-muted-foreground">
            <Package className="mr-2 h-4 w-4" />
            {app.type}
          </div>
          {app.lastDeployed && (
            <div className="flex items-center text-sm text-muted-foreground">
              <Clock className="mr-2 h-4 w-4" />
              {formatDistanceToNow(new Date(app.lastDeployed))}前部署
            </div>
          )}
        </div>
      </CardContent>
      <CardFooter>
        {app.status === 'idle' || app.status === 'stopped' ? (
          <Button 
            onClick={handleDeploy} 
            disabled={isDeploying}
            className="w-full"
          >
            {isDeploying ? '部署中...' : '运行'}
          </Button>
        ) : app.status === 'running' ? (
          <Button 
            onClick={handleStop} 
            variant="destructive"
            className="w-full"
          >
            停止
          </Button>
        ) : (
          <Button disabled className="w-full">
            {app.status}
          </Button>
        )}
      </CardFooter>
    </Card>
  )
}
```

#### 4.2.3 运行状态页面

**页面布局：**
```
┌─────────────────────────────────────────────────────────────┐
│                      运行状态监控                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │运行中   │  │  警告   │  │  错误   │  │    总实例       │ │
│  │   18    │  │   3     │  │   1     │  │     22         │ │
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐ │
│  │                   应用状态表格                          │ │
│  │ ┌─────┬─────┬─────┬─────┬─────┬─────┬─────────────────┐ │ │
│  │ │应用 │状态 │运行 │CPU  │内存 │网络 │     操作        │ │ │
│  │ │名称 │     │节点 │使用 │使用 │流量 │                 │ │ │
│  │ ├─────┼─────┼─────┼─────┼─────┼─────┼─────────────────┤ │ │
│  │ │AppA │运行 │节点1│45%  │62%  │1.2M │查看│重启│停止  │ │ │
│  │ │AppB │警告 │节点2│78%  │89%  │2.5M │查看│重启│停止  │ │ │
│  │ └─────┴─────┴─────┴─────┴─────┴─────┴─────────────────┘ │ │
│  └─────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────────────────────────┐ │
│  │   自动刷新      │  │            拓扑图                   │ │
│  │   ⏱ 30秒       │  │                                     │ │
│  └─────────────────┘  └─────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

**表格：应用状态、运行节点、资源使用情况**

```tsx
function ApplicationStatusTable() {
  const { applicationStatuses, loading } = useApplicationStatuses()
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [refreshInterval, setRefreshInterval] = useState(30)

  // 自动刷新逻辑
  useEffect(() => {
    if (!autoRefresh) return
    
    const interval = setInterval(() => {
      // 刷新数据
      refetch()
    }, refreshInterval * 1000)
    
    return () => clearInterval(interval)
  }, [autoRefresh, refreshInterval])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold">应用运行状态</h3>
        <div className="flex items-center space-x-2">
          <Switch
            checked={autoRefresh}
            onCheckedChange={setAutoRefresh}
          />
          <Label>自动刷新</Label>
          <Select value={refreshInterval.toString()} onValueChange={(v) => setRefreshInterval(Number(v))}>
            <SelectTrigger className="w-20">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="10">10秒</SelectItem>
              <SelectItem value="30">30秒</SelectItem>
              <SelectItem value="60">1分钟</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>应用名称</TableHead>
            <TableHead>状态</TableHead>
            <TableHead>运行时间</TableHead>
            <TableHead>CPU使用</TableHead>
            <TableHead>内存使用</TableHead>
            <TableHead>网络流量</TableHead>
            <TableHead>运行节点</TableHead>
            <TableHead>操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {applicationStatuses.map((status) => (
            <TableRow key={status.id}>
              <TableCell className="font-medium">{status.name}</TableCell>
              <TableCell>
                <div className="flex items-center space-x-2">
                  <StatusIndicator status={status.status} />
                  <span>{status.status}</span>
                </div>
              </TableCell>
              <TableCell>{status.uptime}</TableCell>
              <TableCell>
                <div className="flex items-center space-x-2">
                  <Progress value={status.cpu} className="w-16" />
                  <span className="text-sm">{status.cpu}%</span>
                </div>
              </TableCell>
              <TableCell>
                <div className="flex items-center space-x-2">
                  <Progress value={status.memory} className="w-16" />
                  <span className="text-sm">{status.memory}%</span>
                </div>
              </TableCell>
              <TableCell>{formatBytes(status.network)}/s</TableCell>
              <TableCell>
                 <div className="flex flex-wrap gap-1">
                   {status.runningOn.map((node) => (
                     <Badge key={node} variant="outline" className="text-xs">
                       {node}
                     </Badge>
                   ))}
                 </div>
               </TableCell>
               <TableCell>
                 <DropdownMenu>
                   <DropdownMenuTrigger asChild>
                     <Button variant="ghost" size="sm">
                       <MoreHorizontal className="h-4 w-4" />
                     </Button>
                   </DropdownMenuTrigger>
                   <DropdownMenuContent>
                     <DropdownMenuItem onClick={() => viewLogs(status.id)}>
                       查看日志
                     </DropdownMenuItem>
                     <DropdownMenuItem onClick={() => restartApplication(status.id)}>
                       重启
                     </DropdownMenuItem>
                     <DropdownMenuItem onClick={() => stopApplication(status.id)}>
                       停止
                     </DropdownMenuItem>
                   </DropdownMenuContent>
                 </DropdownMenu>
               </TableCell>
             </TableRow>
           ))}
         </TableBody>
       </Table>
     </div>
   )
 }
 ```

 ### 4.3 与后端交互流程

 #### 4.3.1 添加资源流程

 **流程描述：**
 前端表单 → HTTP API → 后端验证 → 返回状态

 ```
 ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
 │   前端表单   │    │  HTTP API   │    │   后端验证   │
 └─────────────┘    └─────────────┘    └─────────────┘
         │                   │                   │
         │ 提交资源信息       │                   │
         ├──────────────────►│                   │
         │                   │ POST /api/resources│
         │                   ├──────────────────►│
         │                   │                   │
         │                   │ 验证URL和Token     │
         │                   │◄──────────────────┤
         │                   │                   │
         │                   │ 测试连接          │
         │                   │◄──────────────────┤
         │                   │                   │
         │ 返回结果          │                   │
         │◄──────────────────┤                   │
         │                   │                   │
 ```

 #### 4.3.2 部署应用流程

 **流程描述：**
 前端点击运行 → API请求 → 后端启动容器/Pod → 返回运行状态

 ```
 ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
 │  前端操作   │    │  API请求    │    │  后端处理   │
 └─────────────┘    └─────────────┘    └─────────────┘
         │                   │                   │
         │ 点击运行按钮       │                   │
         ├──────────────────►│                   │
         │                   │ POST /api/deploy  │
         │                   ├──────────────────►│
         │                   │                   │
         │                   │ 选择资源节点       │
         │                   │◄──────────────────┤
         │                   │                   │
         │                   │ 启动容器/Pod      │
         │                   │◄──────────────────┤
         │                   │                   │
         │ 更新UI状态        │                   │
         │◄──────────────────┤                   │
         │                   │                   │
 ```

 #### 4.3.3 状态查询流程

 **流程描述：**
 前端轮询/WebSocket → 后端实时状态 → 前端更新显示

 ```
 ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
 │  前端轮询   │    │  WebSocket  │    │  状态更新   │
 └─────────────┘    └─────────────┘    └─────────────┘
         │                   │                   │
         │ 建立WebSocket连接  │                   │
         ├──────────────────►│                   │
         │                   │                   │
         │                   │ 订阅状态更新       │
         │                   ├──────────────────►│
         │                   │                   │
         │                   │ 推送状态变化       │
         │                   │◄──────────────────┤
         │                   │                   │
         │ 更新UI显示        │                   │
         │◄──────────────────┤                   │
         │                   │                   │
 ```

 ## 5. 数据模型设计

 ### 5.1 Resource数据结构

 ```typescript
 interface Resource {
   id: string                    // 资源唯一标识
   name: string                  // 资源名称
   type: 'kubernetes' | 'docker' | 'vm'  // 资源类型
   url: string                   // API Server URL
   token: string                 // 认证Token
   status: 'connected' | 'disconnected' | 'error'  // 连接状态
   
   // 资源容量信息
   cpu: {
     total: number               // 总CPU核心数
     used: number                // 已使用CPU核心数
     available: number           // 可用CPU核心数
   }
   
   memory: {
     total: number               // 总内存(GB)
     used: number                // 已使用内存(GB)
     available: number           // 可用内存(GB)
   }
   
   storage: {
     total: number               // 总存储(GB)
     used: number                // 已使用存储(GB)
     available: number           // 可用存储(GB)
   }
   
   gpu?: {
     total: number               // 总GPU数量
     used: number                // 已使用GPU数量
     available: number           // 可用GPU数量
   }
   
   // 元数据
   description?: string          // 资源描述
   tags?: string[]              // 资源标签
   location?: string            // 资源位置
   lastUpdated: string          // 最后更新时间
   createdAt: string            // 创建时间
 }
 ```

 ### 5.2 Application数据结构

 ```typescript
 interface Application {
   id: string                    // 应用唯一标识
   name: string                  // 应用名称
   description: string           // 应用描述
   
   // Git信息
   gitUrl: string               // Git仓库URL
   branch: string               // Git分支
   commitHash?: string          // 提交哈希
   
   // 应用配置
   type: 'web' | 'api' | 'worker' | 'database'  // 应用类型
   image?: string               // 容器镜像
   command?: string[]           // 启动命令
   env?: Record<string, string> // 环境变量
   
   // 资源需求
   resources: {
     cpu: number                 // CPU需求(核心数)
     memory: number              // 内存需求(GB)
     gpu?: number                // GPU需求(卡数)
     storage?: number            // 存储需求(GB)
   }
   
   // 网络配置
   ports?: {
     containerPort: number       // 容器端口
     servicePort?: number        // 服务端口
     protocol: 'TCP' | 'UDP'     // 协议类型
   }[]
   
   // 健康检查
   healthCheck?: {
     path: string                // 健康检查路径
     port: number                // 健康检查端口
     interval: number            // 检查间隔(秒)
     timeout: number             // 超时时间(秒)
   }
   
   // 运行状态
   status: 'idle' | 'running' | 'stopped' | 'error' | 'deploying'
   instances?: number           // 实例数量
   runningOn?: string[]         // 运行的资源节点
   
   // 时间戳
   lastDeployed?: string        // 最后部署时间
   createdAt: string            // 创建时间
   updatedAt: string            // 更新时间
 }
 ```

 ### 5.3 ApplicationStatus数据结构

 ```typescript
 interface ApplicationStatus {
   id: string                    // 应用ID
   name: string                  // 应用名称
   status: 'running' | 'stopped' | 'error' | 'warning'  // 运行状态
   
   // 运行时信息
   uptime: string               // 运行时间
   instances: number            // 实例数量
   runningOn: string[]          // 运行节点
   
   // 资源使用情况
   cpu: number                  // CPU使用率(%)
   memory: number               // 内存使用率(%)
   network: number              // 网络使用率(bytes/s)
   storage: number              // 存储使用率(%)
   
   // 性能指标
   metrics: {
     requestsPerSecond?: number  // 每秒请求数
     responseTime?: number       // 响应时间(ms)
     errorRate?: number          // 错误率(%)
   }
   
   // 健康状态
   health: 'healthy' | 'unhealthy' | 'unknown'
   healthChecks: {
     name: string               // 检查名称
     status: 'pass' | 'fail'    // 检查状态
     message?: string           // 状态消息
     lastCheck: string          // 最后检查时间
   }[]
   
   // 日志信息
   logs: {
     timestamp: string          // 时间戳
     level: 'info' | 'warn' | 'error'  // 日志级别
     message: string            // 日志消息
     source?: string            // 日志源
   }[]
   
   // 时间戳
   lastUpdated: string          // 最后更新时间
 }
 ```

 ## 6. 部署与运维

 ### 6.1 构建与运行

 #### 6.1.1 Go后端构建

 ```bash
 # 构建二进制文件
 go build -o iarnet ./cmd/main.go
 
 # 交叉编译
 GOOS=linux GOARCH=amd64 go build -o iarnet-linux ./cmd/main.go
 
 # 使用Docker构建
 docker build -t iarnet:latest .
 ```

 #### 6.1.2 Docker Compose部署

 ```yaml
 # docker-compose.yml
 version: '3.8'
 
 services:
   iarnet:
     build: .
     ports:
       - "8080:8080"
       - "50051:50051"
     volumes:
       - ./config.yaml:/config.yaml
       - /var/run/docker.sock:/var/run/docker.sock
     environment:
       - LOG_LEVEL=info
     restart: unless-stopped
 
   web:
     build: ./web
     ports:
       - "3000:3000"
     environment:
       - NEXT_PUBLIC_API_URL=http://localhost:8080
     depends_on:
       - iarnet
     restart: unless-stopped
 ```

 #### 6.1.3 Helm Chart部署

 ```yaml
 # helm/iarnet/values.yaml
 replicaCount: 3
 
 image:
   repository: iarnet
   tag: latest
   pullPolicy: IfNotPresent
 
 service:
   type: ClusterIP
   port: 8080
   grpcPort: 50051
 
 ingress:
   enabled: true
   className: nginx
   hosts:
     - host: iarnet.example.com
       paths:
         - path: /
           pathType: Prefix
 
 resources:
   limits:
     cpu: 500m
     memory: 512Mi
   requests:
     cpu: 100m
     memory: 128Mi
 
 config:
   mode: k8s
   resourceLimits:
     cpu: "100"
     memory: "200Gi"
     gpu: "20"
 ```

 ### 6.2 日志与监控

 #### 6.2.1 日志配置

 ```yaml
 # 日志配置
 logging:
   level: info
   format: json
   output: stdout
   
   # 日志轮转
   rotation:
     maxSize: 100MB
     maxAge: 30
     maxBackups: 10
     compress: true
 ```

 #### 6.2.2 Prometheus监控

 ```yaml
 # prometheus.yml
 global:
   scrape_interval: 15s
 
 scrape_configs:
   - job_name: 'iarnet'
     static_configs:
       - targets: ['iarnet:8080']
     metrics_path: '/metrics'
     scrape_interval: 10s
 ```

 ### 6.3 Peer节点扩展方式

 #### 6.3.1 手动添加节点

 ```bash
 # 在新节点上启动IARNet服务
 ./iarnet --config=config.yaml --initial-peers=node1:50051,node2:50051
 ```

 #### 6.3.2 自动发现配置

 ```yaml
 # 使用服务发现
 discovery:
   method: "consul"  # 或 "etcd", "dns"
   consul:
     address: "consul:8500"
     service_name: "iarnet"
 ```

 ### 6.4 升级策略

 #### 6.4.1 滚动升级

 ```bash
 # Kubernetes滚动升级
 kubectl set image deployment/iarnet iarnet=iarnet:v2.0.0
 kubectl rollout status deployment/iarnet
 ```

 #### 6.4.2 蓝绿部署

 ```yaml
 # 蓝绿部署配置
 apiVersion: argoproj.io/v1alpha1
 kind: Rollout
 metadata:
   name: iarnet
 spec:
   strategy:
     blueGreen:
       activeService: iarnet-active
       previewService: iarnet-preview
       autoPromotionEnabled: false
 ```

 ## 7. 安全设计

 ### 7.1 API Token验证

 ```go
 // JWT Token验证中间件
 func JWTAuthMiddleware(next http.Handler) http.Handler {
     return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
         authHeader := r.Header.Get("Authorization")
         if authHeader == "" {
             http.Error(w, "Missing authorization header", http.StatusUnauthorized)
             return
         }
         
         tokenString := strings.TrimPrefix(authHeader, "Bearer ")
         token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
             return []byte(secretKey), nil
         })
         
         if err != nil || !token.Valid {
             http.Error(w, "Invalid token", http.StatusUnauthorized)
             return
         }
         
         next.ServeHTTP(w, r)
     })
 }
 ```

 ### 7.2 TLS支持

 ```yaml
 # TLS配置
 tls:
   enabled: true
   cert_file: "/etc/ssl/certs/iarnet.crt"
   key_file: "/etc/ssl/private/iarnet.key"
   
 # gRPC TLS配置
 grpc:
   tls:
     enabled: true
     cert_file: "/etc/ssl/certs/grpc.crt"
     key_file: "/etc/ssl/private/grpc.key"
 ```

 ```go
 // TLS服务器配置
 func createTLSServer(handler http.Handler, certFile, keyFile string) *http.Server {
     tlsConfig := &tls.Config{
         MinVersion: tls.VersionTLS12,
         CipherSuites: []uint16{
             tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
             tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
             tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
         },
     }
     
     server := &http.Server{
         Handler:   handler,
         TLSConfig: tlsConfig,
     }
     
     return server
 }
 ```

 ## 8. 总结

 IARNet是一个功能完整、架构清晰的算力网络资源管理平台。通过Go语言的高性能特性和现代化的Web技术栈，为用户提供了统一的资源管理、应用部署和状态监控功能。

 ### 8.1 核心优势

 1. **技术先进**：采用Go + React的现代化技术栈，充分利用各自的优势
 2. **架构合理**：模块化、微服务化的系统架构，易于扩展和维护
 3. **功能完整**：覆盖资源管理、应用部署、状态监控的完整流程
 4. **部署灵活**：支持Standalone和Kubernetes两种部署模式
 5. **扩展性强**：支持水平扩展和动态节点发现

 ### 8.2 应用场景

 - **企业私有云**：统一管理企业内部的计算资源
 - **混合云环境**：跨云平台的资源管理和应用部署
 - **边缘计算**：分布式边缘节点的统一管理
 - **开发测试**：快速搭建开发测试环境
 - **CI/CD集成**：与持续集成流水线深度集成

 ### 8.3 发展前景

 随着云原生技术的不断发展和企业数字化转型的深入推进，IARNet作为算力资源管理平台具有广阔的发展前景。通过持续的技术创新和功能完善，将为用户提供更加智能、高效、安全的算力资源管理解决方案。