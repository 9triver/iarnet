package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/9triver/iarnet/internal/domain/ignis/task"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/store"
	resourceTypes "github.com/9triver/iarnet/internal/domain/resource/types"
	commonpb "github.com/9triver/iarnet/internal/proto/common"
	actorpb "github.com/9triver/iarnet/internal/proto/ignis/actor"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	"github.com/sirupsen/logrus"
)

type Controller struct {
	appID            string
	events           *EventHub
	toClientChan     chan *ctrlpb.Message
	componentService component.Service
	storeService     store.Service
	nodes            map[string]*task.Node
	runtimes         map[string]*task.Runtime
}

func NewController(componentService component.Service, storeService store.Service, appID string) *Controller {
	return &Controller{
		appID:            appID,
		componentService: componentService,
		storeService:     storeService,
		nodes:            make(map[string]*task.Node),
		runtimes:         make(map[string]*task.Runtime),
	}
}

func (c *Controller) SetToClientChan(toClientChan chan *ctrlpb.Message) {
	c.toClientChan = toClientChan
}

func (c *Controller) ClearToClientChan() {
	c.toClientChan = nil
}

func (c *Controller) SetEvents(events *EventHub) {
	c.events = events
}

func (c *Controller) GetToClientChan() chan *ctrlpb.Message {
	return c.toClientChan
}

func (c *Controller) AppID() string { return c.appID }

func (c *Controller) handleInvokeResponse(ctx context.Context, m *actorpb.InvokeResponse) error {
	logrus.WithFields(logrus.Fields{"session": m.SessionID}).Info("control: invoke response")
	runtimeId := m.SessionID
	rt, ok := c.runtimes[runtimeId]
	if !ok {
		logrus.Errorf("Runtime not found: %s", runtimeId)
		return fmt.Errorf("runtime not found: %s", runtimeId)
	}
	splits := strings.SplitN(m.SessionID, "::", 3)
	name, sessionId, instanceId := splits[0], splits[1], splits[2]
	if m.Error != "" {
		logrus.WithFields(logrus.Fields{"name": name, "session": sessionId, "instance": instanceId, "error": m.Error}).Info("control: invoke failed")
		ret := ctrlpb.NewReturnResult(sessionId, instanceId, name, nil, errors.New(m.Error))
		c.PushToClient(ctx, ret)
		return nil
	}
	rt.Complete(ctx, m.Info)
	delete(c.runtimes, runtimeId)

	ret := ctrlpb.NewReturnResult(sessionId, instanceId, name, m.Result, nil)
	c.PushToClient(ctx, ret)
	return nil
}

func (c *Controller) HandleActorMessage(ctx context.Context, msg *actorpb.Message) error {
	switch m := msg.GetMessage().(type) {
	case *actorpb.Message_InvokeResponse:
		return c.handleInvokeResponse(ctx, m.InvokeResponse)
	default:
		return nil
	}
}

func (c *Controller) HandleClientMessage(ctx context.Context, msg *ctrlpb.Message) error {
	switch m := msg.GetCommand().(type) {
	case *ctrlpb.Message_AppendPyFunc:
		return c.handleAppendPyFunc(ctx, m.AppendPyFunc)
	case *ctrlpb.Message_AppendData:
		return c.handleAppendData(ctx, m.AppendData)
	case *ctrlpb.Message_AppendPyClass:
		return c.handleAppendPyClass(ctx, m.AppendPyClass)
	case *ctrlpb.Message_AppendArg:
		return c.handleAppendArg(ctx, m.AppendArg)
	case *ctrlpb.Message_AppendClassMethodArg:
		return c.handleAppendClassMethodArg(ctx, m.AppendClassMethodArg)
	case *ctrlpb.Message_Invoke:
		return c.handleInvoke(ctx, m.Invoke)
	case *ctrlpb.Message_MarkDAGNodeDone:
		return c.handleMarkDAGNodeDone(ctx, m.MarkDAGNodeDone)
	case *ctrlpb.Message_RequestObject:
		return c.handleRequestObject(ctx, m.RequestObject)
	default:
		return nil
	}
}

func (c *Controller) handleAppendData(ctx context.Context, m *ctrlpb.AppendData) error {
	obj := m.Object
	logrus.WithFields(logrus.Fields{"id": obj.ID, "session": m.SessionID}).Info("control: append data node")
	go func() {
		// TODO: 整理 proto 定义，将 EncodedObject 和 Object 合并
		resp, err := c.storeService.SaveObject(ctx, m.Object)
		if err != nil {
			logrus.Errorf("Failed to save object: %v", err)
			return
		}
		logrus.Infof("Object saved successfully: %s", resp.ID)
		ret := ctrlpb.NewReturnResult(m.SessionID, "", resp.ID, resp, nil)
		c.PushToClient(ctx, ret)
	}()

	return nil
}

