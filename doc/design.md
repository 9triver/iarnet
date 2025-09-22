# IARNet 算力网络资源管理与应用运行平台设计文档

## 1. 项目概述

### 1.1 项目背景

IARNet（Intelligent Application Resource Network）是一个基于Go语言开发的算力网络资源管理与应用运行平台。随着云计算、边缘计算和容器化技术的快速发展，企业面临着日益复杂的分布式算力资源管理挑战。传统的资源管理方式已无法满足现代应用对弹性、高效和智能化资源调度的需求，迫切需要一个统一、智能的平台来管理和调度各种异构计算资源。

**市场背景与技术趋势：**
- **云原生技术普及**：Kubernetes、Docker等容器技术成为主流，但多集群管理复杂度急剧上升
- **边缘计算兴起**：5G、IoT等技术推动计算向边缘扩展，需要统一管理云-边-端资源
- **AI/ML工作负载增长**：机器学习训练和推理对GPU等专用硬件需求激增
- **混合云架构普及**：企业采用多云策略，需要跨云资源统一管理
- **成本优化压力**：资源利用率低下导致成本浪费，需要智能调度优化

该项目旨在解决以下核心痛点：

**资源管理层面：**
- 分散的算力资源难以统一管理和调度
- 异构资源（CPU、GPU、FPGA等）缺乏统一抽象
- 跨集群、跨云的资源可见性不足
- 资源配额和权限管理复杂

**应用部署层面：**
- 容器化应用部署流程复杂，缺乏标准化
- 多环境（开发、测试、生产）配置管理困难
- 应用依赖关系复杂，部署失败率高
- 缺乏应用生命周期自动化管理

**运维监控层面：**
- 资源利用率监控不足，无法及时发现瓶颈
- 缺乏智能的资源调度和自动扩缩容机制
- 故障诊断和根因分析能力不足
- 缺乏统一的日志和指标收集体系

**成本控制层面：**
- 资源闲置和浪费现象严重
- 缺乏精细化的成本分摊和计费机制
- 无法预测和优化资源使用成本

### 1.2 目标与价值

**核心目标：**

**统一资源管理：**
- 提供跨云、跨集群的统一算力资源管理平台
- 支持异构资源（CPU、GPU、内存、存储、网络）的统一抽象和调度
- 实现资源池化，提高资源利用效率
- 支持资源的动态分配和回收

**智能应用编排：**
- 支持容器化应用的快速部署、扩缩容和运行
- 提供声明式的应用配置和依赖管理
- 实现应用的自动化生命周期管理
- 支持多种部署策略（蓝绿、金丝雀、滚动更新）

**智能调度优化：**
- 实现基于AI的智能资源调度和配额控制
- 支持多维度调度策略（性能、成本、能耗、地理位置）
- 提供预测性扩缩容和资源预留机制
- 实现工作负载的智能分布和负载均衡

**全面可观测性：**
- 提供实时的资源监控、应用状态管理和性能分析
- 构建统一的日志、指标和链路追踪体系
- 支持自定义告警规则和智能异常检测
- 提供丰富的可视化Dashboard和报表

**多模式部署：**
- 支持Standalone、Kubernetes、混合云等多种部署模式
- 提供灵活的网络拓扑和安全策略配置
- 支持边缘计算场景下的分布式部署
- 实现跨环境的配置同步和迁移

**业务价值：**

**成本效益：**
- **降低运维成本60%**：通过自动化运维减少人工干预，降低运维人员工作量
- **提高资源利用率40%**：智能调度和资源池化，减少资源闲置和浪费
- **减少部署时间80%**：标准化部署流程，从小时级缩短到分钟级
- **降低故障恢复时间70%**：自动化故障检测和恢复机制

**技术优势：**
- **加速应用交付**：DevOps流水线集成，支持CI/CD自动化部署
- **增强系统稳定性**：多副本、自动故障转移、健康检查机制
- **提升开发效率**：统一的开发、测试、生产环境，减少环境差异问题
- **支持技术演进**：云原生架构，支持微服务、Serverless等新技术

**业务敏捷性：**
- **快速响应市场变化**：弹性扩缩容，快速应对业务峰值
- **支持创新实验**：快速环境搭建，支持新技术验证和A/B测试
- **增强竞争优势**：技术平台化，释放业务团队创新能力
- **提升用户体验**：高可用架构，保障服务稳定性和响应速度

**合规与安全：**
- **增强安全防护**：多层次安全机制，网络隔离、访问控制、数据加密
- **满足合规要求**：审计日志、权限管理、数据治理能力
- **提升灾备能力**：多地域部署、数据备份、业务连续性保障
- **风险控制**：资源配额限制、异常检测、自动化响应机制

### 1.3 系统整体功能

**核心功能模块：**

1. **异构资源提供者管理（Multi-Provider Architecture）**
   
   系统采用创新的多提供者架构，支持同时管理和调度多种异构计算资源：
   
   **本地资源提供者（Local Provider）：**
   - 自动检测并接入本地Docker环境
   - 提供节点内部的容器运行能力
   - 支持本地资源的直接管理和监控
   
   **托管资源提供者（Managed Providers）：**
   - 支持远程Docker主机的统一接入和管理
   - 支持Kubernetes集群的原生集成
   - 提供跨云、跨集群的资源统一抽象
   - 支持异构硬件资源（CPU、GPU、FPGA等）的统一调度
   
   **协作资源提供者（Collaborative Providers）：**
   - 基于Gossip协议的P2P资源发现机制
   - 动态发现和接入网络中的其他IARNet节点
   - 实现真正的分布式算力网络协作
   - 支持资源的动态共享和负载均衡
   
   **智能资源调度：**
   - 统一的资源抽象层，屏蔽底层异构性
   - 基于容量、性能、地理位置的智能调度算法
   - 支持资源预留、配额控制和优先级调度
   - 实现跨提供者的工作负载迁移和故障转移

2. **容器生命周期管理**
   - **多运行时支持**：无缝支持Docker、Kubernetes等容器运行时
   - **统一部署接口**：提供声明式的容器部署规范
   - **生命周期自动化**：容器启动、停止、重启、扩缩容的全自动化管理
   - **镜像管理**：支持多仓库镜像拉取、版本控制和缓存优化
   - **依赖管理**：智能处理容器间的依赖关系和启动顺序
   - **健康检查**：内置健康检查机制，自动故障检测和恢复

3. **分布式资源配额与监控**
   - **多维度资源控制**：CPU、内存、GPU、存储、网络带宽的精细化配额管理
   - **实时监控体系**：基于时间序列的资源使用监控和性能分析
   - **智能告警机制**：支持自定义告警规则和异常检测
   - **资源超限保护**：自动资源回收和限流保护机制
   - **成本分析**：提供资源使用成本分析和优化建议
   - **预测性扩缩容**：基于历史数据和机器学习的资源需求预测

4. **P2P网络发现与协作**
   - **Gossip协议实现**：基于gRPC的高效P2P通信协议
   - **动态节点管理**：支持节点的动态加入、离开和故障恢复
   - **网络拓扑感知**：智能感知网络拓扑和延迟特性
   - **安全通信**：支持TLS加密和身份认证机制
   - **负载均衡**：智能的请求路由和负载分发
   - **数据一致性**：分布式状态同步和一致性保证

