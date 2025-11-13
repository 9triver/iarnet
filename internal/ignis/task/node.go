package task

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/9triver/iarnet/internal/ignis/types"
	"github.com/9triver/iarnet/internal/ignis/utils"
	"github.com/9triver/iarnet/internal/ignis/utils/errors"
	proto "github.com/9triver/iarnet/internal/proto/ignis"
	clusterpb "github.com/9triver/iarnet/internal/proto/ignis/cluster"
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
		params:    append([]string(nil), node.inputs...),
		args:      make(map[string]*proto.Flow),
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
	params    []string
	args      map[string]*proto.Flow
}

func (rt *Runtime) Ready() bool {
	return rt.deps.Empty()
}

func (rt *Runtime) Invoke(ctx context.Context) error {
	rt.cond.L.Lock()
	for !rt.Ready() {
		rt.cond.Wait()
	}

	args := make([]*clusterpb.InvokeArg, 0, len(rt.args))
	for _, param := range rt.params {
		if value, ok := rt.args[param]; ok {
			args = append(args, &clusterpb.InvokeArg{
				Param: param,
				Value: value,
			})
		}
	}
	if len(args) < len(rt.args) {
		for param, value := range rt.args {
			found := false
			for _, existing := range args {
				if existing.Param == param {
					found = true
					break
				}
			}
			if !found {
				args = append(args, &clusterpb.InvokeArg{
					Param: param,
					Value: value,
				})
			}
		}
	}
	rt.cond.L.Unlock()

	if rt.actor == nil {
		return errors.New("no candidate actor selected")
	}

	logrus.WithFields(logrus.Fields{
		"actor":   rt.actor.GetID(),
		"session": rt.sessionID,
		"args":    len(args),
	}).Info("task: invoke request")

	req := &clusterpb.InvokeRequest{
		SessionID: string(rt.sessionID),
		Args:      args,
	}

	return rt.actor.Send(req)
}

func (rt *Runtime) AddArg(param string, value *proto.Flow) error {
	if rt.actor == nil {
		return errors.New("no candidate actor selected")
	}

	logrus.WithFields(logrus.Fields{"actor": rt.actor.GetID(), "param": param}).Info("task: add invoke arg")

	rt.cond.L.Lock()
	rt.args[param] = value
	if rt.deps.Remove(param) {
		if rt.deps.Empty() {
			rt.cond.Signal()
		}
	}
	rt.cond.L.Unlock()

	return nil
}
