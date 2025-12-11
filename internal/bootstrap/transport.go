package bootstrap

import (
	"fmt"

	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/transport/http"
	"github.com/9triver/iarnet/internal/transport/rpc"
	"github.com/9triver/iarnet/internal/transport/zmq"
	"github.com/sirupsen/logrus"
)

// BootstrapTransport 初始化 Transport 层（RPC、HTTP 等）
func bootstrapTransport(iarnet *Iarnet) error {
	// 创建 ZMQ Channeler（现在可以在 Resource 之后初始化）
	// 使用 panic 恢复机制，如果 ZMQ 初始化失败，使用 null channeler
	var channeler component.Channeler
	func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("ZMQ Channeler creation panic in bootstrap: %v", r)
				logrus.Warn("Continuing without ZMQ support, using null channeler")
				channeler = component.NewNullChanneler()
			}
		}()
		ch := zmq.NewChanneler(iarnet.Config.Transport.ZMQ.Port)
		if ch == nil {
			logrus.Warn("ZMQ Channeler initialization returned nil, using null channeler")
			channeler = component.NewNullChanneler()
		} else {
			channeler = ch
		}
	}()

	// 将 channeler 注入到 ResourceManager（可能是 ZMQ channeler 或 null channeler）
	if iarnet.ResourceManager != nil {
		iarnet.ResourceManager.SetChanneler(channeler)
		if _, ok := channeler.(*zmq.ComponentChanneler); ok {
			logrus.Info("ZMQ Channeler injected into ResourceManager")
		} else {
			logrus.Warn("Null Channeler injected into ResourceManager (ZMQ unavailable)")
		}
	}

	// 保存 ZMQ channeler 引用（用于后续关闭，如果是 null channeler 则为 nil）
	if cc, ok := channeler.(*zmq.ComponentChanneler); ok {
		iarnet.Channeler = cc
	}

	// 构建 RPC 服务器地址
	executionAddr := fmt.Sprintf("0.0.0.0:%d", iarnet.Config.Transport.RPC.Ignis.Port)
	storeAddr := fmt.Sprintf("0.0.0.0:%d", iarnet.Config.Transport.RPC.Store.Port)
	loggerAddr := fmt.Sprintf("0.0.0.0:%d", iarnet.Config.Transport.RPC.Logger.Port)
	resourceLoggerAddr := fmt.Sprintf("0.0.0.0:%d", iarnet.Config.Transport.RPC.ResourceLogger.Port)
	discoveryAddr := fmt.Sprintf("0.0.0.0:%d", iarnet.Config.Transport.RPC.Discovery.Port)
	schedulerAddr := fmt.Sprintf("0.0.0.0:%d", iarnet.Config.Transport.RPC.Scheduler.Port)

	// 创建 RPC 服务器管理器（不启动，启动操作在 Start 方法中统一执行）
	opts := rpc.Options{
		ExecutionAddr:          executionAddr,
		StoreAddr:             storeAddr,
		LoggerAddr:            loggerAddr,
		ControllerService:     iarnet.IgnisPlatform,
		StoreService:          iarnet.ResourceManager,
		LoggerService:         iarnet.ApplicationManager,
		ResourceLoggerAddr:    resourceLoggerAddr,
		ResourceLoggerService: iarnet.ResourceManager,
		DiscoveryAddr:         discoveryAddr,
		DiscoveryService:      iarnet.DiscoveryService,
		DiscoveryManager:      iarnet.DiscoveryManager,
		SchedulerAddr:         schedulerAddr,
		SchedulerService:      iarnet.SchedulerService,
	}

	iarnet.RPCManager = rpc.NewManager(opts)

	// 创建 HTTP 服务器
	iarnet.HTTPServer = http.NewServer(http.Options{
		Port:             iarnet.Config.Transport.HTTP.Port,
		AppMgr:           iarnet.ApplicationManager,
		ResMgr:           iarnet.ResourceManager,
		Platform:         iarnet.IgnisPlatform,
		Config:           iarnet.Config,
		DiscoveryService: iarnet.DiscoveryService,
	})

	logrus.Info("Transport layer initialized")
	return nil
}
