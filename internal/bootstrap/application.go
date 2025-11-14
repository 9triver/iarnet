package bootstrap

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/domain/application"
	"github.com/9triver/iarnet/internal/domain/application/metadata"
	"github.com/9triver/iarnet/internal/domain/application/runner"
	"github.com/9triver/iarnet/internal/domain/application/workspace"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

// BootstrapApplication 初始化 Application 模块
func bootstrapApplication(iarnet *Iarnet) error {
	// 初始化 Docker Client（如果还没有）
	if iarnet.DockerClient == nil {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.51"))
		if err != nil {
			return fmt.Errorf("failed to create docker client: %w", err)
		}

		// 测试 Docker 连接
		ctx := context.Background()
		if _, err := cli.Ping(ctx); err != nil {
			return fmt.Errorf("failed to ping docker daemon: %w", err)
		}

		iarnet.DockerClient = cli
		logrus.Info("Docker client initialized")
	}

	// 初始化 Workspace 模块
	workspaceManager := workspace.NewManager()
	workspaceService := workspace.NewService(workspaceManager, iarnet.Config.Application.WorkspaceDir)

	// 初始化 Runner 模块
	runnerManager := runner.NewManager()
	// 转换 RunnerImages 类型（map[string]string -> map[RunnerEnv]string）
	runnerImages := make(map[runner.RunnerEnv]string)
	for k, v := range iarnet.Config.Application.RunnerImages {
		runnerImages[runner.RunnerEnv(k)] = v
	}
	runnerService := runner.NewService(
		runnerManager,
		iarnet.DockerClient,
		int(iarnet.Config.Ignis.Port),
		runnerImages,
	)

	// 初始化 Metadata 模块
	metadataCache := metadata.NewCache()
	metadataService := metadata.NewService(metadataCache)

	// 组装 Application Manager
	appManager := application.NewManager(runnerService, workspaceService, metadataService)
	iarnet.ApplicationManager = appManager

	logrus.Info("Application module initialized")
	return nil
}