func (c *Controller) handleAppendPyFunc(ctx context.Context, m *ctrlpb.AppendPyFunc) error {
	actorGroup := task.NewGroup(m.GetName())
	replicas := int(m.GetReplicas())
	for i := 0; i < replicas; i++ {
		actorName := fmt.Sprintf("%s-%d", m.GetName(), i)
		logrus.Infof("Deploying component for actor %s", actorName)
		component, err := c.componentService.DeployComponent(
			ctx, m.GetName(), resourceTypes.RuntimeEnvPython,
			&resourceTypes.Info{
				CPU:    int64(m.GetResources().GetCPU()),
				Memory: int64(m.GetResources().GetMemory()),
				GPU:    int64(m.GetResources().GetGPU()),
			},
		)
		if err != nil {
			logrus.Errorf("Failed to deploy component: %v", err)
			return err
		}
		logrus.Infof("Component deployed successfully: %s", component.GetID())

		actor := task.NewActor(actorName, component)
		actor.Send(&actorpb.Function{
			Name:          m.GetName(),
			Params:        m.GetParams(),
			Requirements:  m.GetRequirements(),
			PickledObject: m.GetPickledObject(),
			Language:      m.GetLanguage(),
		})
		logrus.Infof("Function sent to actor: %s", actor.GetID())

		go func() {
			for {
				msg := actor.Receive(ctx)
				if msg == nil {
					logrus.Errorf("Actor message is nil")
					return
				}
				if err := c.HandleActorMessage(ctx, msg); err != nil {
					logrus.Errorf("Failed to handle actor message: %v", err)
					return
				}
			}
		}()
		actorGroup.Push(actor)
	}

	c.nodes[m.GetName()] = task.NewNode(m.GetName(), m.GetParams(), actorGroup)

	return nil
}

func (c *Controller) handleAppendArg(ctx context.Context, m *ctrlpb.AppendArg) error {
	logrus.WithFields(logrus.Fields{"name": m.Name, "param": m.Param, "session": m.SessionID, "instance": m.InstanceID}).Info("control: append arg")

	rt, err := c.getOrCreateRuntime(m.Name, m.SessionID, m.InstanceID)
	if err != nil {
		logrus.Errorf("Failed to get or create runtime: %v", err)
		return err
	}

	switch v := m.Value.Object.(type) {
	case *ctrlpb.Data_Ref:
		if err := rt.AddArg(m.Param, v.Ref); err != nil {
			logrus.Errorf("Failed to add runtime argument: %v", err)
			ret := ctrlpb.NewReturnResult(m.SessionID, m.InstanceID, m.Name, nil, err)
			c.PushToClient(ctx, ret)
			return err
		}
	case *ctrlpb.Data_Encoded:
		logrus.WithFields(logrus.Fields{"id": v.Encoded.ID, "session": m.SessionID, "instance": m.InstanceID}).Info("control: append encoded arg")
		go func() {
			resp, err := c.storeService.SaveObject(ctx, v.Encoded)
			if err != nil {
				logrus.Errorf("Failed to save object: %v", err)
				ret := ctrlpb.NewReturnResult(m.SessionID, m.InstanceID, m.Name, nil, err)
				c.PushToClient(ctx, ret)
				return
			}
			logrus.Infof("Object saved successfully: %s", resp.ID)
			if err = rt.AddArg(m.Param, resp); err != nil {
				logrus.Errorf("Failed to add runtime argument: %v", err)
				ret := ctrlpb.NewReturnResult(m.SessionID, m.InstanceID, m.Name, nil, err)
				c.PushToClient(ctx, ret)
				return
			}
		}()
	default:
		logrus.Errorf("Unsupported data type: %T", v)
		return fmt.Errorf("unsupported data type: %T", v)
	}
	return nil
}

func (c *Controller) handleInvoke(ctx context.Context, m *ctrlpb.Invoke) error {
	logrus.WithFields(logrus.Fields{"name": m.Name, "session": m.SessionID, "instance": m.InstanceID}).Info("control: invoke")
	rt, err := c.getOrCreateRuntime(m.Name, m.SessionID, m.InstanceID)
	if err != nil {
		logrus.Errorf("Failed to get or create runtime: %v", err)
		return err
	}
	return rt.Invoke(ctx)
}

func (c *Controller) handleAppendPyClass(ctx context.Context, m *ctrlpb.AppendPyClass) error {
	panic("not implemented")
}

func (c *Controller) handleAppendClassMethodArg(ctx context.Context, m *ctrlpb.AppendClassMethodArg) error {
	panic("not implemented")
}

func (c *Controller) handleMarkDAGNodeDone(ctx context.Context, m *ctrlpb.MarkDAGNodeDone) error {
	return nil
}
func (c *Controller) handleRequestObject(ctx context.Context, m *ctrlpb.RequestObject) error {
	logrus.WithFields(logrus.Fields{"id": m.ID, "source": m.Source}).Info("control: request object")
	object, err := c.storeService.GetObject(ctx, &commonpb.ObjectRef{
		ID:     m.ID,
		Source: m.Source,
	})
	if err != nil {
		logrus.Errorf("Failed to get object: %v", err)
		return err
	}
	ret := ctrlpb.NewResponseObject(m.ID, object, nil)
	return c.PushToClient(ctx, ret)
}

func (c *Controller) emit(ctx context.Context, event Event) {
	if c.events == nil {
		return
	}
	c.events.Publish(ctx, event)
}

func (c *Controller) PushToClient(ctx context.Context, msg *ctrlpb.Message) error {
	if msg == nil {
		return errors.New("push message: nil payload")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.toClientChan <- msg:
		return nil
	}
}

func (c *Controller) getOrCreateRuntime(name, sessionId, instanceId string) (*task.Runtime, error) {
	runtimeId := fmt.Sprintf("%s::%s::%s", name, sessionId, instanceId)
	rt, ok := c.runtimes[runtimeId]
	if !ok {
		node, ok := c.nodes[name]
		if !ok {
			return nil, fmt.Errorf("actor node %s not found", name)
		}

		rt = node.Runtime(runtimeId)
		c.runtimes[runtimeId] = rt
	}

	return rt, nil
}
