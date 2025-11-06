package transport

import (
	"context"

	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
)

// SessionStream 抽象 gRPC 双向流，屏蔽具体传输实现。
type SessionStream interface {
	Context() context.Context
	Send(*ctrlpb.Message) error
	Recv() (*ctrlpb.Message, error)
}

// SessionHandler 处理一次会话生命周期。
type SessionHandler func(stream SessionStream) error

// SessionServer 负责监听并在有新会话时调用注册的 handler。
type SessionServer interface {
	OnSession(handler SessionHandler)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Envelope 与 Messenger 是对 actor/执行侧通信的最小抽象，占位实现可后续替换。
type Envelope struct {
	CorrelationID string
	Payload       []byte
	Metadata      map[string]string
}

type Messenger interface {
	Send(ctx context.Context, addr string, env Envelope) error
	Request(ctx context.Context, addr string, env Envelope) (Envelope, error)
}
