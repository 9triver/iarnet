package types

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

// RuntimeEnv 运行时环境类型
type RuntimeEnv = string

const (
	// RuntimeEnvPython Python 运行时环境
	RuntimeEnvPython RuntimeEnv = "python"
)

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
