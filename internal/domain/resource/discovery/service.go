package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/types"
	registrypb "github.com/9triver/iarnet/internal/proto/global/registry"
	discoverypb "github.com/9triver/iarnet/internal/proto/resource/discovery"
	"github.com/9triver/iarnet/internal/util"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Service 节点发现服务接口
type Service interface {
	// Start 启动服务
	Start(ctx context.Context) error

	// Stop 停止服务
	Stop()

	// PerformGossip 执行一次 gossip（与所有已知 peer 交换信息）
	PerformGossip(ctx context.Context) error

	// QueryResources 查询资源（主动查询）
	QueryResources(ctx context.Context, resourceRequest *types.Info, requiredTags *ResourceTags) ([]*PeerNode, error)

	// GetKnownNodes 获取所有已知节点
	GetKnownNodes() []*PeerNode

	// GetLocalNode 获取本地节点信息
	GetLocalNode() *PeerNode

	// UpdateLocalNode 更新本地节点信息
	// resourceTags 可以是 *ResourceTags 或 *registrypb.ResourceTags（来自 global registry）
	UpdateLocalNode(resourceCapacity *types.Capacity, resourceTags interface{})
}

type service struct {
	manager *NodeDiscoveryManager
}

// NewService 创建节点发现服务
func NewService(manager *NodeDiscoveryManager) Service {
	return &service{
		manager: manager,
	}
}

// Start 启动服务
func (s *service) Start(ctx context.Context) error {
	// 设置 gossip 回调，让 manager 的 gossipLoop 能够调用 service 的 PerformGossip
	s.manager.SetGossipCallback(s.PerformGossip)
	return s.manager.Start(ctx)
}

// Stop 停止服务
func (s *service) Stop() {
	s.manager.Stop()
}

// PerformGossip 执行一次 gossip
func (s *service) PerformGossip(ctx context.Context) error {
	peerAddresses := s.manager.GetPeerAddresses()
	if len(peerAddresses) == 0 {
		logrus.Debug("No peers to gossip with")
		return nil
	}

	// 限制每次 gossip 的 peer 数量
	maxPeers := s.manager.GetMaxGossipPeers()
	if len(peerAddresses) > maxPeers {
		peerAddresses = peerAddresses[:maxPeers]
	}

	// 获取要发送的节点信息
	nodesToSend := s.manager.GetNodesForGossip()

	// 与每个 peer 进行 gossip
	for _, peerAddr := range peerAddresses {
		if err := s.gossipWithPeer(ctx, peerAddr, nodesToSend); err != nil {
			logrus.Debugf("Failed to gossip with peer %s: %v", peerAddr, err)
			// 继续处理其他 peer，不中断
		}
	}

	return nil
}

// gossipWithPeer 与单个 peer 进行 gossip
func (s *service) gossipWithPeer(ctx context.Context, peerAddr string, nodesToSend []*PeerNode) error {
	// 创建 gRPC 连接
	conn, err := grpc.NewClient(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to peer %s: %w", peerAddr, err)
	}
	defer conn.Close()

	// 创建客户端
	client := discoverypb.NewDiscoveryServiceClient(conn)

	// 获取本地节点信息
	localNode := s.manager.GetLocalNode()

	// 转换节点信息为 proto 消息
	protoNodes := make([]*discoverypb.PeerNodeInfo, 0, len(nodesToSend))
	for _, node := range nodesToSend {
		protoNode := convertPeerNodeToProto(node, 0)
		if protoNode != nil {
			// 记录发送的节点资源信息
			resourceInfo := "no resources"
			if node.ResourceCapacity != nil && node.ResourceCapacity.Total != nil {
				resourceInfo = fmt.Sprintf("CPU: %d mC, Memory: %d B, GPU: %d",
					node.ResourceCapacity.Total.CPU,
					node.ResourceCapacity.Total.Memory,
					node.ResourceCapacity.Total.GPU)
			}
			logrus.Debugf("Sending node info in gossip: %s (%s), resources: %s",
				node.NodeName, node.NodeID, resourceInfo)
			protoNodes = append(protoNodes, protoNode)
		}
	}

	// 构建 gossip 消息
	messageID := util.GenIDWith("gossip.")
	gossipMessage := &discoverypb.NodeInfoGossipMessage{
		SenderNodeId:   localNode.NodeID,
		SenderAddress:  localNode.Address,
		SenderDomainId: localNode.DomainID,
		Nodes:          protoNodes,
		MessageId:      messageID,
		Timestamp:      time.Now().UnixNano(),
		Ttl:            int32(s.manager.GetMaxHops()),
		MaxHops:        int32(s.manager.GetMaxHops()),
	}

	// 调用 RPC
	resp, err := client.GossipNodeInfo(ctx, gossipMessage)
	if err != nil {
		return fmt.Errorf("failed to gossip with peer %s: %w", peerAddr, err)
	}

	// 处理响应中的节点信息
	for _, protoNode := range resp.Nodes {
		peerNode := convertProtoToPeerNode(protoNode)
		if peerNode != nil {
			// 记录接收到的节点资源信息
			resourceInfo := "no resources"
			if peerNode.ResourceCapacity != nil && peerNode.ResourceCapacity.Total != nil {
				resourceInfo = fmt.Sprintf("CPU: %d mC, Memory: %d B, GPU: %d",
					peerNode.ResourceCapacity.Total.CPU,
					peerNode.ResourceCapacity.Total.Memory,
					peerNode.ResourceCapacity.Total.GPU)
			}
			logrus.Debugf("Received node info from gossip: %s (%s), resources: %s",
				peerNode.NodeName, peerNode.NodeID, resourceInfo)
			s.manager.ProcessNodeInfo(peerNode, peerAddr)
		}
	}

	logrus.Debugf("Successfully gossiped with peer %s, received %d nodes", peerAddr, len(resp.Nodes))
	return nil
}

