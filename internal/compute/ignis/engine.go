package ignis

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/9triver/iarnet/integration/ignis/deployer"
	ignisMonitor "github.com/9triver/iarnet/integration/ignis/monitor"
	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/compute"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/ignis/monitor"
	"github.com/9triver/ignis/platform"
	"github.com/sirupsen/logrus"
)

// Engine ignis 计算引擎实现
// 负责管理 ignis platform 的生命周期和监控其运行状态
// 用户应用通过 lucas 库主动连接 ignis 并提交任务
type Engine struct {
	platform *platform.Platform
	monitor  monitor.Monitor
	deployer *deployer.Deployer
	address  string // ignis 连接地址

	// 依赖
	appMgr *application.Manager
	resMgr *resource.Manager
	cfg    *config.Config

	// 生命周期
	ctx       context.Context
	cancel    context.CancelFunc
	running   bool
	startTime time.Time
	mu        sync.RWMutex
}

// NewEngine 创建 ignis 引擎实例
func NewEngine(ctx context.Context, cfg *config.Config,
	appMgr *application.Manager, resMgr *resource.Manager,
	cm *deployer.ConnectionManager) (*Engine, error) {

	if cfg.Ignis.Port == 0 {
		return nil, fmt.Errorf("ignis port not configured")
	}

	// 构建 ignis 地址
	ignisAddr := "0.0.0.0:" + strconv.FormatInt(int64(cfg.Ignis.Port), 10)

	// 创建 ignis monitor
	monitorConfig := &ignisMonitor.Config{
		MaxApplications: 1000,
	}
	mon, err := ignisMonitor.New(monitorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ignis monitor: %w", err)
	}

	// 启动 monitor
	if err := mon.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start ignis monitor: %w", err)
	}

	// 创建 deployer
	dep := deployer.NewDeployer(appMgr, resMgr, cm, cfg)

	// 创建 ignis platform
	plat := platform.NewPlatform(ctx, ignisAddr, dep, mon)

	engineCtx, cancel := context.WithCancel(ctx)

	engine := &Engine{
		platform:  plat,
		monitor:   mon,
		deployer:  dep,
		address:   ignisAddr,
		appMgr:    appMgr,
		resMgr:    resMgr,
		cfg:       cfg,
		ctx:       engineCtx,
		cancel:    cancel,
		startTime: time.Now(),
	}

	return engine, nil
}

// Start 启动引擎
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	// 启动 ignis platform（阻塞运行，需要在 goroutine 中）
	go func() {
		if err := e.platform.Run(); err != nil {
			logrus.Errorf("ignis platform error: %v", err)
		}
	}()

	e.running = true
	e.startTime = time.Now()
	logrus.Infof("ignis engine started, listening on %s", e.address)

	return nil
}

// Stop 停止引擎
func (e *Engine) Stop(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	// 停止 platform
	if e.cancel != nil {
		e.cancel()
	}

	// 停止 monitor
	if err := e.monitor.Stop(ctx); err != nil {
		logrus.Errorf("failed to stop monitor: %v", err)
	}

	e.running = false
	logrus.Info("ignis engine stopped")

	return nil
}

// GetAddress 获取引擎连接地址（供用户应用连接）
func (e *Engine) GetAddress() string {
	return e.address
}

// GetStatus 获取引擎状态
func (e *Engine) GetStatus(ctx context.Context) (*compute.EngineStatus, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status := &compute.EngineStatus{
		Running: e.running,
		Address: e.address,
		Uptime:  time.Since(e.startTime),
	}

	// 从 monitor 获取统计信息
	if monStatus, err := e.monitor.Status(ctx); err == nil {
		status.TotalApplications = monStatus.TotalApplications
	}

	// 从 monitor 获取应用列表并统计运行中的应用
	if apps, err := e.monitor.ListApplications(ctx, &monitor.ApplicationFilter{
		Status: []monitor.ApplicationStatus{monitor.AppStatusRunning},
	}); err == nil {
		status.RunningApplications = len(apps)
	}

	// 从 monitor 获取集群资源
	if cluster, err := e.monitor.GetClusterResources(ctx); err == nil {
		status.TotalWorkers = cluster.TotalWorkers
		status.ActiveWorkers = cluster.OnlineWorkers
	}

	return status, nil
}

