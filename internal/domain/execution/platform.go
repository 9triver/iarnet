package execution

import (
	"context"

	"github.com/9triver/iarnet/internal/domain/execution/controller"
	"github.com/9triver/iarnet/internal/domain/execution/task"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
)

var _ controller.Service = (*Platform)(nil)

type Platform struct {
	controllerService controller.Service // 控制器服务 无状态
}

func NewPlatform(controllerService controller.Service) *Platform {
	return &Platform{
		controllerService: controllerService,
	}
}

func (p *Platform) CreateController(ctx context.Context, appID string) (*controller.Controller, error) {
	return p.controllerService.CreateController(ctx, appID)
}

func (p *Platform) GetDAGs(appID string) (map[string]*task.DAG, error) {
	return p.controllerService.GetDAGs(appID)
}

func (p *Platform) Subscribe(eventType controller.EventType, handler controller.EventHandler) {
	p.controllerService.Subscribe(eventType, handler)
}

func (p *Platform) HandleSession(ctx context.Context, recv func() (*ctrlpb.Message, error), send func(*ctrlpb.Message) error) error {
	return p.controllerService.HandleSession(ctx, recv, send)
}

func (p *Platform) GetActors(appID string) (map[string][]*task.Actor, error) {
	return p.controllerService.GetActors(appID)
}

func (p *Platform) RemoveController(ctx context.Context, appID string) error {
	return p.controllerService.RemoveController(ctx, appID)
}

func (p *Platform) ClearController(ctx context.Context, appID string) error {
	return p.controllerService.ClearController(ctx, appID)
}
