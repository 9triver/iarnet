package integration

import (
	"context"
	"fmt"
	"strconv"

	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/ignis/actor/router"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/proto/controller"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/sirupsen/logrus"
)

type Deployer struct {
	am  *application.Manager
	rm  *resource.Manager
	cm  *ConnectionManager
	cfg *config.Config
}

func NewDeployer(am *application.Manager, rm *resource.Manager, cm *ConnectionManager, cfg *config.Config) *Deployer {
	return &Deployer{
		am:  am,
		rm:  rm,
		cm:  cm,
		cfg: cfg,
	}
}

func (d *Deployer) DeployPyFunc(ctx actor.Context, appId string, f *controller.AppendPyFunc, store *proto.StoreRef) ([]*proto.ActorInfo, error) {

	image, ok := d.cfg.ComponentImages["python"]
	if !ok {
		return nil, fmt.Errorf("actor image not found for environment: %s", "python")
	}

	funcMsg := cluster.NewFunction(f.Name, f.Params,
		f.Requirements, f.PickledObject, f.Language)

	infos := make([]*proto.ActorInfo, f.Replicas)

	for i := range f.Replicas {
		name := fmt.Sprintf("%s-%d", f.Name, i)
		connId := fmt.Sprintf("%s:%s", appId, name)
		stream := d.cm.NewConn(context.TODO(), connId)

		cf, err := d.rm.Deploy(context.Background(), resource.ContainerSpec{
			Image: image,
			Requirements: resource.Info{
				CPU:    f.Resources.CPU,
				Memory: f.Resources.Memory,
				GPU:    f.Resources.GPU,
			},
			Env: map[string]string{
				"IGNIG_ADDR": d.cfg.ExternalAddr + ":" + strconv.Itoa(int(25565)),
				// "CONN_ID":    fmt.Sprintf("%s_%s", appId, f.Name),
				"APP_ID":    appId,
				"FUNC_NAME": f.Name,
				// "PYTHON_EXEC": "python",
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to deploy: %w", err)
		}
		logrus.Infof("deployed to provider: %s, container ID: %s", cf.Provider.GetID(), cf.ID)
		d.am.RegisterComponent(appId, name, cf)

		pid := ctx.Spawn(actor.PropsFromProducer(func() actor.Actor {
			return NewStub(stream, store, funcMsg)
		}))

		go func() {
			defer stream.Close()
			for msg := range stream.RecvChan() {
				ctx.Send(pid, msg)
			}
		}()

		info := &proto.ActorInfo{
			Ref: &proto.ActorRef{
				ID:    name,
				PID:   pid,
				Store: store,
			},
			CalcLatency: 0,
			LinkLatency: 0,
		}
		infos[i] = info
	}

	return infos, nil
}

type Stub struct {
	stream  *ClusterStreamImpl
	store   *proto.StoreRef
	funcMsg *cluster.Message
}

func NewStub(stream *ClusterStreamImpl, store *proto.StoreRef, funcMsg *cluster.Message) *Stub {
	return &Stub{
		stream:  stream,
		store:   store,
		funcMsg: funcMsg,
	}
}

func (s *Stub) onClusterMessage(ctx actor.Context, msg *cluster.Message) {
	switch msg.Type {
	case cluster.MessageType_READY:
		s.stream.SendChan() <- s.funcMsg
	case cluster.MessageType_INVOKE, cluster.MessageType_INVOKE_START:
		s.stream.SendChan() <- msg
	case cluster.MessageType_INVOKE_RESPONSE:
		resp := msg.GetInvokeResponse()
		router.Send(ctx, resp.GetTarget().ID, resp)
	default:
		ctx.Logger().Warn("unsupported message type", "msg", msg)
	}
}

func (s *Stub) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *cluster.Message:
		s.onClusterMessage(ctx, msg)
		s.stream.SendChan() <- msg
	default:
		ctx.Logger().Warn("unsupported message type", "msg", msg)
	}
}
