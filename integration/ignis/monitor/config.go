package monitor

// Config monitor 配置
type Config struct {
	// DBPath 数据库文件路径
	DBPath string `json:"dbPath" yaml:"dbPath"`

	// MaxOpenConns 最大打开连接数
	MaxOpenConns int `json:"maxOpenConns" yaml:"maxOpenConns"`

	// MaxIdleConns 最大空闲连接数
	MaxIdleConns int `json:"maxIdleConns" yaml:"maxIdleConns"`

	// ConnMaxLifetime 连接最大生命周期（秒）
	ConnMaxLifetime int `json:"connMaxLifetime" yaml:"connMaxLifetime"`

	// QueueSize 异步写入队列大小
	QueueSize int `json:"queueSize" yaml:"queueSize"`

	// FlushInterval 刷新间隔（毫秒）
	FlushInterval int `json:"flushInterval" yaml:"flushInterval"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		DBPath:          "./ignis_monitor.db",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 300,   // 5分钟
		QueueSize:       10000, // 异步队列大小
		FlushInterval:   1000,  // 1秒刷新一次
	}
}
