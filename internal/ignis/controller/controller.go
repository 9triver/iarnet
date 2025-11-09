package controller

import (
	"context"
	"errors"

	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/resource/component"
	"github.com/sirupsen/logrus"
)

type Controller struct {
	appID            string
	events           *EventHub
	responseCh       chan *ctrlpb.Message
	componentService component.ComponentService
}

const defaultOutboundBuffer = 32

func NewController(componentService component.ComponentService, appID string, events *EventHub) *Controller {
	return &Controller{
		appID:            appID,
		events:           events,
		responseCh:       make(chan *ctrlpb.Message, defaultOutboundBuffer),
		componentService: componentService,
	}
}

func (c *Controller) AppID() string { return c.appID }

func (c *Controller) HandleMessage(ctx context.Context, msg *ctrlpb.Message) error {
	switch m := msg.GetCommand().(type) {
	case *ctrlpb.Message_AppendPyFunc:
		return c.handleAppendPyFunc(ctx, m.AppendPyFunc)
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

func (c *Controller) handleAppendPyFunc(ctx context.Context, m *ctrlpb.AppendPyFunc) error {
	replicas := int(m.GetReplicas())
	for i := 0; i < replicas; i++ {
		logrus.Infof("Deploying component %d", i)
		containerRef, err := c.componentService.DeployComponent(
			ctx, resource.RuntimeEnvPython,
			&resource.ResourceRequest{},
		)
		if err != nil {
			logrus.Errorf("Failed to deploy component: %v", err)
			return err
		}
		logrus.Infof("Component deployed successfully: %s", containerRef.ID)
	}
	return nil
}
func (c *Controller) handleAppendPyClass(ctx context.Context, m *ctrlpb.AppendPyClass) error {
	return nil
}
func (c *Controller) handleAppendArg(ctx context.Context, m *ctrlpb.AppendArg) error { return nil }
func (c *Controller) handleAppendClassMethodArg(ctx context.Context, m *ctrlpb.AppendClassMethodArg) error {
	return nil
}
func (c *Controller) handleInvoke(ctx context.Context, m *ctrlpb.Invoke) error { return nil }
func (c *Controller) handleMarkDAGNodeDone(ctx context.Context, m *ctrlpb.MarkDAGNodeDone) error {
	c.emit(ctx, &DAGNodeStatusChangedEvent{
		AppID:     c.appID,
		NodeID:    m.GetNodeId(),
		SessionID: m.GetSessionId(),
		Status:    DAGNodeStatusDone,
	})
	return nil
}
func (c *Controller) handleRequestObject(ctx context.Context, m *ctrlpb.RequestObject) error {
	return nil
}

func (c *Controller) emit(ctx context.Context, event Event) {
	if c.events == nil {
		return
	}
	c.events.Publish(ctx, event)
}

func (c *Controller) ResponseChan() <-chan *ctrlpb.Message {
	return c.responseCh
}

func (c *Controller) PushResponse(ctx context.Context, msg *ctrlpb.Message) error {
	if msg == nil {
		return errors.New("push message: nil payload")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.responseCh <- msg:
		return nil
	}
}
