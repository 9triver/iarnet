package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/types"
	providerrepo "github.com/9triver/iarnet/internal/infra/repository/resource"
	common "github.com/9triver/iarnet/internal/proto/common"
	"github.com/sirupsen/logrus"
)

// Service Provider 服务接口
// 提供无状态的 Provider 操作服务
type Service interface {
	// LoadProviders 从 repository 加载所有 provider 并加入 manager
	LoadProviders(ctx context.Context) error

	// RegisterProvider 注册 Provider 并建立连接
	RegisterProvider(ctx context.Context, name string, host string, port int) (*Provider, error)

	// UnregisterProvider 注销 Provider 并断开连接
	UnregisterProvider(ctx context.Context, id string) error

	// FindAvailableProvider 查找满足资源要求和语言支持的可用 Provider
	FindAvailableProvider(ctx context.Context, resourceRequest *types.Info, language common.Language) (*Provider, error)

	// GetProvider 获取指定 ID 的 Provider（返回接口类型，可能是 *Provider 或 *FakeProvider）
	GetProvider(id string) ProviderInterface

	// GetAllProviders 获取所有 Provider（返回接口类型列表，包括普通 Provider 和 FakeProvider）
	GetAllProviders() []ProviderInterface
}

type service struct {
	manager      *Manager
	repo         providerrepo.ProviderRepo
	envVariables *EnvVariables
}

// NewService 创建 Provider 服务
func NewService(manager *Manager, repo providerrepo.ProviderRepo, envVariables *EnvVariables) Service {
	s := &service{
		manager:      manager,
		repo:         repo,
		envVariables: envVariables,
	}
	return s
}

// LoadProviders 从 repository 加载所有 provider 并加入 manager
// 在启动时调用，用于恢复持久化的 provider
func (s *service) LoadProviders(ctx context.Context) error {
	if s.repo == nil {
		logrus.Debug("Provider repository is nil, skipping load")
		return nil
	}

	daos, err := s.repo.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to load providers from repository: %w", err)
	}

	logrus.Infof("Loading %d providers from repository", len(daos))

	for _, dao := range daos {
		provider := NewProviderWithID(dao.ID, dao.Name, dao.Host, dao.Port, s.envVariables)
		if err := provider.Connect(ctx); err != nil {
			logrus.Warnf("Failed to connect to provider %s: %v", dao.ID, err)
			continue
		}
		s.manager.Add(provider)

		// 连接成功后，立即执行一次健康检查以更新资源标签
		healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := provider.HealthCheck(healthCtx); err != nil {
			logrus.Warnf("Failed to perform initial health check for provider %s: %v (will retry in next health check cycle)", dao.ID, err)
		} else {
			tags := provider.GetResourceTags()
			if tags != nil {
				logrus.Infof("Provider %s initial health check succeeded, resource tags: CPU=%v, GPU=%v, Memory=%v, Camera=%v",
					dao.ID, tags.CPU, tags.GPU, tags.Memory, tags.Camera)
			}
		}
		cancel()
	}

	logrus.Infof("Successfully loaded %d providers from repository", len(daos))
	return nil
}

// RegisterProvider 注册 Provider 并建立连接
func (s *service) RegisterProvider(ctx context.Context, name string, host string, port int) (*Provider, error) {
	// 创建 provider 实例
	provider := NewProvider(name, host, port, s.envVariables)

	// 持久化到数据库
	if s.repo != nil {
		dao := &providerrepo.ProviderDAO{
			ID:        provider.GetID(),
			Name:      provider.GetName(),
			Host:      provider.GetHost(),
			Port:      provider.GetPort(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := s.repo.Create(ctx, dao); err != nil {
			logrus.Warnf("Failed to persist provider %s to database: %v", provider.GetID(), err)
			// 不返回错误，因为内存中已经注册成功
		}
	}

	// 建立连接
	if err := provider.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to provider %s: %w", provider.GetID(), err)
	}

	s.manager.Add(provider)

	// 连接成功后，立即执行一次健康检查以更新资源标签
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := provider.HealthCheck(healthCtx); err != nil {
		logrus.Warnf("Failed to perform initial health check for provider %s: %v (will retry in next health check cycle)", provider.GetID(), err)
	} else {
		tags := provider.GetResourceTags()
		if tags != nil {
			logrus.Infof("Provider %s initial health check succeeded, resource tags: CPU=%v, GPU=%v, Memory=%v, Camera=%v",
				provider.GetID(), tags.CPU, tags.GPU, tags.Memory, tags.Camera)
		}
	}

	logrus.Infof("Provider %s registered and connected", provider.GetID())
	return provider, nil
}

// UnregisterProvider 注销 Provider 并断开连接
func (s *service) UnregisterProvider(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("provider id is required")
	}

	// 获取 provider
	provider := s.manager.Get(id)
	if provider == nil {
		return fmt.Errorf("provider %s not found", id)
	}

	// 断开连接（如果已连接）
	// 注意：这里需要类型断言，因为 ProviderInterface 可能包含 FakeProvider
	if realProvider, ok := provider.(*Provider); ok {
		if realProvider.GetStatus() == types.ProviderStatusConnected {
			realProvider.Disconnect()
			logrus.Infof("Provider %s disconnected", id)
		}
	} else {
		// FakeProvider 也支持 Disconnect
		if provider.GetStatus() == types.ProviderStatusConnected {
			provider.Disconnect()
			logrus.Infof("Provider %s disconnected", id)
		}
	}

	// 从管理器中移除
	s.manager.Remove(id)

	// 从数据库删除（fake provider 不需要持久化，跳过数据库操作）
	if !provider.IsFake() {
		if s.repo != nil {
			if err := s.repo.Delete(ctx, id); err != nil {
				logrus.Warnf("Failed to delete provider %s from database: %v", id, err)
				// 不返回错误，因为内存中已经移除
			}
		}
	} else {
		logrus.Debugf("Skipping database deletion for fake provider %s", id)
	}

	logrus.Infof("Provider %s unregistered", id)
	return nil
}

