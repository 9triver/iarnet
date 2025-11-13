package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/9triver/iarnet/internal/ignis/task"
	ignispb "github.com/9triver/iarnet/internal/proto/ignis"
	clusterpb "github.com/9triver/iarnet/internal/proto/ignis/cluster"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	storepb "github.com/9triver/iarnet/internal/proto/resource/store"
	"github.com/9triver/iarnet/internal/resource/component"
	"github.com/9triver/iarnet/internal/resource/store"
	resourceTypes "github.com/9triver/iarnet/internal/resource/types"
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

func (c *Controller) handleInvokeResponse(ctx context.Context, m *ignispb.InvokeResponse) error {
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

func (c *Controller) HandleComponentMessage(ctx context.Context, msg *clusterpb.Message) error {
	switch m := msg.GetMessage().(type) {
	case *clusterpb.Message_InvokeResponse:
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
		resp, err := c.storeService.SaveObject(ctx, &storepb.SaveObjectRequest{
			Object: &storepb.EncodedObject{
				ID:       obj.ID,
				Language: storepb.Language(obj.Language),
				Data:     obj.Data,
				IsStream: obj.Stream,
			},
		})
		if err != nil {
			logrus.Errorf("Failed to save object: %v", err)
			return
		}
		if !resp.Success {
			logrus.Errorf("Failed to save object: %s", resp.Error)
			return
		}
		logrus.Infof("Object saved successfully: %s", resp.ObjectRef.ID)
		ret := ctrlpb.NewReturnResult(m.SessionID, "", resp.ObjectRef.ID, &ignispb.Flow{
			ID: resp.ObjectRef.ID,
			Source: &ignispb.StoreRef{
				ID: resp.ObjectRef.Source,
			},
		}, nil)
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

		component.Send(clusterpb.NewMessage(&clusterpb.Function{
			Name:          m.GetName(),
			Params:        m.GetParams(),
			Requirements:  m.GetRequirements(),
			PickledObject: m.GetPickledObject(),
			Language:      m.GetLanguage(),
		}))
		logrus.Infof("Function sent to component: %s", component.GetID())

		go func() {
			for {
				msg := component.Receive(ctx)
				if msg == nil {
					return
				}
				c.HandleComponentMessage(ctx, msg)
			}
		}()

		actorGroup.Push(task.NewActor(actorName, component))
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
			resp, err := c.storeService.SaveObject(ctx, &storepb.SaveObjectRequest{
				Object: &storepb.EncodedObject{
					ID:       v.Encoded.ID,
					Language: storepb.Language(v.Encoded.Language),
					Data:     v.Encoded.Data,
					IsStream: v.Encoded.Stream,
				},
			})
			if err != nil {
				logrus.Errorf("Failed to save object: %v", err)
				ret := ctrlpb.NewReturnResult(m.SessionID, m.InstanceID, m.Name, nil, err)
				c.PushToClient(ctx, ret)
				return
			}
			logrus.Infof("Object saved successfully: %s", resp.ObjectRef.ID)
			if err = rt.AddArg(m.Param, &ignispb.Flow{
				ID: resp.ObjectRef.ID,
				Source: &ignispb.StoreRef{
					ID: resp.ObjectRef.Source,
				},
			}); err != nil {
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
	source := m.Source
	object, err := c.storeService.GetObject(ctx, &storepb.GetObjectRequest{
		ObjectRef: &storepb.ObjectRef{
			ID:     m.ID,
			Source: source,
		},
	})
	if err != nil {
		logrus.Errorf("Failed to get object: %v", err)
		return err
	}
	if object == nil {
		logrus.Errorf("Object not found: %s", source)
		return fmt.Errorf("object not found: %s", source)
	}
	ret := ctrlpb.NewResponseObject(m.ID, &ignispb.EncodedObject{
		ID:       object.Object.ID,
		Language: ignispb.Language(object.Object.Language),
		Data:     object.Object.Data,
		Stream:   object.Object.IsStream,
	}, nil)
	c.PushToClient(ctx, ret)
	return nil
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
