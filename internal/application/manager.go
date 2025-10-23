package application

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/server/request"
	"github.com/9triver/iarnet/internal/websocket"
	"github.com/9triver/iarnet/proto"
	"github.com/9triver/ignis/platform"
	"github.com/9triver/ignis/proto/controller"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

// CodeAnalysisService 代码分析服务接口
type CodeAnalysisService interface {
	AnalyzeCode(ctx context.Context, req *proto.CodeAnalysisRequest) (*proto.CodeAnalysisResponse, error)
}

// LogEntry 表示解析后的日志条目
type LogEntry struct {
	ID        string `json:"id"`        // 日志条目唯一标识
	Timestamp string `json:"timestamp"` // 时间戳
	Level     string `json:"level"`     // 日志级别 (error, warn, info, debug)
	App       string `json:"app"`       // 应用名称
	Message   string `json:"message"`   // 日志消息
	Details   string `json:"details"`   // 详细信息（可选）
	RawLine   string `json:"raw_line"`  // 原始日志行
}

type Manager struct {
	applications    map[string]*AppRef
	nextAppID       int
	mu              sync.RWMutex
	config          *config.Config
	codeBrowsers    map[string]*CodeBrowserInfo // appID -> 代码浏览器信息
	rm              *resource.Manager
	analysisService CodeAnalysisService
	ignisPlatform   *platform.Platform
	dockerClient    *client.Client
	wsHub           *websocket.Hub // WebSocket hub for real-time updates
}

// OnDAGStateChanged 实现StateChangeObserver接口
func (m *Manager) OnDAGStateChanged(event *platform.DAGStateChangeEvent) {
	logrus.Infof("DAG state changed for app %s, node %s", event.AppID, event.NodeID)

	// 通过WebSocket广播DAG状态变化
	if m.wsHub != nil {
		wsEvent := websocket.DAGStateEvent{
			Type:          "dag_node_state_change",
			ApplicationID: event.AppID,
			NodeID:        event.NodeID,
			NodeState:     "done", // 节点完成状态
			Timestamp:     event.Timestamp.Unix(),
			Data: map[string]interface{}{
				"node_type": event.NodeState.Type,
				"done":      event.NodeState.Done,
				"ready":     event.NodeState.Ready,
			},
		}
		m.wsHub.BroadcastDAGStateChange(wsEvent)
	}
}

// CodeBrowserInfo 代码浏览器信息
type CodeBrowserInfo struct {
	Port      int       `json:"port"`
	PID       int       `json:"pid"`
	StartTime time.Time `json:"start_time"`
	Status    string    `json:"status"` // running, stopped
	WorkDir   string    `json:"work_dir"`
	Cmd       *exec.Cmd `json:"-"`
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

	m := &Manager{
		applications:  make(map[string]*AppRef),
		nextAppID:     1,
		config:        config,
		codeBrowsers:  make(map[string]*CodeBrowserInfo),
		rm:            resourceManager,
		ignisPlatform: nil,
		dockerClient:  cli,
		wsHub:         wsHub,
	}

	return m
}

func (m *Manager) SetIgnisPlatform(ignisPlatform *platform.Platform) {
	m.ignisPlatform = ignisPlatform
	logrus.Info("Ignis platform set in application manager")
}

// GetWebSocketHub returns the WebSocket hub for external use
func (m *Manager) GetWebSocketHub() *websocket.Hub {
	return m.wsHub
}

func (m *Manager) RegisterComponent(appID string, name string, cf *resource.ContainerRef) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.applications[appID]
	if !ok {
		return fmt.Errorf("application %s not found", appID)
	}

	app.Components[name] = &Component{
		Name:         name,
		Image:        cf.Spec.Image,
		Status:       ComponentStatusRunning, // TODO: 健康检查
		CreatedAt:    time.Now(),
		DeployedAt:   time.Now(),
		UpdatedAt:    time.Now(),
		ContainerRef: cf,
	}

	return nil
}

func (m *Manager) Stop() {
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

	appRef, ok := m.applications[appID]
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

	return nil
}

// SetAnalysisService 设置代码分析服务
func (m *Manager) SetAnalysisService(service CodeAnalysisService) {
	m.analysisService = service
}

