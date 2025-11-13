package task

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/9triver/iarnet/internal/ignis/types"
	"github.com/9triver/iarnet/internal/ignis/utils"
	"github.com/9triver/iarnet/internal/ignis/utils/errors"
	commonpb "github.com/9triver/iarnet/internal/proto/common"
	actorpb "github.com/9triver/iarnet/internal/proto/ignis/actor"
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
		args:      make(map[string]*commonpb.ObjectRef),
		onComplete: func(ctx context.Context, actor *Actor) {
			node.group.Push(actor)
		},
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
	sessionID  types.SessionID
	complete   chan struct{}
	actor      *Actor
	deps       utils.Set[string]
	cond       *sync.Cond
	params     []string
	args       map[string]*commonpb.ObjectRef
	onComplete func(ctx context.Context, actor *Actor)
	invokeTime time.Time // 记录调用发送时间，用于计算链路延迟
}

func (rt *Runtime) Ready() bool {
	return rt.deps.Empty()
}

func (rt *Runtime) Invoke(ctx context.Context) error {
	rt.cond.L.Lock()
	for !rt.Ready() {
		rt.cond.Wait()
	}

	args := make([]*actorpb.InvokeArg, 0, len(rt.args))
	for _, param := range rt.params {
		if value, ok := rt.args[param]; ok {
			args = append(args, &actorpb.InvokeArg{
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
				args = append(args, &actorpb.InvokeArg{
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

	// 记录调用发送时间，用于计算链路延迟
	rt.invokeTime = time.Now()

	req := &actorpb.InvokeRequest{
		SessionID: string(rt.sessionID),
		Args:      args,
	}

	return rt.actor.Send(req)
}

func (rt *Runtime) AddArg(param string, value *commonpb.ObjectRef) error {
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

func (rt *Runtime) Complete(ctx context.Context, actorInfo *actorpb.ActorInfo) {
	// 计算延迟并更新 Actor 信息
	if actorInfo != nil {
		// 计算总延迟（从发送请求到收到响应的时间，单位：毫秒）
		totalLatency := time.Since(rt.invokeTime).Milliseconds()

		// 从响应中获取计算延迟（Python 端提供）
		calcLatency := actorInfo.CalcLatency

		// 计算链路延迟 = 总延迟 - 计算延迟
		linkLatency := totalLatency - calcLatency
		if linkLatency < 0 {
			linkLatency = 0
		}

		// 使用移动平均更新延迟信息（新值 + 旧值）/ 2
		oldCalcLatency := rt.actor.info.CalcLatency
		oldLinkLatency := rt.actor.info.LinkLatency

		if oldCalcLatency == 0 {
			// 第一次调用，直接使用新值
			rt.actor.info.CalcLatency = calcLatency
			rt.actor.info.LinkLatency = linkLatency
		} else {
			// 使用移动平均
			rt.actor.info.CalcLatency = (oldCalcLatency + calcLatency) / 2
			rt.actor.info.LinkLatency = (oldLinkLatency + linkLatency) / 2
		}

		logrus.WithFields(logrus.Fields{
			"actor":         rt.actor.GetID(),
			"session":       rt.sessionID,
			"calc_latency":  rt.actor.info.CalcLatency,
			"link_latency":  rt.actor.info.LinkLatency,
			"total_latency": totalLatency,
		}).Info("task: updated actor latency")
	}

	rt.onComplete(ctx, rt.actor)
}