5. **服务间通信管理**
   - **多协议支持**：HTTP、gRPC、消息队列、文件传输等多种通信方式
   - **服务发现**：自动化的服务注册和发现机制
   - **路由管理**：智能的服务路由和流量控制
   - **通信优化**：连接池、重试机制、超时控制等性能优化
   - **监控追踪**：完整的通信链路追踪和性能监控

6. **现代化Web管理界面**
   - **响应式设计**：基于Next.js + React的现代化单页应用
   - **实时监控Dashboard**：资源使用、应用状态的实时可视化
   - **拖拽式应用编排**：可视化的应用组件编排和依赖配置
   - **多维度资源视图**：支持集群、节点、应用等多个维度的资源管理
   - **操作审计**：完整的用户操作记录和审计日志
   - **权限管理**：基于角色的访问控制和权限管理

7. **统一API服务层**
   - **RESTful API**：完整的资源管理和应用部署API
   - **GraphQL支持**：灵活的数据查询和订阅接口
   - **WebSocket实时通信**：支持实时状态推送和事件通知
   - **API版本管理**：向后兼容的API版本控制
   - **限流和安全**：API访问限流、认证和授权机制
   - **SDK支持**：多语言SDK和CLI工具支持

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

### 2.2 部署架构

系统采用分布式peer-to-peer架构，支持多种部署模式和异构资源提供者：

#### 2.2.1 分布式P2P网络架构

**核心特性：**
- 基于gRPC的peer-to-peer节点发现机制
- 支持动态节点加入和离开
- Gossip协议实现节点信息传播
- 无中心化的分布式资源管理

**网络拓扑：**
```
┌─────────────────────────────────────────────────────────────┐
│                    IARNet P2P Network                      │
│                                                             │
│  ┌─────────────┐    gRPC:50051    ┌─────────────┐          │
│  │   Node A    │◄─────────────────►│   Node B    │          │
│  │ HTTP:8083   │                   │ HTTP:8084   │          │
│  │             │                   │             │          │
│  │ ┌─────────┐ │                   │ ┌─────────┐ │          │
│  │ │Docker   │ │                   │ │K8s      │ │          │
│  │ │Provider │ │                   │ │Provider │ │          │
│  │ └─────────┘ │                   │ └─────────┘ │          │
│  └─────────────┘                   └─────────────┘          │
│         │                                 │                 │
│         │           ┌─────────────┐       │                 │
│         └──────────►│   Node C    │◄──────┘                 │
│                     │ HTTP:8085   │                         │
│                     │             │                         │
│                     │ ┌─────────┐ │                         │
│                     │ │Remote   │ │                         │
│                     │ │Provider │ │                         │
│                     │ └─────────┘ │                         │
│                     └─────────────┘                         │
└─────────────────────────────────────────────────────────────┘
```

#### 2.2.2 多模式运行时支持

**自动检测模式（默认）：**
- 系统自动检测运行环境
- 优先使用Kubernetes API（如果可用）
- 回退到Docker Engine API
- 配置：`mode: ""`

**独立模式：**
- 单节点部署，使用Docker作为容器运行时
- 适合开发测试和小规模环境
- 配置：`mode: "standalone"`

**Kubernetes模式：**
- 集群部署，使用Kubernetes作为容器编排平台
- 适合生产环境和大规模部署
- 配置：`mode: "k8s"`

#### 2.2.3 异构资源提供者架构

**本地资源提供者：**
```
┌─────────────────────────────────────┐
│         IARNet Node                 │
│  ┌─────────────────────────────────┐ │
│  │      Resource Manager           │ │
│  │  ┌─────────┬─────────────────┐   │ │
│  │  │Local    │ Docker/K8s      │   │ │
│  │  │Provider │ Provider        │   │ │
│  │  └─────────┴─────────────────┘   │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│       Container Runtime             │
│  ┌─────────┬─────────┬─────────────┐ │
│  │Container│Container│  Container  │ │
│  │    A    │    B    │      C      │ │
│  └─────────┴─────────┴─────────────┘ │
└─────────────────────────────────────┘
```

**远程资源提供者：**
```
┌─────────────────────────────────────┐
│         IARNet Node A               │
│  ┌─────────────────────────────────┐ │
│  │      Resource Manager           │ │
│  │  ┌─────────┬─────────────────┐   │ │
│  │  │Local    │ Peer Provider   │   │ │
│  │  │Provider │ (Node B Proxy)  │   │ │
│  │  └─────────┴─────────────────┘   │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
                  │ gRPC Call
                  ▼
┌─────────────────────────────────────┐
│         IARNet Node B               │
│  ┌─────────────────────────────────┐ │
│  │      Docker Provider            │ │
│  │  ┌─────────┬─────────────────┐   │ │
│  │  │Container│Container        │   │ │
│  │  │Deploy   │Management       │   │ │
│  │  └─────────┴─────────────────┘   │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

### 2.3 技术选型

#### 2.3.1 后端技术栈

**核心语言和框架：**
- **Go 1.25.0**：主要编程语言，高性能、并发友好
- **gorilla/mux v1.8.1**：HTTP路由和中间件
- **gRPC v1.75.0**：高性能RPC框架，用于P2P节点通信
- **Protocol Buffers v1.36.8**：数据序列化格式

**容器和编排：**
- **moby/moby v1.52.0**：Docker Engine API客户端
- **k8s.io/client-go v0.34.0**：Kubernetes官方Go客户端
- **k8s.io/api v0.34.0**：Kubernetes API对象定义
- **k8s.io/apimachinery v0.34.0**：Kubernetes核心类型

**分布式和通信：**
- **gRPC Gossip Protocol**：节点发现和信息传播
- **Peer-to-Peer Discovery**：去中心化节点管理
- **Remote Provider Proxy**：跨节点资源调用

**其他核心依赖：**
- **sirupsen/logrus v1.9.3**：结构化日志
- **gopkg.in/yaml.v2 v2.4.0**：YAML配置解析
- **github.com/docker/go-connections v0.5.0**：Docker连接管理
- **github.com/distribution/reference v0.6.0**：容器镜像引用

#### 2.3.2 前端技术栈

**核心框架：**
- **Next.js**：React全栈框架
- **React**：用户界面库
- **TypeScript**：类型安全的JavaScript

**UI组件和样式：**
- **@radix-ui/react-***：现代化无障碍UI组件库
- **Tailwind CSS**：原子化CSS框架
- **shadcn/ui**：基于Radix UI的组件系统
- **Lucide React v0.454.0**：图标库
- **Geist Font**：现代化字体

**开发工具和增强：**
- **@monaco-editor/react v4.7.0**：代码编辑器组件
- **@hookform/resolvers v3.10.0**：表单验证解析器
- **class-variance-authority v0.7.1**：CSS类变体管理
- **clsx v2.1.1**：条件CSS类名工具
- **cmdk v1.0.4**：命令面板组件
- **date-fns v4.1.0**：日期处理库
- **embla-carousel-react v8.5.1**：轮播组件
- **immer**：不可变状态管理
- **input-otp v1.4.1**：OTP输入组件

### 2.4 系统架构图

#### 2.4.1 分布式P2P网络架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           IARNet分布式算力网络                              │
│                                                                             │
│  ┌─────────────────┐         gRPC:50051         ┌─────────────────┐        │
│  │   IARNet Node A │◄─────────────────────────►│   IARNet Node B │        │
│  │   HTTP:8083     │                           │   HTTP:8084     │        │
│  │                 │                           │                 │        │
│  │ ┌─────────────┐ │                           │ ┌─────────────┐ │        │
│  │ │PeerManager  │ │                           │ │PeerManager  │ │        │
│  │ │- Gossip     │ │                           │ │- Discovery  │ │        │
│  │ │- Discovery  │ │                           │ │- Health     │ │        │
│  │ └─────────────┘ │                           │ └─────────────┘ │        │
│  │ ┌─────────────┐ │                           │ ┌─────────────┐ │        │
│  │ │ResourceMgr  │ │                           │ │ResourceMgr  │ │        │
│  │ │- Docker     │ │                           │ │- K8s        │ │        │
│  │ │- Local      │ │                           │ │- Remote     │ │        │
│  │ └─────────────┘ │                           │ └─────────────┘ │        │
│  └─────────────────┘                           └─────────────────┘        │
│           │                                             │                  │
│           │                 ┌─────────────────┐         │                  │
│           └────────────────►│   IARNet Node C │◄────────┘                  │
│                             │   HTTP:8085     │                            │
│                             │                 │                            │
│                             │ ┌─────────────┐ │                            │
│                             │ │PeerProvider │ │                            │
│                             │ │- Proxy      │ │                            │
│                             │ │- Remote     │ │                            │
│                             │ └─────────────┘ │                            │
│                             └─────────────────┘                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### 2.4.2 前后端交互架构

```
┌─────────────────────────────────────┐
│            Browser                  │
│  ┌─────────────────────────────────┐ │
│  │         Next.js App             │ │
│  │  ┌─────────┬─────────────────┐   │ │
│  │  │Pages    │ Radix UI        │   │ │
│  │  │Routes   │ Components      │   │ │
│  │  └─────────┴─────────────────┘   │ │
│  │  ┌─────────────────────────────┐ │ │
│  │  │     Monaco Editor           │ │ │
│  │  │     Code Editing            │ │ │
│  │  └─────────────────────────────┘ │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
                  │
                  │ HTTP REST API
                  ▼
