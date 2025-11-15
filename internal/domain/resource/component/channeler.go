package component

import "context"

// Channeler 定义组件通信通道接口
// 领域层不依赖具体的传输实现，只依赖此接口
type Channeler interface {
	// StartReceiver 启动消息接收器，当收到消息时调用 onMessage 回调
	// componentID: 组件 ID
	// data: 消息数据
	StartReceiver(ctx context.Context, onMessage func(componentID string, data []byte))

	// Send 向指定组件发送消息
	// componentID: 目标组件 ID
	// data: 消息数据
	Send(componentID string, data []byte)

	// Close 关闭通道并释放资源
	Close() error
}

type nullChanneler struct{}

func NewNullChanneler() Channeler {
	return &nullChanneler{}
}

func (n *nullChanneler) StartReceiver(ctx context.Context, onMessage func(componentID string, data []byte)) {
	panic("channeler is not initialized")
}

func (n *nullChanneler) Send(componentID string, data []byte) {
	panic("channeler is not initialized")
}

func (n *nullChanneler) Close() error {
	panic("channeler is not initialized")
}
