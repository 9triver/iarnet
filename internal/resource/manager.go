package resource

import (
	"context"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"
)

type Manager struct {
	limits  Usage
	current Usage
	mu      sync.Mutex
}

func NewManager(limits map[string]string) *Manager {
	rm := &Manager{}
	for k, v := range limits {
		switch ResourceType(k) {
		case CPU:
			rm.limits.CPU, _ = strconv.ParseFloat(v, 64)
		case Memory:
			rm.limits.Memory, _ = parseMemory(v)
		case GPU:
			rm.limits.GPU, _ = strconv.ParseFloat(v, 64)
		}
	}
	return rm
}

func (rm *Manager) GetCapacity(ctx context.Context) (*Capacity, error) {
	return nil, nil
}

func (rm *Manager) GetRealTimeUsage(ctx context.Context) (*Usage, error) {
	return &rm.current, nil
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
