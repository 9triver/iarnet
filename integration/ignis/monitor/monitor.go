package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/9triver/ignis/monitor"
	"github.com/9triver/ignis/proto/controller"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Monitor 纯内存的 monitor 实现
type Monitor struct {
	config    *Config
	mu        sync.RWMutex
	observers []monitor.Observer

	// 内存存储
	applications map[string]*monitor.ApplicationInfo
	nodeStates   map[string]map[string]*monitor.NodeState // appID -> nodeID -> NodeState
	tasks        map[string]*monitor.TaskInfo
	metrics      []*monitor.Metric
	events       []*monitor.Event

	running   bool
	startTime time.Time
	ctx       context.Context
	cancel    context.CancelFunc
}

// New 创建一个新的纯内存 monitor 实例
func New(config *Config) (monitor.Monitor, error) {
	if config == nil {
		config = DefaultConfig()
	}

	m := &Monitor{
		config:       config,
		applications: make(map[string]*monitor.ApplicationInfo),
		nodeStates:   make(map[string]map[string]*monitor.NodeState),
		tasks:        make(map[string]*monitor.TaskInfo),
		metrics:      make([]*monitor.Metric, 0, 1000),
		events:       make([]*monitor.Event, 0, 1000),
		observers:    make([]monitor.Observer, 0),
	}

	logrus.Info("Initialized in-memory ignis monitor")
	return m, nil
}

// ==================== LifecycleManager ====================

// Start 启动 monitor
func (m *Monitor) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("monitor already running")
	}

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.running = true
	m.startTime = time.Now()

	logrus.Info("Monitor started (in-memory mode)")
	return nil
}

// Stop 停止 monitor
func (m *Monitor) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	if m.cancel != nil {
		m.cancel()
	}

	m.running = false
	logrus.Info("Monitor stopped")
	return nil
}

// Status 获取监控系统状态
func (m *Monitor) Status(ctx context.Context) (*monitor.MonitorStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &monitor.MonitorStatus{
		Running:           m.running,
		StartTime:         m.startTime,
		Uptime:            time.Since(m.startTime),
		TotalApplications: len(m.applications),
		TotalEvents:       int64(len(m.events)),
		TotalMetrics:      int64(len(m.metrics)),
		StorageUsed:       0,
		MemoryUsed:        0, // 可以通过 runtime.MemStats 获取
	}

	return status, nil
}

// HealthCheck 健康检查
func (m *Monitor) HealthCheck(ctx context.Context) (*monitor.HealthCheckResult, error) {
	return &monitor.HealthCheckResult{
		Healthy:   m.running,
		Checks:    map[string]bool{"running": m.running},
		Timestamp: time.Now(),
	}, nil
}

// Reset 重置监控系统
func (m *Monitor) Reset(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.applications = make(map[string]*monitor.ApplicationInfo)
	m.nodeStates = make(map[string]map[string]*monitor.NodeState)
	m.tasks = make(map[string]*monitor.TaskInfo)
	m.metrics = make([]*monitor.Metric, 0, 1000)
	m.events = make([]*monitor.Event, 0, 1000)

	logrus.Info("Monitor reset")
	return nil
}

// Export 导出监控数据
func (m *Monitor) Export(ctx context.Context, config *monitor.ExportConfig) ([]byte, error) {
	// 纯内存实现，暂不支持导出
	return nil, fmt.Errorf("export not supported in memory mode")
}

// Import 导入监控数据
func (m *Monitor) Import(ctx context.Context, data []byte) error {
	// 纯内存实现，暂不支持导入
	return fmt.Errorf("import not supported in memory mode")
}

// ==================== ApplicationMonitor ====================

// RegisterApplication 注册应用
func (m *Monitor) RegisterApplication(ctx context.Context, appID string, metadata *monitor.ApplicationMetadata) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if metadata == nil {
		metadata = &monitor.ApplicationMetadata{
			AppID:     appID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	info := &monitor.ApplicationInfo{
		Metadata: metadata,
		Status:   monitor.AppStatusPending,
	}

	m.applications[appID] = info
	m.nodeStates[appID] = make(map[string]*monitor.NodeState)

	logrus.Debugf("Registered application: %s", appID)
	return nil
}

// UnregisterApplication 注销应用
func (m *Monitor) UnregisterApplication(ctx context.Context, appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.applications, appID)
	delete(m.nodeStates, appID)

	// 删除相关任务
	for taskID, task := range m.tasks {
		if task.AppID == appID {
			delete(m.tasks, taskID)
		}
	}

	logrus.Debugf("Unregistered application: %s", appID)
	return nil
}

