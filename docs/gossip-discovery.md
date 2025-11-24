# Gossip 节点发现与资源感知

## 概述

Gossip 节点发现是 iarnet 中用于实现分布式节点自动发现和资源感知的核心机制。通过 gossip 协议，网络中的节点可以自动发现同域内的其他节点，并实时感知它们的资源状态，从而实现去中心化的资源发现和协作。

## 架构设计

### 核心组件

```
┌─────────────────────────────────────────────────────────┐
│                    iarnet 节点                            │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  ┌──────────────────┐      ┌──────────────────┐        │
│  │  ResourceManager │      │ DiscoveryService │        │
│  │                  │◄────►│                  │        │
│  │  - 资源聚合      │      │  - Gossip 执行   │        │
│  │  - Provider管理  │      │  - 节点查询     │        │
│  └──────────────────┘      └──────────────────┘        │
│           │                        │                     │
│           │                        │                     │
│           ▼                        ▼                     │
│  ┌──────────────────┐      ┌──────────────────┐        │
│  │ NodeDiscoveryMgr │      │  gRPC Server     │        │
│  │                  │      │                  │        │
│  │  - 节点管理      │      │  - GossipNodeInfo│        │
│  │  - 资源聚合视图  │      │  - QueryResources│        │
│  │  - 消息去重      │      │  - ExchangePeers │        │
│  └──────────────────┘      └──────────────────┘        │
│                                                           │
└─────────────────────────────────────────────────────────┘
                            │
                            │ gRPC (insecure)
                            │
        ┌───────────────────┴───────────────────┐
        │                                         │
        ▼                                         ▼
┌──────────────┐                        ┌──────────────┐
│  Peer Node 1 │                        │  Peer Node 2 │
└──────────────┘                        └──────────────┘
```

### 模块层次

1. **领域层 (Domain Layer)**
   - `discovery/types.go`: 核心数据类型定义
   - `discovery/manager.go`: 节点发现管理器
   - `discovery/service.go`: 发现服务接口和实现
   - `discovery/aggregate.go`: 资源聚合视图

2. **传输层 (Transport Layer)**
   - `rpc/resource/discovery/server.go`: gRPC 服务器实现
   - `http/resource/api.go`: HTTP API 端点

3. **协议层 (Protocol Layer)**
   - `proto/resource/discovery/discovery.proto`: Protocol Buffer 定义

## 核心概念

### 1. PeerNode（对等节点）

`PeerNode` 表示通过 gossip 发现的同域节点，包含以下信息：

- **节点标识**
  - `NodeID`: 节点唯一标识符（持久化）
  - `NodeName`: 节点名称（来自配置）
  - `Address`: 节点地址（host:port，用于 gRPC 通信）
  - `DomainID`: 所属域 ID（只发现同域节点）

- **资源信息**
  - `ResourceCapacity`: 资源容量（Total/Used/Available）
  - `ResourceTags`: 资源标签（CPU/GPU/Memory/Camera）

- **状态信息**
  - `Status`: 节点状态（online/offline/error）
  - `LastSeen`: 最后活跃时间
  - `LastUpdated`: 最后更新时间

- **Gossip 元数据**
  - `DiscoveredAt`: 首次发现时间
  - `SourcePeer`: 发现来源
  - `Version`: 版本号（用于冲突解决）
  - `GossipCount`: 传播次数（用于 TTL）

### 2. Gossip 协议

Gossip 协议采用**定期随机传播**的方式：

1. **定期传播**: 每隔 `gossip_interval_seconds` 秒执行一次 gossip
2. **随机选择**: 每次随机选择最多 `max_gossip_peers` 个 peer 进行通信
3. **信息交换**: 交换本地节点信息和已知的部分节点信息
4. **TTL 控制**: 通过 `max_hops` 限制消息传播范围，防止无限传播

### 3. 消息去重

为了防止重复处理相同的 gossip 消息，系统维护一个消息去重缓存：

- 使用 `message_id` 作为唯一标识
- 消息在缓存中保留 `messageTTL` 时间（默认 5 分钟）
- 定期清理过期消息

### 4. 节点超时

节点信息具有生存时间（TTL）：

- 如果节点在 `node_ttl_seconds` 秒内没有更新，将被标记为过期
- 过期节点会被自动清理
- 触发 `onNodeLost` 回调

### 5. 版本控制

节点信息使用版本号进行冲突解决：

- 每个节点维护一个递增的版本号
- 当收到其他节点的信息时，只接受版本号更高或相同但时间戳更新的信息
- 确保信息的一致性

## 数据结构和协议

