package bootstrap

import (
	"context"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/application"
	"github.com/9triver/iarnet/internal/domain/ignis/controller"
	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/9triver/iarnet/internal/transport/rpc"
	"github.com/9triver/iarnet/internal/transport/zmq"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

type Iarnet struct {
	// 配置
	Config *config.Config

	// 基础设施
	DockerClient *client.Client
	Channeler    *zmq.ComponentChanneler
	RPCManager   *rpc.Manager

	// Resource 模块
	ResourceManager *resource.Manager

	// Application 模块
	ApplicationManager *application.Manager

	// Ignis 模块
	ControllerManager controller.Manager
	ControllerService controller.Service
}

func (iarnet *Iarnet) Start(ctx context.Context) error {
	iarnet.ResourceManager.StartComponentManager(ctx)
	return nil
}

// Stop 停止所有服务并清理资源
func (iarnet *Iarnet) Stop() error {
	// 停止 RPC 服务器
	if iarnet.RPCManager != nil {
		iarnet.RPCManager.Stop()
		logrus.Info("RPC servers stopped")
	}

	// 关闭 ZMQ Channeler
	if iarnet.Channeler != nil {
		if err := iarnet.Channeler.Close(); err != nil {
			logrus.Errorf("Error closing ZMQ channeler: %v", err)
		}
	}

	// 停止 Runner Manager（如果存在）
	if iarnet.ApplicationManager != nil {
		// 这里可以添加 Application Manager 的清理逻辑
		// 例如：停止所有 runner 的监听
	}

	logrus.Info("All services stopped")
	return nil
}
