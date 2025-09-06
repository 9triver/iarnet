package application

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/server/request"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	applications map[string]*AppRef
	nextAppID    int
	mu           sync.RWMutex
	config       *config.Config
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		applications: make(map[string]*AppRef),
		nextAppID:    1,
		config:       cfg,
	}
}

// CloneGitRepository 克隆Git仓库到本地目录
func (m *Manager) CloneGitRepository(gitUrl, branch, appID string) (string, error) {
	// 从配置中获取工作目录，如果未配置则使用默认值
	workspaceBaseDir := m.config.WorkspaceDir
	if workspaceBaseDir == "" {
		workspaceBaseDir = "./workspaces"
		logrus.Warn("WorkspaceDir not configured, using default: ./workspaces")
	}
	
	// 创建应用专用的工作目录
	workDir := filepath.Join(workspaceBaseDir, appID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create workspace directory: %v", err)
	}

	logrus.Infof("Cloning repository %s (branch: %s) to %s", gitUrl, branch, workDir)

	// 执行git clone命令
	cmd := exec.Command("git", "clone", "-b", branch, "--single-branch", gitUrl, workDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// 如果克隆失败，清理目录
		os.RemoveAll(workDir)
		return "", fmt.Errorf("failed to clone repository: %v", err)
	}

	logrus.Infof("Successfully cloned repository to %s", workDir)
	return workDir, nil
}

func (m *Manager) CreateApplication(createReq *request.CreateApplicationRequest) *AppRef {
	m.mu.Lock()
	defer m.mu.Unlock()
	appID := strconv.Itoa(m.nextAppID)
	m.nextAppID++
	app := &AppRef{
		ID:           appID,
		Name:         createReq.Name,
		ContainerRef: nil,
		Status:       StatusUndeployed,
		Type:         createReq.Type,
		GitUrl:       createReq.GitUrl,
		Branch:       createReq.Branch,
		Description:  createReq.Description,
		Ports:        createReq.Ports,
		HealthCheck:  createReq.HealthCheck,
	}
	m.applications[appID] = app
	logrus.Infof("Application created in manager: ID=%s, Name=%s, Status=%s", appID, createReq.Name, app.Status)

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
	Failed     int `json:"failed"`     // 失败（包含错误和未知状态）
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
		case StatusDeploying:
			// 部署中的应用暂时不计入任何统计分类
			// 可以根据需要添加新的统计字段
		}
	}

	return stats
}
