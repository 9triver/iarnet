package metadata

import (
	"context"

	"github.com/9triver/iarnet/internal/domain/application/types"
)

type Service interface {
	GetAppMetadata(ctx context.Context, appID string) (types.AppMetadata, error)
	UpdateAppMetadata(ctx context.Context, appID string, metadata types.AppMetadata) error
	UpdateAppStatus(ctx context.Context, appID string, status types.AppStatus) error
	CreateAppMetadata(ctx context.Context, appID string, metadata types.AppMetadata) error
	RemoveAppMetadata(ctx context.Context, appID string) error
}

type service struct {
	cache *Cache
}

func NewService(cache *Cache) Service {
	return &service{
		cache: cache,
	}
}

func (s *service) CreateAppMetadata(ctx context.Context, appID string, metadata types.AppMetadata) error {
	s.cache.Set(appID, metadata)
	return nil
}

func (s *service) GetAppMetadata(ctx context.Context, appID string) (types.AppMetadata, error) {
	return s.cache.Get(appID)
}

func (s *service) UpdateAppMetadata(ctx context.Context, appID string, metadata types.AppMetadata) error {
	s.cache.Set(appID, metadata)
	return nil
}

func (s *service) UpdateAppStatus(ctx context.Context, appID string, status types.AppStatus) error {
	metadata, err := s.cache.Get(appID)
	if err != nil {
		return err
	}
	metadata.Status = status
	s.cache.Set(appID, metadata)
	return nil
}

func (s *service) RemoveAppMetadata(ctx context.Context, appID string) error {
	s.cache.Delete(appID)
	return nil
}
