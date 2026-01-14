package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/9triver/iarnet/internal/domain/application/logger"
	"github.com/9triver/iarnet/internal/domain/application/metadata"
	"github.com/9triver/iarnet/internal/domain/application/runner"
	"github.com/9triver/iarnet/internal/domain/application/types"
	"github.com/9triver/iarnet/internal/domain/application/workspace"
	"github.com/9triver/iarnet/internal/domain/execution"
	"github.com/9triver/iarnet/internal/domain/execution/task"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/provider"
	"github.com/9triver/iarnet/internal/domain/resource/scheduler"
	logrus "github.com/sirupsen/logrus"
)

var (
	_ runner.Service    = (*Manager)(nil)
	_ workspace.Service = (*Manager)(nil)
	_ metadata.Service  = (*Manager)(nil)
	_ logger.Service    = (*Manager)(nil)
)

type Manager struct {
	runnerSvc    runner.Service
	workspaceSvc workspace.Service
	metadataSvc  metadata.Service
	platform     *execution.Platform
	loggerSvc    logger.Service
	// 用于资源清理
	resourceManager interface {
		GetComponentManager() component.Manager
		GetProviderService() provider.Service
		GetDiscoveryService() discovery.Service
		GetSchedulerService() scheduler.Service
		GetNodeID() string
	}
}

func NewManager() *Manager {
	return &Manager{}
}

// Dependency Injection

func (m *Manager) SetApplicationRunnerService(runnerSvc runner.Service) *Manager {
	m.runnerSvc = runnerSvc
	return m
}

func (m *Manager) SetApplicationWorkspaceService(workspaceSvc workspace.Service) *Manager {
	m.workspaceSvc = workspaceSvc
	return m
}

func (m *Manager) SetApplicationMetadataService(metadataSvc metadata.Service) *Manager {
	m.metadataSvc = metadataSvc
	return m
}

func (m *Manager) SetIgnisPlatform(platform *execution.Platform) *Manager {
	m.platform = platform
	return m
}

func (m *Manager) SetApplicationLoggerService(loggerSvc logger.Service) *Manager {
	m.loggerSvc = loggerSvc
	return m
}

func (m *Manager) SetResourceManager(resourceManager interface {
	GetComponentManager() component.Manager
	GetProviderService() provider.Service
	GetDiscoveryService() discovery.Service
	GetSchedulerService() scheduler.Service
	GetNodeID() string
}) *Manager {
	m.resourceManager = resourceManager
	return m
}

// Start starts the application manager
func (m *Manager) Start(ctx context.Context) error {

	return nil
}

// Runner methods
func (m *Manager) CreateRunner(ctx context.Context, appID string, codeDir string, env runner.RunnerEnv, envInstallCmd, executeCmd string) error {
	return m.runnerSvc.CreateRunner(ctx, appID, codeDir, env, envInstallCmd, executeCmd)
}

func (m *Manager) StartRunner(ctx context.Context, appID string) error {
	return m.runnerSvc.StartRunner(ctx, appID)
}

func (m *Manager) GetRunnerImages() map[runner.RunnerEnv]string {
	return m.runnerSvc.GetRunnerImages()
}

func (m *Manager) StopRunner(ctx context.Context, appID string) error {
	return m.runnerSvc.StopRunner(ctx, appID)
}

func (m *Manager) RemoveRunner(ctx context.Context, appID string) error {
	return m.runnerSvc.RemoveRunner(ctx, appID)
}