// ListApplications 列出引擎中的所有应用
func (e *Engine) ListApplications(ctx context.Context, filter *compute.ApplicationFilter) ([]*compute.ApplicationInfo, error) {
	// 构建 monitor 过滤器
	var monitorFilter *monitor.ApplicationFilter
	if filter != nil {
		monitorFilter = &monitor.ApplicationFilter{
			Tags:   filter.Tags,
			Limit:  filter.Limit,
			Offset: filter.Offset,
		}

		// 转换状态过滤
		if len(filter.Status) > 0 {
			monitorFilter.Status = make([]monitor.ApplicationStatus, len(filter.Status))
			for i, status := range filter.Status {
				monitorFilter.Status[i] = monitor.ApplicationStatus(status)
			}
		}
	}

	apps, err := e.monitor.ListApplications(ctx, monitorFilter)
	if err != nil {
		return nil, err
	}

	// 转换为 compute 格式
	result := make([]*compute.ApplicationInfo, 0, len(apps))
	for _, app := range apps {
		info := &compute.ApplicationInfo{
			AppID:       app.Metadata.AppID,
			Name:        app.Metadata.Name,
			Status:      compute.ApplicationStatus(app.Status),
			SubmittedAt: app.Metadata.CreatedAt,
			StartedAt:   app.StartTime,
			CompletedAt: app.EndTime,
			Duration:    app.Duration,
			Tags:        app.Metadata.Tags,
		}

		if app.Progress != nil {
			info.Progress = &compute.Progress{
				TotalNodes:     app.Progress.TotalNodes,
				CompletedNodes: app.Progress.CompletedNodes,
				FailedNodes:    app.Progress.FailedNodes,
				RunningNodes:   app.Progress.RunningNodes,
				PendingNodes:   app.Progress.PendingNodes,
				Percentage:     app.Progress.Percentage,
			}
		}

		if app.ErrorMessage != "" {
			info.Error = app.ErrorMessage
		}

		result = append(result, info)
	}

	return result, nil
}

// GetApplicationInfo 获取应用详细信息
func (e *Engine) GetApplicationInfo(ctx context.Context, appID string) (*compute.ApplicationInfo, error) {
	app, err := e.monitor.GetApplicationInfo(ctx, appID)
	if err != nil || app == nil {
		return nil, err
	}

	info := &compute.ApplicationInfo{
		AppID:       app.Metadata.AppID,
		Name:        app.Metadata.Name,
		Status:      compute.ApplicationStatus(app.Status),
		SubmittedAt: app.Metadata.CreatedAt,
		StartedAt:   app.StartTime,
		CompletedAt: app.EndTime,
		Duration:    app.Duration,
		Tags:        app.Metadata.Tags,
	}

	if app.Progress != nil {
		info.Progress = &compute.Progress{
			TotalNodes:     app.Progress.TotalNodes,
			CompletedNodes: app.Progress.CompletedNodes,
			FailedNodes:    app.Progress.FailedNodes,
			RunningNodes:   app.Progress.RunningNodes,
			PendingNodes:   app.Progress.PendingNodes,
			Percentage:     app.Progress.Percentage,
		}
	}

	if app.ErrorMessage != "" {
		info.Error = app.ErrorMessage
	}

	return info, nil
}

