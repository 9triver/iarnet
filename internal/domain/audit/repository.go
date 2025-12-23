package audit

import (
	"context"

	"github.com/9triver/iarnet/internal/domain/audit/types"
)

// Repository 操作日志仓库接口
type Repository interface {
	// SaveOperation 保存操作日志
	SaveOperation(ctx context.Context, log *types.OperationLog) error

	// GetOperations 查询操作日志
	GetOperations(ctx context.Context, options *types.QueryOptions) ([]*types.OperationLog, error)

	// Close 关闭仓库
	Close() error
}
