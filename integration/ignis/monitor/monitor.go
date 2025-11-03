package monitor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/9triver/ignis/monitor"
	"github.com/9triver/ignis/proto/controller"
)

// dbOperation 数据库操作
type dbOperation struct {
	opType string      // 操作类型：insert, update, delete
	table  string      // 表名
	data   interface{} // 数据
}

// cache 内存缓存
type cache struct {
	applications map[string]*monitor.ApplicationInfo
	nodeStates   map[string]map[string]*monitor.NodeState // appID -> nodeID -> NodeState
	tasks        map[string]*monitor.TaskInfo
	mu           sync.RWMutex
}

// Monitor 基于 SQLite 存储的 monitor 实现
type Monitor struct {
	db      *sql.DB
	config  *Config
	cache   *cache
	opChan  chan dbOperation // 异步写入队列
	wg      sync.WaitGroup
	mutex   sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// New 创建一个新的 monitor 实例
func New(config *Config) (monitor.Monitor, error) {
	if config == nil {
		config = DefaultConfig()
	}

	db, err := sql.Open("sqlite3", config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(config.ConnMaxLifetime) * time.Second)

	m := &Monitor{
		db:     db,
		config: config,
		cache: &cache{
			applications: make(map[string]*monitor.ApplicationInfo),
			nodeStates:   make(map[string]map[string]*monitor.NodeState),
			tasks:        make(map[string]*monitor.TaskInfo),
		},
		opChan: make(chan dbOperation, config.QueueSize),
	}

	// 初始化数据库表
	if err := m.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	// 从数据库加载现有数据到缓存
	if err := m.loadCache(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load cache: %w", err)
	}

	return m, nil
}

// initTables 初始化数据库表
func (m *Monitor) initTables() error {
	schema := `
	-- 应用表
	CREATE TABLE IF NOT EXISTS applications (
		app_id TEXT PRIMARY KEY,
		metadata TEXT NOT NULL,
		status TEXT NOT NULL,
		dag TEXT,
		start_time INTEGER,
		end_time INTEGER,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	-- 节点状态表
	CREATE TABLE IF NOT EXISTS node_states (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app_id TEXT NOT NULL,
		node_id TEXT NOT NULL,
		type TEXT NOT NULL,
		status TEXT NOT NULL,
		start_time INTEGER,
		end_time INTEGER,
		duration INTEGER,
		retry_count INTEGER DEFAULT 0,
		error_message TEXT,
		result TEXT,
		updated_at INTEGER NOT NULL,
		UNIQUE(app_id, node_id)
	);

	-- 任务表
	CREATE TABLE IF NOT EXISTS tasks (
		task_id TEXT PRIMARY KEY,
		app_id TEXT NOT NULL,
		node_id TEXT NOT NULL,
		status TEXT NOT NULL,
		function_name TEXT,
		parameters TEXT,
		start_time INTEGER,
		end_time INTEGER,
		duration INTEGER,
		worker_id TEXT,
		retry_count INTEGER DEFAULT 0,
		error_message TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	-- 指标表
	CREATE TABLE IF NOT EXISTS metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		value REAL NOT NULL,
		labels TEXT,
		unit TEXT,
		timestamp INTEGER NOT NULL
	);

	-- 事件表
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		level TEXT NOT NULL,
		app_id TEXT,
		node_id TEXT,
		task_id TEXT,
		worker_id TEXT,
		message TEXT NOT NULL,
		details TEXT,
		source TEXT,
		timestamp INTEGER NOT NULL
	);

	-- 创建索引
	CREATE INDEX IF NOT EXISTS idx_applications_status ON applications(status);
	CREATE INDEX IF NOT EXISTS idx_node_states_app_id ON node_states(app_id);
	CREATE INDEX IF NOT EXISTS idx_node_states_status ON node_states(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_app_id ON tasks(app_id);
	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_metrics_name ON metrics(name);
	CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON metrics(timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
	CREATE INDEX IF NOT EXISTS idx_events_level ON events(level);
	CREATE INDEX IF NOT EXISTS idx_events_app_id ON events(app_id);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
	`

	_, err := m.db.Exec(schema)
	return err
}

// loadCache 从数据库加载数据到缓存
func (m *Monitor) loadCache() error {
	// 加载应用数据
	rows, err := m.db.Query(`SELECT app_id FROM applications`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var appID string
		if err := rows.Scan(&appID); err != nil {
			continue
		}
		if info, err := m.loadApplicationFromDB(context.Background(), appID); err == nil && info != nil {
			m.cache.applications[appID] = info
		}
	}

	// 加载节点状态
	rows, err = m.db.Query(`SELECT app_id, node_id FROM node_states`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var appID, nodeID string
		if err := rows.Scan(&appID, &nodeID); err != nil {
			continue
		}
		if state, err := m.loadNodeStateFromDB(context.Background(), appID, nodeID); err == nil && state != nil {
			if m.cache.nodeStates[appID] == nil {
				m.cache.nodeStates[appID] = make(map[string]*monitor.NodeState)
			}
			m.cache.nodeStates[appID][nodeID] = state
		}
	}

	return nil
}

// asyncWrite 异步写入数据到数据库
func (m *Monitor) asyncWrite(op dbOperation) {
	select {
	case m.opChan <- op:
	default:
		// 队列满了，记录警告但不阻塞
		// 可以考虑添加日志
	}
}

// dbWriter 后台数据库写入器
func (m *Monitor) dbWriter() {
	defer m.wg.Done()

	batchSize := 100
	batch := make([]dbOperation, 0, batchSize)
	ticker := time.NewTicker(time.Duration(m.config.FlushInterval) * time.Millisecond)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		tx, err := m.db.Begin()
		if err != nil {
			batch = batch[:0]
			return
		}

		for _, op := range batch {
			m.executeDBOperation(tx, op)
		}

		tx.Commit()
		batch = batch[:0]
	}

	for {
		select {
		case <-m.ctx.Done():
			flush() // 最后一次刷新
			return

		case op := <-m.opChan:
			batch = append(batch, op)
			if len(batch) >= batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// executeDBOperation 执行数据库操作
func (m *Monitor) executeDBOperation(tx *sql.Tx, op dbOperation) error {
	switch op.table {
	case "applications":
		return m.executeApplicationOp(tx, op)
	case "node_states":
		return m.executeNodeStateOp(tx, op)
	case "tasks":
		return m.executeTaskOp(tx, op)
	case "metrics":
		return m.executeMetricOp(tx, op)
	case "events":
		return m.executeEventOp(tx, op)
	}
	return nil
}

// =============================================================================
// ApplicationMonitor 实现
// =============================================================================

func (m *Monitor) RegisterApplication(ctx context.Context, appID string, metadata *monitor.ApplicationMetadata) error {
	now := time.Now()
	if metadata.CreatedAt.IsZero() {
		metadata.CreatedAt = now
	}
	metadata.UpdatedAt = now

	// 更新缓存
	m.cache.mu.Lock()
	m.cache.applications[appID] = &monitor.ApplicationInfo{
		Metadata: metadata,
		Status:   monitor.AppStatusPending,
		Progress: &monitor.ApplicationProgress{},
		Resources: &monitor.ResourceUsage{
			AppID: appID,
		},
	}
	m.cache.mu.Unlock()

	// 异步写入数据库
	m.asyncWrite(dbOperation{
		opType: "insert",
		table:  "applications",
		data: map[string]interface{}{
			"app_id":     appID,
			"metadata":   metadata,
			"status":     monitor.AppStatusPending,
			"created_at": now.Unix(),
			"updated_at": now.Unix(),
		},
	})

	return nil
}

func (m *Monitor) UnregisterApplication(ctx context.Context, appID string) error {
	// 从缓存删除
	m.cache.mu.Lock()
	delete(m.cache.applications, appID)
	delete(m.cache.nodeStates, appID)
	m.cache.mu.Unlock()

	// 异步从数据库删除
	m.asyncWrite(dbOperation{
		opType: "delete",
		table:  "applications",
		data:   map[string]interface{}{"app_id": appID},
	})

	return nil
}

func (m *Monitor) GetApplicationInfo(ctx context.Context, appID string) (*monitor.ApplicationInfo, error) {
	// 优先从缓存读取
	m.cache.mu.RLock()
	info, exists := m.cache.applications[appID]
	m.cache.mu.RUnlock()

	if exists {
		// 返回副本
		infoCopy := *info
		if info.Metadata != nil {
			metaCopy := *info.Metadata
			infoCopy.Metadata = &metaCopy
		}
		if info.Progress != nil {
			progCopy := *info.Progress
			infoCopy.Progress = &progCopy
		}
		// 动态计算进度
		m.calculateProgressFromCache(appID, &infoCopy)
		return &infoCopy, nil
	}

	// 缓存未命中，从数据库加载
	return m.loadApplicationFromDB(ctx, appID)
}

func (m *Monitor) loadApplicationFromDB(ctx context.Context, appID string) (*monitor.ApplicationInfo, error) {
	var metadataJSON, dagJSON string
	var status string
	var startTime, endTime sql.NullInt64

	err := m.db.QueryRowContext(ctx,
		`SELECT metadata, status, dag, start_time, end_time FROM applications WHERE app_id = ?`,
		appID).Scan(&metadataJSON, &status, &dagJSON, &startTime, &endTime)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var metadata monitor.ApplicationMetadata
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return nil, err
	}

	info := &monitor.ApplicationInfo{
		Metadata: &metadata,
		Status:   monitor.ApplicationStatus(status),
		Progress: &monitor.ApplicationProgress{},
	}

	if dagJSON != "" {
		var dag controller.DAG
		if err := json.Unmarshal([]byte(dagJSON), &dag); err == nil {
			info.DAG = &dag
		}
	}

	if startTime.Valid {
		t := time.Unix(startTime.Int64, 0)
		info.StartTime = &t
	}
	if endTime.Valid {
		t := time.Unix(endTime.Int64, 0)
		info.EndTime = &t
		if info.StartTime != nil {
			info.Duration = t.Sub(*info.StartTime)
		}
	}

	return info, nil
}

func (m *Monitor) calculateProgressFromCache(appID string, info *monitor.ApplicationInfo) {
	m.cache.mu.RLock()
	defer m.cache.mu.RUnlock()

	nodeStates, exists := m.cache.nodeStates[appID]
	if !exists {
		return
	}

	progress := &monitor.ApplicationProgress{}
	for _, state := range nodeStates {
		progress.TotalNodes++
		switch state.Status {
		case monitor.NodeStatusCompleted:
			progress.CompletedNodes++
		case monitor.NodeStatusFailed:
			progress.FailedNodes++
		case monitor.NodeStatusRunning:
			progress.RunningNodes++
		case monitor.NodeStatusPending, monitor.NodeStatusReady:
			progress.PendingNodes++
		}
	}

	if progress.TotalNodes > 0 {
		progress.Percentage = float64(progress.CompletedNodes) / float64(progress.TotalNodes) * 100
	}

	info.Progress = progress
}

func (m *Monitor) ListApplications(ctx context.Context, filter *monitor.ApplicationFilter) ([]*monitor.ApplicationInfo, error) {
	// 从缓存读取
	m.cache.mu.RLock()
	defer m.cache.mu.RUnlock()

	apps := make([]*monitor.ApplicationInfo, 0)
	for _, info := range m.cache.applications {
		// 应用过滤器
		if filter != nil {
			if len(filter.Status) > 0 {
				matched := false
				for _, status := range filter.Status {
					if info.Status == status {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
		}

		// 创建副本
		infoCopy := *info
		if info.Metadata != nil {
			metaCopy := *info.Metadata
			infoCopy.Metadata = &metaCopy
		}
		apps = append(apps, &infoCopy)
	}

	// 应用分页
	if filter != nil {
		if filter.Offset > 0 && filter.Offset < len(apps) {
			apps = apps[filter.Offset:]
		}
		if filter.Limit > 0 && filter.Limit < len(apps) {
			apps = apps[:filter.Limit]
		}
	}

	return apps, nil
}

func (m *Monitor) UpdateApplicationStatus(ctx context.Context, appID string, status monitor.ApplicationStatus) error {
	now := time.Now()

	// 更新缓存
	m.cache.mu.Lock()
	if info, exists := m.cache.applications[appID]; exists {
		info.Status = status
		info.Metadata.UpdatedAt = now

		if status == monitor.AppStatusRunning && info.StartTime == nil {
			info.StartTime = &now
		} else if (status == monitor.AppStatusCompleted || status == monitor.AppStatusFailed || status == monitor.AppStatusTerminated) && info.EndTime == nil {
			info.EndTime = &now
			if info.StartTime != nil {
				info.Duration = now.Sub(*info.StartTime)
			}
		}
	}
	m.cache.mu.Unlock()

	// 异步写入数据库
	m.asyncWrite(dbOperation{
		opType: "update",
		table:  "applications",
		data: map[string]interface{}{
			"app_id":     appID,
			"status":     status,
			"updated_at": now.Unix(),
			"start_time": func() *int64 {
				if status == monitor.AppStatusRunning {
					t := now.Unix()
					return &t
				}
				return nil
			}(),
			"end_time": func() *int64 {
				if status == monitor.AppStatusCompleted || status == monitor.AppStatusFailed || status == monitor.AppStatusTerminated {
					t := now.Unix()
					return &t
				}
				return nil
			}(),
		},
	})

	return nil
}

func (m *Monitor) GetApplicationDAG(ctx context.Context, appID string) (*controller.DAG, error) {
	// 从缓存读取
	m.cache.mu.RLock()
	info, exists := m.cache.applications[appID]
	m.cache.mu.RUnlock()

	if exists && info.DAG != nil {
		return info.DAG, nil
	}

	return nil, nil
}

func (m *Monitor) SetApplicationDAG(ctx context.Context, appID string, dag *controller.DAG) error {
	// 更新缓存
	m.cache.mu.Lock()
	if info, exists := m.cache.applications[appID]; exists {
		info.DAG = dag
		info.Metadata.UpdatedAt = time.Now()
	}
	m.cache.mu.Unlock()

	// 异步写入数据库
	m.asyncWrite(dbOperation{
		opType: "update",
		table:  "applications",
		data: map[string]interface{}{
			"app_id":     appID,
			"dag":        dag,
			"updated_at": time.Now().Unix(),
		},
	})

	return nil
}

func (m *Monitor) GetApplicationMetrics(ctx context.Context, appID string) (*monitor.ApplicationMetrics, error) {
	return &monitor.ApplicationMetrics{
		AppID: appID,
	}, nil
}

// =============================================================================
// NodeMonitor 实现
// =============================================================================

func (m *Monitor) GetNodeState(ctx context.Context, appID, nodeID string) (*monitor.NodeState, error) {
	// 从缓存读取
	m.cache.mu.RLock()
	defer m.cache.mu.RUnlock()

	if appNodes, exists := m.cache.nodeStates[appID]; exists {
		if state, exists := appNodes[nodeID]; exists {
			// 返回副本
			stateCopy := *state
			return &stateCopy, nil
		}
	}

	// 缓存未命中，从数据库加载
	return m.loadNodeStateFromDB(ctx, appID, nodeID)
}

func (m *Monitor) loadNodeStateFromDB(ctx context.Context, appID, nodeID string) (*monitor.NodeState, error) {
	var nodeType, status, errorMsg sql.NullString
	var startTime, endTime, duration sql.NullInt64
	var retryCount int

	err := m.db.QueryRowContext(ctx,
		`SELECT type, status, start_time, end_time, duration, retry_count, error_message
		 FROM node_states WHERE app_id = ? AND node_id = ?`,
		appID, nodeID).Scan(&nodeType, &status, &startTime, &endTime, &duration, &retryCount, &errorMsg)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	state := &monitor.NodeState{
		ID:         nodeID,
		Type:       monitor.NodeType(nodeType.String),
		Status:     monitor.NodeStatus(status.String),
		RetryCount: retryCount,
		UpdatedAt:  time.Now(),
	}

	if startTime.Valid {
		t := time.Unix(startTime.Int64, 0)
		state.StartTime = &t
	}
	if endTime.Valid {
		t := time.Unix(endTime.Int64, 0)
		state.EndTime = &t
	}
	if duration.Valid {
		state.Duration = time.Duration(duration.Int64)
	}
	if errorMsg.Valid {
		state.ErrorMessage = errorMsg.String
	}

	return state, nil
}

func (m *Monitor) GetAllNodeStates(ctx context.Context, appID string) (map[string]*monitor.NodeState, error) {
	// 从缓存读取
	m.cache.mu.RLock()
	defer m.cache.mu.RUnlock()

	appNodes, exists := m.cache.nodeStates[appID]
	if !exists {
		return make(map[string]*monitor.NodeState), nil
	}

	// 返回副本
	states := make(map[string]*monitor.NodeState)
	for nodeID, state := range appNodes {
		stateCopy := *state
		states[nodeID] = &stateCopy
	}

	return states, nil
}

func (m *Monitor) UpdateNodeState(ctx context.Context, appID, nodeID string, state *monitor.NodeState) error {
	now := time.Now()
	state.UpdatedAt = now

	// 更新缓存
	m.cache.mu.Lock()
	if m.cache.nodeStates[appID] == nil {
		m.cache.nodeStates[appID] = make(map[string]*monitor.NodeState)
	}
	// 创建副本存入缓存
	stateCopy := *state
	m.cache.nodeStates[appID][nodeID] = &stateCopy
	m.cache.mu.Unlock()

	// 异步写入数据库
	var startTime, endTime interface{}
	if state.StartTime != nil {
		t := state.StartTime.Unix()
		startTime = &t
	}
	if state.EndTime != nil {
		t := state.EndTime.Unix()
		endTime = &t
	}

	m.asyncWrite(dbOperation{
		opType: "upsert",
		table:  "node_states",
		data: map[string]interface{}{
			"app_id":        appID,
			"node_id":       nodeID,
			"type":          state.Type,
			"status":        state.Status,
			"start_time":    startTime,
			"end_time":      endTime,
			"duration":      state.Duration.Nanoseconds(),
			"retry_count":   state.RetryCount,
			"error_message": state.ErrorMessage,
			"result":        state.Result,
			"updated_at":    now.Unix(),
		},
	})

	return nil
}

func (m *Monitor) MarkNodeReady(ctx context.Context, appID, nodeID string) error {
	return m.updateNodeStatus(ctx, appID, nodeID, monitor.NodeStatusReady)
}

func (m *Monitor) MarkNodeRunning(ctx context.Context, appID, nodeID string) error {
	now := time.Now().Unix()
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO node_states (app_id, node_id, status, start_time, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(app_id, node_id) DO UPDATE SET
		   status = excluded.status,
		   start_time = COALESCE(node_states.start_time, excluded.start_time),
		   updated_at = excluded.updated_at`,
		appID, nodeID, monitor.NodeStatusRunning, now, now)
	return err
}

func (m *Monitor) MarkNodeDone(ctx context.Context, appID, nodeID string, result *monitor.NodeResult) error {
	now := time.Now().Unix()

	var resultJSON sql.NullString
	if result != nil {
		if data, err := json.Marshal(result); err == nil {
			resultJSON = sql.NullString{String: string(data), Valid: true}
		}
	}

	_, err := m.db.ExecContext(ctx,
		`INSERT INTO node_states (app_id, node_id, status, end_time, result, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(app_id, node_id) DO UPDATE SET
		   status = excluded.status,
		   end_time = excluded.end_time,
		   result = excluded.result,
		   duration = excluded.end_time - COALESCE(node_states.start_time, excluded.end_time),
		   updated_at = excluded.updated_at`,
		appID, nodeID, monitor.NodeStatusCompleted, now, resultJSON, now)

	return err
}

func (m *Monitor) MarkNodeFailed(ctx context.Context, appID, nodeID string, err error) error {
	now := time.Now().Unix()
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	_, dbErr := m.db.ExecContext(ctx,
		`INSERT INTO node_states (app_id, node_id, status, end_time, error_message, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(app_id, node_id) DO UPDATE SET
		   status = excluded.status,
		   end_time = excluded.end_time,
		   error_message = excluded.error_message,
		   duration = excluded.end_time - COALESCE(node_states.start_time, excluded.end_time),
		   updated_at = excluded.updated_at`,
		appID, nodeID, monitor.NodeStatusFailed, now, errMsg, now)

	return dbErr
}

func (m *Monitor) updateNodeStatus(ctx context.Context, appID, nodeID string, status monitor.NodeStatus) error {
	now := time.Now().Unix()
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO node_states (app_id, node_id, status, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(app_id, node_id) DO UPDATE SET
		   status = excluded.status,
		   updated_at = excluded.updated_at`,
		appID, nodeID, status, now)
	return err
}

func (m *Monitor) GetNodeMetrics(ctx context.Context, appID, nodeID string) (*monitor.NodeMetrics, error) {
	return &monitor.NodeMetrics{NodeID: nodeID}, nil
}

func (m *Monitor) GetNodeDependencies(ctx context.Context, appID, nodeID string) (*monitor.NodeDependencies, error) {
	return &monitor.NodeDependencies{
		NodeID:       nodeID,
		Predecessors: []string{},
		Successors:   []string{},
		DataDeps:     []string{},
		ControlDeps:  []string{},
	}, nil
}

func (m *Monitor) WatchNodeState(ctx context.Context, appID, nodeID string) (<-chan *monitor.NodeStateChangeEvent, error) {
	ch := make(chan *monitor.NodeStateChangeEvent)
	close(ch)
	return ch, nil
}

// =============================================================================
// TaskMonitor 实现
// =============================================================================

func (m *Monitor) RecordTaskStart(ctx context.Context, taskID string, task *monitor.TaskInfo) error {
	paramsJSON, _ := json.Marshal(task.Parameters)
	now := time.Now().Unix()

	_, err := m.db.ExecContext(ctx,
		`INSERT INTO tasks (task_id, app_id, node_id, status, function_name, parameters, start_time, worker_id, retry_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID, task.AppID, task.NodeID, task.Status, task.FunctionName, string(paramsJSON),
		now, task.WorkerID, task.RetryCount, now, now)

	return err
}

func (m *Monitor) RecordTaskEnd(ctx context.Context, taskID string, result *monitor.TaskResult) error {
	now := time.Now().Unix()
	status := monitor.TaskStatusCompleted
	errorMsg := ""

	if !result.Success {
		status = monitor.TaskStatusFailed
		if result.ErrorDetails != nil {
			errorMsg = result.ErrorDetails.Message
		}
	}

	_, err := m.db.ExecContext(ctx,
		`UPDATE tasks SET status = ?, end_time = ?, error_message = ?, updated_at = ?,
		   duration = ? - COALESCE(start_time, ?)
		 WHERE task_id = ?`,
		status, now, errorMsg, now, now, now, taskID)

	return err
}

func (m *Monitor) GetTaskInfo(ctx context.Context, taskID string) (*monitor.TaskInfo, error) {
	var appID, nodeID, status, functionName, workerID, errorMsg sql.NullString
	var paramsJSON sql.NullString
	var startTime, endTime, duration sql.NullInt64
	var retryCount int

	err := m.db.QueryRowContext(ctx,
		`SELECT app_id, node_id, status, function_name, parameters, start_time, end_time, duration, worker_id, retry_count, error_message
		 FROM tasks WHERE task_id = ?`,
		taskID).Scan(&appID, &nodeID, &status, &functionName, &paramsJSON, &startTime, &endTime, &duration, &workerID, &retryCount, &errorMsg)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	task := &monitor.TaskInfo{
		TaskID:       taskID,
		AppID:        appID.String,
		NodeID:       nodeID.String,
		Status:       monitor.TaskStatus(status.String),
		FunctionName: functionName.String,
		WorkerID:     workerID.String,
		RetryCount:   retryCount,
		ErrorMessage: errorMsg.String,
	}

	if paramsJSON.Valid {
		json.Unmarshal([]byte(paramsJSON.String), &task.Parameters)
	}

	if startTime.Valid {
		t := time.Unix(startTime.Int64, 0)
		task.StartTime = &t
	}
	if endTime.Valid {
		t := time.Unix(endTime.Int64, 0)
		task.EndTime = &t
	}
	if duration.Valid {
		task.Duration = time.Duration(duration.Int64)
	}

	return task, nil
}

func (m *Monitor) ListTasks(ctx context.Context, appID string, filter *monitor.TaskFilter) ([]*monitor.TaskInfo, error) {
	query := `SELECT task_id FROM tasks WHERE app_id = ?`
	args := []interface{}{appID}

	if filter != nil {
		if len(filter.Status) > 0 {
			query += " AND status IN ("
			for i, status := range filter.Status {
				if i > 0 {
					query += ","
				}
				query += "?"
				args = append(args, status)
			}
			query += ")"
		}

		if filter.Limit > 0 {
			query += " LIMIT ?"
			args = append(args, filter.Limit)
		}
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*monitor.TaskInfo
	for rows.Next() {
		var taskID string
		if err := rows.Scan(&taskID); err != nil {
			continue
		}
		if task, err := m.GetTaskInfo(ctx, taskID); err == nil && task != nil {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

func (m *Monitor) GetTaskMetrics(ctx context.Context, taskID string) (*monitor.TaskMetrics, error) {
	return &monitor.TaskMetrics{TaskID: taskID}, nil
}

func (m *Monitor) CancelTask(ctx context.Context, taskID string) error {
	now := time.Now().Unix()
	_, err := m.db.ExecContext(ctx,
		`UPDATE tasks SET status = ?, updated_at = ? WHERE task_id = ?`,
		monitor.TaskStatusCancelled, now, taskID)
	return err
}

// =============================================================================
// ResourceMonitor 实现（简化）
// =============================================================================

func (m *Monitor) RecordResourceUsage(ctx context.Context, appID string, usage *monitor.ResourceUsage) error {
	return nil
}

func (m *Monitor) GetResourceUsage(ctx context.Context, appID string) (*monitor.ResourceUsage, error) {
	return &monitor.ResourceUsage{AppID: appID}, nil
}

func (m *Monitor) GetResourceHistory(ctx context.Context, appID string, timeRange *monitor.TimeRange) ([]*monitor.ResourceSnapshot, error) {
	return []*monitor.ResourceSnapshot{}, nil
}

func (m *Monitor) GetWorkerResources(ctx context.Context, workerID string) (*monitor.WorkerResources, error) {
	return &monitor.WorkerResources{WorkerID: workerID}, nil
}

func (m *Monitor) ListWorkers(ctx context.Context, filter *monitor.WorkerFilter) ([]*monitor.WorkerInfo, error) {
	return []*monitor.WorkerInfo{}, nil
}

func (m *Monitor) GetClusterResources(ctx context.Context) (*monitor.ClusterResources, error) {
	return &monitor.ClusterResources{}, nil
}

// =============================================================================
// MetricsMonitor 实现
// =============================================================================

func (m *Monitor) RecordMetric(ctx context.Context, metric *monitor.Metric) error {
	labelsJSON, _ := json.Marshal(metric.Labels)
	timestamp := metric.Timestamp.Unix()
	if metric.Timestamp.IsZero() {
		timestamp = time.Now().Unix()
	}

	_, err := m.db.ExecContext(ctx,
		`INSERT INTO metrics (name, type, value, labels, unit, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		metric.Name, metric.Type, metric.Value, string(labelsJSON), metric.Unit, timestamp)

	return err
}

func (m *Monitor) RecordMetrics(ctx context.Context, metrics []*monitor.Metric) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO metrics (name, type, value, labels, unit, timestamp) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, metric := range metrics {
		labelsJSON, _ := json.Marshal(metric.Labels)
		timestamp := metric.Timestamp.Unix()
		if metric.Timestamp.IsZero() {
			timestamp = time.Now().Unix()
		}

		if _, err := stmt.ExecContext(ctx, metric.Name, metric.Type, metric.Value, string(labelsJSON), metric.Unit, timestamp); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (m *Monitor) QueryMetrics(ctx context.Context, query *monitor.MetricQuery) ([]*monitor.Metric, error) {
	sqlQuery := `SELECT name, type, value, labels, unit, timestamp FROM metrics WHERE 1=1`
	args := []interface{}{}

	if query != nil {
		if len(query.Names) > 0 {
			sqlQuery += " AND name IN ("
			for i, name := range query.Names {
				if i > 0 {
					sqlQuery += ","
				}
				sqlQuery += "?"
				args = append(args, name)
			}
			sqlQuery += ")"
		}

		if query.TimeRange != nil {
			sqlQuery += " AND timestamp BETWEEN ? AND ?"
			args = append(args, query.TimeRange.Start.Unix(), query.TimeRange.End.Unix())
		}

		if query.Limit > 0 {
			sqlQuery += " LIMIT ?"
			args = append(args, query.Limit)
		}
	}

	rows, err := m.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []*monitor.Metric
	for rows.Next() {
		var name, metricType, unit, labelsJSON string
		var value float64
		var timestamp int64

		if err := rows.Scan(&name, &metricType, &value, &labelsJSON, &unit, &timestamp); err != nil {
			continue
		}

		var labels map[string]string
		json.Unmarshal([]byte(labelsJSON), &labels)

		metrics = append(metrics, &monitor.Metric{
			Name:      name,
			Type:      monitor.MetricType(metricType),
			Value:     value,
			Labels:    labels,
			Unit:      unit,
			Timestamp: time.Unix(timestamp, 0),
		})
	}

	return metrics, nil
}

func (m *Monitor) GetMetricAggregation(ctx context.Context, query *monitor.MetricQuery, aggregation *monitor.AggregationConfig) (*monitor.AggregationResult, error) {
	return &monitor.AggregationResult{
		Query:       query,
		Aggregation: aggregation,
		Results:     []*monitor.AggregationDataPoint{},
	}, nil
}

func (m *Monitor) ListMetricNames(ctx context.Context, prefix string) ([]string, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT DISTINCT name FROM metrics`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		names = append(names, name)
	}

	return names, nil
}

func (m *Monitor) DeleteMetrics(ctx context.Context, query *monitor.MetricQuery) error {
	sqlQuery := `DELETE FROM metrics WHERE 1=1`
	args := []interface{}{}

	if query != nil {
		if len(query.Names) > 0 {
			sqlQuery += " AND name IN ("
			for i, name := range query.Names {
				if i > 0 {
					sqlQuery += ","
				}
				sqlQuery += "?"
				args = append(args, name)
			}
			sqlQuery += ")"
		}
	}

	_, err := m.db.ExecContext(ctx, sqlQuery, args...)
	return err
}

// =============================================================================
// EventMonitor 实现
// =============================================================================

func (m *Monitor) RecordEvent(ctx context.Context, event *monitor.Event) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	detailsJSON, _ := json.Marshal(event.Details)

	_, err := m.db.ExecContext(ctx,
		`INSERT INTO events (id, type, level, app_id, node_id, task_id, worker_id, message, details, source, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.Type, event.Level, event.AppID, event.NodeID, event.TaskID, event.WorkerID,
		event.Message, string(detailsJSON), event.Source, event.Timestamp.Unix())

	return err
}

func (m *Monitor) QueryEvents(ctx context.Context, query *monitor.EventQuery) ([]*monitor.Event, error) {
	sqlQuery := `SELECT id, type, level, app_id, node_id, task_id, worker_id, message, details, source, timestamp FROM events WHERE 1=1`
	args := []interface{}{}

	if query != nil {
		if len(query.Types) > 0 {
			sqlQuery += " AND type IN ("
			for i, t := range query.Types {
				if i > 0 {
					sqlQuery += ","
				}
				sqlQuery += "?"
				args = append(args, t)
			}
			sqlQuery += ")"
		}

		if len(query.Levels) > 0 {
			sqlQuery += " AND level IN ("
			for i, level := range query.Levels {
				if i > 0 {
					sqlQuery += ","
				}
				sqlQuery += "?"
				args = append(args, level)
			}
			sqlQuery += ")"
		}

		if query.AppID != "" {
			sqlQuery += " AND app_id = ?"
			args = append(args, query.AppID)
		}

		if query.TimeRange != nil {
			sqlQuery += " AND timestamp BETWEEN ? AND ?"
			args = append(args, query.TimeRange.Start.Unix(), query.TimeRange.End.Unix())
		}

		sqlQuery += " ORDER BY timestamp"
		if query.SortOrder == "asc" {
			sqlQuery += " ASC"
		} else {
			sqlQuery += " DESC"
		}

		if query.Limit > 0 {
			sqlQuery += " LIMIT ?"
			args = append(args, query.Limit)
		}
	}

	rows, err := m.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*monitor.Event
	for rows.Next() {
		var id, eventType, level, appID, nodeID, taskID, workerID, message, detailsJSON, source string
		var timestamp int64

		if err := rows.Scan(&id, &eventType, &level, &appID, &nodeID, &taskID, &workerID, &message, &detailsJSON, &source, &timestamp); err != nil {
			continue
		}

		var details map[string]string
		json.Unmarshal([]byte(detailsJSON), &details)

		events = append(events, &monitor.Event{
			ID:        id,
			Type:      monitor.EventType(eventType),
			Level:     monitor.EventLevel(level),
			AppID:     appID,
			NodeID:    nodeID,
			TaskID:    taskID,
			WorkerID:  workerID,
			Message:   message,
			Details:   details,
			Source:    source,
			Timestamp: time.Unix(timestamp, 0),
		})
	}

	return events, nil
}

func (m *Monitor) Subscribe(ctx context.Context, filter *monitor.EventFilter) (<-chan *monitor.Event, error) {
	ch := make(chan *monitor.Event)
	close(ch)
	return ch, nil
}

func (m *Monitor) Unsubscribe(ctx context.Context, subscriptionID string) error {
	return nil
}

func (m *Monitor) GetEventStatistics(ctx context.Context, query *monitor.EventQuery) (*monitor.EventStatistics, error) {
	stats := &monitor.EventStatistics{
		EventsByType:  make(map[monitor.EventType]int64),
		EventsByLevel: make(map[monitor.EventLevel]int64),
	}

	rows, err := m.db.QueryContext(ctx, `SELECT type, level, COUNT(*) FROM events GROUP BY type, level`)
	if err != nil {
		return stats, nil
	}
	defer rows.Close()

	for rows.Next() {
		var eventType, level string
		var count int64
		if err := rows.Scan(&eventType, &level, &count); err != nil {
			continue
		}

		stats.TotalEvents += count
		stats.EventsByType[monitor.EventType(eventType)] += count
		stats.EventsByLevel[monitor.EventLevel(level)] += count
	}

	return stats, nil
}

// =============================================================================
// LifecycleManager 实现
// =============================================================================

func (m *Monitor) Start(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return nil
	}

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.running = true

	// 启动异步数据库写入线程
	m.wg.Add(1)
	go m.dbWriter()

	return nil
}

func (m *Monitor) Stop(ctx context.Context) error {
	m.mutex.Lock()
	if !m.running {
		m.mutex.Unlock()
		return nil
	}

	// 关闭异步队列并等待处理完成
	if m.cancel != nil {
		m.cancel()
	}
	m.mutex.Unlock()

	// 等待异步写入完成
	m.wg.Wait()

	// 关闭数据库
	m.mutex.Lock()
	m.running = false
	err := m.db.Close()
	m.mutex.Unlock()

	return err
}

func (m *Monitor) Status(ctx context.Context) (*monitor.MonitorStatus, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var appCount, eventCount, metricCount int64

	m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM applications`).Scan(&appCount)
	m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&eventCount)
	m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM metrics`).Scan(&metricCount)

	return &monitor.MonitorStatus{
		Running:           m.running,
		TotalApplications: int(appCount),
		TotalEvents:       eventCount,
		TotalMetrics:      metricCount,
	}, nil
}

func (m *Monitor) HealthCheck(ctx context.Context) (*monitor.HealthCheckResult, error) {
	result := &monitor.HealthCheckResult{
		Healthy:   true,
		Checks:    make(map[string]bool),
		Timestamp: time.Now(),
	}

	// 检查数据库连接
	if err := m.db.PingContext(ctx); err != nil {
		result.Healthy = false
		result.Checks["database"] = false
		result.ErrorMessages = append(result.ErrorMessages, "database connection failed")
	} else {
		result.Checks["database"] = true
	}

	result.Checks["running"] = m.running

	return result, nil
}

func (m *Monitor) Reset(ctx context.Context) error {
	tables := []string{"applications", "node_states", "tasks", "metrics", "events"}
	for _, table := range tables {
		if _, err := m.db.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return err
		}
	}
	return nil
}

func (m *Monitor) Export(ctx context.Context, config *monitor.ExportConfig) ([]byte, error) {
	return nil, nil
}

func (m *Monitor) Import(ctx context.Context, data []byte) error {
	return nil
}

// =============================================================================
// ObservableMonitor 实现
// =============================================================================

func (m *Monitor) AddObserver(observer monitor.Observer) {}

func (m *Monitor) RemoveObserver(observer monitor.Observer) {}

func (m *Monitor) NotifyObservers(event interface{}) {}