// QueryResources 查询资源
func (s *service) QueryResources(ctx context.Context, resourceRequest *types.Info, requiredTags *ResourceTags) ([]*PeerNode, error) {
	if resourceRequest == nil {
		return nil, fmt.Errorf("resource request is required")
	}

	// 转换为内部类型
	req := &ResourceRequest{
		CPU:    resourceRequest.CPU,
		Memory: resourceRequest.Memory,
		GPU:    resourceRequest.GPU,
	}

	if requiredTags != nil {
		logrus.WithFields(logrus.Fields{
			"cpu":    requiredTags.CPU,
			"gpu":    requiredTags.GPU,
			"memory": requiredTags.Memory,
			"camera": requiredTags.Camera,
		}).Info("QueryResources with required tags")
	} else {
		logrus.Info("QueryResources without required tags")
	}

	// 先从本地已知节点查找
	availableNodes := s.manager.FindAvailableNodes(req, requiredTags)
	if len(availableNodes) > 0 {
		return availableNodes, nil
	}

	// 如果本地没有，通过 gossip 查询其他节点
	localNode := s.manager.GetLocalNode()
	peerAddresses := s.manager.GetPeerAddresses()

	// 构建查询请求
	queryID := util.GenIDWith("query.")
	protoReq := &discoverypb.ResourceQueryRequest{
		QueryId:           queryID,
		RequesterNodeId:   localNode.NodeID,
		RequesterAddress:  localNode.Address,
		RequesterDomainId: localNode.DomainID,
		ResourceRequest: &discoverypb.ResourceRequest{
			Cpu:    resourceRequest.CPU,
			Memory: resourceRequest.Memory,
			Gpu:    resourceRequest.GPU,
		},
		Timestamp:   time.Now().UnixNano(),
		MaxHops:     int32(s.manager.GetMaxHops()),
		Ttl:         int32(s.manager.GetMaxHops()),
		CurrentHops: 0,
	}

	// 如果有资源标签要求，添加到请求中
	if requiredTags != nil {
		protoReq.RequiredTags = &discoverypb.ResourceTags{
			Cpu:    requiredTags.CPU,
			Gpu:    requiredTags.GPU,
			Memory: requiredTags.Memory,
			Camera: requiredTags.Camera,
		}
	}

	// 向所有已知 peer 发送查询请求
	for _, peerAddr := range peerAddresses {
		// 创建带超时的上下文
		queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// 创建 gRPC 连接
		conn, err := grpc.NewClient(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logrus.Debugf("Failed to connect to peer %s for resource query: %v", peerAddr, err)
			continue
		}

		// 创建客户端并查询
		client := discoverypb.NewDiscoveryServiceClient(conn)
		resp, err := client.QueryResources(queryCtx, protoReq)
		conn.Close()

		if err != nil {
			logrus.Debugf("Failed to query resources from peer %s: %v", peerAddr, err)
			continue
		}

		// 处理响应中的可用节点
		for _, protoNode := range resp.AvailableNodes {
			peerNode := convertProtoToPeerNode(protoNode)
			if peerNode != nil {
				// 添加到已知节点
				s.manager.ProcessNodeInfo(peerNode, peerAddr)
				// 检查是否满足要求
				if s.nodeSatisfiesRequest(peerNode, req, requiredTags) {
					availableNodes = append(availableNodes, peerNode)
				}
			}
		}

		// 如果找到足够的节点，可以提前返回
		if len(availableNodes) > 0 {
			break
		}
	}

	if len(availableNodes) > 0 {
		return availableNodes, nil
	}

	return nil, fmt.Errorf("no available nodes found")
}

