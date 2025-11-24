package bootstrap

import (
	"fmt"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/logger"
	"github.com/9triver/iarnet/internal/domain/resource/provider"
	"github.com/9triver/iarnet/internal/domain/resource/scheduler"
	"github.com/9triver/iarnet/internal/domain/resource/store"
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
		// 使用 Host 和 transport.rpc.resource.port 构建节点地址
		host := iarnet.Config.Host
		if host == "" {
			host = "localhost"
		}
		port := iarnet.Config.Transport.RPC.Resource.Port
		if port == 0 {
			// 如果未配置，使用默认值 50051
			port = 50051
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

		// 创建节点发现管理器
		gossipInterval := time.Duration(iarnet.Config.Resource.Discovery.GossipIntervalSeconds) * time.Second
		nodeTTL := time.Duration(iarnet.Config.Resource.Discovery.NodeTTLSeconds) * time.Second

		discoveryManager := discovery.NewNodeDiscoveryManager(
			nodeID,
			nodeName,
			nodeAddr,
			domainID,
			iarnet.Config.InitialPeers,
			gossipInterval,
			nodeTTL,
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
	iarnet.SchedulerService = schedulerService

	logrus.Info("Resource module initialized")
	return nil
}
