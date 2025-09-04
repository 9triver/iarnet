package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/runner"
	"github.com/9triver/iarnet/internal/server/response"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Server struct {
	router *mux.Router
	runner runner.Runner
	resMgr *resource.Manager
	ctx    context.Context
	cancel context.CancelFunc
}

func NewServer(r runner.Runner, rm *resource.Manager) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{router: mux.NewRouter(), runner: r, resMgr: rm, ctx: ctx, cancel: cancel}
	s.router.HandleFunc("/run", s.handleRun).Methods("POST")
	s.router.HandleFunc("/resource/capacity", s.handleResourceCapacity).Methods("GET")
	s.router.HandleFunc("/resource/providers", s.handleResourceProviders).Methods("GET")
	s.router.HandleFunc("/application/overview", s.handleApplicationOverview).Methods("GET")
	return s
}

func (s *Server) handleRun(w http.ResponseWriter, req *http.Request) {
	var spec runner.ContainerSpec
	if err := json.NewDecoder(req.Body).Decode(&spec); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	usageReq := resource.Usage{CPU: spec.CPU, Memory: spec.Memory, GPU: spec.GPU}
	if !s.resMgr.CanAllocate(usageReq) {
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

func (s *Server) handleApplicationOverview(w http.ResponseWriter, req *http.Request) {
	overview := s.runner.GetOverview()
	if err := response.WriteSuccess(w, overview); err != nil {
		logrus.Errorf("Failed to write response: %v", err)
	}
}

func (s *Server) Start(addr string) error {
	logrus.Infof("Starting server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) Stop() {
	s.cancel()
}
