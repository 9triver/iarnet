package store

import (
	"context"

	commonpb "github.com/9triver/iarnet/internal/proto/common"
)

// Service defines store operations without binding to any transport implementation.
type Service interface {
	SaveObject(ctx context.Context, obj *commonpb.EncodedObject) (*commonpb.ObjectRef, error)
	SaveStreamChunk(ctx context.Context, chunk *commonpb.StreamChunk) error
	GetObject(ctx context.Context, ref *commonpb.ObjectRef) (*commonpb.EncodedObject, error)
	GetStreamChunk(ctx context.Context, id string, offset int64) (*commonpb.StreamChunk, error)
	GetID(ctx context.Context) (string, error)
}

type service struct {
	store *Store
}

func NewService(store *Store) Service {
	return &service{
		store: store,
	}
}

func (s *service) SaveObject(ctx context.Context, obj *commonpb.EncodedObject) (*commonpb.ObjectRef, error) {
	s.store.SaveObject(obj)
	return &commonpb.ObjectRef{
		ID:     obj.ID,
		Source: s.store.GetID(),
	}, nil
}

func (s *service) SaveStreamChunk(ctx context.Context, chunk *commonpb.StreamChunk) error {
	return s.store.SaveStreamChunk(chunk)
}

func (s *service) GetObject(ctx context.Context, ref *commonpb.ObjectRef) (*commonpb.EncodedObject, error) {
	obj, err := s.store.GetObject(ref.ID)
	if err != nil {
		return nil, err
	}
	encodedObj, err := obj.Encode()
	if err != nil {
		return nil, err
	}
	return encodedObj, nil
}

func (s *service) GetStreamChunk(ctx context.Context, id string, offset int64) (*commonpb.StreamChunk, error) {
	return s.store.GetStreamChunk(id, offset)
}

func (s *service) GetID(ctx context.Context) (string, error) {
	return string(s.store.GetID()), nil
}
