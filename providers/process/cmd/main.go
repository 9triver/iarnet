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
	"github.com/9triver/iarnet/providers/process/config"
	"github.com/9triver/iarnet/providers/process/provider"
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

	// ZMQ 发送器（暂时为 nil，需要从外部注入）
	// 在实际使用中，这个函数应该由 iarnet 的 ComponentChanneler 提供
	var zmqSender func(componentID string, data []byte)
	zmqSender = func(componentID string, data []byte) {
		// TODO: 集成 iarnet 的 ComponentChanneler
		logrus.Debugf("Would send ZMQ message to component %s (size: %d bytes)", componentID, len(data))
	}

	service, err := provider.NewService(
		cfg.Ignis.Address,
		cfg.ResourceTags,
		totalCapacity,
		cfg.SupportedLanguages,
		zmqSender,
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

	logrus.Infof("Process provider gRPC server listening on :%d", cfg.Server.Port)
	logrus.Infof("Connected to Ignis at %s", cfg.Ignis.Address)

	go func() {
		if err := srv.Serve(lis); err != nil {
			logrus.Fatalf("Failed to serve: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logrus.Infof("Shutting down...")
	srv.GracefulStop()
	logrus.Infof("Shutdown complete")
}
