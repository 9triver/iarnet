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

// IsActive 检查应用是否处于活跃状态
func (s Status) IsActive() bool {
	return s == StatusRunning
}

// IsInactive 检查应用是否处于非活跃状态
func (s Status) IsInactive() bool {
	return s == StatusStopped || s == StatusUndeployed
}

// HasError 检查应用是否处于错误状态
func (s Status) HasError() bool {
	return s == StatusFailed
}

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
	LastDeployed time.Time
}

func (a *AppRef) GetRunningOn() []string {
	// 应用现在通过组件部署，不再直接对应容器
	return []string{}
}

func (a *AppRef) GetLogs(lines int) ([]string, error) {
	// 应用现在通过组件部署，日志获取需要从组件层面处理
	return []string{}, nil
}
