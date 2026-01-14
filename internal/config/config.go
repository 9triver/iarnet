package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Config 应用级配置
// 包含应用运行所需的所有配置，各模块配置通过组合方式引入
type Config struct {
	// 应用基础配置
	Host              string            `yaml:"host"`                // Host for external connection. TODO: 通信问题待解决
	ListenAddr        string            `yaml:"listen_addr"`         // e.g., ":8080"
	PeerListenAddr    string            `yaml:"peer_listen_addr"`    // e.g., ":50051" for gRPC
	InitialPeers      []string          `yaml:"initial_peers"`       // e.g., ["peer1:50051"]
	ResourceLimits    map[string]string `yaml:"resource_limits"`     // e.g., {"cpu": "4", "memory": "8Gi", "gpu": "2"}
	DataDir           string            `yaml:"data_dir"`            // e.g., "./data" - directory for SQLite databases
	EnableLocalDocker bool              `yaml:"enable_local_docker"` // e.g., true - enable local docker provider
	SuperAdmin        *SuperAdminConfig `yaml:"super_admin"`         // 超级管理员配置（仅用于初始化）
	Users             []UserConfig      `yaml:"users"`               // 用户配置列表（已废弃，保留用于向后兼容）

	// 领域模块配置（内联定义，避免循环依赖）
	Application ApplicationConfig `yaml:"application"` // Application module configuration
	Resource    ResourceConfig    `yaml:"resource"`    // Resource module configuration
	Ignis       IgnisConfig       `yaml:"ignis"`       // Ignis module configuration
	Transport   TransportConfig   `yaml:"transport"`   // Transport configuration
	Database    DatabaseConfig    `yaml:"database"`    // Database configuration
	Auth        AuthConfig        `yaml:"auth"`        // Authentication configuration
}

// AuthConfig 认证配置
type AuthConfig struct {
	JWTSecret string `yaml:"jwt_secret"` // JWT 密钥
}

// UserRole 用户角色
type UserRole string

const (
	RoleNormalUser    UserRole = "normal"   // 普通用户
	RolePlatformAdmin UserRole = "platform" // 平台管理员
	RoleSuperAdmin    UserRole = "super"    // 超级管理员
)

// SuperAdminConfig 超级管理员配置（仅用于初始化）
type SuperAdminConfig struct {
	Name     string `yaml:"name"`     // 用户名
	Password string `yaml:"password"` // 密码（明文）
}

// UserConfig 用户配置（已废弃，保留用于向后兼容）
type UserConfig struct {
	Name     string   `yaml:"name"`     // 用户名
	Password string   `yaml:"password"` // 密码（明文，仅用于配置）
	Role     UserRole `yaml:"role"`     // 用户角色：normal（普通用户）、platform（平台管理员）、super（超级管理员）
}

// ApplicationConfig Application 模块配置
type ApplicationConfig struct {
	WorkspaceDir string            `yaml:"workspace_dir"` // e.g., "./workspaces" - directory for git repositories
	RunnerImages map[string]string `yaml:"runner_images"` // e.g., "python:3.11-alpine" - image to use for runner containers
}

// ResourceConfig Resource 模块配置
type ResourceConfig struct {
	GlobalRegistryAddr                 string                 `yaml:"global_registry_addr"`                   // e.g., "localhost:50010" - address of the global registry
	Name                               string                 `yaml:"name"`                                   // e.g., "node.1" - name of the node
	Description                        string                 `yaml:"description"`                            // e.g., "node.1 description" - description of the node
	DomainID                           string                 `yaml:"domain_id"`                              // e.g., "domain.AT9xbJe6RxzkPSL65bkwud" - domain ID of the node
	IsHead                             bool                   `yaml:"is_head"`                                // 是否为 head 节点
	ComponentImages                    map[string]string      `yaml:"component_images"`                       // e.g., "python:3.11-alpine" - image to use for actor containers
	Store                              StoreConfig            `yaml:"store"`                                  // Store configuration
	Discovery                          DiscoveryConfig        `yaml:"discovery"`                              // Gossip 节点发现配置
	SchedulePolicies                   []SchedulePolicyConfig `yaml:"schedule_policies"`                      // 调度策略配置
	ProviderHealthCheckIntervalSeconds float64                `yaml:"provider_health_check_interval_seconds"` // Provider 健康检查间隔（秒，支持小数）
	ProviderUsagePollIntervalSeconds   float64                `yaml:"provider_usage_poll_interval_seconds"`   // Provider 资源使用量轮询间隔（秒，支持小数）
	FakeProviders                      []FakeProviderConfig   `yaml:"fake_providers"`                         // 假 Provider 配置（用于演示）
}

// MemorySize 内存大小，支持字符串格式（如 "4Gi", "512Mi"）或整数（字节数）
type MemorySize int64

