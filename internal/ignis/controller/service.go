package controller

import (
	"context"

	"github.com/9triver/iarnet/internal/resource/component"
	"github.com/9triver/iarnet/internal/resource/store"
)

type Service interface {
	CreateController(ctx context.Context, appID string) (*Controller, error)
}

type service struct {
	manager          Manager
	componentService component.Service
	storeService     store.Service
}

func NewService(manager Manager, componentService component.Service, storeService store.Service) Service {
	return &service{
		manager:          manager,
		componentService: componentService,
		storeService:     storeService,
	}
}

func (s *service) CreateController(ctx context.Context, appID string) (*Controller, error) {
	controller := NewController(s.componentService, s.storeService, appID)
	if err := s.manager.AddController(controller); err != nil {
		return nil, err
	}
	return controller, nil
}
