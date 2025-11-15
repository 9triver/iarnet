package controller

import (
	"context"

	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/store"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
)

type Service interface {
	CreateController(ctx context.Context, appID string) (*Controller, error)
	Subscribe(eventType EventType, handler EventHandler)
	HandleSession(ctx context.Context, recv func() (*ctrlpb.Message, error), send func(*ctrlpb.Message) error) error
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

func (s *service) Subscribe(eventType EventType, handler EventHandler) {
	s.manager.On(eventType, handler)
}

func (s *service) HandleSession(ctx context.Context, recv func() (*ctrlpb.Message, error), send func(*ctrlpb.Message) error) error {
	return s.manager.HandleSession(ctx, recv, send)
}
