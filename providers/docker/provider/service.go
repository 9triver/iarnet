package provider

import (
	"context"
	"fmt"
	"sync"

	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/sirupsen/logrus"
)

// Service 实现 ProviderService gRPC 服务
type Service struct {
	providerpb.UnimplementedProviderServiceServer
	mu         sync.RWMutex
	providerID string
	dockerMgr  *DockerManager
}

// NewService 创建新的 provider 服务
func NewService(dockerMgr *DockerManager) *Service {
	return &Service{
		dockerMgr: dockerMgr,
	}
}

// AssignID 接收服务端分配的 provider ID
func (s *Service) AssignID(ctx context.Context, req *providerpb.AssignIDRequest) (*providerpb.AssignIDResponse, error) {
	if req == nil {
		return &providerpb.AssignIDResponse{
			Success: false,
			Error:   "request is nil",
		}, nil
	}

	if req.ProviderId == "" {
		return &providerpb.AssignIDResponse{
			Success: false,
			Error:   "provider ID is required",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.providerID != "" {
		return &providerpb.AssignIDResponse{
			Success: false,
			Error:   fmt.Sprintf("provider ID already assigned: %s", s.providerID),
		}, nil
	}

	s.providerID = req.ProviderId
	logrus.Infof("Provider ID assigned: %s", s.providerID)

	return &providerpb.AssignIDResponse{
		Success: true,
	}, nil
}

// GetProviderID 获取当前分配的 provider ID
func (s *Service) GetProviderID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.providerID
}
