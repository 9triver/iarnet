package resource

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
// ProviderDAO - 数据访问对象
// ============================================================================

// ProviderDAO Provider 数据访问对象
// 用于数据库持久化，只保存基本信息
// DAO 层只定义数据结构，不依赖领域对象
type ProviderDAO struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Host      string    `db:"host"`
	Port      int       `db:"port"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// ============================================================================
// ProviderRepo - 接口定义
// ============================================================================

// ProviderRepo Provider 仓库接口
// 直接使用 DAO 类型，不依赖领域对象
type ProviderRepo interface {
	Create(ctx context.Context, dao *ProviderDAO) error
	Update(ctx context.Context, dao *ProviderDAO) error
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (*ProviderDAO, error)
	GetAll(ctx context.Context) ([]*ProviderDAO, error)
}

// ============================================================================
// ProviderRepoSQLite - SQLite 实现
// ============================================================================

// providerRepoSQLite SQLite 实现的 ProviderRepo
type providerRepoSQLite struct {
	db  *sql.DB
	cfg *config.Config
}

// NewProviderRepoSQLite 创建基于 SQLite 的 ProviderRepo
func NewProviderRepoSQLite(dbPath string, cfg *config.Config) (ProviderRepo, error) {
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

	// 测试连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &providerRepoSQLite{
		db:  db,
		cfg: cfg,
	}

	// 初始化表结构
	if err := repo.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logrus.Infof("Provider repository initialized with SQLite at %s", dbPath)
	return repo, nil
}

// initSchema 初始化数据库表结构
func (r *providerRepoSQLite) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS providers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		host TEXT NOT NULL,
		port INTEGER NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_providers_host_port ON providers(host, port);
	`

	if _, err := r.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// Close 关闭数据库连接
func (r *providerRepoSQLite) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// Create 创建 Provider
func (r *providerRepoSQLite) Create(ctx context.Context, dao *ProviderDAO) error {
	// 设置时间戳
	now := time.Now()
	if dao.CreatedAt.IsZero() {
		dao.CreatedAt = now
	}
	if dao.UpdatedAt.IsZero() {
		dao.UpdatedAt = now
	}

	query := `
		INSERT INTO providers (id, name, host, port, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		dao.ID,
		dao.Name,
		dao.Host,
		dao.Port,
		dao.CreatedAt,
		dao.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	logrus.Debugf("Provider %s created in database", dao.ID)
	return nil
}

// Update 更新 Provider
func (r *providerRepoSQLite) Update(ctx context.Context, dao *ProviderDAO) error {
	// 设置时间戳
	now := time.Now()
	if dao.UpdatedAt.IsZero() {
		dao.UpdatedAt = now
	}

	query := `
		UPDATE providers
		SET name = ?, host = ?, port = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		dao.Name,
		dao.Host,
		dao.Port,
		dao.UpdatedAt,
		dao.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update provider: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("provider %s not found", dao.ID)
	}

	logrus.Debugf("Provider %s updated in database", dao.ID)
	return nil
}

// Delete 删除 Provider
func (r *providerRepoSQLite) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM providers WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete provider: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("provider %s not found", id)
	}

	logrus.Debugf("Provider %s deleted from database", id)
	return nil
}

// Get 获取指定 ID 的 Provider
func (r *providerRepoSQLite) Get(ctx context.Context, id string) (*ProviderDAO, error) {
	query := `
		SELECT id, name, host, port, created_at, updated_at
		FROM providers
		WHERE id = ?
	`

	var dao ProviderDAO
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&dao.ID,
		&dao.Name,
		&dao.Host,
		&dao.Port,
		&dao.CreatedAt,
		&dao.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("provider %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	return &dao, nil
}

// GetAll 获取所有 Provider
func (r *providerRepoSQLite) GetAll(ctx context.Context) ([]*ProviderDAO, error) {
	query := `
		SELECT id, name, host, port, created_at, updated_at
		FROM providers
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query providers: %w", err)
	}
	defer rows.Close()

	var daos []*ProviderDAO
	for rows.Next() {
		var dao ProviderDAO
		err := rows.Scan(
			&dao.ID,
			&dao.Name,
			&dao.Host,
			&dao.Port,
			&dao.CreatedAt,
			&dao.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan provider: %w", err)
		}

		daos = append(daos, &dao)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating providers: %w", err)
	}

	return daos, nil
}