┌─────────────────────────────────────┐
│         IARNet Backend              │
│  ┌─────────────────────────────────┐ │
│  │        HTTP Server              │ │
│  │     (gorilla/mux)               │ │
│  │  ┌─────────┬─────────────────┐   │ │
│  │  │Resource │ Application     │   │ │
│  │  │API      │ API             │   │ │
│  │  └─────────┴─────────────────┘   │ │
│  └─────────────────────────────────┘ │
│  ┌─────────────────────────────────┐ │
│  │      Business Logic             │ │
│  │  ┌─────────┬─────────────────┐   │ │
│  │  │Resource │ Application     │   │ │
│  │  │Manager  │ Manager         │   │ │
│  │  └─────────┴─────────────────┘   │ │
│  │  ┌─────────┬─────────────────┐   │ │
│  │  │Comm     │ Code Analysis   │   │ │
│  │  │Manager  │ Service         │   │ │
│  │  └─────────┴─────────────────┘   │ │
│  └─────────────────────────────────┘ │
│  ┌─────────────────────────────────┐ │
│  │       gRPC Server               │ │
│  │    (Peer Discovery)             │ │
│  │  ┌─────────┬─────────────────┐   │ │
│  │  │Peer     │ Provider        │   │ │
│  │  │Exchange │ Exchange        │   │ │
│  │  └─────────┴─────────────────┘   │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

#### 2.4.3 多Provider资源管理架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        IARNet资源管理层                                     │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                      Resource Manager                              │   │
│  │  ┌─────────────┬─────────────────┬─────────────────────────────┐   │   │
│  │  │Local        │ Managed         │ Collaborative               │   │   │
│  │  │Providers    │ Providers       │ Providers                   │   │   │
│  │  └─────────────┴─────────────────┴─────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │Docker       │  │K8s          │  │Remote       │  │Other        │        │
│  │Provider     │  │Provider     │  │Provider     │  │Provider     │        │
│  │             │  │             │  │             │  │             │        │
│  │┌───────────┐│  │┌───────────┐│  │┌───────────┐│  │┌───────────┐│        │
│  ││Container  ││  ││Pod        ││  ││gRPC       ││  ││HTTP       ││        │
│  ││Management ││  ││Management ││  ││Proxy      ││  ││Proxy      ││        │
│  │└───────────┘│  │└───────────┘│  │└───────────┘│  │└───────────┘│        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
│         │                 │                 │                 │            │
│         ▼                 ▼                 ▼                 ▼            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │Docker       │  │Kubernetes   │  │Remote Node  │  │Cloud        │        │
│  │Engine       │  │Cluster      │  │IARNet       │  │Provider     │        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
└─────────────────────────────────────────────────────────────────────────────┘
```

## 3. 后端服务设计（Go实现）

### 3.1 系统架构概述

后端服务采用分层架构设计，基于Go 1.25.0构建，是IARNet分布式算力网络的核心引擎。系统通过模块化设计实现高可扩展性和高可用性，支持多种运行时环境和异构资源提供者。

**核心架构特性：**
- **微服务架构**：模块间松耦合，支持独立部署和扩展
- **分布式设计**：基于P2P网络的去中心化资源管理
- **多运行时支持**：统一接口适配Docker、Kubernetes等运行时
- **智能调度**：基于资源使用情况的智能负载均衡
- **实时监控**：全链路的性能监控和健康检查

### 3.2 核心功能模块

#### 3.2.1 异构资源管理层（Resource Manager）

**模块职责：**
- 统一管理本地和远程异构资源提供者
- 实现资源配额控制和使用监控
- 提供智能资源调度和负载均衡
- 支持资源提供者的动态注册和发现

**核心组件：**
```go
type Manager struct {
    limits              Usage
    current             Usage
    internalProvider    Provider            // 节点内部provider
    externalProviders   map[string]Provider // 直接接入的外部provider
    discoveredProviders map[string]Provider // 通过gossip协议发现的provider
    monitor             *ProviderMonitor
}
```

**支持的Provider类型：**
- **Docker Provider**：本地Docker Engine资源管理
- **Kubernetes Provider**：K8s集群资源管理
- **Peer Provider**：远程节点资源代理
- **Remote Provider**：云服务商资源接入

**资源监控特性：**
- 实时资源使用情况跟踪（CPU、内存、GPU、存储）
- 资源配额限制和超限保护
- 提供者健康状态监控和故障恢复
- 多维度资源使用统计和分析

#### 3.2.2 应用生命周期管理层（Application Manager）

**模块职责：**
- 管理应用的完整生命周期（创建、部署、运行、停止、删除）
- 支持Git仓库代码自动拉取和分析
- 实现应用组件的DAG图生成和依赖管理
- 提供代码浏览器和在线编辑功能

**核心组件：**
```go
type Manager struct {
    applications    map[string]*AppRef
    applicationDAGs map[string]*ApplicationDAG
    codeBrowsers    map[string]*CodeBrowserInfo
    resourceManager *resource.Manager
    analysisService CodeAnalysisService
}
```

**应用管理功能：**
- **Git集成**：自动克隆指定分支的代码仓库
- **代码分析**：智能识别应用架构和组件依赖
- **组件编排**：基于DAG的组件部署和依赖管理
- **状态管理**：应用和组件的状态跟踪和更新
- **资源分配**：智能的资源需求评估和分配

**代码浏览器特性：**
- 在线代码浏览和编辑
- 文件树结构展示
- 多语言语法高亮
- 实时代码修改和保存

#### 3.2.3 分布式节点发现层（Peer Discovery）

**模块职责：**
- 实现基于Gossip协议的P2P节点发现
- 管理集群节点的动态加入和离开
- 维护分布式资源提供者信息
- 提供节点健康检查和故障处理

**核心组件：**
```go
type PeerManager struct {
    peers               map[string]*Peer
    discoveredProviders map[string]*DiscoveredProvider
    resourceManager     *resource.Manager
    gossipInterval      time.Duration
}
```

**P2P网络特性：**
- **Gossip协议**：高效的节点信息传播机制
- **自动发现**：新节点自动加入集群网络
- **故障检测**：实时检测和处理节点故障
- **信息同步**：保证节点间资源信息的一致性
- **负载均衡**：智能的跨节点资源调度

**gRPC服务接口：**
```protobuf
service PeerService {
  rpc ExchangePeers(ExchangeRequest) returns (ExchangeResponse);
  rpc ExchangeProviders(ProviderExchangeRequest) returns (ProviderExchangeResponse);
  rpc CallProvider(ProviderCallRequest) returns (ProviderCallResponse);
}
```

#### 3.2.4 智能代码分析层（Code Analysis）

**模块职责：**
- 分析应用代码结构和技术栈
- 生成应用组件DAG图和依赖关系
- 推荐最优的资源配置和部署策略
- 提供智能化的应用部署建议

**核心组件：**
```go
type MockCodeAnalysisService struct {
    resourceManager *resource.Manager
}

