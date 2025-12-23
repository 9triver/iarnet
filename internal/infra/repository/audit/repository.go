package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/audit"
	audittypes "github.com/9triver/iarnet/internal/domain/audit/types"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// OperationLogDAO - 数据访问对象
// ============================================================================

// OperationLogDAO 操作日志数据访问对象
type OperationLogDAO struct {
	ID           string    `db:"id"`
	User         string    `db:"user"`
	Operation    string    `db:"operation"`
	ResourceID   string    `db:"resource_id"`
	ResourceType string    `db:"resource_type"`
	Action       string    `db:"action"`
	Before       string    `db:"before"` // JSON 编码
	After        string    `db:"after"`  // JSON 编码
	Timestamp    time.Time `db:"timestamp"`
	IP           string    `db:"ip"`
	CreatedAt    time.Time `db:"created_at"`
}

// ============================================================================
// OperationLogRepoSQLite - SQLite 实现
// ============================================================================

// operationLogRepoSQLite SQLite 实现的操作日志仓库
type operationLogRepoSQLite struct {
	db *sql.DB
}

// NewOperationLogRepoSQLite 创建基于 SQLite 的操作日志仓库
func NewOperationLogRepoSQLite(dbPath string, cfg *config.Config) (audit.Repository, error) {
	// 确保数据库目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// 打开数据库连接
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=1&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数
	if cfg != nil {
		db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
		db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
		if cfg.Database.ConnMaxLifetimeSeconds > 0 {
			db.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetimeSeconds) * time.Second)
		}
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &operationLogRepoSQLite{
		db: db,
	}

	// 初始化表结构
	if err := repo.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logrus.Infof("Operation log repository initialized with SQLite at %s", dbPath)
	return repo, nil
}

// initSchema 初始化数据库表结构
func (r *operationLogRepoSQLite) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS operation_logs (
		id TEXT PRIMARY KEY,
		user TEXT NOT NULL,
		operation TEXT NOT NULL,
		resource_id TEXT,
		resource_type TEXT,
		action TEXT NOT NULL,
		before TEXT,  -- JSON 编码的操作前状态
		after TEXT,   -- JSON 编码的操作后状态
		timestamp DATETIME NOT NULL,
		ip TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_operation_logs_user ON operation_logs(user);
	CREATE INDEX IF NOT EXISTS idx_operation_logs_operation ON operation_logs(operation);
	CREATE INDEX IF NOT EXISTS idx_operation_logs_resource ON operation_logs(resource_type, resource_id);
	CREATE INDEX IF NOT EXISTS idx_operation_logs_timestamp ON operation_logs(timestamp);
	`

	if _, err := r.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// Close 关闭数据库连接
func (r *operationLogRepoSQLite) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// SaveOperation 保存操作日志
func (r *operationLogRepoSQLite) SaveOperation(ctx context.Context, log *audittypes.OperationLog) error {
	dao := &OperationLogDAO{
		ID:           log.ID,
		User:         log.User,
		Operation:    string(log.Operation),
		ResourceID:   log.ResourceID,
		ResourceType: log.ResourceType,
		Action:       log.Action,
		Timestamp:    log.Timestamp,
		IP:           log.IP,
	}

	// 序列化 Before 和 After
	if log.Before != nil {
		beforeJSON, err := json.Marshal(log.Before)
		if err != nil {
			return fmt.Errorf("failed to marshal before: %w", err)
		}
		dao.Before = string(beforeJSON)
	}

	if log.After != nil {
		afterJSON, err := json.Marshal(log.After)
		if err != nil {
			return fmt.Errorf("failed to marshal after: %w", err)
		}
		dao.After = string(afterJSON)
	}

	// 设置时间戳
	now := time.Now()
	if dao.Timestamp.IsZero() {
		dao.Timestamp = now
	}
	if dao.CreatedAt.IsZero() {
		dao.CreatedAt = now
	}

	query := `
		INSERT INTO operation_logs (
			id, user, operation, resource_id, resource_type, action,
			before, after, timestamp, ip, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		dao.ID,
		dao.User,
		dao.Operation,
		dao.ResourceID,
		dao.ResourceType,
		dao.Action,
		dao.Before,
		dao.After,
		dao.Timestamp,
		dao.IP,
		dao.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert operation log: %w", err)
	}

	return nil
}

// GetOperations 查询操作日志
func (r *operationLogRepoSQLite) GetOperations(ctx context.Context, options *audittypes.QueryOptions) ([]*audittypes.OperationLog, error) {
	query := `
		SELECT id, user, operation, resource_id, resource_type, action,
		       before, after, timestamp, ip, created_at
		FROM operation_logs
		WHERE 1=1
	`
	args := []interface{}{}

	// 构建查询条件
	if options.StartTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, *options.StartTime)
	}
	if options.EndTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, *options.EndTime)
	}
	if options.User != "" {
		query += " AND user = ?"
		args = append(args, options.User)
	}
	if options.Operation != "" {
		query += " AND operation = ?"
		args = append(args, string(options.Operation))
	}
	if options.ResourceID != "" {
		query += " AND resource_id = ?"
		args = append(args, options.ResourceID)
	}

	// 排序：最新的在前
	query += " ORDER BY timestamp DESC"

	// 限制数量（如果指定了limit，查询limit+1条，用于判断是否有更多数据）
	limit := options.Limit
	if limit <= 0 {
		limit = 100
	}

	// 添加 offset
	if options.Offset > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit+1, options.Offset) // 多查询一条用于判断hasMore
	} else {
		query += " LIMIT ?"
		args = append(args, limit+1) // 多查询一条用于判断hasMore
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query operation logs: %w", err)
	}
	defer rows.Close()

	var logs []*audittypes.OperationLog
	for rows.Next() {
		var dao OperationLogDAO
		err := rows.Scan(
			&dao.ID,
			&dao.User,
			&dao.Operation,
			&dao.ResourceID,
			&dao.ResourceType,
			&dao.Action,
			&dao.Before,
			&dao.After,
			&dao.Timestamp,
			&dao.IP,
			&dao.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan operation log: %w", err)
		}

		log := &audittypes.OperationLog{
			ID:           dao.ID,
			User:         dao.User,
			Operation:    audittypes.OperationType(dao.Operation),
			ResourceID:   dao.ResourceID,
			ResourceType: dao.ResourceType,
			Action:       dao.Action,
			Timestamp:    dao.Timestamp,
			IP:           dao.IP,
		}

		// 反序列化 Before 和 After
		if dao.Before != "" {
			var before map[string]interface{}
			if err := json.Unmarshal([]byte(dao.Before), &before); err == nil {
				log.Before = before
			}
		}

		if dao.After != "" {
			var after map[string]interface{}
			if err := json.Unmarshal([]byte(dao.After), &after); err == nil {
				log.After = after
			}
		}

		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating operation logs: %w", err)
	}

	return logs, nil
}
