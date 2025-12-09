package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/application"
	"github.com/9triver/iarnet/internal/domain/execution"
	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	applicationAPI "github.com/9triver/iarnet/internal/transport/http/application"
	resourceAPI "github.com/9triver/iarnet/internal/transport/http/resource"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Options struct {
	Port             int
	Config           *config.Config
	AppMgr           *application.Manager
	ResMgr           *resource.Manager
	Platform         *execution.Platform
	DiscoveryService discovery.Service
}

type Server struct {
	Server *http.Server
	Router *mux.Router
}

func NewServer(opts Options) *Server {
	router := mux.NewRouter()
	applicationAPI.RegisterRoutes(router, opts.AppMgr)
	resourceAPI.RegisterRoutes(router, opts.ResMgr, opts.Config, opts.DiscoveryService)

	return &Server{Server: &http.Server{Addr: fmt.Sprintf("0.0.0.0:%d", opts.Port), Handler: router}, Router: router}
}

func (s *Server) Start() {
	go func() {
		if err := s.Server.ListenAndServe(); err != nil {
			logrus.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()
	logrus.Infof("HTTP server started on %s", s.Server.Addr)
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Server.Shutdown(ctx); err != nil {
		logrus.WithError(err).Error("Failed to stop HTTP server")
	}
	logrus.Info("HTTP server stopped")
}