func (s *MockCodeAnalysisService) AnalyzeCode(ctx context.Context, req *proto.CodeAnalysisRequest) (*proto.CodeAnalysisResponse, error)
```

**分析功能：**
- **语言识别**：自动识别代码语言和框架
- **架构分析**：分析应用的多层架构设计
- **依赖解析**：生成组件间的依赖关系图
- **资源评估**：基于代码复杂度评估资源需求
- **部署建议**：推荐最优的部署配置和策略

**支持的架构模式：**
- 微服务架构（Gateway + API + Database）
- 前后端分离（Frontend + Backend + Cache）
- 容器化部署（Docker + Kubernetes）
- 分布式系统（Load Balancer + Multiple Services）

#### 3.2.5 服务间通信管理层（Communication Manager）

**模块职责：**
- 管理应用组件间的通信路由
- 注册和维护服务端点信息
- 实现服务发现和负载均衡
- 监控通信链路的健康状态

**核心组件：**
```go
type Manager struct {
    endpoints      map[string]*ServiceEndpoint
    routes         map[string]*ServiceRoute
    serviceMap     map[string]string
    resourceManager *resource.Manager
}
```

**通信管理功能：**
- **多协议支持**：HTTP、gRPC、WebSocket、消息队列
- **服务注册**：自动注册组件服务端点
- **路由管理**：智能的服务路由和流量控制
- **健康检查**：实时监控服务端点健康状态
- **故障转移**：自动的服务故障检测和切换

**支持的通信类型：**
```go
type CommunicationType string
const (
    HTTP      CommunicationType = "http"
    GRPC      CommunicationType = "grpc"
    WebSocket CommunicationType = "websocket"
    TCP       CommunicationType = "tcp"
    UDP       CommunicationType = "udp"
)
```

#### 3.2.6 统一API服务层（HTTP Server）

**模块职责：**
- 提供完整的RESTful API接口
- 实现统一的请求处理和响应格式
- 支持API认证、授权和限流
- 提供完整的API文档和SDK

**核心组件：**
```go
type Server struct {
    router *mux.Router
    runner runner.Runner
    resMgr *resource.Manager
    appMgr *application.Manager
    peerMgr *discovery.PeerManager
}
```

**API功能模块：**
- **资源管理API**：资源容量查询、提供者管理、监控数据获取
- **应用管理API**：应用CRUD、组件管理、部署控制
- **节点管理API**：集群节点管理、P2P网络控制
- **监控API**：实时状态查询、性能指标获取
- **文件管理API**：代码文件浏览、内容编辑

**API设计特性：**
- **RESTful设计**：遵循REST设计原则和最佳实践
- **统一响应格式**：标准化的成功和错误响应结构
- **版本控制**：支持API版本管理和向后兼容
- **文档化**：完整的OpenAPI规范和交互式文档
- **SDK支持**：多语言SDK和CLI工具支持

### 3.2 模块划分

#### 3.2.1 API层（HTTP服务器）

**职责：**
- HTTP请求路由和处理
- 请求参数验证和解析
- 响应格式化和错误处理
- 提供RESTful API接口

**核心组件：**
```go
type Server struct {
	router *mux.Router
	runner runner.Runner
	resMgr *resource.Manager
	appMgr *application.Manager
	peerMgr *discovery.PeerManager
	ctx    context.Context
	cancel context.CancelFunc
}

func NewServer(r runner.Runner, rm *resource.Manager, am *application.Manager, pm *discovery.PeerManager) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{router: mux.NewRouter(), runner: r, resMgr: rm, appMgr: am, peerMgr: pm, ctx: ctx, cancel: cancel}
	// 资源管理API
	s.router.HandleFunc("/resource/capacity", s.handleResourceCapacity).Methods("GET")
	s.router.HandleFunc("/resource/providers", s.handleResourceProviders).Methods("GET")
	s.router.HandleFunc("/resource/providers", s.handleRegisterProvider).Methods("POST")
	// 应用管理API
	s.router.HandleFunc("/application/apps", s.handleGetApplications).Methods("GET")
	s.router.HandleFunc("/application/apps", s.handleCreateApplication).Methods("POST")
	s.router.HandleFunc("/application/apps/{id}", s.handleGetApplicationById).Methods("GET")
	s.router.HandleFunc("/application/apps/{id}", s.handleDeleteApplication).Methods("DELETE")
	// 组件管理API
	s.router.HandleFunc("/application/apps/{id}/components", s.handleGetApplicationComponents).Methods("GET")
	s.router.HandleFunc("/application/apps/{id}/analyze", s.handleAnalyzeApplication).Methods("POST")
	s.router.HandleFunc("/application/apps/{id}/deploy-components", s.handleDeployComponents).Methods("POST")
	// 节点管理API
	s.router.HandleFunc("/peer/nodes", s.handleGetPeerNodes).Methods("GET")
	s.router.HandleFunc("/peer/nodes", s.handleAddPeerNode).Methods("POST")
	return s
}
```

#### 3.2.2 应用管理层（Application Manager）

**职责：**
- 管理应用的完整生命周期
- 处理Git仓库克隆和代码分析
- 管理应用组件和依赖关系
- 协调组件部署和运行

**核心组件：**
```go
type Manager struct {
	apps           map[string]*AppRef
	mu             sync.RWMutex
	resMgr         *resource.Manager
	commMgr        *communication.Manager
	analysisClient proto.CodeAnalysisServiceClient
}

