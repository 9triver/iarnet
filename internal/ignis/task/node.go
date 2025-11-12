package task

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/9triver/iarnet/internal/ignis/types"
	"github.com/9triver/iarnet/internal/ignis/utils"
	"github.com/9triver/iarnet/internal/ignis/utils/errors"
	proto "github.com/9triver/iarnet/internal/proto/ignis"
)

type Node struct {
	id     string
	inputs []string // TODO: dependencies of the node
	group  *ActorGroup
}

func (node *Node) Runtime(sessionID types.SessionID) *Runtime {
	return &Runtime{
		sessionID: sessionID,
		actor:     node.group.Select(),
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
	actor     *Actor
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

	if rt.actor == nil {
		return errors.New("no candidate actor selected")
	}

	logrus.WithFields(logrus.Fields{"actor": rt.actor.GetID()}).Info("task: start grouped task")

	err := rt.actor.Send(&proto.InvokeStart{
		Info:      rt.actor.GetInfo(),
		SessionID: rt.sessionID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (rt *Runtime) Invoke(ctx context.Context, param string, value *proto.Flow) error {

	if rt.actor == nil {
		return errors.New("no candidate actor selected")
	}

	logrus.WithFields(logrus.Fields{"actor": rt.actor.GetID(), "param": param, "value": value}).Info("task: invoke")

	err := rt.actor.Send(&proto.Invoke{
		SessionID: rt.sessionID,
		Param:     param,
		Value:     value,
	})
	if err != nil {
		return err
	}

	rt.deps.Remove(param)

	if rt.Ready() {
		rt.cond.L.Lock()
		rt.cond.Signal()
		rt.cond.L.Unlock()
	}

	return nil
}
