package provider

import (
	"sync"

	"github.com/9triver/iarnet/internal/domain/resource/types"
)

// Manager 管理 Provider 实例
// 负责在运行时持有内存中的 Provider 对象
type Manager struct {
	mu        sync.RWMutex
	providers map[string]Provider // provider ID -> Provider
}

// NewManager 创建 Provider 管理器
func NewManager() *Manager {
	return &Manager{
		mu:        sync.RWMutex{},
		providers: make(map[string]Provider),
	}
}

// Add 添加 Provider 到管理器
func (m *Manager) Add(provider Provider) {
	if provider == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[provider.GetID()] = provider
}

// Get 获取指定 ID 的 Provider
func (m *Manager) Get(id string) Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.providers[id]
}

// GetAll 获取所有 Provider
func (m *Manager) GetAll() []Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	providers := make([]Provider, 0, len(m.providers))
	for _, p := range m.providers {
		providers = append(providers, p)
	}
	return providers
}

// GetByStatus 根据状态获取 Provider 列表
func (m *Manager) GetByStatus(status types.ProviderStatus) []Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	providers := make([]Provider, 0)
	for _, p := range m.providers {
		if p.GetStatus() == status {
			providers = append(providers, p)
		}
	}
	return providers
}

// Remove 从管理器中移除 Provider
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.providers, id)
}