// nodeSatisfiesRequest 检查节点是否满足资源请求
func (s *service) nodeSatisfiesRequest(node *PeerNode, req *ResourceRequest, requiredTags *ResourceTags) bool {
	if node.Status != NodeStatusOnline {
		return false
	}

	// 检查资源标签
	if requiredTags != nil {
		if requiredTags.CPU && (node.ResourceTags == nil || !node.ResourceTags.CPU) {
			return false
		}
		if requiredTags.GPU && (node.ResourceTags == nil || !node.ResourceTags.GPU) {
			return false
		}
		if requiredTags.Memory && (node.ResourceTags == nil || !node.ResourceTags.Memory) {
			return false
		}
		if requiredTags.Camera && (node.ResourceTags == nil || !node.ResourceTags.Camera) {
			return false
		}
	}

	// 检查资源容量
	if node.ResourceCapacity != nil && node.ResourceCapacity.Available != nil {
		available := node.ResourceCapacity.Available
		return available.CPU >= req.CPU &&
			available.Memory >= req.Memory &&
			available.GPU >= req.GPU
	}

	return false
}

// convertPeerNodeToProto 将领域模型转换为 proto 消息（用于 service 层）
func convertPeerNodeToProto(node *PeerNode, gossipCount int) *discoverypb.PeerNodeInfo {
	if node == nil {
		return nil
	}

	protoNode := &discoverypb.PeerNodeInfo{
		NodeId:           node.NodeID,
		NodeName:         node.NodeName,
		Address:          node.Address,
		DomainId:         node.DomainID,
		SchedulerAddress: node.SchedulerAddress,
		Status:           convertNodeStatusToProto(node.Status),
		LastSeen:         node.LastSeen.UnixNano(),
		LastUpdated:      node.LastUpdated.UnixNano(),
		Version:          node.Version,
		GossipCount:      int32(gossipCount),
	}

	// 转换资源容量
	if node.ResourceCapacity != nil {
		protoNode.ResourceCapacity = &discoverypb.ResourceCapacity{}
		if node.ResourceCapacity.Total != nil {
			protoNode.ResourceCapacity.Total = &discoverypb.ResourceInfo{
				Cpu:    node.ResourceCapacity.Total.CPU,
				Memory: node.ResourceCapacity.Total.Memory,
				Gpu:    node.ResourceCapacity.Total.GPU,
			}
		}
		if node.ResourceCapacity.Used != nil {
			protoNode.ResourceCapacity.Used = &discoverypb.ResourceInfo{
				Cpu:    node.ResourceCapacity.Used.CPU,
				Memory: node.ResourceCapacity.Used.Memory,
				Gpu:    node.ResourceCapacity.Used.GPU,
			}
		}
		if node.ResourceCapacity.Available != nil {
			protoNode.ResourceCapacity.Available = &discoverypb.ResourceInfo{
				Cpu:    node.ResourceCapacity.Available.CPU,
				Memory: node.ResourceCapacity.Available.Memory,
				Gpu:    node.ResourceCapacity.Available.GPU,
			}
		}
	}

	// 转换资源标签
	if node.ResourceTags != nil {
		protoNode.ResourceTags = &discoverypb.ResourceTags{
			Cpu:    node.ResourceTags.CPU,
			Gpu:    node.ResourceTags.GPU,
			Memory: node.ResourceTags.Memory,
			Camera: node.ResourceTags.Camera,
		}
	}

	return protoNode
}

