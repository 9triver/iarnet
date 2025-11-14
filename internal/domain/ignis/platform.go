package ignis

import (
	"context"

	"github.com/9triver/iarnet/internal/domain/ignis/controller"
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

func (p *Platform) Subscribe(eventType controller.EventType, handler controller.EventHandler) {
	p.controllerService.Subscribe(eventType, handler)
}

func (p *Platform) HandleSession(ctx context.Context, recv func() (*ctrlpb.Message, error), send func(*ctrlpb.Message) error) error {
	return p.controllerService.HandleSession(ctx, recv, send)
}