// RunApplication 运行应用容器
func (m *Manager) RunApplication(appID string) error {
	app, err := m.GetApplication(appID)
	if err != nil {
		return fmt.Errorf("failed to get application: %w", err)
	}

	if app.ContainerID == nil {
		logrus.Infof("Application %s has not created container", appID)
	} else {
		logrus.Infof("Application %s has container ID %s", appID, *app.ContainerID)
		err = m.dockerClient.ContainerRemove(context.Background(), *app.ContainerID, container.RemoveOptions{
			Force: true,
		})
		if err != nil {
			logrus.Errorf("Failed to remove container %s: %v", *app.ContainerID, err)
			return fmt.Errorf("failed to remove container: %w", err)
		} else {
			logrus.Infof("Successfully removed container %s", *app.ContainerID)
		}
	}

	if app.RunnerEnv == nil {
		return fmt.Errorf("application %s has no runner image", appID)
	}

	runnerImage, ok := m.config.RunnerImages[*app.RunnerEnv]
	if !ok || runnerImage == "" {
		return fmt.Errorf("application %s has no runner image", appID)
	}

	hostPath, err := filepath.Abs(*app.CodeDir)
	if err != nil {
		return err
	}

	// 创建容器
	containerID, err := m.dockerClient.ContainerCreate(context.TODO(), &container.Config{
		Image: runnerImage,
		Env:   []string{"APP_ID=" + appID, "IGNIS_PORT=" + strconv.FormatInt(int64(m.config.Ignis.Port), 10), "EXECUTE_CMD=" + *app.ExecuteCmd},
	}, &container.HostConfig{
		Binds: []string{
			hostPath + ":/iarnet/app", // 将宿主机的 app.CodeDir 挂载到容器的 /iarnet/app
		},
		ExtraHosts: []string{
			"host.internal:host-gateway",
		},
	}, nil, nil, "")

	if err != nil {
		logrus.Errorf("Failed to create container for application %s: %v", appID, err)
		return fmt.Errorf("failed to create container: %w", err)
	}

	// 启动容器
	if err := m.dockerClient.ContainerStart(context.TODO(), containerID.ID, container.StartOptions{}); err != nil {
		logrus.Errorf("Failed to start container %s: %v", containerID.ID, err)
		return fmt.Errorf("failed to start container: %w", err)
	}

	app.ContainerID = &containerID.ID

	return nil
}

