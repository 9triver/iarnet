package provider

import (
	"context"
	"fmt"
	"sync"

	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

const providerType = providerpb.ProviderType_PROVIDER_TYPE_DOCKER

type Service struct {
	providerpb.UnimplementedProviderServiceServer
	mu         sync.RWMutex
	providerID string
	client     *client.Client
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

	return &Service{
		client: cli,
	}, nil
}

// Close 关闭 Docker 客户端连接
func (s *Service) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func (s *Service) AssignID(ctx context.Context, req *providerpb.AssignIDRequest) (*providerpb.AssignIDResponse, error) {
	if req == nil {
		return &providerpb.AssignIDResponse{
			Success: false,
			Error:   "request is nil",
		}, nil
	}

	if req.ProviderId == "" {
		return &providerpb.AssignIDResponse{
			Success: false,
			Error:   "provider ID is required",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.providerID != "" {
		return &providerpb.AssignIDResponse{
			Success: false,
			Error:   fmt.Sprintf("provider ID already assigned: %s", s.providerID),
		}, nil
	}

	s.providerID = req.ProviderId
	logrus.Infof("Provider ID assigned: %s", s.providerID)

	return &providerpb.AssignIDResponse{
		Success:      true,
		ProviderType: providerType,
	}, nil
}

func (s *Service) GetCapacity(ctx context.Context, req *providerpb.GetCapacityRequest) (*providerpb.GetCapacityResponse, error) {
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.providerID
}
