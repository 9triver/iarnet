package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	common "github.com/9triver/iarnet/internal/proto/common"
	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

const providerType = "docker"

type Service struct {
	providerpb.UnimplementedServiceServer
	mu           sync.RWMutex
	client       *client.Client
	manager      *Manager // 健康检查状态管理器
	resourceTags *providerpb.ResourceTags
	network      string // 用于部署 component 容器的网络名称

	// 资源容量管理（从配置文件读取）
	totalCapacity *resourcepb.Info // 配置的总容量
	allocated     *resourcepb.Info // 当前已分配的容量（内存中动态维护）

	// 实例资源信息存储（instance_id -> 资源信息）
	instanceResources map[string]*resourcepb.Info // 存储每个实例的资源信息，用于卸载时恢复资源

	// GPU 管理
	gpuIDs       []int       // 配置的可用 GPU ID 列表
	gpuAllocated map[int]int // GPU ID -> 分配次数（支持多个容器共享同一 GPU）
}

func NewService(host, tlsCertPath string, tlsVerify bool, apiVersion string, network string, resourceTags []string, totalCapacity *resourcepb.Info, gpuIDs []int, allowConnectionFailure bool) (*Service, error) {
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
		if allowConnectionFailure {
			logrus.Warnf("Failed to create Docker client: %v (continuing in test mode)", err)
			cli = nil // 设置为 nil，表示未连接
		} else {
			return nil, fmt.Errorf("failed to create Docker client: %w", err)
		}
	}

	// 测试连接
	if cli != nil {
		ctx := context.Background()
		_, err = cli.Ping(ctx)
		if err != nil {
			if allowConnectionFailure {
				logrus.Warnf("Failed to ping Docker daemon: %v (continuing in test mode)", err)
				cli.Close()
				cli = nil // 设置为 nil，表示未连接
			} else {
				cli.Close()
				return nil, fmt.Errorf("failed to ping Docker daemon: %w", err)
			}
		} else {
			logrus.Infof("Successfully connected to Docker daemon at %s", host)
		}
	} else if allowConnectionFailure {
		logrus.Warnf("Docker client is nil, provider will start in test mode without Docker connection")
	}

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

	// 初始化已分配容量为 0
	allocated := &resourcepb.Info{
		Cpu:    0,
		Memory: 0,
		Gpu:    0,
	}

	// 初始化 GPU 分配映射
	gpuAllocated := make(map[int]int)
	for _, gpuID := range gpuIDs {
		gpuAllocated[gpuID] = 0
	}

	// 初始化实例资源信息存储
	instanceResources := make(map[string]*resourcepb.Info)

	service := &Service{
		client:  cli,
		manager: manager,
		network: network,
		resourceTags: &providerpb.ResourceTags{
			Cpu:    slices.Contains(resourceTags, "cpu"),
			Memory: slices.Contains(resourceTags, "memory"),
			Gpu:    slices.Contains(resourceTags, "gpu"),
			Camera: slices.Contains(resourceTags, "camera"),
		},
		totalCapacity:     totalCapacity,
		allocated:         allocated,
		instanceResources: instanceResources,
		gpuIDs:            gpuIDs,
		gpuAllocated:      gpuAllocated,
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

	// Docker provider 默认支持 Python，可以根据配置添加其他语言
	supportedLanguages := []common.Language{
		common.Language_LANG_PYTHON,
		// 可以根据配置添加 common.Language_LANG_GO 等
	}

	return &providerpb.ConnectResponse{
		Success: true,
		ProviderType: &providerpb.ProviderType{
			Name: providerType,
		},
		SupportedLanguages: supportedLanguages,
	}, nil
}

