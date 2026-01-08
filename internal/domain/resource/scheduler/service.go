package scheduler

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/provider"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	"github.com/9triver/iarnet/internal/proto/common"
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

	// ProposeLocalSchedule 在当前节点执行一次“只调度不部署”的本地调度
	// 调用方只会拿到当前节点选中的本地 provider 及其可用资源信息，不会触发实际部署。
	ProposeLocalSchedule(ctx context.Context, req *types.Info) (*LocalScheduleResult, error)

	// CommitLocalSchedule 根据之前 ProposeLocalSchedule 返回的调度结果，在当前节点上确认部署
	// 这是一个两阶段提交的第二阶段：第一阶段只做调度，第二阶段真正执行部署
	CommitLocalSchedule(ctx context.Context, req *CommitLocalScheduleRequest) (*DeployResponse, error)

	// ProposeRemoteSchedule 在远程节点执行一次“只调度不部署”的调度
	// 返回远程节点选中的 provider 及其可用资源信息
	ProposeRemoteSchedule(ctx context.Context, targetNodeID string, targetAddress string, runtimeEnv types.RuntimeEnv, req *types.Info) (*LocalScheduleResult, error)

	// CommitRemoteSchedule 在远程节点上确认部署（两阶段提交的第二阶段）
	CommitRemoteSchedule(ctx context.Context, targetNodeID string, targetAddress string, req *CommitLocalScheduleRequest) (*DeployResponse, error)

	// ListProviders 获取当前节点的所有 Provider 列表及其资源信息
	// 用于无自主调度能力的场景：调用方可以根据 Provider 列表在本地进行调度决策
	ListProviders(ctx context.Context, includeResources bool) (*ProviderListResponse, error)

	// ListRemoteProviders 获取远程节点的所有 Provider 列表
	ListRemoteProviders(ctx context.Context, targetNodeID string, targetAddress string, includeResources bool) (*ProviderListResponse, error)

	// UndeployComponent 移除 component（本地调用）
	UndeployComponent(ctx context.Context, componentID string, providerID string) error

	// UndeployRemoteComponent 在远程节点上移除 component
	UndeployRemoteComponent(ctx context.Context, targetNodeID string, targetAddress string, componentID string, providerID string) error
}

