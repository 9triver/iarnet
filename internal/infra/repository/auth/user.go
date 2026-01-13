package auth

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
// UserDAO - 数据访问对象
// ============================================================================

// UserDAO 用户数据访问对象
type UserDAO struct {
	ID        string          `db:"id"`
	Name      string          `db:"name"`
	Password  string          `db:"password"` // 存储明文密码（用于配置中的超级管理员）或哈希后的密码
	Role      config.UserRole `db:"role"`
	CreatedAt time.Time       `db:"created_at"`
	UpdatedAt time.Time       `db:"updated_at"`
}

// ============================================================================
// UserRepo - 接口定义
// ============================================================================

// UserRepo 用户仓库接口
type UserRepo interface {
	Create(ctx context.Context, dao *UserDAO) error
	Update(ctx context.Context, dao *UserDAO) error
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (*UserDAO, error)
	GetByName(ctx context.Context, name string) (*UserDAO, error)
	GetAll(ctx context.Context) ([]*UserDAO, error)
	Count(ctx context.Context) (int, error)
}

// ============================================================================
// UserRepoSQLite - SQLite 实现
// ============================================================================

// userRepoSQLite SQLite 实现的 UserRepo
type userRepoSQLite struct {
	db *sql.DB
}

// NewUserRepoSQLite 创建基于 SQLite 的 UserRepo
func NewUserRepoSQLite(dbPath string) (UserRepo, error) {
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

	repo := &userRepoSQLite{
		db: db,
	}

	// 初始化表结构
	if err := repo.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logrus.Infof("User repository initialized with SQLite at %s", dbPath)
	return repo, nil
}

// initSchema 初始化数据库表结构
func (r *userRepoSQLite) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'normal',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_users_name ON users(name);
	`
	_, err := r.db.Exec(query)
	return err
}

// Close 关闭数据库连接
func (r *userRepoSQLite) Close() error {
	return r.db.Close()
}

// Create 创建用户
func (r *userRepoSQLite) Create(ctx context.Context, dao *UserDAO) error {
	query := `
		INSERT INTO users (id, name, password, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
	`
	_, err := r.db.ExecContext(ctx, query, dao.ID, dao.Name, dao.Password, string(dao.Role))
	return err
}

// Update 更新用户
func (r *userRepoSQLite) Update(ctx context.Context, dao *UserDAO) error {
	query := `
		UPDATE users
		SET name = ?, password = ?, role = ?, updated_at = datetime('now')
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query, dao.Name, dao.Password, string(dao.Role), dao.ID)
	return err
}

// Delete 删除用户
func (r *userRepoSQLite) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// Get 根据 ID 获取用户
func (r *userRepoSQLite) Get(ctx context.Context, id string) (*UserDAO, error) {
	query := `
		SELECT id, name, password, role, 
		       datetime(created_at) as created_at, 
		       datetime(updated_at) as updated_at
		FROM users
		WHERE id = ?
	`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanRow(row)
}

// GetByName 根据用户名获取用户
func (r *userRepoSQLite) GetByName(ctx context.Context, name string) (*UserDAO, error) {
	query := `
		SELECT id, name, password, role, 
		       datetime(created_at) as created_at, 
		       datetime(updated_at) as updated_at
		FROM users
		WHERE name = ?
	`
	row := r.db.QueryRowContext(ctx, query, name)
	return r.scanRow(row)
}

// GetAll 获取所有用户
func (r *userRepoSQLite) GetAll(ctx context.Context) ([]*UserDAO, error) {
	query := `
		SELECT id, name, password, role, 
		       datetime(created_at) as created_at, 
		       datetime(updated_at) as updated_at
		FROM users
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*UserDAO
	for rows.Next() {
		dao, err := r.scanRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, dao)
	}
	return users, rows.Err()
}

// Count 获取用户总数
func (r *userRepoSQLite) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users`
	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

// scanRow 扫描单行数据
func (r *userRepoSQLite) scanRow(row *sql.Row) (*UserDAO, error) {
	dao := &UserDAO{}
	var roleStr string
	var createdAtStr, updatedAtStr string
	err := row.Scan(&dao.ID, &dao.Name, &dao.Password, &roleStr, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, err
	}
	dao.Role = config.UserRole(roleStr)

	// 解析时间字符串
	createdAt, err := time.Parse("2006-01-02 15:04:05", createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}
	updatedAt, err := time.Parse("2006-01-02 15:04:05", updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	dao.CreatedAt = createdAt
	dao.UpdatedAt = updatedAt
	return dao, nil
}

// scanRows 扫描多行数据
func (r *userRepoSQLite) scanRows(rows *sql.Rows) (*UserDAO, error) {
	dao := &UserDAO{}
	var roleStr string
	var createdAtStr, updatedAtStr string
	err := rows.Scan(&dao.ID, &dao.Name, &dao.Password, &roleStr, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, err
	}
	dao.Role = config.UserRole(roleStr)

	// 解析时间字符串
	createdAt, err := time.Parse("2006-01-02 15:04:05", createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}
	updatedAt, err := time.Parse("2006-01-02 15:04:05", updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	dao.CreatedAt = createdAt
	dao.UpdatedAt = updatedAt
	return dao, nil
}
