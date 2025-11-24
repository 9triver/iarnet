package resource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/logger"
	"github.com/9triver/iarnet/internal/domain/resource/provider"
	"github.com/9triver/iarnet/internal/domain/resource/scheduler"
	"github.com/9triver/iarnet/internal/domain/resource/store"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	providerrepo "github.com/9triver/iarnet/internal/infra/repository/resource"
	commonpb "github.com/9triver/iarnet/internal/proto/common"
	registrypb "github.com/9triver/iarnet/internal/proto/global/registry"
	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	schedulerpb "github.com/9triver/iarnet/internal/proto/resource/scheduler"
	storepb "github.com/9triver/iarnet/internal/proto/resource/store"
	"github.com/9triver/iarnet/internal/util"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	_ store.Service     = (*Manager)(nil)
	_ component.Service = (*Manager)(nil)
	_ logger.Service    = (*Manager)(nil)
)

type Manager struct {
	storepb.UnimplementedServiceServer
	componentService   component.Service
	storeService       store.Service
	providerService    provider.Service
	componentManager   component.Manager
	providerManager    *provider.Manager
	loggerService      logger.Service
	nodeID             string
	name               string
	description        string
	domainID           string
	domainName         string
	isHead             bool
	globalRegistryAddr string        // 全局注册中心地址
	nodeAddress        string        // 节点地址 (host:port)，用于健康检查上报
	healthCheckStop    chan struct{} // 用于停止健康检查 goroutine
	discoveryService   discovery.Service
	schedulerService   scheduler.Service
}

// loadOrGenerateNodeID 从文件加载节点 ID，如果不存在则生成新的并保存
func loadOrGenerateNodeID(dataDir string) string {
	if dataDir == "" {
		dataDir = "./data"
	}

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logrus.Warnf("Failed to create data directory %s: %v, using generated node ID", dataDir, err)
		return util.GenIDWith("node.")
	}

	nodeIDFile := filepath.Join(dataDir, "node_id")

	// 尝试从文件加载节点 ID
	if data, err := os.ReadFile(nodeIDFile); err == nil {
		nodeID := string(data)
		if nodeID != "" {
			logrus.Infof("Loaded existing node ID from %s: %s", nodeIDFile, nodeID)
			return nodeID
		}
	}

	// 文件不存在或内容为空，生成新的节点 ID
	nodeID := util.GenIDWith("node.")

	// 保存节点 ID 到文件
	if err := os.WriteFile(nodeIDFile, []byte(nodeID), 0644); err != nil {
		logrus.Warnf("Failed to save node ID to %s: %v, node ID will be regenerated on next restart", nodeIDFile, err)
	} else {
		logrus.Infof("Generated and saved new node ID to %s: %s", nodeIDFile, nodeID)
	}

	return nodeID
}

func NewManager(channeler component.Channeler, s *store.Store, componentImages map[string]string, providerRepo providerrepo.ProviderRepo, envVariables *provider.EnvVariables, name string, description string, domainID string, dataDir string) *Manager {
	componentManager := component.NewManager(channeler)

	// 初始化 Provider 模块
	providerManager := provider.NewManager()
	providerService := provider.NewService(providerManager, providerRepo, envVariables)

	// 加载或生成节点 ID
	nodeID := loadOrGenerateNodeID(dataDir)

	return &Manager{
		componentService: component.NewService(componentManager, providerService, componentImages),
		storeService:     store.NewService(s),
		providerService:  providerService,
		componentManager: componentManager,
		providerManager:  providerManager,
		nodeID:           nodeID,
		name:             name,
		description:      description,
		domainID:         domainID,
		healthCheckStop:  make(chan struct{}),
	}
}

// dependency injection
// SetLoggerService sets the logger service
func (m *Manager) SetLoggerService(loggerService logger.Service) *Manager {
	m.loggerService = loggerService
	return m
}

// SetChanneler 更新 component manager 的 channeler
// 用于在 Transport 层初始化后注入真正的 channeler
func (m *Manager) SetChanneler(channeler component.Channeler) {
	m.componentManager.SetChanneler(channeler)
}

// SetGlobalRegistryAddr 设置全局注册中心地址
func (m *Manager) SetGlobalRegistryAddr(addr string) {
	m.globalRegistryAddr = addr
}

// SetNodeAddress 设置节点地址（用于健康检查上报）
func (m *Manager) SetNodeAddress(addr string) {
	m.nodeAddress = addr
}

// SetIsHead 设置当前节点是否为 head 节点
func (m *Manager) SetIsHead(isHead bool) {
	m.isHead = isHead
}

// GetNodeID 获取节点 ID
func (m *Manager) GetNodeID() string {
	return m.nodeID
}

// GetNodeName 获取节点名称
func (m *Manager) GetNodeName() string {
	return m.name
}

// GetDomainID 获取域 ID
func (m *Manager) GetDomainID() string {
	return m.domainID
}

// GetDomainName 获取域名称
func (m *Manager) GetDomainName() string {
	return m.domainName
}

// SetDiscoveryService 设置 Discovery 服务（用于同步资源状态和远程调度）
func (m *Manager) SetDiscoveryService(discoveryService discovery.Service) {
	m.discoveryService = discoveryService
}

// SetSchedulerService 设置 Scheduler 服务（用于跨节点调度）
func (m *Manager) SetSchedulerService(schedulerService scheduler.Service) {
	m.schedulerService = schedulerService
}

// Start starts the component manager to receive messages from components
func (m *Manager) Start(ctx context.Context) error {
	// 从 repository 加载 provider
	if err := m.providerService.LoadProviders(ctx); err != nil {
		logrus.Warnf("Failed to load providers from repository: %v", err)
		// 不返回错误，继续启动
	}

	// 启动组件管理器
	if err := m.componentManager.Start(ctx); err != nil {
		return err
	}

	// 启动 provider 健康检测
	if m.providerManager != nil {
		m.providerManager.Start()
	}

	// 注册节点到全局注册中心
	if m.globalRegistryAddr != "" {
		if err := m.registerToGlobalRegistry(ctx); err != nil {
			logrus.Warnf("Failed to register node to global registry: %v", err)
			// 不返回错误，允许节点在注册中心不可用时继续运行
		} else {
			logrus.Infof("Successfully registered node %s to global registry at %s", m.nodeID, m.globalRegistryAddr)

			// 启动健康检查 goroutine
			go m.startHealthCheckLoop(ctx)
		}
	} else {
		logrus.Debug("Global registry address not configured, skipping registration")
	}

	return nil
}

