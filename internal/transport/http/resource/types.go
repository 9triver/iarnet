package resource

import (
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/types"
)

// GetResourceCapacityResponse 获取资源容量响应
type GetResourceCapacityResponse struct {
	Total     ResourceInfo `json:"total"`     // 总资源
	Used      ResourceInfo `json:"used"`      // 已使用资源
	Available ResourceInfo `json:"available"` // 可用资源
}

// ResourceInfo 资源信息
type ResourceInfo struct {
	CPU    int64 `json:"cpu"`    // CPU（millicores）
	Memory int64 `json:"memory"` // 内存（bytes）
	GPU    int64 `json:"gpu"`    // GPU 数量
}

// FromCapacity 从领域层 Capacity 转换为 HTTP 响应
func (r *GetResourceCapacityResponse) FromCapacity(capacity *types.Capacity) *GetResourceCapacityResponse {
	if capacity == nil {
		return r
	}
	if capacity.Total != nil {
		r.Total = ResourceInfo{
			CPU:    capacity.Total.CPU,
			Memory: capacity.Total.Memory,
			GPU:    capacity.Total.GPU,
		}
	}
	if capacity.Used != nil {
		r.Used = ResourceInfo{
			CPU:    capacity.Used.CPU,
			Memory: capacity.Used.Memory,
			GPU:    capacity.Used.GPU,
		}
	}
	if capacity.Available != nil {
		r.Available = ResourceInfo{
			CPU:    capacity.Available.CPU,
			Memory: capacity.Available.Memory,
			GPU:    capacity.Available.GPU,
		}
	}
	return r
}

// GetResourceProvidersResponse 获取资源提供者列表响应
type GetResourceProvidersResponse struct {
	Providers []ProviderItem `json:"providers"` // 提供者列表
	Total     int            `json:"total"`     // 总数
}

// ProviderItem 提供者列表项
type ProviderItem struct {
	ID             string            `json:"id"`                      // 提供者 ID
	Name           string            `json:"name"`                    // 提供者名称
	Type           string            `json:"type"`                    // 提供者类型
	Host           string            `json:"host"`                    // 主机地址
	Port           int               `json:"port"`                    // 端口
	Status         string            `json:"status"`                  // 状态 (connected/disconnected)
	LastUpdateTime time.Time         `json:"last_update_time"`        // 最后更新时间
	ResourceTags   *ResourceTagsInfo `json:"resource_tags,omitempty"` // 资源标签
}

// FromProvider 从领域层 Provider 转换为 ProviderItem
func (p *ProviderItem) FromProvider(provider interface {
	GetID() string
	GetName() string
	GetType() types.ProviderType
	GetHost() string
	GetPort() int
	GetStatus() types.ProviderStatus
	GetLastUpdateTime() time.Time
	GetResourceTags() *types.ResourceTags
}) *ProviderItem {
	p.ID = provider.GetID()
	p.Name = provider.GetName()
	p.Type = string(provider.GetType())
	p.Host = provider.GetHost()
	p.Port = provider.GetPort()
	p.Status = providerStatusToString(provider.GetStatus())
	p.LastUpdateTime = provider.GetLastUpdateTime()
	p.ResourceTags = resourceTagsToInfo(provider.GetResourceTags())
	return p
}

// providerStatusToString 将 ProviderStatus 转换为字符串
func providerStatusToString(status types.ProviderStatus) string {
	switch status {
	case types.ProviderStatusConnected:
		return "connected"
	case types.ProviderStatusDisconnected:
		return "disconnected"
	default:
		return "unknown"
	}
}

// RegisterResourceProviderRequest 注册资源提供者请求
type RegisterResourceProviderRequest struct {
	Name string `json:"name" binding:"required"` // 提供者名称
	Host string `json:"host" binding:"required"` // 主机地址
	Port int    `json:"port" binding:"required"` // 端口
}

// UpdateResourceProviderRequest 更新资源提供者请求
// 目前只支持更新名称，日后可能会支持其他字段（如 host, port 等）
type UpdateResourceProviderRequest struct {
	Name *string `json:"name,omitempty"` // 提供者名称（可选，目前唯一支持的字段）
	// 未来可能添加的字段：
	// Host *string `json:"host,omitempty"` // 主机地址
	// Port *int    `json:"port,omitempty"` // 端口
}

// UpdateResourceProviderResponse 更新资源提供者响应
type UpdateResourceProviderResponse struct {
	ID      string `json:"id"`      // 提供者 ID
	Name    string `json:"name"`    // 更新后的名称
	Message string `json:"message"` // 响应消息
}

// RegisterResourceProviderResponse 注册资源提供者响应
type RegisterResourceProviderResponse struct {
	ID   string `json:"id"`   // 提供者 ID
	Name string `json:"name"` // 提供者名称
}