// UnmarshalYAML 实现 YAML 反序列化，支持字符串和整数格式
func (m *MemorySize) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var v interface{}
	if err := unmarshal(&v); err != nil {
		return err
	}

	switch val := v.(type) {
	case int:
		*m = MemorySize(val)
		return nil
	case int64:
		*m = MemorySize(val)
		return nil
	case string:
		bytes, err := parseMemoryString(val)
		if err != nil {
			return err
		}
		*m = MemorySize(bytes)
		return nil
	default:
		return fmt.Errorf("invalid memory format: %v, expected string (e.g., \"4Gi\") or integer (bytes)", v)
	}
}

// Int64 返回内存大小（字节数）
func (m MemorySize) Int64() int64 {
	return int64(m)
}

// parseMemoryString 解析内存字符串为字节数
// 支持格式：8Gi, 8GB, 8192Mi, 8192MB, 8192, 8G, 8M 等
func parseMemoryString(memoryStr string) (int64, error) {
	if memoryStr == "" {
		return 0, nil
	}

	// 移除空格并转换为小写
	memoryStr = strings.TrimSpace(strings.ToLower(memoryStr))

	// 正则表达式匹配数字和单位
	re := regexp.MustCompile(`^(\d+)([kmgt]?i?b?)$`)
	matches := re.FindStringSubmatch(memoryStr)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid memory format: %s, expected format like 8Gi, 8GB, 8192Mi", memoryStr)
	}

	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value: %s", matches[1])
	}

	unit := matches[2]
	var multiplier int64

	switch unit {
	case "b", "":
		multiplier = 1
	case "kb", "k":
		multiplier = 1000
	case "kib", "ki":
		multiplier = 1024
	case "mb", "m":
		multiplier = 1000 * 1000
	case "mib", "mi":
		multiplier = 1024 * 1024
	case "gb", "g":
		multiplier = 1000 * 1000 * 1000
	case "gib", "gi":
		multiplier = 1024 * 1024 * 1024
	case "tb", "t":
		multiplier = 1000 * 1000 * 1000 * 1000
	case "tib", "ti":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown memory unit: %s", unit)
	}

	return value * multiplier, nil
}

// FakeProviderConfig 假 Provider 配置
type FakeProviderConfig struct {
	Name         string              `yaml:"name"`          // Provider 名称
	Type         string              `yaml:"type"`          // Provider 类型：docker 或 k8s
	CPU          int64               `yaml:"cpu"`           // CPU 总量（millicores）
	Memory       MemorySize          `yaml:"memory"`        // 内存总量（支持字符串格式如 "4Gi" 或整数字节数）
	GPU          int64               `yaml:"gpu"`           // GPU 总量
	Host         string              `yaml:"host"`          // 主机地址（可选，用于显示）
	Port         int                 `yaml:"port"`          // 端口（可选，用于显示）
	ResourceTags *ResourceTagsConfig `yaml:"resource_tags"` // 资源标签配置（可选）
	Usage        *UsageConfig        `yaml:"usage"`         // 资源使用状态配置（可选）
}

// UsageConfig 资源使用状态配置
// 支持两种配置方式：
// 1. cpu_ratio/gpu_ratio/memory_ratio: 分别配置各资源的使用率（0.0-1.0），例如 0.8 表示使用 80%
// 2. used: 直接指定已使用的资源量（如果同时配置，used 优先）
type UsageConfig struct {
	CPURatio    float64             `yaml:"cpu_ratio"`    // CPU 使用率（0.0-1.0），例如 0.8 表示使用 80%
	GPURatio    float64             `yaml:"gpu_ratio"`    // GPU 使用率（0.0-1.0），例如 0.8 表示使用 80%
	MemoryRatio float64             `yaml:"memory_ratio"` // Memory 使用率（0.0-1.0），例如 0.8 表示使用 80%
	Used        *UsedResourceConfig `yaml:"used"`         // 直接指定已使用的资源量（可选，如果配置则优先使用）
}

// UsedResourceConfig 已使用资源配置
type UsedResourceConfig struct {
	CPU    int64      `yaml:"cpu"`    // 已使用的 CPU（millicores）
	Memory MemorySize `yaml:"memory"` // 已使用的内存（支持字符串格式如 "4Gi" 或整数字节数）
	GPU    int64      `yaml:"gpu"`    // 已使用的 GPU
}

// ResourceTagsConfig 资源标签配置
type ResourceTagsConfig struct {
	CPU    bool `yaml:"cpu"`    // 是否支持 CPU
	GPU    bool `yaml:"gpu"`    // 是否支持 GPU
	Memory bool `yaml:"memory"` // 是否支持 Memory
	Camera bool `yaml:"camera"` // 是否支持 Camera
}

// SchedulePolicyConfig 调度策略配置
type SchedulePolicyConfig struct {
	Type   string                 `yaml:"type"`   // 策略类型：resource_safety_margin, node_blacklist, provider_blacklist
	Enable bool                   `yaml:"enable"` // 是否启用
	Params map[string]interface{} `yaml:"params"` // 策略参数
}

