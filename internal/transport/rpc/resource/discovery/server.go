package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	discoverypb "github.com/9triver/iarnet/internal/proto/resource/discovery"
	"github.com/sirupsen/logrus"
)

type Server struct {
	discoverypb.UnimplementedDiscoveryServiceServer
	service discovery.Service
	manager *discovery.NodeDiscoveryManager
}

func NewServer(service discovery.Service, manager *discovery.NodeDiscoveryManager) *Server {
	return &Server{
		service: service,
		manager: manager,
	}
}

// GossipNodeInfo 交换节点信息（gossip 协议）
func (s *Server) GossipNodeInfo(ctx context.Context, req *discoverypb.NodeInfoGossipMessage) (*discoverypb.NodeInfoGossipResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// 检查消息是否已处理（去重）
	if s.manager.IsMessageProcessed(req.MessageId) {
		logrus.Debugf("Message %s already processed, ignoring", req.MessageId)
		// 仍然返回响应，但不处理消息
		return &discoverypb.NodeInfoGossipResponse{
			MessageId: req.MessageId,
			Timestamp: time.Now().UnixNano(),
			Nodes:     []*discoverypb.PeerNodeInfo{},
		}, nil
	}

	// 标记消息已处理
	s.manager.MarkMessageProcessed(req.MessageId)

	// 检查 TTL
	if req.Ttl <= 0 {
		logrus.Debugf("Message %s TTL expired, ignoring", req.MessageId)
		return &discoverypb.NodeInfoGossipResponse{
			MessageId: req.MessageId,
			Timestamp: time.Now().UnixNano(),
			Nodes:     []*discoverypb.PeerNodeInfo{},
		}, nil
	}

	// 只处理同域节点
	localNode := s.manager.GetLocalNode()
	if req.SenderDomainId != localNode.DomainID {
		logrus.Debugf("Ignoring gossip message from different domain: %s (local: %s)", req.SenderDomainId, localNode.DomainID)
		return &discoverypb.NodeInfoGossipResponse{
			MessageId: req.MessageId,
			Timestamp: time.Now().UnixNano(),
			Nodes:     []*discoverypb.PeerNodeInfo{},
		}, nil
	}

	// 处理接收到的节点信息
	for _, nodeInfo := range req.Nodes {
		if nodeInfo == nil {
			continue
		}

		// 转换为领域模型
		peerNode := convertProtoToPeerNode(nodeInfo)
		if peerNode != nil {
			s.manager.ProcessNodeInfo(peerNode, req.SenderAddress)
		}
	}

	// 构建响应：返回本地节点信息和已知节点信息
	responseNodes := s.buildNodeInfoListForGossip(req.Ttl-1, req.MaxHops-1)

	return &discoverypb.NodeInfoGossipResponse{
		MessageId: req.MessageId,
		Timestamp: time.Now().UnixNano(),
		Nodes:     responseNodes,
	}, nil
}

// QueryResources 查询资源（主动查询）
func (s *Server) QueryResources(ctx context.Context, req *discoverypb.ResourceQueryRequest) (*discoverypb.ResourceQueryResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// 检查 TTL
	if req.Ttl <= 0 || req.CurrentHops >= req.MaxHops {
		return &discoverypb.ResourceQueryResponse{
			QueryId:    req.QueryId,
			Timestamp:  time.Now().UnixNano(),
			IsFinal:    true,
			AvailableNodes: []*discoverypb.PeerNodeInfo{},
		}, nil
	}

	// 只处理同域查询
	localNode := s.manager.GetLocalNode()
	if req.RequesterDomainId != localNode.DomainID {
		logrus.Debugf("Ignoring resource query from different domain: %s (local: %s)", req.RequesterDomainId, localNode.DomainID)
		return &discoverypb.ResourceQueryResponse{
			QueryId:    req.QueryId,
			Timestamp:  time.Now().UnixNano(),
			IsFinal:    true,
			AvailableNodes: []*discoverypb.PeerNodeInfo{},
		}, nil
	}

	// 转换资源请求
	resourceRequest := &discovery.ResourceRequest{
		CPU:    req.ResourceRequest.Cpu,
		Memory: req.ResourceRequest.Memory,
		GPU:    req.ResourceRequest.Gpu,
	}

	var requiredTags *discovery.ResourceTags
	if req.RequiredTags != nil {
		requiredTags = &discovery.ResourceTags{
			CPU:    req.RequiredTags.Cpu,
			GPU:    req.RequiredTags.Gpu,
			Memory: req.RequiredTags.Memory,
			Camera: req.RequiredTags.Camera,
		}
	}

	// 查找可用节点
	availableNodes := s.manager.FindAvailableNodes(resourceRequest, requiredTags)

	// 转换为 proto 格式
	protoNodes := make([]*discoverypb.PeerNodeInfo, 0, len(availableNodes))
	for _, node := range availableNodes {
		protoNodes = append(protoNodes, convertPeerNodeToProto(node, int(req.Ttl-1)))
	}

	return &discoverypb.ResourceQueryResponse{
		QueryId:         req.QueryId,
		ResponderNodeId: localNode.NodeID,
		ResponderAddress: localNode.Address,
		AvailableNodes:  protoNodes,
		Timestamp:       time.Now().UnixNano(),
		IsFinal:         req.Ttl <= 1 || req.CurrentHops >= req.MaxHops-1,
	}, nil
}