### Protocol Buffer 定义

主要消息类型：

```protobuf
// 节点信息
message PeerNodeInfo {
    string node_id = 1;
    string node_name = 2;
    string address = 3;
    string domain_id = 4;
    ResourceCapacity resource_capacity = 5;
    ResourceTags resource_tags = 6;
    NodeStatus status = 7;
    int64 last_seen = 8;
    int64 last_updated = 9;
    uint64 version = 10;
    int32 gossip_count = 11;
}

// Gossip 消息
message NodeInfoGossipMessage {
    string sender_node_id = 1;
    string sender_address = 2;
    string sender_domain_id = 3;
    repeated PeerNodeInfo nodes = 4;
    string message_id = 5;
    int64 timestamp = 6;
    int32 ttl = 7;
    int32 max_hops = 8;
}

// 资源查询请求
message ResourceQueryRequest {
    string query_id = 1;
    string requester_node_id = 2;
    ResourceRequest resource_request = 5;
    ResourceTags required_tags = 6;
    int32 max_hops = 8;
    int32 ttl = 9;
    int32 current_hops = 10;
}
```

### gRPC 服务

```protobuf
service DiscoveryService {
    // 交换节点信息（gossip 协议）
    rpc GossipNodeInfo(NodeInfoGossipMessage) returns (NodeInfoGossipResponse);
    
    // 查询资源（主动查询）
    rpc QueryResources(ResourceQueryRequest) returns (ResourceQueryResponse);
    
    // 交换 peer 列表（用于节点发现）
    rpc ExchangePeerList(PeerListExchangeRequest) returns (PeerListExchangeResponse);
    
    // 获取本地节点信息
    rpc GetLocalNodeInfo(GetLocalNodeInfoRequest) returns (GetLocalNodeInfoResponse);
}
```

## 工作流程

### 1. 节点启动

```
1. 加载配置（discovery.enabled = true）
2. 创建 NodeDiscoveryManager
   - 初始化本地节点信息
   - 设置初始 peer 地址列表
3. 创建 DiscoveryService
   - 设置 gossip 回调
4. 启动服务
   - 启动 gossip 循环
   - 启动清理循环
   - 启动 gRPC 服务器
```

### 2. Gossip 传播流程

```
节点 A                         节点 B
  │                              │
  │── GossipNodeInfo ───────────►│
  │   (包含 A 和已知节点信息)      │
  │                              │── 处理节点信息
  │                              │   - 更新已知节点
  │                              │   - 检查版本号
  │                              │
  │◄── NodeInfoGossipResponse ───│
  │   (包含 B 和已知节点信息)      │
  │                              │
  │── 处理响应中的节点信息 ────────│
  │   - 更新已知节点              │
  │   - 触发回调                  │
```

### 3. 资源查询流程

```
查询节点                       目标节点
  │                              │
  │── QueryResources ───────────►│
  │   (资源需求 + 标签)           │
  │                              │── 检查本地资源
  │                              │   - 查找可用节点
  │                              │   - 检查资源标签
  │                              │
  │◄── ResourceQueryResponse ─────│
  │   (可用节点列表)              │
  │                              │
  │── 处理响应                   │
```

### 4. 资源状态同步

```
ResourceManager                DiscoveryService
  │                              │
  │── 健康检查循环 ───────────────►│
  │   (每 30 秒)                  │
  │                              │── 聚合资源状态
  │                              │   - 从所有 Provider
  │                              │   - 计算容量和标签
  │                              │
  │◄── UpdateLocalNode ──────────│
  │   (资源容量 + 标签)           │
  │                              │
  │── 更新本地节点信息 ────────────│
  │   - 更新版本号                │
  │   - 触发 gossip               │
```

## 配置说明

### 配置文件示例

```yaml
resource:
  discovery:
    enabled: true                    # 是否启用 gossip 发现
    gossip_interval_seconds: 30     # Gossip 间隔（秒）
    node_ttl_seconds: 180           # 节点信息过期时间（秒）
    max_gossip_peers: 10            # 每次 gossip 的最大 peer 数量
    max_hops: 5                      # 最大跳数
    query_timeout_seconds: 5        # 资源查询超时时间（秒）
    fanout: 3                        # 每次传播的节点数（fanout）
    use_anti_entropy: true           # 是否使用反熵机制
    anti_entropy_interval_seconds: 300  # 反熵间隔（秒）

transport:
  rpc:
    discovery:
      port: 50005                    # 节点发现 RPC 端口

# 初始 peer 地址列表（可选）
initial_peers:
  - "peer1.example.com:50005"
  - "peer2.example.com:50005"
```