type AppRef struct {
	ID          string
	Name        string
	GitURL      string
	LocalPath   string
	Status      Status
	Components  map[string]*Component
	DAG         *ApplicationDAG
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
```

**核心功能：**
- Git仓库克隆和管理
- 代码分析和组件识别
- 应用组件部署和管理
- 应用状态监控和日志收集

#### 3.2.3 资源管理层（Resource Manager）

**职责：**
- 管理多种类型的资源提供者
- 统一资源分配和监控接口
- 处理本地、托管和协作资源
- 实现资源使用情况监控

**核心组件：**
```go
type Manager struct {
	localProvider         Provider
	managedProviders      []Provider
	collaborativeProviders []Provider
	mu                    sync.RWMutex
}

type Provider interface {
	GetID() string
	GetName() string
	GetType() string
	GetHost() string
	GetPort() int
	GetStatus() ProviderStatus
	GetCapacity(ctx context.Context) (*Capacity, error)
	Deploy(ctx context.Context, spec ContainerSpec) (string, error)
	GetLogs(ctx context.Context, containerID string, lines int) ([]string, error)
	GetLastUpdateTime() time.Time
}
```

**资源提供者类型：**
- `DockerProvider`：Docker运行时提供者
- `K8sProvider`：Kubernetes集群提供者
- `RemoteProvider`：远程节点资源代理

#### 3.2.4 节点发现层（Peer Discovery）

**职责：**
- 实现分布式节点发现机制
- 维护集群节点和资源信息
- 处理节点加入和离开事件
- 实现资源提供者的发现和注册

**核心组件：**
```go
type PeerManager struct {
	peers               map[string]struct{}
	discoveredProviders map[string]*DiscoveredProvider
	resMgr              *resource.Manager
	mu                  sync.Mutex
}

type DiscoveredProvider struct {
	ID          string
	Name        string
	Type        string
	Host        string
	Port        int
	Status      int
	PeerAddress string
	LastSeen    time.Time
}
```

**gRPC服务定义：**
```protobuf
service PeerService {
  rpc ExchangePeers(ExchangeRequest) returns (ExchangeResponse);
  rpc ExchangeProviders(ProviderExchangeRequest) returns (ProviderExchangeResponse);
  rpc CallProvider(ProviderCallRequest) returns (ProviderCallResponse);
}
```

**核心功能：**
- Gossip协议实现节点信息传播
- 资源提供者信息交换
- 远程资源提供者代理调用
- 失效节点清理机制

#### 3.2.5 代码分析层（Code Analysis）

**职责：**
- 分析应用代码结构和依赖关系
- 生成应用组件DAG图
- 推荐资源配置和部署策略
- 提供智能化的应用部署建议

**核心组件：**
```go
type MockCodeAnalysisService struct {
	resourceManager *resource.Manager
}

func (s *MockCodeAnalysisService) AnalyzeCode(ctx context.Context, req *proto.CodeAnalysisRequest) (*proto.CodeAnalysisResponse, error)
```

**分析功能：**
- 代码语言和框架识别
- 组件依赖关系分析
- 资源需求评估
- 部署配置生成

#### 3.2.6 通信管理层（Communication Manager）

**职责：**
- 管理组件间的通信路由
- 注册和维护服务端点
- 实现服务发现和负载均衡
- 监控通信健康状态

**核心组件：**
```go
type Manager struct {
	endpoints      map[string]*ServiceEndpoint
	routes         map[string]*ServiceRoute
	serviceMap     map[string]string
	resourceManager *resource.Manager
	mu             sync.RWMutex
}
```

**通信类型：**
- HTTP RESTful API
- gRPC服务调用
- 消息队列通信
- 流式数据传输

### 3.3 关键功能流程

#### 3.3.1 智能应用部署流程

**完整应用部署架构：**

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Client    │    │ HTTP Server │    │ App Manager │    │    Ignis    │    │ Res Manager │    │  Provider   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
        │                   │                   │                   │                   │                   │
        │ POST /deploy      │                   │                   │                   │                   │
        ├──────────────────►│                   │                   │                   │                   │
        │                   │                   │                   │                   │                   │
        │                   │ Analyze & Deploy  │                   │                   │                   │
        │                   ├──────────────────►│                   │                   │                   │
        │                   │                   │                   │                   │                   │
        │                   │                   │ Clone Repository  │                   │                   │
        │                   │                   ├──────────────────►│                   │                   │
        │                   │                   │                   │                   │                   │
        │                   │                   │ gRPC Call Ignis   │                   │                   │
        │                   │                   ├──────────────────►│                   │                   │
        │                   │                   │                   │                   │                   │
        │                   │                   │                   │ Code Analysis     │                   │
        │                   │                   │                   ├──────────────────►│                   │
        │                   │                   │                   │                   │                   │
        │                   │                   │                   │ Generate DAG      │                   │
        │                   │                   │                   ├──────────────────►│                   │
        │                   │                   │                   │                   │                   │
        │                   │                   │                   │ Select Provider   │                   │
        │                   │                   │                   ├──────────────────►│                   │
        │                   │                   │                   │                   │                   │
        │                   │                   │ Deploy Components │                   │                   │
        │                   │                   ├──────────────────────────────────────►│                   │
        │                   │                   │                   │                   │                   │
        │                   │                   │                   │                   │ Deploy Container  │
        │                   │                   │                   │                   ├──────────────────►│
        │                   │                   │                   │                   │                   │
        │                   │                   │                   │                   │ Update Status     │
        │                   │                   │                   │                   │◄──────────────────┤
        │                   │                   │                   │                   │                   │
        │ 202 Accepted      │                   │                   │                   │                   │
        │◄──────────────────┤                   │                   │                   │                   │
        │                   │                   │                   │                   │                   │
```

**详细部署步骤：**

1. **请求接收与验证**
   ```go
   func (s *Server) handleDeployComponents(w http.ResponseWriter, req *http.Request) {
       appID := mux.Vars(req)["id"]
       app, err := s.appMgr.GetApplication(appID)
       if err != nil {
           response.WriteError(w, http.StatusNotFound, "application not found", err)
           return
       }
   }
   ```

2. **代码仓库克隆**
   - 从Git仓库拉取指定分支代码
   - 验证仓库访问权限和分支有效性
   - 本地存储代码到临时目录

3. **Ignis代码分析平台调用**
   - App Manager通过gRPC调用Ignis代码分析平台
   - Ignis自动识别编程语言和框架类型
   - 分析应用架构和组件依赖关系
   - 生成应用组件DAG图
   - 评估资源需求和部署策略
   - 智能选择最优Provider

4. **组件编排部署**
   ```go
   func (m *Manager) deployComponent(component *Component) error {
       // 通过gRPC调用Ignis获取Provider选择建议
       ignisResp, err := m.ignisClient.SelectProvider(context.Background(), &proto.ProviderRequest{
           Resources: &proto.ResourceRequirement{
               CPU:    component.Resources.CPU,
               Memory: component.Resources.Memory,
               GPU:    component.Resources.GPU,
           },
       })
       if err != nil {
           return fmt.Errorf("failed to select provider: %w", err)
       }
       
       provider, err := m.resourceManager.GetProvider(ignisResp.ProviderID)
       containerSpec := resource.ContainerSpec{
           Image:   component.Image,
           CPU:     component.Resources.CPU,
           Memory:  component.Resources.Memory,
           GPU:     component.Resources.GPU,
       }
       containerID, err := provider.Deploy(context.Background(), containerSpec)
       component.Status = ComponentStatusRunning
       return nil
   }
   ```

5. **资源分配与监控**
   - 实时更新资源使用情况
   - 启动组件健康检查
   - 注册服务发现信息

#### 3.3.2 多Provider资源调度流程

**智能资源分配架构：**

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Request   │    │ Resource    │    │  Provider   │    │   Result    │
│             │    │ Manager     │    │  Selection  │    │             │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
        │                   │                   │                   │
        │ Resource Request  │                   │                   │
        ├──────────────────►│                   │                   │
        │                   │                   │                   │
        │                   │ Check Internal    │                   │
        │                   ├──────────────────►│                   │
        │                   │                   │                   │
        │                   │ Check External    │                   │
        │                   ├──────────────────►│                   │
        │                   │                   │                   │
        │                   │ Check Discovered  │                   │
        │                   ├──────────────────►│                   │
        │                   │                   │                   │
        │                   │ Select Best       │                   │
        │                   ├──────────────────►│                   │
        │                   │                   │                   │
        │ Provider Found    │                   │                   │
        │◄──────────────────┤                   │                   │
        │                   │                   │                   │
```

**资源调度算法：**
```go
func (rm *Manager) canAllocate(req Usage) Provider {
    // 1. 优先检查内部Provider
    if rm.internalProvider != nil && rm.internalProvider.GetStatus() == StatusConnected {
        capacity, err := rm.internalProvider.GetCapacity(context.Background())
        if err == nil && capacity.Available.CPU >= req.CPU &&
           capacity.Available.Memory >= req.Memory &&
           capacity.Available.GPU >= req.GPU {
            return rm.internalProvider
        }
    }
    
    // 2. 检查外部Provider
    for _, provider := range rm.externalProviders {
        if provider.GetStatus() == StatusConnected {
            capacity, err := provider.GetCapacity(context.Background())
            if err == nil && capacity.Available.CPU >= req.CPU &&
               capacity.Available.Memory >= req.Memory &&
               capacity.Available.GPU >= req.GPU {
                return provider
            }
        }
    }
    
    // 3. 检查发现的Provider
    for _, provider := range rm.discoveredProviders {
        if provider.GetStatus() == StatusConnected {
            capacity, err := provider.GetCapacity(context.Background())
            if err == nil && capacity.Available.CPU >= req.CPU &&
               capacity.Available.Memory >= req.Memory &&
               capacity.Available.GPU >= req.GPU {
                return provider
            }
        }
    }
    
    return nil // 无可用Provider
}
```

**Provider优先级策略：**
1. **内部Provider**：本地Docker/K8s资源，延迟最低
2. **外部Provider**：直接管理的远程资源，稳定可靠
3. **发现Provider**：通过P2P网络发现的资源，动态扩展

#### 3.3.3 智能资源配额检查流程

**多Provider配额管理架构：**

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Request   │    │ Resource    │    │  Provider   │    │  Allocator  │
│             │    │ Manager     │    │ Selection   │    │             │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
        │                   │                   │                   │
        │ Check Resources   │                   │                   │
        ├──────────────────►│                   │                   │
        │                   │                   │                   │
        │                   │ Query Providers   │                   │
        │                   ├──────────────────►│                   │
        │                   │                   │                   │
        │                   │ Provider List     │                   │
        │                   │◄──────────────────┤                   │
        │                   │                   │                   │
        │                   │ Calculate Best    │                   │
        │                   ├──────────────────────────────────────►│
        │                   │                   │                   │
        │                   │ Allocation Plan   │                   │
        │                   │◄──────────────────────────────────────┤
        │                   │                   │                   │
        │ Allow/Deny        │                   │                   │
        │◄──────────────────┤                   │                   │
        │                   │                   │                   │
```

**智能配额检查算法：**
```go
func (rm *Manager) CanAllocate(req Usage) (bool, Provider, error) {
    rm.mu.RLock()
    defer rm.mu.RUnlock()
    
    // 1. 按优先级检查Provider
    providers := rm.getProvidersByPriority()
    
    for _, provider := range providers {
        if provider.GetStatus() != StatusConnected {
            continue
        }
        
        // 2. 获取实时容量信息
        capacity, err := provider.GetCapacity(context.Background())
        if err != nil {
            log.Printf("Failed to get capacity from provider %s: %v", 
                      provider.GetID(), err)
            continue
        }
        
        // 3. 检查资源可用性
        if rm.checkResourceAvailability(capacity, req) {
            return true, provider, nil
        }
    }
    
    return false, nil, ErrInsufficientResources
}

func (rm *Manager) checkResourceAvailability(capacity *Capacity, req Usage) bool {
    return capacity.Available.CPU >= req.CPU &&
           capacity.Available.Memory >= req.Memory &&
           capacity.Available.GPU >= req.GPU &&
           capacity.Available.Storage >= req.Storage
}
```

**配额管理特性：**
1. **实时监控**：持续监控各Provider资源使用情况
2. **智能调度**：基于负载均衡和资源效率进行调度
3. **弹性扩展**：自动发现和接入新的资源Provider
4. **故障转移**：Provider故障时自动切换到备用资源

#### 3.3.4 分布式节点发现流程

**P2P网络节点发现架构：**

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Local Node  │    │ Peer Manager│    │ DHT Network │    │Remote Nodes │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
        │                   │                   │                   │
        │ Start Discovery   │                   │                   │
        ├──────────────────►│                   │                   │
        │                   │                   │                   │
        │                   │ Bootstrap DHT     │                   │
        │                   ├──────────────────►│                   │
        │                   │                   │                   │
        │                   │ Announce Self     │                   │
        │                   ├──────────────────►│                   │
        │                   │                   │                   │
        │                   │ Query Peers       │                   │
        │                   ├──────────────────►│                   │
        │                   │                   │                   │
        │                   │ Peer List         │                   │
        │                   │◄──────────────────┤                   │
        │                   │                   │                   │
        │                   │ Connect Peers     │                   │
        │                   ├──────────────────────────────────────►│
        │                   │                   │                   │
        │                   │ Handshake        │                   │
        │                   │◄──────────────────────────────────────┤
        │                   │                   │                   │
        │ Peers Available   │                   │                   │
        │◄──────────────────┤                   │                   │
        │                   │                   │                   │
```

**节点发现实现：**
```go
type PeerManager struct {
    nodeID      string
    peers       map[string]*Peer
    dht         *DHT
    commManager *communication.Manager
    mu          sync.RWMutex
}

func (pm *PeerManager) StartDiscovery() error {
    // 1. 启动DHT网络
    if err := pm.dht.Bootstrap(); err != nil {
        return fmt.Errorf("failed to bootstrap DHT: %w", err)
    }
    
    // 2. 在DHT中宣告自己
    if err := pm.dht.Announce(pm.nodeID); err != nil {
        return fmt.Errorf("failed to announce node: %w", err)
    }
    
    // 3. 定期查询新节点
    go pm.periodicPeerDiscovery()
    
    return nil
}

func (pm *PeerManager) periodicPeerDiscovery() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        peers, err := pm.dht.FindPeers("iarnet")
        if err != nil {
            continue
        }
        
        for _, peer := range peers {
            if peer.ID != pm.nodeID {
                pm.connectToPeer(peer)
            }
        }
    }
}
```

**Gossip协议实现：**
```go
func (pm *PeerManager) gossipOnce() {
    pm.mu.RLock()
    known := make([]string, 0, len(pm.peers))
    for addr := range pm.peers {
        known = append(known, addr)
    }
    pm.mu.RUnlock()
    
    // 随机选择部分节点进行Gossip
    for _, peerAddr := range pm.selectRandomPeers(known, 3) {
        go pm.exchangePeersWithNode(peerAddr, known)
    }
}

