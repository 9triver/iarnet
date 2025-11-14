package config

import (
	"os"

	"github.com/9triver/iarnet/internal/infra/database"
	"github.com/9triver/iarnet/internal/infra/logging"
)

// Config 应用级配置
// 包含应用运行所需的所有配置，各模块配置通过组合方式引入
type Config struct {
	// 应用基础配置
	Mode              string            `yaml:"mode"`                // "standalone" or "k8s"
	Host              string            `yaml:"host"`                // Host for external connection. TODO: 通信问题待解决
	ListenAddr        string            `yaml:"listen_addr"`         // e.g., ":8080"
	PeerListenAddr    string            `yaml:"peer_listen_addr"`    // e.g., ":50051" for gRPC
	InitialPeers      []string          `yaml:"initial_peers"`       // e.g., ["peer1:50051"]
	ResourceLimits    map[string]string `yaml:"resource_limits"`     // e.g., {"cpu": "4", "memory": "8Gi", "gpu": "2"}
	DataDir           string            `yaml:"data_dir"`            // e.g., "./data" - directory for SQLite databases
	EnableLocalDocker bool              `yaml:"enable_local_docker"` // e.g., true - enable local docker provider

	// 基础设施配置
	Database database.Config `yaml:"database"` // Database configuration
	Logging  logging.Config  `yaml:"logging"`  // Logging configuration

	// 领域模块配置（内联定义，避免循环依赖）
	Application ApplicationConfig `yaml:"application"` // Application module configuration
	Resource    ResourceConfig    `yaml:"resource"`    // Resource module configuration
	Ignis       IgnisConfig       `yaml:"ignis"`       // Ignis module configuration
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
	ZMQ             ZMQConfig         `yaml:"zmq"`              // ZMQ configuration
}

// StoreConfig Store 服务配置
type StoreConfig struct {
	Port int `yaml:"port"` // e.g., "50051"
}

// ZMQConfig ZMQ 配置
type ZMQConfig struct {
	Port int `yaml:"port"` // e.g., "5555"
}

// IgnisConfig Ignis 模块配置
type IgnisConfig struct {
	Port int32 `yaml:"port"` // e.g., "50051"
}

// DetectMode 自动检测运行模式（K8s 或 standalone）
func DetectMode() string {
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount"); err == nil {
		return "k8s"
	}
	return "standalone"
}
