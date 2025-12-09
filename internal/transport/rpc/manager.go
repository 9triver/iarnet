package rpc

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	applogger "github.com/9triver/iarnet/internal/domain/application/logger"
	"github.com/9triver/iarnet/internal/domain/execution/controller"
	resdiscovery "github.com/9triver/iarnet/internal/domain/resource/discovery"
	reslogger "github.com/9triver/iarnet/internal/domain/resource/logger"
	resscheduler "github.com/9triver/iarnet/internal/domain/resource/scheduler"
	"github.com/9triver/iarnet/internal/domain/resource/store"
	appLoggerPB "github.com/9triver/iarnet/internal/proto/application/logger"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	discoverypb "github.com/9triver/iarnet/internal/proto/resource/discovery"
	resLoggerPB "github.com/9triver/iarnet/internal/proto/resource/logger"
	schedulerpb "github.com/9triver/iarnet/internal/proto/resource/scheduler"
	storepb "github.com/9triver/iarnet/internal/proto/resource/store"
	appLoggerRPC "github.com/9triver/iarnet/internal/transport/rpc/application/logger"
	controllerrpc "github.com/9triver/iarnet/internal/transport/rpc/ignis/controller"
	discoveryrpc "github.com/9triver/iarnet/internal/transport/rpc/resource/discovery"
	resLoggerRPC "github.com/9triver/iarnet/internal/transport/rpc/resource/logger"
	schedulerrpc "github.com/9triver/iarnet/internal/transport/rpc/resource/scheduler"
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
	IgnisAddr                string
	StoreAddr                string
	LoggerAddr               string
	ResourceLoggerAddr       string
	DiscoveryAddr            string
	SchedulerAddr            string
	ControllerService        controller.Service
	StoreService             store.Service
	LoggerService            applogger.Service
	ResourceLoggerService    reslogger.Service
	DiscoveryService         resdiscovery.Service
	DiscoveryManager         *resdiscovery.NodeDiscoveryManager
	SchedulerService         resscheduler.Service
	IgnisServerOpts          []grpc.ServerOption
	StoreServerOpts          []grpc.ServerOption
	LoggerServerOpts         []grpc.ServerOption
	ResourceLoggerServerOpts []grpc.ServerOption
	DiscoveryServerOpts      []grpc.ServerOption
	SchedulerServerOpts      []grpc.ServerOption
}

// Manager manages the lifecycle of RPC servers.
type Manager struct {
	Ignis          *server
	Store          *server
	Logger         *server
	ResourceLogger *server
	Discovery      *server
	Scheduler      *server
	Options        Options
	startOnce      sync.Once
	stopOnce       sync.Once
}

