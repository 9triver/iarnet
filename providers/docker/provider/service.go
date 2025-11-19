package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

const providerType = "docker"

type Service struct {
	providerpb.UnimplementedServiceServer
	mu      sync.RWMutex
	client  *client.Client
	manager *Manager // 健康检查状态管理器
}

func NewService(host, tlsCertPath string, tlsVerify bool, apiVersion string) (*Service, error) {
	var opts []client.Opt

	if host != "" {
		opts = append(opts, client.WithHost(host))
	} else {
		opts = append(opts, client.FromEnv)
	}

	// 配置 TLS（如果指定）
	if tlsCertPath != "" {
		opts = append(opts, client.WithTLSClientConfig(tlsCertPath, "cert.pem", "key.pem"))
	}

	// 配置 API 版本
	if apiVersion != "" {
		opts = append(opts, client.WithVersion(apiVersion))
	} else {
		opts = append(opts, client.WithAPIVersionNegotiation())
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// 测试连接
	ctx := context.Background()
	_, err = cli.Ping(ctx)
	if err != nil {
		cli.Close()
		return nil, fmt.Errorf("failed to ping Docker daemon: %w", err)
	}

	logrus.Infof("Successfully connected to Docker daemon at %s", host)

	// 创建健康检查管理器
	// 健康检测超时时间：90 秒（假设 iarnet 每 30 秒检测一次，允许 3 次失败）
	// 检查间隔：10 秒
	manager := NewManager(
		90*time.Second,
		10*time.Second,
		func() {
			// 超时回调：清除 provider ID 的逻辑已经在 manager 中处理
			logrus.Debug("Provider ID cleared due to health check timeout")
		},
	)

	service := &Service{
		client:  cli,
		manager: manager,
	}

	// 启动健康检测超时监控
	manager.Start()

	return service, nil
}

// Close 关闭 Docker 客户端连接
func (s *Service) Close() error {
	// 停止健康检测监控
	if s.manager != nil {
		s.manager.Stop()
	}

	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func (s *Service) Connect(ctx context.Context, req *providerpb.ConnectRequest) (*providerpb.ConnectResponse, error) {
	if req == nil {
		return &providerpb.ConnectResponse{
			Success: false,
			Error:   "request is nil",
		}, nil
	}

	if req.ProviderId == "" {
		return &providerpb.ConnectResponse{
			Success: false,
			Error:   "provider ID is required",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.manager.GetProviderID() != "" && s.manager.GetProviderID() != req.ProviderId {
		logrus.Errorf("provider already connected: %s", s.manager.GetProviderID())
		return &providerpb.ConnectResponse{
			Success: false,
			Error:   fmt.Sprintf("provider already connected: %s", s.manager.GetProviderID()),
		}, nil
	}

	// 通过 manager 设置 provider ID（会同时记录健康检测时间）
	if s.manager != nil {
		s.manager.SetProviderID(req.ProviderId)
	}
	logrus.Infof("Provider ID assigned: %s", s.manager.GetProviderID())

	return &providerpb.ConnectResponse{
		Success: true,
		ProviderType: &providerpb.ProviderType{
			Name: providerType,
		},
	}, nil
}

func (s *Service) GetCapacity(ctx context.Context, req *providerpb.GetCapacityRequest) (*providerpb.GetCapacityResponse, error) {
	// 鉴权：如果 provider 已连接，需要验证 provider_id；如果未连接，允许访问
	if err := s.checkAuth(req.ProviderId, true); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	info, err := s.client.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker info: %w", err)
	}

	totalMemory := info.MemTotal        // bytes
	totalCPU := int64(info.NCPU) * 1000 // millicores

	total := &resourcepb.Info{
		Cpu:    totalCPU,
		Memory: totalMemory,
		Gpu:    0, // TODO: add GPU
	}

	// Get current usage
	allocated, err := s.GetAllocated(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get current usage: %w", err)
	}

	available := &resourcepb.Info{
		Cpu:    total.Cpu - allocated.Cpu,
		Memory: total.Memory - allocated.Memory,
		Gpu:    total.Gpu - allocated.Gpu,
	}

	capacity := &resourcepb.Capacity{
		Total:     total,
		Used:      allocated,
		Available: available,
	}
	logrus.Infof("docker provider get capacity, capacity: %v", capacity)

	return &providerpb.GetCapacityResponse{
		Capacity: capacity,
	}, nil
}

func (s *Service) GetAvailable(ctx context.Context, req *providerpb.GetAvailableRequest) (*providerpb.GetAvailableResponse, error) {
	// 鉴权：如果 provider 已连接，需要验证 provider_id；如果未连接，允许访问
	if err := s.checkAuth(req.ProviderId, true); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	capacity, err := s.client.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker info: %w", err)
	}
	totalMemory := capacity.MemTotal        // bytes
	totalCPU := int64(capacity.NCPU) * 1000 // millicores

	total := &resourcepb.Info{
		Cpu:    totalCPU,
		Memory: totalMemory,
		Gpu:    0, // TODO: add GPU
	}
	allocated, err := s.GetAllocated(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get allocated: %w", err)
	}
	return &providerpb.GetAvailableResponse{
		Available: &resourcepb.Info{Cpu: total.Cpu - allocated.Cpu, Memory: total.Memory - allocated.Memory, Gpu: total.Gpu - allocated.Gpu},
	}, nil
}

// GetRealTimeUsage returns current resource usage by all Docker containers
func (s *Service) GetAllocated(ctx context.Context) (*resourcepb.Info, error) {
	containers, err := s.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	logrus.Infof("docker provider get allocated, container count: %d", len(containers))

	var totalCPU, totalMemory int64

	for _, container := range containers {
		// Get container inspect to get resource limits
		inspect, err := s.client.ContainerInspect(ctx, container.ID)
		if err != nil {
			logrus.Warnf("Failed to inspect container %s: %v", container.ID, err)
			continue
		}

		containerName := inspect.Name
		if len(containerName) > 0 && containerName[0] == '/' {
			containerName = containerName[1:] // Remove leading slash
		}

		// Get CPU limit (convert from nano CPUs to millicores)
		var cpuAlloc int64
		if inspect.HostConfig.Resources.NanoCPUs > 0 {
			// Docker NanoCPUs: 1 CPU core = 1e9 NanoCPUs
			// Convert to millicores: 1 CPU core = 1000 millicores
			// So: NanoCPUs / 1e9 * 1000 = NanoCPUs / 1e6
			cpuAlloc = int64(inspect.HostConfig.Resources.NanoCPUs) / 1e6
			logrus.Infof("Container %s: CPU limit set to %d millicores", containerName, cpuAlloc)
		} else {
			// If no CPU limit is set, assume the container can use all available CPUs
			// For now, we'll count it as 1000 millicores (1 CPU core) per container without limits
			cpuAlloc = 1000
			logrus.Infof("Container %s: No CPU limit set, assuming %d millicores", containerName, cpuAlloc)
		}
		totalCPU += cpuAlloc

		// Get memory limit (convert from bytes to GB)
		var memAlloc int64
		if inspect.HostConfig.Resources.Memory > 0 {
			memAlloc = int64(inspect.HostConfig.Resources.Memory)
			logrus.Infof("Container %s: Memory limit set to %d Bytes", containerName, memAlloc)
		} else {
			// If no memory limit is set, assume the container can use a default amount
			// For now, we'll count it as 2GB per container without limits
			memAlloc = 1024 * 1024 * 128 // 128MB
			logrus.Infof("Container %s: No memory limit set, assuming %d Bytes", containerName, memAlloc)
		}
		totalMemory += memAlloc

		// // GPU usage - check for GPU device requests
		// if inspect.HostConfig.Resources.DeviceRequests != nil {
		// 	for _, req := range inspect.HostConfig.Resources.DeviceRequests {
		// 		if req.Driver == "nvidia" {
		// 			// Count GPU devices
		// 			if req.Count > 0 {
		// 				totalGPU += float64(req.Count)
		// 			} else if len(req.DeviceIDs) > 0 {
		// 				totalGPU += float64(len(req.DeviceIDs))
		// 			}
		// 		}
		// 	}
		// }
	}

	logrus.Infof("docker provider get allocated, allocatedCPU: %d, allocatedMemory: %d", totalCPU, totalMemory)

	return &resourcepb.Info{
		Cpu:    totalCPU,
		Memory: totalMemory,
		Gpu:    0, // TODO: add GPU
	}, nil
}

// GetProviderID 获取当前分配的 provider ID
func (s *Service) GetProviderID() string {
	// 优先从 manager 获取（因为 manager 可能已经清除了 ID）
	if s.manager != nil {
		return s.manager.GetProviderID()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.manager.GetProviderID()
}

// checkAuth 检查鉴权
// 如果 provider 已经被连接（有 providerID），则必须验证请求中的 provider_id 是否匹配
// 如果 provider 没有被连接（没有 providerID），则对于 GetCapacity 和 GetAvailable 允许访问（返回 true），对于其他方法返回 false
func (s *Service) checkAuth(requestProviderID string, allowUnconnected bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 如果 provider 还没有被连接
	if s.manager.GetProviderID() == "" {
		if allowUnconnected {
			// 允许未连接的 provider 访问（用于 GetCapacity 和 GetAvailable）
			return nil
		}
		// 其他方法需要先连接
		return fmt.Errorf("provider not connected, please call AssignID first")
	}

	// 如果 provider 已经被连接，必须验证 provider_id
	if requestProviderID == "" {
		return fmt.Errorf("provider_id is required for authenticated requests")
	}

	if requestProviderID != s.manager.GetProviderID() {
		return fmt.Errorf("unauthorized: provider_id mismatch, expected %s, got %s", s.manager.GetProviderID(), requestProviderID)
	}

	return nil
}

func (s *Service) Deploy(ctx context.Context, req *providerpb.DeployRequest) (*providerpb.DeployResponse, error) {
	// 鉴权：DeployComponent 必须验证 provider_id，不允许未连接的 provider 部署
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return &providerpb.DeployResponse{
			Error: fmt.Sprintf("authentication failed: %v", err),
		}, nil
	}

	logrus.WithFields(logrus.Fields{
		"image":            req.Image,
		"env_vars":         req.EnvVars,
		"resource_request": req.ResourceRequest,
	}).Info("docker provider deploy component")
	// 创建容器配置
	containerConfig := &container.Config{
		Image: req.Image,
		Env: func() []string {
			var env []string
			for k, v := range req.EnvVars {
				env = append(env, k+"="+v)
			}
			return env
		}(),
	}

	// 创建主机配置
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			// CPU 单位转换：spec.Requirements.CPU 是毫核心 (millicores)
			// Docker NanoCPUs: 1 CPU core = 1e9 NanoCPUs
			// 1 millicore = 1e6 NanoCPUs
			NanoCPUs: int64(req.ResourceRequest.Cpu * 1e6),
			Memory:   int64(req.ResourceRequest.Memory),
			// GPU: Docker GPU support requires nvidia-docker, assume configured.
		},
		ExtraHosts: []string{
			"host.internal:host-gateway",
		},
		// PortBindings: portBindings,
	}

	// 创建容器
	resp, err := s.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, req.InstanceId)
	if err != nil {
		logrus.Errorf("Failed to create container: %v", err)
		return &providerpb.DeployResponse{
			Error: err.Error(),
		}, nil
	}

	// 启动容器
	err = s.client.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		logrus.Errorf("Failed to start container: %v", err)
		return &providerpb.DeployResponse{
			Error: err.Error(),
		}, nil
	}

	logrus.Infof("Container deployed successfully with ID: %s", resp.ID)
	return &providerpb.DeployResponse{
		Error: "",
	}, nil
}

func (s *Service) HealthCheck(ctx context.Context, req *providerpb.HealthCheckRequest) (*providerpb.HealthCheckResponse, error) {
	// 鉴权：HealthCheck 必须验证 provider_id，不允许未连接的 provider 健康检查
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// 通过 manager 更新最后收到健康检测的时间
	if s.manager != nil {
		s.manager.UpdateHealthCheck()
	}

	return &providerpb.HealthCheckResponse{}, nil
}

func (s *Service) Disconnect(ctx context.Context, req *providerpb.DisconnectRequest) (*providerpb.DisconnectResponse, error) {
	// 鉴权：Disconnect 必须验证 provider_id，不允许未连接的 provider 断开连接
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 通过 manager 清除 provider ID
	if s.manager != nil {
		s.manager.ClearProviderID()
	}
	logrus.Infof("Provider disconnected: %s", req.ProviderId)

	return &providerpb.DisconnectResponse{}, nil
}
