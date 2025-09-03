package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/discovery"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/runner"
	"github.com/9triver/iarnet/internal/server"
	"github.com/9triver/iarnet/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type peerServer struct {
	proto.UnimplementedPeerServiceServer
	pm *discovery.PeerManager
}

func (ps *peerServer) ExchangePeers(ctx context.Context, req *proto.ExchangeRequest) (*proto.ExchangeResponse, error) {
	ps.pm.AddPeers(req.KnownPeers)
	return &proto.ExchangeResponse{KnownPeers: ps.pm.GetPeers()}, nil
}

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
	pm := discovery.NewPeerManager(cfg.InitialPeers)

	// Start gossip
	go pm.StartGossip(context.Background())

	// Start gRPC peer server
	grpcServer := grpc.NewServer()
	proto.RegisterPeerServiceServer(grpcServer, &peerServer{pm: pm})
	go func() {
		lis, err := net.Listen("tcp", cfg.PeerListenAddr)
		if err != nil {
			log.Fatalf("gRPC listen: %v", err)
		}
		logrus.Infof("gRPC server on %s", cfg.PeerListenAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC serve: %v", err)
		}
	}()

	// Start HTTP server
	srv := server.NewServer(r, rm)
	go func() {
		if err := srv.Start(cfg.ListenAddr); err != nil {
			log.Fatalf("HTTP server: %v", err)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	srv.Stop()
	grpcServer.GracefulStop()
	logrus.Info("Shutdown complete")
}
