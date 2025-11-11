package controller

import (
	"context"

	"github.com/9triver/iarnet/internal/resource/component"
)

type Service interface {
	CreateController(ctx context.Context, appID string) (*Controller, error)
}

type service struct {
	manager          Manager
	componentService component.ComponentService
}

func NewService(manager Manager) Service {
	return &service{
		manager: manager,
	}
}

func (s *service) CreateController(ctx context.Context, appID string) (*Controller, error) {
	controller := NewController(s.componentService, appID)
	if err := s.manager.AddController(controller); err != nil {
		return nil, err
	}
	return controller, nil
}