// GetApplicationDAG 获取应用 DAG（返回 application.DAG 格式）
func (e *Engine) GetApplicationDAG(ctx context.Context, appID string) (*compute.DAG, error) {
	// 从 monitor 获取 DAG
	dag, err := e.monitor.GetApplicationDAG(ctx, appID)
	if err != nil || dag == nil {
		return nil, err
	}

	// 获取节点状态
	nodeStates, err := e.monitor.GetAllNodeStates(ctx, appID)
	if err != nil {
		nodeStates = make(map[string]*monitor.NodeState)
	}

	// 转换为 compute.DAG 格式
	computeDAG := &compute.DAG{
		Nodes: make([]*compute.DAGNode, 0),
		Edges: make([]*compute.DAGEdge, 0),
	}

	// 转换节点
	for _, node := range dag.Nodes {
		var dagNode *compute.DAGNode

		if node.Type == "ControlNode" && node.GetControlNode() != nil {
			cn := node.GetControlNode()
			nodeState := nodeStates[cn.Id]

			done := false
			if nodeState != nil {
				done = nodeState.Status == monitor.NodeStatusCompleted
			}

			dagNode = &compute.DAGNode{
				Type: "ControlNode",
				Node: &compute.ControlNode{
					Id:           cn.Id,
					Done:         done,
					FunctionName: cn.FunctionName,
					Params:       cn.Params,
					Current:      cn.Current,
				},
			}
		} else if node.Type == "DataNode" && node.GetDataNode() != nil {
			dn := node.GetDataNode()
			nodeState := nodeStates[dn.Id]

			done := false
			ready := false
			if nodeState != nil {
				done = nodeState.Status == monitor.NodeStatusCompleted
				ready = nodeState.Status == monitor.NodeStatusReady || nodeState.Status == monitor.NodeStatusRunning
			}

			dataNode := &compute.DataNode{
				Id:         dn.Id,
				Done:       done,
				Lambda:     dn.Lambda,
				Ready:      ready,
				ParentNode: dn.ParentNode,
				ChildNode:  dn.ChildNode,
			}

			dagNode = &compute.DAGNode{
				Type: "DataNode",
				Node: dataNode,
			}
		}

		if dagNode != nil {
			computeDAG.Nodes = append(computeDAG.Nodes, dagNode)
		}
	}

	// 构建 DataNode 映射（用于查找 lambda）
	dataNodeMap := make(map[string]string) // nodeID -> lambda
	for _, node := range dag.Nodes {
		if node.Type == "DataNode" && node.GetDataNode() != nil {
			dn := node.GetDataNode()
			dataNodeMap[dn.Id] = dn.Lambda
		}
	}

	// 转换边
	for _, node := range dag.Nodes {
		if node.Type == "ControlNode" && node.GetControlNode() != nil {
			cn := node.GetControlNode()

			// 从 PreDataNodes 到 ControlNode 的边
			for _, preDataNodeID := range cn.PreDataNodes {
				// 查找对应的参数名
				info := ""
				if lambdaID, exists := dataNodeMap[preDataNodeID]; exists {
					if cn.Params != nil {
						if paramName, ok := cn.Params[lambdaID]; ok {
							info = paramName
						}
					}
				}

				computeDAG.Edges = append(computeDAG.Edges, &compute.DAGEdge{
					FromNodeID: preDataNodeID,
					ToNodeID:   cn.Id,
					Info:       info,
				})
			}

			// 从 ControlNode 到输出 DataNode 的边
			if cn.DataNode != "" {
				computeDAG.Edges = append(computeDAG.Edges, &compute.DAGEdge{
					FromNodeID: cn.Id,
					ToNodeID:   cn.DataNode,
					Info:       "", // 控制节点到数据节点的边通常没有信息
				})
			}
		}
	}

	return computeDAG, nil
}

// GetApplicationMetrics 获取应用指标
func (e *Engine) GetApplicationMetrics(ctx context.Context, appID string) (*compute.Metrics, error) {
	metrics, err := e.monitor.GetApplicationMetrics(ctx, appID)
	if err != nil {
		return nil, err
	}

	return &compute.Metrics{
		TotalExecutionTime:     metrics.TotalExecutionTime,
		AverageNodeLatency:     metrics.AverageNodeLatency,
		TotalDataProcessed:     metrics.TotalDataProcessed,
		TotalMessagesExchanged: metrics.TotalMessagesExchanged,
		ErrorRate:              metrics.ErrorRate,
	}, nil
}

// GetNodeStates 获取应用的所有节点状态
func (e *Engine) GetNodeStates(ctx context.Context, appID string) (map[string]*compute.NodeState, error) {
	states, err := e.monitor.GetAllNodeStates(ctx, appID)
	if err != nil {
		return nil, err
	}

	// 转换为 compute 格式
	result := make(map[string]*compute.NodeState)
	for id, state := range states {
		result[id] = &compute.NodeState{
			ID:        state.ID,
			Type:      string(state.Type),
			Status:    string(state.Status),
			StartTime: state.StartTime,
			EndTime:   state.EndTime,
			Duration:  state.Duration,
			Error:     state.ErrorMessage,
		}
	}

	return result, nil
}

