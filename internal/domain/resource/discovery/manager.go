package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/types"
	"github.com/sirupsen/logrus"
)

// NodeDiscoveryManager 管理节点发现和资源感知
type NodeDiscoveryManager struct {
	mu sync.RWMutex

	// 本地节点信息
	localNode *PeerNode

	// 已知节点（节点 ID -> 节点信息）
	knownNodes map[string]*PeerNode

	// 节点地址映射（地址 -> 节点 ID，用于快速查找）
	addressToNodeID map[string]string

	// Peer 地址列表（用于 gossip）
	peerAddresses map[string]struct{} // peer address -> struct{}

	// Gossip 配置
	gossipInterval time.Duration // Gossip 间隔
	nodeTTL        time.Duration // 节点信息过期时间
	maxGossipPeers int           // 每次 gossip 的最大 peer 数量
	maxHops        int           // 最大跳数

	// 消息去重（防止重复处理）
	processedMessages map[string]time.Time // message_id -> timestamp
	messageTTL        time.Duration        // 消息去重 TTL

	// 回调函数
	onNodeDiscovered func(node *PeerNode)            // 节点发现回调
	onNodeUpdated    func(node *PeerNode)            // 节点更新回调
	onNodeLost       func(nodeID string)             // 节点丢失回调
	gossipCallback   func(ctx context.Context) error // Gossip 执行回调（由 service 层设置）

	// 内部状态
	gossipStop    chan struct{} // 停止信号
	cleanupTicker *time.Ticker  // 清理定时器

	// 资源聚合视图
	aggregateView *ResourceAggregateView // 聚合所有节点的资源视图
}

// NewNodeDiscoveryManager 创建节点发现管理器
func NewNodeDiscoveryManager(
	localNodeID string,
	localNodeName string,
	localAddress string,
	localSchedulerAddress string,
	domainID string,
	initialPeers []string,
	gossipInterval time.Duration,
	nodeTTL time.Duration,
) *NodeDiscoveryManager {
	localNode := &PeerNode{
		NodeID:           localNodeID,
		NodeName:         localNodeName,
		Address:          localAddress,
		SchedulerAddress: localSchedulerAddress,
		DomainID:         domainID,
		Status:           NodeStatusOnline,
		DiscoveredAt:     time.Now(),
		LastSeen:         time.Now(),
		LastUpdated:      time.Now(),
		Version:          1,
	}

	peerAddresses := make(map[string]struct{})
	for _, addr := range initialPeers {
		if addr != "" {
			peerAddresses[addr] = struct{}{}
		}
	}

	return &NodeDiscoveryManager{
		localNode:         localNode,
		knownNodes:        make(map[string]*PeerNode),
		addressToNodeID:   make(map[string]string),
		peerAddresses:     peerAddresses,
		gossipInterval:    gossipInterval,
		nodeTTL:           nodeTTL,
		maxGossipPeers:    10,
		maxHops:           5,
		processedMessages: make(map[string]time.Time),
		messageTTL:        5 * time.Minute, // 消息去重 TTL：5 分钟
		gossipStop:        make(chan struct{}),
		aggregateView:     NewResourceAggregateView(),
	}
}

// Start 启动节点发现管理器
func (m *NodeDiscoveryManager) Start(ctx context.Context) error {
	logrus.Info("Starting node discovery manager")

	// 启动清理定时器
	m.cleanupTicker = time.NewTicker(1 * time.Minute)
	go m.cleanupLoop(ctx)

	// 启动 gossip 循环
	go m.gossipLoop(ctx)

	logrus.Info("Node discovery manager started")
	return nil
}

// Stop 停止节点发现管理器
func (m *NodeDiscoveryManager) Stop() {
	logrus.Info("Stopping node discovery manager")

	close(m.gossipStop)
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}

	logrus.Info("Node discovery manager stopped")
}

// GetLocalNode 获取本地节点信息
func (m *NodeDiscoveryManager) GetLocalNode() *PeerNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	return m.copyPeerNode(m.localNode)
}

