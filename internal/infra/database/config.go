package database

// Config 数据库配置
type Config struct {
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

// ApplyDefaults 为配置项设置默认值
func (c *Config) ApplyDefaults(dataDir string) {
	if c.ApplicationDBPath == "" {
		c.ApplicationDBPath = dataDir + "/applications.db"
	}
	if c.ResourceProviderDBPath == "" {
		c.ResourceProviderDBPath = dataDir + "/resource_providers.db"
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = 10
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 5
	}
	if c.ConnMaxLifetimeSeconds == 0 {
		c.ConnMaxLifetimeSeconds = 300 // 5 minutes
	}
}
