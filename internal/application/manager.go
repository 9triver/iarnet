package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/server/request"
	"github.com/9triver/iarnet/internal/util"
	"github.com/9triver/iarnet/internal/websocket"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	nextAppID int
	mu        sync.RWMutex
	config    *config.Config

	// 服务依赖
	workspace WorkspaceService
	runtime   RuntimeService
	logger    LoggerService
	logSystem LogSystem // 新日志系统

	apps map[string]*AppRef

	// 遗留字段（待逐步移除）
	rm    *resource.Manager
	wsHub *websocket.Hub
}

// LogSystem 日志系统接口（简化，避免循环依赖）
type LogSystem interface {
	StartCollectingContainer(containerID string, containerType string, labels map[string]string) error
	StopCollectingContainer(containerID string) error
	Stop() error
}

func NewManager(config *config.Config, resourceManager *resource.Manager) *Manager {

	// 连接本地docker
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.51"))
	if err != nil {
		logrus.Errorf("Failed to create docker client: %v", err)
		return nil
	}

	// 测试docker连接
	_, err = cli.Ping(context.Background())
	if err != nil {
		logrus.Errorf("Failed to ping docker daemon: %v", err)
		return nil
	}

	logrus.Info("Successfully connected to docker daemon")

	// Initialize WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run() // Start the hub in a goroutine

	// 初始化服务
	workspace := NewWorkspace(config.WorkspaceDir)
	runtime := NewRuntime(cli, resourceManager)
	logger := NewLogger(cli)

	// 初始化持久化存储

	m := &Manager{
		config: config,

		// 服务
		workspace: workspace,
		runtime:   runtime,
		logger:    logger,
		logSystem: nil, // 将由外部注入

		// 遗留字段
		rm:    resourceManager,
		wsHub: wsHub,
	}

	return m
}

// SetLogSystem 设置日志系统
func (m *Manager) SetLogSystem(logSystem LogSystem) {
	m.logSystem = logSystem
	logrus.Info("Log system set in application manager")
}

// GetLogSystem 获取日志系统
func (m *Manager) GetLogSystem() LogSystem {
	return m.logSystem
}

func (m *Manager) Stop() {
	// 停止日志系统
	if m.logSystem != nil {
		if err := m.logSystem.Stop(); err != nil {
			logrus.Errorf("Failed to stop log system: %v", err)
		}
	}

	// 移除工作目录
	if m.config.WorkspaceDir != "" {
		if err := os.RemoveAll(m.config.WorkspaceDir); err != nil {
			logrus.Errorf("Failed to remove workspace directory %s: %v", m.config.WorkspaceDir, err)
		} else {
			logrus.Infof("Successfully removed workspace directory: %s", m.config.WorkspaceDir)
		}
	}
	logrus.Info("Application manager stopped")
}

func (m *Manager) UpdateApplication(ctx context.Context, appID string, app *request.UpdateApplicationRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	appRef, ok := m.apps[appID]
	if !ok {
		return fmt.Errorf("application %s not found", appID)
	}

	if app.Name != nil {
		appRef.Name = *app.Name
	}
	if app.GitUrl != nil {
		appRef.GitUrl = app.GitUrl
	}
	if app.Branch != nil {
		appRef.Branch = app.Branch
	}
	if app.Type != nil {
		appRef.Type = *app.Type
	}
	if app.Description != nil {
		appRef.Description = app.Description
	}
	if app.Ports != nil {
		appRef.Ports = *app.Ports
	}
	if app.HealthCheck != nil {
		appRef.HealthCheck = app.HealthCheck
	}
	if app.ExecuteCmd != nil {
		appRef.ExecuteCmd = app.ExecuteCmd
	}
	if app.RunnerEnv != nil {
		appRef.RunnerEnv = app.RunnerEnv
	}

	// 保存到数据库
	m.apps[appID] = appRef
	return nil
}

