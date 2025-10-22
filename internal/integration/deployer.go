package integration

import (
	"context"
	"fmt"
	"strconv"

	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/ignis/actor/router"
	"github.com/9triver/ignis/actor/store"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/proto/controller"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/sirupsen/logrus"
	pb "google.golang.org/protobuf/proto"
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

func (d *Deployer) DeployPyFunc(ctx actor.Context, appId string, f *controller.AppendPyFunc, sr *proto.StoreRef) ([]*proto.ActorInfo, error) {

	image, ok := d.cfg.ComponentImages["python"]
	if !ok {
		return nil, fmt.Errorf("actor image not found for environment: %s", "python")
	}

	funcMsg := cluster.NewFunction(f.Name, f.Params,
		f.Requirements, f.PickledObject, f.Language)

	infos := make([]*proto.ActorInfo, f.Replicas)

	for i := range f.Replicas {
		connId := fmt.Sprintf("%s:%s-%d", appId, f.Name, i)
		stream := d.cm.NewConn(context.TODO(), connId)
		stream.SendChan() <- funcMsg

		cf, err := d.rm.Deploy(context.Background(), resource.ContainerSpec{
			Image: image,
			Requirements: resource.Info{
				CPU:    f.Resources.CPU,
				Memory: f.Resources.Memory,
				GPU:    f.Resources.GPU,
			},
			Env: map[string]string{
				"IGNIS_ADDR": d.cfg.ExternalAddr + ":" + strconv.Itoa(int(25565)),
				"CONN_ID":    connId,
				// "APP_ID":     appId,
				// "FUNC_NAME":  f.Name,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to deploy: %w", err)
		}
		logrus.Infof("deployed to provider: %s, container ID: %s", cf.Provider.GetID(), cf.ID)
		d.am.RegisterComponent(appId, connId, cf)

		pid := ctx.Spawn(actor.PropsFromProducer(func() actor.Actor {
			return NewStub(stream)
		}))

		router.Register(connId, pid)
		router.Register("store-"+connId, pid)

		go func() {
			defer stream.Close()
			for msg := range stream.RecvChan() {
				m := msg.Unwrap()
				if mt, ok := m.(store.RequiresReplyMessage); ok {
					router.RegisterIfAbsent(mt.GetReplyTo(), pid)
				}
				if mt, ok := m.(store.ForwardMessage); ok {
					router.Send(ctx, mt.GetTarget(), mt)
				} else {
					logrus.Warnf("unsupported message type: %+v", m)
				}
			}
		}()

		info := &proto.ActorInfo{
			Ref: &proto.ActorRef{
				ID:    connId,
				PID:   pid,
				Store: sr,
			},
			CalcLatency: 0,
			LinkLatency: 0,
		}
		infos[i] = info
	}

	return infos, nil
}

type Stub struct {
	stream *ClusterStreamImpl
}

func NewStub(stream *ClusterStreamImpl) *Stub {
	return &Stub{
		stream: stream,
	}
}

func (s *Stub) Receive(ctx actor.Context) {
	if msg, ok := ctx.Message().(pb.Message); ok {
		m := cluster.NewMessage(msg)
		if m == nil {
			ctx.Logger().Warn("unsupported message type", "msg", msg)
		} else {
			s.stream.SendChan() <- m
		}
	} else {
		ctx.Logger().Warn("unsupported message type", "msg", ctx.Message())
	}
}
