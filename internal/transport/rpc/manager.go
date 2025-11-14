package rpc

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/9triver/iarnet/internal/domain/ignis/controller"
	"github.com/9triver/iarnet/internal/domain/resource/store"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	storepb "github.com/9triver/iarnet/internal/proto/resource/store"
	controllerrpc "github.com/9triver/iarnet/internal/transport/rpc/ignis/controller"
	storerpc "github.com/9triver/iarnet/internal/transport/rpc/resource/store"
)

type server struct {
	Server   *grpc.Server
	Listener net.Listener
}

func (rs *server) GracefulStop() {
	if rs == nil {
		return
	}
	if rs.Server != nil {
		rs.Server.GracefulStop()
	}
	if rs.Listener != nil {
		_ = rs.Listener.Close()
	}
}

func (rs *server) Stop() {
	if rs == nil {
		return
	}
	if rs.Server != nil {
		rs.Server.Stop()
	}
	if rs.Listener != nil {
		_ = rs.Listener.Close()
	}
}

// Options enumerates all required fields to bootstrap RPC servers.
type Options struct {
	IgnisAddr       string
	StoreAddr       string
	ControllerMgr   controller.Manager
	StoreService    store.Service
	IgnisServerOpts []grpc.ServerOption
	StoreServerOpts []grpc.ServerOption
}

// Servers bundles the running RPC servers for easy lifecycle management.
type servers struct {
	Ignis *server
	Store *server
}

var (
	svrs      *servers
	startOnce sync.Once
	stopOnce  sync.Once
)

// Start launches the Ignis and Store RPC servers using the provided options.
func Start(opts Options) error {
	if opts.ControllerMgr == nil {
		return errors.New("controller manager is required")
	}
	if opts.StoreService == nil {
		return errors.New("store service is required")
	}
	if opts.IgnisAddr == "" {
		return errors.New("ignis listen address is required")
	}
	if opts.StoreAddr == "" {
		return errors.New("store listen address is required")
	}

	startOnce.Do(func() {
		ignis, err := startServer(opts.IgnisAddr, opts.IgnisServerOpts, func(s *grpc.Server) {
			ctrlpb.RegisterServiceServer(s, controllerrpc.NewServer(opts.ControllerMgr))
		})
		if err != nil {
			logrus.WithError(err).Error("failed to start ignis server")
		}

		store, err := startServer(opts.StoreAddr, opts.StoreServerOpts, func(s *grpc.Server) {
			storepb.RegisterServiceServer(s, storerpc.NewServer(opts.StoreService))
		})
		if err != nil {
			ignis.Stop()
			logrus.WithError(err).Error("failed to start store server")
		}

		svrs = &servers{Ignis: ignis, Store: store}
	})

	return nil
}

func Stop() {
	if svrs == nil {
		return
	}
	stopOnce.Do(func() {
		shutdownWithTimeout(svrs.Ignis, 30*time.Second)
		shutdownWithTimeout(svrs.Store, 30*time.Second)
		svrs = nil
	})
}

func shutdownWithTimeout(s *server, timeout time.Duration) {
	if s == nil {
		return
	}

	done := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(timeout):
		logrus.Warn("grpc server graceful stop timed out, forcing stop")
		s.Stop()
	}
}

func startServer(addr string, opts []grpc.ServerOption, register func(*grpc.Server)) (*server, error) {
	lis, err := net.Listen("tcp4", addr)
	if err != nil {
		return nil, err
	}

	s := grpc.NewServer(opts...)
	register(s)

	go func() {
		if err := s.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logrus.WithError(err).Error("grpc server stopped unexpectedly")
		}
	}()

	return &server{Server: s, Listener: lis}, nil
}
