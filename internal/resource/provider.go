package resource

import (
	"context"
	"time"
)

type Status int32

const (
	StatusUnknown      Status = 0
	StatusConnected    Status = 1
	StatusDisconnected Status = 2
)

type Provider interface {
	GetCapacity(ctx context.Context) (*Capacity, error)
	GetType() string
	GetID() string
	GetName() string
	GetHost() string
	GetPort() int
	GetLastUpdateTime() time.Time
	GetStatus() Status
	Deploy(ctx context.Context, spec ContainerSpec) (string, error)
	GetLogs(d string, lines int) ([]string, error)
}
