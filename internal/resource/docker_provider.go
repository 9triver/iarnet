package resource

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

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
	client         *client.Client
	providerID     string
	config         DockerConfig
	lastUpdateTime time.Time
	status         Status
	name           string
}

// NewDockerProvider creates a new Docker resource provider
func NewDockerProvider(providerID string, name string, config interface{}) (*DockerProvider, error) {
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
		name:       name,
		config:     dockerConfig,
	}

	dp.lastUpdateTime = time.Now()
	dp.status = StatusConnected

	return dp, nil
}

// GetCapacity returns the total system capacity available to Docker
func (dp *DockerProvider) GetCapacity(ctx context.Context) (*Capacity, error) {
	info, err := dp.client.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker info: %w", err)
	}

	// Convert memory from bytes to GB
	totalMemoryGB := info.MemTotal

	// Get CPU count
	totalCPU := int64(info.NCPU)

	// For GPU, we need to check if nvidia-docker is available
	// This is a simplified approach - in production you might want to use nvidia-ml-go
	// totalGPU := dp.getGPUCount(ctx)

	total := &Info{
		CPU:    totalCPU,
		Memory: totalMemoryGB,
		GPU:    0,
	}

	// Get current usage
	allocated, err := dp.GetAllocated(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current usage: %w", err)
	}

	available := &Info{
		CPU:    total.CPU - allocated.CPU,
		Memory: total.Memory - allocated.Memory,
		GPU:    total.GPU - allocated.GPU,
	}

	return &Capacity{
		Total:     total,
		Used:      allocated,
		Available: available,
	}, nil
}

// GetRealTimeUsage returns current resource usage by all Docker containers
func (dp *DockerProvider) GetAllocated(ctx context.Context) (*Info, error) {
	containers, err := dp.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	logrus.Infof("docker provider get allocated, container count: %d", len(containers))

	var totalCPU, totalMemory int64

	for _, container := range containers {
		// Get container inspect to get resource limits
		inspect, err := dp.client.ContainerInspect(ctx, container.ID)
		if err != nil {
			logrus.Warnf("Failed to inspect container %s: %v", container.ID, err)
			continue
		}

		containerName := inspect.Name
		if len(containerName) > 0 && containerName[0] == '/' {
			containerName = containerName[1:] // Remove leading slash
		}

		// Get CPU limit (convert from nano CPUs to CPU cores)
		var cpuAlloc int64
		if inspect.HostConfig.Resources.NanoCPUs > 0 {
			cpuAlloc = int64(inspect.HostConfig.Resources.NanoCPUs) / 1e6
			logrus.Infof("Container %s: CPU limit set to %d cores", containerName, cpuAlloc)
		} else {
			// If no CPU limit is set, assume the container can use all available CPUs
			// For now, we'll count it as 1 CPU core per container without limits
			cpuAlloc = 1
			logrus.Infof("Container %s: No CPU limit set, assuming %d cores", containerName, cpuAlloc)
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

	return &Info{
		CPU:    totalCPU,
		Memory: totalMemory,
	}, nil
}

// GetType returns the provider type
func (dp *DockerProvider) GetType() string {
	return "docker"
}

// GetProviderID returns the provider ID
func (dp *DockerProvider) GetID() string {
	return dp.providerID
}

func (dp *DockerProvider) GetName() string {
	return dp.name
}

// GetHost returns the Docker daemon host
func (dp *DockerProvider) GetHost() string {
	if dp.config.Host != "" {
		// Parse host from config (e.g., "tcp://192.168.1.100:2376")
		host := dp.config.Host
		host = strings.TrimPrefix(host, "tcp://")
		if strings.Contains(host, ":") {
			return strings.Split(host, ":")[0]
		}
		return host
	}
	return "localhost" // Default for local Docker
}

// GetPort returns the Docker daemon port
func (dp *DockerProvider) GetPort() int {
	if dp.config.Host != "" {
		// Parse port from config (e.g., "tcp://192.168.1.100:2376")
		host := dp.config.Host
		host = strings.TrimPrefix(host, "tcp://")
		if strings.Contains(host, ":") {
			portStr := strings.Split(host, ":")[1]
			if port, err := strconv.Atoi(portStr); err == nil {
				return port
			}
		}
	}
	return 2376 // Default Docker daemon port
}

// IsDockerAvailable checks if Docker daemon is available and accessible
func IsDockerAvailable() bool {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return false
	}
	defer cli.Close()

	// Try to ping Docker daemon
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = cli.Ping(ctx)
	return err == nil
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

	dp.name = "standalone"
	dp.lastUpdateTime = time.Now()
	dp.status = StatusConnected

	return dp, nil
}

// // getGPUCount attempts to detect available GPUs
// // This is a simplified implementation
// func (dp *DockerProvider) getGPUCount(ctx context.Context) float64 {
// 	// Try to get GPU info from Docker info
// 	info, err := dp.client.Info(ctx)
// 	if err != nil {
// 		return 0
// 	}

// 	// Check for nvidia runtime
// 	for runtime := range info.Runtimes {
// 		if runtime == "nvidia" {
// 			// If nvidia runtime is available, assume at least 1 GPU
// 			// In production, you'd want to use nvidia-ml-go or similar
// 			return 1
// 		}
// 	}

// 	// Check environment variable or other methods
// 	// This is a placeholder - implement based on your needs
// 	return 0
// }

func (dp *DockerProvider) GetLastUpdateTime() time.Time {
	return dp.lastUpdateTime
}

// Close closes the Docker client connection
func (dp *DockerProvider) Close() error {
	if dp.client != nil {
		return dp.client.Close()
	}
	return nil
}

func (dp *DockerProvider) GetStatus() Status {
	return dp.status
}

// getDockerHost 解析Docker主机地址，返回主机IP或域名
func (dp *DockerProvider) getDockerHost() string {
	if dp.config.Host == "" {
		// 如果没有配置Host，默认为本地主机
		return "localhost"
	}

	// 解析Docker主机地址，支持格式如：tcp://192.168.1.100:2376
	host := dp.config.Host
	if after, ok := strings.CutPrefix(host, "tcp://"); ok {
		host = after
	}
	if strings.HasPrefix(host, "unix://") {
		// Unix socket连接，表示本地主机
		return "localhost"
	}

	// 提取主机部分（去掉端口）
	if colonIndex := strings.LastIndex(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	return host
}

// createPortBindings 为容器端口创建主机端口映射，让Docker自动分配主机端口
func (dp *DockerProvider) createPortBindings(containerPorts []int) (nat.PortSet, nat.PortMap, error) {
	portSet := nat.PortSet{}
	portMap := nat.PortMap{}

	dockerHost := dp.getDockerHost()
	logrus.Infof("Creating port mappings on Docker host: %s (Docker will auto-assign host ports)", dockerHost)

	for _, containerPort := range containerPorts {
		// 创建容器端口
		port, err := nat.NewPort("tcp", strconv.Itoa(containerPort))
		if err != nil {
			return nil, nil, fmt.Errorf("invalid container port %d: %w", containerPort, err)
		}

		// 添加到端口集合（暴露容器端口）
		portSet[port] = struct{}{}

		// 创建端口绑定，HostPort为空字符串表示让Docker自动分配
		portMap[port] = []nat.PortBinding{
			{
				HostIP:   "0.0.0.0", // 绑定到所有网络接口
				HostPort: "",        // 空字符串让Docker自动分配可用端口
			},
		}

		logrus.Infof("Container port %d will be auto-mapped by Docker on host %s", containerPort, dockerHost)
	}

	return portSet, portMap, nil
}

// logActualPortMappings 获取并记录容器的实际端口映射
func (dp *DockerProvider) logActualPortMappings(ctx context.Context, containerID string) error {
	// 获取容器信息
	containerJSON, err := dp.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	dockerHost := dp.getDockerHost()

	// 记录端口映射信息
	if containerJSON.NetworkSettings != nil && containerJSON.NetworkSettings.Ports != nil {
		logrus.Infof("Actual port mappings for container %s on host %s:", containerID, dockerHost)
		for containerPort, bindings := range containerJSON.NetworkSettings.Ports {
			if len(bindings) > 0 {
				for _, binding := range bindings {
					logrus.Infof("  Container port %s -> Host %s:%s",
						containerPort, binding.HostIP, binding.HostPort)
				}
			}
		}
	} else {
		logrus.Infof("No port mappings found for container %s", containerID)
	}

	return nil
}

func (dp *DockerProvider) Deploy(ctx context.Context, spec ContainerSpec) (string, error) {
	// 创建端口映射
	// var exposedPorts nat.PortSet
	// var portBindings nat.PortMap
	var err error

	// 创建容器配置
	containerConfig := &container.Config{
		Image: spec.Image,
		Cmd:   spec.Command,
		// ExposedPorts: exposedPorts,
	}

	// 创建主机配置
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			NanoCPUs: int64(spec.Requirements.CPU * 1e6), // Rough conversion
			Memory:   int64(spec.Requirements.Memory),
			// GPU: Docker GPU support requires nvidia-docker, assume configured.
		},
		// PortBindings: portBindings,
	}

	// 创建容器
	resp, err := dp.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// 启动容器
	err = dp.client.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	logrus.Infof("Container deployed successfully with ID: %s", resp.ID)
	return resp.ID, nil
}

