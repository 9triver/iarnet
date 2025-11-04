package resource

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/9triver/iarnet/internal/config"
	"github.com/lithammer/shortuuid/v4"
	"github.com/sirupsen/logrus"
)

// ProviderType defines the type of resource provider
type ProviderType string

const (
	ProviderTypeDocker ProviderType = "docker"
	ProviderTypeK8s    ProviderType = "k8s"
)

// String returns the string representation of providerType
func (pt ProviderType) String() string {
	return string(pt)
}

type Manager struct {
	limits           Usage
	current          Usage
	mu               sync.RWMutex
	internalProvider Provider            // 节点内部provider
	localProviders   map[string]Provider // 直接接入的外部provider
	remoteProviders  map[string]Provider // 通过gossip协议发现的provider
	monitor          *ProviderMonitor
	cfg              *config.Config
	store            Store // 持久化存储
}

func NewManager(cfg *config.Config) *Manager {
	limits := cfg.ResourceLimits

	// 初始化持久化存储
	storeConfig := &StoreConfig{
		MaxOpenConns:           cfg.Database.MaxOpenConns,
		MaxIdleConns:           cfg.Database.MaxIdleConns,
		ConnMaxLifetimeSeconds: cfg.Database.ConnMaxLifetimeSeconds,
	}
	store, err := NewStoreWithConfig(cfg.Database.ResourceProviderDBPath, storeConfig)
	if err != nil {
		logrus.Errorf("Failed to initialize resource provider store: %v", err)
		return nil
	}

	rm := &Manager{
		internalProvider: nil,
		localProviders:   make(map[string]Provider),
		remoteProviders:  make(map[string]Provider),
		cfg:              cfg,
		store:            store,
	}
	rm.monitor = NewProviderMonitor(rm)

	for k, v := range limits {
		switch Type(k) {
		case CPU:
			rm.limits.CPU, _ = strconv.ParseFloat(v, 64)
		case Memory:
			rm.limits.Memory, _ = parseMemory(v)
		case GPU:
			rm.limits.GPU, _ = strconv.ParseFloat(v, 64)
		}
	}

	// Check if Docker is available before creating internal provider
	if cfg.EnableLocalDocker {
		localDockerProvider, err := GetLocalDockerProvider()
		if err != nil {
			logrus.Warnf("Docker is available but failed to create local Docker provider: %v", err)
		} else {
			rm.localProviders[localDockerProvider.GetID()] = localDockerProvider
			logrus.Infof("Local Docker provider created successfully")
		}
	} else {
		logrus.Infof("Docker is not available, skipping internal provider creation")
	}

	// 从数据库加载已保存的 local providers 并重新连接
	if err := rm.loadProvidersFromStore(); err != nil {
		logrus.Errorf("Failed to load providers from store: %v", err)
	}

	// Initialize provider monitor
	rm.monitor = NewProviderMonitor(rm)

	return rm
}

// RegisterProvider creates and registers a remote resource provider by type
func (rm *Manager) RegisterProvider(providerType ProviderType, name string, config interface{}) (string, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Generate provider ID using shortuuid
	providerID := fmt.Sprintf("%s-%s", providerType, shortuuid.New())

	// Create provider based on type
	var provider Provider
	var err error

	switch providerType {
	case ProviderTypeDocker:
		provider, err = NewDockerProvider(providerID, name, config)
		if err != nil {
			return "", fmt.Errorf("failed to create Docker provider: %w", err)
		}
		// 保存到数据库
		if dockerConfig, ok := config.(DockerConfig); ok {
			if err := rm.saveProviderToStore(providerID, providerType, name, dockerConfig); err != nil {
				logrus.Errorf("Failed to save Docker provider to store: %v", err)
			}
		}
	case ProviderTypeK8s:
		provider, err = NewK8sProvider(providerID, name, config)
		if err != nil {
			return "", fmt.Errorf("failed to create K8s provider: %w", err)
		}
		// 保存到数据库
		if k8sConfig, ok := config.(K8sConfig); ok {
			if err := rm.saveProviderToStore(providerID, providerType, name, k8sConfig); err != nil {
				logrus.Errorf("Failed to save K8s provider to store: %v", err)
			}
		}
	default:
		return "", fmt.Errorf("unsupported provider type: %s", providerType)
	}

	// Store as external provider
	rm.localProviders[providerID] = provider
	return providerID, nil
}

