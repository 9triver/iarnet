package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/9triver/iarnet/internal/util"
	"github.com/9triver/iarnet/providers/unikernel/config"
	"github.com/9triver/iarnet/providers/unikernel/provider"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func main() {
	util.InitLogger()

	configPath := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logrus.Fatalf("Failed to load config: %v", err)
	}

	// 解析资源容量配置（必须配置）
	if cfg.Resource.CPU == 0 && cfg.Resource.Memory == "" && cfg.Resource.GPU == 0 {
		logrus.Fatalf("Resource capacity must be configured in config file. Please set resource.cpu, resource.memory, and/or resource.gpu")
	}

	memoryBytes, err := cfg.Resource.ParseMemory()
	if err != nil {
		logrus.Fatalf("Failed to parse memory config: %v", err)
	}

	totalCapacity := &resourcepb.Info{
		Cpu:    cfg.Resource.CPU,
		Memory: memoryBytes,
		Gpu:    cfg.Resource.GPU,
	}
	logrus.Infof("Using configured resource capacity: CPU=%d millicores, Memory=%d bytes (%s), GPU=%d",
		totalCapacity.Cpu, totalCapacity.Memory, cfg.Resource.Memory, totalCapacity.Gpu)

	service, err := provider.NewService(
		cfg.ResourceTags,
		totalCapacity,
		cfg.SupportedLanguages,
		cfg.DNS.Hosts,
		cfg.WebSocket.Port,
		cfg.Unikernel.BaseDir,
		cfg.Unikernel.Solo5HvtPath,
	)
	if err != nil {
		logrus.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	lis, err := net.Listen("tcp4", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		logrus.Fatalf("Failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	providerpb.RegisterServiceServer(srv, service)

	logrus.Infof("Unikernel provider gRPC server listening on :%d", cfg.Server.Port)
	logrus.Infof("WebSocket server listening on :%d", cfg.WebSocket.Port)

	go func() {
		if err := srv.Serve(lis); err != nil {
			logrus.Fatalf("Failed to serve: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logrus.Infof("Received shutdown signal, shutting down gracefully...")

	// 先停止 gRPC 服务器（停止接受新请求）
	srv.GracefulStop()
	logrus.Infof("gRPC server stopped")

	// 然后关闭 service（这会关闭所有 unikernel 进程）
	// defer 会在函数返回时执行，但这里我们显式调用以确保顺序
	service.Close()

	logrus.Infof("Shutdown complete")
}
