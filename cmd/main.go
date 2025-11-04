package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/9triver/iarnet/integration/ignis/deployer"
	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/compute/ignis"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/discovery"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/server"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
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
	logrus.Infof("Running in mode: %s", cfg.Mode)

	// 初始化资源管理器
	rm := resource.NewManager(cfg)

	// 初始化应用管理器
	am := application.NewManager(cfg, rm)
	if am == nil {
		log.Fatal("Failed to create application manager - Docker connection failed")
	}

	// 启动 actor cluster gRPC 服务器
	lis, err1 := net.Listen("tcp", "0.0.0.0:25565")
	logrus.Infof("Actor cluster RPC server listening on %s", lis.Addr().String())

	if err1 != nil {
		log.Fatalf("RPC server: %v", err1)
	}
	svr := grpc.NewServer(
		grpc.MaxRecvMsgSize(512*1024*1024),
		grpc.MaxSendMsgSize(512*1024*1024),
	)
	defer svr.Stop()

	go func() {
		if err := svr.Serve(lis); err != nil {
			logrus.Errorf("gRPC server failed: %v", err)
		}
	}()

	cm := deployer.NewConnectionManager()
	cluster.RegisterServiceServer(svr, cm)

	// 初始化计算引擎（ignis）
	var computeEngine *ignis.Engine = nil
	if cfg.Ignis.Port != 0 {
		computeEngine, err = ignis.NewEngine(context.Background(), cfg, am, rm, cm)
		if err != nil {
			log.Fatalf("Failed to create compute engine: %v", err)
		}

		// 启动计算引擎
		if err := computeEngine.Start(context.Background()); err != nil {
			log.Fatalf("Failed to start compute engine: %v", err)
		}
		logrus.Info("Compute engine (ignis) started successfully")
	}

	// 设置计算引擎到 application manager
	if computeEngine != nil {
		am.SetComputeEngine(computeEngine)
	}

	// 初始化节点发现管理器
	pm := discovery.NewPeerManager(cfg.InitialPeers, rm)

	// Start gossip (if needed)
	// go pm.StartGossip(context.Background())

	// 启动 gRPC 节点服务器
	grpcSrv := server.NewGRPCServer(rm, pm)
	go func() {
		if err := grpcSrv.Start(cfg.PeerListenAddr); err != nil {
			log.Fatalf("gRPC peer server: %v", err)
		}
	}()

	// 启动 HTTP API 服务器
	srv := server.NewServer(rm, am, pm, cfg)
	go func() {
		if err := srv.Start(cfg.ListenAddr); err != nil {
			log.Fatalf("HTTP server: %v", err)
		}
	}()

	logrus.Info("Iarnet started successfully")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logrus.Info("Shutting down...")
	srv.Stop()
	grpcSrv.Stop()
	rm.StopMonitoring()

	// 停止计算引擎
	if computeEngine != nil {
		if err := computeEngine.Stop(context.Background()); err != nil {
			logrus.Errorf("Failed to stop compute engine: %v", err)
		}
	}

	logrus.Info("Shutdown complete")
}
