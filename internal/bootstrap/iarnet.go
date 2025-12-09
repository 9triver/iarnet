package bootstrap

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/application"
	"github.com/9triver/iarnet/internal/domain/execution"
	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/scheduler"
	"github.com/9triver/iarnet/internal/transport/http"
	"github.com/9triver/iarnet/internal/transport/rpc"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

type Iarnet struct {
	// 配置
	Config *config.Config

	// 基础设施
	DockerClient *client.Client
	Channeler    component.Channeler // 使用接口类型，不依赖具体实现
	RPCManager   *rpc.Manager
	HTTPServer   *http.Server

	// Resource 模块
	ResourceManager *resource.Manager

	// Discovery 模块
	DiscoveryManager *discovery.NodeDiscoveryManager
	DiscoveryService discovery.Service

	// Scheduler 模块
	SchedulerService scheduler.Service

	// Application 模块
	ApplicationManager *application.Manager

	// Execution 模块
	IgnisPlatform *execution.Platform
}

// Start 启动所有服务
func (iarnet *Iarnet) Start(ctx context.Context) error {

	// 1. 启动组件管理器
	if iarnet.ResourceManager != nil {
		iarnet.ResourceManager.Start(ctx)
		logrus.Info("Component manager started")
	} else {
		return fmt.Errorf("resource manager is not initialized")
	}

	// 1.5. 启动 Discovery 服务（如果启用）
	if iarnet.DiscoveryService != nil {
		if err := iarnet.DiscoveryService.Start(ctx); err != nil {
			return fmt.Errorf("failed to start discovery service: %w", err)
		}
		logrus.Info("Discovery service started")
	}

	// 2. 启动 Application 管理器
	if iarnet.ApplicationManager != nil {
		iarnet.ApplicationManager.Start(ctx)
		logrus.Info("Application manager started")
	} else {
		return fmt.Errorf("application manager is not initialized")
	}

	// 3. 启动 RPC 服务器
	if iarnet.RPCManager != nil {
		if err := iarnet.RPCManager.Start(); err != nil {
			return fmt.Errorf("failed to start rpc servers: %w", err)
		}
	} else {
		return fmt.Errorf("rpc manager is not initialized")
	}

	// 4. 启动 HTTP 服务器
	if iarnet.HTTPServer != nil {
		iarnet.HTTPServer.Start()
		logrus.Info("HTTP server started")
	} else {
		return fmt.Errorf("http server is not initialized")
	}

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

	// 停止 Resource Manager（包括健康检测和轮询服务）
	if iarnet.ResourceManager != nil {
		iarnet.ResourceManager.Stop()
		logrus.Info("Resource manager stopped")
	}

	// 停止 Discovery 服务
	if iarnet.DiscoveryService != nil {
		iarnet.DiscoveryService.Stop()
		logrus.Info("Discovery service stopped")
	}

	logrus.Info("All services stopped")
	return nil
}
