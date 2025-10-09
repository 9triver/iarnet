package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/discovery"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/server/request"
	"github.com/9triver/iarnet/internal/server/response"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Server struct {
	router  *mux.Router
	resMgr  *resource.Manager
	appMgr  *application.Manager
	peerMgr *discovery.PeerManager
	config  *config.Config
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewServer(rm *resource.Manager, am *application.Manager, pm *discovery.PeerManager, cfg *config.Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{router: mux.NewRouter(), resMgr: rm, appMgr: am, peerMgr: pm, config: cfg, ctx: ctx, cancel: cancel}
	// s.router.HandleFunc("/run", s.handleRun).Methods("POST")
	s.router.HandleFunc("/resource/capacity", s.handleResourceCapacity).Methods("GET")
	s.router.HandleFunc("/resource/providers", s.handleResourceProviders).Methods("GET")
	s.router.HandleFunc("/resource/providers", s.handleRegisterProvider).Methods("POST")
	s.router.HandleFunc("/resource/providers/{id}", s.handleUnregisterProvider).Methods("DELETE")

	s.router.HandleFunc("/application/apps", s.handleGetApplications).Methods("GET")
	s.router.HandleFunc("/application/apps", s.handleCreateApplication).Methods("POST")
	s.router.HandleFunc("/application/apps/{id}", s.handleGetApplicationById).Methods("GET")
	s.router.HandleFunc("/application/apps/{id}", s.handleUpdateApplication).Methods("PUT")
	s.router.HandleFunc("/application/apps/{id}", s.handleDeleteApplication).Methods("DELETE")
	s.router.HandleFunc("/application/apps/{id}/run", s.handleRunApplication).Methods("POST")
	s.router.HandleFunc("/application/apps/{id}/stop", s.handleStopApplication).Methods("POST")
	s.router.HandleFunc("/application/apps/{id}/logs", s.handleGetApplicationLogs).Methods("GET")
	s.router.HandleFunc("/application/apps/{id}/logs/parsed", s.handleGetApplicationLogsParsed).Methods("GET")
	s.router.HandleFunc("/application/stats", s.handleGetApplicationStats).Methods("GET")
	s.router.HandleFunc("/application/runner-environments", s.handleGetRunnerEnvironments).Methods("GET")
	// s.router.HandleFunc("/application/create", s.handleCreateApplication).Methods("POST")
	// s.router.HandleFunc("/application/apps/{id}/code-browser", s.handleStartCodeBrowser).Methods("POST")
	// s.router.HandleFunc("/application/apps/{id}/code-browser", s.handleStopCodeBrowser).Methods("DELETE")
	// s.router.HandleFunc("/application/apps/{id}/code-browser/status", s.handleGetCodeBrowserStatus).Methods("GET")
	s.router.HandleFunc("/application/apps/{id}/files", s.handleGetFileTree).Methods("GET")
	s.router.HandleFunc("/application/apps/{id}/files/content", s.handleGetFileContent).Methods("GET")
	s.router.HandleFunc("/application/apps/{id}/files/content", s.handleSaveFileContent).Methods("PUT")
	s.router.HandleFunc("/application/apps/{id}/files", s.handleCreateFile).Methods("POST")
	s.router.HandleFunc("/application/apps/{id}/files", s.handleDeleteFile).Methods("DELETE")
	s.router.HandleFunc("/application/apps/{id}/directories", s.handleCreateDirectory).Methods("POST")
	s.router.HandleFunc("/application/apps/{id}/directories", s.handleDeleteDirectory).Methods("DELETE")

	// 组件相关API
	s.router.HandleFunc("/application/apps/{id}/dag", s.handleGetApplicationDAG).Methods("GET")
	s.router.HandleFunc("/application/apps/{id}/analyze", s.handleAnalyzeApplication).Methods("POST")
	s.router.HandleFunc("/application/apps/{id}/deploy-components", s.handleDeployComponents).Methods("POST")
	s.router.HandleFunc("/application/apps/{id}/components/{componentId}/start", s.handleStartComponent).Methods("POST")
	s.router.HandleFunc("/application/apps/{id}/components/{componentId}/stop", s.handleStopComponent).Methods("POST")
	s.router.HandleFunc("/application/apps/{id}/components/{componentId}/status", s.handleGetComponentStatus).Methods("GET")
	s.router.HandleFunc("/application/apps/{id}/components/{componentId}/logs", s.handleGetComponentLogs).Methods("GET")
	s.router.HandleFunc("/application/apps/{id}/components/{componentId}/resource-usage", s.handleGetComponentResourceUsage).Methods("GET")
	s.router.HandleFunc("/application/components/resource-usage", s.handleGetAllComponentsResourceUsage).Methods("GET")

	// Peer管理相关API
	s.router.HandleFunc("/peer/nodes", s.handleGetPeerNodes).Methods("GET")
	s.router.HandleFunc("/peer/nodes", s.handleAddPeerNode).Methods("POST")
	s.router.HandleFunc("/peer/nodes/{id}", s.handleRemovePeerNode).Methods("DELETE")

	return s
}

func (s *Server) handleUpdateApplication(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]
	if id == "" {
		response.WriteError(w, http.StatusBadRequest, "application id is required", nil)
		return
	}
	var updateApp request.UpdateApplicationRequest
	if err := json.NewDecoder(req.Body).Decode(&updateApp); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	// 序列化为JSON以便于阅读
	updateJSON, _ := json.Marshal(updateApp)
	logrus.Infof("Updating application id:%s with %s", id, string(updateJSON))
	if err := s.appMgr.UpdateApplication(s.ctx, id, &updateApp); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to update application", err)
		return
	}
	response.Accepted("Application updated").WriteJSON(w)
}

