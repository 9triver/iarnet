package rpc

import (
	"context"
	"net"
	"sync"

	"github.com/9triver/iarnet/internal/ignis/transport"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	"google.golang.org/grpc"
)

type Server struct {
	addr    string
	srv     *grpc.Server
	handler transport.SessionHandler
	mu      sync.RWMutex
	ctrlpb.UnimplementedServiceServer
}

func NewServer(addr string) *Server {
	return &Server{addr: addr}
}

func (s *Server) OnSession(h transport.SessionHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = h
}

func (s *Server) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.srv = grpc.NewServer()
	ctrlpb.RegisterServiceServer(s.srv, s)
	go func() { _ = s.srv.Serve(lis) }()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.srv != nil {
		s.srv.GracefulStop()
	}
	return nil
}

// 实现 ctrlpb.ServiceServer 接口
func (s *Server) Session(stream grpc.BidiStreamingServer[ctrlpb.Message, ctrlpb.Message]) error {
	s.mu.RLock()
	h := s.handler
	s.mu.RUnlock()
	if h == nil {
		return nil
	}
	return h(&grpcStream{stream})
}

// grpcStream 适配器，实现 transport.SessionStream
type grpcStream struct {
	grpc.BidiStreamingServer[ctrlpb.Message, ctrlpb.Message]
}

func (gs *grpcStream) Context() context.Context       { return gs.BidiStreamingServer.Context() }
func (gs *grpcStream) Send(m *ctrlpb.Message) error   { return gs.BidiStreamingServer.Send(m) }
func (gs *grpcStream) Recv() (*ctrlpb.Message, error) { return gs.BidiStreamingServer.Recv() }
