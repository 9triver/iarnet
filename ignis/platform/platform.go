package platform

import (
	"context"
	"path"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/sirupsen/logrus"

	"github.com/9triver/ignis/actor/remote"
	"github.com/9triver/ignis/actor/remote/ipc"
	"github.com/9triver/ignis/actor/remote/rpc"
	"github.com/9triver/ignis/configs"
	"github.com/9triver/ignis/platform/control"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/controller"
	"github.com/9triver/ignis/utils"
)

type Platform struct {
	// Root context of platform
	ctx context.Context
	// Main actor system of the platform
	sys *actor.ActorSystem
	// Control connection manager
	cm remote.ControllerManager
	// Executor connection manager
	em remote.ExecutorManager
	// Application infos
	appInfos map[string]*ApplicationInfo
	// Controller actor refs
	controllerActorRefs map[string]*proto.ActorRef
}

func (p *Platform) Run() error {
	ctx, cancel := context.WithCancel(p.ctx)
	defer cancel()

	go func() {
		if err := p.cm.Run(ctx); err != nil {
			panic(err)
		}
	}()

	go func() {
		if err := p.em.Run(ctx); err != nil {
			panic(err)
		}
	}()

	go func() {
		for {
			ctrlr := p.cm.NextController()
			if ctrlr == nil {
				continue
			}
			msg := <-ctrlr.RecvChan()
			if msg.Type == controller.CommandType_FR_REGISTER_REQUEST {
				req := msg.GetRegisterRequest()
				if req == nil {
					logrus.Error("Register request is nil")
					continue
				}
				appID := req.GetApplicationID()
				if _, ok := p.appInfos[appID]; ok {
					logrus.Errorf("Application ID %s is conflicted", appID)
					continue
				}
				appInfo := &ApplicationInfo{ID: appID}
				actorRef := control.SpawnTaskControllerV2(p.sys.Root, appID, appInfo, ctrlr, func() {
					delete(p.appInfos, appID)
				})
				p.appInfos[appID] = appInfo
				p.controllerActorRefs[appID] = actorRef
				logrus.Infof("Application %s is registered", appID)
			} else {
				logrus.Errorf("The first message %s is not register request", msg.Type)
			}
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

func NewPlatform(ctx context.Context, cfg *configs.Config) *Platform {
	opt := utils.WithLogger()
	ipcAddr := "ipc://" + path.Join(configs.StoragePath, "em-ipc")

	return &Platform{
		ctx: ctx,
		sys: actor.NewActorSystem(opt),
		cm:  rpc.NewManager(cfg.RpcAddr),
		em:  ipc.NewManager(ipcAddr),
	}
}

type ApplicationInfo struct {
	ID string
}
