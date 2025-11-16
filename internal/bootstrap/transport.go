package bootstrap

import (
	"fmt"

	"github.com/9triver/iarnet/internal/transport/http"
	"github.com/9triver/iarnet/internal/transport/rpc"
	"github.com/9triver/iarnet/internal/transport/zmq"
	"github.com/sirupsen/logrus"
)

// BootstrapTransport 初始化 Transport 层（RPC、HTTP 等）
func bootstrapTransport(iarnet *Iarnet) error {
	// 创建 ZMQ Channeler（现在可以在 Resource 之后初始化）
	channeler := zmq.NewChanneler(iarnet.Config.Transport.ZMQ.Port)

	// 将真正的 channeler 注入到 ResourceManager
	if iarnet.ResourceManager != nil {
		iarnet.ResourceManager.SetChanneler(channeler)
		logrus.Info("ZMQ Channeler injected into ResourceManager")
	}

	// 保存 channeler 引用（用于后续关闭）
	iarnet.Channeler = channeler

	// 构建 RPC 服务器地址
	ignisAddr := fmt.Sprintf("0.0.0.0:%d", iarnet.Config.Transport.RPC.Ignis.Port)
	storeAddr := fmt.Sprintf("0.0.0.0:%d", iarnet.Config.Transport.RPC.Store.Port)

	// 创建 RPC 服务器管理器（不启动，启动操作在 Start 方法中统一执行）
	iarnet.RPCManager = rpc.NewManager(rpc.Options{
		IgnisAddr:         ignisAddr,
		StoreAddr:         storeAddr,
		ControllerService: iarnet.IgnisPlatform,
		StoreService:      iarnet.ResourceManager,
	})

	// 创建 HTTP 服务器
	iarnet.HTTPServer = http.NewServer(http.Options{
		Port:     iarnet.Config.Transport.HTTP.Port,
		AppMgr:   iarnet.ApplicationManager,
		ResMgr:   iarnet.ResourceManager,
		Platform: iarnet.IgnisPlatform,
		Config:   iarnet.Config,
	})

	logrus.Info("Transport layer initialized")
	return nil
}
