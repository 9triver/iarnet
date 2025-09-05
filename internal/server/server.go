package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
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
	s.router.HandleFunc("/resource/providers", s.handleRegisterProvider).Methods("POST")
	s.router.HandleFunc("/resource/providers/{id}", s.handleUnregisterProvider).Methods("DELETE")

	s.router.HandleFunc("/application/apps", s.handleGetApplications).Methods("GET")
	s.router.HandleFunc("/application/apps/{id}", s.handleGetApplicationById).Methods("GET")
	s.router.HandleFunc("/application/apps/{id}/logs", s.handleGetApplicationLogs).Methods("GET")
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
	categorizedProviders := s.resMgr.GetProviders()

	var localProvider *response.ResourceProviderInfo
	managedProviders := []response.ResourceProviderInfo{}
	collaborativeProviders := []response.ResourceProviderInfo{}

	// 处理本机 provider（单个对象）
	if categorizedProviders.LocalProvider != nil {
		capacity, err := categorizedProviders.LocalProvider.GetCapacity(s.ctx)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "failed to get resource capacity", err)
			return
		}

		localProvider = &response.ResourceProviderInfo{
			ID:     categorizedProviders.LocalProvider.GetID(),
			Name:   categorizedProviders.LocalProvider.GetName(),
			Host:   "localhost",
			Type:   categorizedProviders.LocalProvider.GetType(),
			Status: categorizedProviders.LocalProvider.GetStatus(),
			CPUUsage: response.UsageInfo{
				Used:  capacity.Used.CPU,
				Total: capacity.Total.CPU,
			},
			MemoryUsage: response.UsageInfo{
				Used:  capacity.Used.Memory,
				Total: capacity.Total.Memory,
			},
			LastUpdateTime: categorizedProviders.LocalProvider.GetLastUpdateTime().Format("2006-01-02 15:04:05"),
		}
	}

	// 处理托管 providers
	for _, provider := range categorizedProviders.ManagedProviders {
		capacity, err := provider.GetCapacity(s.ctx)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "failed to get resource capacity", err)
			return
		}

		providerInfo := response.ResourceProviderInfo{
			ID:     provider.GetID(),
			Name:   provider.GetName(),
			Host:   provider.GetHost(),
			Port:   provider.GetPort(),
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
		managedProviders = append(managedProviders, providerInfo)
	}

	// 处理协作 providers
	for _, provider := range categorizedProviders.CollaborativeProviders {
		capacity, err := provider.GetCapacity(s.ctx)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "failed to get resource capacity", err)
			return
		}

		providerInfo := response.ResourceProviderInfo{
			ID:     provider.GetID(),
			Name:   provider.GetName(),
			Host:   provider.GetHost(),
			Port:   provider.GetPort(),
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
		collaborativeProviders = append(collaborativeProviders, providerInfo)
	}

	getResourceProvidersResponse := response.GetResourceProvidersResponse{
		LocalProvider:         localProvider,
		ManagedProviders:      managedProviders,
		CollaborativeProviders: collaborativeProviders,
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

// handleGetApplicationById 处理获取单个应用详情请求
func (s *Server) handleGetApplicationById(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	logrus.Infof("Received request to get application by ID: %s", appID)

	app, err := s.appMgr.GetApplication(appID)
	if err != nil {
		logrus.Warnf("Application not found: %s", appID)
		response.WriteError(w, http.StatusNotFound, "application not found", err)
		return
	}

	appInfo := response.ApplicationInfo{
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
	}

	if err := response.WriteSuccess(w, appInfo); err != nil {
		logrus.Errorf("Failed to write application response: %v", err)
		return
	}
	logrus.Debugf("Successfully sent application details for ID: %s", appID)
}

// handleGetApplicationLogs 处理获取应用日志请求
func (s *Server) handleGetApplicationLogs(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	logrus.Infof("Received request to get logs for application ID: %s", appID)

	// 获取查询参数
	linesParam := req.URL.Query().Get("lines")
	lines := 100 // 默认返回100行
	if linesParam != "" {
		if parsedLines, err := strconv.Atoi(linesParam); err == nil && parsedLines > 0 {
			lines = parsedLines
		}
	}

	// 验证应用是否存在
	app, err := s.appMgr.GetApplication(appID)
	if err != nil {
		logrus.Warnf("Application not found for logs request: %s", appID)
		response.WriteError(w, http.StatusNotFound, "application not found", err)
		return
	}

	// 模拟日志数据（实际应该从容器运行时获取）
	logs, err := app.GetLogs(lines)
	if err != nil {
		logrus.Errorf("Failed to get logs for application %s: %v", appID, err)
		response.WriteError(w, http.StatusInternalServerError, "failed to get logs", err)
		return
	}

	// 限制返回的日志行数
	if len(logs) > lines {
		logs = logs[len(logs)-lines:]
	}

	logsResponse := &response.GetApplicationLogsResponse{
		ApplicationId:   appID,
		ApplicationName: app.Name,
		Logs:            logs,
		TotalLines:      len(logs),
		RequestedLines:  lines,
	}

	if err := response.WriteSuccess(w, logsResponse); err != nil {
		logrus.Errorf("Failed to write logs response: %v", err)
		return
	}
}

// handleRegisterProvider 处理注册资源提供者请求
func (s *Server) handleRegisterProvider(w http.ResponseWriter, req *http.Request) {
	var registerReq request.RegisterProviderRequest
	if err := json.NewDecoder(req.Body).Decode(&registerReq); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// 验证provider类型
	if registerReq.Type != "docker" && registerReq.Type != "k8s" {
		response.WriteError(w, http.StatusBadRequest, "unsupported provider type", nil)
		return
	}

	// 根据类型转换配置
	var config interface{}
	switch registerReq.Type {
	case "docker":
		// 将config转换为DockerConfig
		configBytes, err := json.Marshal(registerReq.Config)
		if err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid docker config", err)
			return
		}
		var dockerConfig resource.DockerConfig
		if err := json.Unmarshal(configBytes, &dockerConfig); err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid docker config format", err)
			return
		}
		config = dockerConfig
	case "k8s":
		// 将config转换为K8sConfig
		configBytes, err := json.Marshal(registerReq.Config)
		if err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid k8s config", err)
			return
		}
		var k8sConfig resource.K8sConfig
		if err := json.Unmarshal(configBytes, &k8sConfig); err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid k8s config format", err)
			return
		}
		config = k8sConfig
	}

	// 注册provider
	var providerType resource.ProviderType
	switch registerReq.Type {
	case "docker":
		providerType = resource.ProviderTypeDocker
	case "k8s":
		providerType = resource.ProviderTypeK8s
	}
	providerID, err := s.resMgr.RegisterProvider(providerType, registerReq.Name, config)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to register provider", err)
		return
	}

	// 返回成功响应
	registerResp := response.RegisterProviderResponse{
		ProviderID: providerID,
		Message:    "Provider registered successfully",
	}

	if err := response.WriteSuccess(w, registerResp); err != nil {
		logrus.Errorf("Failed to write register provider response: %v", err)
	}
}

// handleUnregisterProvider 处理注销资源提供者请求
func (s *Server) handleUnregisterProvider(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	providerID := vars["id"]

	if providerID == "" {
		response.WriteError(w, http.StatusBadRequest, "provider ID is required", nil)
		return
	}

	// 检查provider是否存在
	_, err := s.resMgr.GetProvider(providerID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "provider not found", err)
		return
	}

	// 注销provider
	s.resMgr.UnregisterProvider(providerID)

	// 返回成功响应
	unregisterResp := response.UnregisterProviderResponse{
		Message: "Provider unregistered successfully",
	}

	if err := response.WriteSuccess(w, unregisterResp); err != nil {
		logrus.Errorf("Failed to write unregister provider response: %v", err)
	}
}
