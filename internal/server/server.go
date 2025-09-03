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
	resMgr *resource.Manager
	ctx    context.Context
	cancel context.CancelFunc
}

func NewServer(r runner.Runner, rm *resource.Manager) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{router: mux.NewRouter(), runner: r, resMgr: rm, ctx: ctx, cancel: cancel}
	s.router.HandleFunc("/run", s.handleRun).Methods("POST")
	s.router.HandleFunc("/capacity", s.handleCapacity).Methods("GET")
	return s
}

func (s *Server) handleRun(w http.ResponseWriter, req *http.Request) {
	var spec runner.ContainerSpec
	if err := json.NewDecoder(req.Body).Decode(&spec); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	usageReq := resource.Usage{CPU: spec.CPU, Memory: spec.Memory, GPU: spec.GPU}
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

func (s *Server) handleCapacity(w http.ResponseWriter, req *http.Request) {
	capacity, err := s.resMgr.GetCapacity(s.ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(capacity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) Start(addr string) error {
	logrus.Infof("Starting server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) Stop() {
	s.cancel()
}
