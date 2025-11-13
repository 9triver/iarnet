package types

import (
	"context"

	actorpb "github.com/9triver/iarnet/internal/proto/execution_ignis/actor"
)

type RuntimeEnv = string

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
	Notify(ctx context.Context, message *actorpb.Message) error
}

type Consumer interface {
	Consume(ctx context.Context, message *actorpb.Message) error
}

type ConsumerSupplier interface {
	GetConsumers() ([]Consumer, error)
}

type Envelope struct {
	ComponentID string
	Message     *actorpb.Message
}

type ObjectID = string

type StoreID = string