// GetApplicationInfo 获取应用信息
func (m *Monitor) GetApplicationInfo(ctx context.Context, appID string) (*monitor.ApplicationInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.applications[appID]
	if !exists {
		return nil, fmt.Errorf("application %s not found", appID)
	}

	return info, nil
}

// ListApplications 列出所有应用
func (m *Monitor) ListApplications(ctx context.Context, filter *monitor.ApplicationFilter) ([]*monitor.ApplicationInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]*monitor.ApplicationInfo, 0, len(m.applications))
	for _, app := range m.applications {
		// 应用过滤器
		if filter != nil {
			if len(filter.Status) > 0 {
				matched := false
				for _, status := range filter.Status {
					if app.Status == status {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
		}
		apps = append(apps, app)
	}

	return apps, nil
}

// UpdateApplicationStatus 更新应用状态
func (m *Monitor) UpdateApplicationStatus(ctx context.Context, appID string, status monitor.ApplicationStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.applications[appID]
	if !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	oldStatus := info.Status
	info.Status = status

	logrus.Debugf("Application %s status changed: %s -> %s", appID, oldStatus, status)
	return nil
}

// GetApplicationDAG 获取应用的 DAG
func (m *Monitor) GetApplicationDAG(ctx context.Context, appID string) (*controller.DAG, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.applications[appID]
	if !exists {
		return nil, fmt.Errorf("application %s not found", appID)
	}

	return info.DAG, nil
}

// SetApplicationDAG 设置应用的 DAG
func (m *Monitor) SetApplicationDAG(ctx context.Context, appID string, dag *controller.DAG) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.applications[appID]
	if !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	info.DAG = dag
	logrus.Debugf("Set DAG for application: %s", appID)
	return nil
}

// GetApplicationMetrics 获取应用级别的聚合指标
func (m *Monitor) GetApplicationMetrics(ctx context.Context, appID string) (*monitor.ApplicationMetrics, error) {
	// 纯内存实现，返回空指标
	return &monitor.ApplicationMetrics{
		AppID: appID,
	}, nil
}

// ==================== NodeMonitor ====================

// GetNodeState 获取节点状态
func (m *Monitor) GetNodeState(ctx context.Context, appID, nodeID string) (*monitor.NodeState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.nodeStates[appID] == nil {
		return nil, fmt.Errorf("application %s not found", appID)
	}

	state, exists := m.nodeStates[appID][nodeID]
	if !exists {
		return nil, fmt.Errorf("node %s not found in application %s", nodeID, appID)
	}

	return state, nil
}

// GetAllNodeStates 获取应用的所有节点状态
func (m *Monitor) GetAllNodeStates(ctx context.Context, appID string) (map[string]*monitor.NodeState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states, exists := m.nodeStates[appID]
	if !exists {
		return make(map[string]*monitor.NodeState), nil
	}

	return states, nil
}

// UpdateNodeState 更新节点状态
func (m *Monitor) UpdateNodeState(ctx context.Context, appID, nodeID string, state *monitor.NodeState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.applications[appID]; !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	if m.nodeStates[appID] == nil {
		m.nodeStates[appID] = make(map[string]*monitor.NodeState)
	}

	m.nodeStates[appID][nodeID] = state
	logrus.Debugf("Updated node state: app=%s, node=%s, status=%s", appID, nodeID, state.Status)
	return nil
}

// MarkNodeReady 标记节点为就绪
func (m *Monitor) MarkNodeReady(ctx context.Context, appID, nodeID string) error {
	return m.updateNodeStatus(ctx, appID, nodeID, monitor.NodeStatusReady)
}

// MarkNodeRunning 标记节点为运行中
func (m *Monitor) MarkNodeRunning(ctx context.Context, appID, nodeID string) error {
	return m.updateNodeStatus(ctx, appID, nodeID, monitor.NodeStatusRunning)
}

// MarkNodeDone 标记节点为完成
func (m *Monitor) MarkNodeDone(ctx context.Context, appID, nodeID string, result *monitor.NodeResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.nodeStates[appID] == nil {
		m.nodeStates[appID] = make(map[string]*monitor.NodeState)
	}

	state, exists := m.nodeStates[appID][nodeID]
	if !exists {
		state = &monitor.NodeState{
			ID:     nodeID,
			Status: monitor.NodeStatusCompleted,
		}
		m.nodeStates[appID][nodeID] = state
	} else {
		state.Status = monitor.NodeStatusCompleted
	}

	if result != nil {
		state.Result = result
	}

	logrus.Debugf("Marked node done: app=%s, node=%s", appID, nodeID)
	return nil
}

// MarkNodeFailed 标记节点为失败
func (m *Monitor) MarkNodeFailed(ctx context.Context, appID, nodeID string, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.nodeStates[appID] == nil {
		m.nodeStates[appID] = make(map[string]*monitor.NodeState)
	}

	state, exists := m.nodeStates[appID][nodeID]
	if !exists {
		state = &monitor.NodeState{
			ID:     nodeID,
			Status: monitor.NodeStatusFailed,
		}
		m.nodeStates[appID][nodeID] = state
	} else {
		state.Status = monitor.NodeStatusFailed
	}

	if err != nil {
		state.ErrorMessage = err.Error()
	}

	logrus.Debugf("Marked node failed: app=%s, node=%s, error=%v", appID, nodeID, err)
	return nil
}

// GetNodeMetrics 获取节点的执行指标
func (m *Monitor) GetNodeMetrics(ctx context.Context, appID, nodeID string) (*monitor.NodeMetrics, error) {
	return &monitor.NodeMetrics{
		NodeID: nodeID,
	}, nil
}

// GetNodeDependencies 获取节点的依赖关系
func (m *Monitor) GetNodeDependencies(ctx context.Context, appID, nodeID string) (*monitor.NodeDependencies, error) {
	return &monitor.NodeDependencies{
		NodeID: nodeID,
	}, nil
}

// WatchNodeState 监听节点状态变化
func (m *Monitor) WatchNodeState(ctx context.Context, appID, nodeID string) (<-chan *monitor.NodeStateChangeEvent, error) {
	// 纯内存实现，暂不支持 watch
	ch := make(chan *monitor.NodeStateChangeEvent)
	close(ch)
	return ch, nil
}

// updateNodeStatus 更新节点状态（内部方法）
func (m *Monitor) updateNodeStatus(ctx context.Context, appID, nodeID string, status monitor.NodeStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.nodeStates[appID] == nil {
		m.nodeStates[appID] = make(map[string]*monitor.NodeState)
	}

	state, exists := m.nodeStates[appID][nodeID]
	if !exists {
		state = &monitor.NodeState{
			ID:     nodeID,
			Status: status,
		}
		m.nodeStates[appID][nodeID] = state
	} else {
		state.Status = status
	}

	logrus.Debugf("Updated node status: app=%s, node=%s, status=%s", appID, nodeID, status)
	return nil
}

// ==================== TaskMonitor ====================

// RecordTaskStart 记录任务开始
func (m *Monitor) RecordTaskStart(ctx context.Context, taskID string, taskInfo *monitor.TaskInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks[taskID] = taskInfo
	logrus.Debugf("Recorded task start: %s", taskID)
	return nil
}

// RecordTaskEnd 记录任务结束
func (m *Monitor) RecordTaskEnd(ctx context.Context, taskID string, result *monitor.TaskResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if result != nil {
		if result.Success {
			task.Status = monitor.TaskStatusCompleted
		} else {
			task.Status = monitor.TaskStatusFailed
			if result.ErrorDetails != nil {
				task.ErrorMessage = result.ErrorDetails.Message
			}
		}
	}

	logrus.Debugf("Recorded task end: %s, status=%s", taskID, task.Status)
	return nil
}

// GetTaskInfo 获取任务信息
func (m *Monitor) GetTaskInfo(ctx context.Context, taskID string) (*monitor.TaskInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return task, nil
}

// ListTasks 列出任务
func (m *Monitor) ListTasks(ctx context.Context, appID string, filter *monitor.TaskFilter) ([]*monitor.TaskInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*monitor.TaskInfo, 0)
	for _, task := range m.tasks {
		if task.AppID == appID {
			// 应用过滤器
			if filter != nil {
				if len(filter.Status) > 0 {
					matched := false
					for _, status := range filter.Status {
						if task.Status == status {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}
			}
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

// GetTaskMetrics 获取任务指标
func (m *Monitor) GetTaskMetrics(ctx context.Context, taskID string) (*monitor.TaskMetrics, error) {
	return &monitor.TaskMetrics{
		TaskID: taskID,
	}, nil
}

// CancelTask 取消任务
func (m *Monitor) CancelTask(ctx context.Context, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Status = monitor.TaskStatusCancelled
	logrus.Debugf("Cancelled task: %s", taskID)
	return nil
}

// ==================== ResourceMonitor ====================

// RecordResourceUsage 记录资源使用情况
func (m *Monitor) RecordResourceUsage(ctx context.Context, appID string, usage *monitor.ResourceUsage) error {
	// 纯内存实现，不持久化资源使用历史
	logrus.Debugf("Recorded resource usage for app %s", appID)
	return nil
}

// GetResourceUsage 获取资源使用情况
func (m *Monitor) GetResourceUsage(ctx context.Context, appID string) (*monitor.ResourceUsage, error) {
	return &monitor.ResourceUsage{
		AppID:     appID,
		Timestamp: time.Now(),
	}, nil
}

// GetResourceHistory 获取资源使用历史
func (m *Monitor) GetResourceHistory(ctx context.Context, appID string, timeRange *monitor.TimeRange) ([]*monitor.ResourceSnapshot, error) {
	return []*monitor.ResourceSnapshot{}, nil
}

// GetWorkerResources 获取Worker资源信息
func (m *Monitor) GetWorkerResources(ctx context.Context, workerID string) (*monitor.WorkerResources, error) {
	return &monitor.WorkerResources{
		WorkerID:  workerID,
		UpdatedAt: time.Now(),
	}, nil
}

// ListWorkers 列出所有Worker
func (m *Monitor) ListWorkers(ctx context.Context, filter *monitor.WorkerFilter) ([]*monitor.WorkerInfo, error) {
	return []*monitor.WorkerInfo{}, nil
}

// GetClusterResources 获取集群资源汇总
func (m *Monitor) GetClusterResources(ctx context.Context) (*monitor.ClusterResources, error) {
	return &monitor.ClusterResources{
		UpdatedAt: time.Now(),
	}, nil
}

// ==================== MetricsMonitor ====================

// RecordMetric 记录指标
func (m *Monitor) RecordMetric(ctx context.Context, metric *monitor.Metric) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 保留最近的 1000 条指标
	m.metrics = append(m.metrics, metric)
	if len(m.metrics) > 1000 {
		m.metrics = m.metrics[len(m.metrics)-1000:]
	}

	return nil
}

// RecordMetrics 批量记录指标
func (m *Monitor) RecordMetrics(ctx context.Context, metrics []*monitor.Metric) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics = append(m.metrics, metrics...)
	if len(m.metrics) > 1000 {
		m.metrics = m.metrics[len(m.metrics)-1000:]
	}

	return nil
}

// QueryMetrics 查询指标
func (m *Monitor) QueryMetrics(ctx context.Context, query *monitor.MetricQuery) ([]*monitor.Metric, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 简单过滤
	results := make([]*monitor.Metric, 0)
	for _, metric := range m.metrics {
		if query.Names != nil && len(query.Names) > 0 {
			matched := false
			for _, name := range query.Names {
				if name == metric.Name {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		results = append(results, metric)
	}

	return results, nil
}

// GetMetricAggregation 获取指标聚合结果
func (m *Monitor) GetMetricAggregation(ctx context.Context, query *monitor.MetricQuery, aggregation *monitor.AggregationConfig) (*monitor.AggregationResult, error) {
	return &monitor.AggregationResult{
		Query:       query,
		Aggregation: aggregation,
		Results:     []*monitor.AggregationDataPoint{},
	}, nil
}

// ListMetricNames 列出所有指标名称
func (m *Monitor) ListMetricNames(ctx context.Context, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nameSet := make(map[string]bool)
	for _, metric := range m.metrics {
		nameSet[metric.Name] = true
	}

	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}

	return names, nil
}

// DeleteMetrics 删除指标数据
func (m *Monitor) DeleteMetrics(ctx context.Context, query *monitor.MetricQuery) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 简单实现：清空所有指标
	m.metrics = make([]*monitor.Metric, 0, 1000)
	return nil
}

// ==================== EventMonitor ====================

// RecordEvent 记录事件
func (m *Monitor) RecordEvent(ctx context.Context, event *monitor.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成ID
	if event.ID == "" {
		event.ID = uuid.New().String()
	}

	// 设置时间戳
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// 保留最近的 1000 条事件
	m.events = append(m.events, event)
	if len(m.events) > 1000 {
		m.events = m.events[len(m.events)-1000:]
	}

	logrus.Debugf("Recorded event: type=%s, level=%s, message=%s", event.Type, event.Level, event.Message)

	// 通知观察者（不持有锁）
	go m.notifyObservers(event)

	return nil
}

// QueryEvents 查询事件
func (m *Monitor) QueryEvents(ctx context.Context, query *monitor.EventQuery) ([]*monitor.Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 简单过滤
	results := make([]*monitor.Event, 0)
	for _, event := range m.events {
		if query.Types != nil && len(query.Types) > 0 {
			matched := false
			for _, t := range query.Types {
				if event.Type == t {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if query.Levels != nil && len(query.Levels) > 0 {
			matched := false
			for _, l := range query.Levels {
				if event.Level == l {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if query.AppID != "" && event.AppID != query.AppID {
			continue
		}
		results = append(results, event)
	}

	return results, nil
}

// Subscribe 订阅事件
func (m *Monitor) Subscribe(ctx context.Context, filter *monitor.EventFilter) (<-chan *monitor.Event, error) {
	// 纯内存实现，暂不支持订阅
	ch := make(chan *monitor.Event)
	close(ch)
	return ch, nil
}

// Unsubscribe 取消订阅
func (m *Monitor) Unsubscribe(ctx context.Context, subscriptionID string) error {
	return nil
}

// GetEventStatistics 获取事件统计
func (m *Monitor) GetEventStatistics(ctx context.Context, query *monitor.EventQuery) (*monitor.EventStatistics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &monitor.EventStatistics{
		TotalEvents:   int64(len(m.events)),
		EventsByType:  make(map[monitor.EventType]int64),
		EventsByLevel: make(map[monitor.EventLevel]int64),
	}

	for _, event := range m.events {
		stats.EventsByType[event.Type]++
		stats.EventsByLevel[event.Level]++
	}

	return stats, nil
}

// ==================== ObservableMonitor ====================

// AddObserver 添加观察者
func (m *Monitor) AddObserver(observer monitor.Observer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.observers = append(m.observers, observer)
	logrus.Debugf("Added observer, total observers: %d", len(m.observers))
}

// RemoveObserver 移除观察者
func (m *Monitor) RemoveObserver(observer monitor.Observer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, obs := range m.observers {
		if obs == observer {
			m.observers = append(m.observers[:i], m.observers[i+1:]...)
			logrus.Debugf("Removed observer, total observers: %d", len(m.observers))
			return
		}
	}
}

// NotifyObservers 通知所有观察者
func (m *Monitor) NotifyObservers(event interface{}) {
	// 根据事件类型分发
	switch e := event.(type) {
	case *monitor.Event:
		m.notifyObservers(e)
	default:
		logrus.Warnf("Unknown event type: %T", event)
	}
}

// notifyObservers 通知所有观察者
func (m *Monitor) notifyObservers(event *monitor.Event) {
	// 不持有锁进行通知，避免死锁
	m.mu.RLock()
	observers := make([]monitor.Observer, len(m.observers))
	copy(observers, m.observers)
	m.mu.RUnlock()

	for _, observer := range observers {
		go func(obs monitor.Observer) {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("Observer panic: %v", r)
				}
			}()
			obs.OnEvent(event)
		}(observer)
	}
}
