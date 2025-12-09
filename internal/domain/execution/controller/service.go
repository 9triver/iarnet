package controller

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/domain/execution/task"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/store"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
)

type Service interface {
	CreateController(ctx context.Context, appID string) (*Controller, error)
	GetDAGs(appID string) (map[string]*task.DAG, error)
	GetActors(appID string) (map[string][]*task.Actor, error)
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
	if err := s.manager.Add(controller); err != nil {
		return nil, err
	}
	return controller, nil
}

func (s *service) GetDAGs(appID string) (map[string]*task.DAG, error) {
	controller := s.manager.Get(appID)
	if controller == nil {
		return nil, fmt.Errorf("controller not found")
	}
	return controller.GetDAGs(), nil
}

func (s *service) Subscribe(eventType EventType, handler EventHandler) {
	s.manager.On(eventType, handler)
}

func (s *service) HandleSession(ctx context.Context, recv func() (*ctrlpb.Message, error), send func(*ctrlpb.Message) error) error {
	return s.manager.HandleSession(ctx, recv, send)
}

func (s *service) GetActors(appID string) (map[string][]*task.Actor, error) {
	controller := s.manager.Get(appID)
	if controller == nil {
		return nil, fmt.Errorf("controller not found")
	}
	return controller.GetActors(), nil
}