// CleanupApplicationResources 清理应用的所有资源
// 包括：移除 runner 容器、清理 components、清理 controller 数据
func (m *Manager) CleanupApplicationResources(ctx context.Context, appID string) error {
	if m.resourceManager == nil {
		logrus.Warnf("Resource manager not set, skipping resource cleanup for application %s", appID)
		return nil
	}

	componentManager := m.resourceManager.GetComponentManager()
	providerService := m.resourceManager.GetProviderService()

	// 1. 获取应用的 controller，从中获取所有 actors 和 components
	actors, err := m.platform.GetActors(appID)
	if err != nil {
		logrus.Warnf("Failed to get actors for application %s: %v, continuing cleanup", appID, err)
		actors = nil
	}

	// 2. 收集所有需要清理的 components
	componentsToCleanup := make(map[string]*component.Component) // componentID -> Component
	for _, actorList := range actors {
		for _, actor := range actorList {
			comp := actor.GetComponent()
			if comp != nil {
				componentsToCleanup[comp.GetID()] = comp
			}
		}
	}

	// 3. 从 provider 中移除 components
	for componentID, comp := range componentsToCleanup {
		providerID := comp.GetProviderID()
		if providerID != "" {
			// 检查是否为跨域部署（providerID 格式为 "providerID@nodeID"）
			if strings.Contains(providerID, "@") {
				// 跨域场景：解析 nodeID 和实际的 providerID
				parts := strings.SplitN(providerID, "@", 2)
				if len(parts) != 2 {
					logrus.Warnf("Invalid cross-node provider ID format: %s for component %s", providerID, componentID)
					continue
				}
				actualProviderID := parts[0]
				remoteNodeID := parts[1]

				// 获取 discovery service 以查找远程节点
				discoverySvc := m.resourceManager.GetDiscoveryService()
				if discoverySvc == nil {
					logrus.Warnf("Discovery service not available, cannot undeploy cross-node component %s from node %s", componentID, remoteNodeID)
					continue
				}

				// 从已知节点中查找远程节点
				knownNodes := discoverySvc.GetKnownNodes()
				var remoteNodeAddress string
				found := false
				for _, node := range knownNodes {
					if node.NodeID == remoteNodeID {
						remoteNodeAddress = node.Address
						found = true
						break
					}
				}

				if !found {
					logrus.Warnf("Remote node %s not found for component %s, cannot undeploy", remoteNodeID, componentID)
					continue
				}

				// 通过远程 iarnet 节点的 scheduler RPC 调用 UndeployComponent
				schedulerSvc := m.resourceManager.GetSchedulerService()
				if schedulerSvc == nil {
					logrus.Warnf("Scheduler service not available, cannot undeploy cross-node component %s from node %s", componentID, remoteNodeID)
					continue
				}

				// 使用远程节点的 SchedulerAddress（如果可用），否则使用 Address
				schedulerAddress := remoteNodeAddress
				for _, node := range knownNodes {
					if node.NodeID == remoteNodeID && node.SchedulerAddress != "" {
						schedulerAddress = node.SchedulerAddress
						break
					}
				}

				if err := schedulerSvc.UndeployRemoteComponent(ctx, remoteNodeID, schedulerAddress, componentID, actualProviderID); err != nil {
					logrus.Warnf("Failed to undeploy component %s from remote provider %s@%s: %v", componentID, actualProviderID, remoteNodeID, err)
				} else {
					logrus.Infof("Undeployed component %s from remote provider %s@%s", componentID, actualProviderID, remoteNodeID)
				}
			} else {
				// 本地部署：使用本地 provider
				// 去掉 "local." 前缀（如果存在），因为 provider manager 中存储的 ID 没有这个前缀
				actualProviderID := providerID
				if strings.HasPrefix(providerID, "local.") {
					actualProviderID = strings.TrimPrefix(providerID, "local.")
				}
				p := providerService.GetProvider(actualProviderID)
				if p != nil {
					// 检查是否为 fake provider（fake provider 不支持卸载）
					if p.IsFake() {
						return fmt.Errorf("fake provider %s does not support undeployment", providerID)
					}

					// 类型断言为 *provider.Provider
					realProvider, ok := p.(*provider.Provider)
					if !ok {
						return fmt.Errorf("provider %s is not a real provider", providerID)
					}

					if err := realProvider.Undeploy(ctx, componentID); err != nil {
						logrus.Warnf("Failed to undeploy component %s from provider %s: %v", componentID, actualProviderID, err)
					} else {
						logrus.Infof("Undeployed component %s from provider %s", componentID, actualProviderID)
					}
				} else {
					logrus.Warnf("Provider %s not found for component %s", actualProviderID, componentID)
				}
			}
		} else {
			logrus.Errorf("Provider id can not be empty. component %s", comp.GetID())
		}
	}

	// 4. 从 component manager 中移除 components
	for componentID := range componentsToCleanup {
		if err := componentManager.RemoveComponent(ctx, componentID); err != nil {
			logrus.Warnf("Failed to remove component %s from manager: %v", componentID, err)
		} else {
			logrus.Infof("Removed component %s from manager", componentID)
		}
	}

	// 5. 清理 controller 的状态（不移除 controller，只清理其内部状态）
	if err := m.platform.ClearController(ctx, appID); err != nil {
		logrus.Warnf("Failed to clear controller state for application %s: %v", appID, err)
	} else {
		logrus.Infof("Cleared controller state for application %s", appID)
	}

	// 6. 移除 runner 容器
	if err := m.RemoveRunner(ctx, appID); err != nil {
		logrus.Warnf("Failed to remove runner for application %s: %v", appID, err)
	} else {
		logrus.Infof("Removed runner for application %s", appID)
	}

	logrus.Infof("Resource cleanup completed for application %s", appID)
	return nil
}

