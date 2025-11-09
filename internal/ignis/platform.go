package ignis

import (
	"github.com/9triver/iarnet/internal/ignis/controller"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	"github.com/9triver/iarnet/internal/resource"
	"google.golang.org/grpc"
)

type Platform struct {
	controllerManager controller.Manager // 控制器管理器 有状态
	controllerService controller.Service // 控制器服务 无状态
}

func NewPlatform(componentService resource.ComponentService) *Platform {
	controllerManager := controller.NewManager(componentService)
	controllerService := controller.NewService(controllerManager)
	return &Platform{
		controllerManager: controllerManager,
		controllerService: controllerService,
	}
}

func (p *Platform) RegisterHandlers(srv *grpc.Server) {
	ctrlpb.RegisterServiceServer(srv, p.controllerService)
}

func (p *Platform) OnControllerEvent(eventType controller.EventType, handler controller.EventHandler) {
	p.controllerManager.On(eventType, handler)
}
