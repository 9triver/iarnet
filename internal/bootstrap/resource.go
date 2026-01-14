package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/logger"
	"github.com/9triver/iarnet/internal/domain/resource/policy"
	"github.com/9triver/iarnet/internal/domain/resource/provider"
	"github.com/9triver/iarnet/internal/domain/resource/scheduler"
	"github.com/9triver/iarnet/internal/domain/resource/store"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	providerrepo "github.com/9triver/iarnet/internal/infra/repository/resource"
	"github.com/sirupsen/logrus"
)

// BootstrapResource 初始化 Resource 模块
func bootstrapResource(iarnet *Iarnet) error {
	// 初始化 Store
	storeInstance := store.NewStore()

	// 初始化 Provider Repository
	var providerRepo providerrepo.ProviderRepo
	dbPath := iarnet.Config.Database.ResourceProviderDBPath
	if repo, err := providerrepo.NewProviderRepoSQLite(dbPath, iarnet.Config); err != nil {
		logrus.Warnf("Failed to initialize provider repository: %v, continuing without persistence", err)
	} else {
		providerRepo = repo
		logrus.Infof("Provider repository initialized at %s", dbPath)
	}

	// 使用占位符 channeler 初始化 Resource Manager
	// 真正的 channeler 会在 Transport 层创建后注入
	nullChanneler := component.NewNullChanneler()

	// 转换配置的间隔时间为 time.Duration
	providerHealthCheckInterval := time.Duration(iarnet.Config.Resource.ProviderHealthCheckIntervalSeconds*1000) * time.Millisecond
	providerUsagePollInterval := time.Duration(iarnet.Config.Resource.ProviderUsagePollIntervalSeconds*1000) * time.Millisecond

	resourceManager := resource.NewManager(
		nullChanneler,
		storeInstance,
		iarnet.Config.Resource.ComponentImages,
		providerRepo,
		&provider.EnvVariables{
			IarnetHost: iarnet.Config.Host,
			ZMQPort:    iarnet.Config.Transport.ZMQ.Port,
			StorePort:  iarnet.Config.Transport.RPC.Store.Port,
			LoggerPort: iarnet.Config.Transport.RPC.ResourceLogger.Port,
		},
		iarnet.Config.Resource.Name,
		iarnet.Config.Resource.Description,
		iarnet.Config.Resource.DomainID,
		iarnet.Config.DataDir,
		providerHealthCheckInterval,
		providerUsagePollInterval,
	)

	var resourceLoggerService logger.Service
	resourceLoggerRepo, err := providerrepo.NewLoggerRepoSQLite(iarnet.Config.Database.ResourceLoggerDBPath, iarnet.Config)
	if err != nil {
		logrus.Warnf("Failed to initialize resource logger repository: %v, continuing without resource logger persistence", err)
		resourceLoggerService = nil
	} else {
		resourceLoggerService = logger.NewService(resourceLoggerRepo)
	}
	iarnet.ResourceManager = resourceManager.SetLoggerService(resourceLoggerService)

	// 设置全局注册中心地址
	if iarnet.Config.Resource.GlobalRegistryAddr != "" {
		iarnet.ResourceManager.SetGlobalRegistryAddr(iarnet.Config.Resource.GlobalRegistryAddr)

		// 设置节点地址（用于健康检查上报）
		// 使用 Host 和 transport.rpc.scheduler.port 构建节点地址（供全局调度器访问）
		host := iarnet.Config.Host
		if host == "" {
			host = "localhost"
		}
		port := iarnet.Config.Transport.RPC.Scheduler.Port
		if port == 0 {
			// 如果未配置，使用默认值 50006
			port = 50006
		}
		nodeAddr := fmt.Sprintf("%s:%d", host, port)
		iarnet.ResourceManager.SetNodeAddress(nodeAddr)
		logrus.Infof("Node address set to %s for health check reporting", nodeAddr)
		logrus.Infof("Global registry address configured: %s", iarnet.Config.Resource.GlobalRegistryAddr)
	}

	// 初始化 Discovery 服务（如果启用）
	if iarnet.Config.Resource.Discovery.Enabled {
		// 获取节点信息
		nodeID := resourceManager.GetNodeID()
		nodeName := iarnet.Config.Resource.Name
		domainID := iarnet.Config.Resource.DomainID

		// 构建节点地址（用于 discovery）
		host := iarnet.Config.Host
		if host == "" {
			host = "localhost"
		}
		discoveryPort := iarnet.Config.Transport.RPC.Discovery.Port
		if discoveryPort == 0 {
			discoveryPort = 50005 // 默认端口
		}
		nodeAddr := fmt.Sprintf("%s:%d", host, discoveryPort)

		schedulerPort := iarnet.Config.Transport.RPC.Scheduler.Port
		if schedulerPort == 0 {
			schedulerPort = 50006
		}
		schedulerAddr := fmt.Sprintf("%s:%d", host, schedulerPort)

		// 创建节点发现管理器
		// 支持小数秒配置，转换为毫秒后乘以 time.Millisecond
		var gossipInterval time.Duration
		var gossipIntervalMin time.Duration
		var gossipIntervalMax time.Duration

		if iarnet.Config.Resource.Discovery.GossipIntervalMinSeconds > 0 && iarnet.Config.Resource.Discovery.GossipIntervalMaxSeconds > 0 {
			// 使用区间随机
			gossipIntervalMin = time.Duration(iarnet.Config.Resource.Discovery.GossipIntervalMinSeconds*1000) * time.Millisecond
			gossipIntervalMax = time.Duration(iarnet.Config.Resource.Discovery.GossipIntervalMaxSeconds*1000) * time.Millisecond
			// 固定间隔设为 0，表示不使用
			gossipInterval = 0
		} else {
			// 使用固定间隔（向后兼容）
			gossipInterval = time.Duration(iarnet.Config.Resource.Discovery.GossipIntervalSeconds*1000) * time.Millisecond
			gossipIntervalMin = 0
			gossipIntervalMax = 0
		}

		nodeTTL := time.Duration(iarnet.Config.Resource.Discovery.NodeTTLSeconds) * time.Second

		discoveryManager := discovery.NewNodeDiscoveryManager(
			nodeID,
			nodeName,
			nodeAddr,
			schedulerAddr,
			domainID,
			iarnet.Config.InitialPeers,
			gossipInterval,
			gossipIntervalMin,
			gossipIntervalMax,
			nodeTTL,
			iarnet.Config.Resource.Discovery.LogNodeInfoUpdates,
		)

		// 设置配置参数
		discoveryManager.SetMaxGossipPeers(iarnet.Config.Resource.Discovery.MaxGossipPeers)
		discoveryManager.SetMaxHops(iarnet.Config.Resource.Discovery.MaxHops)

		// 创建 discovery 服务
		discoveryService := discovery.NewService(discoveryManager)

		// 保存到 iarnet 结构
		iarnet.DiscoveryManager = discoveryManager
		iarnet.DiscoveryService = discoveryService

		// 将 discovery service 设置到 resource manager，用于同步资源状态
		resourceManager.SetDiscoveryService(discoveryService)

		logrus.Infof("Discovery service initialized: node_id=%s, address=%s, domain=%s", nodeID, nodeAddr, domainID)
	} else {
		logrus.Debug("Discovery service is disabled")
	}

	// 初始化 Scheduler 服务
	schedulerService := scheduler.NewService(
		resourceManager,
		iarnet.DiscoveryService,
	)
	resourceManager.SetSchedulerService(schedulerService)
	iarnet.SchedulerService = schedulerService
	resourceManager.SetIsHead(iarnet.Config.Resource.IsHead)

	// 初始化调度策略链
	if len(iarnet.Config.Resource.SchedulePolicies) > 0 {
		policyFactory := policy.NewFactory()
		policyConfigs := make([]policy.PolicyConfig, 0, len(iarnet.Config.Resource.SchedulePolicies))
		for _, cfg := range iarnet.Config.Resource.SchedulePolicies {
			policyConfigs = append(policyConfigs, policy.PolicyConfig{
				Type:   cfg.Type,
				Enable: cfg.Enable,
				Params: cfg.Params,
			})
		}
		policyChain, err := policyFactory.CreateChain(policyConfigs)
		if err != nil {
			logrus.Warnf("Failed to create schedule policy chain: %v, continuing without policies", err)
		} else {
			resourceManager.SetSchedulePolicyChain(policyChain)
			logrus.Infof("Schedule policy chain initialized with %d policies", len(policyConfigs))
		}
	} else {
		logrus.Debug("No schedule policies configured")
	}

	// 初始化 Fake Providers（如果配置了）
	if len(iarnet.Config.Resource.FakeProviders) > 0 {
		if err := initializeFakeProviders(iarnet.ResourceManager, iarnet.Config.Resource.FakeProviders); err != nil {
			logrus.Warnf("Failed to initialize fake providers: %v", err)
		} else {
			logrus.Infof("Initialized %d fake providers", len(iarnet.Config.Resource.FakeProviders))
		}
	}

	logrus.Info("Resource module initialized")
	return nil
}