func (s *Server) handleRun(w http.ResponseWriter, req *http.Request) {
	// var spec runner.ContainerSpec
	// if err := json.NewDecoder(req.Body).Decode(&spec); err != nil {
	// 	response.WriteError(w, http.StatusBadRequest, "invalid request body", err)
	// 	return
	// }
	// usageReq := resource.Usage{CPU: spec.CPU, Memory: spec.Memory, GPU: spec.GPU}
	// // if s.resMgr.CanAllocate(usageReq) == nil {
	// // 	response.ServiceUnavailable("Resource limit exceeded").WriteJSON(w)
	// // 	return
	// // }
	// if err := s.runner.Run(s.ctx, spec); err != nil {
	// 	response.WriteError(w, http.StatusInternalServerError, "failed to run container", err)
	// 	return
	// }
	// s.resMgr.Allocate(usageReq)
	// response.Accepted("Container run request accepted").WriteJSON(w)
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
		LocalProvider:          localProvider,
		ManagedProviders:       managedProviders,
		CollaborativeProviders: collaborativeProviders,
	}

	if err := response.WriteSuccess(w, getResourceProvidersResponse); err != nil {
		logrus.Errorf("Failed to write response: %v", err)
	}
}

// handleSaveFileContent 保存文件内容
func (s *Server) handleSaveFileContent(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	filePath := req.URL.Query().Get("path")

	if filePath == "" {
		response.WriteError(w, http.StatusBadRequest, "file path is required", nil)
		return
	}

	var saveReq request.SaveFileRequest
	if err := json.NewDecoder(req.Body).Decode(&saveReq); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	err := s.appMgr.SaveFileContent(appID, filePath, saveReq.Content)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to save file", err)
		return
	}

	saveResponse := response.SaveFileResponse{
		Message:  "File saved successfully",
		FilePath: filePath,
	}

	if err := response.WriteSuccess(w, saveResponse); err != nil {
		logrus.Errorf("Failed to write response: %v", err)
	}
}

// handleCreateFile 创建新文件
func (s *Server) handleCreateFile(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]

	var createReq request.CreateFileRequest
	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	err := s.appMgr.CreateFile(appID, createReq.FilePath)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create file", err)
		return
	}

	createResponse := response.CreateFileResponse{
		Message:  "File created successfully",
		FilePath: createReq.FilePath,
	}

	if err := response.WriteSuccess(w, createResponse); err != nil {
		logrus.Errorf("Failed to write response: %v", err)
	}
}

// handleDeleteFile 删除文件
func (s *Server) handleDeleteFile(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	filePath := req.URL.Query().Get("path")

	if filePath == "" {
		response.WriteError(w, http.StatusBadRequest, "file path is required", nil)
		return
	}

	err := s.appMgr.DeleteFile(appID, filePath)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete file", err)
		return
	}

	deleteResponse := response.DeleteFileResponse{
		Message:  "File deleted successfully",
		FilePath: filePath,
	}

	if err := response.WriteSuccess(w, deleteResponse); err != nil {
		logrus.Errorf("Failed to write response: %v", err)
	}
}

// handleCreateDirectory 创建新目录
func (s *Server) handleCreateDirectory(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]

	var createReq request.CreateDirectoryRequest
	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	err := s.appMgr.CreateDirectory(appID, createReq.DirectoryPath)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create directory", err)
		return
	}

	createResponse := response.CreateDirectoryResponse{
		Message:       "Directory created successfully",
		DirectoryPath: createReq.DirectoryPath,
	}

	if err := response.WriteSuccess(w, createResponse); err != nil {
		logrus.Errorf("Failed to write response: %v", err)
	}
}

