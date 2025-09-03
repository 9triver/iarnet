package resource

import (
	"context"
	"fmt"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

// 全局变量，指向本地 Docker provider 实例
var localDockerProvider *DockerProvider

// DockerConfig holds configuration for Docker connection
type DockerConfig struct {
	// Host is the Docker daemon host (e.g., "tcp://192.168.1.100:2376")
	// If empty, uses local Docker daemon
	Host string

	// TLSCertPath is the path to TLS certificate directory
	// Required for secure remote connections
	TLSCertPath string

	// TLSVerify enables TLS verification
	TLSVerify bool

	// APIVersion specifies Docker API version to use
	// If empty, uses version negotiation
	APIVersion string
}

// DockerProvider implements Provider interface for Docker
type DockerProvider struct {
	client     *client.Client
	providerID string
	config     DockerConfig
}

// NewDockerProvider creates a new Docker resource provider
func NewDockerProvider(providerID string, config interface{}) (*DockerProvider, error) {
	var opts []client.Opt

	if config == nil {
		return nil, fmt.Errorf("empty config type for Docker provider")
	}

	// apply config if specified
	dockerConfig, ok := config.(DockerConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for Docker provider")
	}

	if dockerConfig.Host != "" {
		opts = append(opts, client.WithHost(dockerConfig.Host))
	} else {
		// Use environment variables for local connection
		return nil, fmt.Errorf("host can not be empty")
	}

	// Configure TLS if specified
	if dockerConfig.TLSCertPath != "" {
		opts = append(opts, client.WithTLSClientConfig(dockerConfig.TLSCertPath, "cert.pem", "key.pem"))
	}

	// Configure API version
	if dockerConfig.APIVersion != "" {
		opts = append(opts, client.WithVersion(dockerConfig.APIVersion))
	} else {
		opts = append(opts, client.WithAPIVersionNegotiation())
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	dp := &DockerProvider{
		client:     cli,
		providerID: providerID,
		config:     dockerConfig,
	}

	return dp, nil
}

// GetCapacity returns the total system capacity available to Docker
func (dp *DockerProvider) GetCapacity(ctx context.Context) (*Capacity, error) {
	info, err := dp.client.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker info: %w", err)
	}

	// Convert memory from bytes to GB
	totalMemoryGB := float64(info.MemTotal) / (1024 * 1024 * 1024)

	// Get CPU count
	totalCPU := float64(info.NCPU)

	// For GPU, we need to check if nvidia-docker is available
	// This is a simplified approach - in production you might want to use nvidia-ml-go
	totalGPU := dp.getGPUCount(ctx)

	total := Usage{
		CPU:    totalCPU,
		Memory: totalMemoryGB,
		GPU:    totalGPU,
	}

	// Get current usage
	used, err := dp.GetRealTimeUsage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current usage: %w", err)
	}

	available := Usage{
		CPU:    total.CPU - used.CPU,
		Memory: total.Memory - used.Memory,
		GPU:    total.GPU - used.GPU,
	}

	return &Capacity{
		Total:     total,
		Used:      *used,
		Available: available,
	}, nil
}

// GetRealTimeUsage returns current resource usage by all Docker containers
func (dp *DockerProvider) GetRealTimeUsage(ctx context.Context) (*Usage, error) {
	containers, err := dp.client.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var totalCPU, totalMemory, totalGPU float64

	for _, container := range containers {
		// Get container stats
		stats, err := dp.client.ContainerStats(ctx, container.ID, false)
		if err != nil {
			logrus.Warnf("Failed to get stats for container %s: %v", container.ID, err)
			continue
		}

		// Parse CPU usage
		var statsJSON types.StatsJSON
		if err := stats.Body.Close(); err != nil {
			logrus.Warnf("Failed to close stats body for container %s: %v", container.ID, err)
		}

		// For simplicity, we'll use container inspect to get resource limits
		// In production, you'd want to parse the actual stats JSON
		inspect, err := dp.client.ContainerInspect(ctx, container.ID)
		if err != nil {
			logrus.Warnf("Failed to inspect container %s: %v", container.ID, err)
			continue
		}

		// Get CPU limit (convert from nano CPUs to CPU cores)
		if inspect.HostConfig.Resources.NanoCPUs > 0 {
			totalCPU += float64(inspect.HostConfig.Resources.NanoCPUs) / 1e9
		}

		// Get memory limit (convert from bytes to GB)
		if inspect.HostConfig.Resources.Memory > 0 {
			totalMemory += float64(inspect.HostConfig.Resources.Memory) / (1024 * 1024 * 1024)
		}

		// GPU usage - check for GPU device requests
		if inspect.HostConfig.Resources.DeviceRequests != nil {
			for _, req := range inspect.HostConfig.Resources.DeviceRequests {
				if req.Driver == "nvidia" {
					// Count GPU devices
					if req.Count > 0 {
						totalGPU += float64(req.Count)
					} else if len(req.DeviceIDs) > 0 {
						totalGPU += float64(len(req.DeviceIDs))
					}
				}
			}
		}
	}

	return &Usage{
		CPU:    totalCPU,
		Memory: totalMemory,
		GPU:    totalGPU,
	}, nil
}

// GetProviderType returns the provider type
func (dp *DockerProvider) GetProviderType() string {
	return "docker"
}

// GetProviderID returns the provider ID
func (dp *DockerProvider) GetProviderID() string {
	return dp.providerID
}

// GetLocalDockerProvider creates a new local Docker provider instance
func GetLocalDockerProvider() (*DockerProvider, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create local Docker client: %w", err)
	}

	dp := &DockerProvider{
		client:     cli,
		providerID: "local-docker",
	}

	return dp, nil
}

// getGPUCount attempts to detect available GPUs
// This is a simplified implementation
func (dp *DockerProvider) getGPUCount(ctx context.Context) float64 {
	// Try to get GPU info from Docker info
	info, err := dp.client.Info(ctx)
	if err != nil {
		return 0
	}

	// Check for nvidia runtime
	for runtime := range info.Runtimes {
		if runtime == "nvidia" {
			// If nvidia runtime is available, assume at least 1 GPU
			// In production, you'd want to use nvidia-ml-go or similar
			return 1
		}
	}

	// Check environment variable or other methods
	// This is a placeholder - implement based on your needs
	return 0
}

// Close closes the Docker client connection
func (dp *DockerProvider) Close() error {
	if dp.client != nil {
		return dp.client.Close()
	}
	return nil
}
