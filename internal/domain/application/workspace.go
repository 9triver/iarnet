package application

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// WorkspaceService 工作空间服务接口
type WorkspaceService interface {
	// Git 仓库管理
	CloneRepository(appID, gitURL, branch, workDir string) error
	PullRepository(appID string) error

	// 文件操作
	GetFileTree(appID, path string) ([]FileInfo, error)
	GetFileContent(appID, filePath string) (string, string, error)
	SaveFileContent(appID, filePath, content string) error
	CreateFile(appID, filePath string) error
	DeleteFile(appID, filePath string) error

	// 目录操作
	CreateDirectory(appID, dirPath string) error
	DeleteDirectory(appID, dirPath string) error

	// 工作目录管理
	GetWorkDir(appID string) string
	SetWorkDir(appID, workDir string)
	CleanWorkDir(appID string) error
}

// workspace 工作空间服务实现
type workspace struct {
	baseDir  string
	workDirs map[string]string // appID -> workDir
	mu       sync.RWMutex
}

// NewWorkspace 创建工作空间服务
func NewWorkspace(baseDir string) WorkspaceService {
	if baseDir == "" {
		baseDir = "./workspaces"
	}

	// 确保基础目录存在
	os.MkdirAll(baseDir, 0755)

	return &workspace{
		baseDir:  baseDir,
		workDirs: make(map[string]string),
	}
}

// CloneRepository 克隆 Git 仓库
func (w *workspace) CloneRepository(appID, gitURL, branch, workDir string) error {
	// 记录工作目录
	w.mu.Lock()
	w.workDirs[appID] = workDir
	w.mu.Unlock()

	// 执行 git clone
	cmd := exec.Command("git", "clone", "-b", branch, "--single-branch", gitURL, workDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		os.RemoveAll(workDir)
		return fmt.Errorf("failed to clone repository: %v", err)
	}

	logrus.Infof("Successfully cloned repository to %s", workDir)
	return nil
}

// PullRepository 拉取仓库更新
func (w *workspace) PullRepository(appID string) error {
	workDir := w.GetWorkDir(appID)
	if workDir == "" {
		return fmt.Errorf("work directory not found for app %s", appID)
	}

	cmd := exec.Command("git", "pull")
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull repository: %v", err)
	}

	return nil
}

// GetFileTree 获取文件树
func (w *workspace) GetFileTree(appID, path string) ([]FileInfo, error) {
	workDir := w.GetWorkDir(appID)
	if workDir == "" {
		return nil, fmt.Errorf("work directory not found for app %s", appID)
	}

	requestPath := filepath.Join(workDir, path)

	// 安全检查
	if !strings.HasPrefix(requestPath, workDir) {
		return nil, errors.New("invalid path: outside workspace")
	}

	// 检查路径是否存在
	if _, err := os.Stat(requestPath); os.IsNotExist(err) {
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

		info, err := file.Info()
		var size int64
		var modTime string
		if err != nil {
			size = 0
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
func (w *workspace) GetFileContent(appID, filePath string) (string, string, error) {
	workDir := w.GetWorkDir(appID)
	if workDir == "" {
		return "", "", fmt.Errorf("work directory not found for app %s", appID)
	}

	requestPath := filepath.Join(workDir, filePath)

	// 安全检查
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
	content, err := os.ReadFile(requestPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read file: %v", err)
	}

	// 检测语言类型
	ext := filepath.Ext(filePath)
	language := detectLanguage(ext)

	return string(content), language, nil
}

// SaveFileContent 保存文件内容
func (w *workspace) SaveFileContent(appID, filePath, content string) error {
	workDir := w.GetWorkDir(appID)
	if workDir == "" {
		return fmt.Errorf("work directory not found for app %s", appID)
	}

	fullPath := filepath.Join(workDir, filePath)

	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	logrus.Infof("Saved file: %s for application %s", filePath, appID)
	return nil
}

// CreateFile 创建新文件
func (w *workspace) CreateFile(appID, filePath string) error {
	workDir := w.GetWorkDir(appID)
	if workDir == "" {
		return fmt.Errorf("work directory not found for app %s", appID)
	}

	fullPath := filepath.Join(workDir, filePath)

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
func (w *workspace) DeleteFile(appID, filePath string) error {
	workDir := w.GetWorkDir(appID)
	if workDir == "" {
		return fmt.Errorf("work directory not found for app %s", appID)
	}

	fullPath := filepath.Join(workDir, filePath)

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
func (w *workspace) CreateDirectory(appID, dirPath string) error {
	workDir := w.GetWorkDir(appID)
	if workDir == "" {
		return fmt.Errorf("work directory not found for app %s", appID)
	}

	fullPath := filepath.Join(workDir, dirPath)

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
func (w *workspace) DeleteDirectory(appID, dirPath string) error {
	workDir := w.GetWorkDir(appID)
	if workDir == "" {
		return fmt.Errorf("work directory not found for app %s", appID)
	}

	fullPath := filepath.Join(workDir, dirPath)

	// 检查目录是否存在
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("directory not found: %s", dirPath)
	}

	// 确保是目录而不是文件
	if !info.IsDir() {
		return fmt.Errorf("path is a file, not a directory: %s", dirPath)
	}

	// 删除目录
	if err := os.RemoveAll(fullPath); err != nil {
		return fmt.Errorf("failed to delete directory: %v", err)
	}

	logrus.Infof("Deleted directory: %s for application %s", dirPath, appID)
	return nil
}

// GetWorkDir 获取工作目录
func (w *workspace) GetWorkDir(appID string) string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.workDirs[appID]
}

// SetWorkDir 设置工作目录
func (w *workspace) SetWorkDir(appID, workDir string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.workDirs[appID] = workDir
}

// CleanWorkDir 清理工作目录
func (w *workspace) CleanWorkDir(appID string) error {
	workDir := w.GetWorkDir(appID)
	if workDir == "" {
		return nil // 已经清理或不存在
	}

	if err := os.RemoveAll(workDir); err != nil {
		return fmt.Errorf("failed to remove workspace directory: %v", err)
	}

	w.mu.Lock()
	delete(w.workDirs, appID)
	w.mu.Unlock()

	logrus.Infof("Cleaned workspace for app %s", appID)
	return nil
}

// FileInfo 文件信息
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

// detectLanguage 根据文件扩展名检测语言类型
func detectLanguage(ext string) string {
	languageMap := map[string]string{
		".js":         "javascript",
		".ts":         "typescript",
		".jsx":        "javascript",
		".tsx":        "typescript",
		".py":         "python",
		".go":         "go",
		".java":       "java",
		".cpp":        "cpp",
		".c":          "c",
		".cs":         "csharp",
		".rb":         "ruby",
		".php":        "php",
		".swift":      "swift",
		".kt":         "kotlin",
		".rs":         "rust",
		".html":       "html",
		".css":        "css",
		".scss":       "scss",
		".less":       "less",
		".json":       "json",
		".yaml":       "yaml",
		".yml":        "yaml",
		".xml":        "xml",
		".md":         "markdown",
		".sql":        "sql",
		".sh":         "shell",
		".bash":       "shell",
		".dockerfile": "dockerfile",
	}

	if lang, ok := languageMap[strings.ToLower(ext)]; ok {
		return lang
	}
	return "plaintext"
}
