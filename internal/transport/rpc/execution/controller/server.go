package controller

import (
	"github.com/9triver/iarnet/internal/domain/execution/controller"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
)

type Server struct {
	ctrlpb.UnimplementedServiceServer
	controllerService controller.Service
}

func NewServer(controllerService controller.Service) *Server {
	return &Server{controllerService: controllerService}
}

func (s *Server) Session(stream ctrlpb.Service_SessionServer) error {
	ctx := stream.Context()
	return s.controllerService.HandleSession(ctx, stream.Recv, stream.Send)
}