// convertProtoToPeerNode 将 proto 消息转换为领域模型（用于 service 层）
func convertProtoToPeerNode(proto *discoverypb.PeerNodeInfo) *PeerNode {
	if proto == nil {
		return nil
	}

	node := &PeerNode{
		NodeID:           proto.NodeId,
		NodeName:         proto.NodeName,
		Address:          proto.Address,
		SchedulerAddress: proto.SchedulerAddress,
		DomainID:         proto.DomainId,
		Status:           convertProtoToNodeStatus(proto.Status),
		LastSeen:         time.Unix(0, proto.LastSeen),
		LastUpdated:      time.Unix(0, proto.LastUpdated),
		Version:          proto.Version,
		GossipCount:      int(proto.GossipCount),
	}

	// 转换资源容量
	if proto.ResourceCapacity != nil {
		node.ResourceCapacity = &types.Capacity{}
		if proto.ResourceCapacity.Total != nil {
			node.ResourceCapacity.Total = &types.Info{
				CPU:    proto.ResourceCapacity.Total.Cpu,
				Memory: proto.ResourceCapacity.Total.Memory,
				GPU:    proto.ResourceCapacity.Total.Gpu,
			}
		}
		if proto.ResourceCapacity.Used != nil {
			node.ResourceCapacity.Used = &types.Info{
				CPU:    proto.ResourceCapacity.Used.Cpu,
				Memory: proto.ResourceCapacity.Used.Memory,
				GPU:    proto.ResourceCapacity.Used.Gpu,
			}
		}
		if proto.ResourceCapacity.Available != nil {
			node.ResourceCapacity.Available = &types.Info{
				CPU:    proto.ResourceCapacity.Available.Cpu,
				Memory: proto.ResourceCapacity.Available.Memory,
				GPU:    proto.ResourceCapacity.Available.Gpu,
			}
		}
	}

	// 转换资源标签
	if proto.ResourceTags != nil {
		node.ResourceTags = &ResourceTags{
			CPU:    proto.ResourceTags.Cpu,
			GPU:    proto.ResourceTags.Gpu,
			Memory: proto.ResourceTags.Memory,
			Camera: proto.ResourceTags.Camera,
		}
	}

	return node
}

// convertNodeStatusToProto 转换节点状态到 proto
func convertNodeStatusToProto(status NodeStatus) discoverypb.NodeStatus {
	switch status {
	case NodeStatusOnline:
		return discoverypb.NodeStatus_NODE_STATUS_ONLINE
	case NodeStatusOffline:
		return discoverypb.NodeStatus_NODE_STATUS_OFFLINE
	case NodeStatusError:
		return discoverypb.NodeStatus_NODE_STATUS_ERROR
	default:
		return discoverypb.NodeStatus_NODE_STATUS_UNKNOWN
	}
}

// convertProtoToNodeStatus 转换 proto 到节点状态
func convertProtoToNodeStatus(status discoverypb.NodeStatus) NodeStatus {
	switch status {
	case discoverypb.NodeStatus_NODE_STATUS_ONLINE:
		return NodeStatusOnline
	case discoverypb.NodeStatus_NODE_STATUS_OFFLINE:
		return NodeStatusOffline
	case discoverypb.NodeStatus_NODE_STATUS_ERROR:
		return NodeStatusError
	default:
		return NodeStatusUnknown
	}
}

// GetKnownNodes 获取所有已知节点
func (s *service) GetKnownNodes() []*PeerNode {
	return s.manager.GetKnownNodes()
}

// GetLocalNode 获取本地节点信息
func (s *service) GetLocalNode() *PeerNode {
	return s.manager.GetLocalNode()
}

// UpdateLocalNode 更新本地节点信息
// resourceTags 可以是 *ResourceTags 或 *registrypb.ResourceTags（来自 global registry）
func (s *service) UpdateLocalNode(resourceCapacity *types.Capacity, resourceTags interface{}) {
	var tags *ResourceTags

	// 处理不同类型的 resourceTags
	switch v := resourceTags.(type) {
	case *ResourceTags:
		tags = v
	case *registrypb.ResourceTags:
		// 从 registrypb.ResourceTags 转换
		if v != nil {
			tags = &ResourceTags{
				CPU:    v.Cpu,
				GPU:    v.Gpu,
				Memory: v.Memory,
				Camera: v.Camera,
			}
		}
	case nil:
		tags = nil
	default:
		logrus.Debugf("Unknown resourceTags type: %T, skipping tags update", v)
		tags = nil
	}

	s.manager.UpdateLocalNode(resourceCapacity, tags)
}
