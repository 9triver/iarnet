package controller

import (
	"context"
	"errors"

	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
)

// Controller 聚合根：负责单个应用的会话绑定与消息处理生命周期。
type Controller struct {
	appID      string
	events     *EventHub
	responseCh chan *ctrlpb.Message
}

const defaultOutboundBuffer = 32

func NewController(appID string, events *EventHub) *Controller {
	return &Controller{
		appID:      appID,
		events:     events,
		responseCh: make(chan *ctrlpb.Message, defaultOutboundBuffer),
	}
}

func (c *Controller) AppID() string { return c.appID }

// Start 可做预热/注册等，最小实现直接返回。
func (c *Controller) Start(ctx context.Context) error { return nil }

// Stop 做资源回收/取消订阅。
func (c *Controller) Stop(ctx context.Context) error { return nil }

// HandleMessage 领域入口：将上行消息转换为领域动作。
func (c *Controller) HandleMessage(ctx context.Context, msg *ctrlpb.Message) error {
	switch m := msg.GetCommand().(type) {
	case *ctrlpb.Message_RegisterRequest:
		return c.handleRegister(ctx, m.RegisterRequest)
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

func (c *Controller) handleRegister(ctx context.Context, m *ctrlpb.RegisterRequest) error { return nil }
func (c *Controller) handleAppendPyFunc(ctx context.Context, m *ctrlpb.AppendPyFunc) error {
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

// ResponseChan 返回向客户端发送消息的通道视图。
func (c *Controller) ResponseChan() <-chan *ctrlpb.Message {
	return c.responseCh
}

// PushResponse 将消息放入客户端发送队列。
func (c *Controller) PushResponse(ctx context.Context, msg *ctrlpb.Message) error {
	if msg == nil {
		return errors.New("push message: nil payload")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.responseCh <- msg:
		return nil
	}
}

var errSessionAlreadyActive = errors.New("controller session already active")