func (pm *PeerManager) exchangePeersWithNode(peerAddr string, known []string) {
    conn, err := grpc.Dial(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        pm.removePeer(peerAddr) // 连接失败，移除节点
        return
    }
    defer conn.Close()
    
    client := proto.NewPeerServiceClient(conn)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    resp, err := client.ExchangePeers(ctx, &proto.ExchangeRequest{
        NodeId:     pm.nodeID,
        KnownPeers: known,
    })
    if err != nil {
        pm.removePeer(peerAddr)
        return
    }
    
    pm.AddPeers(resp.KnownPeers)
}
```

**节点连接与验证：**
1. **身份验证**：验证节点身份和网络权限
2. **能力协商**：交换节点资源能力信息
3. **状态同步**：同步网络状态和拓扑信息
4. **心跳维护**：定期检查连接状态
```

### 3.4 接口设计（API Spec）

#### 3.4.1 资源管理API

**GET /resource/capacity（查看资源容量）**

**功能描述：**
获取当前节点的资源容量信息

**请求格式：**
```http
GET /resource/capacity
```

**响应格式：**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "cpu": 8.0,
  "memory": 16.0,
  "gpu": 2.0,
  "storage": 500.0
}
```

**GET /resource/providers（获取资源提供者列表）**

**功能描述：**
获取所有已注册的资源提供者信息

**请求格式：**
```http
GET /resource/providers
```

**响应格式：**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "providers": [
    {
      "id": "local-docker",
      "name": "Local Docker",
      "type": "docker",
      "host": "localhost",
      "port": 2376,
      "status": "active",
      "capacity": {
        "cpu": 4.0,
        "memory": 8.0,
        "gpu": 1.0
      }
    }
  ]
}
```

