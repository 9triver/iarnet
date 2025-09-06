package application

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/server/request"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	applications map[string]*AppRef
	nextAppID    int
	mu           sync.RWMutex
	config       *config.Config
	codeBrowsers map[string]*CodeBrowserInfo // appID -> 代码浏览器信息
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

func NewManager(config *config.Config) *Manager {
	return &Manager{
		applications: make(map[string]*AppRef),
		nextAppID:    1,
		config:       config,
		codeBrowsers: make(map[string]*CodeBrowserInfo),
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
	if _, err := os.Stat(requestPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("path not found: %s", requestPath)
	}

	// 读取目录内容
	files, err := ioutil.ReadDir(requestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %v", err)
	}

	// 构建文件信息列表
	var fileInfos []FileInfo
	for _, file := range files {
		relativePath := filepath.Join(path, file.Name())
		if path == "/" || path == "" {
			relativePath = file.Name()
		}

		fileInfo := FileInfo{
			Name:    file.Name(),
			Path:    relativePath,
			IsDir:   file.IsDir(),
			Size:    file.Size(),
			ModTime: file.ModTime().Format("2006-01-02 15:04:05"),
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
