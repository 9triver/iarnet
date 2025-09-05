package resource

import (
	"context"
	"fmt"
	"strconv"
	"sync"

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
	limits              Usage
	current             Usage
	mu                  sync.RWMutex
	internalProvider    Provider            // 节点内部provider
	externalProviders   map[string]Provider // 直接接入的外部provider
	discoveredProviders map[string]Provider // 通过gossip协议发现的provider
	nextProviderID      int
	monitor             *ProviderMonitor
}

func NewManager(limits map[string]string) *Manager {
	rm := &Manager{
		internalProvider:    nil,
		externalProviders:   make(map[string]Provider),
		discoveredProviders: make(map[string]Provider),
		nextProviderID:      1,
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

	localDockerProvider, err := GetLocalDockerProvider()
	if err != nil {
		logrus.Errorf("failed to create local Docker provider: %v", err)
	} else {
		rm.internalProvider = localDockerProvider
	}

	// Initialize provider monitor
	rm.monitor = NewProviderMonitor(rm)

	return rm
}

// RegisterProvider creates and registers a remote resource provider by type
func (rm *Manager) RegisterProvider(providerType ProviderType, config interface{}) (string, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Generate auto-increment provider ID
	providerID := fmt.Sprintf("%s-%d", providerType, rm.nextProviderID)
	rm.nextProviderID++

	// Create provider based on type
	var provider Provider
	var err error

	switch providerType {
	case ProviderTypeDocker:
		provider, err = NewDockerProvider(providerID, config)
		if err != nil {
			return "", fmt.Errorf("failed to create Docker provider: %w", err)
		}
	case ProviderTypeK8s:
		provider, err = NewK8sProvider(providerID, config)
		if err != nil {
			return "", fmt.Errorf("failed to create K8s provider: %w", err)
		}
	default:
		return "", fmt.Errorf("unsupported provider type: %s", providerType)
	}

	// Store as external provider
	rm.externalProviders[providerID] = provider
	return providerID, nil
}

// RegisterDiscoveredProvider registers a provider discovered through gossip protocol
func (rm *Manager) RegisterDiscoveredProvider(provider Provider) {
	rm.mu.Lock()
	rm.discoveredProviders[provider.GetID()] = provider
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
	if _, exists := rm.externalProviders[providerID]; exists {
		delete(rm.externalProviders, providerID)
		rm.RemoveProviderFromMonitoring(providerID)
		logrus.Infof("Unregistered external provider: %s", providerID)
		return
	}

	// Check and remove from discovered providers
	if _, exists := rm.discoveredProviders[providerID]; exists {
		delete(rm.discoveredProviders, providerID)
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
	if provider, exists := rm.externalProviders[providerID]; exists {
		return provider, nil
	}

	// Check discovered providers
	if provider, exists := rm.discoveredProviders[providerID]; exists {
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
	totalProviders += len(rm.externalProviders) + len(rm.discoveredProviders)

	if totalProviders == 0 {
		// Fallback to static limits if no providers
		return &Capacity{
			Total: rm.limits,
			Used:  rm.current,
			Available: Usage{
				CPU:    rm.limits.CPU - rm.current.CPU,
				Memory: rm.limits.Memory - rm.current.Memory,
				GPU:    rm.limits.GPU - rm.current.GPU,
			},
		}, nil
	}

	var totalCapacity, totalAllocated Usage

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
	for _, provider := range rm.externalProviders {
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
	for _, provider := range rm.discoveredProviders {
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

	available := Usage{
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

	logrus.Infof("Starting deployment process for container with image: %s", containerSpec.Image)
	logrus.Debugf("Container spec: CPU=%.1f, Memory=%dMB, GPU=%d, Ports=%v",
		containerSpec.CPU, containerSpec.Memory, containerSpec.GPU, containerSpec.Ports)

	// 检查资源是否充足
	usageReq := Usage{CPU: containerSpec.CPU, Memory: containerSpec.Memory, GPU: containerSpec.GPU}
	logrus.Debugf("Checking resource availability: CPU=%.1f, Memory=%dMB, GPU=%d",
		usageReq.CPU, usageReq.Memory, usageReq.GPU)

	provider := rm.canAllocate(usageReq)
	if provider == nil {
		logrus.Errorf("Resource allocation failed: insufficient resources for CPU=%.1f, Memory=%dMB, GPU=%d",
			usageReq.CPU, usageReq.Memory, usageReq.GPU)
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

func (rm *Manager) canAllocate(req Usage) Provider {
	logrus.Debugf("Checking resource allocation: Requested(CPU=%.1f, Memory=%.1f, GPU=%.1f)", req.CPU, req.Memory, req.GPU)

	totalProviders := 0
	if rm.internalProvider != nil {
		totalProviders++
	}
	totalProviders += len(rm.externalProviders) + len(rm.discoveredProviders)
	logrus.Debugf("Searching for available provider among %d providers", totalProviders)

	// Check internal provider first
	if rm.internalProvider != nil {
		logrus.Debugf("Checking internal provider %s with status %v", rm.internalProvider.GetID(), rm.internalProvider.GetStatus())
		if rm.internalProvider.GetStatus() == StatusConnected {
			capacity, err := rm.internalProvider.GetCapacity(context.Background())
			if err != nil {
				logrus.WithError(err).Warnf("Failed to get capacity for internal provider %s", rm.internalProvider.GetID())
			} else {
				logrus.Debugf("Internal provider %s capacity: Available(CPU=%.1f, Memory=%.1f, GPU=%.1f)",
					rm.internalProvider.GetID(), capacity.Available.CPU, capacity.Available.Memory, capacity.Available.GPU)
				if capacity.Available.CPU >= req.CPU &&
					capacity.Available.Memory >= req.Memory &&
					capacity.Available.GPU >= req.GPU {
					logrus.Infof("Found suitable internal provider: %s for resource allocation", rm.internalProvider.GetID())
					return rm.internalProvider
				}
			}
		}
	}

	// Check external providers
	for _, provider := range rm.externalProviders {
		logrus.Debugf("Checking remote provider %s with status %v", provider.GetID(), provider.GetStatus())
		if provider.GetStatus() == StatusConnected {
			capacity, err := provider.GetCapacity(context.Background())
			if err != nil {
				logrus.WithError(err).Warnf("Failed to get capacity for external provider %s", provider.GetID())
				continue
			}
			logrus.Debugf("Remote provider %s capacity: Available(CPU=%.1f, Memory=%.1f, GPU=%.1f)",
				provider.GetID(), capacity.Available.CPU, capacity.Available.Memory, capacity.Available.GPU)
			if capacity.Available.CPU >= req.CPU &&
				capacity.Available.Memory >= req.Memory &&
				capacity.Available.GPU >= req.GPU {
				logrus.Infof("Found suitable external provider: %s for resource allocation", provider.GetID())
				return provider
			}
		}
	}

	// Check discovered providers
	for _, provider := range rm.discoveredProviders {
		logrus.Debugf("Checking provider %s with status %v", provider.GetID(), provider.GetStatus())
		if provider.GetStatus() == StatusConnected {
			// 获取 provider 的容量信息
			capacity, err := provider.GetCapacity(context.Background())
			if err != nil {
				logrus.WithError(err).Warnf("Failed to get capacity for provider %s", provider.GetID())
				continue
			}

			logrus.Debugf("Provider %s capacity: Available(CPU=%.1f, Memory=%.1f, GPU=%.1f)",
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
	logrus.Warnf("No suitable provider found for resource allocation: CPU=%.1f, Memory=%.1f, GPU=%.1f", req.CPU, req.Memory, req.GPU)
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
	for _, provider := range rm.externalProviders {
		rm.monitor.AddProvider(provider)
	}
	for _, provider := range rm.discoveredProviders {
		rm.monitor.AddProvider(provider)
	}
	rm.mu.RUnlock()
}

// GetProviders 返回所有注册的资源提供者
// CategorizedProviders represents providers categorized by their source
type CategorizedProviders struct {
	InternalProvider    Provider   `json:"internal_provider"`
	ExternalProviders   []Provider `json:"external_providers"`
	DiscoveredProviders []Provider `json:"discovered_providers"`
}

// GetProviders returns providers categorized by their source
func (rm *Manager) GetProviders() *CategorizedProviders {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := &CategorizedProviders{
		InternalProvider:    rm.internalProvider,
		ExternalProviders:   make([]Provider, 0, len(rm.externalProviders)),
		DiscoveredProviders: make([]Provider, 0, len(rm.discoveredProviders)),
	}

	// Convert external providers map to slice
	for _, provider := range rm.externalProviders {
		result.ExternalProviders = append(result.ExternalProviders, provider)
	}

	// Convert discovered providers map to slice
	for _, provider := range rm.discoveredProviders {
		result.DiscoveredProviders = append(result.DiscoveredProviders, provider)
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