// ProviderInterface 已在 manager.go 中定义，这里不再重复定义

// FindAvailableProvider 查找满足资源要求和语言支持的可用 Provider
// 优先使用缓存数据，如果找不到合适的 provider，会尝试强制刷新后重试
// 注意：会排除 fake provider
func (s *service) FindAvailableProvider(ctx context.Context, resourceRequest *types.Info, language common.Language) (*Provider, error) {
	if resourceRequest == nil {
		return nil, fmt.Errorf("resource request is required")
	}

	// 获取所有已连接的 Provider
	connectedProviders := s.manager.GetByStatus(types.ProviderStatusConnected)

	// 第一轮：使用缓存数据查找
	for _, provider := range connectedProviders {
		// 排除 fake provider
		if provider.IsFake() {
			logrus.Debugf("Skipping fake provider %s in schedule", provider.GetID())
			continue
		}

		// 类型断言为 *Provider（因为 fake provider 已被排除）
		realProvider, ok := provider.(*Provider)
		if !ok {
			logrus.Debugf("Skipping provider %s: not a real provider", provider.GetID())
			continue
		}
		// 检查 Provider 状态
		if provider.GetStatus() != types.ProviderStatusConnected {
			logrus.Debugf("Skipping provider %s: status is not connected", provider.GetID())
			continue
		}

		// 检查语言支持（如果指定了语言）
		if language != common.Language_LANG_UNKNOWN && !provider.SupportsLanguage(language) {
			logrus.Debugf("Provider %s does not support language %v", provider.GetID(), language)
			continue
		}

		if !providerHasRequiredTags(provider.GetResourceTags(), resourceRequest.Tags) {
			logrus.Debugf("Provider %s does not satisfy required tags", provider.GetID())
			continue
		}

		// 获取可用资源（优先使用缓存）
		available, err := provider.GetAvailable(ctx)
		if err != nil {
			logrus.Warnf("Failed to get available resources from provider %s: %v", provider.GetID(), err)
			continue
		}
		logrus.Debugf("Available resources from provider %s (cached): %v", provider.GetID(), available)

		// 检查是否满足资源要求
		if !satisfiesResourceRequest(available, resourceRequest) {
			logrus.Debugf("Provider %s does not have sufficient resources (cached data)", provider.GetID())
			continue
		}

		return realProvider, nil
	}

	// 第二轮：如果第一轮没找到，强制刷新后重试
	logrus.Debugf("No provider found with cached data, trying with fresh data...")
	for _, provider := range connectedProviders {
		// 排除 fake provider
		if provider.IsFake() {
			continue
		}

		// 类型断言为 *Provider
		realProvider, ok := provider.(*Provider)
		if !ok {
			continue
		}

		if provider.GetStatus() != types.ProviderStatusConnected {
			continue
		}

		// 检查语言支持（如果指定了语言）
		if language != common.Language_LANG_UNKNOWN && !provider.SupportsLanguage(language) {
			logrus.Debugf("Provider %s does not support language %v (fresh)", provider.GetID(), language)
			continue
		}

		if !providerHasRequiredTags(provider.GetResourceTags(), resourceRequest.Tags) {
			logrus.Debugf("Provider %s does not satisfy required tags (fresh)", provider.GetID())
			continue
		}

		// 强制刷新并获取可用资源
		available, err := provider.GetAvailable(ctx, true) // forceRefresh = true
		if err != nil {
			logrus.Warnf("Failed to get available resources from provider %s (fresh): %v", provider.GetID(), err)
			continue
		}
		logrus.Debugf("Available resources from provider %s (fresh): %v", provider.GetID(), available)

		// 检查是否满足资源要求
		if !satisfiesResourceRequest(available, resourceRequest) {
			logrus.Debugf("Provider %s does not have sufficient resources (fresh data)", provider.GetID())
			continue
		}

		return realProvider, nil
	}

	return nil, fmt.Errorf("no available provider found that satisfies the resource requirements")
}

// GetProvider 获取指定 ID 的 Provider（返回接口类型）
func (s *service) GetProvider(id string) ProviderInterface {
	return s.manager.Get(id)
}

// GetAllProviders 获取所有 Provider（返回接口类型列表，包括普通 Provider 和 FakeProvider）
func (s *service) GetAllProviders() []ProviderInterface {
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

func providerHasRequiredTags(providerTags *types.ResourceTags, required []string) bool {
	if len(required) == 0 {
		return true
	}
	if providerTags == nil {
		return false
	}

	for _, tag := range required {
		switch strings.ToLower(tag) {
		case "cpu":
			if !providerTags.CPU {
				return false
			}
		case "gpu":
			if !providerTags.GPU {
				return false
			}
		case "memory":
			if !providerTags.Memory {
				return false
			}
		case "camera":
			if !providerTags.Camera {
				return false
			}
		default:
			return false
		}
	}
	return true
}
