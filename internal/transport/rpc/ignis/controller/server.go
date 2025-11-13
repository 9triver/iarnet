package controller

import (
	"github.com/9triver/iarnet/internal/domain/ignis/controller"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
)

type Server struct {
	ctrlpb.UnimplementedServiceServer
	manager controller.Manager
}

func NewServer(manager controller.Manager) *Server {
	return &Server{manager: manager}
}

func (s *Server) Session(stream ctrlpb.Service_SessionServer) error {
	ctx := stream.Context()
	return s.manager.HandleSession(ctx, stream.Recv, stream.Send)
}
