package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/9triver/iarnet/internal/util"
	"github.com/9triver/iarnet/providers/docker/config"
	"github.com/9triver/iarnet/providers/docker/provider"
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

	service, err := provider.NewService(
		cfg.Docker.Host,
		cfg.Docker.TLSCertPath,
		cfg.Docker.TLSVerify,
		cfg.Docker.APIVersion,
		cfg.ResourceTags,
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

	logrus.Infof("Docker provider gRPC server listening on :%d", cfg.Server.Port)

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
