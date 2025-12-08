package provider

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Manager 管理 provider 的健康检查状态
type Manager struct {
	mu                sync.RWMutex
	lastHealthCheck   time.Time // 最后收到健康检测的时间
	providerID        string    // 当前分配的 provider ID
	healthCheckCtx    context.Context
	healthCheckCancel context.CancelFunc
	healthCheckWg     sync.WaitGroup
	timeout           time.Duration // 健康检测超时时间
	checkInterval     time.Duration // 检查间隔
	onTimeout         func()        // 超时回调函数
}

// NewManager 创建新的健康检查管理器
func NewManager(timeout, checkInterval time.Duration, onTimeout func()) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		healthCheckCtx:    ctx,
		healthCheckCancel: cancel,
		timeout:           timeout,
		checkInterval:     checkInterval,
		onTimeout:         onTimeout,
	}
}

// Start 启动健康检测超时监控
func (m *Manager) Start() {
	m.healthCheckWg.Add(1)
	go m.monitor()
	logrus.Info("Provider health check monitor started")
}

// Stop 停止健康检测超时监控
func (m *Manager) Stop() {
	if m.healthCheckCancel != nil {
		m.healthCheckCancel()
		m.healthCheckWg.Wait()
		logrus.Info("Provider health check monitor stopped")
	}
}

// UpdateHealthCheck 更新最后收到健康检测的时间
func (m *Manager) UpdateHealthCheck() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastHealthCheck = time.Now()
}

// SetProviderID 设置 provider ID（通常在 Connect 时调用）
func (m *Manager) SetProviderID(providerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providerID = providerID
	m.lastHealthCheck = time.Now() // 分配 ID 时记录时间
}

// ClearProviderID 清除 provider ID（超时或断开连接时调用）
func (m *Manager) ClearProviderID() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providerID = ""
	m.lastHealthCheck = time.Time{}
}

// GetProviderID 获取当前分配的 provider ID
func (m *Manager) GetProviderID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.providerID
}

// monitor 监控健康检测超时
func (m *Manager) monitor() {
	defer m.healthCheckWg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.healthCheckCtx.Done():
			return
		case <-ticker.C:
			m.mu.RLock()
			hasID := m.providerID != ""
			lastCheck := m.lastHealthCheck
			m.mu.RUnlock()

			// 如果已分配 ID 但超过超时时间没有收到健康检测，则触发超时回调
			if hasID && !lastCheck.IsZero() {
				elapsed := time.Since(lastCheck)
				if elapsed > m.timeout {
					logrus.Warnf("No health check received for %v, clearing provider ID %s", elapsed, m.providerID)
					m.ClearProviderID()
					if m.onTimeout != nil {
						m.onTimeout()
					}
				}
			}
		}
	}
}