// NewManager creates a new RPC server manager.
func NewManager(opts Options) *Manager {
	return &Manager{
		Ignis:          nil,
		Store:          nil,
		Logger:         nil,
		ResourceLogger: nil,
		Discovery:      nil,
		Scheduler:      nil,
		Options:        opts,
		startOnce:      sync.Once{},
		stopOnce:       sync.Once{},
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
	if m.Options.ResourceLoggerAddr == "" {
		return errors.New("resource logger listen address is required")
	}
	if m.Options.ResourceLoggerService == nil {
		return errors.New("resource logger service is required")
	}

	m.startOnce.Do(func() {
		var startedServers []*server

		// 配置 Ignis 服务器选项，添加最大接收消息大小限制，TODO: 加入配置文件
		ignisOpts := append([]grpc.ServerOption{}, m.Options.IgnisServerOpts...)
		ignisOpts = append(ignisOpts, grpc.MaxRecvMsgSize(512*1024*1024))

		// 启动 Ignis 服务器
		ignis, err := startServer(m.Options.IgnisAddr, ignisOpts, func(s *grpc.Server) {
			ctrlpb.RegisterServiceServer(s, controllerrpc.NewServer(m.Options.ControllerService))
		})
		if err != nil {
			logrus.WithError(err).Error("failed to start ignis server")
		} else {
			logrus.Infof("Ignis server listening on %s", m.Options.IgnisAddr)
			m.Ignis = ignis
			startedServers = append(startedServers, ignis)
		}

		// 配置 Store 服务器选项，添加最大接收消息大小限制，TODO: 加入配置文件
		storeOpts := append([]grpc.ServerOption{}, m.Options.StoreServerOpts...)
		storeOpts = append(storeOpts, grpc.MaxRecvMsgSize(512*1024*1024))

		// 启动 Store 服务器
		store, err := startServer(m.Options.StoreAddr, storeOpts, func(s *grpc.Server) {
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
				appLoggerPB.RegisterLoggerServiceServer(s, appLoggerRPC.NewServer(m.Options.LoggerService))
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
		// 启动 Resource Logger 服务器（如果配置了）
		if m.Options.ResourceLoggerAddr != "" && m.Options.ResourceLoggerService != nil {
			resLogger, err := startServer(m.Options.ResourceLoggerAddr, m.Options.ResourceLoggerServerOpts, func(s *grpc.Server) {
				resLoggerPB.RegisterLoggerServiceServer(s, resLoggerRPC.NewServer(m.Options.ResourceLoggerService))
			})
			if err != nil {
				logrus.WithError(err).Error("failed to start resource logger server")
				for _, s := range startedServers {
					s.Stop()
				}
			} else {
				logrus.Infof("Resource logger server listening on %s", m.Options.ResourceLoggerAddr)
				m.ResourceLogger = resLogger
			}
		} else if m.Options.ResourceLoggerAddr != "" {
			logrus.Warn("Resource logger address is configured but service is not provided, skipping resource logger server")
		}

		// 启动 Discovery 服务器（如果配置了）
		if m.Options.DiscoveryAddr != "" && m.Options.DiscoveryService != nil && m.Options.DiscoveryManager != nil {
			discoveryOpts := append([]grpc.ServerOption{}, m.Options.DiscoveryServerOpts...)
			discoveryOpts = append(discoveryOpts, grpc.MaxRecvMsgSize(512*1024*1024))

			discovery, err := startServer(m.Options.DiscoveryAddr, discoveryOpts, func(s *grpc.Server) {
				discoverypb.RegisterDiscoveryServiceServer(s, discoveryrpc.NewServer(m.Options.DiscoveryService, m.Options.DiscoveryManager))
			})
			if err != nil {
				logrus.WithError(err).Error("failed to start discovery server")
				for _, s := range startedServers {
					s.Stop()
				}
			} else {
				logrus.Infof("Discovery server listening on %s", m.Options.DiscoveryAddr)
				m.Discovery = discovery
			}
		} else if m.Options.DiscoveryAddr != "" {
			logrus.Warn("Discovery address is configured but service or manager is not provided, skipping discovery server")
		}

		// 启动 Scheduler 服务器（如果配置了）
		if m.Options.SchedulerAddr != "" && m.Options.SchedulerService != nil {
			schedulerOpts := append([]grpc.ServerOption{}, m.Options.SchedulerServerOpts...)
			schedulerOpts = append(schedulerOpts, grpc.MaxRecvMsgSize(512*1024*1024))

			scheduler, err := startServer(m.Options.SchedulerAddr, schedulerOpts, func(s *grpc.Server) {
				schedulerpb.RegisterSchedulerServiceServer(s, schedulerrpc.NewServer(m.Options.SchedulerService))
			})
			if err != nil {
				logrus.WithError(err).Error("failed to start scheduler server")
				for _, s := range startedServers {
					s.Stop()
				}
			} else {
				logrus.Infof("Scheduler server listening on %s", m.Options.SchedulerAddr)
				m.Scheduler = scheduler
			}
		} else if m.Options.SchedulerAddr != "" {
			logrus.Warn("Scheduler address is configured but service is not provided, skipping scheduler server")
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
		shutdownWithTimeout(m.ResourceLogger, 30*time.Second)
		shutdownWithTimeout(m.Discovery, 30*time.Second)
		shutdownWithTimeout(m.Scheduler, 30*time.Second)
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
