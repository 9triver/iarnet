# 测试说明

## 1 算力网络资源态势感知

> `test/resource/provider_service_test.go`
> `test/resource/gossip_discovery_test.go`
> `test/resource/remote_provider_test.go`

| 测试用例 | 关联大纲 | 说明 |
| --- | --- | --- |
| `TestProviderService_RegisterDockerProvider` | 1.1 本地资源接入（Docker） | 搭建假设的 Docker Provider，验证 gRPC 接入、类型识别与容量同步流程。 |
| `TestProviderService_RegisterK8sProvider` | 1.2 本地资源接入（K8s） | 独立 K8s Provider 场景，确保节点能接入 K8s 集群并拉取容量。 |
| `TestProviderService_SelectLocalProviderPrefersAvailable` | 1.1 本地资源筛选 | 注册 Provider 后依据 `types.Info` 进行匹配，验证节点内部“本地优先”的筛选逻辑。 |
| `TestProviderService_SelectProviderInsufficientCapacity` | 1.2 请求溢出反馈 | 请求超出容量时返回错误，体现总体方案中的资源不足告警路径。 |
| `TestProviderService_LoadProviders_Docker` | 1.1 持久化恢复（Docker） | 从 DAO 还原 Docker Provider，校验节点重启后仍能感知其资源。 |
| `TestProviderService_LoadProviders_K8s` | 1.2 持久化恢复（K8s） | 同理验证 K8s Provider 的状态恢复并保持 Connected。 |
| `TestGossipDiscoveryRegistersRemotePeers` | 1.3 Gossip 资源发现（新增 peers） | 模拟跨节点 Provider/Peers 交换，确保能建立新的 peer 连接。 |
| `TestGossipDiscoveryCachesRemoteProviderView` | 1.3 Gossip 资源发现（聚合视图） | 缓存远端节点上报的 Provider 信息并标记来源 peer，形成全局视图。 |
| `TestGossipDiscoveryCleanupRemovesStaleProviders` | 1.3 Gossip 资源发现（过期治理） | 回拨 `lastSeen` 并清理失联节点，防止过期资源干扰调度。 |
| `TestRemoteDockerProviderOperations` | 1.4 远程资源委托（Docker） | 在 Gossip 已同步 endpoint 的前提下，验证远程 Docker Provider 的 Connect/GetCapacity/Deploy。 |
| `TestRemoteK8sProviderOperations` | 1.5 远程资源委托（K8s） | 复用同步信息连接 K8s Provider，读取可用算力。 |
| `TestProviderManagerAggregateCapacity` | 1.6 资源实时使用监控 | 聚合多 Provider Total/Used/Available，形成全局监控指标。 |

## 2 面向跨资源域调度的分级调度框架

> `test/resource/remote_provider_test.go`
> `test/resource/scheduler_policy_test.go`

| 测试用例 | 关联大纲 | 说明 |
| --- | --- | --- |
| `TestGossipAwareScheduler_LocalPriority` | 2.1 节点自治优先调度 | 引入 Gossip 感知但仍命中本地 Provider，体现 3.3.4 强调的“就近调度”准则。 |
| `TestGossipAwareScheduler_RemoteFallback` | 2.2 跨域委托调度策略 | 本地不足时，根据 Gossip 汇报的远端节点总资源发起委托，重现“本地→远端”分级协同。 |
| `TestGossipAwareScheduler_RemoteStaleRemoved` | 2.3 过期节点清理 | Gossip 将长时间未上报的远端节点剔除，调度层不再考虑其资源，保障视图时效性。 |
| `TestGossipAwareScheduler_RemoteUnavailable` | 2.3 故障兜底 | 远端节点拒绝或无法接管任务时，调度层能够停止委托并返回失败，体现故障回退路径。 |

## 3 面向算力网络的智能应用运行平台

> `test/application/manager_test.go`
> `test/application/workspace_service_test.go`

