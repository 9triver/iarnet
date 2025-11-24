# 设计文档

## 1. 算力网络资源态势感知

### 1.1 目标与范围
- 实时掌握多域算力节点（CPU、GPU、内存等）的可用态势，支撑调度决策。
- 帮助前端可视化和策略引擎对节点进行健康评估、瓶颈定位与容量规划。
- 在保持节点自治的前提下，通过轻量级协议实现跨域共享视图。

### 1.2 关键组件
- `Resource Provider`：部署在每个 `iarnet` 节点内，负责采集容器运行时、Docker/OCI 资源指标与作业元数据，并写入本地 `resource_logger.db`。
- `Registry Manager`（`iarnet-global/internal/domain/registry`）：维护域、节点、资源标签与容量模型，为调度器提供一致数据视图。
- `Resource Logger`：周期性拉取 Docker 运行时、系统指标，生成结构化日志块并持久化，供前端与策略检索。
- `Gossip Discovery`：依据 `config.yaml` 中的 `resource.discovery` 配置，进行节点心跳扩散、TTL 淘汰以及反熵同步，保证在无中心场景下的拓扑发现。
- `Global Registry`：通过 gRPC 对外暴露查询接口，前端与全局调度器均以此为唯一事实来源。

### 1.3 数据流与处理
1. Provider 采集的资源事件先落本地 SQLite，再通过事件总线推送给节点内部的状态管理器。
2. Discovery 模块按照 `gossip_interval_seconds` 触发多跳扩散，互换节点在线状态与 `ResourceTags`，并利用 `anti_entropy_interval_seconds` 保证最终一致性。
3. Registry Manager 将节点状态聚合到域级别，生成 `Domain.ResourceTags`、在线节点计数等摘要；同时对节点容量做快照（`ResourceCapacity.Available`）。
4. 全球视图同步至 `iarnet-global` 前端：`docker-compose` 中暴露 `3002` 端口为 Web 控制面，依赖 `BACKEND_URL` 调取后台查询接口展示态势。
5. 调度器或策略引擎可以基于上述视图做资源过滤（标签、容量、地域等），并反向写入约束策略（如黑名单、优先级）。

### 1.4 异常与健壮性
- 心跳丢失通过 `node_ttl_seconds` 触发节点过期，Registry Manager 自动将状态标记为 `offline`，调度器可避免选中。
- Provider 在断网时以本地日志回放方式补发，保证指标完整。
- 通过 Head 节点的 `status` 与 `last_seen` 字段，识别跨域控制面是否健康。

## 2. 面向跨资源域调度的分级调度框架

### 2.1 分级模型
- **全局层（Global Scheduler）**：运行于 `iarnet-global/internal/domain/scheduler`，维护全局域清单，负责跨域择优与调度委托。
- **域级 Head 节点**：每个域由一个 Head 节点对接全局调度器（`Node.IsHead=true`），对域内资源进行再分配，屏蔽域内拓扑细节。
- **边缘执行节点**：最终承载算力工作负载的真实节点，可接入 Docker Provider、Kubernetes、裸机等异构资源。

### 2.2 核心流程
1. 用户或业务系统提交调度请求至全局调度器，包含资源需求（`schedulerpb.DeployComponentRequest.ResourceRequest`）与数据局部性等策略。
2. 调度服务 `service.selectRandomNode` 先对域进行过滤：排除无在线节点或容量不足的域，再在满足条件的域内随机挑选合适 Head 节点。
3. 全局调度器通过 gRPC 将请求转发给选中的 Head 节点（`forwardToNode`），并在日志中记录域、节点信息以便追踪。
4. Head 节点内部有本地调度器，结合域内 Provider 上报的运行队列、负载均衡策略（轮询、最少连接、地理亲和等），将任务派发给真正的执行节点。
5. 执行结果（成功/失败、资源使用）回传至 Head 节点，再通过委托链路推送给全局调度器，用于后续策略调整。

### 2.3 框架特性
- **解耦**：全局层不关注域内细节，只关心可调度能力与 SLA；域内可独立演进。
- **容错**：若 Head 节点不可达，调度器会回退并重新挑选候选域；域内调度失败也会向上反馈，触发重调度。
- **弹性**：通过 `registry.Manager` 的无锁快照与随机化选择，快速扩展到大量域；域内可采用自定义策略插件。
- **安全治理**：跨域交互通过 TLS/gRPC（目前 demo 使用 insecure，生产需替换），配合访问控制与审计。

### 2.4 与部署体系的对接
- `docker-compose.yaml` 将 `iarnet-global`、`iarnet-1/2`、多个 Provider 及 `dind` 环境编排在同一测试网络，模拟多域场景。
- `provider-docker-*` 通过 `DOCKER_HOST=tcp://dind-*` 独立控制底层容器；`iarnet-*` 主体通过宿主机 Docker socket 管理工作空间与运行时。
- `global_registry_addr` 指向 `host.docker.internal:50010`，使得各域 Head 节点可访问全局控制面。

## 3. 算力网络资源调度委托

### 3.1 委托概念
- 调度委托是指全局调度器在筛选符合条件的域后，将调度权转交给该域的 Head 节点，由其在域内完成最终编排。
- 该机制兼顾全局最优与域内自治，避免全局调度器直接管理所有节点导致的扩展与安全问题。

### 3.2 请求生命周期
1. **受理**：`DeployComponent` 对外暴露 RPC，校验请求与资源描述；若参数缺失，直接返回 `failureResponse`。
2. **候选筛选**：读取 `Registry.Manager` 缓存的 `Domain` 与 `Node` 信息，剔除 `NodeStatus!=online`、缺少 `Address` 或资源不足的节点。
3. **委托发起**：使用 `grpc.NewClient` 连接目标 Head 节点，对接其调度服务；默认超时 `10s`，可在配置中调整以适配跨地域网络。
4. **域内执行**：Head 节点接管后，可根据任务类型调用 Docker Provider / Kubernetes / 自研执行器，并在本地记录任务状态、日志。
5. **结果回传**：Raw 执行结果封装为 `schedulerpb.DeployComponentResponse`，携带 `Success` 与 `Error` 字段，由全局调度器透传给请求方。

### 3.3 管控与可视化
- 委托链路的关键操作记录在 `logrus` 日志中，可被集中收集进行审计。
- 前端可结合 `resource_logger` 中的执行记录和全局调度日志，展示“委托拓扑”与任务生命周期。
- 若需要强一致审计，可引入事件溯源（如 Kafka/JetStream）记录每一次委托及域内调度决策，便于追责。

### 3.4 扩展方向
- **策略插件**：在委托前增加策略评估（成本、能耗、合规性），支持多目标优化。
- **委托仲裁**：支持失败自动降级到其他域，或并行委托多域做竞价运行。
- **零信任传输**：结合 SPIFFE/SPIRE 或 mTLS，实现跨域身份认证和链路加密。

---

本文档覆盖当前项目在资源态势、分级调度与委托链路三方面的架构设计，可作为后续研发迭代与评审的基础材料。