// handleDeleteDirectory 删除目录
func (s *Server) handleDeleteDirectory(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	dirPath := req.URL.Query().Get("path")

	if dirPath == "" {
		response.WriteError(w, http.StatusBadRequest, "directory path is required", nil)
		return
	}

	err := s.appMgr.DeleteDirectory(appID, dirPath)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete directory", err)
		return
	}

	deleteResponse := response.DeleteDirectoryResponse{
		Message:       "Directory deleted successfully",
		DirectoryPath: dirPath,
	}

	if err := response.WriteSuccess(w, deleteResponse); err != nil {
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
			GitUrl:       app.GitUrl,
			Branch:       app.Branch,
			Type:         app.Type,
			Description:  app.Description,
			Ports:        app.Ports,
			HealthCheck:  app.HealthCheck,
			Status:       app.Status,
			LastDeployed: app.LastDeployed.Format("2006-01-02 15:04:05"),
			RunningOn:    app.GetRunningOn(),
			RunnerEnv:    app.RunnerEnv,
			ExecuteCmd:   app.ExecuteCmd,
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

	logrus.Infof("Creating application: name=%s", createReq.Name)

	// 验证必填字段
	if createReq.Name == "" {
		logrus.Warn("Application name is missing in create request")
		response.WriteError(w, http.StatusBadRequest, "application name is required", nil)
		return
	}

	// 验证Git导入字段
	logrus.Debugf("Validating git import fields")
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

	// 创建应用
	logrus.Infof("Creating application instance for: %s", createReq.Name)
	app, err := s.appMgr.CreateApplication(&createReq)
	if err != nil {
		logrus.Errorf("Failed to create application instance: %v", err)
		response.WriteError(w, http.StatusInternalServerError, "failed to create application instance", err)
		return
	}
	logrus.Infof("Application created with ID: %s", app.ID)

	// 应用创建成功，状态保持为未部署
	// 应用导入后不自动启动，等待用户手动启动
	logrus.Infof("Application %s imported successfully, status: %s", app.ID, app.Status)

	// 返回简单的成功响应
	responseData := map[string]interface{}{
		"message": "Application imported successfully",
		"id":      app.ID,
		"status":  string(app.Status),
	}
	if err := response.WriteSuccess(w, responseData); err != nil {
		logrus.Errorf("Failed to write create application response: %v", err)
		return
	}

	logrus.Infof("Successfully created application: %s (ID: %s)", createReq.Name, app.ID)
}

func (s *Server) Start(addr string) error {
	logrus.Infof("Starting server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) Stop() {
	s.cancel()
	s.appMgr.Stop()
	logrus.Info("Server stopped")
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
		GitUrl:       app.GitUrl,
		Branch:       app.Branch,
		Type:         app.Type,
		Description:  app.Description,
		Ports:        app.Ports,
		HealthCheck:  app.HealthCheck,
		Status:       app.Status,
		LastDeployed: app.LastDeployed.Format("2006-01-02 15:04:05"),
		RunningOn:    app.GetRunningOn(),
		RunnerEnv:    app.RunnerEnv,
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

	// 从Docker容器获取真实日志
	logs, err := s.appMgr.GetApplicationLogs(appID, lines)
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

// handleGetApplicationLogsParsed 处理获取应用解析后日志请求
func (s *Server) handleGetApplicationLogsParsed(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	logrus.Infof("Received request to get parsed logs for application ID: %s", appID)

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
		logrus.Warnf("Application not found for parsed logs request: %s", appID)
		response.WriteError(w, http.StatusNotFound, "application not found", err)
		return
	}

	// 从Docker容器获取解析后的日志
	logs, err := s.appMgr.GetApplicationLogsParsed(appID, lines)
	if err != nil {
		logrus.Errorf("Failed to get parsed logs for application %s: %v", appID, err)
		response.WriteError(w, http.StatusInternalServerError, "failed to get parsed logs", err)
		return
	}

	// 限制返回的日志行数
	if len(logs) > lines {
		logs = logs[len(logs)-lines:]
	}

	logsResponse := &response.GetApplicationLogsParsedResponse{
		ApplicationId:   appID,
		ApplicationName: app.Name,
		Logs:            logs,
		TotalLines:      len(logs),
		RequestedLines:  lines,
	}

	if err := response.WriteSuccess(w, logsResponse); err != nil {
		logrus.Errorf("Failed to write parsed logs response: %v", err)
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

// handleStartCodeBrowser 启动代码浏览器
func (s *Server) handleStartCodeBrowser(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]

	// 获取应用信息
	_, err := s.appMgr.GetApplication(appID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "Application not found", err)
		return
	}

	// 启动代码浏览器
	port, err := s.appMgr.StartCodeBrowser(appID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to start code browser", err)
		return
	}

	startResponse := response.StartCodeBrowserResponse{
		Message: "Code browser started successfully",
		Port:    port,
		URL:     fmt.Sprintf("http://localhost:%d", port),
	}

	if err := response.WriteSuccess(w, startResponse); err != nil {
		logrus.Errorf("Failed to write start code browser response: %v", err)
		return
	}
}

// handleStopCodeBrowser 停止代码浏览器
func (s *Server) handleStopCodeBrowser(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]

	// 停止代码浏览器
	err := s.appMgr.StopCodeBrowser(appID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to stop code browser", err)
		return
	}

	stopResponse := response.StopCodeBrowserResponse{
		Message: "Code browser stopped successfully",
	}

	if err := response.WriteSuccess(w, stopResponse); err != nil {
		logrus.Errorf("Failed to write stop code browser response: %v", err)
		return
	}
}

