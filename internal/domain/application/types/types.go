package types

import (
	"time"

	ignistypes "github.com/9triver/iarnet/internal/domain/ignis/types"
	resourcetypes "github.com/9triver/iarnet/internal/domain/resource/types"
)

// 应用领域类型
// application 依赖 ignis 和 resource，可以使用两者的类型

// AppID 应用标识符
type AppID = string

// FileInfo 文件信息
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

// AppStatus 应用状态
type AppStatus string

const (
	AppStatusRunning    AppStatus = "running"   // 运行中
	AppStatusStopped    AppStatus = "stopped"   // 已停止
	AppStatusFailed     AppStatus = "error"     // 失败
	AppStatusUndeployed AppStatus = "idle"      // 未部署
	AppStatusDeploying  AppStatus = "deploying" // 部署中
	AppStatusCloning    AppStatus = "cloning"   // 克隆中
)

// AppMetadata 应用元数据
type AppMetadata struct {
	ID            AppID
	Name          string
	Status        AppStatus
	GitUrl        string
	Branch        string
	Description   string
	ContainerID   string
	LastDeployed  time.Time
	ExecuteCmd    string
	EnvInstallCmd string
	RunnerEnv     string
}

// RunnerEnv 运行环境类型
type RunnerEnv = string

const (
	RunnerEnvPython RunnerEnv = "python"
	RunnerEnvGo     RunnerEnv = "go"
	RunnerEnvJava   RunnerEnv = "java"
)

// RunnerStatus Runner 状态
type RunnerStatus string

const (
	RunnerStatusUnknown  RunnerStatus = "unknown"
	RunnerStatusIdle     RunnerStatus = "idle"
	RunnerStatusRunning  RunnerStatus = "running"
	RunnerStatusStarting RunnerStatus = "starting"
	RunnerStatusStopping RunnerStatus = "stopping"
	RunnerStatusFailed   RunnerStatus = "failed"
	RunnerStatusStopped  RunnerStatus = "stopped"
)

// 重新导出 ignis 领域类型以便使用
type ActorID = ignistypes.ActorID
type SessionID = ignistypes.SessionID
type RuntimeID = ignistypes.RuntimeID

// 重新导出 resource 领域类型以便使用
type Info = resourcetypes.Info
type Capacity = resourcetypes.Capacity
type ResourceRequest = resourcetypes.ResourceRequest
type RuntimeEnv = resourcetypes.RuntimeEnv
type ProviderType = resourcetypes.ProviderType
type ProviderStatus = resourcetypes.ProviderStatus
type ObjectID = resourcetypes.ObjectID
type StoreID = resourcetypes.StoreID

// 重新导出 resource 常量
const (
	RuntimeEnvPython = resourcetypes.RuntimeEnvPython
	ProviderStatusUnknown      = resourcetypes.ProviderStatusUnknown
	ProviderStatusConnected    = resourcetypes.ProviderStatusConnected
	ProviderStatusDisconnected = resourcetypes.ProviderStatusDisconnected
)
