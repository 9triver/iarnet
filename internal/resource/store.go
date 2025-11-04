package resource

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

// Store 资源 provider 持久化存储接口
type Store interface {
	// SaveLocalProvider 保存 local provider 配置
	SaveLocalProvider(provider *ProviderConfig) error

	// GetLocalProvider 获取 local provider 配置
	GetLocalProvider(providerID string) (*ProviderConfig, error)

	// GetAllLocalProviders 获取所有 local providers 配置
	GetAllLocalProviders() ([]*ProviderConfig, error)

	// DeleteLocalProvider 删除 local provider 配置
	DeleteLocalProvider(providerID string) error

	// UpdateProviderStatus 更新 provider 状态
	UpdateProviderStatus(providerID string, status Status) error

	// Close 关闭数据库连接
	Close() error
}

// ProviderConfig 持久化的 provider 配置
type ProviderConfig struct {
	ProviderID   string       `json:"providerId"`
	ProviderType ProviderType `json:"providerType"`
	Name         string       `json:"name"`
	Config       string       `json:"config"` // JSON 序列化的配置
	Status       Status       `json:"status"`
	CreatedAt    int64        `json:"createdAt"`
	UpdatedAt    int64        `json:"updatedAt"`
}

// store SQLite 持久化存储实现
type store struct {
	db     *sql.DB
	mu     sync.RWMutex
	cache  map[string]*ProviderConfig // 内存缓存
	dbPath string
}