// handleGetCodeBrowserStatus 获取代码浏览器状态
func (s *Server) handleGetCodeBrowserStatus(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]

	// 获取代码浏览器状态
	status, err := s.appMgr.GetCodeBrowserStatus(appID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to get code browser status", err)
		return
	}

	if err := response.WriteSuccess(w, status); err != nil {
		logrus.Errorf("Failed to write code browser status response: %v", err)
		return
	}
}

// handleGetFileTree 获取文件树
func (s *Server) handleGetFileTree(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]

	// 获取查询参数
	path := req.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	// 通过Application Manager获取文件树
	fileTree, err := s.appMgr.GetFileTree(appID, path)

	// 转换为响应模型
	var fileTreeResp []response.FileInfo
	for _, file := range fileTree {
		fileTreeResp = append(fileTreeResp, response.FileInfo{
			Name:    file.Name,
			Path:    file.Path,
			IsDir:   file.IsDir,
			Size:    file.Size,
			ModTime: file.ModTime,
		})
	}

	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to get file tree", err)
		return
	}

	if err := response.WriteSuccess(w, response.GetFileTreeResponse{
		Files: fileTreeResp,
	}); err != nil {
		logrus.Errorf("Failed to write file tree response: %v", err)
		return
	}
}

// handleGetFileContent 获取文件内容
func (s *Server) handleGetFileContent(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]

	// 获取查询参数
	filePath := req.URL.Query().Get("path")
	if filePath == "" {
		response.WriteError(w, http.StatusBadRequest, "File path is required", nil)
		return
	}

	// 通过Application Manager获取文件内容
	content, language, err := s.appMgr.GetFileContent(appID, filePath)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to get file content", err)
		return
	}

	fileContent := map[string]interface{}{
		"content":  content,
		"language": language,
		"path":     filePath,
	}

	if err := response.WriteSuccess(w, fileContent); err != nil {
		logrus.Errorf("Failed to write file content response: %v", err)
		return
	}
}

// handleGetApplicationDAG 获取应用的DAG图
func (s *Server) handleGetApplicationDAG(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	logrus.Infof("Received request to get DAG for application: %s", appID)

	// 验证应用是否存在
	_, err := s.appMgr.GetApplication(appID)
	if err != nil {
		logrus.Warnf("Application not found: %s", appID)
		response.WriteError(w, http.StatusNotFound, "application not found", err)
		return
	}

	// 获取应用的DAG信息
	dag, err := s.appMgr.GetApplicationDAG(appID)
	if err != nil {
		logrus.Warnf("No DAG found for application: %s", appID)
		response.WriteError(w, http.StatusNotFound, "application DAG not found", err)
		return
	}

	if err := response.WriteSuccess(w, response.GetDAGResponse{
		DAG: dag,
	}); err != nil {
		logrus.Errorf("Failed to write DAG response: %v", err)
		return
	}
	logrus.Debugf("Successfully sent DAG for application: %s", appID)
}

// handleAnalyzeApplication 分析应用代码并生成Actor组件DAG
func (s *Server) handleAnalyzeApplication(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	logrus.Infof("Received request to analyze application: %s", appID)

	// 验证应用是否存在
	app, err := s.appMgr.GetApplication(appID)
	if err != nil {
		logrus.Warnf("Application not found: %s", appID)
		response.WriteError(w, http.StatusNotFound, "application not found", err)
		return
	}

	// 执行代码分析
	err = s.appMgr.AnalyzeAndDeployApplication(appID)
	if err != nil {
		logrus.Errorf("Failed to analyze application %s: %v", appID, err)
		response.WriteError(w, http.StatusInternalServerError, "failed to analyze application", err)
		return
	}

	// 更新应用状态
	s.appMgr.UpdateApplicationStatus(appID, "analyzed")

	analysisResult := map[string]interface{}{
		"message":     "Application analyzed successfully",
		"application": app.Name,
		"status":      "analyzed",
	}

	if err := response.WriteSuccess(w, analysisResult); err != nil {
		logrus.Errorf("Failed to write analysis response: %v", err)
		return
	}
	logrus.Debugf("Successfully analyzed application: %s", appID)
}

