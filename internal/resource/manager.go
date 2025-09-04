package resource

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"
)

// providerType defines the type of resource provider
type providerType string

// ProviderType contains all available provider types
var ProviderType = struct {
	Docker providerType
}{
	Docker: "docker",
}

// String returns the string representation of providerType
func (pt providerType) String() string {
	return string(pt)
}

type Manager struct {
	limits         Usage
	current        Usage
	mu             sync.RWMutex
	providers      map[string]Provider
	nextProviderID int
}

func NewManager(limits map[string]string) *Manager {
	rm := &Manager{
		providers:      make(map[string]Provider),
		nextProviderID: 1,
	}
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
		rm.providers["local"] = localDockerProvider
	}
	return rm
}

// RegisterProvider creates and registers a resource provider by type
func (rm *Manager) RegisterProvider(providerType providerType, config interface{}) (string, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Generate auto-increment provider ID
	providerID := fmt.Sprintf("%s-%d", providerType, rm.nextProviderID)
	rm.nextProviderID++

	// Create provider based on type
	var provider Provider
	var err error

	switch providerType {
	case ProviderType.Docker:
		provider, err = NewDockerProvider(providerID, config)
		if err != nil {
			return "", fmt.Errorf("failed to create Docker provider: %w", err)
		}
	default:
		return "", fmt.Errorf("unsupported provider type: %s", providerType)
	}

	rm.providers[providerID] = provider
	return providerID, nil
}

// UnregisterProvider removes a resource provider
func (rm *Manager) UnregisterProvider(providerID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.providers, providerID)
}

// GetProvider returns a registered provider by ID
func (rm *Manager) GetProvider(providerID string) (Provider, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	provider, exists := rm.providers[providerID]
	if !exists {
		return nil, fmt.Errorf("provider with ID %s not found", providerID)
	}
	return provider, nil
}

// GetCapacity returns aggregated capacity from all providers
func (rm *Manager) GetCapacity(ctx context.Context) (*Capacity, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if len(rm.providers) == 0 {
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

	for _, provider := range rm.providers {
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

	provider := rm.CanAllocate(usageReq)
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

func (rm *Manager) CanAllocate(req Usage) Provider {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	logrus.Debugf("Checking resource allocation: Requested(CPU=%.1f, Memory=%.1f, GPU=%.1f)", req.CPU, req.Memory, req.GPU)
	logrus.Debugf("Searching for available provider among %d providers", len(rm.providers))

	// 遍历所有 providers，找到满足条件的 provider
	for _, provider := range rm.providers {
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
	// TODO: Implement polling for real usage.
}

// GetProviders 返回所有注册的资源提供者
func (rm *Manager) GetProviders() []Provider {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	providers := make([]Provider, 0, len(rm.providers))
	for _, provider := range rm.providers {
		providers = append(providers, provider)
	}
	return providers
}
