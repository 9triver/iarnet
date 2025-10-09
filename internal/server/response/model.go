package response

import (
	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/resource"
)

// ResourceProviderInfo 表示资源提供者的详细信息
type ResourceProviderInfo struct {
	ID             string          `json:"id"`               // 资源提供者ID
	Name           string          `json:"name"`             // 资源名称
	Host           string          `json:"host"`             // 资源主机地址
	Port           int             `json:"port"`             // 资源端口
	Type           string          `json:"type"`             // 类型 (K8s, Docker等)
	Status         resource.Status `json:"status"`           // 状态 (已连接, 断开连接等)
	CPUUsage       UsageInfo       `json:"cpu_usage"`        // CPU使用率信息
	MemoryUsage    UsageInfo       `json:"memory_usage"`     // 内存使用率信息
	LastUpdateTime string          `json:"last_update_time"` // 最后更新时间
}

// UsageInfo 表示资源使用率信息
type UsageInfo struct {
	Used  float64 `json:"used"`  // 已使用量
	Total float64 `json:"total"` // 总量
}

type GetResourceProvidersResponse struct {
	LocalProvider          *ResourceProviderInfo  `json:"local_provider"`          // 本机 provider（无或一个）
	ManagedProviders       []ResourceProviderInfo `json:"managed_providers"`       // 托管的外部 provider（无或多个）
	CollaborativeProviders []ResourceProviderInfo `json:"collaborative_providers"` // 通过协作发现的 provider（无或多个）
}

type GetApplicationsOverViewResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type ApplicationInfo struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	GitUrl       *string            `json:"gitUrl,omitempty"`
	Branch       *string            `json:"branch,omitempty"`
	Type         string             `json:"type"` // "web", "api", "worker", "database"
	Description  *string            `json:"description,omitempty"`
	Ports        []int              `json:"ports,omitempty"`
	HealthCheck  *string            `json:"healthCheck,omitempty"`
	LastDeployed string             `json:"lastDeployed"`
	RunningOn    []string           `json:"runningOn"`
	Status       application.Status `json:"status"`
	RunnerEnv    *string            `json:"runnerEnv,omitempty"`  // 运行环境，如 "python", "node", "go" 等
	ExecuteCmd   *string            `json:"executeCmd,omitempty"` // 执行命令，如 "python app.py" 等
}

type GetApplicationsResponse struct {
	Applications []ApplicationInfo `json:"applications"`
}

type GetApplicationLogsResponse struct {
	ApplicationId   string   `json:"applicationId"`
	ApplicationName string   `json:"applicationName"`
	Logs            []string `json:"logs"`
	TotalLines      int      `json:"totalLines"`
	RequestedLines  int      `json:"requestedLines"`
}

type GetApplicationLogsParsedResponse struct {
	ApplicationId   string                  `json:"applicationId"`
	ApplicationName string                  `json:"applicationName"`
	Logs            []*application.LogEntry `json:"logs"`
	TotalLines      int                     `json:"totalLines"`
	RequestedLines  int                     `json:"requestedLines"`
}

// RegisterProviderResponse 注册资源提供者响应结构
type RegisterProviderResponse struct {
	ProviderID string `json:"providerId"` // 生成的提供者ID
	Message    string `json:"message"`    // 响应消息
}

// UnregisterProviderResponse 注销资源提供者响应结构
type UnregisterProviderResponse struct {
	Message string `json:"message"` // 响应消息
}

// StartCodeBrowserResponse 启动代码浏览器响应结构
type StartCodeBrowserResponse struct {
	Message string `json:"message"` // 响应消息
	Port    int    `json:"port"`    // 代码浏览器端口
	URL     string `json:"url"`     // 访问URL
}

// StopCodeBrowserResponse 停止代码浏览器响应结构
type StopCodeBrowserResponse struct {
	Message string `json:"message"` // 响应消息
}

type GetFileTreeResponse struct {
	Files []FileInfo `json:"files"`
}

type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

// PeerNodeInfo peer节点信息
type PeerNodeInfo struct {
	Address string `json:"address"` // peer节点地址
	Status  string `json:"status"`  // 节点状态: "connected", "disconnected", "unknown"
}

// GetPeerNodesResponse 获取peer节点列表响应
type GetPeerNodesResponse struct {
	Nodes []PeerNodeInfo `json:"nodes"` // peer节点列表
	Total int            `json:"total"` // 总数
}

// AddPeerNodeResponse 添加peer节点响应
type AddPeerNodeResponse struct {
	Message string `json:"message"` // 响应消息
	Address string `json:"address"` // 添加的节点地址
}

// RemovePeerNodeResponse 删除peer节点响应结构
type RemovePeerNodeResponse struct {
	Message string `json:"message"` // 响应消息
	Address string `json:"address"` // 删除的节点地址
}

// SaveFileResponse 保存文件响应结构
type SaveFileResponse struct {
	Message  string `json:"message"`  // 响应消息
	FilePath string `json:"filePath"` // 文件路径
}

// CreateFileResponse 创建文件响应结构
type CreateFileResponse struct {
	Message  string `json:"message"`  // 响应消息
	FilePath string `json:"filePath"` // 文件路径
}

// DeleteFileResponse 删除文件响应结构
type DeleteFileResponse struct {
	Message  string `json:"message"`  // 响应消息
	FilePath string `json:"filePath"` // 文件路径
}

// CreateDirectoryResponse 创建目录响应结构
type CreateDirectoryResponse struct {
	Message       string `json:"message"`       // 响应消息
	DirectoryPath string `json:"directoryPath"` // 目录路径
}

// DeleteDirectoryResponse 删除目录响应结构
type DeleteDirectoryResponse struct {
	Message       string `json:"message"`       // 响应消息
	DirectoryPath string `json:"directoryPath"` // 目录路径
}

type RunnerEnvironment struct {
	Name string `json:"name"`
}

type GetDAGResponse struct {
	DAG *application.DAG `json:"dag"`
}
