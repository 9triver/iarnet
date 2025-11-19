package store

import (
	"context"
	"fmt"

	domainstore "github.com/9triver/iarnet/internal/domain/resource/store"
	storepb "github.com/9triver/iarnet/internal/proto/resource/store"
)

type Server struct {
	storepb.UnimplementedServiceServer
	svc domainstore.Service
}

func NewServer(svc domainstore.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) SaveObject(ctx context.Context, req *storepb.SaveObjectRequest) (*storepb.SaveObjectResponse, error) {
	ref, err := s.svc.SaveObject(ctx, req.Object)
	if err != nil {
		return nil, err
	}
	return &storepb.SaveObjectResponse{
		ObjectRef: ref,
		Success:   true,
		Error:     "",
	}, nil
}

func (s *Server) GetObject(ctx context.Context, req *storepb.GetObjectRequest) (*storepb.GetObjectResponse, error) {
	obj, err := s.svc.GetObject(ctx, req.ObjectRef)
	if err != nil {
		return nil, err
	}
	return &storepb.GetObjectResponse{Object: obj}, nil
}

func (s *Server) GetStreamChunk(ctx context.Context, req *storepb.GetStreamChunkRequest) (*storepb.GetStreamChunkResponse, error) {
	chunk, err := s.svc.GetStreamChunk(ctx, req.ObjectID, req.Offset)
	if err != nil {
		return nil, err
	}
	return &storepb.GetStreamChunkResponse{Chunk: chunk}, nil
}

func (s *Server) SaveStreamChunk(ctx context.Context, req *storepb.SaveStreamChunkRequest) (*storepb.SaveStreamChunkResponse, error) {
	if req == nil || req.Chunk == nil {
		return nil, fmt.Errorf("chunk is required")
	}
	if err := s.svc.SaveStreamChunk(ctx, req.Chunk); err != nil {
		return nil, err
	}
	return &storepb.SaveStreamChunkResponse{}, nil
}