// RegisterDiscoveredProvider registers a provider discovered through gossip protocol
func (rm *Manager) RegisterDiscoveredProvider(provider Provider) {
	rm.mu.Lock()
	rm.remoteProviders[provider.GetID()] = provider
	rm.mu.Unlock()

	// Add to monitoring
	rm.AddProviderToMonitoring(provider)

	logrus.Infof("Registered discovered provider: %s", provider.GetID())
}

// UnregisterProvider removes a resource provider
func (rm *Manager) UnregisterProvider(providerID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Check and remove from external providers
	if _, exists := rm.localProviders[providerID]; exists {
		delete(rm.localProviders, providerID)
		rm.RemoveProviderFromMonitoring(providerID)

		// 从数据库删除
		if err := rm.store.DeleteLocalProvider(providerID); err != nil {
			logrus.Errorf("Failed to delete provider from store: %v", err)
		}

		logrus.Infof("Unregistered external provider: %s", providerID)
		return
	}

	// Check and remove from discovered providers
	if _, exists := rm.remoteProviders[providerID]; exists {
		delete(rm.remoteProviders, providerID)
		rm.RemoveProviderFromMonitoring(providerID)
		logrus.Infof("Unregistered discovered provider: %s", providerID)
		return
	}

	// Cannot remove internal provider
	if rm.internalProvider != nil && rm.internalProvider.GetID() == providerID {
		logrus.Warnf("Cannot unregister internal provider: %s", providerID)
	}
}

// GetProvider returns a registered provider by ID
func (rm *Manager) GetProvider(providerID string) (Provider, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Check internal provider
	if rm.internalProvider != nil && rm.internalProvider.GetID() == providerID {
		return rm.internalProvider, nil
	}

	// Check external providers
	if provider, exists := rm.localProviders[providerID]; exists {
		return provider, nil
	}

	// Check discovered providers
	if provider, exists := rm.remoteProviders[providerID]; exists {
		return provider, nil
	}

	// Provider not found
	return nil, fmt.Errorf("provider with ID %s not found", providerID)
}

// GetCapacity returns aggregated capacity from all providers
func (rm *Manager) GetCapacity(ctx context.Context) (*Capacity, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Check if we have any providers
	totalProviders := 0
	if rm.internalProvider != nil {
		totalProviders++
	}
	totalProviders += len(rm.localProviders) + len(rm.remoteProviders)

	if totalProviders == 0 {
		// Return zero capacity if no providers are available
		return &Capacity{
			Total:     &Info{CPU: 0, Memory: 0, GPU: 0},
			Used:      &Info{CPU: 0, Memory: 0, GPU: 0},
			Available: &Info{CPU: 0, Memory: 0, GPU: 0},
		}, nil
	}

	totalCapacity, totalAllocated := &Info{CPU: 0, Memory: 0, GPU: 0}, &Info{CPU: 0, Memory: 0, GPU: 0}

	// Process internal provider
	if rm.internalProvider != nil {
		capacity, err := rm.internalProvider.GetCapacity(ctx)
		if err != nil {
			logrus.Warnf("Failed to get capacity from internal provider %s: %v", rm.internalProvider.GetID(), err)
		} else {
			totalCapacity.CPU += capacity.Total.CPU
			totalCapacity.Memory += capacity.Total.Memory
			totalCapacity.GPU += capacity.Total.GPU
			totalAllocated.CPU += capacity.Used.CPU
			totalAllocated.Memory += capacity.Used.Memory
			totalAllocated.GPU += capacity.Used.GPU
		}
	}

	// Process external providers
	for _, provider := range rm.localProviders {
		capacity, err := provider.GetCapacity(ctx)
		if err != nil {
			logrus.Warnf("Failed to get capacity from external provider %s: %v", provider.GetID(), err)
			continue
		}
		totalCapacity.CPU += capacity.Total.CPU
		totalCapacity.Memory += capacity.Total.Memory
		totalCapacity.GPU += capacity.Total.GPU
		totalAllocated.CPU += capacity.Used.CPU
		totalAllocated.Memory += capacity.Used.Memory
		totalAllocated.GPU += capacity.Used.GPU
	}

	// Process discovered providers
	for _, provider := range rm.remoteProviders {
		capacity, err := provider.GetCapacity(ctx)
		if err != nil {
			logrus.Warnf("Failed to get capacity from provider %s: %v", provider.GetID(), err)
			continue
		}

		totalCapacity.CPU += capacity.Total.CPU
		totalCapacity.Memory += capacity.Total.Memory
		totalCapacity.GPU += capacity.Total.GPU

		totalAllocated.CPU += capacity.Used.CPU
		totalAllocated.Memory += capacity.Used.Memory
		totalAllocated.GPU += capacity.Used.GPU
	}

	available := &Info{
		CPU:    totalCapacity.CPU - totalAllocated.CPU,
		Memory: totalCapacity.Memory - totalAllocated.Memory,
		GPU:    totalCapacity.GPU - totalAllocated.GPU,
	}

	return &Capacity{
		Total:     totalCapacity,
		Used:      totalAllocated,
		Available: available,
	}, nil
}

