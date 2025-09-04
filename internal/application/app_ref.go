package application

import "github.com/9triver/iarnet/internal/resource"

type Status int32

const (
	StatusRunning    Status = iota // 运行中
	StatusStopped                  // 已停止
	StatusFailed                   // 失败
	StatusUnknown                  // 未知
	StatusUndeployed               // 未部署
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
	return s == StatusFailed || s == StatusUnknown
}

type AppRef struct {
	ID           string
	Name         string
	ContainerRef *resource.ContainerRef
	Status       Status
}