// RunApplication 运行应用容器
func (m *Manager) RunApplication(appID string) error {
	app, err := m.GetApplication(appID)
	if err != nil {
		return fmt.Errorf("failed to get application: %w", err)
	}

	// 如果已有容器，先移除
	if app.ContainerID != nil {
		logrus.Infof("Application %s has container ID %s, removing it", appID, *app.ContainerID)
		if err := m.runtime.RemoveContainer(context.Background(), *app.ContainerID); err != nil {
			logrus.Errorf("Failed to remove container %s: %v", *app.ContainerID, err)
			return fmt.Errorf("failed to remove container: %w", err)
		}
		logrus.Infof("Successfully removed container %s", *app.ContainerID)
	}

	if app.RunnerEnv == nil {
		return fmt.Errorf("application %s has no runner environment specified", appID)
	}

	runnerImage, ok := m.config.RunnerImages[*app.RunnerEnv]
	if !ok || runnerImage == "" {
		return fmt.Errorf("runner image not configured for environment: %s", *app.RunnerEnv)
	}

	// 委托给 runtime 服务启动应用 runner 容器
	containerID, err := m.runtime.StartApplicationRunner(
		context.Background(),
		appID,
		runnerImage,
		*app.CodeDir,
		*app.ExecuteCmd,
		m.config.Ignis.Port,
	)
	if err != nil {
		logrus.Errorf("Failed to start application runner for %s: %v", appID, err)
		return fmt.Errorf("failed to start application runner: %w", err)
	}

	app.ContainerID = &containerID

	// 开始收集日志
	if m.logSystem != nil {
		labels := map[string]string{
			"app_id":   appID,
			"app_name": app.Name,
		}
		if err := m.logSystem.StartCollectingContainer(containerID, "runner", labels); err != nil {
			logrus.Warnf("Failed to start log collection for container %s: %v", containerID, err)
		}
	}

	logrus.Infof("Successfully started application %s with container ID %s", appID, containerID)

	return nil
}

// SaveFileContent 保存文件内容
func (m *Manager) SaveFileContent(appID, filePath, content string) error {
	return m.workspace.SaveFileContent(appID, filePath, content)
}

// CreateFile 创建新文件
func (m *Manager) CreateFile(appID, filePath string) error {
	return m.workspace.CreateFile(appID, filePath)
}

// DeleteFile 删除文件
func (m *Manager) DeleteFile(appID, filePath string) error {
	return m.workspace.DeleteFile(appID, filePath)
}

// CreateDirectory 创建目录
func (m *Manager) CreateDirectory(appID, dirPath string) error {
	return m.workspace.CreateDirectory(appID, dirPath)
}

// DeleteDirectory 删除目录
func (m *Manager) DeleteDirectory(appID, dirPath string) error {
	return m.workspace.DeleteDirectory(appID, dirPath)
}

// StopApplication 停止应用容器
func (m *Manager) StopApplication(appID string) error {
	app, err := m.GetApplication(appID)
	if err != nil {
		return fmt.Errorf("failed to get application: %w", err)
	}

	if app.ContainerID == nil {
		logrus.Infof("Application %s is not running (no container ID)", appID)
		return nil
	}

	logrus.Infof("Stopping application %s (container ID: %s)", appID, *app.ContainerID)

	// 停止日志收集
	if m.logSystem != nil {
		if err := m.logSystem.StopCollectingContainer(*app.ContainerID); err != nil {
			logrus.Warnf("Failed to stop log collection for container %s: %v", *app.ContainerID, err)
		}
	}

	// 委托给 runtime 服务停止容器
	if err := m.runtime.StopContainer(context.Background(), *app.ContainerID); err != nil {
		logrus.Errorf("Failed to stop container %s: %v", *app.ContainerID, err)
		return fmt.Errorf("failed to stop container: %w", err)
	}

	logrus.Infof("Successfully stopped container %s", *app.ContainerID)
	return nil
}

func (m *Manager) CreateApplication(createReq *request.CreateApplicationRequest) (*AppRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 从数据库获取下一个应用ID
	appID := util.GenIDWith("app.")

	// 创建应用专用的工作目录
	workspaceBaseDir := m.config.WorkspaceDir
	if workspaceBaseDir == "" {
		workspaceBaseDir = "./workspaces"
		logrus.Warn("WorkspaceDir not configured, using default: ./workspaces")
	}
	workDir := filepath.Join(workspaceBaseDir, appID)

	// 使用 workspace 服务克隆仓库
	if err := m.workspace.CloneRepository(appID, *createReq.GitUrl, *createReq.Branch, workDir); err != nil {
		return nil, err
	}

	app := &AppRef{
		ID:          appID,
		Name:        createReq.Name,
		Status:      StatusUndeployed,
		Components:  make(map[string]*Component),
		Type:        createReq.Type,
		GitUrl:      createReq.GitUrl,
		Branch:      createReq.Branch,
		Description: createReq.Description,
		Ports:       createReq.Ports,
		ContainerID: nil,
		HealthCheck: createReq.HealthCheck,
		ExecuteCmd:  createReq.ExecuteCmd,
		RunnerEnv:   createReq.RunnerEnv,
		CodeDir:     &workDir,
	}

	m.apps[appID] = app

	logrus.Infof("Application created in manager: ID=%s, Name=%s, Status=%s", appID, createReq.Name, app.Status)

	return app, nil
}