### 配置参数说明

| 参数 | 说明 | 默认值 | 建议值 |
|------|------|--------|--------|
| `enabled` | 是否启用 gossip 发现 | `false` | `true` |
| `gossip_interval_seconds` | Gossip 传播间隔 | `30` | `30-60` |
| `node_ttl_seconds` | 节点信息过期时间 | `180` | `180-300` |
| `max_gossip_peers` | 每次 gossip 的最大 peer 数量 | `10` | `5-10` |
| `max_hops` | 最大跳数（限制传播范围） | `5` | `3-5` |
| `query_timeout_seconds` | 资源查询超时时间 | `5` | `5-10` |
| `fanout` | 每次传播的节点数 | `3` | `3-5` |
| `use_anti_entropy` | 是否使用反熵机制 | `false` | `true` |
| `anti_entropy_interval_seconds` | 反熵间隔 | `300` | `300-600` |

## API 接口

### HTTP API

#### 获取发现的节点列表

**端点**: `GET /api/resource/discovery/nodes`

**响应**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "nodes": [
      {
        "node_id": "node.abc123",
        "node_name": "node.1",
        "address": "192.168.1.100:50005",
        "domain_id": "domain.xyz789",
        "status": "online",
        "cpu": {
          "total": 8000,
          "used": 2000,
          "available": 6000
        },
        "memory": {
          "total": 17179869184,
          "used": 4294967296,
          "available": 12884901888
        },
        "gpu": {
          "total": 2,
          "used": 0,
          "available": 2
        },
        "resource_tags": {
          "cpu": true,
          "gpu": true,
          "memory": true,
          "camera": false
        },
        "last_seen": "2024-01-15T10:30:00Z"
      }
    ],
    "total": 1
  }
}
```

**字段说明**:
- `node_id`: 节点唯一标识符
- `node_name`: 节点名称
- `address`: 节点地址（host:port）
- `domain_id`: 所属域 ID
- `status`: 节点状态（online/offline/error）
- `cpu/memory/gpu`: 资源使用情况
  - `total`: 总资源（CPU: millicores, Memory: bytes, GPU: count）
  - `used`: 已使用资源
  - `available`: 可用资源
- `resource_tags`: 资源标签
- `last_seen`: 最后活跃时间（RFC3339 格式）

### gRPC API

#### GossipNodeInfo

交换节点信息（gossip 协议的核心方法）。

**请求**: `NodeInfoGossipMessage`
**响应**: `NodeInfoGossipResponse`

#### QueryResources

查询满足资源要求的可用节点。

**请求**: `ResourceQueryRequest`
**响应**: `ResourceQueryResponse`

#### ExchangePeerList

交换 peer 地址列表，用于节点发现。

**请求**: `PeerListExchangeRequest`
**响应**: `PeerListExchangeResponse`

#### GetLocalNodeInfo

获取本地节点信息。

**请求**: `GetLocalNodeInfoRequest`
**响应**: `GetLocalNodeInfoResponse`

## 前端集成

### 前端 API 调用

```typescript
// 获取发现的节点列表
const response = await resourcesAPI.getDiscoveredNodes()
const nodes = response.nodes

