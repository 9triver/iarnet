package response

import (
	"github.com/9triver/iarnet/internal/resource"
)

// ResourceProviderInfo 表示资源提供者的详细信息
type ResourceProviderInfo struct {
	ID             string          `json:"id"`               // 资源提供者ID
	Name           string          `json:"name"`             // 资源名称
	URL            string          `json:"url"`              // 资源URL
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
	Providers []ResourceProviderInfo `json:"providers"`
}
