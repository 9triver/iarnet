package application

import (
	"time"
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
	Name       string          `json:"name"`
	Image      string          `json:"image"`
	Status     ComponentStatus `json:"status"`
	DeployedAt time.Time       `json:"deployed_at,omitempty"` // 部署时间
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
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

// DAG 结构已移至 compute 包，这里不再维护
// 使用 compute.DAG, compute.DAGNode, compute.ControlNode, compute.DataNode 等
