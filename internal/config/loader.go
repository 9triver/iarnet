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

}