// handleDeployComponents 部署应用的所有Actor组件
func (s *Server) handleDeployComponents(w http.ResponseWriter, req *http.Request) {
	panic("unimplemented")
	// vars := mux.Vars(req)
	// appID := vars["id"]
	// logrus.Infof("Received request to deploy components for application: %s", appID)

	// // 验证应用是否存在
	// app, err := s.appMgr.GetApplication(appID)
	// if err != nil {
	// 	logrus.Warnf("Application not found: %s", appID)
	// 	response.WriteError(w, http.StatusNotFound, "application not found", err)
	// 	return
	// }

	// // 获取DAG信息
	// dag, err := s.appMgr.GetApplicationDAG(appID)
	// if err != nil {
	// 	logrus.Warnf("No DAG found for application: %s", appID)
	// 	response.WriteError(w, http.StatusNotFound, "application DAG not found, please analyze first", err)
	// 	return
	// }

	// // 部署所有组件
	// deployedComponents := []string{}
	// for _, component := range dag.Components {
	// 	err := s.deployComponent(component)
	// 	if err != nil {
	// 		logrus.Errorf("Failed to deploy component %s: %v", component.ID, err)
	// 		continue
	// 	}
	// 	deployedComponents = append(deployedComponents, component.ID)
	// }

	// // 更新应用状态
	// s.appMgr.UpdateApplicationStatus(appID, "deployed")

	// deployResult := map[string]interface{}{
	// 	"message":             "Components deployed successfully",
	// 	"application":         app.Name,
	// 	"total_components":    len(dag.Components),
	// 	"deployed_components": deployedComponents,
	// 	"failed_components":   len(dag.Components) - len(deployedComponents),
	// }

	// if err := response.WriteSuccess(w, deployResult); err != nil {
	// 	logrus.Errorf("Failed to write deploy response: %v", err)
	// 	return
	// }
	// logrus.Debugf("Successfully deployed components for application: %s", appID)
}

// deployComponent 部署单个组件的辅助函数
func (s *Server) deployComponent(component *application.Component) error {
	// // 创建容器规格
	// spec := runner.ContainerSpec{
	// 	Image:   component.Image,
	// 	CPU:     component.Resources.CPU,
	// 	Memory:  component.Resources.Memory,
	// 	GPU:     component.Resources.GPU,
	// 	EnvVars: component.EnvVars,
	// 	Ports:   component.Ports,
	// }

	// // 检查资源是否可用
	// usageReq := resource.Usage{
	// 	CPU:    component.Resources.CPU,
	// 	Memory: component.Resources.Memory,
	// 	GPU:    component.Resources.GPU,
	// }

	// if err := s.resMgr.CanAllocate(usageReq); err != nil {
	// 	return fmt.Errorf("insufficient resources: %v", err)
	// }

	// // // 运行容器
	// // if err := s.runner.Run(s.ctx, spec); err != nil {
	// // 	return fmt.Errorf("failed to run container: %v", err)
	// // }

	// // 分配资源
	// s.resMgr.Allocate(usageReq)

	// // 更新组件状态
	// component.Status = "running"
	// now := time.Now()
	// component.DeployedAt = &now

	return nil
}

// handleStartComponent 启动组件
func (s *Server) handleStartComponent(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	componentID := vars["componentId"]
	logrus.Infof("Received request to start component: %s in application: %s", componentID, appID)

	// TODO: 实现组件启动逻辑
	// 这里需要根据组件ID找到对应的组件并启动

	startResult := map[string]interface{}{
		"message":      "Component started successfully",
		"component_id": componentID,
		"status":       "running",
	}

	if err := response.WriteSuccess(w, startResult); err != nil {
		logrus.Errorf("Failed to write start component response: %v", err)
		return
	}
	logrus.Debugf("Successfully started component: %s", componentID)
}

// handleStopComponent 停止组件
func (s *Server) handleStopComponent(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	componentID := vars["componentId"]
	logrus.Infof("Received request to stop component: %s in application: %s", componentID, appID)

	// TODO: 实现组件停止逻辑
	// 这里需要根据组件ID找到对应的组件并停止

	stopResult := map[string]interface{}{
		"message":      "Component stopped successfully",
		"component_id": componentID,
		"status":       "stopped",
	}

	if err := response.WriteSuccess(w, stopResult); err != nil {
		logrus.Errorf("Failed to write stop component response: %v", err)
		return
	}
	logrus.Debugf("Successfully stopped component: %s", componentID)
}

// handleGetComponentStatus 获取组件状态
func (s *Server) handleGetComponentStatus(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	componentID := vars["componentId"]
	logrus.Infof("Received request to get status for component: %s in application: %s", componentID, appID)

	// TODO: 实现组件状态查询逻辑
	// 这里需要根据组件ID查询组件的实际状态

	componentStatus := map[string]interface{}{
		"component_id": componentID,
		"status":       "running",
		"cpu_usage":    "45%",
		"memory_usage": "512MB",
		"uptime":       "2h 30m",
		"last_updated": time.Now().Format("2006-01-02 15:04:05"),
	}

	if err := response.WriteSuccess(w, componentStatus); err != nil {
		logrus.Errorf("Failed to write component status response: %v", err)
		return
	}
	logrus.Debugf("Successfully retrieved status for component: %s", componentID)
}

