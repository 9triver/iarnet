package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Mode              string            `yaml:"mode"`                // "standalone" or "k8s"
	ExternalAddr      string            `yaml:"external_addr"`       // External address for actor container connection. TODO: 通信问题待解决
	ListenAddr        string            `yaml:"listen_addr"`         // e.g., ":8080"
	PeerListenAddr    string            `yaml:"peer_listen_addr"`    // e.g., ":50051" for gRPC
	InitialPeers      []string          `yaml:"initial_peers"`       // e.g., ["peer1:50051"]
	ResourceLimits    map[string]string `yaml:"resource_limits"`     // e.g., {"cpu": "4", "memory": "8Gi", "gpu": "2"}
	WorkspaceDir      string            `yaml:"workspace_dir"`       // e.g., "./workspaces" - directory for git repositories
	DataDir           string            `yaml:"data_dir"`            // e.g., "./data" - directory for SQLite databases
	Ignis             IgnisConfig       `yaml:"ignis"`               // Ignis integration configuration
	RunnerImages      RunnerImageConfig `yaml:"runner_images"`       // e.g., "python:3.11-alpine" - image to use for runner containers
	ComponentImages   ActorImageConfig  `yaml:"component_images"`    // e.g., "python:3.11-alpine" - image to use for actor containers
	EnableLocalDocker bool              `yaml:"enable_local_docker"` // e.g., true - enable local docker provider
	Database          DatabaseConfig    `yaml:"database"`            // Database configuration
}

type IgnisConfig struct {
	Port int32 `yaml:"port"` // e.g., "50051"
}

type DatabaseConfig struct {
	// ApplicationDBPath 应用数据库路径
	ApplicationDBPath string `yaml:"application_db_path"` // e.g., "./data/applications.db"

	// ResourceProviderDBPath 资源 provider 数据库路径
	ResourceProviderDBPath string `yaml:"resource_provider_db_path"` // e.g., "./data/resource_providers.db"

	// MaxOpenConns 最大打开连接数
	MaxOpenConns int `yaml:"max_open_conns"` // default: 10

	// MaxIdleConns 最大空闲连接数
	MaxIdleConns int `yaml:"max_idle_conns"` // default: 5

	// ConnMaxLifetimeSeconds 连接最大生命周期（秒）
	ConnMaxLifetimeSeconds int `yaml:"conn_max_lifetime_seconds"` // default: 300 (5 minutes)
}

type RunnerImageConfig map[string]string
type ActorImageConfig map[string]string

func LoadConfig(file string) (*Config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	// 设置默认值
	applyDefaults(&cfg)

	return &cfg, nil
}

// applyDefaults 为配置项设置默认值
func applyDefaults(cfg *Config) {
	// 数据目录默认值
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}

	// 数据库配置默认值
	if cfg.Database.ApplicationDBPath == "" {
		cfg.Database.ApplicationDBPath = cfg.DataDir + "/applications.db"
	}
	if cfg.Database.ResourceProviderDBPath == "" {
		cfg.Database.ResourceProviderDBPath = cfg.DataDir + "/resource_providers.db"
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 10
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 5
	}
	if cfg.Database.ConnMaxLifetimeSeconds == 0 {
		cfg.Database.ConnMaxLifetimeSeconds = 300 // 5 minutes
	}
}

// DetectMode: Auto-detect if running in K8s
func DetectMode() string {
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount"); err == nil {
		return "k8s"
	}
	return "standalone"
}