// registerToGlobalRegistry 通过 gRPC 注册节点到全局注册中心
func (m *Manager) registerToGlobalRegistry(ctx context.Context) error {
	if m.globalRegistryAddr == "" {
		return fmt.Errorf("global registry address not configured")
	}

	// 创建 gRPC 连接
	conn, err := grpc.NewClient(
		m.globalRegistryAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to global registry: %w", err)
	}
	defer conn.Close()

	// 创建注册服务客户端
	client := registrypb.NewServiceClient(conn)

	// 构建注册请求
	req := &registrypb.RegisterNodeRequest{
		DomainId:        m.domainID,
		NodeId:          m.nodeID,
		NodeName:        m.name,
		NodeDescription: m.description,
	}

	// 调用注册方法
	resp, err := client.RegisterNode(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// 保存域信息（如果返回）
	if resp != nil {
		m.domainName = resp.GetDomainName()
	}

	return nil
}

// startHealthCheckLoop 启动健康检查循环，周期性向 global registry 上报节点状态
func (m *Manager) startHealthCheckLoop(ctx context.Context) {
	// 默认健康检查间隔：30 秒
	interval := 30 * time.Second

	// 创建 gRPC 连接（复用连接）
	conn, err := grpc.NewClient(
		m.globalRegistryAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logrus.Errorf("Failed to create gRPC connection to global registry: %v", err)
		return
	}
	defer conn.Close()

	client := registrypb.NewServiceClient(conn)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 立即执行一次健康检查
	m.performHealthCheck(ctx, client, interval)

	for {
		select {
		case <-ticker.C:
			// 执行健康检查
			recommendedInterval := m.performHealthCheck(ctx, client, interval)
			// 如果服务器建议了新的间隔，更新 ticker
			if recommendedInterval > 0 && recommendedInterval != interval {
				interval = recommendedInterval
				ticker.Reset(interval)
				logrus.Debugf("Updated health check interval to %v", interval)
			}
		case <-m.healthCheckStop:
			logrus.Info("Health check loop stopped")
			return
		case <-ctx.Done():
			logrus.Info("Health check loop stopped due to context cancellation")
			return
		}
	}
}

// performHealthCheck 执行一次健康检查，上报节点状态和资源信息
// 返回服务器建议的健康检查间隔（秒），如果为 0 则使用默认值
func (m *Manager) performHealthCheck(ctx context.Context, client registrypb.ServiceClient, defaultInterval time.Duration) time.Duration {
	// 聚合所有 provider 的资源状态
	resourceCapacity, resourceTags := m.aggregateResourceStatus(ctx)

	// 确定节点状态
	// 只要服务正常运行（能够发送健康检查），就认为节点是在线状态
	// 没有资源不代表节点异常，可能是暂时没有可用的 provider 或资源
	nodeStatus := registrypb.NodeStatus_NODE_STATUS_ONLINE
	// 注意：即使 resourceCapacity 为 nil（没有可用资源），节点状态仍然是 ONLINE
	// 因为能够发送健康检查本身就说明服务正常运行

	// 构建健康检查请求
	req := &registrypb.HealthCheckRequest{
		NodeId:           m.nodeID,
		DomainId:         m.domainID,
		Status:           nodeStatus,
		ResourceCapacity: resourceCapacity,
		ResourceTags:     resourceTags,
		Address:          m.nodeAddress,
		Timestamp:        time.Now().UnixNano(),
		IsHead:           m.isHead,
	}

	// 调用健康检查 RPC
	resp, err := client.HealthCheck(ctx, req)
	if err != nil {
		logrus.Warnf("Failed to send health check to global registry: %v", err)
		return 0
	}

	logrus.Debugf("Health check sent successfully, server timestamp: %d, recommended interval: %d seconds",
		resp.GetServerTimestamp(), resp.GetRecommendedIntervalSeconds())

	// 同步更新 discovery 服务的本地节点信息
	if m.discoveryService != nil {
		// 转换 resourceCapacity 和 resourceTags
		var discoveryCapacity *types.Capacity
		if resourceCapacity != nil {
			discoveryCapacity = &types.Capacity{
				Total: &types.Info{
					CPU:    resourceCapacity.Total.Cpu,
					Memory: resourceCapacity.Total.Memory,
					GPU:    resourceCapacity.Total.Gpu,
				},
				Used: &types.Info{
					CPU:    resourceCapacity.Used.Cpu,
					Memory: resourceCapacity.Used.Memory,
					GPU:    resourceCapacity.Used.Gpu,
				},
				Available: &types.Info{
					CPU:    resourceCapacity.Available.Cpu,
					Memory: resourceCapacity.Available.Memory,
					GPU:    resourceCapacity.Available.Gpu,
				},
			}
		}

		var discoveryTags interface{}
		if resourceTags != nil {
			// 使用类型断言，discovery service 会处理转换
			// 这里传递 registrypb.ResourceTags，discovery service 需要自己转换
			discoveryTags = resourceTags
		}

		// 调用 discovery service 更新本地节点信息
		// 注意：这里使用 interface{} 避免循环依赖，discovery service 需要处理类型转换
		m.discoveryService.UpdateLocalNode(discoveryCapacity, discoveryTags)
	}

	// 如果服务器要求重新注册
	if resp.GetRequireReregister() {
		logrus.Warn("Global registry requires re-registration, attempting to re-register...")
		if err := m.registerToGlobalRegistry(ctx); err != nil {
			logrus.Errorf("Failed to re-register node: %v", err)
		} else {
			logrus.Info("Successfully re-registered node")
		}
	}

	// 返回服务器建议的间隔（转换为 time.Duration）
	if resp.GetRecommendedIntervalSeconds() > 0 {
		return time.Duration(resp.GetRecommendedIntervalSeconds()) * time.Second
	}
	return 0
}

// aggregateResourceStatus 聚合所有 provider 的资源状态
// 返回聚合后的资源容量和资源标签
func (m *Manager) aggregateResourceStatus(ctx context.Context) (*registrypb.ResourceCapacity, *registrypb.ResourceTags) {
	providers := m.providerService.GetAllProviders()

	if len(providers) == 0 {
		return nil, nil
	}

	var totalCPU, totalMemory, totalGPU int64
	var usedCPU, usedMemory, usedGPU int64
	var availableCPU, availableMemory, availableGPU int64

	hasCPU := false
	hasGPU := false
	hasMemory := false
	hasCamera := false

	// 遍历所有已连接的 provider，聚合资源
	for _, p := range providers {
		if p.GetStatus() != types.ProviderStatusConnected {
			continue
		}

		// 获取 provider 的资源容量
		capacity, err := p.GetCapacity(ctx)
		if err != nil {
			logrus.Debugf("Failed to get capacity from provider %s: %v", p.GetID(), err)
			continue
		}

		if capacity == nil {
			continue
		}

		// 聚合总资源
		if capacity.Total != nil {
			totalCPU += capacity.Total.CPU
			totalMemory += capacity.Total.Memory
			totalGPU += capacity.Total.GPU
		}

		// 聚合已使用资源
		if capacity.Used != nil {
			usedCPU += capacity.Used.CPU
			usedMemory += capacity.Used.Memory
			usedGPU += capacity.Used.GPU
		}

		// 聚合可用资源
		if capacity.Available != nil {
			availableCPU += capacity.Available.CPU
			availableMemory += capacity.Available.Memory
			availableGPU += capacity.Available.GPU
		}

		// 聚合资源标签（从 provider 的缓存标签获取）
		tags := p.GetResourceTags()
		if tags != nil {
			logrus.Debugf("Aggregating resource tags from provider %s: CPU=%v, GPU=%v, Memory=%v, Camera=%v",
				p.GetID(), tags.CPU, tags.GPU, tags.Memory, tags.Camera)
			if tags.CPU {
				hasCPU = true
			}
			if tags.GPU {
				hasGPU = true
			}
			if tags.Memory {
				hasMemory = true
			}
			if tags.Camera {
				hasCamera = true
			}
		} else {
			logrus.Debugf("Provider %s has no resource tags cached yet", p.GetID())
		}
	}

	// 构建资源容量
	resourceCapacity := &registrypb.ResourceCapacity{
		Total: &registrypb.ResourceInfo{
			Cpu:    totalCPU,
			Memory: totalMemory,
			Gpu:    totalGPU,
		},
		Used: &registrypb.ResourceInfo{
			Cpu:    usedCPU,
			Memory: usedMemory,
			Gpu:    usedGPU,
		},
		Available: &registrypb.ResourceInfo{
			Cpu:    availableCPU,
			Memory: availableMemory,
			Gpu:    availableGPU,
		},
	}

	// 构建资源标签
	resourceTags := &registrypb.ResourceTags{
		Cpu:    hasCPU,
		Gpu:    hasGPU,
		Memory: hasMemory,
		Camera: hasCamera,
	}

	logrus.Infof("Aggregated resource status: providers=%d, tags=[CPU=%v, GPU=%v, Memory=%v, Camera=%v], capacity=[Total: CPU=%d, Memory=%d, GPU=%d]",
		len(providers), hasCPU, hasGPU, hasMemory, hasCamera, totalCPU, totalMemory, totalGPU)

	return resourceCapacity, resourceTags
}

func (m *Manager) SubmitLog(ctx context.Context, componentID string, entry *logger.Entry) (logger.LogID, error) {
	return m.loggerService.SubmitLog(ctx, componentID, entry)
}

func (m *Manager) GetLogs(ctx context.Context, componentID string, options *logger.QueryOptions) (*logger.QueryResult, error) {
	return m.loggerService.GetLogs(ctx, componentID, options)
}

func (m *Manager) GetLogsByTimeRange(ctx context.Context, componentID string, startTime, endTime time.Time, limit int) ([]*logger.Entry, error) {
	return m.loggerService.GetLogsByTimeRange(ctx, componentID, startTime, endTime, limit)
}

func (m *Manager) DeployComponent(ctx context.Context, runtimeEnv types.RuntimeEnv, resourceRequest *types.Info) (*component.Component, error) {
	component, err := m.componentService.DeployComponent(ctx, runtimeEnv, resourceRequest)
	if err == nil {
		component.SetProviderID("local." + component.GetProviderID())
		return component, nil
	}

	if !m.shouldDelegateDeployment(err) {
		return nil, err
	}

	logrus.Warnf("Local deployment failed (%v), attempting to delegate to peer nodes", err)
	peerComponent, peerErr := m.delegateToPeerNodes(ctx, runtimeEnv, resourceRequest)
	if peerErr == nil {
		return peerComponent, nil
	}
	logrus.Warnf("Delegation to peer nodes failed: %v", peerErr)

	globalComponent, globalErr := m.delegateToGlobalScheduler(ctx, runtimeEnv, resourceRequest)
	if globalErr == nil {
		return globalComponent, nil
	}

	return nil, fmt.Errorf("local deployment failed: %w; peer delegation failed: %v; global delegation failed: %v", err, peerErr, globalErr)
}

func (m *Manager) shouldDelegateDeployment(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "failed to find available provider") ||
		strings.Contains(msg, "no available provider")
}

