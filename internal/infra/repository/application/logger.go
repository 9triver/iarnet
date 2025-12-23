package application

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/9triver/iarnet/internal/config"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// LogEntryDAO - 数据访问对象
// ============================================================================

// LogEntryDAO 日志条目数据访问对象
// 用于数据库持久化，只保存日志的基本信息
// DAO 层只定义数据结构，不依赖领域对象或 proto 类型
type LogEntryDAO struct {
	ID            string    `db:"id"`
	ApplicationID string    `db:"application_id"`
	Timestamp     time.Time `db:"timestamp"`
	Level         string    `db:"level"` // 存储为字符串：trace, debug, info, warn, error, fatal, panic
	Message       string    `db:"message"`
	Fields        string    `db:"fields"` // JSON 编码的字段数组
	CallerFile    string    `db:"caller_file"`
	CallerLine    int       `db:"caller_line"`
	CallerFunc    string    `db:"caller_func"`
	CreatedAt     time.Time `db:"created_at"`
}

// ============================================================================
// LoggerRepo - 接口定义
// ============================================================================

// LoggerRepo 日志仓库接口
// 直接使用 DAO 类型，不依赖领域对象或 proto 类型
type LoggerRepo interface {
	SaveLog(ctx context.Context, dao *LogEntryDAO) error
	BatchSaveLogs(ctx context.Context, daos []*LogEntryDAO) error
	GetLogs(ctx context.Context, applicationID string, limit, offset int) ([]*LogEntryDAO, error)
	GetLogsByTimeRange(ctx context.Context, applicationID string, startTime, endTime time.Time, limit int) ([]*LogEntryDAO, error)
	Close() error
}

// ============================================================================
// LoggerRepoSQLite - SQLite 实现
// ============================================================================

// loggerRepoSQLite SQLite 实现的 LoggerRepo
type loggerRepoSQLite struct {
	db *sql.DB
}

// NewLoggerRepoSQLite 创建基于 SQLite 的 LoggerRepo
func NewLoggerRepoSQLite(dbPath string, cfg *config.Config) (LoggerRepo, error) {
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

	repo := &loggerRepoSQLite{
		db: db,
	}

	// 初始化表结构
	if err := repo.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logrus.Infof("Logger repository initialized with SQLite at %s", dbPath)
	return repo, nil
}

// initSchema 初始化数据库表结构
func (r *loggerRepoSQLite) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS application_logs (
		id TEXT PRIMARY KEY,
		application_id TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		level TEXT NOT NULL,
		message TEXT NOT NULL,
		fields TEXT,  -- JSON 编码的字段数组
		caller_file TEXT,
		caller_line INTEGER,
		caller_func TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_application_logs_app_id ON application_logs(application_id);
	CREATE INDEX IF NOT EXISTS idx_application_logs_timestamp ON application_logs(application_id, timestamp);
	CREATE INDEX IF NOT EXISTS idx_application_logs_level ON application_logs(application_id, level);
	`

	if _, err := r.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// Close 关闭数据库连接
func (r *loggerRepoSQLite) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// SaveLog 保存单条日志
func (r *loggerRepoSQLite) SaveLog(ctx context.Context, dao *LogEntryDAO) error {
	// 设置时间戳
	now := time.Now()
	if dao.CreatedAt.IsZero() {
		dao.CreatedAt = now
	}

	query := `
		INSERT INTO application_logs (
			id, application_id, timestamp, level, message, 
			fields, caller_file, caller_line, caller_func, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		dao.ID,
		dao.ApplicationID,
		dao.Timestamp,
		dao.Level,
		dao.Message,
		dao.Fields,
		dao.CallerFile,
		dao.CallerLine,
		dao.CallerFunc,
		dao.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save log: %w", err)
	}

	return nil
}