func (dp *DockerProvider) GetLogs(containerID string, lines int) ([]string, error) {
	logrus.Debugf("Getting logs for container %s, lines: %d", containerID, lines)

	// 创建日志选项
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Tail:       strconv.Itoa(lines),
	}

	// 获取容器日志
	ctx := context.Background()
	logsReader, err := dp.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		logrus.Errorf("Failed to get container logs for %s: %v", containerID, err)
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}
	defer logsReader.Close()

	// 读取日志内容
	var logLines []string
	buffer := make([]byte, 4096)

	for {
		n, err := logsReader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			logrus.Errorf("Error reading container logs for %s: %v", containerID, err)
			return nil, fmt.Errorf("error reading container logs: %w", err)
		}

		if n == 0 {
			break
		}

		// 处理 Docker 多路复用日志格式
		data := buffer[:n]
		offset := 0

		for offset < len(data) {
			// Docker 日志头部格式：[stream_type][0][0][0][size_bytes]
			if offset+8 > len(data) {
				break
			}

			// 读取消息长度（大端序）
			msgSize := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])
			if offset+8+msgSize > len(data) {
				break
			}

			// 提取日志内容
			logContent := string(data[offset+8 : offset+8+msgSize])
			lines := strings.Split(strings.TrimSpace(logContent), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					logLines = append(logLines, strings.TrimSpace(line))
				}
			}

			offset += 8 + msgSize
		}
	}

	// 限制返回的日志行数
	if len(logLines) > lines {
		logLines = logLines[len(logLines)-lines:]
	}

	logrus.Debugf("Successfully retrieved %d log lines for container %s", len(logLines), containerID)
	return logLines, nil
}
