package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

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
	if s.resMgr.CanAllocate(usageReq) == nil {
		response.ServiceUnavailable("Resource limit exceeded").WriteJSON(w)
		return
	}
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

// handleGetApplicationStats 处理应用统计信息请求
func (s *Server) handleGetApplicationStats(w http.ResponseWriter, req *http.Request) {
	stats := s.appMgr.GetApplicationStats()
	if err := response.WriteSuccess(w, stats); err != nil {
		logrus.Errorf("Failed to write application stats response: %v", err)
	}
}

func (s *Server) handleCreateApplication(w http.ResponseWriter, req *http.Request) {
	var createReq request.CreateApplicationRequest
	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// 验证必填字段
	if createReq.Name == "" {
		response.WriteError(w, http.StatusBadRequest, "application name is required", nil)
		return
	}

	if createReq.ImportType != "git" && createReq.ImportType != "docker" {
		response.WriteError(w, http.StatusBadRequest, "importType must be 'git' or 'docker'", nil)
		return
	}

	// 根据导入类型验证相应字段
	switch createReq.ImportType {
	case "git":
		if createReq.GitUrl == nil || *createReq.GitUrl == "" {
			response.WriteError(w, http.StatusBadRequest, "gitUrl is required for git import", nil)
			return
		}
		if createReq.Branch == nil || *createReq.Branch == "" {
			defaultBranch := "main"
			createReq.Branch = &defaultBranch
		}
	case "docker":
		if createReq.DockerImage == nil || *createReq.DockerImage == "" {
			response.WriteError(w, http.StatusBadRequest, "dockerImage is required for docker import", nil)
			return
		}
		if createReq.DockerTag == nil || *createReq.DockerTag == "" {
			defaultTag := "latest"
			createReq.DockerTag = &defaultTag
		}
	}

	// 创建应用
	app := s.appMgr.CreateApplication(createReq.Name)

	// 构建容器规格
	var spec resource.ContainerSpec
	if createReq.ImportType == "docker" {
		// Docker 导入方式直接使用镜像
		spec.Image = *createReq.DockerImage + ":" + *createReq.DockerTag
	} else {
		// Git 导入方式需要构建镜像（这里暂时使用占位符，实际需要实现 Git 构建逻辑）
		spec.Image = "placeholder:" + app.ID // 实际应该是构建后的镜像名
	}

	// 设置默认资源限制
	spec.CPU = 1.0     // 1 CPU 核心
	spec.Memory = 1024 // 1GB 内存
	spec.GPU = 0       // 默认不使用 GPU

	// 如果指定了端口，添加到命令中（这里简化处理）
	if createReq.Port != nil {
		spec.Command = []string{"sh", "-c", "echo 'Application started on port " + strconv.Itoa(*createReq.Port) + "'"}
	}

	// 尝试运行容器（这里可能需要异步处理）
	ctx := req.Context()
	containerRef, err := s.resMgr.Deploy(ctx, spec)
	if err != nil {
		logrus.Errorf("Failed to run application %s: %v", app.ID, err)
		// 更新应用状态为失败
		app.Status = application.StatusFailed
		response.WriteError(w, http.StatusInternalServerError, "failed to deploy application", err)
		return
	}

	app.ContainerRef = containerRef

	// 更新应用状态为运行中
	app.Status = application.StatusRunning

	// 返回简单的成功响应
	if err := response.WriteSuccess(w, map[string]interface{}{
		"message": "Application created successfully",
		"id":      app.ID,
	}); err != nil {
		logrus.Errorf("Failed to write create application response: %v", err)
	}

	logrus.Infof("Successfully created and deployed application: %s (ID: %s)", createReq.Name, app.ID)
}

func (s *Server) Start(addr string) error {
	logrus.Infof("Starting server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) Stop() {
	s.cancel()
}