// DiscoveryConfig Gossip 节点发现配置
type DiscoveryConfig struct {
	Enabled                    bool    `yaml:"enabled"`                       // 是否启用 gossip 发现
	GossipIntervalSeconds      float64 `yaml:"gossip_interval_seconds"`       // Gossip 间隔（秒，支持小数，如 0.5 表示 500ms）。如果同时配置了 min 和 max，则此值作为默认值（向后兼容）
	GossipIntervalMinSeconds   float64 `yaml:"gossip_interval_min_seconds"`   // Gossip 最小间隔（秒，支持小数）。如果配置了此值和 max，则使用区间随机，避免所有节点同时同步
	GossipIntervalMaxSeconds   float64 `yaml:"gossip_interval_max_seconds"`   // Gossip 最大间隔（秒，支持小数）。如果配置了此值和 min，则使用区间随机，避免所有节点同时同步
	NodeTTLSeconds             int     `yaml:"node_ttl_seconds"`              // 节点信息过期时间（秒）
	MaxGossipPeers             int     `yaml:"max_gossip_peers"`              // 每次 gossip 的最大 peer 数量
	MaxHops                    int     `yaml:"max_hops"`                      // 最大跳数
	QueryTimeoutSeconds        int     `yaml:"query_timeout_seconds"`         // 资源查询超时时间（秒）
	Fanout                     int     `yaml:"fanout"`                        // 每次传播的节点数（fanout）
	UseAntiEntropy             bool    `yaml:"use_anti_entropy"`              // 是否使用反熵机制
	AntiEntropyIntervalSeconds int     `yaml:"anti_entropy_interval_seconds"` // 反熵间隔（秒）
	LogNodeInfoUpdates         bool    `yaml:"log_node_info_updates"`         // 是否记录节点信息更新日志（收到 gossip 消息时更新其他节点状态）
}

type TransportConfig struct {
	ZMQ  ZMQConfig  `yaml:"zmq"`
	RPC  RPCConfig  `yaml:"rpc"`
	HTTP HTTPConfig `yaml:"http"`
}

type HTTPConfig struct {
	Port int `yaml:"port"` // e.g., 8080 - HTTP server port
}

// RPCConfig RPC 配置
type RPCConfig struct {
	Resource       RPCResourceConfig       `yaml:"resource"` // 资源服务 RPC 配置
	Ignis          RPCIgnisConfig          `yaml:"ignis"`
	Store          RPCStoreConfig          `yaml:"store"`
	Logger         RPCLoggerConfig         `yaml:"logger"`          // 应用日志服务配置
	ResourceLogger RPCResourceLoggerConfig `yaml:"resource_logger"` // 资源日志服务配置
	Discovery      RPCDiscoveryConfig      `yaml:"discovery"`       // 节点发现服务 RPC 配置
	Scheduler      RPCSchedulerConfig      `yaml:"scheduler"`       // 调度服务 RPC 配置
}

// RPCResourceConfig 资源服务 RPC 配置
type RPCResourceConfig struct {
	Port int `yaml:"port"` // e.g., 50051
}

type RPCIgnisConfig struct {
	Port int `yaml:"port"` // e.g., 50001
}

type RPCStoreConfig struct {
	Port int `yaml:"port"` // e.g., 50002
}

// RPCLoggerConfig 应用日志服务 RPC 配置
type RPCLoggerConfig struct {
	Port int `yaml:"port"` // e.g., 50003
}

// RPCResourceLoggerConfig 资源日志服务 RPC 配置
type RPCResourceLoggerConfig struct {
	Port int `yaml:"port"` // e.g., 50004
}

// RPCDiscoveryConfig 节点发现服务 RPC 配置
type RPCDiscoveryConfig struct {
	Port int `yaml:"port"` // e.g., 50005
}

// RPCSchedulerConfig 调度服务 RPC 配置
type RPCSchedulerConfig struct {
	Port int `yaml:"port"` // e.g., 50006
}

// StoreConfig Store 服务配置
type StoreConfig struct {
}

// ZMQConfig ZMQ 配置
type ZMQConfig struct {
	Port int `yaml:"port"` // e.g., "5555"
}

// IgnisConfig Ignis 模块配置
type IgnisConfig struct {
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	ApplicationDBPath      string `yaml:"application_db_path"`       // Application 数据库路径
	ResourceProviderDBPath string `yaml:"resource_provider_db_path"` // Resource Provider 数据库路径
	ResourceLoggerDBPath   string `yaml:"resource_logger_db_path"`   // Resource Logger 数据库路径
	OperationLogDBPath     string `yaml:"operation_log_db_path"`     // Operation Log 数据库路径
	MaxOpenConns           int    `yaml:"max_open_conns"`            // 最大打开连接数
	MaxIdleConns           int    `yaml:"max_idle_conns"`            // 最大空闲连接数
	ConnMaxLifetimeSeconds int    `yaml:"conn_max_lifetime_seconds"` // 连接最大生存时间（秒）
}
