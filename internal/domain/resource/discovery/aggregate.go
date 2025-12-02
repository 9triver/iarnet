package discovery

import (
	"sort"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/types"
)

// ResourceAggregateView 聚合所有已知节点的资源视图
type ResourceAggregateView struct {
	mu sync.RWMutex

	// 聚合资源容量（所有在线节点的资源总和）
	AggregatedCapacity *types.Capacity

	// 聚合资源标签（所有节点支持的资源类型）
	AggregatedTags *types.ResourceTags

	// 节点统计
	TotalNodes   int // 总节点数
	OnlineNodes  int // 在线节点数
	OfflineNodes int // 离线节点数
	ErrorNodes   int // 错误节点数

	// 按资源类型分组的节点（用于快速查找）
	// key: "cpu", "gpu", "memory", "camera"
	// value: 支持该资源类型的节点列表
	NodesByResourceType map[string][]*PeerNode

	// 按可用资源排序的节点（用于资源调度）
	NodesByAvailability []*PeerNode // 按可用资源降序排列

	// 最后更新时间
	LastUpdated time.Time
}

// NewResourceAggregateView 创建资源聚合视图
func NewResourceAggregateView() *ResourceAggregateView {
	return &ResourceAggregateView{
		AggregatedCapacity: &types.Capacity{
			Total:     &types.Info{},
			Used:      &types.Info{},
			Available: &types.Info{},
		},
		AggregatedTags:      types.NewEmptyResourceTags(),
		NodesByResourceType: make(map[string][]*PeerNode),
		NodesByAvailability: make([]*PeerNode, 0),
		LastUpdated:         time.Now(),
	}
}

// Update 更新聚合视图（基于已知节点列表）
func (v *ResourceAggregateView) Update(nodes []*PeerNode) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// 重置统计
	v.TotalNodes = len(nodes)
	v.OnlineNodes = 0
	v.OfflineNodes = 0
	v.ErrorNodes = 0

	// 重置聚合资源
	v.AggregatedCapacity = &types.Capacity{
		Total:     &types.Info{},
		Used:      &types.Info{},
		Available: &types.Info{},
	}
	v.AggregatedTags = types.NewEmptyResourceTags()

	// 重置分组
	v.NodesByResourceType = make(map[string][]*PeerNode)
	v.NodesByAvailability = make([]*PeerNode, 0)

	// 聚合在线节点的资源
	for _, node := range nodes {
		// 统计节点状态
		switch node.Status {
		case NodeStatusOnline:
			v.OnlineNodes++
		case NodeStatusOffline:
			v.OfflineNodes++
		case NodeStatusError:
			v.ErrorNodes++
		}

		// 只聚合在线节点的资源
		if node.Status != NodeStatusOnline {
			continue
		}

		// 聚合资源容量
		if node.ResourceCapacity != nil {
			if node.ResourceCapacity.Total != nil {
				v.AggregatedCapacity.Total.CPU += node.ResourceCapacity.Total.CPU
				v.AggregatedCapacity.Total.Memory += node.ResourceCapacity.Total.Memory
				v.AggregatedCapacity.Total.GPU += node.ResourceCapacity.Total.GPU
			}
			if node.ResourceCapacity.Used != nil {
				v.AggregatedCapacity.Used.CPU += node.ResourceCapacity.Used.CPU
				v.AggregatedCapacity.Used.Memory += node.ResourceCapacity.Used.Memory
				v.AggregatedCapacity.Used.GPU += node.ResourceCapacity.Used.GPU
			}
			if node.ResourceCapacity.Available != nil {
				v.AggregatedCapacity.Available.CPU += node.ResourceCapacity.Available.CPU
				v.AggregatedCapacity.Available.Memory += node.ResourceCapacity.Available.Memory
				v.AggregatedCapacity.Available.GPU += node.ResourceCapacity.Available.GPU
			}
		}

		// 聚合资源标签
		if node.ResourceTags != nil {
			if node.ResourceTags.CPU {
				v.AggregatedTags.CPU = true
				v.NodesByResourceType["cpu"] = append(v.NodesByResourceType["cpu"], node)
			}
			if node.ResourceTags.GPU {
				v.AggregatedTags.GPU = true
				v.NodesByResourceType["gpu"] = append(v.NodesByResourceType["gpu"], node)
			}
			if node.ResourceTags.Memory {
				v.AggregatedTags.Memory = true
				v.NodesByResourceType["memory"] = append(v.NodesByResourceType["memory"], node)
			}
			if node.ResourceTags.Camera {
				v.AggregatedTags.Camera = true
				v.NodesByResourceType["camera"] = append(v.NodesByResourceType["camera"], node)
			}
		}

		// 添加到可用性排序列表
		v.NodesByAvailability = append(v.NodesByAvailability, node)
	}

	// 按可用资源排序（降序）
	sort.Slice(v.NodesByAvailability, func(i, j int) bool {
		nodeI := v.NodesByAvailability[i]
		nodeJ := v.NodesByAvailability[j]

		// 获取可用资源
		var availI, availJ int64
		if nodeI.ResourceCapacity != nil && nodeI.ResourceCapacity.Available != nil {
			availI = nodeI.ResourceCapacity.Available.CPU + nodeI.ResourceCapacity.Available.Memory/1024/1024 + nodeI.ResourceCapacity.Available.GPU*1000
		}
		if nodeJ.ResourceCapacity != nil && nodeJ.ResourceCapacity.Available != nil {
			availJ = nodeJ.ResourceCapacity.Available.CPU + nodeJ.ResourceCapacity.Available.Memory/1024/1024 + nodeJ.ResourceCapacity.Available.GPU*1000
		}

		return availI > availJ
	})

	v.LastUpdated = time.Now()
}