// UpdateLocalNode 更新本地节点信息（当资源状态变化时调用）
func (m *NodeDiscoveryManager) UpdateLocalNode(
	resourceCapacity *types.Capacity,
	resourceTags *ResourceTags,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldVersion := m.localNode.Version
	oldResourceInfo := "no resources"
	if m.localNode.ResourceCapacity != nil && m.localNode.ResourceCapacity.Total != nil {
		oldResourceInfo = fmt.Sprintf("CPU: %d mC", m.localNode.ResourceCapacity.Total.CPU)
	}

	m.localNode.ResourceCapacity = resourceCapacity
	m.localNode.ResourceTags = resourceTags
	m.localNode.LastUpdated = time.Now()
	m.localNode.Version++
	m.localNode.LastSeen = time.Now()

	newResourceInfo := "no resources"
	if resourceCapacity != nil && resourceCapacity.Total != nil {
		newResourceInfo = fmt.Sprintf("CPU: %d mC", resourceCapacity.Total.CPU)
	}

	logrus.Infof("Updated local node resources (version: %d -> %d): %s -> %s",
		oldVersion, m.localNode.Version, oldResourceInfo, newResourceInfo)

	// 更新聚合视图
	m.updateAggregateView()
}

// GetKnownNodes 获取所有已知节点
func (m *NodeDiscoveryManager) GetKnownNodes() []*PeerNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*PeerNode, 0, len(m.knownNodes))
	for _, node := range m.knownNodes {
		nodes = append(nodes, m.copyPeerNode(node))
	}
	return nodes
}

// GetNodeByID 根据节点 ID 获取节点信息
func (m *NodeDiscoveryManager) GetNodeByID(nodeID string) (*PeerNode, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.knownNodes[nodeID]
	if !ok {
		return nil, false
	}
	return m.copyPeerNode(node), true
}

// GetNodeByAddress 根据地址获取节点信息
func (m *NodeDiscoveryManager) GetNodeByAddress(address string) (*PeerNode, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodeID, ok := m.addressToNodeID[address]
	if !ok {
		return nil, false
	}

	node, ok := m.knownNodes[nodeID]
	if !ok {
		return nil, false
	}
	return m.copyPeerNode(node), true
}

// GetPeerAddresses 获取所有 peer 地址
func (m *NodeDiscoveryManager) GetPeerAddresses() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	addresses := make([]string, 0, len(m.peerAddresses))
	for addr := range m.peerAddresses {
		addresses = append(addresses, addr)
	}
	return addresses
}

// GetMaxGossipPeers 获取每次 gossip 的最大 peer 数量
func (m *NodeDiscoveryManager) GetMaxGossipPeers() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxGossipPeers
}

// SetMaxGossipPeers 设置每次 gossip 的最大 peer 数量
func (m *NodeDiscoveryManager) SetMaxGossipPeers(maxPeers int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxGossipPeers = maxPeers
}

// GetMaxHops 获取最大跳数
func (m *NodeDiscoveryManager) GetMaxHops() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxHops
}

// SetMaxHops 设置最大跳数
func (m *NodeDiscoveryManager) SetMaxHops(maxHops int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxHops = maxHops
}

// AddPeer 添加 peer 地址
func (m *NodeDiscoveryManager) AddPeer(address string) {
	if address == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.peerAddresses[address] = struct{}{}
	logrus.Debugf("Added peer address: %s", address)
}

// RemovePeer 移除 peer 地址
func (m *NodeDiscoveryManager) RemovePeer(address string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.peerAddresses, address)
	logrus.Debugf("Removed peer address: %s", address)
}

