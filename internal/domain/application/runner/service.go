package runner

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/domain/application/types"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

type RunnerEnv = types.RunnerEnv

// Service 运行器服务接口
// 提供无状态的运行器操作服务，所有状态由 Manager 管理
type Service interface {
	GetRunnerImages() map[RunnerEnv]string
	CreateRunner(ctx context.Context, appID, codeDir string, env RunnerEnv, envInstallCmd, executeCmd string) error
	StartRunner(ctx context.Context, appID string) error
	StopRunner(ctx context.Context, appID string) error
	RemoveRunner(ctx context.Context, appID string) error

	// GetRunnerStatus(ctx context.Context, appID string) (*ContainerStatus, error)
}

// service 运行器服务实现
// 无状态服务，负责创建运行器并加入 Manager，通过 Manager 获取运行器实例并委托调用
type service struct {
	manager      *Manager
	ignisPort    int
	loggerPort   int
	images       map[RunnerEnv]string
	dockerClient *client.Client
}

// NewService 创建运行器服务
func NewService(manager *Manager, dockerClient *client.Client, ignisPort int, loggerPort int, images map[RunnerEnv]string) Service {
	return &service{
		manager:      manager,
		dockerClient: dockerClient,
		ignisPort:    ignisPort,
		loggerPort:   loggerPort,
		images:       images,
	}
}

// GetRunnerImages 获取运行器镜像
func (s *service) GetRunnerImages() map[RunnerEnv]string {
	return s.images
}

// CreateRunner 创建运行器
func (s *service) CreateRunner(ctx context.Context, appID, codeDir string, env RunnerEnv, envInstallCmd, executeCmd string) error {
	image, ok := s.images[env]
	if !ok {
		return fmt.Errorf("image not found for environment %s", env)
	}

	// 在 service 中创建运行器领域对象
	runner := NewRunner(s.dockerClient, appID, codeDir, image, &EnvVars{
		IgnisPort:     s.ignisPort,
		LoggerPort:    s.loggerPort,
		EnvInstallCmd: envInstallCmd,
		ExecuteCmd:    executeCmd,
	})

	// 将运行器加入 manager
	s.manager.Add(appID, runner)

	logrus.Infof("Created runner for application %s", appID)
	return nil
}

// StartRunner 启动运行器
func (s *service) StartRunner(ctx context.Context, appID string) error {
	runner, err := s.manager.Get(appID)
	if err != nil {
		return err
	}

	// 委托给领域对象执行启动操作（包含健康检查）
	return runner.Start(ctx)
}

// StopRunner 停止运行器
func (s *service) StopRunner(ctx context.Context, appID string) error {
	runner, err := s.manager.Get(appID)
	if err != nil {
		return err
	}

	// 委托给领域对象执行停止操作
	return runner.Stop(ctx)
}

// RemoveRunner 删除运行器
func (s *service) RemoveRunner(ctx context.Context, appID string) error {
	runner, err := s.manager.Get(appID)
	if err != nil {
		return err
	}

	// 委托给领域对象执行清理操作
	if err := runner.Clear(ctx); err != nil {
		return err
	}

	// 删除成功后，从 manager 中移除运行器引用
	s.manager.Remove(appID)

	logrus.Infof("Removed runner for application %s", appID)
	return nil
}
