package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/9triver/iarnet/internal/domain/ignis/types"
	"github.com/9triver/iarnet/internal/domain/ignis/utils"
	"github.com/9triver/iarnet/internal/domain/ignis/utils/errors"
	commonpb "github.com/9triver/iarnet/internal/proto/common"
	actorpb "github.com/9triver/iarnet/internal/proto/ignis/actor"
)

type Function struct {
	name     string
	inputs   []string // TODO: dependencies of the node
	group    *ActorGroup
	runtimes map[string]*Runtime
}

func (f *Function) Runtime(sessionID string, instanceID string) types.RuntimeID {
	runtimeID := fmt.Sprintf("%s::%s::%s", f.name, sessionID, instanceID)
	runtime, ok := f.runtimes[runtimeID]
	if !ok {
		runtime = &Runtime{
			functionName: f.name,
			runtimeID:    runtimeID,
			actor:        f.group.Select(),
			deps:         utils.MakeSetFromSlice(f.inputs),
			cond:         sync.NewCond(&sync.Mutex{}),
			params:       append([]string(nil), f.inputs...),
			args:         make(map[string]*commonpb.ObjectRef),
		}
		f.runtimes[runtimeID] = runtime
	} else {
		logrus.WithFields(logrus.Fields{"runtime": runtimeID}).Errorf("task: runtime already exists")
	}
	return runtimeID
}

func NewFunction(name string, inputs []string, group *ActorGroup) *Function {
	return &Function{
		name:     name,
		inputs:   inputs,
		group:    group,
		runtimes: make(map[string]*Runtime),
	}
}

func (f *Function) GetName() string {
	return f.name
}

func (f *Function) GetActors() []*Actor {
	return f.group.GetAll()
}

func (f *Function) IsReady(runtimeID types.RuntimeID) bool {
	runtime, ok := f.runtimes[runtimeID]
	if !ok {
		return false
	}
	return runtime.Ready()
}

// AddArg 添加参数
func (f *Function) AddArg(runtimeID types.RuntimeID, param string, value *commonpb.ObjectRef) error {
	runtime, ok := f.runtimes[runtimeID]
	if !ok {
		logrus.WithFields(logrus.Fields{"runtime": runtimeID}).Errorf("task: runtime not found")
		return fmt.Errorf("runtime not found: %s", runtimeID)
	}
	return runtime.AddArg(param, value)
}

// Invoke 执行函数
func (f *Function) Invoke(ctx context.Context, runtimeID types.RuntimeID) error {
	runtime, ok := f.runtimes[runtimeID]
	if !ok {
		logrus.WithFields(logrus.Fields{"runtime": runtimeID}).Errorf("task: runtime not found")
		return fmt.Errorf("runtime not found: %s", runtimeID)
	}
	return runtime.Invoke(ctx)
}

// Complete 完成函数执行
func (f *Function) Done(ctx context.Context, runtimeID types.RuntimeID, actorInfo *actorpb.ActorInfo) error {
	runtime, ok := f.runtimes[runtimeID]
	if !ok {
		logrus.WithFields(logrus.Fields{"runtime": runtimeID}).Errorf("task: runtime not found")
		return fmt.Errorf("runtime not found: %s", runtimeID)
	}
	actor := runtime.Done(ctx, actorInfo)
	if actor == nil {
		logrus.WithFields(logrus.Fields{"runtime": runtimeID}).Errorf("task: actor not found")
		return fmt.Errorf("actor not found: %s", runtimeID)
	}
	f.group.Push(actor)
	return nil
}

type Runtime struct {
	functionName string
	runtimeID    types.RuntimeID
	actor        *Actor
	deps         utils.Set[string]
	cond         *sync.Cond
	params       []string
	args         map[string]*commonpb.ObjectRef
	onComplete   func(ctx context.Context, actor *Actor)
	invokeTime   time.Time // 记录调用发送时间，用于计算链路延迟
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
		"runtime": rt.runtimeID,
		"args":    len(args),
	}).Info("task: invoke request")

	// 记录调用发送时间，用于计算链路延迟
	rt.invokeTime = time.Now()

	req := &actorpb.InvokeRequest{
		RuntimeID: string(rt.runtimeID),
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

func (rt *Runtime) Done(ctx context.Context, actorInfo *actorpb.ActorInfo) *Actor {
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
			"function":      rt.functionName,
			"runtime":       rt.runtimeID,
			"calc_latency":  rt.actor.info.CalcLatency,
			"link_latency":  rt.actor.info.LinkLatency,
			"total_latency": totalLatency,
		}).Info("task: updated actor latency")
	}

	return rt.actor
}