// NewStore 创建新的存储实例
func NewStore(dbPath string) (Store, error) {
	if dbPath == "" {
		dbPath = "./data/resource_providers.db"
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	s := &store{
		db:     db,
		cache:  make(map[string]*ProviderConfig),
		dbPath: dbPath,
	}

	// 初始化数据库表
	if err := s.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	// 从数据库加载数据到缓存
	if err := s.loadCache(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load cache: %w", err)
	}

	logrus.Infof("Resource provider store initialized with %d providers", len(s.cache))
	return s, nil
}

// initTables 初始化数据库表
func (s *store) initTables() error {
	schema := `
	-- Local provider 配置表
	CREATE TABLE IF NOT EXISTS local_providers (
		provider_id TEXT PRIMARY KEY,
		provider_type TEXT NOT NULL,
		name TEXT NOT NULL,
		config TEXT NOT NULL,
		status INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	-- 创建索引
	CREATE INDEX IF NOT EXISTS idx_providers_type ON local_providers(provider_type);
	CREATE INDEX IF NOT EXISTS idx_providers_status ON local_providers(status);
	`

	_, err := s.db.Exec(schema)
	return err
}

// loadCache 从数据库加载数据到缓存
func (s *store) loadCache() error {
	rows, err := s.db.Query(`
		SELECT provider_id, provider_type, name, config, status, created_at, updated_at
		FROM local_providers
		ORDER BY provider_id
	`)
	if err != nil {
		return fmt.Errorf("failed to query providers: %w", err)
	}
	defer rows.Close()

	s.mu.Lock()
	defer s.mu.Unlock()

	for rows.Next() {
		var providerConfig ProviderConfig
		var providerType string
		var status int

		err := rows.Scan(
			&providerConfig.ProviderID,
			&providerType,
			&providerConfig.Name,
			&providerConfig.Config,
			&status,
			&providerConfig.CreatedAt,
			&providerConfig.UpdatedAt,
		)
		if err != nil {
			logrus.Errorf("Failed to scan provider row: %v", err)
			continue
		}

		providerConfig.ProviderType = ProviderType(providerType)
		providerConfig.Status = Status(status)

		s.cache[providerConfig.ProviderID] = &providerConfig
	}

	return rows.Err()
}

// SaveLocalProvider 保存 local provider 配置
func (s *store) SaveLocalProvider(provider *ProviderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 插入或更新
	_, err := s.db.Exec(`
		INSERT INTO local_providers (
			provider_id, provider_type, name, config, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider_id) DO UPDATE SET
			provider_type = excluded.provider_type,
			name = excluded.name,
			config = excluded.config,
			status = excluded.status,
			updated_at = excluded.updated_at
	`,
		provider.ProviderID,
		string(provider.ProviderType),
		provider.Name,
		provider.Config,
		int(provider.Status),
		provider.CreatedAt,
		provider.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save provider: %w", err)
	}

	// 更新缓存
	s.cache[provider.ProviderID] = provider
	logrus.Infof("Saved provider to store: ID=%s, Type=%s, Name=%s", provider.ProviderID, provider.ProviderType, provider.Name)

	return nil
}

// GetLocalProvider 获取 local provider 配置
func (s *store) GetLocalProvider(providerID string) (*ProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 先从缓存读取
	if provider, ok := s.cache[providerID]; ok {
		return provider, nil
	}

	// 缓存未命中，从数据库读取
	return s.loadProviderFromDB(providerID)
}

// loadProviderFromDB 从数据库加载 provider
func (s *store) loadProviderFromDB(providerID string) (*ProviderConfig, error) {
	var providerConfig ProviderConfig
	var providerType string
	var status int

	err := s.db.QueryRow(`
		SELECT provider_id, provider_type, name, config, status, created_at, updated_at
		FROM local_providers
		WHERE provider_id = ?
	`, providerID).Scan(
		&providerConfig.ProviderID,
		&providerType,
		&providerConfig.Name,
		&providerConfig.Config,
		&status,
		&providerConfig.CreatedAt,
		&providerConfig.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query provider: %w", err)
	}

	providerConfig.ProviderType = ProviderType(providerType)
	providerConfig.Status = Status(status)

	// 更新缓存
	s.mu.RUnlock()
	s.mu.Lock()
	s.cache[providerID] = &providerConfig
	s.mu.Unlock()
	s.mu.RLock()

	return &providerConfig, nil
}

// GetAllLocalProviders 获取所有 local providers 配置
func (s *store) GetAllLocalProviders() ([]*ProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	providers := make([]*ProviderConfig, 0, len(s.cache))
	for _, provider := range s.cache {
		providers = append(providers, provider)
	}

	return providers, nil
}

// DeleteLocalProvider 删除 local provider 配置
func (s *store) DeleteLocalProvider(providerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查缓存中是否存在
	if _, ok := s.cache[providerID]; !ok {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	// 删除数据库记录
	_, err := s.db.Exec(`DELETE FROM local_providers WHERE provider_id = ?`, providerID)
	if err != nil {
		return fmt.Errorf("failed to delete provider: %w", err)
	}

	// 从缓存删除
	delete(s.cache, providerID)
	logrus.Infof("Deleted provider from store: ID=%s", providerID)

	return nil
}

// UpdateProviderStatus 更新 provider 状态
func (s *store) UpdateProviderStatus(providerID string, status Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新数据库
	_, err := s.db.Exec(
		`UPDATE local_providers SET status = ?, updated_at = ? WHERE provider_id = ?`,
		int(status), s.getCurrentTimestamp(), providerID,
	)
	if err != nil {
		return fmt.Errorf("failed to update provider status: %w", err)
	}

	// 更新缓存
	if provider, ok := s.cache[providerID]; ok {
		provider.Status = status
		provider.UpdatedAt = s.getCurrentTimestamp()
	}

	logrus.Debugf("Updated provider status in store: ID=%s, Status=%d", providerID, status)
	return nil
}

// getCurrentTimestamp 获取当前时间戳
func (s *store) getCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// Close 关闭数据库连接
func (s *store) Close() error {
	return s.db.Close()
}

// SerializeDockerConfig 序列化 Docker 配置
func SerializeDockerConfig(config DockerConfig) (string, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to serialize Docker config: %w", err)
	}
	return string(data), nil
}

// DeserializeDockerConfig 反序列化 Docker 配置
func DeserializeDockerConfig(configStr string) (DockerConfig, error) {
	var config DockerConfig
	err := json.Unmarshal([]byte(configStr), &config)
	if err != nil {
		return DockerConfig{}, fmt.Errorf("failed to deserialize Docker config: %w", err)
	}
	return config, nil
}

// SerializeK8sConfig 序列化 K8s 配置
func SerializeK8sConfig(config K8sConfig) (string, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to serialize K8s config: %w", err)
	}
	return string(data), nil
}

// DeserializeK8sConfig 反序列化 K8s 配置
func DeserializeK8sConfig(configStr string) (K8sConfig, error) {
	var config K8sConfig
	err := json.Unmarshal([]byte(configStr), &config)
	if err != nil {
		return K8sConfig{}, fmt.Errorf("failed to deserialize K8s config: %w", err)
	}
	return config, nil
}
