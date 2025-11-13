package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/9triver/iarnet/internal/config"
	domaincontroller "github.com/9triver/iarnet/internal/domain/ignis/controller"
	domainresource "github.com/9triver/iarnet/internal/domain/resource"
	domaincomponent "github.com/9triver/iarnet/internal/domain/resource/component"
	domainstore "github.com/9triver/iarnet/internal/domain/resource/store"
	"github.com/9triver/iarnet/internal/transport/rpc"
	"github.com/9triver/iarnet/internal/transport/zmq"
	"github.com/9triver/iarnet/internal/util"
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

	util.InitLogger()

	lis, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", cfg.Ignis.Port))
	if err != nil {
		logrus.Fatalf("failed to listen: %v", err)
	}
	server := grpc.NewServer()

	channeler := zmq.NewChanneler(cfg.ZMQ.Port)
	store := domainstore.NewStore()
	resourceManager := domainresource.NewManager(channeler, store, cfg.ComponentImages)
	controllerManager := domaincontroller.NewManager(resourceManager)
	rpc.RegisterIgnisServer(server, controllerManager)

	logrus.Infof("Ignis server listening on %s", lis.Addr().String())
	go func() {
		if err := server.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logrus.Fatalf("failed to serve: %v", err)
		}
	}()

	storeLis, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", cfg.Resource.Store.Port))
	if err != nil {
		logrus.Fatalf("failed to listen: %v", err)
	}
	storeServer := grpc.NewServer()
	rpc.RegisterStoreServiceServer(storeServer, resourceManager)

	logrus.Infof("Store server listening on %s", storeLis.Addr().String())
	go func() {
		if err := storeServer.Serve(storeLis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logrus.Fatalf("failed to serve store service: %v", err)
		}
	}()

	controllerService := domaincontroller.NewService(controllerManager, resourceManager, resourceManager)

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start component manager to receive messages from components
	if err := resourceManager.StartComponentManager(ctx); err != nil {
		logrus.Fatalf("failed to start component manager: %v", err)
	}
	logrus.Info("Component manager started")

	logrus.Info("Iarnet started successfully")

	provider := domaincomponent.NewProvider("local-docker", "localhost", 50051)
	resourceManager.RegisterProvider(provider)
	logrus.Infof("Local docker provider registered")
	controllerService.CreateController(context.Background(), "1")
	logrus.Infof("Controller 1 created for test")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logrus.Info("Shutting down...")

	// Cancel context to stop component manager and ZMQ receiver
	cancel()

	// Close ZMQ Channeler to release port and resources
	// This must be done before closing listeners to ensure ZMQ socket is properly closed
	if err := channeler.Close(); err != nil {
		logrus.Errorf("Error closing ZMQ channeler: %v", err)
	}

	server.GracefulStop()
	storeServer.GracefulStop()
	logrus.Info("gRPC servers stopped")

	// Close listeners to stop accepting new connections
	lis.Close()
	storeLis.Close()
	logrus.Info("Listeners closed")

	logrus.Info("Shutdown complete")
}