// SaveFileContent 保存文件内容
func (m *Manager) SaveFileContent(appID, filePath, content string) error {
	m.mu.RLock()
	app, exists := m.applications[appID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	// 构建完整的文件路径
	fullPath := filepath.Join(*app.CodeDir, filePath)

	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// 写入文件内容
	if err := ioutil.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	logrus.Infof("Saved file: %s for application %s", filePath, appID)
	return nil
}

// CreateFile 创建新文件
func (m *Manager) CreateFile(appID, filePath string) error {
	m.mu.RLock()
	app, exists := m.applications[appID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	// 构建完整的文件路径
	fullPath := filepath.Join(*app.CodeDir, filePath)

	// 检查文件是否已存在
	if _, err := os.Stat(fullPath); err == nil {
		return fmt.Errorf("file already exists: %s", filePath)
	}

	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// 创建空文件
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	file.Close()

	logrus.Infof("Created file: %s for application %s", filePath, appID)
	return nil
}

// DeleteFile 删除文件
func (m *Manager) DeleteFile(appID, filePath string) error {
	m.mu.RLock()
	app, exists := m.applications[appID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	// 构建完整的文件路径
	fullPath := filepath.Join(*app.CodeDir, filePath)

	// 检查文件是否存在
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// 确保是文件而不是目录
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	// 删除文件
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file: %v", err)
	}

	logrus.Infof("Deleted file: %s for application %s", filePath, appID)
	return nil
}

// CreateDirectory 创建目录
func (m *Manager) CreateDirectory(appID, dirPath string) error {
	m.mu.RLock()
	app, exists := m.applications[appID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	// 构建完整的目录路径
	fullPath := filepath.Join(*app.CodeDir, dirPath)

	// 检查目录是否已存在
	if _, err := os.Stat(fullPath); err == nil {
		return fmt.Errorf("directory already exists: %s", dirPath)
	}

	// 创建目录
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	logrus.Infof("Created directory: %s for application %s", dirPath, appID)
	return nil
}

// DeleteDirectory 删除目录
func (m *Manager) DeleteDirectory(appID, dirPath string) error {
	m.mu.RLock()
	app, exists := m.applications[appID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	// 构建完整的目录路径
	fullPath := filepath.Join(*app.CodeDir, dirPath)

	// 检查目录是否存在
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("directory not found: %s", dirPath)
	}

	// 确保是目录而不是文件
	if !info.IsDir() {
		return fmt.Errorf("path is a file, not a directory: %s", dirPath)
	}

	// 删除目录及其内容
	if err := os.RemoveAll(fullPath); err != nil {
		return fmt.Errorf("failed to delete directory: %v", err)
	}

	logrus.Infof("Deleted directory: %s for application %s", dirPath, appID)
	return nil
}

// StopApplication 停止应用容器
func (m *Manager) StopApplication(appID string) error {
	app, err := m.GetApplication(appID)
	if err != nil {
		return fmt.Errorf("failed to get application: %w", err)
	}

	if app.ContainerID == nil {
		logrus.Infof("Application %s is not running", appID)
	} else {
		logrus.Infof("Application %s is running, container ID: %s", appID, *app.ContainerID)
		err = m.dockerClient.ContainerStop(context.Background(), *app.ContainerID, container.StopOptions{})
		if err != nil {
			logrus.Errorf("Failed to stop container %s: %v", *app.ContainerID, err)
			return fmt.Errorf("failed to stop container: %w", err)
		} else {
			logrus.Infof("Successfully stopped container %s", *app.ContainerID)
		}
	}

	return nil
}

func (m *Manager) CreateApplication(createReq *request.CreateApplicationRequest) (*AppRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	appID := strconv.Itoa(m.nextAppID)
	m.nextAppID++

	// 从配置中获取工作目录，如果未配置则使用默认值
	workspaceBaseDir := m.config.WorkspaceDir
	if workspaceBaseDir == "" {
		workspaceBaseDir = "./workspaces"
		logrus.Warn("WorkspaceDir not configured, using default: ./workspaces")
	}

	// 创建应用专用的工作目录
	workDir := filepath.Join(workspaceBaseDir, appID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %v", err)
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
	m.applications[appID] = app
	logrus.Infof("Application created in manager: ID=%s, Name=%s, Status=%s", appID, createReq.Name, app.Status)

	logrus.Infof("Cloning repository %s (branch: %s) to %s", *createReq.GitUrl, *createReq.Branch, workDir)

	// 执行git clone命令
	cmd := exec.Command("git", "clone", "-b", *createReq.Branch, "--single-branch", *createReq.GitUrl, workDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// 如果克隆失败，清理目录
		os.RemoveAll(workDir)
		return nil, fmt.Errorf("failed to clone repository: %v", err)
	}

	logrus.Infof("Successfully cloned repository to %s", workDir)

	return app, nil
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

// DeleteApplication 删除应用
func (m *Manager) DeleteApplication(appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查应用是否存在
	app, ok := m.applications[appID]
	if !ok {
		logrus.Warnf("Attempted to delete non-existent application: %s", appID)
		return errors.New("application not found")
	}

	// 如果应用正在运行，先停止它
	if app.Status == StatusRunning {
		logrus.Infof("Stopping running application before deletion: %s", appID)
		// if err := m.StopApplicationComponents(appID); err != nil {
		// 	logrus.Warnf("Failed to stop application components during deletion: %v", err)
		// 	// 继续删除，即使停止失败
		// }
	}

	// 停止代码浏览器（如果正在运行）
	if _, exists := m.codeBrowsers[appID]; exists {
		logrus.Infof("Stopping code browser for application: %s", appID)
		if err := m.StopCodeBrowser(appID); err != nil {
			logrus.Warnf("Failed to stop code browser during deletion: %v", err)
		}
		delete(m.codeBrowsers, appID)
	}

	// 删除应用目录（如果存在）
	workspaceDir := m.config.WorkspaceDir
	if workspaceDir == "" {
		workspaceDir = "./workspaces"
	}
	appDir := filepath.Join(workspaceDir, appID)
	if _, err := os.Stat(appDir); err == nil {
		if err := os.RemoveAll(appDir); err != nil {
			logrus.Warnf("Failed to remove application directory %s: %v", appDir, err)
			// 继续删除，即使目录删除失败
		}
	}

	// 从内存中删除应用
	delete(m.applications, appID)
	logrus.Infof("Application deleted successfully: ID=%s, Name=%s", appID, app.Name)

	return nil
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

// StartCodeBrowser 启动代码浏览器
func (m *Manager) StartCodeBrowser(appID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查应用是否存在
	_, exists := m.applications[appID]
	if !exists {
		return 0, errors.New("application not found")
	}

	// 检查是否已经有代码浏览器在运行
	if browserInfo, exists := m.codeBrowsers[appID]; exists && browserInfo.Status == "running" {
		return browserInfo.Port, nil
	}

	// 获取应用的工作目录
	workspaceBaseDir := m.config.WorkspaceDir
	if workspaceBaseDir == "" {
		workspaceBaseDir = "./workspaces"
	}
	workDir := filepath.Join(workspaceBaseDir, appID)

	// 检查工作目录是否存在
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		return 0, fmt.Errorf("workspace directory does not exist: %s", workDir)
	}

	// 找到可用端口
	port, err := m.findAvailablePort()
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %v", err)
	}

	// 启动code-server
	cmd := exec.Command("code-server",
		"--bind-addr", fmt.Sprintf("0.0.0.0:%d", port),
		"--auth", "none",
		"--disable-telemetry",
		workDir,
	)

	err = cmd.Start()
	if err != nil {
		return 0, fmt.Errorf("failed to start code-server: %v", err)
	}

	// 记录代码浏览器信息
	m.codeBrowsers[appID] = &CodeBrowserInfo{
		Port:      port,
		PID:       cmd.Process.Pid,
		StartTime: time.Now(),
		Status:    "running",
		WorkDir:   workDir,
		Cmd:       cmd,
	}

	logrus.Infof("Started code browser for app %s on port %d", appID, port)
	return port, nil
}

// StopCodeBrowser 停止代码浏览器
func (m *Manager) StopCodeBrowser(appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	browserInfo, exists := m.codeBrowsers[appID]
	if !exists {
		return errors.New("code browser not found")
	}

	if browserInfo.Status != "running" {
		return errors.New("code browser is not running")
	}

	// 停止进程
	if browserInfo.Cmd != nil && browserInfo.Cmd.Process != nil {
		err := browserInfo.Cmd.Process.Kill()
		if err != nil {
			logrus.Errorf("Failed to kill code browser process: %v", err)
			return err
		}
	}

	// 更新状态
	browserInfo.Status = "stopped"
	logrus.Infof("Stopped code browser for app %s", appID)
	return nil
}

// GetCodeBrowserStatus 获取代码浏览器状态
func (m *Manager) GetCodeBrowserStatus(appID string) (*CodeBrowserInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	browserInfo, exists := m.codeBrowsers[appID]
	if !exists {
		return &CodeBrowserInfo{
			Status: "stopped",
		}, nil
	}

	// 检查进程是否还在运行
	if browserInfo.Cmd != nil && browserInfo.Cmd.Process != nil {
		// 尝试发送信号0来检查进程是否存在
		err := browserInfo.Cmd.Process.Signal(os.Signal(nil))
		if err != nil {
			// 进程不存在，更新状态
			browserInfo.Status = "stopped"
		}
	}

	return browserInfo, nil
}

// findAvailablePort 找到可用端口
func (m *Manager) findAvailablePort() (int, error) {
	// 从8080开始查找可用端口
	for port := 8080; port <= 9000; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, errors.New("no available port found")
}

// FileInfo 文件信息结构
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

// GetFileTree 获取应用的文件树
func (m *Manager) GetFileTree(appID, path string) ([]FileInfo, error) {
	// 获取应用信息
	_, err := m.GetApplication(appID)
	if err != nil {
		return nil, fmt.Errorf("application not found: %v", err)
	}

	// 获取工作目录
	workspaceDir := m.config.WorkspaceDir
	if workspaceDir == "" {
		workspaceDir = "./workspaces"
	}

	// 构建完整路径
	workDir := filepath.Join(workspaceDir, appID)
	requestPath := filepath.Join(workDir, path)

	// 安全检查：确保路径在工作目录内
	if !strings.HasPrefix(requestPath, workDir) {
		return nil, errors.New("invalid path: outside workspace")
	}

	// 检查路径是否存在
	if _, err = os.Stat(requestPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("path not found: %s", requestPath)
	}

	// 读取目录内容
	files, err := os.ReadDir(requestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %v", err)
	}

	// 构建文件信息列表
	var fileInfos []FileInfo
	for _, file := range files {
		if file.Name() == ".git" {
			continue
		}

		relativePath := filepath.Join(path, file.Name())
		if path == "/" || path == "" {
			relativePath = file.Name()
		}

		// 获取文件信息,处理可能的错误
		info, err := file.Info()
		var size int64
		var modTime string
		if err != nil {
			size = 0 // 如果获取失败则设为0
			modTime = ""
		} else {
			size = info.Size()
			modTime = info.ModTime().Format("2006-01-02 15:04:05")
		}

		fileInfo := FileInfo{
			Name:    file.Name(),
			Path:    relativePath,
			IsDir:   file.IsDir(),
			Size:    size,
			ModTime: modTime,
		}
		fileInfos = append(fileInfos, fileInfo)
	}

	return fileInfos, nil
}

// GetFileContent 获取文件内容
func (m *Manager) GetFileContent(appID, filePath string) (string, string, error) {
	// 获取应用信息
	_, err := m.GetApplication(appID)
	if err != nil {
		return "", "", fmt.Errorf("application not found: %v", err)
	}

	// 获取工作目录
	workspaceDir := m.config.WorkspaceDir
	if workspaceDir == "" {
		workspaceDir = "./workspaces"
	}

	// 构建完整路径
	workDir := filepath.Join(workspaceDir, appID)
	requestPath := filepath.Join(workDir, filePath)

	// 安全检查：确保路径在工作目录内
	if !strings.HasPrefix(requestPath, workDir) {
		return "", "", errors.New("invalid path: outside workspace")
	}

	// 检查文件是否存在
	fileInfo, err := os.Stat(requestPath)
	if os.IsNotExist(err) {
		return "", "", fmt.Errorf("file not found: %s", requestPath)
	}
	if fileInfo.IsDir() {
		return "", "", errors.New("path is a directory, not a file")
	}

	// 检查文件大小（限制为10MB）
	if fileInfo.Size() > 10*1024*1024 {
		return "", "", errors.New("file too large (max 10MB)")
	}

	// 读取文件内容
	content, err := ioutil.ReadFile(requestPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read file: %v", err)
	}

	// 检测语言类型
	ext := filepath.Ext(filePath)
	language := m.detectLanguage(ext)

	return string(content), language, nil
}

// detectLanguage 根据文件扩展名检测语言类型
func (m *Manager) detectLanguage(ext string) string {
	langMap := map[string]string{
		".js":    "javascript",
		".jsx":   "javascript",
		".ts":    "typescript",
		".tsx":   "typescript",
		".py":    "python",
		".go":    "go",
		".java":  "java",
		".c":     "c",
		".cpp":   "cpp",
		".h":     "c",
		".hpp":   "cpp",
		".cs":    "csharp",
		".php":   "php",
		".rb":    "ruby",
		".rs":    "rust",
		".kt":    "kotlin",
		".swift": "swift",
		".html":  "html",
		".css":   "css",
		".scss":  "scss",
		".sass":  "sass",
		".less":  "less",
		".json":  "json",
		".xml":   "xml",
		".yaml":  "yaml",
		".yml":   "yaml",
		".toml":  "toml",
		".ini":   "ini",
		".sh":    "shell",
		".bash":  "shell",
		".zsh":   "shell",
		".fish":  "shell",
		".ps1":   "powershell",
		".sql":   "sql",
		".md":    "markdown",
		".txt":   "plaintext",
	}

	if lang, exists := langMap[strings.ToLower(ext)]; exists {
		return lang
	}
	return "plaintext"
}

// readApplicationCode 读取应用代码内容
func (m *Manager) readApplicationCode(appDir string) (string, error) {
	// 简化实现：读取主要文件内容
	var codeContent strings.Builder

	err := filepath.Walk(appDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过隐藏文件和目录
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// 只读取代码文件
		if !info.IsDir() && m.isCodeFile(info.Name()) {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			codeContent.WriteString(fmt.Sprintf("// File: %s\n%s\n\n", path, string(content)))
		}

		return nil
	})

	return codeContent.String(), err
}

// isCodeFile 判断是否为代码文件
func (m *Manager) isCodeFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	codeExts := []string{".js", ".ts", ".py", ".go", ".java", ".cpp", ".c", ".php", ".rb", ".cs", ".json", ".yaml", ".yml", ".dockerfile"}

	for _, codeExt := range codeExts {
		if ext == codeExt {
			return true
		}
	}
	return false
}

// detectLanguageFromCode 从代码内容检测编程语言
func (m *Manager) detectLanguageFromCode(codeContent string) string {
	content := strings.ToLower(codeContent)

	if strings.Contains(content, "package.json") || strings.Contains(content, "npm") || strings.Contains(content, "node_modules") {
		return "javascript"
	}
	if strings.Contains(content, "requirements.txt") || strings.Contains(content, "import ") && strings.Contains(content, "def ") {
		return "python"
	}
	if strings.Contains(content, "go.mod") || strings.Contains(content, "package main") {
		return "go"
	}
	if strings.Contains(content, "pom.xml") || strings.Contains(content, "public class") {
		return "java"
	}

	return "unknown"
}

// detectFrameworkFromCode 从代码内容检测框架
func (m *Manager) detectFrameworkFromCode(codeContent string) string {
	content := strings.ToLower(codeContent)

	if strings.Contains(content, "next.js") || strings.Contains(content, "next/") {
		return "next.js"
	}
	if strings.Contains(content, "react") {
		return "react"
	}
	if strings.Contains(content, "django") {
		return "django"
	}
	if strings.Contains(content, "flask") {
		return "flask"
	}
	if strings.Contains(content, "spring") {
		return "spring"
	}

	return "unknown"
}

// getAvailableProviders 获取可用的资源提供者
func (m *Manager) getAvailableProviders() []*proto.ProviderInfo {
	providers := m.rm.GetProviders()
	var protoProviders []*proto.ProviderInfo

	// 转换本地提供者（包含内部和外部托管）
	for _, provider := range providers.LocalProviders {
		protoProviders = append(protoProviders, &proto.ProviderInfo{
			Id:     provider.GetID(),
			Name:   provider.GetName(),
			Type:   string(provider.GetType()),
			Status: 1, // 假设状态为活跃
		})
	}

	// 转换远程提供者（通过协作发现）
	for _, provider := range providers.RemoteProviders {
		protoProviders = append(protoProviders, &proto.ProviderInfo{
			Id:     provider.GetID(),
			Name:   provider.GetName(),
			Type:   string(provider.GetType()),
			Status: 1,
		})
	}

	return protoProviders
}

// GetApplicationDAG 获取应用的DAG图
func (m *Manager) GetApplicationDAG(appID string) (*DAG, error) {
	// m.mu.RLock()
	// defer m.mu.RUnlock()

	// 获取ApplicationInfo并注册观察者
	appInfo := m.ignisPlatform.GetApplicationInfo(appID)
	if appInfo == nil {
		return nil, errors.New("application info not found")
	}

	// 注册观察者（如果还没有注册的话）
	appInfo.AddObserver(m)

	ignisDAG := appInfo.GetDAG()
	if ignisDAG == nil {
		return nil, errors.New("application DAG not found")
	}

	return m.ConvertToApplicationDAG(ignisDAG, appInfo), nil
}

func (m *Manager) ConvertToApplicationDAG(dag *controller.DAG, appInfo *platform.ApplicationInfo) *DAG {
	protoNodes := dag.GetNodes()

	protoDataNodeMap := make(map[string]*controller.DataNode)

	// 获取最新的节点状态
	nodeStates := appInfo.GetNodeStates()

	// 转换节点
	var appNodes []*DAGNode

	for _, protoNode := range protoNodes {
		var appNode *DAGNode

		if protoNode.GetType() == "ControlNode" {
			controlNode := protoNode.GetControlNode()
			nodeID := controlNode.GetId()
			
			// 从最新状态获取信息
			var done bool
			var lastUpdated time.Time
			if nodeState, exists := nodeStates[nodeID]; exists {
				done = nodeState.Done
				lastUpdated = nodeState.UpdateAt
			} else {
				done = controlNode.GetDone()
				lastUpdated = time.Now()
			}
			
			appNode = &DAGNode{
				Type: "ControlNode",
				Node: &ControlNode{
					Id:           nodeID,
					Done:         done,
					FunctionName: controlNode.GetFunctionName(),
					Params:       controlNode.GetParams(),
					Current:      controlNode.GetCurrent(),
					LastUpdated:  lastUpdated,
				},
			}
		} else if protoNode.GetType() == "DataNode" {
			dataNode := protoNode.GetDataNode()
			nodeID := dataNode.GetId()
			
			// 从最新状态获取信息
			var done, ready bool
			var lastUpdated time.Time
			if nodeState, exists := nodeStates[nodeID]; exists {
				done = nodeState.Done
				ready = nodeState.Ready
				lastUpdated = nodeState.UpdateAt
			} else {
				done = dataNode.GetDone()
				ready = dataNode.GetReady()
				lastUpdated = time.Now()
			}
			
			appDataNode := &DataNode{
				Id:          nodeID,
				Done:        done,
				Lambda:      dataNode.GetLambda(),
				Ready:       ready,
				ChildNode:   dataNode.GetChildNode(),
				LastUpdated: lastUpdated,
			}

			// 处理可选字段
			if dataNode.GetParentNode() != "" {
				parentNode := dataNode.GetParentNode()
				appDataNode.ParentNode = &parentNode
			}

			appNode = &DAGNode{
				Type: "DataNode",
				Node: appDataNode,
			}

			protoDataNodeMap[dataNode.GetId()] = dataNode
		}

		if appNode != nil {
			appNodes = append(appNodes, appNode)
		}
	}

	// 构建边
	var edges []*DAGEdge

	for _, protoNode := range protoNodes {
		if protoNode.GetType() == "DataNode" {
			continue
		}
		controlNode := protoNode.GetControlNode()

		// 从PreDataNodes到ControlNode的边
		for _, preDataNodeId := range controlNode.GetPreDataNodes() {
			// 查找对应的参数名
			preDataNode := protoDataNodeMap[preDataNodeId]
			if preDataNode == nil {
				logrus.Errorf("PreDataNode %s not found for ControlNode %s", preDataNodeId, controlNode.GetId())
				continue
			}

			var info string
			lambdaId := preDataNode.GetLambda()
			if controlNode.GetParams() != nil {
				paramName, ok := controlNode.GetParams()[lambdaId]
				if !ok {
					logrus.Errorf("Param for lambda %s not found for ControlNode %s", lambdaId, controlNode.GetId())
					continue
				}
				info = paramName
			}

			edge := &DAGEdge{
				FromNodeID: preDataNodeId,
				ToNodeID:   controlNode.GetId(),
				Info:       info,
			}
			edges = append(edges, edge)
		}

		// 从ControlNode到DataNode的边
		if controlNode.GetDataNode() != "" {
			edge := &DAGEdge{
				FromNodeID: controlNode.GetId(),
				ToNodeID:   controlNode.GetDataNode(),
				Info:       "", // 控制节点到数据节点的边通常没有信息
			}
			edges = append(edges, edge)
		}
	}

	return &DAG{
		Nodes: appNodes,
		Edges: edges,
	}
}

// GetApplicationLogs 获取应用的Docker容器日志
func (m *Manager) GetApplicationLogs(appID string, lines int) ([]string, error) {
	m.mu.RLock()
	app, exists := m.applications[appID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("application %s not found", appID)
	}

	// 如果应用没有容器ID，返回空日志
	if app.ContainerID == nil || *app.ContainerID == "" {
		return []string{
			"Application is not running in a container",
			"No logs available",
		}, nil
	}

	// 直接使用Docker client获取日志
	logs, err := m.getDockerLogs(*app.ContainerID, lines)
	if err != nil {
		logrus.Errorf("Failed to get logs for container %s: %v", *app.ContainerID, err)
		return []string{
			fmt.Sprintf("Error retrieving logs: %v", err),
		}, nil
	}

	return logs, nil
}

// GetApplicationLogsParsed 获取应用的解析后的Docker容器日志
func (m *Manager) GetApplicationLogsParsed(appID string, lines int) ([]*LogEntry, error) {
	m.mu.RLock()
	app, exists := m.applications[appID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("application %s not found", appID)
	}

	// 如果应用没有容器ID，返回空日志
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
			{
				ID:        fmt.Sprintf("%d", time.Now().UnixNano()+1),
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
				Level:     "info",
				App:       app.Name,
				Message:   "No logs available",
				RawLine:   "No logs available",
			},
		}, nil
	}

	// 直接使用Docker client获取日志
	rawLogs, err := m.getDockerLogs(*app.ContainerID, lines)
	if err != nil {
		logrus.Errorf("Failed to get logs for container %s: %v", *app.ContainerID, err)
		return []*LogEntry{
			{
				ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
				Level:     "error",
				App:       app.Name,
				Message:   fmt.Sprintf("Error retrieving logs: %v", err),
				RawLine:   fmt.Sprintf("Error retrieving logs: %v", err),
			},
		}, nil
	}

	// 解析每一行日志
	var parsedLogs []*LogEntry
	for _, line := range rawLogs {
		if strings.TrimSpace(line) != "" {
			parsedLog := m.parseDockerLogLine(line, app.Name)
			parsedLogs = append(parsedLogs, parsedLog)
		}
	}

	return parsedLogs, nil
}

