package audit

import (
	"context"

	"github.com/9triver/iarnet/internal/domain/audit/types"
)

// Manager 操作日志管理器
type Manager struct {
	svc Service
}

// NewManager 创建操作日志管理器
func NewManager(svc Service) *Manager {
	return &Manager{
		svc: svc,
	}
}

// RecordOperation 记录操作日志
func (m *Manager) RecordOperation(ctx context.Context, log *types.OperationLog) error {
	return m.svc.RecordOperation(ctx, log)
}

// GetOperations 查询操作日志
func (m *Manager) GetOperations(ctx context.Context, options *types.QueryOptions) (*types.QueryResult, error) {
	return m.svc.GetOperations(ctx, options)
}
