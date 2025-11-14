package workspace

import (
	"fmt"
	"sync"
)

// Manager 工作空间管理器
// 负责在运行时持有内存中的工作空间领域对象，不负责创建
type Manager struct {
	mu         sync.RWMutex
	workspaces map[string]*Workspace
}

// NewManager 创建工作空间管理器
func NewManager() *Manager {
	return &Manager{
		mu:         sync.RWMutex{},
		workspaces: make(map[string]*Workspace),
	}
}

// Add 添加工作空间到管理器
func (m *Manager) Add(appID string, workspace *Workspace) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.workspaces[appID] = workspace
}

// Get 获取工作空间，如果不存在则返回错误
func (m *Manager) Get(appID string) (*Workspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workspace, ok := m.workspaces[appID]
	if !ok {
		return nil, fmt.Errorf("workspace not found for app %s", appID)
	}

	return workspace, nil
}

// Remove 移除工作空间
func (m *Manager) Remove(appID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.workspaces, appID)
}
