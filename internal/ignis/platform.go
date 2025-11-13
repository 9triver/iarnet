package ignis

import (
	"github.com/9triver/iarnet/internal/ignis/controller"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	"github.com/9triver/iarnet/internal/resource/component"
	"github.com/9triver/iarnet/internal/resource/store"
	"google.golang.org/grpc"
)

type Platform struct {
	controllerManager controller.Manager // 控制器管理器 有状态
	controllerService controller.Service // 控制器服务 无状态
}

func NewPlatform(componentService component.Service, storeService store.Service) *Platform {
	controllerManager := controller.NewManager(componentService)
	controllerService := controller.NewService(controllerManager, componentService, storeService)
	return &Platform{
		controllerManager: controllerManager,
		controllerService: controllerService,
	}
}

func (p *Platform) RegisterHandlers(srv *grpc.Server) {
	ctrlpb.RegisterServiceServer(srv, p.controllerManager)
}

func (p *Platform) OnControllerEvent(eventType controller.EventType, handler controller.EventHandler) {
	p.controllerManager.On(eventType, handler)
}