func (s *Service) GetCapacity(ctx context.Context, req *providerpb.GetCapacityRequest) (*providerpb.GetCapacityResponse, error) {
	// 鉴权：如果 provider 已连接，需要验证 provider_id；如果未连接，允许访问
	if err := s.checkAuth(req.ProviderId, true); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	s.mu.RLock()
	total := s.totalCapacity
	allocated := s.allocated
	s.mu.RUnlock()

	// 必须从配置文件获取容量，如果未配置则返回错误
	if total == nil {
		return nil, fmt.Errorf("resource capacity not configured, please set resource capacity in config file")
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

	s.mu.RLock()
	total := s.totalCapacity
	allocated := s.allocated
	s.mu.RUnlock()

	// 必须从配置文件获取容量，如果未配置则返回错误
	if total == nil {
		return nil, fmt.Errorf("resource capacity not configured, please set resource capacity in config file")
	}

	return &providerpb.GetAvailableResponse{
		Available: &resourcepb.Info{
			Cpu:    total.Cpu - allocated.Cpu,
			Memory: total.Memory - allocated.Memory,
			Gpu:    total.Gpu - allocated.Gpu,
		},
	}, nil
}

// GetAllocated returns current allocated resources (from memory)
// 返回内存中维护的已分配资源
func (s *Service) GetAllocated(ctx context.Context) (*resourcepb.Info, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回内存中维护的已分配资源
	return &resourcepb.Info{
		Cpu:    s.allocated.Cpu,
		Memory: s.allocated.Memory,
		Gpu:    s.allocated.Gpu,
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

	// 检查 Docker 客户端是否可用
	if s.client == nil {
		return &providerpb.DeployResponse{
			Error: "Docker client is not available (test mode or connection failed)",
		}, nil
	}

	// 根据语言选择镜像（如果提供了语言，优先使用语言；否则使用旧的 image 字段以保持向后兼容）
	var image string
	if req.Language != common.Language_LANG_UNKNOWN {
		// 根据语言选择镜像
		switch req.Language {
		case common.Language_LANG_PYTHON:
			image = "iarnet/component:python_3.11-latest"
		case common.Language_LANG_GO:
			image = "golang:1.21-alpine" // 默认 Go 镜像，可以根据配置修改
		default:
			return &providerpb.DeployResponse{
				Error: fmt.Sprintf("unsupported language: %v", req.Language),
			}, nil
		}
		logrus.Infof("Selected image %s for language %v", image, req.Language)
	} else if req.Image != "" {
		// 向后兼容：如果提供了 image，使用它
		image = req.Image
		logrus.Warnf("Using deprecated image field: %s. Consider using language field instead.", image)
	} else {
		return &providerpb.DeployResponse{
			Error: "either language or image must be provided",
		}, nil
	}

	logrus.WithFields(logrus.Fields{
		"image":            image,
		"language":         req.Language,
		"env_vars":         req.EnvVars,
		"resource_request": req.ResourceRequest,
	}).Info("docker provider deploy component")

	// 获取 provider ID 用于标记容器
	providerID := s.manager.GetProviderID()

	// 创建容器配置
	containerConfig := &container.Config{
		Image: image,
		Env: func() []string {
			var env []string
			for k, v := range req.EnvVars {
				env = append(env, k+"="+v)
			}
			return env
		}(),
		Labels: map[string]string{
			"iarnet.provider_id": providerID,
			"iarnet.managed":     "true",
			"iarnet.instance_id": req.InstanceId, // 用于 Undeploy 时查找容器
		},
	}

	// 创建主机配置
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			// CPU 单位转换：spec.Requirements.CPU 是毫核心 (millicores)
			// Docker NanoCPUs: 1 CPU core = 1e9 NanoCPUs
			// 1 millicore = 1e6 NanoCPUs
			NanoCPUs: int64(req.ResourceRequest.Cpu * 1e6),
			Memory:   int64(req.ResourceRequest.Memory),
		},
		ExtraHosts: []string{
			"host.internal:host-gateway",
		},
		Mounts: func() []mount.Mount {
			var mounts []mount.Mount

			// 定义需要挂载的目录映射
			mountConfigs := []struct {
				source   string
				target   string
				readOnly bool
			}{
				{
					source:   "/home/xhy/iarnet-demo/eps_helmet/images",
					target:   "/app/images",
					readOnly: false,
				},
				{
					source:   "/home/xhy/iarnet-demo/eps_helmet",
					target:   "/app/eps_helmet",
					readOnly: false,
				},
			}

			// 检查每个目录是否存在，只挂载存在的目录
			for _, cfg := range mountConfigs {
				// 检查源路径是否存在
				if info, err := os.Stat(cfg.source); err == nil {
					// 如果是目录，直接使用
					if info.IsDir() {
						mounts = append(mounts, mount.Mount{
							Type:     mount.TypeBind,
							Source:   cfg.source,
							Target:   cfg.target,
							ReadOnly: cfg.readOnly,
						})
						logrus.Infof("Mounting directory: %s -> %s", cfg.source, cfg.target)
					} else {
						// 如果是文件，也允许挂载
						mounts = append(mounts, mount.Mount{
							Type:     mount.TypeBind,
							Source:   cfg.source,
							Target:   cfg.target,
							ReadOnly: cfg.readOnly,
						})
						logrus.Infof("Mounting file: %s -> %s", cfg.source, cfg.target)
					}
				} else {
					// 目录不存在，记录警告但不挂载
					logrus.Warnf("Skipping mount: source path does not exist: %s (target: %s)", cfg.source, cfg.target)
				}
			}

			return mounts
		}(),
		// Runtime: "nvidia",
		// PortBindings: portBindings,
	}

	// 配置 GPU 设备请求
	if req.ResourceRequest.Gpu > 0 {
		allocatedGPUs := s.allocateGPUs(int(req.ResourceRequest.Gpu))
		if len(allocatedGPUs) > 0 {
			// 将 GPU ID 转换为字符串列表
			gpuIDStrs := make([]string, len(allocatedGPUs))
			for i, id := range allocatedGPUs {
				gpuIDStrs[i] = strconv.Itoa(id)
			}
			hostConfig.Resources.DeviceRequests = []container.DeviceRequest{
				{
					Driver:    "nvidia",
					DeviceIDs: gpuIDStrs,
					Capabilities: [][]string{
						{"gpu"},
					},
				},
			}
			logrus.Infof("Allocated GPUs for container: %v", allocatedGPUs)
		} else {
			logrus.Warnf("No GPU IDs configured, using default GPU allocation")
			// // 如果没有配置 GPU ID，使用 count 方式分配
			// hostConfig.Resources.DeviceRequests = []container.DeviceRequest{
			// 	{
			// 		Driver: "nvidia",
			// 		Count:  int(req.ResourceRequest.Gpu),
			// 		Capabilities: [][]string{
			// 			{"gpu"},
			// 		},
			// 	},
			// }
		}
	}

	// 配置网络（如果指定了网络名称）
	var networkingConfig *network.NetworkingConfig
	if s.network != "" {
		networkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				s.network: {},
			},
		}
		logrus.Infof("Deploying container to network: %s", s.network)
	}

	// 创建容器
	resp, err := s.client.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, req.InstanceId)
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

	// 更新已分配的资源容量（在内存中维护）
	// 同时保存该实例的资源信息，用于卸载时恢复资源
	s.mu.Lock()
	s.allocated.Cpu += req.ResourceRequest.Cpu
	s.allocated.Memory += req.ResourceRequest.Memory
	s.allocated.Gpu += req.ResourceRequest.Gpu
	// 保存实例资源信息到内存map中
	s.instanceResources[req.InstanceId] = &resourcepb.Info{
		Cpu:    req.ResourceRequest.Cpu,
		Memory: req.ResourceRequest.Memory,
		Gpu:    req.ResourceRequest.Gpu,
	}
	s.mu.Unlock()

	logrus.Infof("Container deployed successfully with ID: %s, allocated resources: CPU=%d, Memory=%d, GPU=%d",
		resp.ID, req.ResourceRequest.Cpu, req.ResourceRequest.Memory, req.ResourceRequest.Gpu)
	return &providerpb.DeployResponse{
		Error: "",
	}, nil
}