// handleGetComponentLogs 获取组件日志
func (s *Server) handleGetComponentLogs(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	componentID := vars["componentId"]
	logrus.Infof("Received request to get logs for component: %s in application: %s", componentID, appID)

	// 获取查询参数
	linesParam := req.URL.Query().Get("lines")
	lines := 100 // 默认返回100行
	if linesParam != "" {
		if parsedLines, err := strconv.Atoi(linesParam); err == nil && parsedLines > 0 {
			lines = parsedLines
		}
	}

	// TODO: 实现组件日志获取逻辑
	// 这里需要根据组件ID获取组件的实际日志

	// 模拟日志数据
	mockLogs := []string{
		"2024-01-15 10:00:01 [INFO] Component started successfully",
		"2024-01-15 10:00:02 [INFO] Initializing component services",
		"2024-01-15 10:00:03 [INFO] Component is ready to serve requests",
		"2024-01-15 10:00:04 [DEBUG] Processing request from upstream component",
		"2024-01-15 10:00:05 [INFO] Request processed successfully",
	}

	// 限制返回的日志行数
	if len(mockLogs) > lines {
		mockLogs = mockLogs[len(mockLogs)-lines:]
	}

	logsResponse := map[string]interface{}{
		"component_id":    componentID,
		"logs":            mockLogs,
		"total_lines":     len(mockLogs),
		"requested_lines": lines,
	}

	if err := response.WriteSuccess(w, logsResponse); err != nil {
		logrus.Errorf("Failed to write component logs response: %v", err)
		return
	}
	logrus.Debugf("Successfully retrieved logs for component: %s", componentID)
}

// handleDeleteApplication 处理删除应用请求
func (s *Server) handleDeleteApplication(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	logrus.Infof("Received request to delete application ID: %s", appID)

	// 验证应用是否存在
	_, err := s.appMgr.GetApplication(appID)
	if err != nil {
		logrus.Warnf("Application not found for deletion request: %s", appID)
		response.WriteError(w, http.StatusNotFound, "application not found", err)
		return
	}

	// 删除应用
	if err := s.appMgr.DeleteApplication(appID); err != nil {
		logrus.Errorf("Failed to delete application %s: %v", appID, err)
		response.WriteError(w, http.StatusInternalServerError, "failed to delete application", err)
		return
	}

	// 返回成功响应
	deleteResponse := map[string]interface{}{
		"message": "Application deleted successfully",
		"app_id":  appID,
	}

	if err := response.WriteSuccess(w, deleteResponse); err != nil {
		logrus.Errorf("Failed to write delete response: %v", err)
		return
	}
	logrus.Infof("Successfully deleted application: %s", appID)
}

// handleGetComponentResourceUsage 获取单个组件的实时资源使用状态
func (s *Server) handleGetComponentResourceUsage(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	componentID := vars["componentId"]
	logrus.Infof("Received request to get resource usage for component: %s in application: %s", componentID, appID)

	// TODO: 实现组件实时资源使用状态查询逻辑
	// 这里需要根据组件ID查询组件的实际资源使用情况

	resourceUsage := map[string]interface{}{
		"component_id":   componentID,
		"application_id": appID,
		"timestamp":      time.Now().Unix(),
		"cpu": map[string]interface{}{
			"usage_percent": 45.2,
			"cores_used":    1.8,
			"total_cores":   4,
		},
		"memory": map[string]interface{}{
			"usage_bytes":   536870912, // 512MB
			"usage_percent": 25.6,
			"total_bytes":   2147483648, // 2GB
		},
		"network": map[string]interface{}{
			"bytes_in":    1048576, // 1MB
			"bytes_out":   2097152, // 2MB
			"packets_in":  1024,
			"packets_out": 2048,
		},
		"disk": map[string]interface{}{
			"read_bytes":    10485760,   // 10MB
			"write_bytes":   5242880,    // 5MB
			"usage_bytes":   1073741824, // 1GB
			"usage_percent": 10.5,
		},
		"status":         "running",
		"uptime_seconds": 9000, // 2.5 hours
	}

	if err := response.WriteSuccess(w, resourceUsage); err != nil {
		logrus.Errorf("Failed to write component resource usage response: %v", err)
		return
	}
	logrus.Debugf("Successfully retrieved resource usage for component: %s", componentID)
}

