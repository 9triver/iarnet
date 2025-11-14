package logging

// Config 日志系统配置
type Config struct {
	Enabled              bool   `yaml:"enabled"`                // 是否启用日志系统
	DataDir              string `yaml:"data_dir"`               // 日志数据目录
	DBPath               string `yaml:"db_path"`                // 日志元数据数据库路径
	ChunkDurationMinutes int    `yaml:"chunk_duration_minutes"` // 块时间长度（分钟）
	ChunkMaxLines        int    `yaml:"chunk_max_lines"`        // 块最大行数
	ChunkMaxSizeMB       int    `yaml:"chunk_max_size_mb"`      // 块最大大小（MB）
	CompressionLevel     int    `yaml:"compression_level"`      // 压缩级别（1-9）
	RetentionDays        int    `yaml:"retention_days"`         // 保留天数
	CleanupIntervalHours int    `yaml:"cleanup_interval_hours"` // 清理间隔（小时）
	MaxDiskUsageGB       int    `yaml:"max_disk_usage_gb"`      // 最大磁盘使用（GB）
	BufferSize           int    `yaml:"buffer_size"`            // 缓冲区大小
	FlushIntervalSeconds int    `yaml:"flush_interval_seconds"` // 刷新间隔（秒）
	BatchSize            int    `yaml:"batch_size"`             // 批量大小
}

// ApplyDefaults 为配置项设置默认值
func (c *Config) ApplyDefaults(dataDir string) {
	if c.DataDir == "" {
		c.DataDir = dataDir + "/logs"
	}
	if c.DBPath == "" {
		c.DBPath = dataDir + "/logs.db"
	}
	if c.ChunkDurationMinutes == 0 {
		c.ChunkDurationMinutes = 5
	}
	if c.ChunkMaxLines == 0 {
		c.ChunkMaxLines = 10000
	}
	if c.ChunkMaxSizeMB == 0 {
		c.ChunkMaxSizeMB = 10
	}
	if c.CompressionLevel == 0 {
		c.CompressionLevel = 6
	}
	if c.RetentionDays == 0 {
		c.RetentionDays = 7
	}
	if c.CleanupIntervalHours == 0 {
		c.CleanupIntervalHours = 1
	}
	if c.MaxDiskUsageGB == 0 {
		c.MaxDiskUsageGB = 10
	}
	if c.BufferSize == 0 {
		c.BufferSize = 10000
	}
	if c.FlushIntervalSeconds == 0 {
		c.FlushIntervalSeconds = 5
	}
	if c.BatchSize == 0 {
		c.BatchSize = 1000
	}
}