// getDockerLogs 直接使用Docker client获取容器日志
func (m *Manager) getDockerLogs(containerID string, lines int) ([]string, error) {
	if m.dockerClient == nil {
		return []string{"Docker client not available"}, nil
	}

	ctx := context.Background()

	// 设置日志选项
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       strconv.Itoa(lines),
		Timestamps: true,
	}

	// 获取容器日志
	logs, err := m.dockerClient.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %v", err)
	}
	defer logs.Close()

	// 读取日志内容
	logBytes, err := ioutil.ReadAll(logs)
	if err != nil {
		return nil, fmt.Errorf("failed to read logs: %v", err)
	}

	// 将日志按行分割
	logLines := strings.Split(string(logBytes), "\n")

	// 过滤空行
	var filteredLogs []string
	for _, line := range logLines {
		if strings.TrimSpace(line) != "" {
			filteredLogs = append(filteredLogs, line)
		}
	}

	return filteredLogs, nil
}

// parseDockerLogLine 解析单行Docker日志
func (m *Manager) parseDockerLogLine(line, appName string) *LogEntry {
	// 生成唯一ID
	id := fmt.Sprintf("%d", time.Now().UnixNano())

	// Docker日志格式通常为: 2024-01-15T10:30:45.123456789Z message
	// 或者带有stream前缀: stdout/stderr 2024-01-15T10:30:45.123456789Z message

	// 移除Docker stream前缀 (stdout/stderr)
	cleanLine := line
	if strings.HasPrefix(line, "stdout ") || strings.HasPrefix(line, "stderr ") {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) > 1 {
			cleanLine = parts[1]
		}
	}

	// 使用正则表达式匹配时间戳，支持带有前缀字符的格式
	// 匹配格式: [可选前缀字符]YYYY-MM-DDTHH:MM:SS[.微秒][Z] 消息内容
	timestampRegex := regexp.MustCompile(`^[^0-9]*(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?)\s+(.*)$`)
	matches := timestampRegex.FindStringSubmatch(cleanLine)

	var timestamp, message string
	if len(matches) >= 3 {
		timestamp = matches[1]
		message = matches[2]
	} else {
		// 如果没有匹配到时间戳，使用当前时间
		timestamp = time.Now().Format(time.RFC3339)
		message = cleanLine
	}

	// 解析结构化日志和提取msg内容
	parsedMessage, level := m.parseStructuredLog(message)

	// 如果没有从结构化日志中检测到级别，使用通用检测
	if level == "" {
		level = m.detectLogLevel(parsedMessage)
	}

	// 格式化时间戳为更友好的格式
	if parsedTime, err := time.Parse(time.RFC3339, timestamp); err == nil {
		timestamp = parsedTime.Format("2006-01-02 15:04:05")
	}

	return &LogEntry{
		ID:        id,
		Timestamp: timestamp,
		Level:     level,
		App:       appName,
		Message:   parsedMessage,
		RawLine:   line,
	}
}

