package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/runner"
	"github.com/9triver/iarnet/internal/server/request"
	"github.com/9triver/iarnet/internal/server/response"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Server struct {
	router *mux.Router
	runner runner.Runner
	resMgr *resource.Manager
	appMgr *application.Manager
	ctx    context.Context
	cancel context.CancelFunc
}

func NewServer(r runner.Runner, rm *resource.Manager, am *application.Manager) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{router: mux.NewRouter(), runner: r, resMgr: rm, appMgr: am, ctx: ctx, cancel: cancel}
	s.router.HandleFunc("/run", s.handleRun).Methods("POST")
	s.router.HandleFunc("/resource/capacity", s.handleResourceCapacity).Methods("GET")
	s.router.HandleFunc("/resource/providers", s.handleResourceProviders).Methods("GET")

	s.router.HandleFunc("/application/apps", s.handleGetApplications).Methods("GET")
	s.router.HandleFunc("/application/stats", s.handleGetApplicationStats).Methods("GET")
	s.router.HandleFunc("/application/create", s.handleCreateApplication).Methods("POST")

	return s
}

func (s *Server) handleRun(w http.ResponseWriter, req *http.Request) {
	var spec runner.ContainerSpec
	if err := json.NewDecoder(req.Body).Decode(&spec); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	usageReq := resource.Usage{CPU: spec.CPU, Memory: spec.Memory, GPU: spec.GPU}
	// if s.resMgr.CanAllocate(usageReq) == nil {
	// 	response.ServiceUnavailable("Resource limit exceeded").WriteJSON(w)
	// 	return
	// }
	if err := s.runner.Run(s.ctx, spec); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to run container", err)
		return
	}
	s.resMgr.Allocate(usageReq)
	response.Accepted("Container run request accepted").WriteJSON(w)
}

func (s *Server) handleResourceCapacity(w http.ResponseWriter, req *http.Request) {
	capacity, err := s.resMgr.GetCapacity(s.ctx)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to get resource capacity", err)
		return
	}
	if err := response.WriteSuccess(w, capacity); err != nil {
		logrus.Errorf("Failed to write response: %v", err)
	}
}

func (s *Server) handleResourceProviders(w http.ResponseWriter, req *http.Request) {
	providers := s.resMgr.GetProviders()

	providerInfos := []response.ResourceProviderInfo{}

	for _, provider := range providers {

		capacity, err := provider.GetCapacity(s.ctx)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "failed to get resource capacity", err)
			return
		}

		providerInfo := response.ResourceProviderInfo{
			ID:     provider.GetID(),
			Name:   provider.GetName(),
			URL:    "http://localhost:2376", // TODO: 默认Docker URL，实际应该从provider获取
			Type:   provider.GetType(),
			Status: provider.GetStatus(),
			CPUUsage: response.UsageInfo{
				Used:  capacity.Used.CPU,
				Total: capacity.Total.CPU,
			},
			MemoryUsage: response.UsageInfo{
				Used:  capacity.Used.Memory,
				Total: capacity.Total.Memory,
			},
			LastUpdateTime: provider.GetLastUpdateTime().Format("2006-01-02 15:04:05"),
		}

		providerInfos = append(providerInfos, providerInfo)
	}

	getResourceProvidersResponse := response.GetResourceProvidersResponse{
		Providers: providerInfos,
	}

	if err := response.WriteSuccess(w, getResourceProvidersResponse); err != nil {
		logrus.Errorf("Failed to write response: %v", err)
	}
}

// handleGetApplications 处理获取所有应用请求
func (s *Server) handleGetApplications(w http.ResponseWriter, req *http.Request) {
	logrus.Info("Received request to get all applications")
	apps := s.appMgr.GetAllApplications()
	logrus.Infof("Retrieved %d applications from manager", len(apps))

	appInfos := []response.ApplicationInfo{}
	for _, app := range apps {
		appInfos = append(appInfos, response.ApplicationInfo{
			ID:           app.ID,
			Name:         app.Name,
			ImportType:   app.ImportType,
			GitUrl:       app.GitUrl,
			Branch:       app.Branch,
			DockerImage:  app.DockerImage,
			DockerTag:    app.DockerTag,
			Type:         app.Type,
			Description:  app.Description,
			Ports:        app.Ports,
			HealthCheck:  app.HealthCheck,
			Status:       app.Status,
			LastDeployed: app.LastDeployed.Format("2006-01-02 15:04:05"),
			RunningOn:    app.GetRunningOn(),
		})
	}

	getApplicationsResponse := response.GetApplicationsResponse{
		Applications: appInfos,
	}

	if err := response.WriteSuccess(w, getApplicationsResponse); err != nil {
		logrus.Errorf("Failed to write applications response: %v", err)
		return
	}
	logrus.Debug("Successfully sent applications list response")
}

// handleGetApplicationStats 处理获取应用统计信息请求
func (s *Server) handleGetApplicationStats(w http.ResponseWriter, req *http.Request) {
	logrus.Info("Received request to get application statistics")
	stats := s.appMgr.GetApplicationStats()
	logrus.Infof("Application statistics: Total=%d, Running=%d, Stopped=%d, Undeployed=%d, Failed=%d",
		stats.Total, stats.Running, stats.Stopped, stats.Undeployed, stats.Failed)
	if err := response.WriteSuccess(w, stats); err != nil {
		logrus.Errorf("Failed to write application stats response: %v", err)
		return
	}
	logrus.Debug("Successfully sent application statistics response")
}

