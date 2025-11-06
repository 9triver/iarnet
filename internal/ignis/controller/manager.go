package controller

import (
	"context"
	"sync"
)

type Manager interface {
	GetOrCreateController(ctx context.Context, appID string) (*Controller, error)
	AttachSession(appID, sessionID string) error
	DetachSession(appID, sessionID string)
	ReleaseIfIdle(ctx context.Context, appID string) bool
	StopAll(ctx context.Context) error
}

type manager struct {
	mu      sync.RWMutex
	controllers map[string]*Controller // appID -> entry
}

func NewManager() Manager {
	return &manager{
		controllers: make(map[string]*Controller),
	}
}

func (m *manager) GetOrCreateController(ctx context.Context, appID string) (*Controller, error) {
	m.mu.RLock()
	if e, ok := m.entries[appID]; ok {
		m.mu.RUnlock()
		return e, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entries[appID]; ok {
		return e, nil
	}

	ctrl := NewController(appID)
	// 延迟启动：首次需要时启动
	if err := ctrl.Start(ctx); err != nil {
		return nil, err
	}
	m.entries[appID] = ctrl
	return ctrl, nil
}

func (m *manager) AttachSession(appID, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !ok {
		// 惰性创建：未预先 GetOrCreate 的情况下也允许绑定
		ctrl := NewController(appID)
		// 启动
		if err := ctrl.Start(context.Background()); err != nil {
			return err
		}
		m.entries[appID] = ctrl
	}
	return nil
}

func (m *manager) DetachSession(appID, sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
		delete(m.entries, appID)
	}
}

func (m *manager) ReleaseIfIdle(ctx context.Context, appID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !ok {
		return false
	}
	if len(m.entries) > 0 {
		return false
	}
	_ = ctrl.Stop(ctx)
	delete(m.entries, appID)
	return true
}

func (m *manager) StopAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for appID := range m.entries {
		_ = m.entries[appID].Stop(ctx)
		delete(m.entries, appID)
	}
	return nil
}
