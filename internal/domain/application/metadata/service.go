package metadata

import (
	"context"
	"time"

	"github.com/9triver/iarnet/internal/domain/application/types"
	"github.com/9triver/iarnet/internal/util"
)

type Service interface {
	GetAllAppMetadata(ctx context.Context) ([]types.AppMetadata, error)
	GetAppMetadata(ctx context.Context, appID string) (types.AppMetadata, error)
	UpdateAppMetadata(ctx context.Context, appID string, metadata types.AppMetadata) error
	UpdateAppStatus(ctx context.Context, appID string, status types.AppStatus) error
	CreateAppMetadata(ctx context.Context, metadata types.AppMetadata) (types.AppID, error)
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

func (s *service) GetAllAppMetadata(ctx context.Context) ([]types.AppMetadata, error) {
	return s.cache.GetAll()
}

func (s *service) CreateAppMetadata(ctx context.Context, metadata types.AppMetadata) (types.AppID, error) {
	appID := types.AppID(util.GenIDWith("app."))
	metadata.ID = appID
	s.cache.Set(appID, metadata)
	return appID, nil
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
	// 当应用状态变为运行中时，更新最后部署时间
	if status == types.AppStatusRunning {
		metadata.LastDeployed = time.Now()
	}
	s.cache.Set(appID, metadata)
	return nil
}

func (s *service) RemoveAppMetadata(ctx context.Context, appID string) error {
	s.cache.Delete(appID)
	return nil
}
