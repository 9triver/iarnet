package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/9triver/iarnet/internal/bootstrap"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/util"
	"github.com/sirupsen/logrus"
)

func main() {
	configFile := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Load config: %v", err)
	}
	if cfg.Mode == "" {
		cfg.Mode = config.DetectMode()
	}

	util.InitLogger()

	// 使用 Bootstrap 初始化所有模块
	iarnet, err := bootstrap.Initialize(cfg)
	if err != nil {
		logrus.Fatalf("Failed to initialize: %v", err)
	}
	defer iarnet.Stop()

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动服务（包括组件管理器）
	if err := iarnet.Start(ctx); err != nil {
		logrus.Fatalf("Failed to start services: %v", err)
	}
	logrus.Info("Component manager started")

	// 测试代码：注册 provider 和创建 controller
	provider := component.NewProvider("local-docker", "localhost", 50051)
	iarnet.ResourceManager.RegisterProvider(provider)
	logrus.Infof("Local docker provider registered")

	if _, err := iarnet.ControllerService.CreateController(context.Background(), "1"); err != nil {
		logrus.Warnf("Failed to create test controller: %v", err)
	} else {
		logrus.Infof("Controller 1 created for test")
	}

	logrus.Info("Iarnet started successfully")

	// 优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logrus.Info("Shutting down...")

	// 取消上下文以停止组件管理器和 ZMQ 接收器
	cancel()

	logrus.Info("Shutdown complete")
}