// handleGetAllComponentsResourceUsage 获取所有组件的实时资源使用状态
func (s *Server) handleGetAllComponentsResourceUsage(w http.ResponseWriter, req *http.Request) {
	logrus.Info("Received request to get resource usage for all components")

	// TODO: 实现所有组件实时资源使用状态查询逻辑
	// 这里需要查询所有运行中组件的实际资源使用情况

	// 模拟多个组件的资源使用数据
	allComponentsUsage := map[string]interface{}{
		"timestamp":        time.Now().Unix(),
		"total_components": 3,
		"components": []map[string]interface{}{
			{
				"component_id":   "comp-001",
				"application_id": "app-001",
				"name":           "web-server",
				"cpu": map[string]interface{}{
					"usage_percent": 45.2,
					"cores_used":    1.8,
					"total_cores":   4,
				},
				"memory": map[string]interface{}{
					"usage_bytes":   536870912,
					"usage_percent": 25.6,
					"total_bytes":   2147483648,
				},
				"status": "running",
			},
			{
				"component_id":   "comp-002",
				"application_id": "app-001",
				"name":           "database",
				"cpu": map[string]interface{}{
					"usage_percent": 23.8,
					"cores_used":    0.95,
					"total_cores":   4,
				},
				"memory": map[string]interface{}{
					"usage_bytes":   1073741824,
					"usage_percent": 51.2,
					"total_bytes":   2147483648,
				},
				"status": "running",
			},
			{
				"component_id":   "comp-003",
				"application_id": "app-002",
				"name":           "api-gateway",
				"cpu": map[string]interface{}{
					"usage_percent": 12.5,
					"cores_used":    0.5,
					"total_cores":   4,
				},
				"memory": map[string]interface{}{
					"usage_bytes":   268435456,
					"usage_percent": 12.8,
					"total_bytes":   2147483648,
				},
				"status": "running",
			},
		},
		"summary": map[string]interface{}{
			"total_cpu_usage_percent":    27.2,
			"total_memory_usage_bytes":   1879048192,
			"total_memory_usage_percent": 29.9,
			"running_components":         3,
			"stopped_components":         0,
		},
	}

	if err := response.WriteSuccess(w, allComponentsUsage); err != nil {
		logrus.Errorf("Failed to write all components resource usage response: %v", err)
		return
	}
	logrus.Debug("Successfully retrieved resource usage for all components")
}

// handleGetPeerNodes 处理获取peer节点列表请求
func (s *Server) handleGetPeerNodes(w http.ResponseWriter, req *http.Request) {
	logrus.Info("Received request to get peer nodes")

	// 获取所有已知的peer节点
	peers := s.peerMgr.GetPeers()
	var nodes []response.PeerNodeInfo

	for _, peerAddr := range peers {
		// 简单的状态检查，这里可以根据需要实现更复杂的健康检查
		status := "unknown"
		nodes = append(nodes, response.PeerNodeInfo{
			Address: peerAddr,
			Status:  status,
		})
	}

	getPeerNodesResponse := response.GetPeerNodesResponse{
		Nodes: nodes,
		Total: len(nodes),
	}

	if err := response.WriteSuccess(w, getPeerNodesResponse); err != nil {
		logrus.Errorf("Failed to write peer nodes response: %v", err)
		return
	}
	logrus.Debug("Successfully sent peer nodes response")
}

