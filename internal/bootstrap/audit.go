package bootstrap

import (
	"fmt"
	"path/filepath"

	"github.com/9triver/iarnet/internal/domain/audit"
	auditrepo "github.com/9triver/iarnet/internal/infra/repository/audit"
	"github.com/sirupsen/logrus"
)

// bootstrapAudit 初始化操作日志模块
func bootstrapAudit(iarnet *Iarnet) error {
	// 构建操作日志数据库路径
	dbPath := iarnet.Config.Database.OperationLogDBPath
	if dbPath == "" {
		dbPath = filepath.Join(iarnet.Config.DataDir, "operation_logs.db")
	}

	// 创建操作日志仓库
	repo, err := auditrepo.NewOperationLogRepoSQLite(dbPath, iarnet.Config)
	if err != nil {
		return fmt.Errorf("failed to create operation log repository: %w", err)
	}

	// 创建操作日志服务
	svc := audit.NewService(repo)

	// 创建操作日志管理器
	iarnet.AuditManager = audit.NewManager(svc)

	logrus.Info("Audit module initialized")
	return nil
}
