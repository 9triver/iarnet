package store

import (
	"context"

	storepb "github.com/9triver/iarnet/internal/proto/resource/store"
)

// TODO: rpc service 与 内部 service 分离
type Service interface {
	storepb.ServiceServer
}

type service struct {
	storepb.UnimplementedServiceServer
	store *Store
}

func NewService(store *Store) Service {
	return &service{
		store: store,
	}
}

func (s *service) SaveObject(ctx context.Context, request *storepb.SaveObjectRequest) (*storepb.SaveObjectResponse, error) {
	s.store.SaveObject(request.Object)
	return &storepb.SaveObjectResponse{
		ObjectRef: &storepb.ObjectRef{
			ID:     request.Object.ID,
			Source: s.store.GetID(),
		},
		Success: true,
		Error:   "",
	}, nil
}

func (s *service) GetObject(ctx context.Context, request *storepb.GetObjectRequest) (*storepb.GetObjectResponse, error) {
	return nil, nil
}

func (s *service) GetStreamChunk(ctx context.Context, request *storepb.GetStreamChunkRequest) (*storepb.GetStreamChunkResponse, error) {
	return nil, nil
}
