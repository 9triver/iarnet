package bootstrap

import (
	"fmt"

	"github.com/9triver/iarnet/internal/transport/rpc"
	"github.com/9triver/iarnet/internal/transport/zmq"
	"github.com/sirupsen/logrus"
)

// BootstrapTransport 初始化 Transport 层（RPC、HTTP 等）
func bootstrapTransport(iarnet *Iarnet) error {

	channeler := zmq.NewChanneler(iarnet.Config.Resource.ZMQ.Port)
	iarnet.Channeler = channeler

	// 构建 RPC 服务器地址
	ignisAddr := fmt.Sprintf("0.0.0.0:%d", iarnet.Config.Ignis.Port)
	storeAddr := fmt.Sprintf("0.0.0.0:%d", iarnet.Config.Resource.Store.Port)

	// 创建 RPC 服务器管理器（不启动，启动操作在 Start 方法中统一执行）
	rpcManager := rpc.NewManager(rpc.Options{
		IgnisAddr:     ignisAddr,
		StoreAddr:     storeAddr,
		ControllerMgr: iarnet.ControllerManager,
		StoreService:  iarnet.ResourceManager,
	})

	iarnet.RPCManager = rpcManager
	logrus.Info("Transport layer initialized")
	return nil
}