func parseMemory(memStr string) (float64, error) {
	// Parse memory string and return bytes
	if len(memStr) > 2 {
		unit := memStr[len(memStr)-2:]
		valStr := memStr[:len(memStr)-2]
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return 0, err
		}
		switch unit {
		case "Ki":
			return val * 1024, nil // KB to bytes
		case "Mi":
			return val * 1024 * 1024, nil // MB to bytes
		case "Gi":
			return val * 1024 * 1024 * 1024, nil // GB to bytes
		case "Ti":
			return val * 1024 * 1024 * 1024 * 1024, nil // TB to bytes
		}
	}
	// If no unit specified, assume bytes
	val, err := strconv.ParseFloat(memStr, 64)
	return val, err
}

func (rm *Manager) Deploy(ctx context.Context, containerSpec ContainerSpec) (*ContainerRef, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	req := containerSpec.Requirements

	logrus.Infof("Starting deployment process for container with image: %s", containerSpec.Image)
	logrus.Infof("Container spec: CPU=%dmc, Memory=%dBytes, GPU=%d, Ports=%v",
		req.CPU, req.Memory, req.GPU, containerSpec.Ports)

	// 检查资源是否充足
	provider := rm.canAllocate(req)
	if provider == nil {
		logrus.Errorf("Resource allocation failed: insufficient resources for CPU=%d, Memory=%d, GPU=%d",
			req.CPU, req.Memory, req.GPU)
		return nil, fmt.Errorf("resource limit exceeded")
	}
	logrus.Infof("Resource provider found for deployment: %T", provider)

	// 部署应用
	logrus.Info("Deploying container to resource provider")
	containerID, err := provider.Deploy(ctx, containerSpec)
	if err != nil {
		logrus.Errorf("Container deployment failed on provider: %v", err)
		return nil, fmt.Errorf("failed to deploy application: %w", err)
	}
	logrus.Infof("Container deployed successfully with ID: %s", containerID)

	// TODO: sync cache
	logrus.Debug("TODO: Implement cache synchronization after deployment")

	containerRef := &ContainerRef{
		ID:       containerID,
		Provider: provider,
		Spec:     containerSpec,
	}
	logrus.Infof("Deployment completed successfully: ContainerID=%s", containerID)
	return containerRef, nil
}

func (rm *Manager) canAllocate(req Info) Provider {
	logrus.Debugf("Checking resource allocation: Requested(CPU=%dmc, Memory=%dBytes, GPU=%d)", req.CPU, req.Memory, req.GPU)

	totalProviders := 0
	if rm.internalProvider != nil {
		totalProviders++
	}
	totalProviders += len(rm.localProviders) + len(rm.remoteProviders)
	logrus.Debugf("Searching for available provider among %d providers", totalProviders)

	// Check local providers
	for _, provider := range rm.localProviders {
		logrus.Debugf("Checking remote provider %s with status %v", provider.GetID(), provider.GetStatus())
		if provider.GetStatus() == StatusConnected {
			capacity, err := provider.GetCapacity(context.Background())
			if err != nil {
				logrus.WithError(err).Warnf("Failed to get capacity for local provider %s", provider.GetID())
				continue
			}
			logrus.Debugf("Local provider %s capacity: Available(CPU=%d, Memory=%d, GPU=%d)",
				provider.GetID(), capacity.Available.CPU, capacity.Available.Memory, capacity.Available.GPU)
			if capacity.Available.CPU >= req.CPU &&
				capacity.Available.Memory >= req.Memory &&
				capacity.Available.GPU >= req.GPU {
				logrus.Infof("Found suitable local provider: %s for resource allocation", provider.GetID())
				return provider
			}
		}
	}

	// Check remote providers
	for _, provider := range rm.remoteProviders {
		logrus.Debugf("Checking provider %s with status %v", provider.GetID(), provider.GetStatus())
		if provider.GetStatus() == StatusConnected {
			// 获取 provider 的容量信息
			capacity, err := provider.GetCapacity(context.Background())
			if err != nil {
				logrus.WithError(err).Warnf("Failed to get capacity for provider %s", provider.GetID())
				continue
			}

			logrus.Debugf("Remote provider %s capacity: Available(CPU=%d, Memory=%d, GPU=%d)",
				provider.GetID(), capacity.Available.CPU, capacity.Available.Memory, capacity.Available.GPU)

			// 检查是否有足够的资源
			if capacity.Available.CPU >= req.CPU &&
				capacity.Available.Memory >= req.Memory &&
				capacity.Available.GPU >= req.GPU {
				logrus.Infof("Found suitable provider: %s for resource allocation", provider.GetID())
				return provider
			} else {
				logrus.Debugf("Provider %s has insufficient resources", provider.GetID())
			}
		}
	}

	// 如果没有找到满足条件的 provider，返回 nil
	logrus.Warnf("No suitable provider found for resource allocation: CPU=%d, Memory=%d, GPU=%d", req.CPU, req.Memory, req.GPU)
	return nil
}

