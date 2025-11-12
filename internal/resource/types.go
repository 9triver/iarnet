package resource

import (
	"context"

	"github.com/9triver/iarnet/internal/ignis/objects"
	proto "github.com/9triver/iarnet/internal/proto/ignis"
	clusterpb "github.com/9triver/iarnet/internal/proto/ignis/cluster"
	"github.com/asynkron/protoactor-go/actor"
)

type RuntimeEnv string

type Info struct {
	CPU    int64 `json:"cpu"`    // millicores
	Memory int64 `json:"memory"` // bytes
	GPU    int64 `json:"gpu"`
}

type Capacity struct {
	Total     *Info `json:"total"`
	Used      *Info `json:"used"`
	Available *Info `json:"available"`
}

const (
	RuntimeEnvPython RuntimeEnv = "python"
)

type ResourceRequest Info

type ProviderType string

type ProviderStatus int32

const (
	ProviderStatusUnknown      ProviderStatus = 0
	ProviderStatusConnected    ProviderStatus = 1
	ProviderStatusDisconnected ProviderStatus = 2
)

type Subscriber interface {
	Notify(ctx context.Context, message *clusterpb.Message) error
}

type Consumer interface {
	Consume(ctx context.Context, message *clusterpb.Message) error
}

type ConsumerSupplier interface {
	GetConsumers() ([]Consumer, error)
}

type Envelope struct {
	ComponentID string
	Message     *clusterpb.Message
}

// SaveObject is sent to store when actor generates new return objects from functions
type SaveObjectRequest struct {
	Value    objects.Interface                        // object or stream
	Callback func(ctx actor.Context, ref *proto.Flow) // called when object saving is completed
}

type ObjectID = string