// ProcessNodeInfo 处理接收到的节点信息（从 gossip 消息中）
func (m *NodeDiscoveryManager) ProcessNodeInfo(node *PeerNode, sourcePeer string) {
	if node == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 只处理同域节点
	if node.DomainID != m.localNode.DomainID {
		logrus.Debugf("Ignoring node from different domain: %s (local domain: %s)", node.DomainID, m.localNode.DomainID)
		return
	}

	// 忽略本地节点
	if node.NodeID == m.localNode.NodeID {
		return
	}

	existing, exists := m.knownNodes[node.NodeID]

	if !exists {
		// 新节点发现
		node.DiscoveredAt = time.Now()
		node.SourcePeer = sourcePeer
		m.knownNodes[node.NodeID] = node
		m.addressToNodeID[node.Address] = node.NodeID

		// 记录资源信息
		resourceInfo := "no resources"
		if node.ResourceCapacity != nil && node.ResourceCapacity.Total != nil {
			resourceInfo = fmt.Sprintf("CPU: %d mC, Memory: %d B, GPU: %d",
				node.ResourceCapacity.Total.CPU,
				node.ResourceCapacity.Total.Memory,
				node.ResourceCapacity.Total.GPU)
		}
		logrus.Infof("Discovered new node: %s (%s) at %s, resources: %s", node.NodeName, node.NodeID, node.Address, resourceInfo)

		// 更新聚合视图
		m.updateAggregateView()

		// 触发回调
		if m.onNodeDiscovered != nil {
			go m.onNodeDiscovered(m.copyPeerNode(node))
		}
	} else {
		// 更新现有节点
		oldVersion := existing.Version
		if existing.UpdateFrom(node) {
			existing.SourcePeer = sourcePeer
			existing.LastSeen = time.Now()

			// 记录资源信息变化
			oldResourceInfo := "no resources"
			newResourceInfo := "no resources"
			if existing.ResourceCapacity != nil && existing.ResourceCapacity.Total != nil {
				oldResourceInfo = fmt.Sprintf("CPU: %d mC", existing.ResourceCapacity.Total.CPU)
			}
			if node.ResourceCapacity != nil && node.ResourceCapacity.Total != nil {
				newResourceInfo = fmt.Sprintf("CPU: %d mC", node.ResourceCapacity.Total.CPU)
			}
			logrus.Infof("Updated node info: %s (version: %d -> %d), resources: %s -> %s",
				node.NodeID, oldVersion, existing.Version, oldResourceInfo, newResourceInfo)

			// 更新聚合视图
			m.updateAggregateView()

			// 触发回调
			if m.onNodeUpdated != nil {
				go m.onNodeUpdated(m.copyPeerNode(existing))
			}
		}
	}
}

// FindAvailableNodes 查找满足资源要求的可用节点
func (m *NodeDiscoveryManager) FindAvailableNodes(
	resourceRequest *ResourceRequest,
	requiredTags *ResourceTags,
) []*PeerNode {
	if resourceRequest == nil {
		return nil
	}

	// 转换为 types.Info
	req := &types.Info{
		CPU:    resourceRequest.CPU,
		Memory: resourceRequest.Memory,
		GPU:    resourceRequest.GPU,
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.aggregateView.FindAvailableNodes(req, requiredTags)
}

// GetAggregateView 获取资源聚合视图
func (m *NodeDiscoveryManager) GetAggregateView() *ResourceAggregateView {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.aggregateView
}

// SetOnNodeDiscovered 设置节点发现回调
func (m *NodeDiscoveryManager) SetOnNodeDiscovered(callback func(node *PeerNode)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onNodeDiscovered = callback
}

// SetOnNodeUpdated 设置节点更新回调
func (m *NodeDiscoveryManager) SetOnNodeUpdated(callback func(node *PeerNode)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onNodeUpdated = callback
}

// SetOnNodeLost 设置节点丢失回调
func (m *NodeDiscoveryManager) SetOnNodeLost(callback func(nodeID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onNodeLost = callback
}

// gossipLoop gossip 循环
func (m *NodeDiscoveryManager) gossipLoop(ctx context.Context) {
	ticker := time.NewTicker(m.gossipInterval)
	defer ticker.Stop()

	// 立即执行一次
	m.performGossip(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.gossipStop:
			return
		case <-ticker.C:
			m.performGossip(ctx)
		}
	}
}

// SetGossipCallback 设置 gossip 执行回调（由 service 层调用）
func (m *NodeDiscoveryManager) SetGossipCallback(callback func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gossipCallback = callback
}

// performGossip 执行一次 gossip（由外部服务调用）
func (m *NodeDiscoveryManager) performGossip(ctx context.Context) {
	// 如果有回调函数，调用它执行实际的 gossip
	m.mu.RLock()
	callback := m.gossipCallback
	m.mu.RUnlock()

	if callback != nil {
		if err := callback(ctx); err != nil {
			logrus.Debugf("Gossip callback failed: %v", err)
		}
	}

	// 更新聚合视图
	m.mu.Lock()
	m.updateAggregateView()
	m.mu.Unlock()
}

// cleanupLoop 清理循环
func (m *NodeDiscoveryManager) cleanupLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.gossipStop:
			return
		case <-m.cleanupTicker.C:
			m.cleanup()
		}
	}
}

