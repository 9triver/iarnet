package docker

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config Docker provider 配置
type Config struct {
	Server ServerConfig `yaml:"server"`
	Docker DockerConfig `yaml:"docker"`
}

// ServerConfig gRPC 服务器配置
type ServerConfig struct {
	Port int `yaml:"port"`
}

// DockerConfig Docker 引擎配置
type DockerConfig struct {
	Host        string `yaml:"host"`
	TLSCertPath string `yaml:"tls_cert_path"`
	TLSVerify   bool   `yaml:"tls_verify"`
	APIVersion  string `yaml:"api_version"`
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 设置默认值
	if config.Server.Port == 0 {
		config.Server.Port = 50051
	}

	if config.Docker.Host == "" {
		config.Docker.Host = "unix:///var/run/docker.sock"
	}

	return &config, nil
}
