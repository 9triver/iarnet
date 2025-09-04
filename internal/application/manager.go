package application

import (
	"errors"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"
)

type Manager struct {
	applications map[string]*AppRef
	nextAppID    int
	mu           sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		applications: make(map[string]*AppRef),
		nextAppID:    1,
	}
}

func (m *Manager) CreateApplication(name string) *AppRef {
	m.mu.Lock()
	defer m.mu.Unlock()
	appID := strconv.Itoa(m.nextAppID)
	m.nextAppID++
	app := &AppRef{
		ID:     appID,
		Name:   name,
		Status: StatusUndeployed,
	}
	m.applications[appID] = app
	logrus.Infof("Application created in manager: ID=%s, Name=%s, Status=%s", appID, name, StatusUndeployed)
	return app
}

func (m *Manager) GetApplication(appID string) (*AppRef, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	app, ok := m.applications[appID]
	if !ok {
		return nil, errors.New("application not found")
	}
	return app, nil
}

func (m *Manager) GetAllApplications() []*AppRef {
	m.mu.RLock()
	defer m.mu.RUnlock()
	apps := make([]*AppRef, 0, len(m.applications))
	for _, app := range m.applications {
		apps = append(apps, app)
	}
	return apps
}

// UpdateApplicationStatus 更新应用状态
func (m *Manager) UpdateApplicationStatus(appID string, status Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	app, ok := m.applications[appID]
	if !ok {
		logrus.Warnf("Attempted to update status for non-existent application: %s", appID)
		return errors.New("application not found")
	}
	oldStatus := app.Status
	app.Status = status
	logrus.Infof("Application status updated: ID=%s, OldStatus=%s, NewStatus=%s", appID, oldStatus, status)
	return nil
}

// ApplicationStats 应用统计信息
type ApplicationStats struct {
	Total      int `json:"total"`      // 总应用数
	Running    int `json:"running"`    // 运行中
	Stopped    int `json:"stopped"`    // 已停止
	Undeployed int `json:"undeployed"` // 未部署
	Failed     int `json:"failed"`     // 失败
	Unknown    int `json:"unknown"`    // 未知
}

// GetApplicationStats 获取应用统计信息
func (m *Manager) GetApplicationStats() ApplicationStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := ApplicationStats{}

	for _, app := range m.applications {
		stats.Total++
		switch app.Status {
		case StatusRunning:
			stats.Running++
		case StatusStopped:
			stats.Stopped++
		case StatusUndeployed:
			stats.Undeployed++
		case StatusFailed:
			stats.Failed++
		case StatusUnknown:
			stats.Unknown++
		}
	}

	return stats
}
