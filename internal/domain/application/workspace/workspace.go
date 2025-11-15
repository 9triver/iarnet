package workspace

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/9triver/iarnet/internal/domain/application/types"
	"github.com/sirupsen/logrus"
)

// Workspace 工作空间领域对象
// 封装了工作空间的文件操作、目录操作和 Git 操作
type Workspace struct {
	dir string
}

// NewWorkspace 创建工作空间领域对象
func NewWorkspace(dir string) *Workspace {
	return &Workspace{
		dir: dir,
	}
}

// GetDir 获取工作空间目录路径
func (w *Workspace) GetDir() string {
	return w.dir
}

// CloneRepository 克隆 Git 仓库到工作空间
func (w *Workspace) CloneRepository(gitURL, branch string) error {
	// 执行 git clone
	cmd := exec.Command("git", "clone", "-b", branch, "--single-branch", gitURL, w.dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		os.RemoveAll(w.dir)
		return fmt.Errorf("failed to clone repository: %v", err)
	}

	logrus.Infof("Successfully cloned repository to %s", w.dir)
	return nil
}

// PullRepository 拉取仓库更新
func (w *Workspace) PullRepository() error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = w.dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull repository: %v", err)
	}

	return nil
}

// GetFileTree 获取文件树
func (w *Workspace) GetFileTree(path string) ([]types.FileInfo, error) {
	requestPath := filepath.Join(w.dir, path)

	// 安全检查：确保路径在工作空间内
	if !strings.HasPrefix(requestPath, w.dir) {
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
	var fileInfos []types.FileInfo
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

		fileInfo := types.FileInfo{
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
func (w *Workspace) GetFileContent(filePath string) (string, string, error) {
	requestPath := filepath.Join(w.dir, filePath)

	// 安全检查：确保路径在工作空间内
	if !strings.HasPrefix(requestPath, w.dir) {
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
func (w *Workspace) SaveFileContent(filePath, content string) error {
	fullPath := filepath.Join(w.dir, filePath)

	// 安全检查：确保路径在工作空间内
	if !strings.HasPrefix(fullPath, w.dir) {
		return errors.New("invalid path: outside workspace")
	}

	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	logrus.Infof("Saved file: %s", filePath)
	return nil
}

// CreateFile 创建新文件
func (w *Workspace) CreateFile(filePath string) error {
	fullPath := filepath.Join(w.dir, filePath)

	// 安全检查：确保路径在工作空间内
	if !strings.HasPrefix(fullPath, w.dir) {
		return errors.New("invalid path: outside workspace")
	}

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

	logrus.Infof("Created file: %s", filePath)
	return nil
}

// DeleteFile 删除文件
func (w *Workspace) DeleteFile(filePath string) error {
	fullPath := filepath.Join(w.dir, filePath)

	// 安全检查：确保路径在工作空间内
	if !strings.HasPrefix(fullPath, w.dir) {
		return errors.New("invalid path: outside workspace")
	}

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

	logrus.Infof("Deleted file: %s", filePath)
	return nil
}

// CreateDirectory 创建目录
func (w *Workspace) CreateDirectory(dirPath string) error {
	fullPath := filepath.Join(w.dir, dirPath)

	// 安全检查：确保路径在工作空间内
	if !strings.HasPrefix(fullPath, w.dir) {
		return errors.New("invalid path: outside workspace")
	}

	// 检查目录是否已存在
	if _, err := os.Stat(fullPath); err == nil {
		return fmt.Errorf("directory already exists: %s", dirPath)
	}

	// 创建目录
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	logrus.Infof("Created directory: %s", dirPath)
	return nil
}

// DeleteDirectory 删除目录
func (w *Workspace) DeleteDirectory(dirPath string) error {
	fullPath := filepath.Join(w.dir, dirPath)

	// 安全检查：确保路径在工作空间内
	if !strings.HasPrefix(fullPath, w.dir) {
		return errors.New("invalid path: outside workspace")
	}

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

	logrus.Infof("Deleted directory: %s", dirPath)
	return nil
}

// Clean 清理工作目录
func (w *Workspace) Clean() error {
	if err := os.RemoveAll(w.dir); err != nil {
		return fmt.Errorf("failed to remove workspace directory: %v", err)
	}

	logrus.Infof("Cleaned workspace: %s", w.dir)
	return nil
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
