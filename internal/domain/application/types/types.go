package types

import "time"

// FileInfo 文件信息
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

type AppStatus string

const (
	AppStatusRunning    AppStatus = "running"   // 运行中
	AppStatusStopped    AppStatus = "stopped"   // 已停止
	AppStatusFailed     AppStatus = "error"     // 失败
	AppStatusUndeployed AppStatus = "idle"      // 未部署
	AppStatusDeploying  AppStatus = "deploying" // 部署中
	AppStatusCloning    AppStatus = "cloning"   // 克隆中
)

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

type RunnerEnv = string

const (
	RunnerEnvPython RunnerEnv = "python"
	RunnerEnvGo     RunnerEnv = "go"
	RunnerEnvJava   RunnerEnv = "java"
)

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

type AppID = string
