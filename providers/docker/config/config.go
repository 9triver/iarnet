package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config Docker provider 配置
type Config struct {
	Server       ServerConfig `yaml:"server"`
	Docker       DockerConfig `yaml:"docker"`
	ResourceTags []string     `yaml:"resource_tags"`
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

func LoadConfig(path string) (*Config, error) {
	var config Config

	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	} else {
		config = getDefaultConfig()
	}

	return &config, nil
}

func getDefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port: 50051,
		},
		Docker: DockerConfig{
			Host:        "unix:///var/run/docker.sock",
			TLSCertPath: "",
			TLSVerify:   false,
			APIVersion:  "",
		},
		ResourceTags: []string{"cpu", "memory"},
	}
}
