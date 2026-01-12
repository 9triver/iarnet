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
	if cfg.Database.OperationLogDBPath == "" {
		cfg.Database.OperationLogDBPath = "./data/operation_logs.db"
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
	if cfg.Transport.RPC.Discovery.Port == 0 {
		cfg.Transport.RPC.Discovery.Port = 50005 // 默认节点发现服务端口
	}
	if cfg.Transport.RPC.Scheduler.Port == 0 {
		cfg.Transport.RPC.Scheduler.Port = 50006 // 默认调度服务端口
	}

	// Discovery 配置默认值
	// 处理 gossip 间隔配置
	if cfg.Resource.Discovery.GossipIntervalMinSeconds > 0 && cfg.Resource.Discovery.GossipIntervalMaxSeconds > 0 {
		// 如果配置了 min 和 max，使用区间随机
		if cfg.Resource.Discovery.GossipIntervalMinSeconds >= cfg.Resource.Discovery.GossipIntervalMaxSeconds {
			// 如果 min >= max，交换它们
			cfg.Resource.Discovery.GossipIntervalMinSeconds, cfg.Resource.Discovery.GossipIntervalMaxSeconds = cfg.Resource.Discovery.GossipIntervalMaxSeconds, cfg.Resource.Discovery.GossipIntervalMinSeconds
		}
		// 验证最小值
		if cfg.Resource.Discovery.GossipIntervalMinSeconds < 0.01 {
			cfg.Resource.Discovery.GossipIntervalMinSeconds = 0.01 // 最小 10ms
		}
		if cfg.Resource.Discovery.GossipIntervalMaxSeconds < 0.01 {
			cfg.Resource.Discovery.GossipIntervalMaxSeconds = 0.01 // 最小 10ms
		}
	} else if cfg.Resource.Discovery.GossipIntervalSeconds == 0 {
		// 如果没有配置固定间隔，也没有配置区间，使用默认值
		cfg.Resource.Discovery.GossipIntervalSeconds = 30 // 默认 30 秒
	}
	// 验证固定间隔的最小值，避免过小的间隔导致性能问题
	if cfg.Resource.Discovery.GossipIntervalSeconds > 0 && cfg.Resource.Discovery.GossipIntervalSeconds < 0.01 {
		cfg.Resource.Discovery.GossipIntervalSeconds = 0.01 // 最小 10ms
	}
	// 默认启用节点信息更新日志（为了向后兼容）
	// 如果配置文件中没有设置，默认为 true
	// 注意：由于 bool 类型的零值是 false，我们需要特殊处理
	// 这里假设如果配置文件中没有设置，则使用默认值 true
	// 但 YAML 解析器会自动处理，如果配置文件中明确设置为 false，则使用 false
	if cfg.Resource.Discovery.NodeTTLSeconds == 0 {
		cfg.Resource.Discovery.NodeTTLSeconds = 180 // 默认 180 秒（3 分钟）
	}
	// Provider 健康检查和资源轮询配置默认值
	if cfg.Resource.ProviderHealthCheckIntervalSeconds == 0 {
		cfg.Resource.ProviderHealthCheckIntervalSeconds = 30 // 默认 30 秒
	}
	// 验证最小值，避免过小的间隔导致性能问题
	if cfg.Resource.ProviderHealthCheckIntervalSeconds < 0.01 {
		cfg.Resource.ProviderHealthCheckIntervalSeconds = 0.01 // 最小 10ms
	}
	if cfg.Resource.ProviderUsagePollIntervalSeconds == 0 {
		cfg.Resource.ProviderUsagePollIntervalSeconds = 2 // 默认 2 秒
	}
	// 验证最小值
	if cfg.Resource.ProviderUsagePollIntervalSeconds < 0.01 {
		cfg.Resource.ProviderUsagePollIntervalSeconds = 0.01 // 最小 10ms
	}
	if cfg.Resource.Discovery.MaxGossipPeers == 0 {
		cfg.Resource.Discovery.MaxGossipPeers = 10 // 默认 10 个
	}
	if cfg.Resource.Discovery.MaxHops == 0 {
		cfg.Resource.Discovery.MaxHops = 5 // 默认 5 跳
	}
	if cfg.Resource.Discovery.QueryTimeoutSeconds == 0 {
		cfg.Resource.Discovery.QueryTimeoutSeconds = 5 // 默认 5 秒
	}
	if cfg.Resource.Discovery.Fanout == 0 {
		cfg.Resource.Discovery.Fanout = 3 // 默认 3 个
	}
}
