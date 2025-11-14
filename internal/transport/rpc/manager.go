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

// Options enumerates all required fields to start RPC servers.
type Options struct {
	IgnisAddr       string
	StoreAddr       string
	ControllerMgr   controller.Manager
	StoreService    store.Service
	IgnisServerOpts []grpc.ServerOption
	StoreServerOpts []grpc.ServerOption
}

// Manager manages the lifecycle of RPC servers.
type Manager struct {
	Ignis     *server
	Store     *server
	Options   Options
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewManager creates a new RPC server manager.
func NewManager(opts Options) *Manager {
	return &Manager{
		Ignis:     nil,
		Store:     nil,
		Options:   opts,
		startOnce: sync.Once{},
		stopOnce:  sync.Once{},
	}
}

// Start launches the Ignis and Store RPC servers.
func (m *Manager) Start() error {
	if m.Options.ControllerMgr == nil {
		return errors.New("controller manager is required")
	}
	if m.Options.StoreService == nil {
		return errors.New("store service is required")
	}
	if m.Options.IgnisAddr == "" {
		return errors.New("ignis listen address is required")
	}
	if m.Options.StoreAddr == "" {
		return errors.New("store listen address is required")
	}

	m.startOnce.Do(func() {
		ignis, err := startServer(m.Options.IgnisAddr, m.Options.IgnisServerOpts, func(s *grpc.Server) {
			ctrlpb.RegisterServiceServer(s, controllerrpc.NewServer(m.Options.ControllerMgr))
		})
		if err != nil {
			logrus.WithError(err).Error("failed to start ignis server")
		}

		store, err := startServer(m.Options.StoreAddr, m.Options.StoreServerOpts, func(s *grpc.Server) {
			storepb.RegisterServiceServer(s, storerpc.NewServer(m.Options.StoreService))
		})
		if err != nil {
			if ignis != nil {
				ignis.Stop()
			}
			logrus.WithError(err).Error("failed to start store server")
		}

		m.Ignis = ignis
		m.Store = store
	})

	return nil
}

// Stop stops all RPC servers gracefully.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		shutdownWithTimeout(m.Ignis, 30*time.Second)
		shutdownWithTimeout(m.Store, 30*time.Second)
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
