package provider

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/domain/resource/types"
	"github.com/sirupsen/logrus"
)

// Service Provider 服务接口
// 提供无状态的 Provider 操作服务
type Service interface {
	// RegisterProvider 注册 Provider 并建立连接
	RegisterProvider(ctx context.Context, provider Provider) error

	// FindAvailableProvider 查找满足资源要求的可用 Provider
	FindAvailableProvider(ctx context.Context, resourceRequest *types.Info) (Provider, error)

	// GetProvider 获取指定 ID 的 Provider
	GetProvider(id string) Provider

	// GetAllProviders 获取所有 Provider
	GetAllProviders() []Provider
}

type service struct {
	manager *Manager
}

// NewService 创建 Provider 服务
func NewService(manager *Manager) Service {
	return &service{
		manager: manager,
	}
}

// RegisterProvider 注册 Provider 并建立连接
func (s *service) RegisterProvider(ctx context.Context, provider Provider) error {
	if provider == nil {
		return fmt.Errorf("provider is nil")
	}

	// 建立连接
	if err := provider.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to provider %s: %w", provider.GetID(), err)
	}

	// 添加到管理器
	s.manager.Add(provider)
	logrus.Infof("Provider %s registered and connected", provider.GetID())

	return nil
}

// FindAvailableProvider 查找满足资源要求的可用 Provider
func (s *service) FindAvailableProvider(ctx context.Context, resourceRequest *types.Info) (Provider, error) {
	if resourceRequest == nil {
		return nil, fmt.Errorf("resource request is required")
	}

	// 获取所有已连接的 Provider
	connectedProviders := s.manager.GetByStatus(types.ProviderStatusConnected)

	for _, provider := range connectedProviders {
		// 检查 Provider 状态
		if provider.GetStatus() != types.ProviderStatusConnected {
			logrus.Debugf("Skipping provider %s: status is not connected", provider.GetID())
			continue
		}

		// 获取可用资源
		available, err := provider.GetAvailable(ctx)
		if err != nil {
			logrus.Warnf("Failed to get available resources from provider %s: %v", provider.GetID(), err)
			continue
		}
		logrus.Debugf("Available resources from provider %s: %v", provider.GetID(), available)

		// 检查是否满足资源要求
		if !satisfiesResourceRequest(available, resourceRequest) {
			logrus.Debugf("Provider %s does not have sufficient resources", provider.GetID())
			continue
		}

		return provider, nil
	}

	return nil, fmt.Errorf("no available provider found that satisfies the resource requirements")
}

// GetProvider 获取指定 ID 的 Provider
func (s *service) GetProvider(id string) Provider {
	return s.manager.Get(id)
}

// GetAllProviders 获取所有 Provider
func (s *service) GetAllProviders() []Provider {
	return s.manager.GetAll()
}

// satisfiesResourceRequest 检查可用资源是否满足资源请求
func satisfiesResourceRequest(available *types.Info, request *types.Info) bool {
	if available == nil || request == nil {
		return false
	}

	// 检查 CPU
	if available.CPU < request.CPU {
		return false
	}

	// 检查 Memory
	if available.Memory < request.Memory {
		return false
	}

	// 检查 GPU
	if available.GPU < request.GPU {
		return false
	}

	return true
}
