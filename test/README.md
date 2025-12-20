# 测试说明

本文档对应《测试大纲》中“算力网络资源态势感知”“面向跨资源域调度的分级调度框架”“算力网络资源调度委托”全部子项。每个测试用例均可直接在 `iarnet/test` 目录下通过 `go test ./...` 运行；若测试依赖 Docker，请提前保证本地 Docker Daemon 可用。下文按模块说明所覆盖的子测试点、前置条件与关键执行步骤。

---

## 1. 算力网络资源态势感知

### 1.1 本地资源接入（Docker Provider / 本地资源接入、k8s 资源接入）
- **文件**：`situation-awareness/docker_provider_test.go`
- **覆盖子测试点**
  - Provider 连接注册、鉴权与心跳
  - 资源容量/可用量/标签读取
  - 资源监控任务（GetAllocated/HealthCheck）
  - 多次查询的实时性与一致性
- **前置条件**
  - 本地已安装 Docker，`docker` CLI 可正常连接
  - 允许以 root/当前用户运行容器
- **执行步骤概览**
  1. 初始化 Provider Service，调用 `Connect` 完成注册，检验返回的 `ProviderID`、Token。
  2. 通过 `HealthCheck`/`GetAllocated` 获取总量、已用、可用资源，打印 CPU/Memory/GPU 等指标并核对一致性。
  3. 创建受限资源的测试容器（如 1CPU/256MB），再次查询资源占用，确认指标随容器变化。
  4. 连续多次查询验证实时性；随后删除容器并确认释放效果。
  5. 在“连接状态下”场景中追加鉴权（Token 校验）、异常链路测试。

### 1.2 远程资源发现（Gossip 资源发现）
- **文件**：`situation-awareness/gossip_discovery_test.go`
- **覆盖子测试点**
  - 新增 peers：通过 `ProcessNodeInfo` 模拟 Gossip 接收并触发 `OnNodeDiscovered`
  - 聚合视图：验证聚合算力总量、标签、可用节点查询
  - 过期治理：基于短 TTL 构造 `fresh` 与 `stale` 节点，验证过期清理与回调
- **前置条件**
  - 不依赖真实网络，完全使用内存管理器；只需本地 Go 环境
- **执行步骤概览**
  1. 创建 `NodeDiscoveryManager`，设置本地节点资源与标签。
  2. 依次注入 1~N 个 Peer，校验节点被加入、资源信息正确，打印拓扑。
  3. 聚合视图测试中，构建多节点场景，调用 `GetAggregateView` 检查总量、标签与查询接口。
  4. 过期治理测试设置极短 TTL，构造已过期节点，等待/触发清理逻辑，验证 `OnNodeLost` 回调和节点移除。

### 1.3 资源状态监控（资源实时使用查询 / 健康检查）
- **文件**：`situation-awareness/resource_monitoring_test.go`
- **覆盖子测试点**
  - 实时使用查询：`GetAllocated` 在容器创建/销毁前后的变化
  - 健康检查监控：`HealthCheck` 多次执行的一致性、标签正确性
  - 监控连续性：多次轮询、对比首尾数据保证一致
- **前置条件**
  - 同 1.1，需可用 Docker 环境
- **执行步骤概览**
  1. 注册 Provider，记录初始资源。
  2. 创建受控容器改变占用，再次查询确保数值提升。
  3. 进行多次 `GetAllocated`/`HealthCheck` 调用并打印结果。
  4. 健康检查场景中同时验证容量计算（Total = Used + Available）、标签保持一致。
  5. 清理容器后再次查询，确认资源下降。

### 1.4 远程 Docker 感知 / 远程 k8s 感知
- **文件**：`situation-awareness/remote_docker_perception_test.go`（以及其他远程感知测试）
- **覆盖子测试点**
  - 通过远程 API 拉取远端 Docker/K8s 集群资源
  - 结合 Gossip/Discovery 获取跨域资源态势
- **前置条件**
  - 需在测试配置中提供远程端点或使用 mock client（文件内部包含模拟客户端）
- **执行步骤概览**
  1. 构造远程 Provider/Cluster 客户端，伪造资源快照。
  2. 将快照写入 Discovery Manager，验证跨域容量被聚合。
  3. 断言资源标签、可用节点列表中包含远端节点。

---

## 2. 面向跨资源域调度的分级调度框架

### 2.1 本地优先调度
- **文件**：`hierarchical-scheduling/local_priority_test.go`
- **覆盖子测试点**
  - 本地资源充足：优先在本地完成部署，不触发远程查询
  - 本地资源不足：可靠地向上层返回错误，由上层决定是否升级
- **前置条件**
  - 使用 fake local manager/ discovery，无真实依赖
- **执行步骤**
  1. 构造虚拟本地节点与远程拓扑，输出节点视图。
  2. 场景一：发起低需求请求，断言 `scheduler.Service` 返回本地 Provider，`discovery` 未被调用。
  3. 场景二：本地 manager 返回 “无可用资源” 错误，确认返回体携带错误文本且未触发远程查询。

### 2.2 跨域调度（触发条件 / 目标节点选择 / 过期节点清理 / 失败降级）
- **文件**：`hierarchical-scheduling/cross_domain_test.go`
- **覆盖子测试点**
  - **触发条件**：本地失败且错误包含 “failed to find available provider” 时自动升级
  - **目标节点选择**：优先选可调度地址、资源最多的节点
  - **过期节点清理**：根据 TTL 过滤掉久未更新节点
  - **失败降级**：域内调度失败后降级至全局调度
