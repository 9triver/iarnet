package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

// LoadConfig 从文件加载配置并应用默认值
func LoadConfig(file string) (*Config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// 应用默认值
	ApplyDefaults(cfg)

	return cfg, nil
}

// ApplyDefaults 为配置项设置默认值
func ApplyDefaults(cfg *Config) {
	// 数据目录默认值
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}

	// 数据库配置默认值
	if cfg.Database.ApplicationDBPath == "" {
		cfg.Database.ApplicationDBPath = "./data/applications.db"
	}
	if cfg.Database.ResourceProviderDBPath == "" {
		cfg.Database.ResourceProviderDBPath = "./data/resource_providers.db"
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 10
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 5
	}
	if cfg.Database.ConnMaxLifetimeSeconds == 0 {
		cfg.Database.ConnMaxLifetimeSeconds = 300
	}

	// RPC 配置默认值
	if cfg.Transport.RPC.Logger.Port == 0 {
		cfg.Transport.RPC.Logger.Port = 50003 // 默认日志服务端口
	}
}
