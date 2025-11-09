package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/9triver/iarnet/providers/docker"
	"github.com/9triver/iarnet/providers/docker/provider"
	"google.golang.org/grpc"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	// 加载配置
	cfg, err := docker.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 创建 Docker 管理器
	dockerMgr, err := provider.NewDockerManager(
		cfg.Docker.Host,
		cfg.Docker.TLSCertPath,
		cfg.Docker.TLSVerify,
		cfg.Docker.APIVersion,
	)
	if err != nil {
		log.Fatalf("Failed to create Docker manager: %v", err)
	}
	defer dockerMgr.Close()

	// 创建 gRPC 服务
	service := provider.NewService(dockerMgr)

	// 启动 gRPC 服务器
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	providerpb.RegisterProviderServiceServer(srv, service)

	log.Printf("Docker provider gRPC server listening on :%d", cfg.Server.Port)

	// 启动服务器
	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	srv.GracefulStop()
	log.Println("Shutdown complete")
}
