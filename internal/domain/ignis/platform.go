package ignis

import (
	domaincontroller "github.com/9triver/iarnet/internal/domain/ignis/controller"
	domaincomponent "github.com/9triver/iarnet/internal/domain/resource/component"
	domainstore "github.com/9triver/iarnet/internal/domain/resource/store"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	controllerrpc "github.com/9triver/iarnet/internal/transport/rpc/ignis/controller"
	"google.golang.org/grpc"
)

type Platform struct {
	controllerManager domaincontroller.Manager // 控制器管理器 有状态
	controllerService domaincontroller.Service // 控制器服务 无状态
}

func NewPlatform(componentService domaincomponent.Service, storeService domainstore.Service) *Platform {
	controllerManager := domaincontroller.NewManager(componentService)
	controllerService := domaincontroller.NewService(controllerManager, componentService, storeService)
	return &Platform{
		controllerManager: controllerManager,
		controllerService: controllerService,
	}
}

func (p *Platform) RegisterHandlers(srv *grpc.Server) {
	ctrlpb.RegisterServiceServer(srv, controllerrpc.NewServer(p.controllerManager))
}

func (p *Platform) OnControllerEvent(eventType domaincontroller.EventType, handler domaincontroller.EventHandler) {
	p.controllerManager.On(eventType, handler)
}
