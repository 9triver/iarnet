package compute

import (
	"context"
	"time"
)

// Engine 计算引擎抽象接口
// 计算引擎作为独立的服务运行，用户应用主动连接并提交任务
// Engine 负责管理计算平台的生命周期和监控其运行状态
type Engine interface {
	// Start 启动计算引擎
	Start(ctx context.Context) error

	// Stop 停止计算引擎
	Stop(ctx context.Context) error

	// GetAddress 获取计算引擎的连接地址（供用户应用连接）
	GetAddress() string

	// GetStatus 获取引擎状态
	GetStatus(ctx context.Context) (*EngineStatus, error)

	// ListApplications 列出引擎中的所有应用（由用户提交的）
	ListApplications(ctx context.Context, filter *ApplicationFilter) ([]*ApplicationInfo, error)

	// GetApplicationInfo 获取应用详细信息
	GetApplicationInfo(ctx context.Context, appID string) (*ApplicationInfo, error)

	// GetApplicationDAG 获取应用的 DAG
	GetApplicationDAG(ctx context.Context, appID string) (*DAG, error)

	// GetApplicationMetrics 获取应用的执行指标
	GetApplicationMetrics(ctx context.Context, appID string) (*Metrics, error)

	// GetNodeStates 获取应用的节点状态
	GetNodeStates(ctx context.Context, appID string) (map[string]*NodeState, error)

	// QueryEvents 查询引擎事件
	QueryEvents(ctx context.Context, query *EventQuery) ([]*Event, error)

	// Subscribe 订阅引擎事件（用于实时监控）
	Subscribe(ctx context.Context, filter *EventFilter) (<-chan *Event, error)

	// GetClusterInfo 获取集群信息
	GetClusterInfo(ctx context.Context) (*ClusterInfo, error)
}

// EngineStatus 引擎状态
type EngineStatus struct {
	Running             bool          `json:"running"`
	Address             string        `json:"address"`
	Uptime              time.Duration `json:"uptime"`
	TotalApplications   int           `json:"totalApplications"`
	RunningApplications int           `json:"runningApplications"`
	TotalWorkers        int           `json:"totalWorkers"`
	ActiveWorkers       int           `json:"activeWorkers"`
}

// ApplicationStatus 应用状态
type ApplicationStatus string

const (
	AppStatusPending   ApplicationStatus = "pending"
	AppStatusRunning   ApplicationStatus = "running"
	AppStatusCompleted ApplicationStatus = "completed"
	AppStatusFailed    ApplicationStatus = "failed"
	AppStatusCancelled ApplicationStatus = "cancelled"
)

// ApplicationInfo 应用信息（用户提交到引擎的应用）
type ApplicationInfo struct {
	AppID       string            `json:"appId"`       // 引擎中的应用 ID
	Name        string            `json:"name"`        // 应用名称
	Status      ApplicationStatus `json:"status"`      // 当前状态
	SubmittedAt time.Time         `json:"submittedAt"` // 提交时间
	StartedAt   *time.Time        `json:"startedAt,omitempty"`
	CompletedAt *time.Time        `json:"completedAt,omitempty"`
	Duration    time.Duration     `json:"duration"`
	Progress    *Progress         `json:"progress,omitempty"`
	Error       string            `json:"error,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"` // 标签（如 iarnet_app_id）
}

// Progress 执行进度
type Progress struct {
	TotalNodes     int     `json:"totalNodes"`
	CompletedNodes int     `json:"completedNodes"`
	FailedNodes    int     `json:"failedNodes"`
	RunningNodes   int     `json:"runningNodes"`
	PendingNodes   int     `json:"pendingNodes"`
	Percentage     float64 `json:"percentage"`
}

// ApplicationFilter 应用过滤器
type ApplicationFilter struct {
	Status    []ApplicationStatus `json:"status,omitempty"`
	Tags      map[string]string   `json:"tags,omitempty"`
	StartTime *TimeRange          `json:"startTime,omitempty"`
	Limit     int                 `json:"limit,omitempty"`
	Offset    int                 `json:"offset,omitempty"`
}

// DAG 有向无环图
type DAG struct {
	Nodes []*DAGNode `json:"nodes,omitempty"`
	Edges []*DAGEdge `json:"edges,omitempty"`
}

// GetNodes 获取节点列表
func (d *DAG) GetNodes() []*DAGNode {
	if d != nil {
		return d.Nodes
	}
	return nil
}

// DAGNode DAG 节点
type DAGNode struct {
	Type string      `json:"type,omitempty"` // "ControlNode" or "DataNode"
	Node interface{} `json:"node,omitempty"` // ControlNode 或 DataNode
}

// GetType 获取节点类型
func (n *DAGNode) GetType() string {
	if n != nil {
		return n.Type
	}
	return ""
}

// GetControlNode 获取控制节点
func (n *DAGNode) GetControlNode() *ControlNode {
	if n != nil && n.Type == "ControlNode" {
		if cn, ok := n.Node.(*ControlNode); ok {
			return cn
		}
	}
	return nil
}

// GetDataNode 获取数据节点
func (n *DAGNode) GetDataNode() *DataNode {
	if n != nil && n.Type == "DataNode" {
		if dn, ok := n.Node.(*DataNode); ok {
			return dn
		}
	}
	return nil
}

