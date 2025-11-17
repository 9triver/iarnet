package workspace

import (
	"context"
	"os"
	"path/filepath"

	"github.com/9triver/iarnet/internal/domain/application/types"
	"github.com/sirupsen/logrus"
)

// Service 工作空间服务接口
// 提供无状态的工作空间操作服务，所有状态由 Manager 管理
type Service interface {
	// Git 仓库管理
	CloneRepository(ctx context.Context, appID, gitURL, branch string) (string, error)
	PullRepository(ctx context.Context, appID string) error

	// 文件操作
	GetFileTree(ctx context.Context, appID, path string) ([]types.FileInfo, error)
	GetFileContent(ctx context.Context, appID, filePath string) (string, string, error)
	SaveFileContent(ctx context.Context, appID, filePath, content string) error
	CreateFile(ctx context.Context, appID, filePath string) error
	DeleteFile(ctx context.Context, appID, filePath string) error

	// 目录操作
	CreateDirectory(ctx context.Context, appID, dirPath string) error
	DeleteDirectory(ctx context.Context, appID, dirPath string) error

	// 工作目录管理
	CleanWorkDir(ctx context.Context, appID string) error
	// GetWorkspaceDir 获取工作空间目录路径
	GetWorkspaceDir(ctx context.Context, appID string) (string, error)
}

// service 工作空间服务实现
// 无状态服务，负责创建工作空间并加入 Manager，通过 Manager 获取工作空间实例并委托调用
type service struct {
	manager *Manager
	baseDir string
}

// NewService 创建工作空间服务
func NewService(manager *Manager, baseDir string) Service {
	if baseDir == "" {
		baseDir = "./workspaces"
	}

	// 确保基础目录存在
	os.MkdirAll(baseDir, 0755)

	return &service{
		manager: manager,
		baseDir: baseDir,
	}
}

// CloneRepository 克隆 Git 仓库
func (s *service) CloneRepository(ctx context.Context, appID, gitURL, branch string) (string, error) {
	// 在 service 中创建工作空间领域对象
	workspaceDir := filepath.Join(s.baseDir, appID)
	workspace := NewWorkspace(workspaceDir)

	// 委托给领域对象执行克隆操作
	if err := workspace.CloneRepository(gitURL, branch); err != nil {
		return "", err
	}

	// 克隆成功后，将工作空间加入 manager
	s.manager.Add(appID, workspace)

	logrus.Infof("Cloned repository for application %s to %s", appID, workspaceDir)
	return workspaceDir, nil
}

// PullRepository 拉取仓库更新
func (s *service) PullRepository(ctx context.Context, appID string) error {
	workspace, err := s.manager.Get(appID)
	if err != nil {
		return err
	}

	// 委托给领域对象执行拉取操作
	return workspace.PullRepository()
}

// GetFileTree 获取文件树
func (s *service) GetFileTree(ctx context.Context, appID, path string) ([]types.FileInfo, error) {
	workspace, err := s.manager.Get(appID)
	if err != nil {
		return nil, err
	}

	// 委托给领域对象获取文件树
	return workspace.GetFileTree(path)
}

// GetFileContent 获取文件内容
func (s *service) GetFileContent(ctx context.Context, appID, filePath string) (string, string, error) {
	workspace, err := s.manager.Get(appID)
	if err != nil {
		return "", "", err
	}

	// 委托给领域对象获取文件内容
	return workspace.GetFileContent(filePath)
}

// SaveFileContent 保存文件内容
func (s *service) SaveFileContent(ctx context.Context, appID, filePath, content string) error {
	workspace, err := s.manager.Get(appID)
	if err != nil {
		return err
	}

	// 委托给领域对象保存文件内容
	if err := workspace.SaveFileContent(filePath, content); err != nil {
		return err
	}

	logrus.Infof("Saved file: %s for application %s", filePath, appID)
	return nil
}

// CreateFile 创建新文件
func (s *service) CreateFile(ctx context.Context, appID, filePath string) error {
	workspace, err := s.manager.Get(appID)
	if err != nil {
		return err
	}

	// 委托给领域对象创建文件
	if err := workspace.CreateFile(filePath); err != nil {
		return err
	}

	logrus.Infof("Created file: %s for application %s", filePath, appID)
	return nil
}

// DeleteFile 删除文件
func (s *service) DeleteFile(ctx context.Context, appID, filePath string) error {
	workspace, err := s.manager.Get(appID)
	if err != nil {
		return err
	}

	// 委托给领域对象删除文件
	if err := workspace.DeleteFile(filePath); err != nil {
		return err
	}

	logrus.Infof("Deleted file: %s for application %s", filePath, appID)
	return nil
}

// CreateDirectory 创建目录
func (s *service) CreateDirectory(ctx context.Context, appID, dirPath string) error {
	workspace, err := s.manager.Get(appID)
	if err != nil {
		return err
	}

	// 委托给领域对象创建目录
	if err := workspace.CreateDirectory(dirPath); err != nil {
		return err
	}

	logrus.Infof("Created directory: %s for application %s", dirPath, appID)
	return nil
}

// DeleteDirectory 删除目录
func (s *service) DeleteDirectory(ctx context.Context, appID, dirPath string) error {
	workspace, err := s.manager.Get(appID)
	if err != nil {
		return err
	}

	// 委托给领域对象删除目录
	if err := workspace.DeleteDirectory(dirPath); err != nil {
		return err
	}

	logrus.Infof("Deleted directory: %s for application %s", dirPath, appID)
	return nil
}

// CleanWorkDir 清理工作目录
func (s *service) CleanWorkDir(ctx context.Context, appID string) error {
	workspace, err := s.manager.Get(appID)
	if err != nil {
		return err
	}

	// 委托给领域对象清理工作目录
	if err := workspace.Clean(); err != nil {
		return err
	}

	// 从 manager 中移除工作空间引用
	s.manager.Remove(appID)

	logrus.Infof("Cleaned workspace for app %s", appID)
	return nil
}

// GetWorkspaceDir 获取工作空间目录路径
func (s *service) GetWorkspaceDir(ctx context.Context, appID string) (string, error) {
	workspace, err := s.manager.Get(appID)
	if err != nil {
		// 如果工作空间不存在，返回预期的路径（即使目录还不存在）
		return filepath.Join(s.baseDir, appID), nil
	}
	return workspace.GetDir(), nil
}