// handleAddPeerNode 处理添加peer节点请求
func (s *Server) handleAddPeerNode(w http.ResponseWriter, req *http.Request) {
	logrus.Info("Received request to add peer node")

	var addReq request.AddPeerNodeRequest
	if err := json.NewDecoder(req.Body).Decode(&addReq); err != nil {
		logrus.Errorf("Failed to decode add peer node request: %v", err)
		response.WriteError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// 验证地址格式
	if addReq.Address == "" {
		logrus.Error("Peer address is required")
		response.WriteError(w, http.StatusBadRequest, "peer address is required", nil)
		return
	}

	// 添加peer节点
	s.peerMgr.AddPeers([]string{addReq.Address})
	logrus.Infof("Added peer node: %s", addReq.Address)

	addPeerResponse := response.AddPeerNodeResponse{
		Message: "Peer node added successfully",
		Address: addReq.Address,
	}

	if err := response.WriteSuccess(w, addPeerResponse); err != nil {
		logrus.Errorf("Failed to write add peer node response: %v", err)
		return
	}
	logrus.Debug("Successfully sent add peer node response")
}

// handleRemovePeerNode 处理删除peer节点请求
func (s *Server) handleRemovePeerNode(w http.ResponseWriter, req *http.Request) {
	logrus.Info("Received request to remove peer node")

	// 从URL路径中获取peer节点ID（地址）
	vars := mux.Vars(req)
	peerAddr := vars["id"]

	if peerAddr == "" {
		logrus.Error("Peer address is required")
		response.WriteError(w, http.StatusBadRequest, "peer address is required", nil)
		return
	}

	// 删除peer节点
	removed := s.peerMgr.RemovePeer(peerAddr)
	if !removed {
		logrus.Warnf("Peer node not found: %s", peerAddr)
		response.WriteError(w, http.StatusNotFound, "peer node not found", nil)
		return
	}

	logrus.Infof("Removed peer node: %s", peerAddr)

	removePeerResponse := response.RemovePeerNodeResponse{
		Message: "Peer node removed successfully",
		Address: peerAddr,
	}

	if err := response.WriteSuccess(w, removePeerResponse); err != nil {
		logrus.Errorf("Failed to write remove peer node response: %v", err)
		return
	}
	logrus.Debug("Successfully sent remove peer node response")
}

func (s *Server) handleGetRunnerEnvironments(w http.ResponseWriter, req *http.Request) {
	logrus.Debug("Received get runner environments request")

	// 从配置中获取运行环境信息
	runnerEnvs := make([]response.RunnerEnvironment, 0)

	for name := range s.config.RunnerImages {
		runnerEnvs = append(runnerEnvs, response.RunnerEnvironment{
			Name: name,
		})
	}

	runnerEnvsResponse := map[string]interface{}{
		"environments": runnerEnvs,
	}

	if err := response.WriteSuccess(w, runnerEnvsResponse); err != nil {
		logrus.Errorf("Failed to write runner environments response: %v", err)
		return
	}
	logrus.Debug("Successfully sent runner environments response")
}

// handleRunApplication 处理运行应用请求
func (s *Server) handleRunApplication(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	logrus.Infof("Received request to run application: %s", appID)

	// 更新应用状态为部署中
	err := s.appMgr.UpdateApplicationStatus(appID, application.StatusDeploying)
	if err != nil {
		logrus.Errorf("Failed to update application status: %v", err)
		response.WriteError(w, http.StatusNotFound, "application not found", err)
		return
	}

	// 运行应用
	err = s.appMgr.RunApplication(appID)
	if err != nil {
		logrus.Errorf("Failed to run application %s: %v", appID, err)
		// 更新状态为失败
		s.appMgr.UpdateApplicationStatus(appID, application.StatusFailed)
		response.WriteError(w, http.StatusInternalServerError, "failed to run application", err)
		return
	}

	// 更新应用状态为运行中
	err = s.appMgr.UpdateApplicationStatus(appID, application.StatusRunning)
	if err != nil {
		logrus.Errorf("Failed to update application status to running: %v", err)
	}

	// 获取更新后的应用信息
	app, err := s.appMgr.GetApplication(appID)
	if err != nil {
		logrus.Errorf("Failed to get application after running: %v", err)
		response.WriteError(w, http.StatusInternalServerError, "failed to get application", err)
		return
	}

	appInfo := response.ApplicationInfo{
		ID:           app.ID,
		Name:         app.Name,
		GitUrl:       app.GitUrl,
		Branch:       app.Branch,
		Type:         app.Type,
		Description:  app.Description,
		Ports:        app.Ports,
		HealthCheck:  app.HealthCheck,
		Status:       app.Status,
		LastDeployed: app.LastDeployed.Format("2006-01-02 15:04:05"),
		RunningOn:    app.GetRunningOn(),
		RunnerEnv:    app.RunnerEnv,
	}

	if err := response.WriteSuccess(w, appInfo); err != nil {
		logrus.Errorf("Failed to write run application response: %v", err)
		return
	}
	logrus.Infof("Successfully started application: %s", appID)
}

// handleStopApplication 处理停止应用请求
func (s *Server) handleStopApplication(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	appID := vars["id"]
	logrus.Infof("Received request to stop application: %s", appID)

	// 停止应用
	err := s.appMgr.StopApplication(appID)
	if err != nil {
		logrus.Errorf("Failed to stop application %s: %v", appID, err)
		response.WriteError(w, http.StatusInternalServerError, "failed to stop application", err)
		return
	}

	// 更新应用状态为已停止
	err = s.appMgr.UpdateApplicationStatus(appID, application.StatusStopped)
	if err != nil {
		logrus.Errorf("Failed to update application status to stopped: %v", err)
	}

	// 获取更新后的应用信息
	app, err := s.appMgr.GetApplication(appID)
	if err != nil {
		logrus.Errorf("Failed to get application after stopping: %v", err)
		response.WriteError(w, http.StatusInternalServerError, "failed to get application", err)
		return
	}

	appInfo := response.ApplicationInfo{
		ID:           app.ID,
		Name:         app.Name,
		GitUrl:       app.GitUrl,
		Branch:       app.Branch,
		Type:         app.Type,
		Description:  app.Description,
		Ports:        app.Ports,
		HealthCheck:  app.HealthCheck,
		Status:       app.Status,
		LastDeployed: app.LastDeployed.Format("2006-01-02 15:04:05"),
		RunningOn:    app.GetRunningOn(),
		RunnerEnv:    app.RunnerEnv,
	}

	if err := response.WriteSuccess(w, appInfo); err != nil {
		logrus.Errorf("Failed to write stop application response: %v", err)
		return
	}
	logrus.Infof("Successfully stopped application: %s", appID)
}
