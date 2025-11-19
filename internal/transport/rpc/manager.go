package rpc

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/9triver/iarnet/internal/domain/application/logger"
	"github.com/9triver/iarnet/internal/domain/ignis/controller"
	"github.com/9triver/iarnet/internal/domain/resource/store"
	loggerpb "github.com/9triver/iarnet/internal/proto/application/logger"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	storepb "github.com/9triver/iarnet/internal/proto/resource/store"
	loggerrpc "github.com/9triver/iarnet/internal/transport/rpc/application/logger"
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
	IgnisAddr         string
	StoreAddr         string
	LoggerAddr        string
	ControllerService controller.Service
	StoreService      store.Service
	LoggerService     logger.Service
	IgnisServerOpts   []grpc.ServerOption
	StoreServerOpts   []grpc.ServerOption
	LoggerServerOpts  []grpc.ServerOption
}

// Manager manages the lifecycle of RPC servers.
type Manager struct {
	Ignis     *server
	Store     *server
	Logger    *server
	Options   Options
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewManager creates a new RPC server manager.
func NewManager(opts Options) *Manager {
	return &Manager{
		Ignis:     nil,
		Store:     nil,
		Logger:    nil,
		Options:   opts,
		startOnce: sync.Once{},
		stopOnce:  sync.Once{},
	}
}

// Start launches the Ignis, Store, and Logger RPC servers.
func (m *Manager) Start() error {
	if m.Options.ControllerService == nil {
		return errors.New("controller service is required")
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
		var startedServers []*server

		// 启动 Ignis 服务器
		ignis, err := startServer(m.Options.IgnisAddr, m.Options.IgnisServerOpts, func(s *grpc.Server) {
			ctrlpb.RegisterServiceServer(s, controllerrpc.NewServer(m.Options.ControllerService))
		})
		if err != nil {
			logrus.WithError(err).Error("failed to start ignis server")
		} else {
			logrus.Infof("Ignis server listening on %s", m.Options.IgnisAddr)
			m.Ignis = ignis
			startedServers = append(startedServers, ignis)
		}

		// 启动 Store 服务器
		store, err := startServer(m.Options.StoreAddr, m.Options.StoreServerOpts, func(s *grpc.Server) {
			storepb.RegisterServiceServer(s, storerpc.NewServer(m.Options.StoreService))
		})
		if err != nil {
			logrus.WithError(err).Error("failed to start store server")
			// 停止已启动的服务器
			for _, s := range startedServers {
				s.Stop()
			}
		} else {
			logrus.Infof("Store server listening on %s", m.Options.StoreAddr)
			m.Store = store
			startedServers = append(startedServers, store)
		}

		// 启动 Logger 服务器（如果配置了）
		if m.Options.LoggerAddr != "" && m.Options.LoggerService != nil {
			logger, err := startServer(m.Options.LoggerAddr, m.Options.LoggerServerOpts, func(s *grpc.Server) {
				loggerpb.RegisterLoggerServiceServer(s, loggerrpc.NewServer(m.Options.LoggerService))
			})
			if err != nil {
				logrus.WithError(err).Error("failed to start logger server")
				// 停止已启动的服务器
				for _, s := range startedServers {
					s.Stop()
				}
			} else {
				logrus.Infof("Logger server listening on %s", m.Options.LoggerAddr)
				m.Logger = logger
			}
		} else if m.Options.LoggerAddr != "" {
			logrus.Warn("Logger address is configured but logger service is not provided, skipping logger server")
		}
	})

	return nil
}

// Stop stops all RPC servers gracefully.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		shutdownWithTimeout(m.Ignis, 30*time.Second)
		shutdownWithTimeout(m.Store, 30*time.Second)
		shutdownWithTimeout(m.Logger, 30*time.Second)
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
