package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/runner"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Server struct {
	router *mux.Router
	runner runner.Runner
	resMgr *resource.ResourceManager
	ctx    context.Context
	cancel context.CancelFunc
}

func NewServer(r runner.Runner, rm *resource.ResourceManager) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{router: mux.NewRouter(), runner: r, resMgr: rm, ctx: ctx, cancel: cancel}
	s.router.HandleFunc("/run", s.handleRun).Methods("POST")
	return s
}

func (s *Server) handleRun(w http.ResponseWriter, req *http.Request) {
	var spec runner.ContainerSpec
	if err := json.NewDecoder(req.Body).Decode(&spec); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	usageReq := resource.ResourceUsage{CPU: spec.CPU, Memory: spec.Memory, GPU: spec.GPU}
	if !s.resMgr.CanAllocate(usageReq) {
		http.Error(w, "Resource limit exceeded", http.StatusServiceUnavailable)
		return
	}
	if err := s.runner.Run(s.ctx, spec); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.resMgr.Allocate(usageReq)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) Start(addr string) error {
	logrus.Infof("Starting server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) Stop() {
	s.cancel()
}
