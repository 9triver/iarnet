package types

import (
	common "github.com/9triver/iarnet/internal/proto/common"
)

// 资源领域类型
// resource 是最底层领域，定义自己的类型

// Info 资源信息
type Info struct {
	CPU    int64    `json:"cpu"`    // millicores
	Memory int64    `json:"memory"` // bytes
	GPU    int64    `json:"gpu"`
	Tags   []string `json:"tags,omitempty"`
}

// Capacity 资源容量
type Capacity struct {
	Total     *Info `json:"total"`
	Used      *Info `json:"used"`
	Available *Info `json:"available"`
}

// ResourceRequest 资源请求（用于查询）
type ResourceRequest Info

// ResourceTags 资源标签（描述节点/provider 支持的计算资源类型）
type ResourceTags struct {
	CPU    bool
	GPU    bool
	Memory bool
	Camera bool
}

// NewEmptyResourceTags 创建空的资源标签
func NewEmptyResourceTags() *ResourceTags {
	return &ResourceTags{
		CPU:    false,
		GPU:    false,
		Memory: false,
		Camera: false,
	}
}

// NewResourceTags 创建资源标签
func NewResourceTags(cpu, gpu, memory, camera bool) *ResourceTags {
	return &ResourceTags{
		CPU:    cpu,
		GPU:    gpu,
		Memory: memory,
		Camera: camera,
	}
}

// HasResource 检查是否支持指定的资源类型
func (rt *ResourceTags) HasResource(resourceType string) bool {
	if rt == nil {
		return false
	}
	switch resourceType {
	case "cpu":
		return rt.CPU
	case "gpu":
		return rt.GPU
	case "memory":
		return rt.Memory
	case "camera":
		return rt.Camera
	default:
		return false
	}
}

// RuntimeEnv 运行时环境类型
type RuntimeEnv = string

const (
	// RuntimeEnvPython Python 运行时环境
	RuntimeEnvPython RuntimeEnv = "python"
	// RuntimeEnvGo Go 运行时环境
	RuntimeEnvGo RuntimeEnv = "go"
	// RuntimeEnvUnikernel Unikernel 运行时环境
	RuntimeEnvUnikernel RuntimeEnv = "unikernel"
)

// RuntimeEnvToLanguage 将 RuntimeEnv 转换为 common.Language
func RuntimeEnvToLanguage(runtimeEnv RuntimeEnv) common.Language {
	switch runtimeEnv {
	case RuntimeEnvPython:
		return common.Language_LANG_PYTHON
	case RuntimeEnvGo:
		return common.Language_LANG_GO
	case RuntimeEnvUnikernel:
		return common.Language_LANG_UNIKERNEL
	default:
		return common.Language_LANG_UNKNOWN
	}
}

// ProviderType 提供者类型
type ProviderType = string

// ProviderStatus 提供者状态
type ProviderStatus int32

const (
	// ProviderStatusUnknown 未知状态
	ProviderStatusUnknown ProviderStatus = 0
	// ProviderStatusConnected 已连接
	ProviderStatusConnected ProviderStatus = 1
	// ProviderStatusDisconnected 已断开
	ProviderStatusDisconnected ProviderStatus = 2
)

// ObjectID 对象标识符（resource 领域使用）
type ObjectID = string

// StoreID 存储标识符（resource 领域使用）
type StoreID = string
