package application

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/resource"
	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

// RuntimeService 容器运行时服务接口
type RuntimeService interface {
	// StartContainer 启动容器（通用方法）
	StartContainer(ctx context.Context, spec *ContainerSpec) (string, error)

	// StartApplicationRunner 启动应用 runner 容器（特定逻辑）
	StartApplicationRunner(ctx context.Context, appID, image, codeDir, executeCmd string, ignisPort int32) (string, error)

	// StopContainer 停止容器
	StopContainer(ctx context.Context, containerID string) error

	// GetContainerStatus 获取容器状态
	GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatus, error)

	// RemoveContainer 删除容器
	RemoveContainer(ctx context.Context, containerID string) error

	// RegisterComponent 注册组件
	RegisterComponent(appID, name string, ref *resource.ContainerRef) error

	// GetComponents 获取应用的所有组件
	GetComponents(appID string) map[string]*Component
}

// ContainerSpec 容器规格
type ContainerSpec struct {
	Image       string
	Name        string
	Env         []string
	Mounts      []string
	Ports       map[string]string
	WorkDir     string
	Cmd         []string
	NetworkMode string
}

// ContainerStatus 容器状态
type ContainerStatus struct {
	ID      string
	Status  string // running, exited, etc.
	Running bool
}

// runtime 容器运行时服务实现
type runtime struct {
	dockerClient *client.Client
	resMgr       *resource.Manager
	components   map[string]map[string]*Component // appID -> componentName -> Component
	mu           sync.RWMutex
}

// NewRuntime 创建容器运行时服务
func NewRuntime(dockerClient *client.Client, resMgr *resource.Manager) RuntimeService {
	return &runtime{
		dockerClient: dockerClient,
		resMgr:       resMgr,
		components:   make(map[string]map[string]*Component),
	}
}

// StartContainer 启动容器
func (r *runtime) StartContainer(ctx context.Context, spec *ContainerSpec) (string, error) {
	if r.dockerClient == nil {
		return "", fmt.Errorf("docker client not available")
	}

	// 拉取镜像
	logrus.Infof("Pulling image: %s", spec.Image)
	reader, err := r.dockerClient.ImagePull(ctx, spec.Image, image.PullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader)

	// 构建容器配置
	config := &container.Config{
		Image:      spec.Image,
		Env:        spec.Env,
		WorkingDir: spec.WorkDir,
		Cmd:        spec.Cmd,
	}

	// 构建主机配置
	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(spec.NetworkMode),
	}

	// 处理挂载
	for _, mount := range spec.Mounts {
		parts := strings.Split(mount, ":")
		if len(parts) == 2 {
			hostConfig.Binds = append(hostConfig.Binds, mount)
		}
	}

	// 处理端口映射
	if len(spec.Ports) > 0 {
		config.ExposedPorts = make(map[nat.Port]struct{})
		hostConfig.PortBindings = make(map[nat.Port][]nat.PortBinding)

		for containerPort, hostPort := range spec.Ports {
			port := nat.Port(containerPort)
			config.ExposedPorts[port] = struct{}{}
			hostConfig.PortBindings[port] = []nat.PortBinding{
				{HostPort: hostPort},
			}
		}
	}

	// 创建容器
	resp, err := r.dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, spec.Name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// 启动容器
	if err := r.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	logrus.Infof("Started container: %s (ID: %s)", spec.Name, resp.ID)
	return resp.ID, nil
}

// StartApplicationRunner 启动应用 runner 容器（针对用户应用的特定逻辑）
func (r *runtime) StartApplicationRunner(ctx context.Context, appID, image, codeDir, executeCmd string, ignisPort int32) (string, error) {
	if r.dockerClient == nil {
		return "", fmt.Errorf("docker client not available")
	}

	// 获取绝对路径
	hostPath, err := filepath.Abs(codeDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// 构建环境变量
	env := []string{
		"APP_ID=" + appID,
		"IGNIS_PORT=" + strconv.FormatInt(int64(ignisPort), 10),
		"EXECUTE_CMD=" + executeCmd,
	}

	// 创建容器配置
	containerConfig := &container.Config{
		Image: image,
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
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// 启动容器
	if err := r.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	logrus.Infof("Started application runner container for app %s (ID: %s)", appID, resp.ID)
	return resp.ID, nil
}

// StopContainer 停止容器
func (r *runtime) StopContainer(ctx context.Context, containerID string) error {
	if r.dockerClient == nil {
		return fmt.Errorf("docker client not available")
	}

	timeout := 30 // 30 秒超时
	options := container.StopOptions{
		Timeout: &timeout,
	}

	if err := r.dockerClient.ContainerStop(ctx, containerID, options); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	logrus.Infof("Stopped container: %s", containerID)
	return nil
}

// GetContainerStatus 获取容器状态
func (r *runtime) GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatus, error) {
	if r.dockerClient == nil {
		return nil, fmt.Errorf("docker client not available")
	}

	inspect, err := r.dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	return &ContainerStatus{
		ID:      inspect.ID,
		Status:  inspect.State.Status,
		Running: inspect.State.Running,
	}, nil
}

// RemoveContainer 删除容器
func (r *runtime) RemoveContainer(ctx context.Context, containerID string) error {
	if r.dockerClient == nil {
		return fmt.Errorf("docker client not available")
	}

	options := container.RemoveOptions{
		Force: true,
	}

	if err := r.dockerClient.ContainerRemove(ctx, containerID, options); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	logrus.Infof("Removed container: %s", containerID)
	return nil
}

// RegisterComponent 注册组件
func (r *runtime) RegisterComponent(appID, name string, ref *resource.ContainerRef) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.components[appID] == nil {
		r.components[appID] = make(map[string]*Component)
	}

	r.components[appID][name] = &Component{
		Name:         name,
		Image:        ref.Spec.Image,
		Status:       ComponentStatusRunning,
		CreatedAt:    time.Now(),
		DeployedAt:   time.Now(),
		UpdatedAt:    time.Now(),
		ContainerRef: ref,
	}

	logrus.Infof("Registered component %s for app %s", name, appID)
	return nil
}

// GetComponents 获取应用的所有组件
func (r *runtime) GetComponents(appID string) map[string]*Component {
	r.mu.RLock()
	defer r.mu.RUnlock()

	components, exists := r.components[appID]
	if !exists {
		return make(map[string]*Component)
	}

	// 返回副本
	result := make(map[string]*Component)
	for k, v := range components {
		compCopy := *v
		result[k] = &compCopy
	}

	return result
}
