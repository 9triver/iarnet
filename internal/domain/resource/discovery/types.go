package discovery

import (
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/types"
)

// ResourceTags 资源标签（描述节点支持的计算资源类型）
type ResourceTags struct {
	CPU    bool
	GPU    bool
	Memory bool
	Camera bool
}

// NewEmptyResourceTags 创建空的资源标签
func NewEmptyResourceTags() *ResourceTags {
	return &ResourceTags{
		CPU:    false,
		GPU:    false,
		Memory: false,
		Camera: false,
	}
}

// NewResourceTags 创建资源标签
func NewResourceTags(cpu, gpu, memory, camera bool) *ResourceTags {
	return &ResourceTags{
		CPU:    cpu,
		GPU:    gpu,
		Memory: memory,
		Camera: camera,
	}
}

// HasResource 检查是否支持指定的资源类型
func (rt *ResourceTags) HasResource(resourceType string) bool {
	if rt == nil {
		return false
	}
	switch resourceType {
	case "cpu":
		return rt.CPU
	case "gpu":
		return rt.GPU
	case "memory":
		return rt.Memory
	case "camera":
		return rt.Camera
	default:
		return false
	}
}

// NodeStatus 节点状态
type NodeStatus string

const (
	// NodeStatusOnline 节点在线
	NodeStatusOnline NodeStatus = "online"
	// NodeStatusOffline 节点离线
	NodeStatusOffline NodeStatus = "offline"
	// NodeStatusError 节点错误
	NodeStatusError NodeStatus = "error"
	// NodeStatusUnknown 节点未知
	NodeStatusUnknown NodeStatus = "unknown"
)

// ResourceRequest 资源请求（用于查询）
type ResourceRequest struct {
	CPU    int64
	Memory int64
	GPU    int64
}

// PeerNode 表示通过 gossip 发现的同域节点
// 复用 global registry 的 Node 概念，但用于 peer-to-peer 发现
type PeerNode struct {
	// 节点标识（与 global registry 保持一致）
	NodeID   string // 节点唯一标识符（持久化的 node_id）
	NodeName string // 节点名称（来自配置 resource.name）
	Address  string // 节点地址，格式：host:port（用于 gRPC 通信）
	DomainID string // 所属域 ID（只发现同域节点）

	// 资源信息（复用现有类型）
	ResourceCapacity *types.Capacity // 资源容量（Total/Used/Available）
	ResourceTags     *ResourceTags   // 资源标签（CPU/GPU/Memory/Camera）

	// 状态信息
	Status      NodeStatus // 节点状态（online/offline/error）
	LastSeen    time.Time  // 最后活跃时间
	LastUpdated time.Time  // 最后更新时间

	// Gossip 元数据
	DiscoveredAt time.Time // 首次发现时间
	SourcePeer   string    // 发现来源（哪个 peer 告知的）
	Version      uint64    // 版本号（用于冲突解决）
	GossipCount  int       // 传播次数（用于 TTL）
}

// IsStale 检查节点信息是否过期
func (n *PeerNode) IsStale(ttl time.Duration) bool {
	return time.Since(n.LastSeen) > ttl
}

// UpdateFrom 从另一个节点信息更新（版本控制）
func (n *PeerNode) UpdateFrom(other *PeerNode) bool {
	if other == nil {
		return false
	}

	// 版本控制：只接受版本号更高或相同但时间戳更新的信息
	if other.Version < n.Version {
		return false
	}
	if other.Version == n.Version && other.LastUpdated.Before(n.LastUpdated) {
		return false
	}

	// 更新信息
	n.NodeName = other.NodeName
	n.Address = other.Address
	n.DomainID = other.DomainID
	// 资源信息：始终更新（包括 nil），因为这是节点当前的真实状态
	// 如果节点资源信息从 nil 变为有值，说明节点恢复了资源，应该更新
	// 如果节点资源信息从有值变为 nil，说明节点失去了资源，也应该更新
	n.ResourceCapacity = other.ResourceCapacity
	n.ResourceTags = other.ResourceTags
	n.Status = other.Status
	n.LastSeen = other.LastSeen
	n.LastUpdated = other.LastUpdated
	n.Version = other.Version
	n.GossipCount = other.GossipCount

	return true
}