func (s *Server) handleCreateApplication(w http.ResponseWriter, req *http.Request) {
	logrus.Infof("Received create application request from %s", req.RemoteAddr)

	var createReq request.CreateApplicationRequest
	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		logrus.Errorf("Failed to decode create application request: %v", err)
		response.WriteError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	logrus.Infof("Creating application: name=%s, importType=%s", createReq.Name, createReq.ImportType)

	// 验证必填字段
	if createReq.Name == "" {
		logrus.Warn("Application name is missing in create request")
		response.WriteError(w, http.StatusBadRequest, "application name is required", nil)
		return
	}

	if createReq.ImportType != "git" && createReq.ImportType != "docker" {
		logrus.Warnf("Invalid import type: %s", createReq.ImportType)
		response.WriteError(w, http.StatusBadRequest, "importType must be 'git' or 'docker'", nil)
		return
	}

	// 根据导入类型验证相应字段
	logrus.Debugf("Validating import type specific fields for: %s", createReq.ImportType)
	switch createReq.ImportType {
	case "git":
		if createReq.GitUrl == nil || *createReq.GitUrl == "" {
			logrus.Warn("Git URL is missing for git import")
			response.WriteError(w, http.StatusBadRequest, "gitUrl is required for git import", nil)
			return
		}
		if createReq.Branch == nil || *createReq.Branch == "" {
			defaultBranch := "main"
			createReq.Branch = &defaultBranch
			logrus.Infof("Using default branch 'main' for git import")
		}
		logrus.Infof("Git import validated: url=%s, branch=%s", *createReq.GitUrl, *createReq.Branch)
	case "docker":
		if createReq.DockerImage == nil || *createReq.DockerImage == "" {
			logrus.Warn("Docker image is missing for docker import")
			response.WriteError(w, http.StatusBadRequest, "dockerImage is required for docker import", nil)
			return
		}
		if createReq.DockerTag == nil || *createReq.DockerTag == "" {
			defaultTag := "latest"
			createReq.DockerTag = &defaultTag
			logrus.Infof("Using default tag 'latest' for docker import")
		}
		logrus.Infof("Docker import validated: image=%s, tag=%s", *createReq.DockerImage, *createReq.DockerTag)
	}

	// 创建应用
	logrus.Infof("Creating application instance for: %s", createReq.Name)
	app := s.appMgr.CreateApplication(&createReq)
	logrus.Infof("Application created with ID: %s", app.ID)

	// 构建容器规格
	logrus.Info("Building container specification")
	var spec resource.ContainerSpec
	if createReq.ImportType == "docker" {
		// Docker 导入方式直接使用镜像
		spec.Image = *createReq.DockerImage + ":" + *createReq.DockerTag
		logrus.Infof("Using Docker image: %s", spec.Image)
	} else {
		// Git 导入方式需要构建镜像（这里暂时使用占位符，实际需要实现 Git 构建逻辑）
		spec.Image = "placeholder:" + app.ID // 实际应该是构建后的镜像名
		logrus.Infof("Using placeholder image for Git import: %s", spec.Image)
		logrus.Warn("Git build logic not implemented yet, using placeholder image")
	}

	// 设置默认资源限制
	spec.CPU = 1.0                   // 1 CPU 核心
	spec.Memory = 1024 * 1024 * 1024 // 1GB 内存
	spec.GPU = 0                     // 默认不使用 GPU
	logrus.Infof("Resource limits set: CPU=%.1f, Memory=%dMB, GPU=%d", spec.CPU, spec.Memory, spec.GPU)

	if len(createReq.Ports) > 0 {
		spec.Ports = createReq.Ports
		logrus.Infof("Ports configured: %v", spec.Ports)
	} else {
		logrus.Info("No ports specified for application")
	}

	// 尝试运行容器（这里可能需要异步处理）
	logrus.Infof("Starting deployment for application %s", app.ID)
	ctx := req.Context()
	containerRef, err := s.resMgr.Deploy(ctx, spec)
	if err != nil {
		logrus.Errorf("Failed to deploy application %s: %v", app.ID, err)
		// 更新应用状态为失败
		app.Status = application.StatusFailed
		logrus.Errorf("Application %s status updated to FAILED", app.ID)
		response.WriteError(w, http.StatusInternalServerError, "failed to deploy application", err)
		return
	}

	logrus.Infof("Container deployed successfully for application %s, containerRef: %s", app.ID, containerRef)
	app.ContainerRef = containerRef

	// 更新应用状态为运行中
	app.Status = application.StatusRunning
	logrus.Infof("Application %s status updated to RUNNING", app.ID)

	app.LastDeployed = time.Now()

	// 返回简单的成功响应
	responseData := map[string]interface{}{
		"message": "Application created successfully",
		"id":      app.ID,
	}
	if err := response.WriteSuccess(w, responseData); err != nil {
		logrus.Errorf("Failed to write create application response: %v", err)
		return
	}

	logrus.Infof("Successfully created and deployed application: %s (ID: %s, ContainerRef: %s)", createReq.Name, app.ID, containerRef)
}

func (s *Server) Start(addr string) error {
	logrus.Infof("Starting server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) Stop() {
	s.cancel()
}