// 节点数据结构
interface DiscoveredNodeItem {
  node_id: string
  node_name: string
  address: string
  domain_id: string
  status: "online" | "offline" | "error"
  cpu?: ResourceUsage
  memory?: ResourceUsage
  gpu?: ResourceUsage
  resource_tags?: ResourceTagsInfo
  last_seen: string
}
```

### 前端显示

远程资源面板显示通过 gossip 发现的节点信息：

- **节点名称**: 显示节点名称和节点 ID
- **地址**: 显示节点地址（host:port）
- **状态**: 显示节点状态（在线/离线/错误）
- **CPU 使用率**: 显示 CPU 使用百分比和已用/总量
- **内存使用率**: 显示内存使用百分比和已用/总量
- **最后更新**: 显示最后活跃时间
- **操作**: 提供刷新按钮

## 实现细节

### 1. 节点发现管理器 (NodeDiscoveryManager)

**职责**:
- 管理本地节点信息
- 维护已知节点列表
- 处理节点信息更新
- 管理 peer 地址列表
- 维护资源聚合视图
- 消息去重
- 节点超时清理

**关键方法**:
- `Start(ctx)`: 启动管理器（启动 gossip 循环和清理循环）
- `Stop()`: 停止管理器
- `GetKnownNodes()`: 获取所有已知节点
- `ProcessNodeInfo(node, sourcePeer)`: 处理接收到的节点信息
- `UpdateLocalNode(capacity, tags)`: 更新本地节点信息

### 2. 发现服务 (DiscoveryService)

**职责**:
- 执行 gossip 传播
- 处理资源查询
- 交换 peer 列表
- 提供本地节点信息

**关键方法**:
- `PerformGossip(ctx)`: 执行一次 gossip
- `QueryResources(ctx, request, tags)`: 查询资源
- `GetKnownNodes()`: 获取已知节点
- `UpdateLocalNode(capacity, tags)`: 更新本地节点信息

### 3. 资源聚合视图 (ResourceAggregateView)

**职责**:
- 聚合所有已知节点的资源
- 按资源类型分组节点
- 按可用资源排序节点
- 查找满足条件的可用节点

**关键方法**:
- `Update(nodes)`: 更新聚合视图
- `FindAvailableNodes(request, tags)`: 查找可用节点

### 4. gRPC 服务器

**职责**:
- 接收和处理 gossip 消息
- 处理资源查询请求
- 处理 peer 列表交换请求
- 提供本地节点信息查询

**关键方法**:
- `GossipNodeInfo(ctx, req)`: 处理 gossip 消息
- `QueryResources(ctx, req)`: 处理资源查询
- `ExchangePeerList(ctx, req)`: 处理 peer 列表交换
- `GetLocalNodeInfo(ctx, req)`: 提供本地节点信息

## 最佳实践

### 1. 配置建议

- **小规模网络** (< 10 节点):
  - `gossip_interval_seconds`: 30
  - `max_gossip_peers`: 5
  - `max_hops`: 3

- **中等规模网络** (10-50 节点):
  - `gossip_interval_seconds`: 30
  - `max_gossip_peers`: 10
  - `max_hops`: 5

- **大规模网络** (> 50 节点):
  - `gossip_interval_seconds`: 60
  - `max_gossip_peers`: 10
  - `max_hops`: 5-7

### 2. 网络要求

- 节点之间需要能够通过 gRPC 通信
- 建议使用内网或 VPN 连接
- 确保防火墙允许 discovery 端口（默认 50005）

### 3. 性能考虑

- Gossip 间隔不宜过短（建议 >= 30 秒）
- 每次 gossip 的 peer 数量不宜过多（建议 <= 10）
- 合理设置节点 TTL，避免频繁清理和重新发现

### 4. 故障处理

- 节点离线：通过 TTL 机制自动清理
- 网络分区：不同分区的节点无法发现彼此
- 消息丢失：通过定期 gossip 自动恢复

## 故障排查

### 常见问题

1. **节点无法发现其他节点**
   - 检查 `discovery.enabled` 是否为 `true`
   - 检查 `initial_peers` 配置是否正确
   - 检查网络连接和防火墙设置
   - 检查 discovery 端口是否可访问

2. **节点信息不更新**
   - 检查 gossip 间隔配置
   - 检查节点是否在线
   - 查看日志中的错误信息

3. **资源信息不准确**
   - 检查 ResourceManager 是否正确同步资源状态
   - 检查 Provider 是否正常连接
   - 检查资源聚合逻辑

### 日志查看

关键日志位置：
- 节点发现: `Node discovery manager started`
- Gossip 执行: `Performing gossip...`
- 节点发现: `Discovered new node: ...`
- 节点更新: `Updated node info: ...`
- 节点超时: `Node ... timed out and removed`

## 未来改进

1. **反熵机制**: 实现更完善的反熵机制，确保信息一致性
2. **加密通信**: 支持 TLS 加密的 gRPC 通信
3. **负载均衡**: 基于资源使用率的智能负载均衡
4. **跨域发现**: 支持跨域节点发现（需要权限控制）
5. **性能优化**: 优化大规模网络的 gossip 性能

## 相关文件

- 协议定义: `proto/resource/discovery/discovery.proto`
- 领域模型: `internal/domain/resource/discovery/types.go`
- 管理器: `internal/domain/resource/discovery/manager.go`
- 服务实现: `internal/domain/resource/discovery/service.go`
- 聚合视图: `internal/domain/resource/discovery/aggregate.go`
- gRPC 服务器: `internal/transport/rpc/resource/discovery/server.go`
- HTTP API: `internal/transport/http/resource/api.go`
- 前端页面: `web/app/resources/page.tsx`

## 参考资料

- [Gossip Protocol](https://en.wikipedia.org/wiki/Gossip_protocol)
- [gRPC Documentation](https://grpc.io/docs/)
- [Protocol Buffers](https://protobuf.dev/)

