package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/9triver/iarnet/internal/analysis"
	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/discovery"
	"github.com/9triver/iarnet/internal/integration"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/server"
	"github.com/9triver/ignis/platform"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
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

	if err != nil {
		log.Fatalf("Runner init: %v", err)
	}

	rm := resource.NewManager(cfg)
	am := application.NewManager(cfg, rm)
	if am == nil {
		log.Fatal("Failed to create application manager - Docker connection failed")
	}

	lis, err1 := net.Listen("tcp", "0.0.0.0:25565")
	logrus.Infof("RPC server listening on %s", lis.Addr().String())

	if err1 != nil {
		log.Fatalf("RPC server: %v", err1)
	}
	svr := grpc.NewServer(
		grpc.MaxRecvMsgSize(512*1024*1024),
		grpc.MaxSendMsgSize(512*1024*1024),
	)
	defer svr.Stop()

	// fix: 启动 gRPC 服务器
	go func() {
		if err := svr.Serve(lis); err != nil {
			logrus.Errorf("gRPC server failed: %v", err)
		}
	}()

	cm := integration.NewConnectionManager()
	cluster.RegisterServiceServer(svr, cm)

	// 初始化 Ignis 平台
	var ignisPlatform *platform.Platform = nil
	if cfg.Ignis.Port != 0 {
		ignisPlatform = platform.NewPlatform(
			context.Background(), "0.0.0.0:"+strconv.FormatInt(int64(cfg.Ignis.Port), 10),
			integration.NewDeployer(am, rm, cm, cfg),
		)
		if err != nil {
			log.Fatalf("Ignis platform init: %v", err)
		}
	}

	am.SetIgnisPlatform(ignisPlatform)

	go ignisPlatform.Run()

	// 创建并设置代码分析服务
	analysisService := analysis.NewMockCodeAnalysisService(rm)
	am.SetAnalysisService(analysisService)

	pm := discovery.NewPeerManager(cfg.InitialPeers, rm)

	// Start gossip
	// go pm.StartGossip(context.Background())

	// Start gRPC peer server
	grpcSrv := server.NewGRPCServer(rm, pm)
	go func() {
		if err := grpcSrv.Start(cfg.PeerListenAddr); err != nil {
			log.Fatalf("gRPC server: %v", err)
		}
	}()

	// Start HTTP server
	srv := server.NewServer(rm, am, pm, cfg)
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
