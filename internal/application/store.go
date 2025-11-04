package application

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/resource"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

// Store 应用数据持久化存储接口
type Store interface {
	// CreateApplication 创建应用
	CreateApplication(app *AppRef) error

	// GetApplication 获取应用
	GetApplication(appID string) (*AppRef, error)

	// GetAllApplications 获取所有应用
	GetAllApplications() ([]*AppRef, error)

	// UpdateApplication 更新应用
	UpdateApplication(app *AppRef) error

	// DeleteApplication 删除应用
	DeleteApplication(appID string) error

	// UpdateApplicationStatus 更新应用状态
	UpdateApplicationStatus(appID string, status Status) error

	// GetNextAppID 获取下一个应用ID
	GetNextAppID() (int, error)

	// Close 关闭数据库连接
	Close() error
}

// store SQLite 持久化存储实现
type store struct {
	db     *sql.DB
	mu     sync.RWMutex
	cache  map[string]*AppRef // 内存缓存
	dbPath string
}

// StoreConfig 存储配置
type StoreConfig struct {
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int
}

// NewStore 创建新的存储实例
func NewStore(dbPath string) (Store, error) {
	return NewStoreWithConfig(dbPath, nil)
}

// NewStoreWithConfig 使用配置创建存储实例
func NewStoreWithConfig(dbPath string, config *StoreConfig) (Store, error) {
	if dbPath == "" {
		dbPath = "./data/applications.db"
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数（使用配置或默认值）
	if config != nil {
		db.SetMaxOpenConns(config.MaxOpenConns)
		db.SetMaxIdleConns(config.MaxIdleConns)
		if config.ConnMaxLifetimeSeconds > 0 {
			db.SetConnMaxLifetime(time.Duration(config.ConnMaxLifetimeSeconds) * time.Second)
		}
	} else {
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)
	}

	s := &store{
		db:     db,
		cache:  make(map[string]*AppRef),
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

	logrus.Infof("Application store initialized with %d applications", len(s.cache))
	return s, nil
}

// initTables 初始化数据库表
func (s *store) initTables() error {
	schema := `
	-- 应用表
	CREATE TABLE IF NOT EXISTS applications (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		status TEXT NOT NULL,
		type TEXT NOT NULL,
		git_url TEXT,
		branch TEXT,
		description TEXT,
		ports TEXT, -- JSON array of integers
		health_check TEXT,
		container_id TEXT,
		last_deployed INTEGER,
		execute_cmd TEXT,
		code_dir TEXT,
		runner_env TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	-- 组件表
	CREATE TABLE IF NOT EXISTS application_components (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app_id TEXT NOT NULL,
		name TEXT NOT NULL,
		image TEXT NOT NULL,
		status TEXT NOT NULL,
		deployed_at INTEGER,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		container_ref TEXT, -- JSON serialized ContainerRef
		FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE,
		UNIQUE(app_id, name)
	);

	-- 创建索引
	CREATE INDEX IF NOT EXISTS idx_applications_status ON applications(status);
	CREATE INDEX IF NOT EXISTS idx_applications_type ON applications(type);
	CREATE INDEX IF NOT EXISTS idx_components_app_id ON application_components(app_id);
	CREATE INDEX IF NOT EXISTS idx_components_status ON application_components(status);
	`

	_, err := s.db.Exec(schema)
	return err
}

// loadCache 从数据库加载数据到缓存
func (s *store) loadCache() error {
	rows, err := s.db.Query(`
		SELECT id, name, status, type, git_url, branch, description, ports,
		       health_check, container_id, last_deployed, execute_cmd, code_dir,
		       runner_env, created_at, updated_at
		FROM applications
		ORDER BY id
	`)
	if err != nil {
		return fmt.Errorf("failed to query applications: %w", err)
	}
	defer rows.Close()

	s.mu.Lock()
	defer s.mu.Unlock()

	for rows.Next() {
		app := &AppRef{
			Components: make(map[string]*Component),
		}

		var portsJSON sql.NullString
		var lastDeployed sql.NullInt64
		var createdAt, updatedAt int64

		err := rows.Scan(
			&app.ID, &app.Name, &app.Status, &app.Type,
			&app.GitUrl, &app.Branch, &app.Description, &portsJSON,
			&app.HealthCheck, &app.ContainerID, &lastDeployed,
			&app.ExecuteCmd, &app.CodeDir, &app.RunnerEnv,
			&createdAt, &updatedAt,
		)
		if err != nil {
			logrus.Errorf("Failed to scan application row: %v", err)
			continue
		}

		// 解析 ports
		if portsJSON.Valid && portsJSON.String != "" {
			var ports []int
			if err := json.Unmarshal([]byte(portsJSON.String), &ports); err == nil {
				app.Ports = ports
			}
		}

		// 解析 last_deployed
		if lastDeployed.Valid {
			app.LastDeployed = time.Unix(lastDeployed.Int64, 0)
		}

		// 加载组件
		components, err := s.loadComponents(app.ID)
		if err != nil {
			logrus.Errorf("Failed to load components for app %s: %v", app.ID, err)
		} else {
			app.Components = components
		}

		s.cache[app.ID] = app
	}

	return rows.Err()
}

// loadComponents 加载应用的组件
func (s *store) loadComponents(appID string) (map[string]*Component, error) {
	rows, err := s.db.Query(`
		SELECT name, image, status, deployed_at, created_at, updated_at, container_ref
		FROM application_components
		WHERE app_id = ?
	`, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to query components: %w", err)
	}
	defer rows.Close()

	components := make(map[string]*Component)
	for rows.Next() {
		comp := &Component{}
		var deployedAt sql.NullInt64
		var createdAt, updatedAt int64
		var containerRefJSON sql.NullString

		err := rows.Scan(
			&comp.Name, &comp.Image, &comp.Status, &deployedAt,
			&createdAt, &updatedAt, &containerRefJSON,
		)
		if err != nil {
			logrus.Errorf("Failed to scan component row: %v", err)
			continue
		}

		if deployedAt.Valid {
			comp.DeployedAt = time.Unix(deployedAt.Int64, 0)
		}
		comp.CreatedAt = time.Unix(createdAt, 0)
		comp.UpdatedAt = time.Unix(updatedAt, 0)

		// 解析 container_ref
		if containerRefJSON.Valid && containerRefJSON.String != "" {
			var containerRef resource.ContainerRef
			if err := json.Unmarshal([]byte(containerRefJSON.String), &containerRef); err == nil {
				comp.ContainerRef = &containerRef
			}
		}

		components[comp.Name] = comp
	}

	return components, rows.Err()
}

// CreateApplication 创建应用
func (s *store) CreateApplication(app *AppRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	portsJSON, _ := json.Marshal(app.Ports)

	var lastDeployed int64
	if !app.LastDeployed.IsZero() {
		lastDeployed = app.LastDeployed.Unix()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO applications (
			id, name, status, type, git_url, branch, description, ports,
			health_check, container_id, last_deployed, execute_cmd, code_dir,
			runner_env, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		app.ID, app.Name, app.Status, app.Type,
		app.GitUrl, app.Branch, app.Description, string(portsJSON),
		app.HealthCheck, app.ContainerID, lastDeployed,
		app.ExecuteCmd, app.CodeDir, app.RunnerEnv,
		now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert application: %w", err)
	}

	// 保存组件
	if err := s.saveComponents(tx, app.ID, app.Components); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 更新缓存
	s.cache[app.ID] = app
	logrus.Infof("Application created in store: ID=%s, Name=%s", app.ID, app.Name)

	return nil
}

// GetApplication 获取应用
func (s *store) GetApplication(appID string) (*AppRef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 先从缓存读取
	if app, ok := s.cache[appID]; ok {
		return app, nil
	}

	// 缓存未命中，从数据库读取
	return s.loadApplicationFromDB(appID)
}

// loadApplicationFromDB 从数据库加载应用
func (s *store) loadApplicationFromDB(appID string) (*AppRef, error) {
	app := &AppRef{
		ID:         appID,
		Components: make(map[string]*Component),
	}

	var portsJSON sql.NullString
	var lastDeployed sql.NullInt64
	var createdAt, updatedAt int64

	err := s.db.QueryRow(`
		SELECT name, status, type, git_url, branch, description, ports,
		       health_check, container_id, last_deployed, execute_cmd, code_dir,
		       runner_env, created_at, updated_at
		FROM applications
		WHERE id = ?
	`, appID).Scan(
		&app.Name, &app.Status, &app.Type,
		&app.GitUrl, &app.Branch, &app.Description, &portsJSON,
		&app.HealthCheck, &app.ContainerID, &lastDeployed,
		&app.ExecuteCmd, &app.CodeDir, &app.RunnerEnv,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("application not found: %s", appID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query application: %w", err)
	}

	// 解析 ports
	if portsJSON.Valid && portsJSON.String != "" {
		var ports []int
		if err := json.Unmarshal([]byte(portsJSON.String), &ports); err == nil {
			app.Ports = ports
		}
	}

	// 解析 last_deployed
	if lastDeployed.Valid {
		app.LastDeployed = time.Unix(lastDeployed.Int64, 0)
	}

	// 加载组件
	components, err := s.loadComponents(appID)
	if err != nil {
		logrus.Errorf("Failed to load components for app %s: %v", appID, err)
	} else {
		app.Components = components
	}

	// 更新缓存
	s.mu.RUnlock()
	s.mu.Lock()
	s.cache[appID] = app
	s.mu.Unlock()
	s.mu.RLock()

	return app, nil
}

// GetAllApplications 获取所有应用
func (s *store) GetAllApplications() ([]*AppRef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	apps := make([]*AppRef, 0, len(s.cache))
	for _, app := range s.cache {
		apps = append(apps, app)
	}

	return apps, nil
}

// UpdateApplication 更新应用
func (s *store) UpdateApplication(app *AppRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	portsJSON, _ := json.Marshal(app.Ports)

	var lastDeployed int64
	if !app.LastDeployed.IsZero() {
		lastDeployed = app.LastDeployed.Unix()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE applications SET
			name = ?, status = ?, type = ?, git_url = ?, branch = ?,
			description = ?, ports = ?, health_check = ?, container_id = ?,
			last_deployed = ?, execute_cmd = ?, code_dir = ?, runner_env = ?,
			updated_at = ?
		WHERE id = ?
	`,
		app.Name, app.Status, app.Type,
		app.GitUrl, app.Branch, app.Description, string(portsJSON),
		app.HealthCheck, app.ContainerID, lastDeployed,
		app.ExecuteCmd, app.CodeDir, app.RunnerEnv,
		now, app.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update application: %w", err)
	}

	// 更新组件
	if err := s.saveComponents(tx, app.ID, app.Components); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 更新缓存
	s.cache[app.ID] = app
	logrus.Debugf("Application updated in store: ID=%s", app.ID)

	return nil
}

// DeleteApplication 删除应用
func (s *store) DeleteApplication(appID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查缓存中是否存在
	if _, ok := s.cache[appID]; !ok {
		return fmt.Errorf("application not found: %s", appID)
	}

	// 删除数据库记录（CASCADE 会自动删除组件）
	_, err := s.db.Exec(`DELETE FROM applications WHERE id = ?`, appID)
	if err != nil {
		return fmt.Errorf("failed to delete application: %w", err)
	}

	// 从缓存删除
	delete(s.cache, appID)
	logrus.Infof("Application deleted from store: ID=%s", appID)

	return nil
}

// UpdateApplicationStatus 更新应用状态
func (s *store) UpdateApplicationStatus(appID string, status Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新数据库
	now := time.Now().Unix()
	_, err := s.db.Exec(
		`UPDATE applications SET status = ?, updated_at = ? WHERE id = ?`,
		status, now, appID,
	)
	if err != nil {
		return fmt.Errorf("failed to update application status: %w", err)
	}

	// 更新缓存
	if app, ok := s.cache[appID]; ok {
		app.Status = status
	}

	logrus.Debugf("Application status updated in store: ID=%s, Status=%s", appID, status)
	return nil
}

// GetNextAppID 获取下一个应用ID
func (s *store) GetNextAppID() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	maxID := 0
	for _, app := range s.cache {
		var appID int
		if _, err := fmt.Sscanf(app.ID, "%d", &appID); err == nil {
			if appID > maxID {
				maxID = appID
			}
		}
	}

	return maxID + 1, nil
}

// saveComponents 保存组件到数据库
func (s *store) saveComponents(tx *sql.Tx, appID string, components map[string]*Component) error {
	// 先删除所有现有组件
	_, err := tx.Exec(`DELETE FROM application_components WHERE app_id = ?`, appID)
	if err != nil {
		return fmt.Errorf("failed to delete existing components: %w", err)
	}

	// 插入新组件
	now := time.Now().Unix()
	for _, comp := range components {
		var deployedAt int64
		if !comp.DeployedAt.IsZero() {
			deployedAt = comp.DeployedAt.Unix()
		}

		var containerRefJSON string
		if comp.ContainerRef != nil {
			// 序列化 ContainerRef
			if data, err := json.Marshal(comp.ContainerRef); err == nil {
				containerRefJSON = string(data)
			}
		}

		createdAt := comp.CreatedAt.Unix()
		updatedAt := comp.UpdatedAt.Unix()
		if createdAt == 0 {
			createdAt = now
		}
		if updatedAt == 0 {
			updatedAt = now
		}

		_, err := tx.Exec(`
			INSERT INTO application_components (
				app_id, name, image, status, deployed_at, created_at, updated_at, container_ref
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, appID, comp.Name, comp.Image, comp.Status, deployedAt, createdAt, updatedAt, containerRefJSON)
		if err != nil {
			return fmt.Errorf("failed to insert component: %w", err)
		}
	}

	return nil
}

// Close 关闭数据库连接
func (s *store) Close() error {
	return s.db.Close()
}