| 测试用例 | 关联大纲 | 说明 |
| --- | --- | --- |
| `TestWorkspaceService_ResourceView` | 3.1 资源查看功能 | 通过真实工作目录构造，拉取文件树并识别 `main.go` 语言，验证"可视化查看"能力。 |
| `TestWorkspaceService_ResourceAccessAndManagement` | 3.2/3.3 资源接入+管理 | 覆盖目录/文件创建、保存、删除与 Clean 操作，体现接入后的工作区治理。 |
| `TestManager_CreateApplication` | 3.4 应用导入功能 | 以 fake Runner/Workspace 驱动异步克隆与 Runner 创建流程，验证导入后状态切换为 `Undeployed`。 |
| `TestManager_RunApplication` | 3.5 应用部署功能 | 基于元数据、工作区、Runner 依赖协同，验证部署成功后状态更新至 `Running`。 |
| `TestManager_RunApplication_FailureUpdatesStatus` | 3.6 应用管理功能 | 注入 Runner 启动失败，期望状态回写为 `Failed`，体现运行态管理与告警。 |

## 4 Web端功能测试

### 4.1 资源管理页面功能

> `test/web/resources_page_test.go`

| 测试用例 | 关联功能 | 说明 |
| --- | --- | --- |
| `TestResourcesPage_LoadResourceList` | 资源列表查看 | 验证页面加载时正确获取并显示所有Provider列表。 |
| `TestResourcesPage_DisplayGlobalCapacity` | 全局资源容量统计 | 验证页面顶部正确显示全局CPU、内存总容量、已使用量和可用量。 |
| `TestResourcesPage_RegisterDockerProvider` | Docker Provider注册 | 验证通过表单注册Docker Provider，包括URL解析、连接测试和注册流程。 |
| `TestResourcesPage_RegisterK8sProvider` | K8s Provider注册 | 验证通过表单注册K8s Provider，包括配置验证和注册流程。 |
| `TestResourcesPage_EditProvider` | 资源编辑功能 | 验证编辑Provider信息（名称、URL、Token等）并更新成功。 |
| `TestResourcesPage_DeleteProvider` | 资源删除功能 | 验证删除Provider操作，包括确认提示和删除后的列表更新。 |
| `TestResourcesPage_RefreshProvider` | 资源刷新功能 | 验证单个Provider的状态刷新，包括容量信息更新和状态同步。 |
| `TestResourcesPage_TestConnection` | 连接测试功能 | 验证Provider连接测试，包括成功和失败场景的提示信息。 |
| `TestResourcesPage_DisplayProviderStatus` | 资源状态显示 | 验证Provider状态（connected/disconnected/error）的正确显示和颜色标识。 |
| `TestResourcesPage_DisplayProviderCapacity` | 资源容量显示 | 验证每个Provider的CPU、内存使用情况的正确显示和格式化。 |

### 4.2 应用管理页面功能

> `test/web/applications_page_test.go`

| 测试用例 | 关联功能 | 说明 |
| --- | --- | --- |
| `TestApplicationsPage_LoadApplicationList` | 应用列表查看 | 验证页面加载时正确获取并显示所有应用列表。 |
| `TestApplicationsPage_DisplayApplicationStats` | 应用统计信息显示 | 验证应用统计卡片正确显示总数、运行中、已停止、未部署、失败数量。 |
| `TestApplicationsPage_CreateApplicationFromGit` | Git仓库应用导入 | 验证从Git仓库导入应用，包括URL验证、分支选择和导入流程。 |
| `TestApplicationsPage_ValidateGitUrl` | Git URL验证 | 验证Git URL格式验证（HTTPS和SSH格式）和错误提示。 |
| `TestApplicationsPage_ConvertGitUrl` | Git URL转换 | 验证SSH格式Git URL转换为HTTPS格式。 |
| `TestApplicationsPage_EditApplication` | 应用编辑功能 | 验证编辑应用信息（名称、描述、执行命令等）并更新成功。 |
| `TestApplicationsPage_DeleteApplication` | 应用删除功能 | 验证删除应用操作，包括确认提示和删除后的列表更新。 |
| `TestApplicationsPage_RunApplication` | 应用部署功能 | 验证启动应用部署，包括状态更新为deploying和running。 |
| `TestApplicationsPage_StopApplication` | 应用停止功能 | 验证停止运行中的应用，包括状态更新为stopped。 |
| `TestApplicationsPage_RefreshApplicationList` | 应用列表刷新 | 验证手动刷新应用列表和统计数据。 |
| `TestApplicationsPage_DisplayApplicationStatus` | 应用状态显示 | 验证应用状态（idle/running/stopped/deploying/cloning/error）的正确显示和颜色标识。 |
| `TestApplicationsPage_NavigateToDetail` | 应用详情跳转 | 验证点击应用卡片跳转到应用详情页面。 |

