package service

import (
	"context"
	"fmt"

	dom "github.com/9triver/iarnet/internal/ignis/controller"
	"github.com/9triver/iarnet/internal/ignis/manager"
	"github.com/9triver/iarnet/internal/ignis/transport"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
)

// ControllerService 以 transport.SessionServer 为入口，委托 manager。
type ControllerService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type controllerService struct {
	server  transport.SessionServer
	manager manager.ControllerManager
}

func NewControllerService(server transport.SessionServer, mgr manager.ControllerManager) ControllerService {
	s := &controllerService{server: server, manager: mgr}
	server.OnSession(s.handleSession)
	return s
}

func (s *controllerService) Start(ctx context.Context) error { return s.server.Start(ctx) }
func (s *controllerService) Stop(ctx context.Context) error  { return s.server.Stop(ctx) }

func (s *controllerService) handleSession(stream transport.SessionStream) error {
	// 简化：第一条消息必须是 RegisterRequest，携带 ApplicationID
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	reg, ok := first.Command.(*ctrlpb.Message_RegisterRequest)
	if !ok || reg.RegisterRequest == nil {
		return fmt.Errorf("first message must be RegisterRequest")
	}
	appID := reg.RegisterRequest.ApplicationID
	ctrl, err := s.manager.GetOrCreateController(stream.Context(), appID)
	if err != nil {
		return err
	}
	// 简化 sessionID：直接使用 appID 生成一次性标识或由上层注入；这里用 appID 代替
	_ = s.manager.AttachSession(appID, appID)
	defer func() { s.manager.DetachSession(appID, appID); s.manager.ReleaseIfIdle(stream.Context(), appID) }()

	// 将 RegisterRequest 转为领域消息
	_ = ctrl.HandleMessage(stream.Context(), &dom.RegisterRequest{ApplicationID: appID})

	// 后续消息简单忽略或可逐步映射为领域消息
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		_ = msg // TODO: 后续映射为 dom.Message 并调用 ctrl.HandleMessage
	}
}
