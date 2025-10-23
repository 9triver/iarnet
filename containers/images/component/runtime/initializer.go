package runtime

import (
	"context"

	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
)

type Initializer interface {
	Initialize(ctx context.Context, fn *cluster.Function, addr string, connId string) error
	Cleanup(ctx context.Context) error
	Language() proto.Language
}
