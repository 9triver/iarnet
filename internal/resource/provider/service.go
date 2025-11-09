package provider

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/sirupsen/logrus"
)

// Service 实现 provider gRPC 服务
type Service struct {
	providerpb.UnimplementedProviderServiceServer
	manager *resource.Manager
}

// NewService 创建新的 provider 服务
func NewService(manager *resource.Manager) *Service {
	return &Service{
		manager: manager,
	}
}

// RegisterProvider 处理 provider 注册请求并分配 ID
func (s *Service) RegisterProvider(ctx context.Context, req *providerpb.RegisterProviderRequest) (*providerpb.RegisterProviderResponse, error) {
	if req == nil {
		return &providerpb.RegisterProviderResponse{
			Success: false,
			Error:   "request is nil",
		}, nil
	}

	if req.Name == "" {
		return &providerpb.RegisterProviderResponse{
			Success: false,
			Error:   "provider name is required",
		}, nil
	}

	if req.Host == "" {
		return &providerpb.RegisterProviderResponse{
			Success: false,
			Error:   "provider host is required",
		}, nil
	}

	if req.Port <= 0 {
		return &providerpb.RegisterProviderResponse{
			Success: false,
			Error:   "provider port must be greater than 0",
		}, nil
	}

	if req.Type == "" {
		return &providerpb.RegisterProviderResponse{
			Success: false,
			Error:   "provider type is required",
		}, nil
	}

	// 调用 Manager 注册 provider 并分配 ID
	providerID, err := s.manager.RegisterProvider(
		resource.ProviderType(req.Type),
		req.Name,
		map[string]interface{}{
			"host":   req.Host,
			"port":   int(req.Port),
			"config": req.Config,
		},
	)

	if err != nil {
		logrus.Errorf("Failed to register provider %s: %v", req.Name, err)
		return &providerpb.RegisterProviderResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to register provider: %v", err),
		}, nil
	}

	logrus.Infof("Successfully registered provider %s with ID %s", req.Name, providerID)
	return &providerpb.RegisterProviderResponse{
		ProviderId: providerID,
		Success:    true,
	}, nil
}