// BatchSaveLogs 批量保存日志
func (r *loggerRepoSQLite) BatchSaveLogs(ctx context.Context, daos []*LogEntryDAO) error {
	if len(daos) == 0 {
		return nil
	}

	// 使用事务批量插入
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO application_logs (
			id, application_id, timestamp, level, message, 
			fields, caller_file, caller_line, caller_func, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, dao := range daos {
		if dao.CreatedAt.IsZero() {
			dao.CreatedAt = now
		}

		_, err := stmt.ExecContext(ctx,
			dao.ID,
			dao.ApplicationID,
			dao.Timestamp,
			dao.Level,
			dao.Message,
			dao.Fields,
			dao.CallerFile,
			dao.CallerLine,
			dao.CallerFunc,
			dao.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert log entry: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logrus.Debugf("Batch saved %d log entries", len(daos))
	return nil
}

// GetLogs 获取日志列表
func (r *loggerRepoSQLite) GetLogs(ctx context.Context, applicationID string, limit, offset int) ([]*LogEntryDAO, error) {
	query := `
		SELECT id, application_id, timestamp, level, message, 
		       fields, caller_file, caller_line, caller_func, created_at
		FROM application_logs
		WHERE application_id = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, applicationID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var daos []*LogEntryDAO
	for rows.Next() {
		var dao LogEntryDAO
		err := rows.Scan(
			&dao.ID,
			&dao.ApplicationID,
			&dao.Timestamp,
			&dao.Level,
			&dao.Message,
			&dao.Fields,
			&dao.CallerFile,
			&dao.CallerLine,
			&dao.CallerFunc,
			&dao.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}
		daos = append(daos, &dao)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating logs: %w", err)
	}

	return daos, nil
}

// GetLogsByTimeRange 根据时间范围获取日志
func (r *loggerRepoSQLite) GetLogsByTimeRange(ctx context.Context, applicationID string, startTime, endTime time.Time, limit int) ([]*LogEntryDAO, error) {
	logrus.Debugf("GetLogsByTimeRange: applicationID=%s, startTime=%v (Local), endTime=%v (Local), limit=%d",
		applicationID, startTime.Local(), endTime.Local(), limit)

	query := `
		SELECT id, application_id, timestamp, level, message, 
		       fields, caller_file, caller_line, caller_func, created_at
		FROM application_logs
		WHERE application_id = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp DESC
	`

	var rows *sql.Rows
	var err error
	if limit > 0 {
		query += " LIMIT ?"
		rows, err = r.db.QueryContext(ctx, query, applicationID, startTime, endTime, limit)
	} else {
		// limit 为 0 时，不添加 LIMIT 子句，返回全部日志
		rows, err = r.db.QueryContext(ctx, query, applicationID, startTime, endTime)
	}
	if err != nil {
		logrus.Errorf("Failed to query logs by time range: %v, query: %s, args: applicationID=%s, startTime=%v, endTime=%v",
			err, query, applicationID, startTime, endTime)
		return nil, fmt.Errorf("failed to query logs by time range: %w", err)
	}
	defer rows.Close()

	var daos []*LogEntryDAO
	for rows.Next() {
		var dao LogEntryDAO
		err := rows.Scan(
			&dao.ID,
			&dao.ApplicationID,
			&dao.Timestamp,
			&dao.Level,
			&dao.Message,
			&dao.Fields,
			&dao.CallerFile,
			&dao.CallerLine,
			&dao.CallerFunc,
			&dao.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}
		daos = append(daos, &dao)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating logs: %w", err)
	}

	logrus.Debugf("GetLogsByTimeRange: found %d logs for applicationID=%s", len(daos), applicationID)
	if len(daos) > 0 {
		logrus.Debugf("First log timestamp: %v (Local), Last log timestamp: %v (Local)",
			daos[0].Timestamp.Local(), daos[len(daos)-1].Timestamp.Local())
	} else {
		logrus.Debugf("No logs found for applicationID=%s in time range [%v, %v]",
			applicationID, startTime.Local(), endTime.Local())
	}

	return daos, nil
}
