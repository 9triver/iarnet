package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/ignis/controller"
	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/store"
	"github.com/9triver/iarnet/internal/transport/rpc"
	"github.com/9triver/iarnet/internal/transport/zmq"
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

	channeler := zmq.NewChanneler(cfg.ZMQ.Port)
	store := store.NewStore()
	resourceManager := resource.NewManager(channeler, store, cfg.ComponentImages)
	controllerManager := controller.NewManager(resourceManager)

	ripcAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Ignis.Port)
	storeAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Resource.Store.Port)
	if err := rpc.Start(rpc.Options{
		IgnisAddr:     ripcAddr,
		StoreAddr:     storeAddr,
		ControllerMgr: controllerManager,
		StoreService:  resourceManager,
	}); err != nil {
		logrus.Fatalf("failed to start rpc servers: %v", err)
	}
	logrus.Infof("Ignis server listening on %s", ripcAddr)
	logrus.Infof("Store server listening on %s", storeAddr)

	controllerService := controller.NewService(controllerManager, resourceManager, resourceManager)

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start component manager to receive messages from components
	if err := resourceManager.StartComponentManager(ctx); err != nil {
		logrus.Fatalf("failed to start component manager: %v", err)
	}
	logrus.Info("Component manager started")

	logrus.Info("Iarnet started successfully")

	provider := component.NewProvider("local-docker", "localhost", 50051)
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

	rpc.Stop()
	logrus.Info("gRPC servers stopped")

	logrus.Info("Shutdown complete")
}
