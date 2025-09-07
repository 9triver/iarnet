package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/9triver/iarnet/internal/analysis"
	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/discovery"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/runner"
	"github.com/9triver/iarnet/internal/server"
	"github.com/sirupsen/logrus"
)

// Remove the old peerServer implementation as it's now in server/grpc_server.go

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
	logrus.Infof("Running in mode: %s", cfg.Mode)

	var r runner.Runner
	switch cfg.Mode {
	case "standalone":
		r, err = runner.NewStandaloneRunner()
	case "k8s":
		r, err = runner.NewK8sRunner()
	default:
		log.Fatalf("Invalid mode: %s", cfg.Mode)
	}
	if err != nil {
		log.Fatalf("Runner init: %v", err)
	}

	rm := resource.NewManager(cfg.ResourceLimits)
	am := application.NewManager(cfg, rm)
	
	// 创建并设置代码分析服务
	analysisService := analysis.NewMockCodeAnalysisService(rm)
	am.SetAnalysisService(analysisService)

	// // 添加一些示例应用数据用于测试
	// app1 := am.CreateApplication("用户管理系统")
	// app2 := am.CreateApplication("数据处理服务")
	// am.CreateApplication("API网关")

	// // 模拟一些应用状态变化
	// am.UpdateApplicationStatus(app1.ID, application.StatusRunning)
	// am.UpdateApplicationStatus(app2.ID, application.StatusRunning)
	// // 第三个应用保持未部署状态

	pm := discovery.NewPeerManager(cfg.InitialPeers, rm)

	// Start gossip
	go pm.StartGossip(context.Background())

	// Start gRPC peer server
	grpcSrv := server.NewGRPCServer(rm, pm)
	go func() {
		if err := grpcSrv.Start(cfg.PeerListenAddr); err != nil {
			log.Fatalf("gRPC server: %v", err)
		}
	}()

	// Start HTTP server
	srv := server.NewServer(r, rm, am, pm)
	go func() {
		if err := srv.Start(cfg.ListenAddr); err != nil {
			log.Fatalf("HTTP server: %v", err)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logrus.Info("Shutting down...")
	srv.Stop()
	grpcSrv.Stop()
	rm.StopMonitoring()
	logrus.Info("Shutdown complete")
}