- **前置条件**
  - 纯 mock（`mockLocal`、`mockPeerExecutor`、`mockGlobal`）即可
- **执行步骤**
  1. 触发条件：local 连续失败 -> peer selector 返回节点 -> peer executor 执行；校验路径为 `peer`。
  2. 目标节点：准备两个 Peer（一个含 SchedulerAddress），确保调度命中资源更优+可调度节点。
  3. 过期节点：构造 `LastSeen` 远古节点与新鲜节点，设置 TTL，验证只会命中新鲜节点。
  4. 失败降级：本地失败，域内执行出错，最终走 `globalExecutor`。

---

## 3. 算力网络资源调度委托（两阶段提交机制）

- **文件**：`delegated-scheduling/delegated_scheduling_test.go`

本模块测试委托调度机制，采用两阶段提交（Propose -> Commit）的方式，支持策略评估和本地决策。所有测试用例使用 Mock 实现，不依赖真实网络环境。

### 3.1 一次成功部署
- **测试函数**：`TestDelegatedScheduling_SuccessfulDeployment`
- **覆盖子测试点**
  - 本地节点通过 RPC 调用远程节点的 `ProposeRemoteSchedule` 获取调度结果
  - 使用策略链评估远程调度结果（资源安全裕度策略）
  - 策略评估通过后，调用 `CommitRemoteSchedule` 确认部署
  - 验证两阶段提交流程的完整性和正确性
- **执行步骤**
  1. 本地节点调用 `ProposeRemoteSchedule`，传入资源请求和目标节点信息
  2. 远程节点返回调度结果（NodeID、ProviderID、可用资源）
  3. 本地节点使用策略链评估调度结果，验证资源安全裕度
  4. 策略评估通过后，调用 `CommitRemoteSchedule` 确认部署
  5. 验证部署成功，返回正确的 NodeID 和 ProviderID

### 3.2 拒绝并重新调度
- **测试函数**：`TestDelegatedScheduling_RejectAndRetry`
- **覆盖子测试点**
  - 第一次调度结果被策略拒绝（资源不满足安全裕度要求）
  - 重新调用 `ProposeRemoteSchedule` 获取新的调度结果
  - 第二次调度结果通过策略评估，成功部署
  - 验证重试机制和策略拒绝流程
- **执行步骤**
  1. 第一次调用 `ProposeRemoteSchedule`，获取调度结果
  2. 策略评估：资源不满足安全裕度要求，返回 `DecisionReject`
  3. 记录拒绝原因，重新调用 `ProposeRemoteSchedule`
  4. 第二次获取调度结果，资源充足，满足安全裕度要求
  5. 策略评估通过，调用 `CommitRemoteSchedule` 确认部署
  6. 验证最终部署成功

### 3.3 直接获取列表并本地决策
- **测试函数**：`TestDelegatedScheduling_ListProvidersAndLocalDecision`
- **覆盖子测试点**
  - 远程节点无自主调度能力时，调用 `ListRemoteProviders` 获取 Provider 列表
  - 本地节点根据资源需求在 Provider 列表中进行调度决策
  - 选择满足资源需求且资源最充足的 Provider
  - 使用选中的 Provider 调用 `CommitRemoteSchedule` 确认部署
- **执行步骤**
  1. 调用 `ListRemoteProviders`，传入目标节点信息和 `includeResources=true`
  2. 获取远程节点的所有 Provider 列表及其资源信息（可用资源、总容量、已使用资源、资源标签）
  3. 本地节点遍历 Provider 列表，根据资源需求（CPU、Memory、GPU）筛选满足条件的 Provider
  4. 在满足条件的 Provider 中选择资源最充足的（按可用 CPU 排序）
  5. 使用选中的 ProviderID 调用 `CommitRemoteSchedule` 确认部署
  6. 验证部署成功，返回正确的 ProviderID

---

## 4. 运行方式与注意事项

### 4.1 运行全部测试
在 `iarnet` 根目录执行：
```bash
go test -v ./test/...
```

### 4.2 运行各子任务测试
- **态势感知**：`go test -v ./test/situation-awareness`
- **分级调度**：`go test -v ./test/hierarchical-scheduling`
- **委托调度**：`go test -v ./test/delegated-scheduling`

### 4.3 前置条件
- **Docker 相关测试**：需要本地 Docker 环境可用
  - 建议先运行 `docker ps` 确保守护进程存活
  - 必要时请以 root 或加入 `docker` 组
  - 需要 Docker 的测试会自动跳过（SKIP）如果 Docker 不可用
- **Kubernetes 相关测试**：需要可用的 Kubernetes 集群
  - 需要配置 `~/.kube/config` 或设置 `KUBECONFIG` 环境变量
  - 如果 Kubernetes 不可用，相关测试会自动跳过（SKIP）
- **委托调度测试**：完全使用 Mock 实现，无需额外依赖

### 4.4 测试结果说明
- **PASS**：测试通过
- **SKIP**：测试跳过（通常因为前置条件不满足，如 Docker/K8s 不可用）
- **FAIL**：测试失败（需要检查代码或环境配置）

### 4.5 测试覆盖范围
- ✅ **态势感知**：Docker Provider、K8s Provider、Gossip 发现、资源监控
- ✅ **分级调度**：本地优先调度、跨域调度
- ✅ **委托调度**：两阶段提交、策略评估、本地决策
