package store

import (
	"context"

	commonpb "github.com/9triver/iarnet/internal/proto/common"
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
		ObjectRef: &commonpb.ObjectRef{
			ID:     request.Object.ID,
			Source: s.store.GetID(),
		},
		Success: true,
		Error:   "",
	}, nil
}

func (s *service) GetObject(ctx context.Context, request *storepb.GetObjectRequest) (*storepb.GetObjectResponse, error) {
	obj, err := s.store.GetObject(request.ObjectRef.ID)
	if err != nil {
		return nil, err
	}
	encodedObj, err := obj.Encode()
	if err != nil {
		return nil, err
	}
	return &storepb.GetObjectResponse{
		Object: encodedObj,
	}, nil
}

func (s *service) GetStreamChunk(ctx context.Context, request *storepb.GetStreamChunkRequest) (*storepb.GetStreamChunkResponse, error) {
	return nil, nil
}
