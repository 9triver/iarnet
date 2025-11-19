package provider

import (
	"context"
	"fmt"
	"sync"

	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
)

// Service 为 Kubernetes Provider 的占位实现。
// 这里提供最小化实现以便测试，可在后续补全实际逻辑。
type Service struct {
	providerpb.UnimplementedServiceServer
	mu         sync.Mutex
	providerID string
	capacity   *resourcepb.Capacity
}

func NewService(kubeconfig string) (*Service, error) {
	return &Service{
		capacity: &resourcepb.Capacity{
			Total:     &resourcepb.Info{Cpu: 8000, Memory: 32 << 30, Gpu: 0},
			Used:      &resourcepb.Info{Cpu: 1000, Memory: 4 << 30, Gpu: 0},
			Available: &resourcepb.Info{Cpu: 7000, Memory: 28 << 30, Gpu: 0},
		},
	}, nil
}

func (s *Service) Connect(ctx context.Context, req *providerpb.ConnectRequest) (*providerpb.ConnectResponse, error) {
	if req == nil || req.ProviderId == "" {
		return &providerpb.ConnectResponse{Success: false, Error: "provider ID required"}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.providerID != "" && s.providerID != req.ProviderId {
		return &providerpb.ConnectResponse{Success: false, Error: "already connected"}, nil
	}
	s.providerID = req.ProviderId
	return &providerpb.ConnectResponse{
		Success: true,
		ProviderType: &providerpb.ProviderType{
			Name: "k8s",
		},
	}, nil
}

func (s *Service) GetCapacity(ctx context.Context, req *providerpb.GetCapacityRequest) (*providerpb.GetCapacityResponse, error) {
	if err := s.ensureConnected(req.GetProviderId()); err != nil {
		return nil, err
	}
	return &providerpb.GetCapacityResponse{
		Capacity: s.capacity,
	}, nil
}

func (s *Service) GetAvailable(ctx context.Context, req *providerpb.GetAvailableRequest) (*providerpb.GetAvailableResponse, error) {
	if err := s.ensureConnected(req.GetProviderId()); err != nil {
		return nil, err
	}
	return &providerpb.GetAvailableResponse{
		Available: s.capacity.Available,
	}, nil
}

func (s *Service) ensureConnected(providerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.providerID == "" {
		return fmt.Errorf("provider not connected")
	}
	if providerID != "" && providerID != s.providerID {
		return fmt.Errorf("provider id mismatch")
	}
	return nil
}