// Undeploy 移除已部署的 component（停止并删除容器）
func (s *Service) Undeploy(ctx context.Context, req *providerpb.UndeployRequest) (*providerpb.UndeployResponse, error) {
	// 鉴权：Undeploy 必须验证 provider_id，不允许未连接的 provider 移除
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return &providerpb.UndeployResponse{
			Error: fmt.Sprintf("authentication failed: %v", err),
		}, nil
	}

	// 检查 Docker 客户端是否可用
	if s.client == nil {
		return &providerpb.UndeployResponse{
			Error: "Docker client is not available (test mode or connection failed)",
		}, nil
	}

	logrus.WithFields(logrus.Fields{
		"instance_id": req.InstanceId,
	}).Info("docker provider undeploy component")

	// 查找容器（通过 instance_id，即容器名称）
	containerID := req.InstanceId

	// 先尝试通过容器名称查找容器
	containers, err := s.client.ContainerList(ctx, container.ListOptions{
		All: true, // 包括已停止的容器
	})
	if err != nil {
		logrus.Errorf("Failed to list containers: %v", err)
		return &providerpb.UndeployResponse{
			Error: fmt.Sprintf("failed to list containers: %v", err),
		}, nil
	}

	// 查找匹配的容器（通过名称或 ID）
	// 优先通过标签查找（iarnet.instance_id 标签）
	var targetContainer *container.Summary
	providerID := s.manager.GetProviderID()

	for _, c := range containers {
		// 首先检查标签（如果容器有 iarnet.instance_id 标签，优先使用）
		if c.Labels != nil {
			if instanceID, ok := c.Labels["iarnet.instance_id"]; ok && instanceID == containerID {
				// 验证 provider_id 是否匹配
				if providerIDLabel, ok := c.Labels["iarnet.provider_id"]; ok && providerIDLabel == providerID {
					targetContainer = &c
					break
				}
			}
		}

		// 如果没有找到，检查容器名称是否匹配 instance_id
		if targetContainer == nil && c.Names != nil {
			for _, name := range c.Names {
				// 容器名称通常以 / 开头，需要去掉
				cleanName := strings.TrimPrefix(name, "/")
				if cleanName == containerID || strings.HasSuffix(cleanName, containerID) {
					// 验证 provider_id 是否匹配（通过标签）
					if c.Labels != nil {
						if providerIDLabel, ok := c.Labels["iarnet.provider_id"]; ok && providerIDLabel == providerID {
							targetContainer = &c
							break
						}
					}
				}
			}
		}

		// 最后检查容器 ID 是否匹配（支持短 ID 和长 ID）
		if targetContainer == nil {
			if c.ID == containerID || strings.HasPrefix(c.ID, containerID) {
				// 验证 provider_id 是否匹配（通过标签）
				if c.Labels != nil {
					if providerIDLabel, ok := c.Labels["iarnet.provider_id"]; ok && providerIDLabel == providerID {
						targetContainer = &c
						break
					}
				}
			}
		}

		if targetContainer != nil {
			break
		}
	}

	if targetContainer == nil {
		logrus.Warnf("Container not found for instance_id: %s", containerID)
		// 容器不存在，尝试从内存中清理资源信息（幂等性）
		s.mu.Lock()
		if resources, exists := s.instanceResources[containerID]; exists {
			// 如果内存中有该实例的资源信息，释放资源
			s.allocated.Cpu -= resources.Cpu
			if s.allocated.Cpu < 0 {
				s.allocated.Cpu = 0
			}
			s.allocated.Memory -= resources.Memory
			if s.allocated.Memory < 0 {
				s.allocated.Memory = 0
			}
			s.allocated.Gpu -= resources.Gpu
			if s.allocated.Gpu < 0 {
				s.allocated.Gpu = 0
			}
			// 释放 GPU ID 分配
			s.releaseGPUs(int(resources.Gpu))
			// 删除内存中的资源信息
			delete(s.instanceResources, containerID)
			logrus.Infof("Cleaned up resources from memory for instance_id: %s", containerID)
		}
		s.mu.Unlock()
		// 容器不存在，返回成功（幂等性）
		return &providerpb.UndeployResponse{
			Error: "",
		}, nil
	}

	// 从内存中获取该实例的资源信息（部署时保存的）
	var cpu int64
	var memory int64
	var gpu int64
	foundInMemory := false

	// 先尝试从内存中读取资源信息（读锁）
	s.mu.RLock()
	if resources, exists := s.instanceResources[containerID]; exists {
		// 使用内存中保存的资源信息
		cpu = resources.Cpu
		memory = resources.Memory
		gpu = resources.Gpu
		foundInMemory = true
		logrus.Infof("Retrieved resources from memory for instance_id: %s, CPU=%d, Memory=%d, GPU=%d",
			containerID, cpu, memory, gpu)
	}
	s.mu.RUnlock()

	// 如果内存中没有资源信息（可能是旧版本部署的容器），尝试从容器信息中获取
	if !foundInMemory {
		logrus.Warnf("Resource info not found in memory for instance_id: %s, trying to get from container", containerID)
		// 获取容器信息以获取资源使用情况
		containerInfo, err := s.client.ContainerInspect(ctx, targetContainer.ID)
		if err != nil {
			logrus.Warnf("Failed to inspect container %s: %v, continuing with undeploy", targetContainer.ID, err)
		} else {
			if containerInfo.HostConfig != nil {
				// CPU: NanoCPUs -> millicores
				// 1 CPU core = 1e9 NanoCPUs, 1 millicore = 1e6 NanoCPUs
				if containerInfo.HostConfig.Resources.NanoCPUs > 0 {
					cpu = containerInfo.HostConfig.Resources.NanoCPUs / 1e6
				}
				// Memory: bytes
				if containerInfo.HostConfig.Resources.Memory > 0 {
					memory = containerInfo.HostConfig.Resources.Memory
				}
				// GPU: 从 DeviceRequests 中获取
				if len(containerInfo.HostConfig.Resources.DeviceRequests) > 0 {
					for _, dr := range containerInfo.HostConfig.Resources.DeviceRequests {
						if dr.Driver != "" && strings.Contains(strings.ToLower(dr.Driver), "nvidia") {
							if len(dr.DeviceIDs) > 0 {
								gpu += int64(len(dr.DeviceIDs))
							} else if dr.Count > 0 {
								gpu += int64(dr.Count)
							}
						}
					}
				}
			}
		}
	}

	// 释放资源（使用内存中保存的数据或从容器信息中获取的数据）
	if cpu > 0 || memory > 0 || gpu > 0 {
		s.mu.Lock()
		s.allocated.Cpu -= cpu
		if s.allocated.Cpu < 0 {
			s.allocated.Cpu = 0
		}
		s.allocated.Memory -= memory
		if s.allocated.Memory < 0 {
			s.allocated.Memory = 0
		}
		s.allocated.Gpu -= gpu
		if s.allocated.Gpu < 0 {
			s.allocated.Gpu = 0
		}
		// 释放 GPU ID 分配
		s.releaseGPUs(int(gpu))
		// 删除内存中的资源信息（如果存在）
		delete(s.instanceResources, containerID)
		// 在锁内读取剩余资源值用于日志
		remainingCpu := s.allocated.Cpu
		remainingMemory := s.allocated.Memory
		remainingGpu := s.allocated.Gpu
		s.mu.Unlock()
		logrus.Infof("Released resources: CPU=%d, Memory=%d, GPU=%d, remaining allocated: CPU=%d, Memory=%d, GPU=%d",
			cpu, memory, gpu, remainingCpu, remainingMemory, remainingGpu)
	}

	// 停止容器（如果正在运行）
	if targetContainer.State == "running" {
		timeout := 30 // 30 秒超时
		options := container.StopOptions{
			Timeout: &timeout,
		}
		if err := s.client.ContainerStop(ctx, targetContainer.ID, options); err != nil {
			logrus.Warnf("Failed to stop container %s: %v, continuing with removal", targetContainer.ID, err)
		} else {
			logrus.Infof("Stopped container: %s", targetContainer.ID)
		}
	}

	// 删除容器
	removeOptions := container.RemoveOptions{
		Force: true, // 强制删除，即使容器正在运行
	}
	if err := s.client.ContainerRemove(ctx, targetContainer.ID, removeOptions); err != nil {
		logrus.Errorf("Failed to remove container %s: %v", targetContainer.ID, err)
		return &providerpb.UndeployResponse{
			Error: fmt.Sprintf("failed to remove container: %v", err),
		}, nil
	}

	logrus.Infof("Container undeployed successfully: %s", targetContainer.ID)
	return &providerpb.UndeployResponse{
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

	s.mu.RLock()
	total := s.totalCapacity
	allocated := s.allocated
	s.mu.RUnlock()

	// 必须从配置文件获取容量，如果未配置则返回错误
	if total == nil {
		return nil, fmt.Errorf("resource capacity not configured, please set resource capacity in config file")
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

	// // 设置资源标签
	// // Docker provider 支持 CPU 和 Memory
	// // GPU 支持取决于是否配置了 nvidia-docker（这里假设支持，实际可以检测）
	// // Camera 不支持（Docker 通常不支持摄像头设备）
	// resourceTags := &providerpb.ResourceTags{
	// 	Cpu:    true,  // Docker 支持 CPU
	// 	Memory: true,  // Docker 支持内存
	// 	Gpu:    false, // GPU 需要 nvidia-docker，默认设为 false，可以根据实际情况检测
	// 	Camera: false, // Docker 不支持摄像头
	// }
	resourceTags := s.resourceTags

	// Docker provider 默认支持 Python，可以根据配置添加其他语言
	supportedLanguages := []common.Language{
		common.Language_LANG_PYTHON,
		// 可以根据配置添加 common.Language_LANG_GO 等
	}

	return &providerpb.HealthCheckResponse{
		Capacity:           capacity,
		ResourceTags:       resourceTags,
		SupportedLanguages: supportedLanguages,
	}, nil
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

// allocateGPUs 分配指定数量的 GPU，返回分配的 GPU ID 列表
// 优先分配使用次数最少的 GPU（负载均衡）
func (s *Service) allocateGPUs(count int) []int {
	if len(s.gpuIDs) == 0 || count <= 0 {
		return nil
	}

	// 复制 gpuIDs 并按分配次数排序
	type gpuUsage struct {
		id    int
		count int
	}
	gpuList := make([]gpuUsage, len(s.gpuIDs))
	for i, id := range s.gpuIDs {
		gpuList[i] = gpuUsage{id: id, count: s.gpuAllocated[id]}
	}

	// 按分配次数升序排序（优先分配使用次数少的 GPU）
	for i := 0; i < len(gpuList)-1; i++ {
		for j := i + 1; j < len(gpuList); j++ {
			if gpuList[i].count > gpuList[j].count {
				gpuList[i], gpuList[j] = gpuList[j], gpuList[i]
			}
		}
	}

	// 分配 GPU
	allocatedCount := count
	if allocatedCount > len(gpuList) {
		allocatedCount = len(gpuList)
	}

	allocated := make([]int, allocatedCount)
	for i := 0; i < allocatedCount; i++ {
		allocated[i] = gpuList[i].id
		s.gpuAllocated[gpuList[i].id]++
	}

	return allocated
}

// releaseGPUs 释放指定数量的 GPU
func (s *Service) releaseGPUs(count int) {
	if len(s.gpuIDs) == 0 || count <= 0 {
		return
	}

	// 按分配次数降序排序（优先释放使用次数多的 GPU）
	type gpuUsage struct {
		id    int
		count int
	}
	gpuList := make([]gpuUsage, 0, len(s.gpuIDs))
	for _, id := range s.gpuIDs {
		if s.gpuAllocated[id] > 0 {
			gpuList = append(gpuList, gpuUsage{id: id, count: s.gpuAllocated[id]})
		}
	}

	// 按分配次数降序排序
	for i := 0; i < len(gpuList)-1; i++ {
		for j := i + 1; j < len(gpuList); j++ {
			if gpuList[i].count < gpuList[j].count {
				gpuList[i], gpuList[j] = gpuList[j], gpuList[i]
			}
		}
	}

	// 释放 GPU
	released := 0
	for i := 0; i < len(gpuList) && released < count; i++ {
		s.gpuAllocated[gpuList[i].id]--
		released++
	}
}

// ReleaseResources 释放已分配的资源（当容器停止或删除时调用）
// 这是一个内部方法，用于在容器停止/删除时释放资源
func (s *Service) ReleaseResources(cpu, memory, gpu int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.allocated.Cpu -= cpu
	if s.allocated.Cpu < 0 {
		s.allocated.Cpu = 0
	}
	s.allocated.Memory -= memory
	if s.allocated.Memory < 0 {
		s.allocated.Memory = 0
	}
	s.allocated.Gpu -= gpu
	if s.allocated.Gpu < 0 {
		s.allocated.Gpu = 0
	}

	// 释放 GPU ID 分配
	s.releaseGPUs(int(gpu))

	logrus.Infof("Released resources: CPU=%d, Memory=%d, GPU=%d, remaining allocated: CPU=%d, Memory=%d, GPU=%d",
		cpu, memory, gpu, s.allocated.Cpu, s.allocated.Memory, s.allocated.Gpu)
}

// GetRealTimeUsage 获取实时资源使用情况
// 统计该 provider 部署的所有 component 容器的实时负载
// 使用 Docker 流式 Stats API 获取真实的实时数据
func (s *Service) GetRealTimeUsage(ctx context.Context, req *providerpb.GetRealTimeUsageRequest) (*providerpb.GetRealTimeUsageResponse, error) {
	// 鉴权：必须验证 provider_id
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	providerID := s.manager.GetProviderID()
	if providerID == "" {
		return nil, fmt.Errorf("provider not connected")
	}

	// 检查 Docker 客户端是否可用
	if s.client == nil {
		return &providerpb.GetRealTimeUsageResponse{
			Usage: &resourcepb.Info{
				Cpu:    0,
				Memory: 0,
				Gpu:    0,
			},
		}, nil
	}

	// 列出所有容器
	containers, err := s.client.ContainerList(ctx, container.ListOptions{
		All: false, // 只列出运行中的容器
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var totalCpu int64    // millicores
	var totalMemory int64 // bytes
	var totalGpu int64    // GPU 数量（Docker 中通常通过设备数量统计）

	// 过滤出由该 provider 部署的容器
	var targetContainers []container.Summary
	for _, c := range containers {
		if c.Labels["iarnet.provider_id"] == providerID {
			targetContainers = append(targetContainers, c)
		}
	}

	if len(targetContainers) == 0 {
		return &providerpb.GetRealTimeUsageResponse{
			Usage: &resourcepb.Info{
				Cpu:    0,
				Memory: 0,
				Gpu:    0,
			},
		}, nil
	}

	// 使用并发处理容器，避免阻塞
	type containerUsage struct {
		cpu    int64
		memory int64
		gpu    int64
	}

	var wg sync.WaitGroup
	usageChan := make(chan containerUsage, len(targetContainers))
	// 限制并发数，避免过多 goroutine
	maxConcurrency := 10
	semaphore := make(chan struct{}, maxConcurrency)

	for _, c := range targetContainers {
		wg.Add(1)
		semaphore <- struct{}{} // 获取信号量
		go func(containerID string) {
			defer wg.Done()
			defer func() { <-semaphore }() // 释放信号量

			// 创建独立的上下文，避免被取消影响其他容器
			containerCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// 获取容器信息（用于获取 CPU 限制和 GPU 设备）
			containerInfo, err := s.client.ContainerInspect(containerCtx, containerID)
			if err != nil {
				logrus.Warnf("Failed to inspect container %s: %v", containerID, err)
				usageChan <- containerUsage{0, 0, 0}
				return
			}

			// 使用流式 API 获取实时统计信息
			// 需要获取两个时间点的数据才能准确计算 CPU 使用率
			stats, err := s.client.ContainerStats(containerCtx, containerID, true) // stream=true 启用流式 API
			if err != nil {
				logrus.Warnf("Failed to get stats for container %s: %v", containerID, err)
				usageChan <- containerUsage{0, 0, 0}
				return
			}

			// 创建带超时的上下文，用于读取统计流
			statsCtx, statsCancel := context.WithTimeout(containerCtx, 3*time.Second)

			decoder := json.NewDecoder(stats.Body)
			var firstStats *container.StatsResponse
			var secondStats *container.StatsResponse

			// 读取第一个数据点
			var v1 container.StatsResponse
			if err := decoder.Decode(&v1); err != nil {
				stats.Body.Close()
				statsCancel()
				logrus.Warnf("Failed to decode first stats for container %s: %v", containerID, err)
				usageChan <- containerUsage{0, 0, 0}
				return
			}
			firstStats = &v1

			// 等待一段时间（1秒）以获取第二个数据点，用于计算 CPU 使用率
			select {
			case <-time.After(1 * time.Second):
				// 读取第二个数据点
				var v2 container.StatsResponse
				if err := decoder.Decode(&v2); err != nil {
					logrus.Warnf("Failed to decode second stats for container %s: %v", containerID, err)
					// 如果无法获取第二个数据点，使用第一个数据点的内存信息
					secondStats = firstStats
				} else {
					secondStats = &v2
				}
			case <-statsCtx.Done():
				logrus.Warnf("Timeout waiting for second stats for container %s", containerID)
				secondStats = firstStats
			}

			// 关闭统计流和取消上下文
			stats.Body.Close()
			statsCancel()

			// 使用第二个数据点（包含 PreCPUStats）来计算 CPU 使用率
			containerCpu := calculateRealTimeCPU(secondStats, containerInfo)

			// 内存使用（bytes）- 使用实际占用的内存
			var containerMemory int64
			if secondStats.MemoryStats.Usage > 0 {
				containerMemory = int64(secondStats.MemoryStats.Usage)
			}

			// GPU 使用：获取实际的 GPU 使用率（如果可用）
			// 优先尝试从 nvidia-smi 获取实时 GPU 使用率
			containerGpu := getContainerGPUUsage(containerID, containerInfo)

			usageChan <- containerUsage{
				cpu:    containerCpu,
				memory: containerMemory,
				gpu:    containerGpu,
			}
		}(c.ID)
	}

	// 等待所有 goroutine 完成
	go func() {
		wg.Wait()
		close(usageChan)
	}()

	// 聚合所有容器的使用量
	for usage := range usageChan {
		totalCpu += usage.cpu
		totalMemory += usage.memory
		totalGpu += usage.gpu
	}

	return &providerpb.GetRealTimeUsageResponse{
		Usage: &resourcepb.Info{
			Cpu:    totalCpu,
			Memory: totalMemory,
			Gpu:    totalGpu,
		},
	}, nil
}

// calculateRealTimeCPU 计算容器实时使用的 CPU（millicores）
// 直接使用 Docker stats API 返回的实际 CPU 使用数据，不基于分配资源计算
// 计算方式与 docker stats 命令和 Docker Desktop 完全一致
func calculateRealTimeCPU(v *container.StatsResponse, containerInfo container.InspectResponse) int64 {
	// 检查是否有 CPU 统计数据
	if v.CPUStats.CPUUsage.TotalUsage == 0 || v.PreCPUStats.CPUUsage.TotalUsage == 0 {
		return 0
	}

	// 计算 CPU 使用时间差（nanoseconds）
	// cpuDelta: 容器在这段时间内实际使用的 CPU 时间
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
	// systemDelta: 系统在这段时间内的总 CPU 时间（所有核心的总和）
	systemDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)

	if systemDelta <= 0.0 || cpuDelta <= 0.0 {
		return 0
	}

	// 获取系统 CPU 核心数（从统计信息中获取，这是实际可用的 CPU 核心数）
	numCPU := len(v.CPUStats.CPUUsage.PercpuUsage)
	if numCPU == 0 {
		// 如果无法获取，尝试从 OnlineCPUs 获取
		if v.CPUStats.OnlineCPUs > 0 {
			numCPU = int(v.CPUStats.OnlineCPUs)
		} else {
			numCPU = 1 // 默认假设 1 个核心
		}
	}

	// Docker stats 标准计算方式：
	// CPU 使用率 = (cpuDelta / systemDelta) × numCPU
	// 这个值表示容器实际使用的 CPU 核心数（相对于系统总 CPU）
	// 例如：如果系统有 8 核，cpuDelta/systemDelta = 0.5，则容器使用了 0.5 × 8 = 4 个核心

	// 计算实际使用的 CPU 核心数
	cpuUsageRatio := cpuDelta / systemDelta
	usedCPUCores := cpuUsageRatio * float64(numCPU)

	// 转换为 millicores（1 core = 1000 millicores）
	// 例如：0.5 核心 = 500 millicores, 4 核心 = 4000 millicores
	usedCPUMillicores := int64(usedCPUCores * 1000.0)

	return usedCPUMillicores
}

// getContainerGPUUsage 获取容器的 GPU 使用情况
// 如果安装了 nvidia-container-toolkit，尝试从 nvidia-smi 获取该容器进程的实际 GPU 使用量
// 否则回退到统计分配的 GPU 设备数量
func getContainerGPUUsage(containerID string, containerInfo container.InspectResponse) int64 {
	// 首先检查容器是否分配了 GPU
	var gpuCount int64
	if containerInfo.HostConfig != nil && len(containerInfo.HostConfig.DeviceRequests) > 0 {
		for _, dr := range containerInfo.HostConfig.DeviceRequests {
			// 检查是否是 GPU 设备请求
			if dr.Driver != "" && strings.Contains(strings.ToLower(dr.Driver), "nvidia") {
				// 统计分配的 GPU 数量
				if len(dr.DeviceIDs) > 0 {
					gpuCount += int64(len(dr.DeviceIDs))
				} else if dr.Count > 0 {
					gpuCount += int64(dr.Count)
				}
			}
		}
	}

	// 如果没有分配 GPU，返回 0
	if gpuCount == 0 {
		return 0
	}

	// 尝试通过 nvidia-smi 获取该容器进程的实际 GPU 使用量
	// 通过查询容器内的进程 PID 来匹配 GPU 使用情况
	containerGPUUsage, err := getContainerGPUUsageFromNvidiaSMI(containerID, containerInfo)
	if err != nil {
		// 如果无法获取实时使用量，回退到统计分配的 GPU 数量
		logrus.Debugf("Failed to get GPU usage from nvidia-smi for container %s: %v, falling back to device count", containerID, err)
		return gpuCount
	}

	return containerGPUUsage
}

// getContainerGPUUsageFromNvidiaSMI 通过 nvidia-smi 获取容器进程的实际 GPU 使用量
// 通过查询容器内的进程 PID 来匹配 GPU 使用情况
func getContainerGPUUsageFromNvidiaSMI(containerID string, containerInfo container.InspectResponse) (int64, error) {
	// 获取容器内的进程 PID 列表
	containerPIDs, err := getContainerPIDs(containerID)
	if err != nil {
		return 0, fmt.Errorf("failed to get container PIDs: %w", err)
	}

	if len(containerPIDs) == 0 {
		// 容器内没有进程，返回 0
		return 0, nil
	}

	// 执行 nvidia-smi 命令获取使用 GPU 的进程信息
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 查询所有使用 GPU 的进程：pid, gpu_uuid, used_memory
	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-compute-apps=pid,gpu_uuid,used_memory", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to execute nvidia-smi: %w", err)
	}

	// 解析输出，找出属于该容器的进程
	// 格式示例：12345, GPU-12345678-1234-1234-1234-123456789abc, 1024\n
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	// 统计该容器使用的 GPU 数量（基于进程使用的 GPU）
	usedGPUs := make(map[string]bool) // GPU UUID -> 是否被使用

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}

		// 解析 PID
		pidStr := strings.TrimSpace(parts[0])
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// 检查这个 PID 是否属于该容器
		isContainerPID := false
		for _, containerPID := range containerPIDs {
			if pid == containerPID {
				isContainerPID = true
				break
			}
		}

		if isContainerPID {
			// 这个进程属于该容器，记录它使用的 GPU
			if len(parts) >= 2 {
				gpuUUID := strings.TrimSpace(parts[1])
				if gpuUUID != "" {
					usedGPUs[gpuUUID] = true
				}
			}
		}
	}

	// 返回该容器实际使用的 GPU 数量
	return int64(len(usedGPUs)), nil
}

// getContainerPIDs 获取容器内的所有进程 PID
func getContainerPIDs(containerID string) ([]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 使用 docker top 命令获取容器内的进程 PID
	cmd := exec.CommandContext(ctx, "docker", "top", containerID, "-o", "pid")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute docker top: %w", err)
	}

	// 解析输出，跳过第一行（表头）
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var pids []int

	for i, line := range lines {
		if i == 0 {
			// 跳过表头
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}

	return pids, nil
}