// DeployRequest 部署请求
type DeployRequest struct {
	RuntimeEnv            types.RuntimeEnv
	ResourceRequest       *types.Info
	TargetNodeID          string // 目标节点 ID，为空则在本地部署
	TargetAddress         string // 目标节点地址（可选）
	UpstreamZMQAddress    string
	UpstreamStoreAddress  string
	UpstreamLoggerAddress string
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

// LocalScheduleResult 表示在当前节点内做出的调度结果（不触发实际部署）
type LocalScheduleResult struct {
	// 当前节点信息
	NodeID   string
	NodeName string

	// 被选中的本地 Provider ID
	ProviderID string

	// Provider 当前可用资源视图
	Available *types.Info

	// 语言信息（用于后续部署时验证 provider 支持）
	Language common.Language
}

// CommitLocalScheduleRequest 确认部署请求
type CommitLocalScheduleRequest struct {
	RuntimeEnv            types.RuntimeEnv
	ResourceRequest       *types.Info
	ProviderID            string // 从 ProposeLocalSchedule 返回的调度结果中获取
	UpstreamZMQAddress    string
	UpstreamStoreAddress  string
	UpstreamLoggerAddress string
}

// ProviderInfo Provider 信息
type ProviderInfo struct {
	ProviderID    string
	ProviderName  string
	ProviderType  string
	Status        string
	Available     *types.Info
	TotalCapacity *types.Info
	Used          *types.Info
	ResourceTags  *types.ResourceTags
}

// ProviderListResponse Provider 列表响应
type ProviderListResponse struct {
	Success   bool
	Error     string
	NodeID    string
	NodeName  string
	Providers []*ProviderInfo
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

		// 只做本地调度、不做部署（由 resource.Manager 提供）
		ScheduleLocalProvider(ctx context.Context, resourceRequest *types.Info) (*LocalScheduleResult, error)

		// 在指定 provider 上部署（用于两阶段提交的第二阶段）
		DeployComponentOnProvider(ctx context.Context, runtimeEnv types.RuntimeEnv, resourceRequest *types.Info, providerID string) (*component.Component, error)

		// 获取所有 Provider 列表（用于无自主调度能力场景）
		ListAllProviders(ctx context.Context, includeResources bool) ([]*ProviderInfo, error)

		// 移除 component（用于 Undeploy）
		UndeployComponent(ctx context.Context, componentID string, providerID string) error
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

		// 只做本地调度、不做部署（由 resource.Manager 提供）
		ScheduleLocalProvider(ctx context.Context, resourceRequest *types.Info) (*LocalScheduleResult, error)

		// 在指定 provider 上部署（用于两阶段提交的第二阶段）
		DeployComponentOnProvider(ctx context.Context, runtimeEnv types.RuntimeEnv, resourceRequest *types.Info, providerID string) (*component.Component, error)

		// 获取所有 Provider 列表（用于无自主调度能力场景）
		ListAllProviders(ctx context.Context, includeResources bool) ([]*ProviderInfo, error)

		// 移除 component（用于 Undeploy）
		UndeployComponent(ctx context.Context, componentID string, providerID string) error
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
	localCtx := ctx
	if req.UpstreamZMQAddress != "" || req.UpstreamStoreAddress != "" || req.UpstreamLoggerAddress != "" {
		override := &provider.DeploymentEnvOverride{
			ZMQAddress:    req.UpstreamZMQAddress,
			StoreAddress:  req.UpstreamStoreAddress,
			LoggerAddress: req.UpstreamLoggerAddress,
		}
		localCtx = provider.WithDeploymentEnvOverride(ctx, override)
	}

	comp, err := s.localResourceManager.DeployComponent(localCtx, req.RuntimeEnv, req.ResourceRequest)
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

		if targetNode.SchedulerAddress != "" {
			targetAddress = targetNode.SchedulerAddress
		} else {
			targetAddress = targetNode.Address
		}
	}

	if targetAddress == "" {
		return &DeployResponse{
			Success: false,
			Error:   "target address is empty",
		}, nil
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
			Tags:   req.ResourceRequest.Tags,
		},
		TargetNodeId:          "", // 远程节点本地部署，不需要再指定目标
		UpstreamZmqAddress:    req.UpstreamZMQAddress,
		UpstreamStoreAddress:  req.UpstreamStoreAddress,
		UpstreamLoggerAddress: req.UpstreamLoggerAddress,
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

// ProposeLocalSchedule 在当前节点执行一次"只调度不部署"的本地调度
// 注意：此方法需要语言信息，但当前接口没有语言参数
// 如果需要根据语言筛选，需要在请求中添加语言字段或从 context 获取
func (s *service) ProposeLocalSchedule(ctx context.Context, req *types.Info) (*LocalScheduleResult, error) {
	if req == nil {
		return nil, fmt.Errorf("resource request is required")
	}

	// 直接复用本地资源管理器中的调度逻辑（不触发部署）
	// 语言信息会从 context 中获取（如果存在）
	result, err := s.localResourceManager.ScheduleLocalProvider(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule local provider: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("local schedule result is nil")
	}

	return &LocalScheduleResult{
		NodeID:     s.localResourceManager.GetNodeID(),
		NodeName:   s.localResourceManager.GetNodeName(),
		ProviderID: result.ProviderID,
		Available:  result.Available,
		Language:   result.Language,
	}, nil
}

// CommitLocalSchedule 根据之前 ProposeLocalSchedule 返回的调度结果，在当前节点上确认部署
func (s *service) CommitLocalSchedule(ctx context.Context, req *CommitLocalScheduleRequest) (*DeployResponse, error) {
	if req == nil {
		return &DeployResponse{
			Success: false,
			Error:   "request is required",
		}, nil
	}

	if req.ResourceRequest == nil {
		return &DeployResponse{
			Success: false,
			Error:   "resource request is required",
		}, nil
	}

	if req.ProviderID == "" {
		return &DeployResponse{
			Success: false,
			Error:   "provider ID is required",
		}, nil
	}

	// 设置上游地址（如果提供）
	localCtx := ctx
	if req.UpstreamZMQAddress != "" || req.UpstreamStoreAddress != "" || req.UpstreamLoggerAddress != "" {
		override := &provider.DeploymentEnvOverride{
			ZMQAddress:    req.UpstreamZMQAddress,
			StoreAddress:  req.UpstreamStoreAddress,
			LoggerAddress: req.UpstreamLoggerAddress,
		}
		localCtx = provider.WithDeploymentEnvOverride(ctx, override)
	}

	// 将 RuntimeEnv 转换为 Language 并添加到 context（虽然 DeployComponentOnProvider 会再次转换，但保持一致性）
	language := types.RuntimeEnvToLanguage(req.RuntimeEnv)
	if language != common.Language_LANG_UNKNOWN {
		localCtx = provider.WithLanguage(localCtx, language)
	}

	// 在指定的 provider 上部署
	comp, err := s.localResourceManager.DeployComponentOnProvider(localCtx, req.RuntimeEnv, req.ResourceRequest, req.ProviderID)
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

// ProposeRemoteSchedule 在远程节点执行一次“只调度不部署”的调度
func (s *service) ProposeRemoteSchedule(ctx context.Context, targetNodeID string, targetAddress string, runtimeEnv types.RuntimeEnv, req *types.Info) (*LocalScheduleResult, error) {
	if req == nil {
		return nil, fmt.Errorf("resource request is required")
	}

	// 将 RuntimeEnv 转换为 Language 并添加到 context，以便远程节点可以根据语言筛选 provider
	language := types.RuntimeEnvToLanguage(runtimeEnv)
	if language != common.Language_LANG_UNKNOWN {
		ctx = provider.WithLanguage(ctx, language)
	}

	// 获取目标节点地址
	if targetAddress == "" {
		if s.discoveryService == nil {
			return nil, fmt.Errorf("discovery service is not available")
		}

		knownNodes := s.discoveryService.GetKnownNodes()
		var targetNode *discovery.PeerNode
		for _, node := range knownNodes {
			if node.NodeID == targetNodeID {
				targetNode = node
				break
			}
		}

		if targetNode == nil {
			return nil, fmt.Errorf("target node %s not found", targetNodeID)
		}

		if targetNode.SchedulerAddress != "" {
			targetAddress = targetNode.SchedulerAddress
		} else {
			targetAddress = targetNode.Address
		}
	}

	if targetAddress == "" {
		return nil, fmt.Errorf("target address is empty")
	}

	// 连接到远程节点的 scheduler RPC 服务
	conn, err := grpc.NewClient(targetAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to target node: %w", err)
	}
	defer conn.Close()

	// 创建客户端并调用远程调度
	client := schedulerpb.NewSchedulerServiceClient(conn)

	protoReq := &schedulerpb.ProposeLocalScheduleRequest{
		ResourceRequest: &resourcepb.Info{
			Cpu:    req.CPU,
			Memory: req.Memory,
			Gpu:    req.GPU,
			Tags:   req.Tags,
		},
	}

	protoResp, err := client.ProposeLocalSchedule(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("failed to propose schedule on remote node: %w", err)
	}

	if !protoResp.Success {
		return nil, fmt.Errorf("remote schedule proposal rejected: %s", protoResp.Error)
	}

	// 从 context 中获取语言信息（如果存在）
	if lang, ok := provider.GetLanguageFromContext(ctx); ok {
		language = lang
	}

	return &LocalScheduleResult{
		NodeID:     protoResp.NodeId,
		NodeName:   protoResp.NodeName,
		ProviderID: protoResp.ProviderId,
		Available: func() *types.Info {
			if protoResp.Available != nil {
				return &types.Info{
					CPU:    protoResp.Available.Cpu,
					Memory: protoResp.Available.Memory,
					GPU:    protoResp.Available.Gpu,
					Tags:   append([]string(nil), protoResp.Available.Tags...),
				}
			}
			return nil
		}(),
		Language: language,
	}, nil
}

// CommitRemoteSchedule 在远程节点上确认部署（两阶段提交的第二阶段）
func (s *service) CommitRemoteSchedule(ctx context.Context, targetNodeID string, targetAddress string, req *CommitLocalScheduleRequest) (*DeployResponse, error) {
	if req == nil {
		return &DeployResponse{
			Success: false,
			Error:   "request is required",
		}, nil
	}

	// 获取目标节点地址
	if targetAddress == "" {
		if s.discoveryService == nil {
			return &DeployResponse{
				Success: false,
				Error:   "discovery service is not available",
			}, nil
		}

		knownNodes := s.discoveryService.GetKnownNodes()
		var targetNode *discovery.PeerNode
		for _, node := range knownNodes {
			if node.NodeID == targetNodeID {
				targetNode = node
				break
			}
		}

		if targetNode == nil {
			return &DeployResponse{
				Success: false,
				Error:   fmt.Sprintf("target node %s not found", targetNodeID),
			}, nil
		}

		if targetNode.SchedulerAddress != "" {
			targetAddress = targetNode.SchedulerAddress
		} else {
			targetAddress = targetNode.Address
		}
	}

	if targetAddress == "" {
		return &DeployResponse{
			Success: false,
			Error:   "target address is empty",
		}, nil
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

	// 创建客户端并调用远程确认部署
	client := schedulerpb.NewSchedulerServiceClient(conn)

	protoReq := &schedulerpb.CommitLocalScheduleRequest{
		RuntimeEnv: string(req.RuntimeEnv),
		ResourceRequest: &resourcepb.Info{
			Cpu:    req.ResourceRequest.CPU,
			Memory: req.ResourceRequest.Memory,
			Gpu:    req.ResourceRequest.GPU,
			Tags:   req.ResourceRequest.Tags,
		},
		ProviderId:            req.ProviderID,
		UpstreamZmqAddress:    req.UpstreamZMQAddress,
		UpstreamStoreAddress:  req.UpstreamStoreAddress,
		UpstreamLoggerAddress: req.UpstreamLoggerAddress,
	}

	protoResp, err := client.CommitLocalSchedule(ctx, protoReq)
	if err != nil {
		return &DeployResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to commit schedule on remote node: %v", err),
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

// ListProviders 获取当前节点的所有 Provider 列表及其资源信息
func (s *service) ListProviders(ctx context.Context, includeResources bool) (*ProviderListResponse, error) {
	providers, err := s.localResourceManager.ListAllProviders(ctx, includeResources)
	if err != nil {
		return &ProviderListResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ProviderListResponse{
		Success:   true,
		NodeID:    s.localResourceManager.GetNodeID(),
		NodeName:  s.localResourceManager.GetNodeName(),
		Providers: providers,
	}, nil
}

// ListRemoteProviders 获取远程节点的所有 Provider 列表
func (s *service) ListRemoteProviders(ctx context.Context, targetNodeID string, targetAddress string, includeResources bool) (*ProviderListResponse, error) {
	// 获取目标节点地址
	if targetAddress == "" {
		if s.discoveryService == nil {
			return &ProviderListResponse{
				Success: false,
				Error:   "discovery service is not available",
			}, nil
		}

		knownNodes := s.discoveryService.GetKnownNodes()
		var targetNode *discovery.PeerNode
		for _, node := range knownNodes {
			if node.NodeID == targetNodeID {
				targetNode = node
				break
			}
		}

		if targetNode == nil {
			return &ProviderListResponse{
				Success: false,
				Error:   fmt.Sprintf("target node %s not found", targetNodeID),
			}, nil
		}

		if targetNode.SchedulerAddress != "" {
			targetAddress = targetNode.SchedulerAddress
		} else {
			targetAddress = targetNode.Address
		}
	}

	if targetAddress == "" {
		return &ProviderListResponse{
			Success: false,
			Error:   "target address is empty",
		}, nil
	}

	// 连接到远程节点的 scheduler RPC 服务
	conn, err := grpc.NewClient(targetAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &ProviderListResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to connect to target node: %v", err),
		}, nil
	}
	defer conn.Close()

	// 创建客户端并调用远程 ListProviders
	client := schedulerpb.NewSchedulerServiceClient(conn)

	protoReq := &schedulerpb.ListProvidersRequest{
		IncludeResources: includeResources,
	}

	protoResp, err := client.ListProviders(ctx, protoReq)
	if err != nil {
		return &ProviderListResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to list providers on remote node: %v", err),
		}, nil
	}

	if !protoResp.Success {
		return &ProviderListResponse{
			Success: false,
			Error:   protoResp.Error,
		}, nil
	}

	// 转换响应
	providers := make([]*ProviderInfo, 0, len(protoResp.Providers))
	for _, p := range protoResp.Providers {
		providerInfo := &ProviderInfo{
			ProviderID:   p.ProviderId,
			ProviderName: p.ProviderName,
			ProviderType: p.ProviderType,
			Status:       p.Status,
		}

		if p.Available != nil {
			providerInfo.Available = &types.Info{
				CPU:    p.Available.Cpu,
				Memory: p.Available.Memory,
				GPU:    p.Available.Gpu,
				Tags:   append([]string(nil), p.Available.Tags...),
			}
		}

		if p.TotalCapacity != nil {
			providerInfo.TotalCapacity = &types.Info{
				CPU:    p.TotalCapacity.Cpu,
				Memory: p.TotalCapacity.Memory,
				GPU:    p.TotalCapacity.Gpu,
				Tags:   append([]string(nil), p.TotalCapacity.Tags...),
			}
		}

		if p.Used != nil {
			providerInfo.Used = &types.Info{
				CPU:    p.Used.Cpu,
				Memory: p.Used.Memory,
				GPU:    p.Used.Gpu,
				Tags:   append([]string(nil), p.Used.Tags...),
			}
		}

		if p.ResourceTags != nil {
			providerInfo.ResourceTags = &types.ResourceTags{
				CPU:    p.ResourceTags.Cpu,
				GPU:    p.ResourceTags.Gpu,
				Memory: p.ResourceTags.Memory,
				Camera: p.ResourceTags.Camera,
			}
		}

		providers = append(providers, providerInfo)
	}

	return &ProviderListResponse{
		Success:   true,
		NodeID:    protoResp.NodeId,
		NodeName:  protoResp.NodeName,
		Providers: providers,
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
			Tags:   append([]string(nil), info.ResourceUsage.Tags...),
		}
	} else {
		usage = &types.Info{}
	}

	comp := component.NewComponent(info.ComponentId, info.Image, usage)
	return comp
}

// UndeployComponent 移除 component（本地调用）
func (s *service) UndeployComponent(ctx context.Context, componentID string, providerID string) error {
	return s.localResourceManager.UndeployComponent(ctx, componentID, providerID)
}

// UndeployRemoteComponent 在远程节点上移除 component
func (s *service) UndeployRemoteComponent(ctx context.Context, targetNodeID string, targetAddress string, componentID string, providerID string) error {
	if targetAddress == "" {
		return fmt.Errorf("target address is required for remote undeploy")
	}

	// 连接到远程节点的 scheduler 服务
	conn, err := grpc.NewClient(targetAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to remote scheduler at %s: %w", targetAddress, err)
	}
	defer conn.Close()

	client := schedulerpb.NewSchedulerServiceClient(conn)
	req := &schedulerpb.UndeployComponentRequest{
		ComponentId: componentID,
		ProviderId:  providerID,
	}

	resp, err := client.UndeployComponent(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to call remote scheduler UndeployComponent: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("remote scheduler UndeployComponent failed: %s", resp.Error)
	}

	return nil
}
