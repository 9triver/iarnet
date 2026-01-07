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
	"github.com/9triver/iarnet/providers/mock/config"
	"github.com/9triver/iarnet/providers/mock/provider"
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

	// 转换任务执行时间配置
	taskDurationConfig := &provider.TaskDurationConfig{
		SmallMinMs:  cfg.TaskDuration.SmallMinMs,
		SmallMaxMs:  cfg.TaskDuration.SmallMaxMs,
		MediumMinMs: cfg.TaskDuration.MediumMinMs,
		MediumMaxMs: cfg.TaskDuration.MediumMaxMs,
		LargeMinMs:  cfg.TaskDuration.LargeMinMs,
		LargeMaxMs:  cfg.TaskDuration.LargeMaxMs,
	}
	logrus.Infof("Using task duration config: Small=[%d, %d]ms, Medium=[%d, %d]ms, Large=[%d, %d]ms",
		taskDurationConfig.SmallMinMs, taskDurationConfig.SmallMaxMs,
		taskDurationConfig.MediumMinMs, taskDurationConfig.MediumMaxMs,
		taskDurationConfig.LargeMinMs, taskDurationConfig.LargeMaxMs)

	service, err := provider.NewService(
		cfg.ResourceTags,
		totalCapacity,
		taskDurationConfig,
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

	logrus.Infof("Mock provider gRPC server listening on :%d", cfg.Server.Port)

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

