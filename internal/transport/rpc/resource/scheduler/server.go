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
		},
		TargetNodeID:  req.TargetNodeId,
		TargetAddress: req.TargetNodeAddress,
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
			},
			ProviderId: status.Component.GetProviderID(),
		}
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
