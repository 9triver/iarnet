package scheduler

import (
	"context"

	"github.com/9triver/iarnet/internal/domain/resource/scheduler"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	schedulerpb "github.com/9triver/iarnet/internal/proto/resource/scheduler"
	"github.com/sirupsen/logrus"
)

// Server 实现 SchedulerService gRPC 服务
type Server struct {
	schedulerpb.UnimplementedSchedulerServiceServer
	service scheduler.Service
}

// NewServer 创建调度服务 RPC 服务器
func NewServer(service scheduler.Service) *Server {
	return &Server{
		service: service,
	}
}

// DeployComponent 部署 component
func (s *Server) DeployComponent(ctx context.Context, req *schedulerpb.DeployComponentRequest) (*schedulerpb.DeployComponentResponse, error) {
	if req == nil {
		return &schedulerpb.DeployComponentResponse{
			Success: false,
			Error:   "request is required",
		}, nil
	}

	// 转换请求
	deployReq := &scheduler.DeployRequest{
		RuntimeEnv: types.RuntimeEnv(req.RuntimeEnv),
		ResourceRequest: &types.Info{
			CPU:    req.ResourceRequest.Cpu,
			Memory: req.ResourceRequest.Memory,
			GPU:    req.ResourceRequest.Gpu,
			Tags:   req.ResourceRequest.Tags,
		},
		TargetNodeID:          req.TargetNodeId,
		TargetAddress:         req.TargetNodeAddress,
		UpstreamZMQAddress:    req.UpstreamZmqAddress,
		UpstreamStoreAddress:  req.UpstreamStoreAddress,
		UpstreamLoggerAddress: req.UpstreamLoggerAddress,
	}

	// 调用服务
	resp, err := s.service.DeployComponent(ctx, deployReq)
	if err != nil {
		logrus.Errorf("Failed to deploy component: %v", err)
		return &schedulerpb.DeployComponentResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// 转换响应
	protoResp := &schedulerpb.DeployComponentResponse{
		Success:    resp.Success,
		Error:      resp.Error,
		NodeId:     resp.NodeID,
		NodeName:   resp.NodeName,
		ProviderId: resp.ProviderID,
	}

	if resp.Component != nil {
		resourceUsage := resp.Component.GetResourceUsage()
		protoResp.Component = &schedulerpb.ComponentInfo{
			ComponentId: resp.Component.GetID(),
			Image:       resp.Component.GetImage(),
			ResourceUsage: &resourcepb.Info{
				Cpu:    resourceUsage.CPU,
				Memory: resourceUsage.Memory,
				Gpu:    resourceUsage.GPU,
				Tags:   resourceUsage.Tags,
			},
			ProviderId: resp.Component.GetProviderID(),
		}
	}

	return protoResp, nil
}

// GetDeploymentStatus 获取部署状态
func (s *Server) GetDeploymentStatus(ctx context.Context, req *schedulerpb.GetDeploymentStatusRequest) (*schedulerpb.GetDeploymentStatusResponse, error) {
	if req == nil {
		return &schedulerpb.GetDeploymentStatusResponse{
			Success: false,
			Error:   "request is required",
		}, nil
	}

	// 调用服务
	status, err := s.service.GetDeploymentStatus(ctx, req.ComponentId, req.NodeId)
	if err != nil {
		logrus.Errorf("Failed to get deployment status: %v", err)
		return &schedulerpb.GetDeploymentStatusResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// 转换响应
	protoResp := &schedulerpb.GetDeploymentStatusResponse{
		Success: status.Success,
		Error:   status.Error,
		Status:  convertComponentStatusToProto(status.Status),
	}

	if status.Component != nil {
		resourceUsage := status.Component.GetResourceUsage()
		protoResp.Component = &schedulerpb.ComponentInfo{
			ComponentId: status.Component.GetID(),
			Image:       status.Component.GetImage(),
			ResourceUsage: &resourcepb.Info{
				Cpu:    resourceUsage.CPU,
				Memory: resourceUsage.Memory,
				Gpu:    resourceUsage.GPU,
				Tags:   resourceUsage.Tags,
			},
			ProviderId: status.Component.GetProviderID(),
		}
	}

	return protoResp, nil
}

// ProposeLocalSchedule 在当前节点执行一次“只调度不部署”的本地调度
func (s *Server) ProposeLocalSchedule(ctx context.Context, req *schedulerpb.ProposeLocalScheduleRequest) (*schedulerpb.ProposeLocalScheduleResponse, error) {
	if req == nil {
		return &schedulerpb.ProposeLocalScheduleResponse{
			Success: false,
			Error:   "request is required",
		}, nil
	}

	if req.ResourceRequest == nil {
		return &schedulerpb.ProposeLocalScheduleResponse{
			Success: false,
			Error:   "resource_request is required",
		}, nil
	}

	// 转成 domain types.Info
	resourceReq := &types.Info{
		CPU:    req.ResourceRequest.Cpu,
		Memory: req.ResourceRequest.Memory,
		GPU:    req.ResourceRequest.Gpu,
		Tags:   req.ResourceRequest.Tags,
	}

	// 调用 domain 调度逻辑（只调度不部署）
	result, err := s.service.ProposeLocalSchedule(ctx, resourceReq)
	if err != nil {
		logrus.Errorf("Failed to propose local schedule: %v", err)
		return &schedulerpb.ProposeLocalScheduleResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	resp := &schedulerpb.ProposeLocalScheduleResponse{
		Success:    true,
		NodeId:     result.NodeID,
		NodeName:   result.NodeName,
		ProviderId: result.ProviderID,
	}

	if result.Available != nil {
		resp.Available = &resourcepb.Info{
			Cpu:    result.Available.CPU,
			Memory: result.Available.Memory,
			Gpu:    result.Available.GPU,
			Tags:   result.Available.Tags,
		}
	}

	return resp, nil
}

// CommitLocalSchedule 根据之前 ProposeLocalSchedule 返回的调度结果，在当前节点上确认部署
func (s *Server) CommitLocalSchedule(ctx context.Context, req *schedulerpb.CommitLocalScheduleRequest) (*schedulerpb.DeployComponentResponse, error) {
	if req == nil {
		return &schedulerpb.DeployComponentResponse{
			Success: false,
			Error:   "request is required",
		}, nil
	}

	if req.ResourceRequest == nil {
		return &schedulerpb.DeployComponentResponse{
			Success: false,
			Error:   "resource_request is required",
		}, nil
	}

	if req.ProviderId == "" {
		return &schedulerpb.DeployComponentResponse{
			Success: false,
			Error:   "provider_id is required",
		}, nil
	}

	// 转换请求
	commitReq := &scheduler.CommitLocalScheduleRequest{
		RuntimeEnv: types.RuntimeEnv(req.RuntimeEnv),
		ResourceRequest: &types.Info{
			CPU:    req.ResourceRequest.Cpu,
			Memory: req.ResourceRequest.Memory,
			GPU:    req.ResourceRequest.Gpu,
			Tags:   req.ResourceRequest.Tags,
		},
		ProviderID:            req.ProviderId,
		UpstreamZMQAddress:    req.UpstreamZmqAddress,
		UpstreamStoreAddress:  req.UpstreamStoreAddress,
		UpstreamLoggerAddress: req.UpstreamLoggerAddress,
	}

	// 调用服务
	resp, err := s.service.CommitLocalSchedule(ctx, commitReq)
	if err != nil {
		logrus.Errorf("Failed to commit local schedule: %v", err)
		return &schedulerpb.DeployComponentResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// 转换响应
	protoResp := &schedulerpb.DeployComponentResponse{
		Success:    resp.Success,
		Error:      resp.Error,
		NodeId:     resp.NodeID,
		NodeName:   resp.NodeName,
		ProviderId: resp.ProviderID,
	}

	if resp.Component != nil {
		resourceUsage := resp.Component.GetResourceUsage()
		protoResp.Component = &schedulerpb.ComponentInfo{
			ComponentId: resp.Component.GetID(),
			Image:       resp.Component.GetImage(),
			ResourceUsage: &resourcepb.Info{
				Cpu:    resourceUsage.CPU,
				Memory: resourceUsage.Memory,
				Gpu:    resourceUsage.GPU,
				Tags:   resourceUsage.Tags,
			},
			ProviderId: resp.Component.GetProviderID(),
		}
	}

	return protoResp, nil
}

// ListProviders 获取当前节点的所有 Provider 列表
func (s *Server) ListProviders(ctx context.Context, req *schedulerpb.ListProvidersRequest) (*schedulerpb.ListProvidersResponse, error) {
	if req == nil {
		return &schedulerpb.ListProvidersResponse{
			Success: false,
			Error:   "request is required",
		}, nil
	}

	includeResources := req.IncludeResources

	// 调用服务
	resp, err := s.service.ListProviders(ctx, includeResources)
	if err != nil {
		logrus.Errorf("Failed to list providers: %v", err)
		return &schedulerpb.ListProvidersResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// 转换响应
	protoResp := &schedulerpb.ListProvidersResponse{
		Success:   resp.Success,
		Error:     resp.Error,
		NodeId:    resp.NodeID,
		NodeName:  resp.NodeName,
		Providers: make([]*schedulerpb.ProviderInfo, 0, len(resp.Providers)),
	}

	for _, p := range resp.Providers {
		providerInfo := &schedulerpb.ProviderInfo{
			ProviderId:   p.ProviderID,
			ProviderName: p.ProviderName,
			ProviderType: p.ProviderType,
			Status:       p.Status,
		}

		if p.Available != nil {
			providerInfo.Available = &resourcepb.Info{
				Cpu:    p.Available.CPU,
				Memory: p.Available.Memory,
				Gpu:    p.Available.GPU,
				Tags:   append([]string(nil), p.Available.Tags...),
			}
		}

		if p.TotalCapacity != nil {
			providerInfo.TotalCapacity = &resourcepb.Info{
				Cpu:    p.TotalCapacity.CPU,
				Memory: p.TotalCapacity.Memory,
				Gpu:    p.TotalCapacity.GPU,
				Tags:   append([]string(nil), p.TotalCapacity.Tags...),
			}
		}

		if p.Used != nil {
			providerInfo.Used = &resourcepb.Info{
				Cpu:    p.Used.CPU,
				Memory: p.Used.Memory,
				Gpu:    p.Used.GPU,
				Tags:   append([]string(nil), p.Used.Tags...),
			}
		}

		if p.ResourceTags != nil {
			providerInfo.ResourceTags = &schedulerpb.ResourceTags{
				Cpu:    p.ResourceTags.CPU,
				Gpu:    p.ResourceTags.GPU,
				Memory: p.ResourceTags.Memory,
				Camera: p.ResourceTags.Camera,
			}
		}

		protoResp.Providers = append(protoResp.Providers, providerInfo)
	}

	return protoResp, nil
}

// convertComponentStatusToProto 转换 Component 状态到 proto
func convertComponentStatusToProto(status scheduler.ComponentStatus) schedulerpb.ComponentStatus {
	switch status {
	case scheduler.ComponentStatusDeploying:
		return schedulerpb.ComponentStatus_COMPONENT_STATUS_DEPLOYING
	case scheduler.ComponentStatusRunning:
		return schedulerpb.ComponentStatus_COMPONENT_STATUS_RUNNING
	case scheduler.ComponentStatusStopped:
		return schedulerpb.ComponentStatus_COMPONENT_STATUS_STOPPED
	case scheduler.ComponentStatusError:
		return schedulerpb.ComponentStatus_COMPONENT_STATUS_ERROR
	default:
		return schedulerpb.ComponentStatus_COMPONENT_STATUS_UNKNOWN
	}
}
