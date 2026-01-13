package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/application"
	"github.com/9triver/iarnet/internal/domain/audit"
	"github.com/9triver/iarnet/internal/domain/execution"
	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	userrepo "github.com/9triver/iarnet/internal/infra/repository/auth"
	applicationAPI "github.com/9triver/iarnet/internal/transport/http/application"
	auditAPI "github.com/9triver/iarnet/internal/transport/http/audit"
	authAPI "github.com/9triver/iarnet/internal/transport/http/auth"
	resourceAPI "github.com/9triver/iarnet/internal/transport/http/resource"
	httpauth "github.com/9triver/iarnet/internal/transport/http/util/auth"
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
	AuditMgr         *audit.Manager
}

type Server struct {
	Server   *http.Server
	Router   *mux.Router
	UserRepo userrepo.UserRepo
}

func NewServer(opts Options) *Server {
	router := mux.NewRouter()

	// 设置 JWT 密钥（如果配置中有）
	if opts.Config != nil && opts.Config.Auth.JWTSecret != "" {
		httpauth.SetSecret(opts.Config.Auth.JWTSecret)
	}

	// 初始化用户仓库
	userDBPath := opts.Config.DataDir + "/users.db"
	userRepo, err := userrepo.NewUserRepoSQLite(userDBPath)
	if err != nil {
		logrus.Fatalf("Failed to initialize user repository: %v", err)
	}

	// 初始化超级管理员（如果数据库为空且配置中有超级管理员）
	if err := authAPI.InitSuperAdmin(opts.Config, userRepo); err != nil {
		logrus.Errorf("Failed to initialize super admin: %v", err)
	}

	// 应用认证中间件（除了登录接口）
	router.Use(httpauth.AuthMiddleware)

	// 创建认证API实例（用于权限中间件）
	authAPIIstance := authAPI.NewAPI(opts.Config, userRepo)
	
	// 应用权限中间件
	router.Use(authAPIIstance.PermissionMiddleware)

	authAPI.RegisterRoutes(router, opts.Config, userRepo)
	applicationAPI.RegisterRoutes(router, opts.AppMgr, opts.AuditMgr)
	resourceAPI.RegisterRoutes(router, opts.ResMgr, opts.Config, opts.DiscoveryService, opts.AuditMgr)
	auditAPI.RegisterRoutes(router, opts.ResMgr, opts.AuditMgr)

	return &Server{
		Server:   &http.Server{Addr: fmt.Sprintf("0.0.0.0:%d", opts.Port), Handler: router},
		Router:   router,
		UserRepo: userRepo,
	}
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
	// 关闭用户仓库连接
	if s.UserRepo != nil {
		if closer, ok := s.UserRepo.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				logrus.Errorf("Failed to close user repository: %v", err)
			}
		}
	}
	logrus.Info("HTTP server stopped")
}
