package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Mode           string            `yaml:"mode"`             // "standalone" or "k8s"
	ListenAddr     string            `yaml:"listen_addr"`      // e.g., ":8080"
	PeerListenAddr string            `yaml:"peer_listen_addr"` // e.g., ":50051" for gRPC
	InitialPeers   []string          `yaml:"initial_peers"`    // e.g., ["peer1:50051"]
	ResourceLimits map[string]string `yaml:"resource_limits"`  // e.g., {"cpu": "4", "memory": "8Gi", "gpu": "2"}
	WorkspaceDir   string            `yaml:"workspace_dir"`    // e.g., "./workspaces" - directory for git repositories
	Ignis          IgnisConfig       `yaml:"ignis"`            // Ignis integration configuration
}

type IgnisConfig struct {
	MasterAddress string `yaml:"master_address"` // e.g., "localhost:50051"
}

func LoadConfig(file string) (*Config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	return &cfg, err
}

// DetectMode: Auto-detect if running in K8s
func DetectMode() string {
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount"); err == nil {
		return "k8s"
	}
	return "standalone"
}
