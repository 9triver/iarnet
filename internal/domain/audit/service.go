package audit

import (
	"context"
	"time"

	"github.com/9triver/iarnet/internal/domain/audit/types"
)

// Service 操作日志服务接口
type Service interface {
	// RecordOperation 记录操作日志
	RecordOperation(ctx context.Context, log *types.OperationLog) error

	// GetOperations 查询操作日志
	GetOperations(ctx context.Context, options *types.QueryOptions) (*types.QueryResult, error)
}

// service 操作日志服务实现
type service struct {
	repo Repository
}

// NewService 创建操作日志服务
func NewService(repo Repository) Service {
	return &service{
		repo: repo,
	}
}

// RecordOperation 记录操作日志
func (s *service) RecordOperation(ctx context.Context, log *types.OperationLog) error {
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}
	return s.repo.SaveOperation(ctx, log)
}

// GetOperations 查询操作日志
func (s *service) GetOperations(ctx context.Context, options *types.QueryOptions) (*types.QueryResult, error) {
	if options == nil {
		options = &types.QueryOptions{}
	}
	if options.Limit <= 0 {
		options.Limit = 100
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	// 创建一个临时选项用于查询，limit+1用于判断是否有更多数据
	queryOptions := *options
	queryOptions.Limit = options.Limit + 1

	logs, err := s.repo.GetOperations(ctx, &queryOptions)
	if err != nil {
		return nil, err
	}

	// 判断是否有更多数据
	hasMore := len(logs) > options.Limit

	// 如果返回的日志数量超过limit，截取limit条
	if len(logs) > options.Limit {
		logs = logs[:options.Limit]
	}

	return &types.QueryResult{
		Logs:    logs,
		Total:   len(logs), // 注意：这里返回的是实际返回的数量，不是总数
		HasMore: hasMore,
	}, nil
}