// ExchangePeerList 交换 peer 列表（用于节点发现）
func (s *Server) ExchangePeerList(ctx context.Context, req *discoverypb.PeerListExchangeRequest) (*discoverypb.PeerListExchangeResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// 只处理同域请求
	localNode := s.manager.GetLocalNode()
	if req.RequesterDomainId != localNode.DomainID {
		logrus.Debugf("Ignoring peer list exchange from different domain: %s (local: %s)", req.RequesterDomainId, localNode.DomainID)
		return &discoverypb.PeerListExchangeResponse{
			KnownPeers: []string{},
			Timestamp:  time.Now().UnixNano(),
		}, nil
	}

	// 添加请求方的 peer 地址（如果不在列表中）
	if req.RequesterAddress != "" {
		s.manager.AddPeer(req.RequesterAddress)
	}

	// 添加请求方提供的 peer 地址
	for _, peerAddr := range req.KnownPeers {
		if peerAddr != "" && peerAddr != localNode.Address {
			s.manager.AddPeer(peerAddr)
		}
	}

	// 返回本地已知的 peer 列表
	localPeers := s.manager.GetPeerAddresses()

	return &discoverypb.PeerListExchangeResponse{
		KnownPeers: localPeers,
		Timestamp:  time.Now().UnixNano(),
	}, nil
}

// GetLocalNodeInfo 获取本地节点信息
func (s *Server) GetLocalNodeInfo(ctx context.Context, req *discoverypb.GetLocalNodeInfoRequest) (*discoverypb.GetLocalNodeInfoResponse, error) {
	localNode := s.manager.GetLocalNode()
	if localNode == nil {
		return nil, fmt.Errorf("local node info not available")
	}

	protoNode := convertPeerNodeToProto(localNode, 0)

	return &discoverypb.GetLocalNodeInfoResponse{
		NodeInfo: protoNode,
	}, nil
}

// buildNodeInfoListForGossip 构建用于 gossip 的节点信息列表
func (s *Server) buildNodeInfoListForGossip(ttl int32, maxHops int32) []*discoverypb.PeerNodeInfo {
	nodes := s.manager.GetNodesForGossip()
	protoNodes := make([]*discoverypb.PeerNodeInfo, 0, len(nodes))

	for _, node := range nodes {
		protoNodes = append(protoNodes, convertPeerNodeToProto(node, int(ttl)))
	}

	return protoNodes
}

// convertPeerNodeToProto 将领域模型转换为 proto 消息
func convertPeerNodeToProto(node *discovery.PeerNode, gossipCount int) *discoverypb.PeerNodeInfo {
	if node == nil {
		return nil
	}

	protoNode := &discoverypb.PeerNodeInfo{
		NodeId:      node.NodeID,
		NodeName:    node.NodeName,
		Address:     node.Address,
		DomainId:    node.DomainID,
		Status:      convertNodeStatusToProto(node.Status),
		LastSeen:    node.LastSeen.UnixNano(),
		LastUpdated: node.LastUpdated.UnixNano(),
		Version:     node.Version,
		GossipCount: int32(gossipCount),
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

// convertProtoToPeerNode 将 proto 消息转换为领域模型
func convertProtoToPeerNode(proto *discoverypb.PeerNodeInfo) *discovery.PeerNode {
	if proto == nil {
		return nil
	}

	node := &discovery.PeerNode{
		NodeID:      proto.NodeId,
		NodeName:    proto.NodeName,
		Address:     proto.Address,
		DomainID:    proto.DomainId,
		Status:      convertProtoToNodeStatus(proto.Status),
		LastSeen:    time.Unix(0, proto.LastSeen),
		LastUpdated: time.Unix(0, proto.LastUpdated),
		Version:     proto.Version,
		GossipCount: int(proto.GossipCount),
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
		node.ResourceTags = &discovery.ResourceTags{
			CPU:    proto.ResourceTags.Cpu,
			GPU:    proto.ResourceTags.Gpu,
			Memory: proto.ResourceTags.Memory,
			Camera: proto.ResourceTags.Camera,
		}
	}

	return node
}

// convertNodeStatusToProto 转换节点状态到 proto
func convertNodeStatusToProto(status discovery.NodeStatus) discoverypb.NodeStatus {
	switch status {
	case discovery.NodeStatusOnline:
		return discoverypb.NodeStatus_NODE_STATUS_ONLINE
	case discovery.NodeStatusOffline:
		return discoverypb.NodeStatus_NODE_STATUS_OFFLINE
	case discovery.NodeStatusError:
		return discoverypb.NodeStatus_NODE_STATUS_ERROR
	default:
		return discoverypb.NodeStatus_NODE_STATUS_UNKNOWN
	}
}

// convertProtoToNodeStatus 转换 proto 到节点状态
func convertProtoToNodeStatus(status discoverypb.NodeStatus) discovery.NodeStatus {
	switch status {
	case discoverypb.NodeStatus_NODE_STATUS_ONLINE:
		return discovery.NodeStatusOnline
	case discoverypb.NodeStatus_NODE_STATUS_OFFLINE:
		return discovery.NodeStatusOffline
	case discoverypb.NodeStatus_NODE_STATUS_ERROR:
		return discovery.NodeStatusError
	default:
		return discovery.NodeStatusUnknown
	}
}

