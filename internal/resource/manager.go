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
	mu             sync.Mutex
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
	rm.mu.Lock()
	defer rm.mu.Unlock()

	provider, exists := rm.providers[providerID]
	if !exists {
		return nil, fmt.Errorf("provider with ID %s not found", providerID)
	}
	return provider, nil
}

// GetCapacity returns aggregated capacity from all providers
func (rm *Manager) GetCapacity(ctx context.Context) (*Capacity, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if len(rm.providers) == 0 {
		// Fallback to static limits if no providers
		return &Capacity{
			Total:     rm.limits,
			Allocated: rm.current,
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
			logrus.Warnf("Failed to get capacity from provider %s: %v", provider.GetProviderID(), err)
			continue
		}

		totalCapacity.CPU += capacity.Total.CPU
		totalCapacity.Memory += capacity.Total.Memory
		totalCapacity.GPU += capacity.Total.GPU

		totalAllocated.CPU += capacity.Allocated.CPU
		totalAllocated.Memory += capacity.Allocated.Memory
		totalAllocated.GPU += capacity.Allocated.GPU
	}

	available := Usage{
		CPU:    totalCapacity.CPU - totalAllocated.CPU,
		Memory: totalCapacity.Memory - totalAllocated.Memory,
		GPU:    totalCapacity.GPU - totalAllocated.GPU,
	}

	return &Capacity{
		Total:     totalCapacity,
		Allocated: totalAllocated,
		Available: available,
	}, nil
}

func parseMemory(memStr string) (float64, error) {
	// Simple parse, assume Gi/Mi, etc. For demo: assume Gi
	if len(memStr) > 2 && memStr[len(memStr)-2:] == "Gi" {
		return strconv.ParseFloat(memStr[:len(memStr)-2], 64)
	}
	val, err := strconv.ParseFloat(memStr, 64)
	return val, err
}

func (rm *Manager) CanAllocate(req Usage) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if rm.current.CPU+req.CPU > rm.limits.CPU ||
		rm.current.Memory+req.Memory > rm.limits.Memory ||
		rm.current.GPU+req.GPU > rm.limits.GPU {
		return false
	}
	return true
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