// initializeFakeProviders 初始化假 Provider
func initializeFakeProviders(resourceManager *resource.Manager, fakeProviderConfigs []config.FakeProviderConfig) error {
	providerManager := resourceManager.GetProviderManager()
	if providerManager == nil {
		return fmt.Errorf("provider manager is not available")
	}

	for _, cfg := range fakeProviderConfigs {
		// 转换资源标签配置
		var resourceTags *types.ResourceTags
		if cfg.ResourceTags != nil {
			resourceTags = &types.ResourceTags{
				CPU:    cfg.ResourceTags.CPU,
				GPU:    cfg.ResourceTags.GPU,
				Memory: cfg.ResourceTags.Memory,
				Camera: cfg.ResourceTags.Camera,
			}
		}

		// 转换资源使用状态配置
		var usage *provider.UsageConfig
		if cfg.Usage != nil {
			usage = &provider.UsageConfig{
				CPURatio:    cfg.Usage.CPURatio,
				GPURatio:    cfg.Usage.GPURatio,
				MemoryRatio: cfg.Usage.MemoryRatio,
			}
			// 如果直接指定了已使用资源，优先使用
			if cfg.Usage.Used != nil {
				usage.Used = &types.Info{
					CPU:    cfg.Usage.Used.CPU,
					Memory: cfg.Usage.Used.Memory.Int64(), // 将 MemorySize 转换为 int64
					GPU:    cfg.Usage.Used.GPU,
				}
			}
		}

		// 创建 FakeProvider
		fakeProvider := provider.NewFakeProvider(
			cfg.Name,
			cfg.Type,
			cfg.CPU,
			cfg.Memory.Int64(), // 将 MemorySize 转换为 int64
			cfg.GPU,
			cfg.Host,
			cfg.Port,
			resourceTags,
			usage,
		)

		// 连接到假 Provider（模拟连接）
		ctx := context.Background()
		if err := fakeProvider.Connect(ctx); err != nil {
			logrus.Warnf("Failed to connect fake provider %s: %v", fakeProvider.GetID(), err)
			continue
		}

		// 添加到管理器
		providerManager.Add(fakeProvider)
		logrus.Infof("Registered fake provider: %s (type: %s, CPU: %d, Memory: %d, GPU: %d)",
			fakeProvider.GetName(), cfg.Type, cfg.CPU, cfg.Memory.Int64(), cfg.GPU)
	}

	return nil
}
