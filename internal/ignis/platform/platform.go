package platform

import (
	"context"

	"github.com/9triver/iarnet/internal/ignis/controller"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	"google.golang.org/grpc"
)

type Platform struct {
	controllerService controller.Service
	controllerManager controller.Manager
}

func NewPlatform() *Platform {
	controllerManager := controller.NewManager()
	controllerService := controller.NewService(controllerManager)
	return &Platform{
		controllerService: controllerService,
		controllerManager: controllerManager,
	}
}

func (p *Platform) Run(ctx context.Context) error {
	return nil
}

func (p *Platform) RegisterHandlers(srv *grpc.Server) {
	ctrlpb.RegisterServiceServer(srv, p.controllerService)
}

func (p *Platform) OnControllerEvent(eventType controller.EventType, handler controller.EventHandler) {
	p.controllerManager.On(eventType, handler)
}