// Workspace methods
func (m *Manager) CloneRepository(ctx context.Context, appID string, gitURL, branch string) (string, error) {
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

func (m *Manager) GetWorkspaceDir(ctx context.Context, appID string) (string, error) {
	return m.workspaceSvc.GetWorkspaceDir(ctx, appID)
}

// Metadata methods
func (m *Manager) GetAllAppMetadata(ctx context.Context) ([]types.AppMetadata, error) {
	return m.metadataSvc.GetAllAppMetadata(ctx)
}

func (m *Manager) CreateAppMetadata(ctx context.Context, metadata types.AppMetadata) (types.AppID, error) {
	return m.metadataSvc.CreateAppMetadata(ctx, metadata)
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

// Logger methods
func (m *Manager) SubmitLog(ctx context.Context, applicationID string, entry *logger.Entry) (logger.LogID, error) {
	return m.loggerSvc.SubmitLog(ctx, applicationID, entry)
}

func (m *Manager) GetLogs(ctx context.Context, applicationID string, options *logger.QueryOptions) (*logger.QueryResult, error) {
	return m.loggerSvc.GetLogs(ctx, applicationID, options)
}

func (m *Manager) GetLogsByTimeRange(ctx context.Context, applicationID string, startTime, endTime time.Time, limit int) ([]*logger.Entry, error) {
	return m.loggerSvc.GetLogsByTimeRange(ctx, applicationID, startTime, endTime, limit)
}

// application manager methods
// CreateApplication 创建应用，包括创建元数据和异步克隆 Git 仓库
// 返回应用 ID 和错误
func (m *Manager) CreateApplication(ctx context.Context, metadata types.AppMetadata) (types.AppID, error) {
	// 设置初始状态为 cloning
	metadata.Status = types.AppStatusCloning

	// 创建应用元数据
	appID, err := m.metadataSvc.CreateAppMetadata(ctx, metadata)
	if err != nil {
		logrus.Errorf("Failed to create app metadata: %v", err)
		return "", err
	}

	// 创建控制器
	_, err = m.platform.CreateController(ctx, string(appID))
	if err != nil {
		logrus.Errorf("Failed to create controller for application %s: %v", appID, err)
		return "", err
	}

	// 异步克隆 Git 仓库
	go func() {
		ctx := context.Background()
		logrus.Infof("Starting async clone for application %s", appID)

		codeDir, err := m.workspaceSvc.CloneRepository(ctx, string(appID), metadata.GitUrl, metadata.Branch)
		if err != nil {
			logrus.Errorf("Failed to clone repository for application %s: %v", appID, err)
			// 克隆失败，更新状态为 error
			if updateErr := m.metadataSvc.UpdateAppStatus(ctx, string(appID), types.AppStatusFailed); updateErr != nil {
				logrus.Errorf("Failed to update app status to error: %v", updateErr)
			}
			return
		}

		// 克隆成功
		logrus.Infof("Successfully cloned repository for application %s to %s", appID, codeDir)

		// 更新状态为 idle（未部署）
		if err := m.metadataSvc.UpdateAppStatus(ctx, string(appID), types.AppStatusUndeployed); err != nil {
			logrus.Errorf("Failed to update app status to idle: %v", err)
		}
	}()

	logrus.Infof("Application created successfully: id=%s (cloning in background)", appID)
	return appID, nil
}

// RunApplication 运行应用，包括创建和启动 runner
func (m *Manager) RunApplication(ctx context.Context, appID string) error {
	// 更新应用状态为部署中
	if err := m.metadataSvc.UpdateAppStatus(ctx, appID, types.AppStatusDeploying); err != nil {
		logrus.Errorf("Failed to update application status to deploying: %v", err)
		return fmt.Errorf("application not found: %s", appID)
	}

	// 获取应用元数据
	metadata, err := m.metadataSvc.GetAppMetadata(ctx, appID)
	if err != nil {
		logrus.Errorf("Failed to get app metadata: %v", err)
		m.metadataSvc.UpdateAppStatus(ctx, appID, types.AppStatusFailed)
		return fmt.Errorf("application not found: %s", appID)
	}

	// 获取工作空间目录
	codeDir, err := m.workspaceSvc.GetWorkspaceDir(ctx, appID)
	if err != nil {
		logrus.Errorf("Failed to get workspace directory for application %s: %v", appID, err)
		m.metadataSvc.UpdateAppStatus(ctx, appID, types.AppStatusFailed)
		return fmt.Errorf("failed to get workspace directory: %w", err)
	}

	// 创建 runner（如果还没有创建）
	// 注意：runner 可能在创建应用时已经创建，这里需要检查或直接创建
	if err := m.runnerSvc.CreateRunner(ctx, appID, codeDir, runner.RunnerEnv(metadata.RunnerEnv), metadata.EnvInstallCmd, metadata.ExecuteCmd); err != nil {
		logrus.Errorf("Failed to create runner for application %s: %v", appID, err)
		m.metadataSvc.UpdateAppStatus(ctx, appID, types.AppStatusFailed)
		return fmt.Errorf("failed to create runner: %w", err)
	}

	// 启动 runner
	if err := m.runnerSvc.StartRunner(ctx, appID); err != nil {
		logrus.Errorf("Failed to start runner for application %s: %v", appID, err)
		m.metadataSvc.UpdateAppStatus(ctx, appID, types.AppStatusFailed)
		return fmt.Errorf("failed to start runner: %w", err)
	}

	// 更新应用状态为运行中
	if err := m.metadataSvc.UpdateAppStatus(ctx, appID, types.AppStatusRunning); err != nil {
		logrus.Errorf("Failed to update application status to running: %v", err)
		// 不返回错误，因为 runner 已经启动成功
	}

	logrus.Infof("Successfully started application %s", appID)
	return nil
}

func (m *Manager) GetApplicationDAGs(ctx context.Context, appID string) (map[string]*task.DAG, error) {
	return m.platform.GetDAGs(appID)
}

func (m *Manager) GetApplicationActors(ctx context.Context, appID string) (map[string][]*task.Actor, error) {
	return m.platform.GetActors(appID)
}