// UnregisterResourceProviderResponse 注销资源提供者响应
type UnregisterResourceProviderResponse struct {
	ID      string `json:"id"`      // 提供者 ID
	Message string `json:"message"` // 响应消息
}

// TestResourceProviderRequest 测试资源提供者连接请求
type TestResourceProviderRequest struct {
	Name string `json:"name" binding:"required"` // 提供者名称
	Host string `json:"host" binding:"required"` // 主机地址
	Port int    `json:"port" binding:"required"` // 端口
}

// TestResourceProviderResponse 测试资源提供者连接响应
type TestResourceProviderResponse struct {
	Success  bool         `json:"success"`  // 连接是否成功
	Type     string       `json:"type"`     // 提供者类型
	Capacity ResourceInfo `json:"capacity"` // 资源容量（连接成功时返回）
	Message  string       `json:"message"`  // 响应消息或错误信息
}

// GetResourceProviderInfoResponse 获取资源提供者信息响应
type GetResourceProviderInfoResponse struct {
	ID             string            `json:"id"`                      // 提供者 ID
	Name           string            `json:"name"`                    // 提供者名称
	Type           string            `json:"type"`                    // 提供者类型
	Host           string            `json:"host"`                    // 主机地址
	Port           int               `json:"port"`                    // 端口
	Status         string            `json:"status"`                  // 状态 (connected/disconnected/unknown)
	LastUpdateTime time.Time         `json:"last_update_time"`        // 最后更新时间
	ResourceTags   *ResourceTagsInfo `json:"resource_tags,omitempty"` // 资源标签
}

// FromProvider 从领域层 Provider 转换为 GetResourceProviderInfoResponse
func (r *GetResourceProviderInfoResponse) FromProvider(provider interface {
	GetID() string
	GetName() string
	GetType() types.ProviderType
	GetHost() string
	GetPort() int
	GetStatus() types.ProviderStatus
	GetLastUpdateTime() time.Time
	GetResourceTags() *types.ResourceTags
}) *GetResourceProviderInfoResponse {
	r.ID = provider.GetID()
	r.Name = provider.GetName()
	r.Type = string(provider.GetType())
	r.Host = provider.GetHost()
	r.Port = provider.GetPort()
	r.Status = providerStatusToString(provider.GetStatus())
	r.LastUpdateTime = provider.GetLastUpdateTime()
	r.ResourceTags = resourceTagsToInfo(provider.GetResourceTags())
	return r
}

// GetResourceProviderCapacityResponse 获取资源提供者容量响应
type GetResourceProviderCapacityResponse struct {
	Total     ResourceInfo `json:"total"`     // 总资源
	Used      ResourceInfo `json:"used"`      // 已使用资源
	Available ResourceInfo `json:"available"` // 可用资源
}

// FromCapacity 从领域层 Capacity 转换为 GetResourceProviderCapacityResponse
func (r *GetResourceProviderCapacityResponse) FromCapacity(capacity *types.Capacity) *GetResourceProviderCapacityResponse {
	if capacity == nil {
		return r
	}
	if capacity.Total != nil {
		r.Total = ResourceInfo{
			CPU:    capacity.Total.CPU,
			Memory: capacity.Total.Memory,
			GPU:    capacity.Total.GPU,
		}
	}
	if capacity.Used != nil {
		r.Used = ResourceInfo{
			CPU:    capacity.Used.CPU,
			Memory: capacity.Used.Memory,
			GPU:    capacity.Used.GPU,
		}
	}
	if capacity.Available != nil {
		r.Available = ResourceInfo{
			CPU:    capacity.Available.CPU,
			Memory: capacity.Available.Memory,
			GPU:    capacity.Available.GPU,
		}
	}
	return r
}

// GetResourceProviderUsageResponse 获取资源提供者实时使用情况响应
type GetResourceProviderUsageResponse struct {
	Usage ResourceInfo `json:"usage"` // 实时资源使用情况
}

// FromUsage 从领域层 Info 转换为 GetResourceProviderUsageResponse
func (r *GetResourceProviderUsageResponse) FromUsage(usage *types.Info) *GetResourceProviderUsageResponse {
	if usage == nil {
		return r
	}
	r.Usage = ResourceInfo{
		CPU:    usage.CPU,
		Memory: usage.Memory,
		GPU:    usage.GPU,
	}
	return r
}

func resourceTagsToInfo(tags *types.ResourceTags) *ResourceTagsInfo {
	if tags == nil {
		return nil
	}
	return &ResourceTagsInfo{
		CPU:    tags.CPU,
		GPU:    tags.GPU,
		Memory: tags.Memory,
		Camera: tags.Camera,
	}
}