**POST /resource/providers（注册资源提供者）**

**功能描述：**
注册新的资源提供者

**请求格式：**
```http
POST /resource/providers
Content-Type: application/json

{
  "name": "K8s Cluster",
  "type": "k8s",
  "host": "k8s.example.com",
  "port": 6443,
  "config": {
    "kubeconfig": "...",
    "namespace": "default"
  }
}
```

#### 3.4.2 应用管理API

**GET /application/apps（获取应用列表）**

**功能描述：**
获取所有已创建的应用信息

**请求格式：**
```http
GET /application/apps
```

**响应格式：**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "applications": [
    {
      "id": "app-001",
      "name": "Web App",
      "git_url": "https://github.com/user/webapp.git",
      "status": "running",
      "components_count": 3,
      "created_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

**POST /application/apps（创建应用）**

**功能描述：**
从Git仓库创建新应用

**请求格式：**
```http
POST /application/apps
Content-Type: application/json

{
  "name": "My Web App",
  "git_url": "https://github.com/user/webapp.git",
  "branch": "main"
}
```

**GET /application/apps/{id}（获取应用详情）**

**功能描述：**
获取指定应用的详细信息

**响应格式：**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "id": "app-001",
  "name": "Web App",
  "git_url": "https://github.com/user/webapp.git",
  "local_path": "/tmp/apps/app-001",
  "status": "running",
  "components": {
    "frontend": {
      "type": "web",
      "status": "running",
      "provider_id": "local-docker"
    }
  },
  "dag": {
    "components": [...],
    "edges": [...]
  }
}
```

#### 3.4.3 组件管理API

**GET /application/apps/{id}/components（获取应用组件）**

**功能描述：**
获取指定应用的所有组件信息

**响应格式：**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "components": [
    {
      "id": "frontend",
      "name": "Frontend Service",
      "type": "web",
      "status": "running",
      "provider_id": "local-docker",
      "container_id": "abc123",
      "resources": {
        "cpu": 1.0,
        "memory": 512,
        "gpu": 0
      }
    }
  ]
}
```

**POST /application/apps/{id}/analyze（分析应用代码）**

**功能描述：**
分析应用代码结构，生成组件DAG

**响应格式：**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "components": [
    {
      "id": "frontend",
      "name": "Frontend",
      "type": "web",
      "language": "javascript",
      "framework": "react",
      "requirements": {
        "cpu": 1.0,
        "memory": 512,
        "gpu": 0
      }
    }
  ],
  "dag": {
    "edges": [
      {
        "from": "frontend",
        "to": "backend",
        "type": "http"
      }
    ]
  }
}
```

**POST /application/apps/{id}/deploy-components（部署组件）**

**功能描述：**
部署应用的所有组件

**请求格式：**
```http
POST /application/apps/{id}/deploy-components
Content-Type: application/json

{
  "components": ["frontend", "backend"]
}
```

#### 3.4.4 节点管理API

**GET /peer/nodes（获取节点列表）**

**功能描述：**
获取集群中所有已发现的节点信息

**响应格式：**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "nodes": [
    {
      "address": "192.168.1.100:50051",
      "status": "active",
      "last_seen": "2024-01-15T10:35:00Z",
      "providers": [
        {
          "id": "docker-001",
          "type": "docker",
          "status": "active"
        }
      ]
    }
  ]
}
```

**POST /peer/nodes（添加节点）**

**功能描述：**
手动添加新的集群节点

**请求格式：**
```http
POST /peer/nodes
Content-Type: application/json

{
  "address": "192.168.1.101:50051"
 }
 ```

#### 3.4.5 gRPC服务接口

**PeerService（节点发现服务）**

**ExchangePeers（交换节点信息）**

**功能描述：**
在集群节点间交换已知的节点列表，实现节点发现

**请求格式：**
```protobuf
message ExchangeRequest {
  repeated string known_peers = 1;
}
```

**响应格式：**
```protobuf
message ExchangeResponse {
  repeated string known_peers = 1;
}
```

**ExchangeProviders（交换资源提供者信息）**

**功能描述：**
在集群节点间交换资源提供者信息

**请求格式：**
```protobuf
message ProviderExchangeRequest {
  repeated ProviderInfo providers = 1;
}

message ProviderInfo {
  string id = 1;
  string name = 2;
  string type = 3;
  string host = 4;
  int32 port = 5;
  int32 status = 6;
}
```

**响应格式：**
```protobuf
message ProviderExchangeResponse {
  repeated ProviderInfo providers = 1;
}
```

**CallProvider（调用远程资源提供者）**

**功能描述：**
通过代理调用远程节点的资源提供者

**请求格式：**
```protobuf
message ProviderCallRequest {
  string provider_id = 1;
  string method = 2;
  bytes payload = 3;
}
```

**响应格式：**
```protobuf
message ProviderCallResponse {
  bool success = 1;
  bytes response = 2;
  string error = 3;
}
```

**CodeAnalysisService（代码分析服务）**

**AnalyzeCode（分析代码）**

