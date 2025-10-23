package application

import (
	"time"

	"github.com/9triver/iarnet/internal/resource"
)

type Status string

const (
	StatusRunning    Status = "running"   // 运行中
	StatusStopped    Status = "stopped"   // 已停止
	StatusFailed     Status = "error"     // 失败
	StatusUndeployed Status = "idle"      // 未部署
	StatusDeploying  Status = "deploying" // 部署中
)

type AppRef struct {
	ID           string
	Name         string
	Status       Status
	GitUrl       *string
	Branch       *string
	Type         string // "web", "api", "worker", "database"
	Description  *string
	Ports        []int
	HealthCheck  *string
	ContainerID  *string
	LastDeployed time.Time
	ExecuteCmd   *string
	CodeDir      *string
	RunnerEnv    *string
	Components   map[string]*Component
}

func (a *AppRef) GetRunningOn() []string {
	// 应用现在通过组件部署，不再直接对应容器
	return []string{}
}

// Component 表示应用的一个组件 - 可分布式部署的actor执行单元
type Component struct {
	Name         string          `json:"name"`
	Image        string          `json:"image"`
	Status       ComponentStatus `json:"status"`
	DeployedAt   time.Time       `json:"deployed_at,omitempty"` // 部署时间
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	ContainerRef *resource.ContainerRef
}

// ComponentStatus 组件状态
type ComponentStatus string

const (
	ComponentStatusPending   ComponentStatus = "pending"
	ComponentStatusDeploying ComponentStatus = "deploying"
	ComponentStatusRunning   ComponentStatus = "running"
	ComponentStatusStopped   ComponentStatus = "stopped"
	ComponentStatusFailed    ComponentStatus = "failed"
)

// DAG Node Messages
type ControlNode struct {
	Id           string            `json:"id,omitempty"`
	Done         bool              `json:"done,omitempty"`
	FunctionName string            `json:"functionName,omitempty"`
	Params       map[string]string `json:"params,omitempty"` // lambda_id -> parameter_name mapping
	Current      int32             `json:"current,omitempty"`
	LastUpdated  time.Time         `json:"lastUpdated,omitempty"` // 最后更新时间
}

func (x *ControlNode) isDAGNode_Node() {}

func (x *ControlNode) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *ControlNode) GetDone() bool {
	if x != nil {
		return x.Done
	}
	return false
}

func (x *ControlNode) GetFunctionName() string {
	if x != nil {
		return x.FunctionName
	}
	return ""
}

func (x *ControlNode) GetParams() map[string]string {
	if x != nil {
		return x.Params
	}
	return nil
}

func (x *ControlNode) GetCurrent() int32 {
	if x != nil {
		return x.Current
	}
	return 0
}

type DataNode struct {
	Id          string    `json:"id,omitempty"`
	Done        bool      `json:"done,omitempty"`
	Lambda      string    `json:"lambda,omitempty"` // lambda id
	Ready       bool      `json:"ready,omitempty"`
	ParentNode  *string   `json:"parentNode,omitempty"` // id of parent data node (nullable)
	ChildNode   []string  `json:"childNode,omitempty"`  // ids of child data nodes
	LastUpdated time.Time `json:"lastUpdated,omitempty"` // 最后更新时间
}

func (x *DataNode) isDAGNode_Node() {}

func (x *DataNode) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *DataNode) GetDone() bool {
	if x != nil {
		return x.Done
	}
	return false
}

func (x *DataNode) GetLambda() string {
	if x != nil {
		return x.Lambda
	}
	return ""
}

func (x *DataNode) GetReady() bool {
	if x != nil {
		return x.Ready
	}
	return false
}

func (x *DataNode) GetParentNode() string {
	if x != nil && x.ParentNode != nil {
		return *x.ParentNode
	}
	return ""
}

func (x *DataNode) GetChildNode() []string {
	if x != nil {
		return x.ChildNode
	}
	return nil
}

type DAGNode struct {
	Type string `json:"type,omitempty"` // "ControlNode" or "DataNode"
	// Types that are valid to be assigned to Node:
	//
	//	*ControlNode
	//	*DataNode
	Node isDAGNode_Node `json:"node,omitempty"`
}

func (x *DAGNode) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *DAGNode) GetNode() isDAGNode_Node {
	if x != nil {
		return x.Node
	}
	return nil
}

func (x *DAGNode) GetControlNode() *ControlNode {
	if x != nil {
		if x, ok := x.Node.(*ControlNode); ok {
			return x
		}
	}
	return nil
}

func (x *DAGNode) GetDataNode() *DataNode {
	if x != nil {
		if x, ok := x.Node.(*DataNode); ok {
			return x
		}
	}
	return nil
}

type isDAGNode_Node interface {
	isDAGNode_Node()
}

type DAGEdge struct {
	FromNodeID string `json:"fromNodeId,omitempty"`
	ToNodeID   string `json:"toNodeId,omitempty"`
	Info       string `json:"info,omitempty"`
}

type DAG struct {
	Nodes []*DAGNode `json:"nodes,omitempty"`
	Edges []*DAGEdge `json:"edges,omitempty"`
}

func (x *DAG) GetNodes() []*DAGNode {
	if x != nil {
		return x.Nodes
	}
	return nil
}