// parseStructuredLog 解析结构化日志，提取msg内容和级别
func (m *Manager) parseStructuredLog(message string) (string, string) {
	// 检查是否为结构化日志格式: time="..." level=... msg="..."
	structuredRegex := regexp.MustCompile(`time="[^"]*"\s+level=(\w+)\s+msg="([^"]*)"`)
	matches := structuredRegex.FindStringSubmatch(message)

	if len(matches) >= 3 {
		level := strings.ToLower(matches[1])
		msg := matches[2]
		return msg, level
	}

	// 检查其他可能的结构化格式: level=... msg="..."
	altStructuredRegex := regexp.MustCompile(`level=(\w+)\s+msg="([^"]*)"`)
	altMatches := altStructuredRegex.FindStringSubmatch(message)

	if len(altMatches) >= 3 {
		level := strings.ToLower(altMatches[1])
		msg := altMatches[2]
		return msg, level
	}

	// 对于非结构化日志，检查是否有内嵌的时间戳前缀需要移除
	// 格式如: 2025-09-26T13:28:51.152575929Z hello world
	cleanedMessage := m.removeEmbeddedTimestamp(message)

	return cleanedMessage, ""
}

// removeEmbeddedTimestamp 移除消息中嵌入的时间戳前缀
func (m *Manager) removeEmbeddedTimestamp(message string) string {
	// 匹配各种时间戳格式并移除，包括前面可能的乱码字符
	timestampPatterns := []string{
		// 带乱码前缀和+的ISO 8601格式: [乱码]+2025-09-26T13:39:04.056588557Z hello world
		`^[^\d]*\+?\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?\s+`,
		// 带乱码前缀的ISO 8601格式: [乱码]2025-09-26T13:28:51.152575929Z
		`^[^\d]*\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?\s+`,
		// 简单日期时间格式: 2025-09-26 13:28:51
		`^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?\s+`,
		// 其他常见格式
		`^\[\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\]\s+`,
	}

	for _, pattern := range timestampPatterns {
		regex := regexp.MustCompile(pattern)
		if regex.MatchString(message) {
			return regex.ReplaceAllString(message, "")
		}
	}

	return message
}