**功能描述：**
分析应用代码结构，生成组件DAG和资源需求

**请求格式：**
```protobuf
message CodeAnalysisRequest {
  string project_path = 1;
  string language = 2;
  string framework = 3;
}
```

**响应格式：**
```protobuf
message CodeAnalysisResponse {
  repeated Component components = 1;
  repeated DAGEdge edges = 2;
}

message Component {
  string id = 1;
  string name = 2;
  string type = 3;
  string language = 4;
  string framework = 5;
  ResourceRequirements requirements = 6;
}

message ResourceRequirements {
  double cpu = 1;
  int64 memory = 2;
  double gpu = 3;
  int64 storage = 4;
}

message DAGEdge {
  string from = 1;
  string to = 2;
  string type = 3;
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

### 3.6 配置文件说明

#### 3.6.1 配置文件结构

**config.yaml配置文件：**
```yaml
mode: ""  # 运行模式："standalone"、"k8s" 或 "" (自动检测)
listen_addr: ":8083"  # HTTP服务监听地址
peer_listen_addr: ":50051"  # gRPC服务监听地址
initial_peers:  # 初始节点列表
  - "peer2.example.com:50051"
  - "peer3.example.com:50051"
resource_limits:  # 资源限制
  cpu: "8"      # CPU核心数
  memory: "16Gi" # 内存大小
  gpu: "4"      # GPU数量
workspace_dir: "../workspaces"  # Git仓库工作目录
```

#### 3.6.2 配置参数说明

**基础配置：**
- `mode`: 运行模式，支持自动检测或手动指定
  - `""`: 自动检测模式（默认）
  - `"standalone"`: 独立模式，使用Docker运行时
  - `"k8s"`: Kubernetes模式，使用K8s API

- `listen_addr`: HTTP API服务监听地址，默认":8083"
- `peer_listen_addr`: gRPC节点发现服务监听地址，默认":50051"

**集群配置：**
- `initial_peers`: 初始集群节点列表，用于节点发现的种子节点
- `workspace_dir`: Git仓库克隆的本地工作目录

**资源配置：**
- `resource_limits.cpu`: 节点CPU核心数限制
- `resource_limits.memory`: 节点内存限制（支持Gi、Mi等单位）
- `resource_limits.gpu`: 节点GPU数量限制

#### 3.6.3 环境变量支持

系统支持通过环境变量覆盖配置文件参数：

```bash
# 设置运行模式
export IARNET_MODE=standalone

# 设置监听地址
export IARNET_LISTEN_ADDR=:8080
export IARNET_PEER_LISTEN_ADDR=:50051

# 设置资源限制
export IARNET_CPU_LIMIT=16
export IARNET_MEMORY_LIMIT=32Gi
export IARNET_GPU_LIMIT=8

# 设置工作目录
export IARNET_WORKSPACE_DIR=/var/lib/iarnet/workspaces
```

### 3.7 部署指南

#### 3.7.1 Docker部署

**构建镜像：**
```dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o cps ./cmd/main.go

FROM ubuntu:22.04
COPY --from=builder /app/cps /usr/bin/cps
CMD ["cps", "--config=/config.yaml"]
```

**运行容器：**
```bash
# 构建镜像
docker build -t iarnet:latest .

# 运行容器
docker run -d \
  --name iarnet \
  -p 8083:8083 \
  -p 50051:50051 \
  -v $(pwd)/config.yaml:/config.yaml \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd)/workspaces:/workspaces \
  iarnet:latest
```

**Docker Compose部署：**
```yaml
version: '3.8'
services:
  iarnet:
    build: .
    ports:
      - "8083:8083"
      - "50051:50051"
    volumes:
      - ./config.yaml:/config.yaml
      - /var/run/docker.sock:/var/run/docker.sock
      - ./workspaces:/workspaces
    environment:
      - IARNET_MODE=standalone
    restart: unless-stopped
```

#### 3.7.2 Kubernetes部署

**部署清单：**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: iarnet-deployment
  namespace: iarnet-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: iarnet
  template:
    metadata:
      labels:
        app: iarnet
    spec:
      serviceAccountName: iarnet-sa
      containers:
      - name: iarnet
        image: iarnet:latest
        ports:
        - containerPort: 8083
          name: http
        - containerPort: 50051
          name: grpc
        volumeMounts:
        - name: config
          mountPath: /config.yaml
          subPath: config.yaml
        - name: workspace
          mountPath: /workspaces
        env:
        - name: IARNET_MODE
          value: "k8s"
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
      volumes:
      - name: config
        configMap:
          name: iarnet-config
      - name: workspace
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: iarnet-service
  namespace: iarnet-system
spec:
  selector:
    app: iarnet
  ports:
  - name: http
    port: 8083
    targetPort: 8083
  - name: grpc
    port: 50051
    targetPort: 50051
  type: ClusterIP
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: iarnet-config
  namespace: iarnet-system
data:
  config.yaml: |
    mode: k8s
    listen_addr: ":8083"
    peer_listen_addr: ":50051"
    initial_peers: []
    resource_limits:
      cpu: "8"
      memory: "16Gi"
      gpu: "4"
    workspace_dir: "/workspaces"
```

**RBAC配置：**
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: iarnet-sa
  namespace: iarnet-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: iarnet-role
  namespace: iarnet-system
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["create", "delete", "get", "list", "watch"]
- apiGroups: [""]
  resources: ["pods/log"]
  verbs: ["get"]
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: iarnet-rolebinding
  namespace: iarnet-system
subjects:
- kind: ServiceAccount
  name: iarnet-sa
  namespace: iarnet-system
roleRef:
  kind: Role
  name: iarnet-role
  apiGroup: rbac.authorization.k8s.io
```

**部署命令：**
```bash
# 创建命名空间
kubectl create namespace iarnet-system

# 应用部署清单
kubectl apply -f k8s-deployment.yaml

# 检查部署状态
kubectl get pods -n iarnet-system
kubectl get svc -n iarnet-system

# 查看日志
kubectl logs -f deployment/iarnet-deployment -n iarnet-system
```

#### 3.7.3 集群部署

**多节点集群部署：**

1. **准备节点配置**
```yaml
# node1.yaml
mode: "standalone"
listen_addr: ":8083"
peer_listen_addr: ":50051"
initial_peers:
  - "node2.example.com:50051"
  - "node3.example.com:50051"
resource_limits:
  cpu: "8"
  memory: "16Gi"
  gpu: "2"
workspace_dir: "/var/lib/iarnet/workspaces"
```

2. **启动集群节点**
```bash
# 节点1
docker run -d --name iarnet-node1 \
  -p 8083:8083 -p 50051:50051 \
  -v $(pwd)/node1.yaml:/config.yaml \
  iarnet:latest

# 节点2
docker run -d --name iarnet-node2 \
  -p 8084:8083 -p 50052:50051 \
  -v $(pwd)/node2.yaml:/config.yaml \
  iarnet:latest

# 节点3
docker run -d --name iarnet-node3 \
  -p 8085:8083 -p 50053:50051 \
  -v $(pwd)/node3.yaml:/config.yaml \
  iarnet:latest
```

3. **验证集群状态**
```bash
# 检查节点发现
curl http://localhost:8083/peer/nodes

# 检查资源提供者
curl http://localhost:8083/resource/providers
```

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