// ControlNode 控制节点
type ControlNode struct {
	Id           string            `json:"id,omitempty"`
	Done         bool              `json:"done,omitempty"`
	FunctionName string            `json:"functionName,omitempty"`
	Params       map[string]string `json:"params,omitempty"`
	Current      int32             `json:"current,omitempty"`
}

// DataNode 数据节点
type DataNode struct {
	Id         string   `json:"id,omitempty"`
	Done       bool     `json:"done,omitempty"`
	Lambda     string   `json:"lambda,omitempty"`
	Ready      bool     `json:"ready,omitempty"`
	ParentNode *string  `json:"parentNode,omitempty"`
	ChildNode  []string `json:"childNode,omitempty"`
}

// GetId 获取节点ID
func (cn *ControlNode) GetId() string {
	if cn != nil {
		return cn.Id
	}
	return ""
}

// GetDone 获取完成状态
func (cn *ControlNode) GetDone() bool {
	if cn != nil {
		return cn.Done
	}
	return false
}

// GetFunctionName 获取函数名
func (cn *ControlNode) GetFunctionName() string {
	if cn != nil {
		return cn.FunctionName
	}
	return ""
}

// GetParams 获取参数
func (cn *ControlNode) GetParams() map[string]string {
	if cn != nil {
		return cn.Params
	}
	return nil
}

// GetCurrent 获取当前值
func (cn *ControlNode) GetCurrent() int32 {
	if cn != nil {
		return cn.Current
	}
	return 0
}

// GetId 获取数据节点ID
func (dn *DataNode) GetId() string {
	if dn != nil {
		return dn.Id
	}
	return ""
}

// GetDone 获取完成状态
func (dn *DataNode) GetDone() bool {
	if dn != nil {
		return dn.Done
	}
	return false
}

// GetLambda 获取 Lambda
func (dn *DataNode) GetLambda() string {
	if dn != nil {
		return dn.Lambda
	}
	return ""
}

// GetReady 获取就绪状态
func (dn *DataNode) GetReady() bool {
	if dn != nil {
		return dn.Ready
	}
	return false
}

// GetParentNode 获取父节点
func (dn *DataNode) GetParentNode() string {
	if dn != nil && dn.ParentNode != nil {
		return *dn.ParentNode
	}
	return ""
}

// GetChildNode 获取子节点
func (dn *DataNode) GetChildNode() []string {
	if dn != nil {
		return dn.ChildNode
	}
	return nil
}

// DAGEdge DAG 边
type DAGEdge struct {
	FromNodeID string `json:"fromNodeId,omitempty"`
	ToNodeID   string `json:"toNodeId,omitempty"`
	Info       string `json:"info,omitempty"`
}

// NodeState 节点状态
type NodeState struct {
	ID        string        `json:"id"`
	Type      string        `json:"type"`
	Status    string        `json:"status"`
	StartTime *time.Time    `json:"startTime,omitempty"`
	EndTime   *time.Time    `json:"endTime,omitempty"`
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
}

// Metrics 执行指标
type Metrics struct {
	TotalExecutionTime     time.Duration `json:"totalExecutionTime"`
	AverageNodeLatency     time.Duration `json:"averageNodeLatency"`
	TotalDataProcessed     int64         `json:"totalDataProcessed"`
	TotalMessagesExchanged int64         `json:"totalMessagesExchanged"`
	NodesExecuted          int           `json:"nodesExecuted"`
	ErrorRate              float64       `json:"errorRate"`
}

// ClusterInfo 集群信息
type ClusterInfo struct {
	TotalWorkers  int       `json:"totalWorkers"`
	OnlineWorkers int       `json:"onlineWorkers"`
	TotalCPUCores int       `json:"totalCpuCores"`
	UsedCPUCores  float64   `json:"usedCpuCores"`
	TotalMemory   int64     `json:"totalMemory"`
	UsedMemory    int64     `json:"usedMemory"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Event 引擎事件
type Event struct {
	Type      EventType              `json:"type"`
	AppID     string                 `json:"appId,omitempty"`
	NodeID    string                 `json:"nodeId,omitempty"`
	Message   string                 `json:"message"`
	Level     string                 `json:"level"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// EventType 事件类型
type EventType string

const (
	EventTypeAppRegistered EventType = "app.registered"
	EventTypeAppStarted    EventType = "app.started"
	EventTypeAppCompleted  EventType = "app.completed"
	EventTypeAppFailed     EventType = "app.failed"
	EventTypeNodeCompleted EventType = "node.completed"
	EventTypeNodeFailed    EventType = "node.failed"
	EventTypeWorkerJoined  EventType = "worker.joined"
	EventTypeWorkerLeft    EventType = "worker.left"
)

// EventQuery 事件查询
type EventQuery struct {
	AppID     string      `json:"appId,omitempty"`
	Types     []EventType `json:"types,omitempty"`
	Levels    []string    `json:"levels,omitempty"`
	TimeRange *TimeRange  `json:"timeRange,omitempty"`
	Limit     int         `json:"limit,omitempty"`
}

// EventFilter 事件过滤器
type EventFilter struct {
	AppID  string      `json:"appId,omitempty"`
	Types  []EventType `json:"types,omitempty"`
	Levels []string    `json:"levels,omitempty"`
}

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