func (m *Manager) GetApplication(appID string) (*AppRef, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	app, ok := m.apps[appID]
	if !ok {
		return nil, fmt.Errorf("application %s not found", appID)
	}
	return app, nil
}

func (m *Manager) GetAllApplications() map[string]*AppRef {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.apps
}

// DeleteApplication 删除应用
func (m *Manager) DeleteApplication(appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查应用是否存在
	app, ok := m.apps[appID]
	if !ok {
		logrus.Warnf("Attempted to delete non-existent application: %s", appID)
		return fmt.Errorf("application %s not found", appID)
	}

	// 如果应用正在运行，先停止它
	if app.Status == StatusRunning {
		logrus.Infof("Stopping running application before deletion: %s", appID)
		// if err := m.StopApplicationComponents(appID); err != nil {
		// 	logrus.Warnf("Failed to stop application components during deletion: %v", err)
		// 	// 继续删除，即使停止失败
		// }
	}

	// 使用 workspace 服务清理工作目录
	if err := m.workspace.CleanWorkDir(appID); err != nil {
		logrus.Warnf("Failed to clean workspace for app %s: %v", appID, err)
	}

	// 从数据库删除应用
	delete(m.apps, appID)

	logrus.Infof("Application deleted successfully: ID=%s, Name=%s", appID, app.Name)

	return nil
}

// UpdateApplicationStatus 更新应用状态
func (m *Manager) UpdateApplicationStatus(appID string, status Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取应用以记录旧状态
	app, ok := m.apps[appID]
	if !ok {
		logrus.Warnf("Attempted to update status for non-existent application: %s", appID)
		return fmt.Errorf("application %s not found", appID)
	}

	oldStatus := app.Status

	// 更新数据库
	app.Status = status
	m.apps[appID] = app
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

	for _, app := range m.apps {

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

// GetFileTree 获取应用的文件树
func (m *Manager) GetFileTree(appID, path string) ([]FileInfo, error) {
	// 委托给 workspace 服务
	return m.workspace.GetFileTree(appID, path)
}

// GetFileContent 获取文件内容
func (m *Manager) GetFileContent(appID, filePath string) (string, string, error) {
	// 委托给 workspace 服务
	return m.workspace.GetFileContent(appID, filePath)
}

// GetApplicationLogs 获取应用的Docker容器日志
func (m *Manager) GetApplicationLogs(appID string, lines int) ([]string, error) {
	m.mu.RLock()
	app, ok := m.apps[appID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("application %s not found", appID)
	}

	// 如果应用没有容器ID，返回空日志
	if app.ContainerID == nil || *app.ContainerID == "" {
		return []string{
			"Application is not running in a container",
			"No logs available",
		}, nil
	}

	// 委托给 logger 服务
	return m.logger.GetLogs(context.Background(), *app.ContainerID, lines)
}

// GetApplicationLogsParsed 获取应用的解析后的Docker容器日志
func (m *Manager) GetApplicationLogsParsed(appID string, lines int) ([]*LogEntry, error) {
	m.mu.RLock()
	app, ok := m.apps[appID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("application %s not found", appID)
	}

	// 如果应用没有容器ID，返回默认消息
	if app.ContainerID == nil || *app.ContainerID == "" {
		return []*LogEntry{
			{
				ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
				Level:     "info",
				App:       app.Name,
				Message:   "Application is not running in a container",
				RawLine:   "Application is not running in a container",
			},
		}, nil
	}

	// 委托给 logger 服务
	return m.logger.GetLogsParsed(context.Background(), *app.ContainerID, app.Name, lines)
}
