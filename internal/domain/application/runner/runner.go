package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/9triver/iarnet/internal/domain/application/types"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

// Runner 运行器领域对象
// 封装了容器运行时的操作逻辑
type Runner struct {
	containerID   string
	dockerClient  *client.Client
	appID         string
	codeDir       string
	image         string
	ignisPort     int
	envInstallCmd string
	executeCmd    string
	status        types.RunnerStatus
	statusMu      sync.RWMutex // 保护 status 字段的锁
}

// NewRunner 创建运行器领域对象
func NewRunner(dockerClient *client.Client, appID, codeDir, image string, ignisPort int, envInstallCmd, executeCmd string) *Runner {
	return &Runner{
		dockerClient:  dockerClient,
		appID:         appID,
		codeDir:       codeDir,
		image:         image,
		ignisPort:     ignisPort,
		envInstallCmd: envInstallCmd,
		executeCmd:    executeCmd,
		status:        types.RunnerStatusIdle,
	}
}

// Start 启动运行器
func (r *Runner) Start(ctx context.Context) error {
	if r.dockerClient == nil {
		return fmt.Errorf("docker client not available")
	}

	switch r.status {
	case types.RunnerStatusStarting, types.RunnerStatusRunning, types.RunnerStatusStopping:
		return fmt.Errorf("runner is already running")
	}

	// 获取绝对路径
	hostPath, err := filepath.Abs(r.codeDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// 构建环境变量
	env := []string{
		"APP_ID=" + r.appID,
		"IGNIS_PORT=" + strconv.FormatInt(int64(r.ignisPort), 10),
		"ENV_INSTALL_CMD=" + r.envInstallCmd,
		"EXECUTE_CMD=" + r.executeCmd,
	}

	// 创建容器配置
	containerConfig := &container.Config{
		Image: r.image,
		Env:   env,
	}

	// 创建主机配置
	hostConfig := &container.HostConfig{
		Binds: []string{
			hostPath + ":/iarnet/app", // 挂载代码目录到容器
		},
		ExtraHosts: []string{
			"host.internal:host-gateway", // 允许容器访问宿主机
		},
	}

	// 创建容器
	resp, err := r.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// 启动容器
	if err := r.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	r.containerID = resp.ID
	logrus.Infof("Started application runner container for app %s (ID: %s)", r.appID, resp.ID)

	return nil
}

// Stop 停止容器
func (r *Runner) Stop(ctx context.Context) error {
	if r.dockerClient == nil {
		return fmt.Errorf("docker client not available")
	}

	timeout := 30 // 30 秒超时
	options := container.StopOptions{
		Timeout: &timeout,
	}

	if err := r.dockerClient.ContainerStop(ctx, r.containerID, options); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	logrus.Infof("Stopped container: %s", r.containerID)
	return nil
}

// GetStatus 获取运行器状态（线程安全）
func (r *Runner) GetStatus(ctx context.Context) (types.RunnerStatus, error) {
	r.statusMu.RLock()
	defer r.statusMu.RUnlock()
	return r.status, nil
}

// setStatus 设置运行器状态（线程安全，内部方法）
func (r *Runner) setStatus(status types.RunnerStatus) {
	r.statusMu.Lock()
	defer r.statusMu.Unlock()
	r.status = status
}

// RemoveContainer 删除容器
func (r *Runner) Clear(ctx context.Context) error {
	if r.dockerClient == nil {
		return fmt.Errorf("docker client not available")
	}

	options := container.RemoveOptions{
		Force: true,
	}

	if err := r.dockerClient.ContainerRemove(ctx, r.containerID, options); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	logrus.Infof("Removed container: %s", r.containerID)
	return nil
}