// detectLogLevel 从日志消息中检测日志级别
func (m *Manager) detectLogLevel(message string) string {
	messageLower := strings.ToLower(message)

	// 检查常见的日志级别关键词
	if strings.Contains(messageLower, "error") || strings.Contains(messageLower, "err") ||
		strings.Contains(messageLower, "fatal") || strings.Contains(messageLower, "panic") ||
		strings.Contains(messageLower, "exception") || strings.Contains(messageLower, "failed") {
		return "error"
	}

	if strings.Contains(messageLower, "warn") || strings.Contains(messageLower, "warning") {
		return "warn"
	}

	if strings.Contains(messageLower, "debug") || strings.Contains(messageLower, "trace") {
		return "debug"
	}

	// 检查日志级别标记 [ERROR], [WARN], [INFO], [DEBUG]
	levelRegex := regexp.MustCompile(`\[(ERROR|WARN|WARNING|INFO|DEBUG|TRACE)\]`)
	if matches := levelRegex.FindStringSubmatch(message); len(matches) > 1 {
		switch strings.ToLower(matches[1]) {
		case "error":
			return "error"
		case "warn", "warning":
			return "warn"
		case "debug", "trace":
			return "debug"
		case "info":
			return "info"
		}
	}

	// 默认为info级别
	return "info"
}