### 4.3 应用详情页面功能

> `test/web/application_detail_page_test.go`

| 测试用例 | 关联功能 | 说明 |
| --- | --- | --- |
| `TestApplicationDetailPage_LoadApplicationInfo` | 应用详情加载 | 验证应用详情页正确加载应用基本信息（名称、描述、Git URL、状态等）。 |
| `TestApplicationDetailPage_DisplayDAGVisualization` | DAG可视化显示 | 验证DAG图的正确渲染，包括控制节点和数据节点的显示。 |
| `TestApplicationDetailPage_DAGNodeStatusUpdate` | DAG节点状态更新 | 验证通过WebSocket接收DAG节点状态更新并实时刷新可视化。 |
| `TestApplicationDetailPage_LoadFileTree` | 文件树加载 | 验证应用文件树的正确加载和目录结构显示。 |
| `TestApplicationDetailPage_ViewFileContent` | 文件内容查看 | 验证查看文件内容功能，包括代码高亮显示。 |
| `TestApplicationDetailPage_EditFileContent` | 文件内容编辑 | 验证编辑文件内容并保存成功。 |
| `TestApplicationDetailPage_CreateFile` | 文件创建功能 | 验证创建新文件功能。 |
| `TestApplicationDetailPage_DeleteFile` | 文件删除功能 | 验证删除文件功能。 |
| `TestApplicationDetailPage_LoadComponents` | 组件列表加载 | 验证应用组件列表的正确加载，包括组件状态和资源使用情况。 |
| `TestApplicationDetailPage_ViewComponentLogs` | 组件日志查看 | 验证查看单个组件的日志信息。 |
| `TestApplicationDetailPage_LoadApplicationLogs` | 应用日志加载 | 验证应用日志的正确加载，包括日志行数选择和解析。 |
| `TestApplicationDetailPage_LogSearchAndFilter` | 日志搜索和过滤 | 验证日志关键字搜索和日志级别过滤功能。 |
| `TestApplicationDetailPage_WebSocketRealtimeUpdate` | WebSocket实时更新 | 验证WebSocket连接建立和DAG状态实时更新机制。 |
| `TestApplicationDetailPage_EditApplicationConfig` | 应用配置编辑 | 验证编辑应用配置（执行命令、环境安装命令、运行环境等）。 |


## 运行方式

> 按用户当前要求，暂不执行测试；如需验证，可在本地执行：
> - iarnet 平台侧（资源/调度）：`cd iarnet && go test ./test/resource/...`
> - iarnet 平台侧（应用/工作区）：`cd iarnet && go test ./test/application/...`
> - Provider 端：`cd iarnet/providers/test && go test ./...`
> - Web 端功能测试：`cd iarnet && go test ./test/web/...`（待实现）

后续会继续按照《测试大纲》完善：

- Gossip 资源发现、PeerProvider 远程委托测试；
- Application / Runner / Workspace 等平台层测试；
- 典型算力协同场景的端到端验证。

如需新增测试，请遵循：

1. 在 `iarnet/test/<子域>` 下创建对应 `_test.go`；
2. 在本文件补充说明，以便与研究方案保持映射。

