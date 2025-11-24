package provider

import (
	"context"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/types"
	"github.com/sirupsen/logrus"
)

// Manager 管理 Provider 实例
// 负责在运行时持有内存中的 Provider 对象
type Manager struct {
	mu        sync.RWMutex
	providers map[string]*Provider // provider ID -> Provider

	// 健康检测相关
	healthCheckInterval time.Duration // 健康检测间隔
	healthCheckTimeout  time.Duration // 健康检测超时时间
	healthCheckCtx      context.Context
	healthCheckCancel   context.CancelFunc
	healthCheckWg       sync.WaitGroup
}

// NewManager 创建 Provider 管理器
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		mu:                  sync.RWMutex{},
		providers:           make(map[string]*Provider),
		healthCheckInterval: 30 * time.Second, // 默认 30 秒检测一次
		healthCheckTimeout:  5 * time.Second,  // 默认 5 秒超时
		healthCheckCtx:      ctx,
		healthCheckCancel:   cancel,
	}
}

func (m *Manager) Start() {
	m.healthCheckWg.Add(1)
	go m.healthCheckLoop()
	logrus.Info("Provider health check started")
}

func (m *Manager) Stop() {
	if m.healthCheckCancel != nil {
		m.healthCheckCancel()
	}
	m.healthCheckWg.Wait()
	logrus.Info("Provider health check stopped")
}

// healthCheckLoop 健康检测循环
func (m *Manager) healthCheckLoop() {
	defer m.healthCheckWg.Done()

	ticker := time.NewTicker(m.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.healthCheckCtx.Done():
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检测
func (m *Manager) performHealthCheck() {
	m.mu.RLock()
	providers := make([]*Provider, 0, len(m.providers))
	for _, p := range m.providers {
		providers = append(providers, p)
	}
	m.mu.RUnlock()

	for _, provider := range providers {
		// 只检测已连接的 provider
		if provider.GetStatus() != types.ProviderStatusConnected {
			continue
		}

		// 创建带超时的上下文
		ctx, cancel := context.WithTimeout(m.healthCheckCtx, m.healthCheckTimeout)

		// 执行健康检测
		err := provider.HealthCheck(ctx)
		cancel()

		if err != nil {
			providerID := provider.GetID()
			logrus.Warnf("Provider %s (host: %s:%d) health check failed: %v, updating status to disconnected",
				providerID, provider.GetHost(), provider.GetPort(), err)
			provider.SetStatus(types.ProviderStatusDisconnected)
		} else {
			// 健康检查成功，记录日志
			tags := provider.GetResourceTags()
			if tags != nil {
				logrus.Debugf("Provider %s health check succeeded: CPU=%v, GPU=%v, Memory=%v, Camera=%v",
					provider.GetID(), tags.CPU, tags.GPU, tags.Memory, tags.Camera)
			} else {
				logrus.Debugf("Provider %s health check succeeded (no resource tags yet)", provider.GetID())
			}
		}
	}
}

// Add 添加 Provider 到管理器
func (m *Manager) Add(provider *Provider) {
	if provider == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// 使用 ID 作为 key
	id := provider.GetID()
	if id == "" {
		return
	}
	m.providers[id] = provider
}

// Get 获取指定 ID 的 Provider
func (m *Manager) Get(id string) *Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.providers[id]
}

// GetAll 获取所有 Provider
func (m *Manager) GetAll() []*Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	providers := make([]*Provider, 0, len(m.providers))
	for _, p := range m.providers {
		providers = append(providers, p)
	}
	return providers
}

// GetByStatus 根据状态获取 Provider 列表
func (m *Manager) GetByStatus(status types.ProviderStatus) []*Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	providers := make([]*Provider, 0)
	for _, p := range m.providers {
		if p.GetStatus() == status {
			providers = append(providers, p)
		}
	}
	return providers
}

// Remove 从管理器中移除 Provider（通过 ID）
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.providers, id)
}