func (rm *Manager) Allocate(req Usage) {
	rm.mu.Lock()
	rm.current.CPU += req.CPU
	rm.current.Memory += req.Memory
	rm.current.GPU += req.GPU
	rm.mu.Unlock()
	logrus.Infof("Allocated: %+v, Current: %+v", req, rm.current)
}

func (rm *Manager) Deallocate(req Usage) {
	rm.mu.Lock()
	rm.current.CPU -= req.CPU
	rm.current.Memory -= req.Memory
	rm.current.GPU -= req.GPU
	rm.mu.Unlock()
	logrus.Infof("Deallocated: %+v, Current: %+v", req, rm.current)
}

// Monitor: Would poll Docker/K8s for actual usage, but for simplicity, assume requested == used.
func (rm *Manager) StartMonitoring() {
	// Start provider health monitoring
	rm.monitor.Start()

	// Add existing providers to monitoring
	rm.mu.RLock()
	if rm.internalProvider != nil {
		rm.monitor.AddProvider(rm.internalProvider)
	}
	for _, provider := range rm.localProviders {
		rm.monitor.AddProvider(provider)
	}
	for _, provider := range rm.remoteProviders {
		rm.monitor.AddProvider(provider)
	}
	rm.mu.RUnlock()
}

// GetProviders 返回所有注册的资源提供者
// CategorizedProviders represents providers categorized by their source
type CategorizedProviders struct {
	LocalProviders  []Provider `json:"local_providers"`  // 本地资源（包含内部和外部托管）
	RemoteProviders []Provider `json:"remote_providers"` // 远程资源（通过协作发现）
}

// GetProviders returns providers categorized by their source
func (rm *Manager) GetProviders() *CategorizedProviders {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := &CategorizedProviders{
		LocalProviders:  make([]Provider, 0, len(rm.localProviders)+1),
		RemoteProviders: make([]Provider, 0, len(rm.remoteProviders)),
	}

	// Add internal provider to local providers if exists
	if rm.internalProvider != nil {
		result.LocalProviders = append(result.LocalProviders, rm.internalProvider)
	}

	// Add external providers to local providers
	for _, provider := range rm.localProviders {
		result.LocalProviders = append(result.LocalProviders, provider)
	}

	// Add discovered providers to remote providers
	for _, provider := range rm.remoteProviders {
		result.RemoteProviders = append(result.RemoteProviders, provider)
	}

	return result
}

// StopMonitoring stops the provider monitoring
func (rm *Manager) StopMonitoring() {
	if rm.monitor != nil {
		rm.monitor.Stop()
	}
}

// HandleProviderFailure handles when a provider fails
func (rm *Manager) HandleProviderFailure(providerID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	logrus.Warnf("Handling failure for provider %s", providerID)

	// For now, we keep the provider but mark it as failed
	// In a more sophisticated implementation, we might:
	// 1. Migrate running containers to other providers
	// 2. Remove the provider from load balancing
	// 3. Attempt automatic recovery
}

