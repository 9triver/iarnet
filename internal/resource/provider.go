package resource

import "context"

type Provider interface {
	GetCapacity(ctx context.Context) (*Capacity, error)
	GetRealTimeUsage(ctx context.Context) (*Usage, error)
	GetProviderType() string
	GetProviderID() string
}
