package config

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

	// 领域模块配置（内联定义，避免循环依赖）
	Application ApplicationConfig `yaml:"application"` // Application module configuration
	Resource    ResourceConfig    `yaml:"resource"`    // Resource module configuration
	Ignis       IgnisConfig       `yaml:"ignis"`       // Ignis module configuration
	Transport   TransportConfig   `yaml:"transport"`   // Transport configuration
	Database    DatabaseConfig    `yaml:"database"`    // Database configuration
}

// ApplicationConfig Application 模块配置
type ApplicationConfig struct {
	WorkspaceDir string            `yaml:"workspace_dir"` // e.g., "./workspaces" - directory for git repositories
	RunnerImages map[string]string `yaml:"runner_images"` // e.g., "python:3.11-alpine" - image to use for runner containers
}

// ResourceConfig Resource 模块配置
type ResourceConfig struct {
	ComponentImages map[string]string `yaml:"component_images"` // e.g., "python:3.11-alpine" - image to use for actor containers
	Store           StoreConfig       `yaml:"store"`            // Store configuration
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
	Ignis  RPCIgnisConfig  `yaml:"ignis"`
	Store  RPCStoreConfig  `yaml:"store"`
	Logger RPCLoggerConfig `yaml:"logger"` // 应用日志服务配置
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
	MaxOpenConns           int    `yaml:"max_open_conns"`            // 最大打开连接数
	MaxIdleConns           int    `yaml:"max_idle_conns"`            // 最大空闲连接数
	ConnMaxLifetimeSeconds int    `yaml:"conn_max_lifetime_seconds"` // 连接最大生存时间（秒）
}
