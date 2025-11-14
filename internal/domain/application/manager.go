package application

import (
	"context"

	"github.com/9triver/iarnet/internal/domain/application/metadata"
	"github.com/9triver/iarnet/internal/domain/application/runner"
	"github.com/9triver/iarnet/internal/domain/application/types"
	"github.com/9triver/iarnet/internal/domain/application/workspace"
)

var (
	_ runner.Service    = (*Manager)(nil)
	_ workspace.Service = (*Manager)(nil)
	_ metadata.Service  = (*Manager)(nil)
)

type Manager struct {
	runnerSvc    runner.Service
	workspaceSvc workspace.Service
	metadataSvc  metadata.Service
}

func NewManager(runnerSvc runner.Service, workspaceSvc workspace.Service, metadataSvc metadata.Service) *Manager {
	return &Manager{runnerSvc: runnerSvc, workspaceSvc: workspaceSvc, metadataSvc: metadataSvc}
}

// Start starts the application manager
func (m *Manager) Start(ctx context.Context) error {

	return nil
}

// Runner methods
func (m *Manager) CreateRunner(ctx context.Context, appID string, codeDir string, env runner.RunnerEnv) error {
	return m.runnerSvc.CreateRunner(ctx, appID, codeDir, env)
}

func (m *Manager) StartRunner(ctx context.Context, appID string) error {
	return m.runnerSvc.StartRunner(ctx, appID)
}

func (m *Manager) StopRunner(ctx context.Context, appID string) error {
	return m.runnerSvc.StopRunner(ctx, appID)
}

func (m *Manager) RemoveRunner(ctx context.Context, appID string) error {
	return m.runnerSvc.RemoveRunner(ctx, appID)
}

// Workspace methods
func (m *Manager) CloneRepository(ctx context.Context, appID string, gitURL, branch string) error {
	return m.workspaceSvc.CloneRepository(ctx, appID, gitURL, branch)
}

func (m *Manager) PullRepository(ctx context.Context, appID string) error {
	return m.workspaceSvc.PullRepository(ctx, appID)
}

func (m *Manager) GetFileTree(ctx context.Context, appID string, path string) ([]types.FileInfo, error) {
	return m.workspaceSvc.GetFileTree(ctx, appID, path)
}

func (m *Manager) GetFileContent(ctx context.Context, appID string, filePath string) (string, string, error) {
	return m.workspaceSvc.GetFileContent(ctx, appID, filePath)
}

func (m *Manager) SaveFileContent(ctx context.Context, appID string, filePath string, content string) error {
	return m.workspaceSvc.SaveFileContent(ctx, appID, filePath, content)
}

func (m *Manager) CreateFile(ctx context.Context, appID string, filePath string) error {
	return m.workspaceSvc.CreateFile(ctx, appID, filePath)
}

func (m *Manager) DeleteFile(ctx context.Context, appID string, filePath string) error {
	return m.workspaceSvc.DeleteFile(ctx, appID, filePath)
}

func (m *Manager) CreateDirectory(ctx context.Context, appID string, dirPath string) error {
	return m.workspaceSvc.CreateDirectory(ctx, appID, dirPath)
}

func (m *Manager) DeleteDirectory(ctx context.Context, appID string, dirPath string) error {
	return m.workspaceSvc.DeleteDirectory(ctx, appID, dirPath)
}

func (m *Manager) CleanWorkDir(ctx context.Context, appID string) error {
	return m.workspaceSvc.CleanWorkDir(ctx, appID)
}

// Metadata methods
func (m *Manager) CreateAppMetadata(ctx context.Context, appID string, metadata types.AppMetadata) error {
	return m.metadataSvc.CreateAppMetadata(ctx, appID, metadata)
}

func (m *Manager) GetAppMetadata(ctx context.Context, appID string) (types.AppMetadata, error) {
	return m.metadataSvc.GetAppMetadata(ctx, appID)
}

func (m *Manager) UpdateAppMetadata(ctx context.Context, appID string, metadata types.AppMetadata) error {
	return m.metadataSvc.UpdateAppMetadata(ctx, appID, metadata)
}

func (m *Manager) UpdateAppStatus(ctx context.Context, appID string, status types.AppStatus) error {
	return m.metadataSvc.UpdateAppStatus(ctx, appID, status)
}

func (m *Manager) RemoveAppMetadata(ctx context.Context, appID string) error {
	return m.metadataSvc.RemoveAppMetadata(ctx, appID)
}