// QueryEvents 查询引擎事件
func (e *Engine) QueryEvents(ctx context.Context, query *compute.EventQuery) ([]*compute.Event, error) {
	// 构建 monitor 查询
	monitorQuery := &monitor.EventQuery{
		AppID: query.AppID,
		Limit: query.Limit,
	}

	if query.TimeRange != nil {
		monitorQuery.TimeRange = &monitor.TimeRange{
			Start: query.TimeRange.Start,
			End:   query.TimeRange.End,
		}
	}

	if len(query.Types) > 0 {
		monitorQuery.Types = make([]monitor.EventType, len(query.Types))
		for i, t := range query.Types {
			monitorQuery.Types[i] = monitor.EventType(t)
		}
	}

	if len(query.Levels) > 0 {
		monitorQuery.Levels = make([]monitor.EventLevel, len(query.Levels))
		for i, l := range query.Levels {
			monitorQuery.Levels[i] = monitor.EventLevel(l)
		}
	}

	events, err := e.monitor.QueryEvents(ctx, monitorQuery)
	if err != nil {
		return nil, err
	}

	// 转换为 compute 事件
	result := make([]*compute.Event, 0, len(events))
	for _, event := range events {
		result = append(result, &compute.Event{
			Type:      compute.EventType(event.Type),
			AppID:     event.AppID,
			NodeID:    event.NodeID,
			Message:   event.Message,
			Level:     string(event.Level),
			Timestamp: event.Timestamp,
			Data:      convertDetails(event.Details),
		})
	}

	return result, nil
}

// Subscribe 订阅引擎事件
func (e *Engine) Subscribe(ctx context.Context, filter *compute.EventFilter) (<-chan *compute.Event, error) {
	// 订阅 monitor 事件
	monitorFilter := &monitor.EventFilter{
		AppID: filter.AppID,
	}

	if len(filter.Types) > 0 {
		monitorFilter.Types = make([]monitor.EventType, len(filter.Types))
		for i, t := range filter.Types {
			monitorFilter.Types[i] = monitor.EventType(t)
		}
	}

	if len(filter.Levels) > 0 {
		monitorFilter.Levels = make([]monitor.EventLevel, len(filter.Levels))
		for i, l := range filter.Levels {
			monitorFilter.Levels[i] = monitor.EventLevel(l)
		}
	}

	monitorCh, err := e.monitor.Subscribe(ctx, monitorFilter)
	if err != nil {
		return nil, err
	}

	// 转换 monitor 事件为 compute 事件
	computeCh := make(chan *compute.Event, 100)
	go func() {
		defer close(computeCh)
		for monitorEvent := range monitorCh {
			computeEvent := &compute.Event{
				Type:      compute.EventType(monitorEvent.Type),
				AppID:     monitorEvent.AppID,
				NodeID:    monitorEvent.NodeID,
				Message:   monitorEvent.Message,
				Level:     string(monitorEvent.Level),
				Timestamp: monitorEvent.Timestamp,
				Data:      convertDetails(monitorEvent.Details),
			}
			select {
			case computeCh <- computeEvent:
			case <-ctx.Done():
				return
			}
		}
	}()

	return computeCh, nil
}

// GetClusterInfo 获取集群信息
func (e *Engine) GetClusterInfo(ctx context.Context) (*compute.ClusterInfo, error) {
	cluster, err := e.monitor.GetClusterResources(ctx)
	if err != nil {
		return nil, err
	}

	return &compute.ClusterInfo{
		TotalWorkers:  cluster.TotalWorkers,
		OnlineWorkers: cluster.OnlineWorkers,
		TotalCPUCores: cluster.TotalCPUCores,
		UsedCPUCores:  cluster.UsedCPUCores,
		TotalMemory:   cluster.TotalMemory,
		UsedMemory:    cluster.UsedMemory,
		UpdatedAt:     cluster.UpdatedAt,
	}, nil
}

// GetPlatform 获取底层的 ignis platform（用于特殊需求）
func (e *Engine) GetPlatform() *platform.Platform {
	return e.platform
}

// GetMonitor 获取 ignis monitor（用于高级查询）
func (e *Engine) GetMonitor() monitor.Monitor {
	return e.monitor
}

// convertDetails 转换详情为通用 map
func convertDetails(details map[string]string) map[string]interface{} {
	if details == nil {
		return nil
	}

	result := make(map[string]interface{})
	for k, v := range details {
		result[k] = v
	}
	return result
}