// FindAvailableNodes 查找满足资源要求的可用节点
func (v *ResourceAggregateView) FindAvailableNodes(
	resourceRequest *types.Info,
	requiredTags *types.ResourceTags,
) []*PeerNode {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var availableNodes []*PeerNode

	for _, node := range v.NodesByAvailability {
		if node.Status != NodeStatusOnline {
			continue
		}

		// 检查资源标签
		if requiredTags != nil {
			if requiredTags.CPU && (node.ResourceTags == nil || !node.ResourceTags.CPU) {
				continue
			}
			if requiredTags.GPU && (node.ResourceTags == nil || !node.ResourceTags.GPU) {
				continue
			}
			if requiredTags.Memory && (node.ResourceTags == nil || !node.ResourceTags.Memory) {
				continue
			}
			if requiredTags.Camera && (node.ResourceTags == nil || !node.ResourceTags.Camera) {
				continue
			}
		}

		// 检查资源容量
		if node.ResourceCapacity != nil && node.ResourceCapacity.Available != nil {
			available := node.ResourceCapacity.Available
			if available.CPU >= resourceRequest.CPU &&
				available.Memory >= resourceRequest.Memory &&
				available.GPU >= resourceRequest.GPU {
				availableNodes = append(availableNodes, node)
			}
		}
	}

	return availableNodes
}

// GetAggregatedCapacity 获取聚合的资源容量
func (v *ResourceAggregateView) GetAggregatedCapacity() *types.Capacity {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// 返回副本
	if v.AggregatedCapacity == nil {
		return nil
	}

	return &types.Capacity{
		Total: &types.Info{
			CPU:    v.AggregatedCapacity.Total.CPU,
			Memory: v.AggregatedCapacity.Total.Memory,
			GPU:    v.AggregatedCapacity.Total.GPU,
		},
		Used: &types.Info{
			CPU:    v.AggregatedCapacity.Used.CPU,
			Memory: v.AggregatedCapacity.Used.Memory,
			GPU:    v.AggregatedCapacity.Used.GPU,
		},
		Available: &types.Info{
			CPU:    v.AggregatedCapacity.Available.CPU,
			Memory: v.AggregatedCapacity.Available.Memory,
			GPU:    v.AggregatedCapacity.Available.GPU,
		},
	}
}

// GetAggregatedTags 获取聚合的资源标签
func (v *ResourceAggregateView) GetAggregatedTags() *types.ResourceTags {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.AggregatedTags == nil {
		return types.NewEmptyResourceTags()
	}

	// 返回副本
	return &types.ResourceTags{
		CPU:    v.AggregatedTags.CPU,
		GPU:    v.AggregatedTags.GPU,
		Memory: v.AggregatedTags.Memory,
		Camera: v.AggregatedTags.Camera,
	}
}