// cleanup 清理过期节点和消息
func (m *NodeDiscoveryManager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var lostNodes []string

	// 清理过期节点
	for nodeID, node := range m.knownNodes {
		if node.IsStale(m.nodeTTL) {
			logrus.Infof("Node %s (%s) is stale, removing", node.NodeName, nodeID)
			delete(m.knownNodes, nodeID)
			delete(m.addressToNodeID, node.Address)
			lostNodes = append(lostNodes, nodeID)
		}
	}

	// 清理过期消息
	for messageID, timestamp := range m.processedMessages {
		if now.Sub(timestamp) > m.messageTTL {
			delete(m.processedMessages, messageID)
		}
	}

	// 更新聚合视图
	if len(lostNodes) > 0 {
		m.updateAggregateView()

		// 触发节点丢失回调
		if m.onNodeLost != nil {
			for _, nodeID := range lostNodes {
				go m.onNodeLost(nodeID)
			}
		}
	}
}

// updateAggregateView 更新聚合视图
func (m *NodeDiscoveryManager) updateAggregateView() {
	// 构建节点列表（包括本地节点和已知节点）
	nodes := make([]*PeerNode, 0, len(m.knownNodes)+1)
	nodes = append(nodes, m.localNode)
	for _, node := range m.knownNodes {
		nodes = append(nodes, node)
	}

	// 更新聚合视图
	m.aggregateView.Update(nodes)
}

// copyPeerNode 复制节点信息（避免并发问题）
func (m *NodeDiscoveryManager) copyPeerNode(node *PeerNode) *PeerNode {
	if node == nil {
		return nil
	}

	copy := &PeerNode{
		NodeID:           node.NodeID,
		NodeName:         node.NodeName,
		Address:          node.Address,
		SchedulerAddress: node.SchedulerAddress,
		DomainID:         node.DomainID,
		Status:           node.Status,
		LastSeen:         node.LastSeen,
		LastUpdated:      node.LastUpdated,
		DiscoveredAt:     node.DiscoveredAt,
		SourcePeer:       node.SourcePeer,
		Version:          node.Version,
		GossipCount:      node.GossipCount,
	}

	// 复制资源容量
	if node.ResourceCapacity != nil {
		copy.ResourceCapacity = &types.Capacity{}
		if node.ResourceCapacity.Total != nil {
			copy.ResourceCapacity.Total = &types.Info{
				CPU:    node.ResourceCapacity.Total.CPU,
				Memory: node.ResourceCapacity.Total.Memory,
				GPU:    node.ResourceCapacity.Total.GPU,
			}
		}
		if node.ResourceCapacity.Used != nil {
			copy.ResourceCapacity.Used = &types.Info{
				CPU:    node.ResourceCapacity.Used.CPU,
				Memory: node.ResourceCapacity.Used.Memory,
				GPU:    node.ResourceCapacity.Used.GPU,
			}
		}
		if node.ResourceCapacity.Available != nil {
			copy.ResourceCapacity.Available = &types.Info{
				CPU:    node.ResourceCapacity.Available.CPU,
				Memory: node.ResourceCapacity.Available.Memory,
				GPU:    node.ResourceCapacity.Available.GPU,
			}
		}
	}

	// 复制资源标签
	if node.ResourceTags != nil {
		copy.ResourceTags = &ResourceTags{
			CPU:    node.ResourceTags.CPU,
			GPU:    node.ResourceTags.GPU,
			Memory: node.ResourceTags.Memory,
			Camera: node.ResourceTags.Camera,
		}
	}

	return copy
}

// IsMessageProcessed 检查消息是否已处理
func (m *NodeDiscoveryManager) IsMessageProcessed(messageID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.processedMessages[messageID]
	return exists
}

// MarkMessageProcessed 标记消息已处理
func (m *NodeDiscoveryManager) MarkMessageProcessed(messageID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.processedMessages[messageID] = time.Now()
}

// GetNodesForGossip 获取用于 gossip 的节点列表（包括本地节点和已知节点）
func (m *NodeDiscoveryManager) GetNodesForGossip() []*PeerNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*PeerNode, 0, len(m.knownNodes)+1)
	nodes = append(nodes, m.copyPeerNode(m.localNode))
	for _, node := range m.knownNodes {
		nodes = append(nodes, m.copyPeerNode(node))
	}
	return nodes
}
