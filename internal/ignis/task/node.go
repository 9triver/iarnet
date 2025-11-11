package task

import (
	"context"
	"sync"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/sirupsen/logrus"

	"github.com/9triver/iarnet/internal/ignis/types"
	"github.com/9triver/iarnet/internal/ignis/utils"
	"github.com/9triver/ignis/proto"
	clusterpb "github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/utils/errors"
)

type Node struct {
	id     string
	inputs []string // TODO: dependencies of the node
	group  *ActorGroup
}

func (node *Node) Runtime(sessionID types.SessionID) *Runtime {
	return &Runtime{
		sessionID: sessionID,
		actorInfo: node.group.Select(),
		deps:      utils.MakeSetFromSlice(node.inputs),
		cond:      sync.NewCond(&sync.Mutex{}),
	}
}

func NewNode(id string, inputs []string, group *ActorGroup) *Node {
	return &Node{
		id:     id,
		inputs: inputs,
		group:  group,
	}
}

type Runtime struct {
	sessionID types.SessionID
	actorInfo *ActorInfo
	deps      utils.Set[string]
	cond      *sync.Cond
}

func (rt *Runtime) Ready() bool {
	return rt.deps.Empty()
}

func (rt *Runtime) Start(ctx context.Context) error {
	rt.cond.L.Lock()
	for !rt.Ready() {
		rt.cond.Wait()
	}
	rt.cond.L.Unlock()

	if rt.actorInfo == nil {
		return errors.New("no candidate actor selected")
	}

	logrus.Infof("task: start grouped task", "actor", rt.actorInfo.ID)

	rt.actorInfo.Component.Send(clusterpb.NewMessage(&proto.InvokeStart{
		Info:      &rt.actorInfo.ActorInfo,
		SessionID: rt.sessionID,
	}))

	return nil
}

func (rt *Runtime) Invoke(ctx actor.Context, param string, value *proto.Flow) (err error) {

	if rt.actorInfo == nil {
		return errors.New("no candidate actor selected")
	}

	rt.actorInfo.Component.Send(clusterpb.NewMessage(&proto.Invoke{
		Target:    rt.actorInfo.ID,
		SessionID: rt.sessionID,
		Param:     param,
		Value:     value,
	}))

	rt.deps.Remove(param)

	if rt.Ready() {
		rt.cond.L.Lock()
		rt.cond.Signal()
		rt.cond.L.Unlock()
	}

	return nil
}
