package controller

import (
	"context"
)

// Controller 聚合根：负责单个应用的会话绑定与消息处理生命周期。
type Controller struct {
	appID string
}

func NewController(appID string) *Controller {
	return &Controller{appID: appID}
}

func (c *Controller) AppID() string { return c.appID }

// Start 可做预热/注册等，最小实现直接返回。
func (c *Controller) Start(ctx context.Context) error { return nil }

// Stop 做资源回收/取消订阅。
func (c *Controller) Stop(ctx context.Context) error { return nil }

// HandleMessage 领域入口：将上行消息转换为领域动作。
func (c *Controller) HandleMessage(ctx context.Context, msg Message) error {
	switch m := msg.(type) {
	case *RegisterRequest:
		return c.handleRegister(ctx, m)
	case *AppendPyFunc:
		return c.handleAppendPyFunc(ctx, m)
	case *AppendPyClass:
		return c.handleAppendPyClass(ctx, m)
	case *AppendArg:
		return c.handleAppendArg(ctx, m)
	case *AppendClassMethodArg:
		return c.handleAppendClassMethodArg(ctx, m)
	case *Invoke:
		return c.handleInvoke(ctx, m)
	case *MarkDAGNodeDone:
		return c.handleMarkDAGNodeDone(ctx, m)
	case *RequestObject:
		return c.handleRequestObject(ctx, m)
	default:
		return nil
	}
}

func (c *Controller) handleRegister(ctx context.Context, m *RegisterRequest) error    { return nil }
func (c *Controller) handleAppendPyFunc(ctx context.Context, m *AppendPyFunc) error   { return nil }
func (c *Controller) handleAppendPyClass(ctx context.Context, m *AppendPyClass) error { return nil }
func (c *Controller) handleAppendArg(ctx context.Context, m *AppendArg) error         { return nil }
func (c *Controller) handleAppendClassMethodArg(ctx context.Context, m *AppendClassMethodArg) error {
	return nil
}
func (c *Controller) handleInvoke(ctx context.Context, m *Invoke) error { return nil }
func (c *Controller) handleMarkDAGNodeDone(ctx context.Context, m *MarkDAGNodeDone) error {
	return nil
}
func (c *Controller) handleRequestObject(ctx context.Context, m *RequestObject) error { return nil }
