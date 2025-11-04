package monitor

// Config monitor 配置（纯内存实现，保留配置结构以备将来扩展）
type Config struct {
	// MaxApplications 最大应用数量（可选的限制）
	MaxApplications int `json:"maxApplications" yaml:"maxApplications"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		MaxApplications: 1000,
	}
}
