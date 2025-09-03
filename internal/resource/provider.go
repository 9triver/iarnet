package resource

import "context"

type Provider interface {
	GetCapacity(ctx context.Context) (*Capacity, error)
	GetProviderType() string
	GetProviderID() string
}
