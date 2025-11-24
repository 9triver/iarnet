package scheduler

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	schedulerpb "github.com/9triver/iarnet/internal/proto/resource/scheduler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Service 提供远程调度服务
type Service interface {
	// DeployComponent 部署 component，支持本地和远程部署
	DeployComponent(ctx context.Context, req *DeployRequest) (*DeployResponse, error)

	// GetDeploymentStatus 获取部署状态
	GetDeploymentStatus(ctx context.Context, componentID string, nodeID string) (*DeploymentStatus, error)
}

// DeployRequest 部署请求
type DeployRequest struct {
	RuntimeEnv      types.RuntimeEnv
	ResourceRequest *types.Info
	TargetNodeID    string // 目标节点 ID，为空则在本地部署
	TargetAddress   string // 目标节点地址（可选）
}

// DeployResponse 部署响应
type DeployResponse struct {
	Success    bool
	Error      string
	Component  *component.Component
	NodeID     string
	NodeName   string
	ProviderID string
}

// DeploymentStatus 部署状态
type DeploymentStatus struct {
	Success   bool
	Error     string
	Status    ComponentStatus
	Component *component.Component
}

// ComponentStatus Component 状态
type ComponentStatus int32

const (
	ComponentStatusUnknown   ComponentStatus = 0
	ComponentStatusDeploying ComponentStatus = 1
	ComponentStatusRunning   ComponentStatus = 2
	ComponentStatusStopped   ComponentStatus = 3
	ComponentStatusError     ComponentStatus = 4
)

// service 实现 Service 接口
type service struct {
	// 本地资源管理器（用于本地部署）
	localResourceManager interface {
		DeployComponent(ctx context.Context, runtimeEnv types.RuntimeEnv, resourceRequest *types.Info) (*component.Component, error)
		GetNodeID() string
		GetNodeName() string
	}

	// Discovery 服务（用于查找远程节点）
	discoveryService discovery.Service
}

// NewService 创建调度服务
func NewService(
	localResourceManager interface {
		DeployComponent(ctx context.Context, runtimeEnv types.RuntimeEnv, resourceRequest *types.Info) (*component.Component, error)
		GetNodeID() string
		GetNodeName() string
	},
	discoveryService discovery.Service,
) Service {
	return &service{
		localResourceManager: localResourceManager,
		discoveryService:     discoveryService,
	}
}

// DeployComponent 部署 component
func (s *service) DeployComponent(ctx context.Context, req *DeployRequest) (*DeployResponse, error) {
	if req == nil {
		return &DeployResponse{
			Success: false,
			Error:   "request is required",
		}, nil
	}

	// 如果没有指定目标节点，在本地部署
	if req.TargetNodeID == "" {
		return s.deployLocally(ctx, req)
	}

	// 远程部署
	return s.deployRemotely(ctx, req)
}

// deployLocally 在本地节点部署
func (s *service) deployLocally(ctx context.Context, req *DeployRequest) (*DeployResponse, error) {
	comp, err := s.localResourceManager.DeployComponent(ctx, req.RuntimeEnv, req.ResourceRequest)
	if err != nil {
		return &DeployResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &DeployResponse{
		Success:    true,
		Component:  comp,
		NodeID:     s.localResourceManager.GetNodeID(),
		NodeName:   s.localResourceManager.GetNodeName(),
		ProviderID: comp.GetProviderID(),
	}, nil
}

// deployRemotely 在远程节点部署
func (s *service) deployRemotely(ctx context.Context, req *DeployRequest) (*DeployResponse, error) {
	// 获取目标节点地址
	targetAddress := req.TargetAddress
	if targetAddress == "" {
		// 从 discovery service 获取节点地址
		if s.discoveryService == nil {
			return &DeployResponse{
				Success: false,
				Error:   "discovery service is not available",
			}, nil
		}

		// 查找目标节点（通过已知节点列表）
		knownNodes := s.discoveryService.GetKnownNodes()
		var targetNode *discovery.PeerNode
		for _, node := range knownNodes {
			if node.NodeID == req.TargetNodeID {
				targetNode = node
				break
			}
		}

		if targetNode == nil {
			return &DeployResponse{
				Success: false,
				Error:   fmt.Sprintf("target node %s not found", req.TargetNodeID),
			}, nil
		}

		targetAddress = targetNode.Address
	}

	// 连接到远程节点的 scheduler RPC 服务
	conn, err := grpc.NewClient(targetAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &DeployResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to connect to target node: %v", err),
		}, nil
	}
	defer conn.Close()

	// 创建客户端并调用远程部署
	client := schedulerpb.NewSchedulerServiceClient(conn)

	protoReq := &schedulerpb.DeployComponentRequest{
		RuntimeEnv: string(req.RuntimeEnv),
		ResourceRequest: &resourcepb.Info{
			Cpu:    req.ResourceRequest.CPU,
			Memory: req.ResourceRequest.Memory,
			Gpu:    req.ResourceRequest.GPU,
		},
		TargetNodeId: "", // 远程节点本地部署，不需要再指定目标
	}

	protoResp, err := client.DeployComponent(ctx, protoReq)
	if err != nil {
		return &DeployResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to deploy on remote node: %v", err),
		}, nil
	}

	if !protoResp.Success {
		return &DeployResponse{
			Success: false,
			Error:   protoResp.Error,
		}, nil
	}

	comp := convertComponentInfoFromProto(protoResp.Component)
	if comp != nil {
		comp.SetProviderID(protoResp.ProviderId)
	}

	return &DeployResponse{
		Success:    true,
		Component:  comp,
		NodeID:     protoResp.NodeId,
		NodeName:   protoResp.NodeName,
		ProviderID: protoResp.ProviderId,
	}, nil
}

// GetDeploymentStatus 获取部署状态
func (s *service) GetDeploymentStatus(ctx context.Context, componentID string, nodeID string) (*DeploymentStatus, error) {
	// TODO: 实现获取部署状态的逻辑
	return &DeploymentStatus{
		Success: false,
		Error:   "not implemented",
	}, nil
}

func convertComponentInfoFromProto(info *schedulerpb.ComponentInfo) *component.Component {
	if info == nil {
		return nil
	}

	var usage *types.Info
	if info.ResourceUsage != nil {
		usage = &types.Info{
			CPU:    info.ResourceUsage.Cpu,
			Memory: info.ResourceUsage.Memory,
			GPU:    info.ResourceUsage.Gpu,
		}
	} else {
		usage = &types.Info{}
	}

	comp := component.NewComponent(info.ComponentId, info.Image, usage)
	return comp
}