// loadProvidersFromStore 从数据库加载 providers 并重新建立连接
func (rm *Manager) loadProvidersFromStore() error {
	providers, err := rm.store.GetAllLocalProviders()
	if err != nil {
		return fmt.Errorf("failed to get all local providers: %w", err)
	}

	for _, providerConfig := range providers {
		// 根据配置重新创建 provider
		var config interface{}

		switch providerConfig.ProviderType {
		case ProviderTypeDocker:
			dockerConfig, err := DeserializeDockerConfig(providerConfig.Config)
			if err != nil {
				logrus.Errorf("Failed to deserialize Docker config for provider %s: %v", providerConfig.ProviderID, err)
				continue
			}
			config = dockerConfig
		case ProviderTypeK8s:
			k8sConfig, err := DeserializeK8sConfig(providerConfig.Config)
			if err != nil {
				logrus.Errorf("Failed to deserialize K8s config for provider %s: %v", providerConfig.ProviderID, err)
				continue
			}
			config = k8sConfig
		default:
			logrus.Warnf("Unknown provider type %s for provider %s", providerConfig.ProviderType, providerConfig.ProviderID)
			continue
		}

		// 创建 provider 实例
		var provider Provider
		switch providerConfig.ProviderType {
		case ProviderTypeDocker:
			provider, err = NewDockerProvider(providerConfig.ProviderID, providerConfig.Name, config)
			if err != nil {
				logrus.Errorf("Failed to recreate Docker provider %s: %v", providerConfig.ProviderID, err)
				// 更新状态为断开
				rm.store.UpdateProviderStatus(providerConfig.ProviderID, StatusDisconnected)
				continue
			}
		case ProviderTypeK8s:
			provider, err = NewK8sProvider(providerConfig.ProviderID, providerConfig.Name, config)
			if err != nil {
				logrus.Errorf("Failed to recreate K8s provider %s: %v", providerConfig.ProviderID, err)
				// 更新状态为断开
				rm.store.UpdateProviderStatus(providerConfig.ProviderID, StatusDisconnected)
				continue
			}
		}

		// 注册到 localProviders
		rm.localProviders[providerConfig.ProviderID] = provider
		// 更新状态为已连接
		rm.store.UpdateProviderStatus(providerConfig.ProviderID, StatusConnected)
		logrus.Infof("Loaded and connected provider from store: ID=%s, Type=%s, Name=%s",
			providerConfig.ProviderID, providerConfig.ProviderType, providerConfig.Name)
	}

	return nil
}

// saveProviderToStore 保存 provider 配置到数据库
func (rm *Manager) saveProviderToStore(providerID string, providerType ProviderType, name string, config interface{}) error {
	var configStr string
	var err error

	switch providerType {
	case ProviderTypeDocker:
		dockerConfig, ok := config.(DockerConfig)
		if !ok {
			return fmt.Errorf("invalid Docker config type")
		}
		configStr, err = SerializeDockerConfig(dockerConfig)
		if err != nil {
			return err
		}
	case ProviderTypeK8s:
		k8sConfig, ok := config.(K8sConfig)
		if !ok {
			return fmt.Errorf("invalid K8s config type")
		}
		configStr, err = SerializeK8sConfig(k8sConfig)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported provider type: %s", providerType)
	}

	providerConfig := &ProviderConfig{
		ProviderID:   providerID,
		ProviderType: providerType,
		Name:         name,
		Config:       configStr,
		Status:       StatusConnected,
		CreatedAt:    getCurrentTimestamp(),
		UpdatedAt:    getCurrentTimestamp(),
	}

	return rm.store.SaveLocalProvider(providerConfig)
}

// Close 关闭 Manager 及其持久化存储
func (rm *Manager) Close() error {
	// 停止监控
	if rm.monitor != nil {
		rm.monitor.Stop()
	}

	// 关闭存储
	if rm.store != nil {
		return rm.store.Close()
	}

	return nil
}

// HandleProviderRecovery handles when a provider recovers
func (rm *Manager) HandleProviderRecovery(providerID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	logrus.Infof("Handling recovery for provider %s", providerID)

	// Provider is back online and can accept new workloads
	// In a more sophisticated implementation, we might:
	// 1. Re-enable the provider for load balancing
	// 2. Perform health verification
	// 3. Gradually increase load
}

// AddProviderToMonitoring adds a provider to the monitoring system
func (rm *Manager) AddProviderToMonitoring(provider Provider) {
	if rm.monitor != nil {
		rm.monitor.AddProvider(provider)
	}
}

// RemoveProviderFromMonitoring removes a provider from the monitoring system
func (rm *Manager) RemoveProviderFromMonitoring(providerID string) {
	if rm.monitor != nil {
		rm.monitor.RemoveProvider(providerID)
	}
}

// GetProviderHealthStatus returns the health status of all providers
func (rm *Manager) GetProviderHealthStatus() map[string]bool {
	if rm.monitor != nil {
		return rm.monitor.GetAllHealthStatus()
	}
	return make(map[string]bool)
}