func (m *Manager) delegateToPeerNodes(ctx context.Context, runtimeEnv types.RuntimeEnv, resourceRequest *types.Info) (*component.Component, error) {
	if m.discoveryService == nil || m.schedulerService == nil {
		return nil, fmt.Errorf("discovery service or scheduler service not configured")
	}
	if resourceRequest == nil {
		return nil, fmt.Errorf("resource request is nil")
	}

	nodes, err := m.discoveryService.QueryResources(ctx, resourceRequest, nil)
	if err != nil {
		return nil, fmt.Errorf("query resources via discovery service failed: %w", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no in-domain nodes have sufficient resources")
	}

	for _, node := range nodes {
		resp, deployErr := m.schedulerService.DeployComponent(ctx, &scheduler.DeployRequest{
			RuntimeEnv:      runtimeEnv,
			ResourceRequest: resourceRequest,
			TargetNodeID:    node.NodeID,
		})
		if deployErr != nil {
			logrus.Warnf("Failed to delegate deployment to node %s (%s): %v", node.NodeName, node.NodeID, deployErr)
			continue
		}
		if resp == nil || !resp.Success {
			if resp != nil && resp.Error != "" {
				logrus.Warnf("Node %s rejected deployment: %s", node.NodeID, resp.Error)
			}
			continue
		}
		if resp.Component != nil {
			resp.Component.SetProviderID(fmt.Sprintf("remote.%s@%s", resp.ProviderID, resp.NodeID))
		}
		logrus.Infof("Delegated component deployment to node %s (%s)", node.NodeName, node.NodeID)
		return resp.Component, nil
	}

	return nil, fmt.Errorf("all candidate nodes rejected the deployment request")
}

func (m *Manager) delegateToGlobalScheduler(ctx context.Context, runtimeEnv types.RuntimeEnv, resourceRequest *types.Info) (*component.Component, error) {
	if m.globalRegistryAddr == "" {
		return nil, fmt.Errorf("global scheduler address is not configured")
	}
	if resourceRequest == nil {
		return nil, fmt.Errorf("resource request is nil")
	}

	conn, err := grpc.NewClient(m.globalRegistryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect global scheduler: %w", err)
	}
	defer conn.Close()

	client := schedulerpb.NewSchedulerServiceClient(conn)
	protoReq := &schedulerpb.DeployComponentRequest{
		RuntimeEnv: string(runtimeEnv),
		ResourceRequest: &resourcepb.Info{
			Cpu:    resourceRequest.CPU,
			Memory: resourceRequest.Memory,
			Gpu:    resourceRequest.GPU,
		},
	}

	protoResp, err := client.DeployComponent(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("global scheduler RPC failed: %w", err)
	}
	if protoResp == nil {
		return nil, fmt.Errorf("global scheduler returned empty response")
	}
	if !protoResp.Success {
		return nil, fmt.Errorf("global scheduler rejected deployment: %s", protoResp.Error)
	}

	component, convErr := convertProtoComponent(protoResp.Component)
	if convErr != nil {
		return nil, fmt.Errorf("global scheduler response invalid: %w", convErr)
	}
	if component != nil {
		component.SetProviderID(fmt.Sprintf("global.%s@%s", protoResp.ProviderId, protoResp.NodeId))
	}

	logrus.Infof("Delegated component deployment to global scheduler node %s", protoResp.NodeId)
	return component, nil
}

func convertProtoComponent(info *schedulerpb.ComponentInfo) (*component.Component, error) {
	if info == nil {
		return nil, fmt.Errorf("component info is empty")
	}

	var usage *types.Info
	if info.ResourceUsage != nil {
		usage = &types.Info{
			CPU:    info.ResourceUsage.Cpu,
			Memory: info.ResourceUsage.Memory,
			GPU:    info.ResourceUsage.Gpu,
		}
	} else {
		usage = &types.Info{}
	}

	comp := component.NewComponent(info.ComponentId, info.Image, usage)
	return comp, nil
}

func (m *Manager) RegisterProvider(name string, host string, port int) (*provider.Provider, error) {
	return m.providerService.RegisterProvider(context.Background(), name, host, port)
}

// UnregisterProvider 注销 Provider
func (m *Manager) UnregisterProvider(id string) error {
	return m.providerService.UnregisterProvider(context.Background(), id)
}

// GetAllProviders 获取所有注册的 Provider
func (m *Manager) GetAllProviders() []*provider.Provider {
	return m.providerService.GetAllProviders()
}

// GetProvider 获取指定 ID 的 Provider
func (m *Manager) GetProvider(id string) *provider.Provider {
	return m.providerService.GetProvider(id)
}

func (m *Manager) SaveObject(ctx context.Context, obj *commonpb.EncodedObject) (*commonpb.ObjectRef, error) {
	return m.storeService.SaveObject(ctx, obj)
}

func (m *Manager) SaveStreamChunk(ctx context.Context, chunk *commonpb.StreamChunk) error {
	return m.storeService.SaveStreamChunk(ctx, chunk)
}

func (m *Manager) GetObject(ctx context.Context, ref *commonpb.ObjectRef) (*commonpb.EncodedObject, error) {
	return m.storeService.GetObject(ctx, ref)
}

func (m *Manager) GetStreamChunk(ctx context.Context, id string, offset int64) (*commonpb.StreamChunk, error) {
	return m.storeService.GetStreamChunk(ctx, id, offset)
}

// TODO: implement resource manager
// old version
// // String returns the string representation of providerType
// func (pt ProviderType) String() string {
// 	return string(pt)
// }

// type Manager struct {
// 	limits           Usage
// 	current          Usage
// 	mu               sync.RWMutex
// 	internalProvider Provider            // 节点内部provider
// 	localProviders   map[string]Provider // 直接接入的外部provider
// 	remoteProviders  map[string]Provider // 通过gossip协议发现的provider
// 	monitor          *ProviderMonitor
// 	cfg              *config.Config
// 	store            Store // 持久化存储
// }

// func NewManager(cfg *config.Config) *Manager {
// 	limits := cfg.ResourceLimits

// 	// 初始化持久化存储
// 	storeConfig := &StoreConfig{
// 		MaxOpenConns:           cfg.Database.MaxOpenConns,
// 		MaxIdleConns:           cfg.Database.MaxIdleConns,
// 		ConnMaxLifetimeSeconds: cfg.Database.ConnMaxLifetimeSeconds,
// 	}
// 	store, err := NewStoreWithConfig(cfg.Database.ResourceProviderDBPath, storeConfig)
// 	if err != nil {
// 		logrus.Errorf("Failed to initialize resource provider store: %v", err)
// 		return nil
// 	}

// 	rm := &Manager{
// 		internalProvider: nil,
// 		localProviders:   make(map[string]Provider),
// 		remoteProviders:  make(map[string]Provider),
// 		cfg:              cfg,
// 		store:            store,
// 	}
// 	rm.monitor = NewProviderMonitor(rm)

// 	for k, v := range limits {
// 		switch Type(k) {
// 		case CPU:
// 			rm.limits.CPU, _ = strconv.ParseFloat(v, 64)
// 		case Memory:
// 			rm.limits.Memory, _ = parseMemory(v)
// 		case GPU:
// 			rm.limits.GPU, _ = strconv.ParseFloat(v, 64)
// 		}
// 	}

// 	// Check if Docker is available before creating internal provider
// 	if cfg.EnableLocalDocker {
// 		localDockerProvider, err := GetLocalDockerProvider()
// 		if err != nil {
// 			logrus.Warnf("Docker is available but failed to create local Docker provider: %v", err)
// 		} else {
// 			rm.localProviders[localDockerProvider.GetID()] = localDockerProvider
// 			logrus.Infof("Local Docker provider created successfully")
// 		}
// 	} else {
// 		logrus.Infof("Docker is not available, skipping internal provider creation")
// 	}

// 	// 从数据库加载已保存的 local providers 并重新连接
// 	if err := rm.loadProvidersFromStore(); err != nil {
// 		logrus.Errorf("Failed to load providers from store: %v", err)
// 	}

// 	// Initialize provider monitor
// 	rm.monitor = NewProviderMonitor(rm)

// 	return rm
// }

// // RegisterProvider creates and registers a remote resource provider by type
// func (rm *Manager) RegisterProvider(providerType ProviderType, name string, config interface{}) (string, error) {
// 	rm.mu.Lock()
// 	defer rm.mu.Unlock()

// 	// Generate provider ID using shortuuid
// 	providerID := fmt.Sprintf("%s-%s", providerType, shortuuid.New())

// 	// Create provider based on type
// 	var provider Provider
// 	var err error

// 	switch providerType {
// 	case ProviderTypeDocker:
// 		provider, err = NewDockerProvider(providerID, name, config)
// 		if err != nil {
// 			return "", fmt.Errorf("failed to create Docker provider: %w", err)
// 		}
// 		// 保存到数据库
// 		if dockerConfig, ok := config.(DockerConfig); ok {
// 			if err := rm.saveProviderToStore(providerID, providerType, name, dockerConfig); err != nil {
// 				logrus.Errorf("Failed to save Docker provider to store: %v", err)
// 			}
// 		}
// 	case ProviderTypeK8s:
// 		provider, err = NewK8sProvider(providerID, name, config)
// 		if err != nil {
// 			return "", fmt.Errorf("failed to create K8s provider: %w", err)
// 		}
// 		// 保存到数据库
// 		if k8sConfig, ok := config.(K8sConfig); ok {
// 			if err := rm.saveProviderToStore(providerID, providerType, name, k8sConfig); err != nil {
// 				logrus.Errorf("Failed to save K8s provider to store: %v", err)
// 			}
// 		}
// 	default:
// 		return "", fmt.Errorf("unsupported provider type: %s", providerType)
// 	}

// 	// Store as external provider
// 	rm.localProviders[providerID] = provider
// 	return providerID, nil
// }

// // RegisterDiscoveredProvider registers a provider discovered through gossip protocol
// func (rm *Manager) RegisterDiscoveredProvider(provider Provider) {
// 	rm.mu.Lock()
// 	rm.remoteProviders[provider.GetID()] = provider
// 	rm.mu.Unlock()

// 	// Add to monitoring
// 	rm.AddProviderToMonitoring(provider)

// 	logrus.Infof("Registered discovered provider: %s", provider.GetID())
// }

// // UnregisterProvider removes a resource provider
// func (rm *Manager) UnregisterProvider(providerID string) {
// 	rm.mu.Lock()
// 	defer rm.mu.Unlock()

// 	// Check and remove from external providers
// 	if _, exists := rm.localProviders[providerID]; exists {
// 		delete(rm.localProviders, providerID)
// 		rm.RemoveProviderFromMonitoring(providerID)

// 		// 从数据库删除
// 		if err := rm.store.DeleteLocalProvider(providerID); err != nil {
// 			logrus.Errorf("Failed to delete provider from store: %v", err)
// 		}

// 		logrus.Infof("Unregistered external provider: %s", providerID)
// 		return
// 	}

// 	// Check and remove from discovered providers
// 	if _, exists := rm.remoteProviders[providerID]; exists {
// 		delete(rm.remoteProviders, providerID)
// 		rm.RemoveProviderFromMonitoring(providerID)
// 		logrus.Infof("Unregistered discovered provider: %s", providerID)
// 		return
// 	}

// 	// Cannot remove internal provider
// 	if rm.internalProvider != nil && rm.internalProvider.GetID() == providerID {
// 		logrus.Warnf("Cannot unregister internal provider: %s", providerID)
// 	}
// }

// // GetProvider returns a registered provider by ID
// func (rm *Manager) GetProvider(providerID string) (Provider, error) {
// 	rm.mu.RLock()
// 	defer rm.mu.RUnlock()

// 	// Check internal provider
// 	if rm.internalProvider != nil && rm.internalProvider.GetID() == providerID {
// 		return rm.internalProvider, nil
// 	}

// 	// Check external providers
// 	if provider, exists := rm.localProviders[providerID]; exists {
// 		return provider, nil
// 	}

// 	// Check discovered providers
// 	if provider, exists := rm.remoteProviders[providerID]; exists {
// 		return provider, nil
// 	}

// 	// Provider not found
// 	return nil, fmt.Errorf("provider with ID %s not found", providerID)
// }

// // GetCapacity returns aggregated capacity from all providers
// func (rm *Manager) GetCapacity(ctx context.Context) (*Capacity, error) {
// 	rm.mu.RLock()
// 	defer rm.mu.RUnlock()

// 	// Check if we have any providers
// 	totalProviders := 0
// 	if rm.internalProvider != nil {
// 		totalProviders++
// 	}
// 	totalProviders += len(rm.localProviders) + len(rm.remoteProviders)

// 	if totalProviders == 0 {
// 		// Return zero capacity if no providers are available
// 		return &Capacity{
// 			Total:     &Info{CPU: 0, Memory: 0, GPU: 0},
// 			Used:      &Info{CPU: 0, Memory: 0, GPU: 0},
// 			Available: &Info{CPU: 0, Memory: 0, GPU: 0},
// 		}, nil
// 	}

// 	totalCapacity, totalAllocated := &Info{CPU: 0, Memory: 0, GPU: 0}, &Info{CPU: 0, Memory: 0, GPU: 0}

// 	// Process internal provider
// 	if rm.internalProvider != nil {
// 		capacity, err := rm.internalProvider.GetCapacity(ctx)
// 		if err != nil {
// 			logrus.Warnf("Failed to get capacity from internal provider %s: %v", rm.internalProvider.GetID(), err)
// 		} else {
// 			totalCapacity.CPU += capacity.Total.CPU
// 			totalCapacity.Memory += capacity.Total.Memory
// 			totalCapacity.GPU += capacity.Total.GPU
// 			totalAllocated.CPU += capacity.Used.CPU
// 			totalAllocated.Memory += capacity.Used.Memory
// 			totalAllocated.GPU += capacity.Used.GPU
// 		}
// 	}

// 	// Process external providers
// 	for _, provider := range rm.localProviders {
// 		capacity, err := provider.GetCapacity(ctx)
// 		if err != nil {
// 			logrus.Warnf("Failed to get capacity from external provider %s: %v", provider.GetID(), err)
// 			continue
// 		}
// 		totalCapacity.CPU += capacity.Total.CPU
// 		totalCapacity.Memory += capacity.Total.Memory
// 		totalCapacity.GPU += capacity.Total.GPU
// 		totalAllocated.CPU += capacity.Used.CPU
// 		totalAllocated.Memory += capacity.Used.Memory
// 		totalAllocated.GPU += capacity.Used.GPU
// 	}

// 	// Process discovered providers
// 	for _, provider := range rm.remoteProviders {
// 		capacity, err := provider.GetCapacity(ctx)
// 		if err != nil {
// 			logrus.Warnf("Failed to get capacity from provider %s: %v", provider.GetID(), err)
// 			continue
// 		}

// 		totalCapacity.CPU += capacity.Total.CPU
// 		totalCapacity.Memory += capacity.Total.Memory
// 		totalCapacity.GPU += capacity.Total.GPU

// 		totalAllocated.CPU += capacity.Used.CPU
// 		totalAllocated.Memory += capacity.Used.Memory
// 		totalAllocated.GPU += capacity.Used.GPU
// 	}

// 	available := &Info{
// 		CPU:    totalCapacity.CPU - totalAllocated.CPU,
// 		Memory: totalCapacity.Memory - totalAllocated.Memory,
// 		GPU:    totalCapacity.GPU - totalAllocated.GPU,
// 	}

// 	return &Capacity{
// 		Total:     totalCapacity,
// 		Used:      totalAllocated,
// 		Available: available,
// 	}, nil
// }

// func parseMemory(memStr string) (float64, error) {
// 	// Parse memory string and return bytes
// 	if len(memStr) > 2 {
// 		unit := memStr[len(memStr)-2:]
// 		valStr := memStr[:len(memStr)-2]
// 		val, err := strconv.ParseFloat(valStr, 64)
// 		if err != nil {
// 			return 0, err
// 		}
// 		switch unit {
// 		case "Ki":
// 			return val * 1024, nil // KB to bytes
// 		case "Mi":
// 			return val * 1024 * 1024, nil // MB to bytes
// 		case "Gi":
// 			return val * 1024 * 1024 * 1024, nil // GB to bytes
// 		case "Ti":
// 			return val * 1024 * 1024 * 1024 * 1024, nil // TB to bytes
// 		}
// 	}
// 	// If no unit specified, assume bytes
// 	val, err := strconv.ParseFloat(memStr, 64)
// 	return val, err
// }

// func (rm *Manager) Deploy(ctx context.Context, containerSpec ContainerSpec) (*ContainerRef, error) {
// 	rm.mu.Lock()
// 	defer rm.mu.Unlock()

// 	req := containerSpec.Requirements

// 	logrus.Infof("Starting deployment process for container with image: %s", containerSpec.Image)
// 	logrus.Infof("Container spec: CPU=%dmc, Memory=%dBytes, GPU=%d, Ports=%v",
// 		req.CPU, req.Memory, req.GPU, containerSpec.Ports)

// 	// 检查资源是否充足
// 	provider := rm.canAllocate(req)
// 	if provider == nil {
// 		logrus.Errorf("Resource allocation failed: insufficient resources for CPU=%d, Memory=%d, GPU=%d",
// 			req.CPU, req.Memory, req.GPU)
// 		return nil, fmt.Errorf("resource limit exceeded")
// 	}
// 	logrus.Infof("Resource provider found for deployment: %T", provider)

// 	// 部署应用
// 	logrus.Info("Deploying container to resource provider")
// 	containerID, err := provider.Deploy(ctx, containerSpec)
// 	if err != nil {
// 		logrus.Errorf("Container deployment failed on provider: %v", err)
// 		return nil, fmt.Errorf("failed to deploy application: %w", err)
// 	}
// 	logrus.Infof("Container deployed successfully with ID: %s", containerID)

// 	// TODO: sync cache
// 	logrus.Debug("TODO: Implement cache synchronization after deployment")

// 	containerRef := &ContainerRef{
// 		ID:       containerID,
// 		Provider: provider,
// 		Spec:     containerSpec,
// 	}
// 	logrus.Infof("Deployment completed successfully: ContainerID=%s", containerID)
// 	return containerRef, nil
// }

// func (rm *Manager) canAllocate(req Info) Provider {
// 	logrus.Debugf("Checking resource allocation: Requested(CPU=%dmc, Memory=%dBytes, GPU=%d)", req.CPU, req.Memory, req.GPU)

// 	totalProviders := 0
// 	if rm.internalProvider != nil {
// 		totalProviders++
// 	}
// 	totalProviders += len(rm.localProviders) + len(rm.remoteProviders)
// 	logrus.Debugf("Searching for available provider among %d providers", totalProviders)

// 	// Check local providers
// 	for _, provider := range rm.localProviders {
// 		logrus.Debugf("Checking remote provider %s with status %v", provider.GetID(), provider.GetStatus())
// 		if provider.GetStatus() == StatusConnected {
// 			capacity, err := provider.GetCapacity(context.Background())
// 			if err != nil {
// 				logrus.WithError(err).Warnf("Failed to get capacity for local provider %s", provider.GetID())
// 				continue
// 			}
// 			logrus.Debugf("Local provider %s capacity: Available(CPU=%d, Memory=%d, GPU=%d)",
// 				provider.GetID(), capacity.Available.CPU, capacity.Available.Memory, capacity.Available.GPU)
// 			if capacity.Available.CPU >= req.CPU &&
// 				capacity.Available.Memory >= req.Memory &&
// 				capacity.Available.GPU >= req.GPU {
// 				logrus.Infof("Found suitable local provider: %s for resource allocation", provider.GetID())
// 				return provider
// 			}
// 		}
// 	}

// 	// Check remote providers
// 	for _, provider := range rm.remoteProviders {
// 		logrus.Debugf("Checking provider %s with status %v", provider.GetID(), provider.GetStatus())
// 		if provider.GetStatus() == StatusConnected {
// 			// 获取 provider 的容量信息
// 			capacity, err := provider.GetCapacity(context.Background())
// 			if err != nil {
// 				logrus.WithError(err).Warnf("Failed to get capacity for provider %s", provider.GetID())
// 				continue
// 			}

// 			logrus.Debugf("Remote provider %s capacity: Available(CPU=%d, Memory=%d, GPU=%d)",
// 				provider.GetID(), capacity.Available.CPU, capacity.Available.Memory, capacity.Available.GPU)

// 			// 检查是否有足够的资源
// 			if capacity.Available.CPU >= req.CPU &&
// 				capacity.Available.Memory >= req.Memory &&
// 				capacity.Available.GPU >= req.GPU {
// 				logrus.Infof("Found suitable provider: %s for resource allocation", provider.GetID())
// 				return provider
// 			} else {
// 				logrus.Debugf("Provider %s has insufficient resources", provider.GetID())
// 			}
// 		}
// 	}

// 	// 如果没有找到满足条件的 provider，返回 nil
// 	logrus.Warnf("No suitable provider found for resource allocation: CPU=%d, Memory=%d, GPU=%d", req.CPU, req.Memory, req.GPU)
// 	return nil
// }

// func (rm *Manager) Allocate(req Usage) {
// 	rm.mu.Lock()
// 	rm.current.CPU += req.CPU
// 	rm.current.Memory += req.Memory
// 	rm.current.GPU += req.GPU
// 	rm.mu.Unlock()
// 	logrus.Infof("Allocated: %+v, Current: %+v", req, rm.current)
// }

// func (rm *Manager) Deallocate(req Usage) {
// 	rm.mu.Lock()
// 	rm.current.CPU -= req.CPU
// 	rm.current.Memory -= req.Memory
// 	rm.current.GPU -= req.GPU
// 	rm.mu.Unlock()
// 	logrus.Infof("Deallocated: %+v, Current: %+v", req, rm.current)
// }

// // Monitor: Would poll Docker/K8s for actual usage, but for simplicity, assume requested == used.
// func (rm *Manager) StartMonitoring() {
// 	// Start provider health monitoring
// 	rm.monitor.Start()

// 	// Add existing providers to monitoring
// 	rm.mu.RLock()
// 	if rm.internalProvider != nil {
// 		rm.monitor.AddProvider(rm.internalProvider)
// 	}
// 	for _, provider := range rm.localProviders {
// 		rm.monitor.AddProvider(provider)
// 	}
// 	for _, provider := range rm.remoteProviders {
// 		rm.monitor.AddProvider(provider)
// 	}
// 	rm.mu.RUnlock()
// }

// // GetProviders 返回所有注册的资源提供者
// // CategorizedProviders represents providers categorized by their source
// type CategorizedProviders struct {
// 	LocalProviders  []Provider `json:"local_providers"`  // 本地资源（包含内部和外部托管）
// 	RemoteProviders []Provider `json:"remote_providers"` // 远程资源（通过协作发现）
// }

// // GetProviders returns providers categorized by their source
// func (rm *Manager) GetProviders() *CategorizedProviders {
// 	rm.mu.RLock()
// 	defer rm.mu.RUnlock()

// 	result := &CategorizedProviders{
// 		LocalProviders:  make([]Provider, 0, len(rm.localProviders)+1),
// 		RemoteProviders: make([]Provider, 0, len(rm.remoteProviders)),
// 	}

// 	// Add internal provider to local providers if exists
// 	if rm.internalProvider != nil {
// 		result.LocalProviders = append(result.LocalProviders, rm.internalProvider)
// 	}

// 	// Add external providers to local providers
// 	for _, provider := range rm.localProviders {
// 		result.LocalProviders = append(result.LocalProviders, provider)
// 	}

// 	// Add discovered providers to remote providers
// 	for _, provider := range rm.remoteProviders {
// 		result.RemoteProviders = append(result.RemoteProviders, provider)
// 	}

// 	return result
// }

// // StopMonitoring stops the provider monitoring
// func (rm *Manager) StopMonitoring() {
// 	if rm.monitor != nil {
// 		rm.monitor.Stop()
// 	}
// }

// // HandleProviderFailure handles when a provider fails
// func (rm *Manager) HandleProviderFailure(providerID string) {
// 	rm.mu.Lock()
// 	defer rm.mu.Unlock()

// 	logrus.Warnf("Handling failure for provider %s", providerID)

// 	// For now, we keep the provider but mark it as failed
// 	// In a more sophisticated implementation, we might:
// 	// 1. Migrate running containers to other providers
// 	// 2. Remove the provider from load balancing
// 	// 3. Attempt automatic recovery
// }

// // loadProvidersFromStore 从数据库加载 providers 并重新建立连接
// func (rm *Manager) loadProvidersFromStore() error {
// 	providers, err := rm.store.GetAllLocalProviders()
// 	if err != nil {
// 		return fmt.Errorf("failed to get all local providers: %w", err)
// 	}

// 	for _, providerConfig := range providers {
// 		// 根据配置重新创建 provider
// 		var config interface{}

// 		switch providerConfig.ProviderType {
// 		case ProviderTypeDocker:
// 			dockerConfig, err := DeserializeDockerConfig(providerConfig.Config)
// 			if err != nil {
// 				logrus.Errorf("Failed to deserialize Docker config for provider %s: %v", providerConfig.ProviderID, err)
// 				continue
// 			}
// 			config = dockerConfig
// 		case ProviderTypeK8s:
// 			k8sConfig, err := DeserializeK8sConfig(providerConfig.Config)
// 			if err != nil {
// 				logrus.Errorf("Failed to deserialize K8s config for provider %s: %v", providerConfig.ProviderID, err)
// 				continue
// 			}
// 			config = k8sConfig
// 		default:
// 			logrus.Warnf("Unknown provider type %s for provider %s", providerConfig.ProviderType, providerConfig.ProviderID)
// 			continue
// 		}

// 		// 创建 provider 实例
// 		var provider Provider
// 		switch providerConfig.ProviderType {
// 		case ProviderTypeDocker:
// 			provider, err = NewDockerProvider(providerConfig.ProviderID, providerConfig.Name, config)
// 			if err != nil {
// 				logrus.Errorf("Failed to recreate Docker provider %s: %v", providerConfig.ProviderID, err)
// 				// 更新状态为断开
// 				rm.store.UpdateProviderStatus(providerConfig.ProviderID, StatusDisconnected)
// 				continue
// 			}
// 		case ProviderTypeK8s:
// 			provider, err = NewK8sProvider(providerConfig.ProviderID, providerConfig.Name, config)
// 			if err != nil {
// 				logrus.Errorf("Failed to recreate K8s provider %s: %v", providerConfig.ProviderID, err)
// 				// 更新状态为断开
// 				rm.store.UpdateProviderStatus(providerConfig.ProviderID, StatusDisconnected)
// 				continue
// 			}
// 		}

// 		// 注册到 localProviders
// 		rm.localProviders[providerConfig.ProviderID] = provider
// 		// 更新状态为已连接
// 		rm.store.UpdateProviderStatus(providerConfig.ProviderID, StatusConnected)
// 		logrus.Infof("Loaded and connected provider from store: ID=%s, Type=%s, Name=%s",
// 			providerConfig.ProviderID, providerConfig.ProviderType, providerConfig.Name)
// 	}

// 	return nil
// }

// // saveProviderToStore 保存 provider 配置到数据库
// func (rm *Manager) saveProviderToStore(providerID string, providerType ProviderType, name string, config interface{}) error {
// 	var configStr string
// 	var err error

// 	switch providerType {
// 	case ProviderTypeDocker:
// 		dockerConfig, ok := config.(DockerConfig)
// 		if !ok {
// 			return fmt.Errorf("invalid Docker config type")
// 		}
// 		configStr, err = SerializeDockerConfig(dockerConfig)
// 		if err != nil {
// 			return err
// 		}
// 	case ProviderTypeK8s:
// 		k8sConfig, ok := config.(K8sConfig)
// 		if !ok {
// 			return fmt.Errorf("invalid K8s config type")
// 		}
// 		configStr, err = SerializeK8sConfig(k8sConfig)
// 		if err != nil {
// 			return err
// 		}
// 	default:
// 		return fmt.Errorf("unsupported provider type: %s", providerType)
// 	}

// 	providerConfig := &ProviderConfig{
// 		ProviderID:   providerID,
// 		ProviderType: providerType,
// 		Name:         name,
// 		Config:       configStr,
// 		Status:       StatusConnected,
// 		CreatedAt:    getCurrentTimestamp(),
// 		UpdatedAt:    getCurrentTimestamp(),
// 	}

// 	return rm.store.SaveLocalProvider(providerConfig)
// }

// // Close 关闭 Manager 及其持久化存储
// func (rm *Manager) Close() error {
// 	// 停止监控
// 	if rm.monitor != nil {
// 		rm.monitor.Stop()
// 	}

// 	// 关闭存储
// 	if rm.store != nil {
// 		return rm.store.Close()
// 	}

// 	return nil
// }

// // HandleProviderRecovery handles when a provider recovers
// func (rm *Manager) HandleProviderRecovery(providerID string) {
// 	rm.mu.Lock()
// 	defer rm.mu.Unlock()

// 	logrus.Infof("Handling recovery for provider %s", providerID)

// 	// Provider is back online and can accept new workloads
// 	// In a more sophisticated implementation, we might:
// 	// 1. Re-enable the provider for load balancing
// 	// 2. Perform health verification
// 	// 3. Gradually increase load
// }

// // AddProviderToMonitoring adds a provider to the monitoring system
// func (rm *Manager) AddProviderToMonitoring(provider Provider) {
// 	if rm.monitor != nil {
// 		rm.monitor.AddProvider(provider)
// 	}
// }

// // RemoveProviderFromMonitoring removes a provider from the monitoring system
// func (rm *Manager) RemoveProviderFromMonitoring(providerID string) {
// 	if rm.monitor != nil {
// 		rm.monitor.RemoveProvider(providerID)
// 	}
// }

// // GetProviderHealthStatus returns the health status of all providers
// func (rm *Manager) GetProviderHealthStatus() map[string]bool {
// 	if rm.monitor != nil {
// 		return rm.monitor.GetAllHealthStatus()
// 	}
// 	return make(map[string]bool)
// }
