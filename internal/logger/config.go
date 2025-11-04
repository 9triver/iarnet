package logger

import (
	"time"

	appConfig "github.com/9triver/iarnet/internal/config"
)

// ConfigFromAppConfig 从应用配置转换为日志系统配置
func ConfigFromAppConfig(cfg *appConfig.Config) *LogSystemConfig {
	if !cfg.Logging.Enabled {
		return &LogSystemConfig{Enabled: false}
	}

	return &LogSystemConfig{
		Enabled: cfg.Logging.Enabled,
		Collector: CollectorConfig{
			BufferSize:    cfg.Logging.BufferSize,
			FlushInterval: time.Duration(cfg.Logging.FlushIntervalSeconds) * time.Second,
			BatchSize:     cfg.Logging.BatchSize,
			MaxLineSize:   64 * 1024, // 64KB
			FollowTimeout: 30 * time.Second,
		},
		Storage: StorageConfig{
			DataDir:          cfg.Logging.DataDir,
			DBPath:           cfg.Logging.DBPath,
			ChunkDuration:    time.Duration(cfg.Logging.ChunkDurationMinutes) * time.Minute,
			ChunkMaxLines:    cfg.Logging.ChunkMaxLines,
			ChunkMaxSize:     int64(cfg.Logging.ChunkMaxSizeMB) * 1024 * 1024,
			CompressionLevel: cfg.Logging.CompressionLevel,
			RetentionPeriod:  time.Duration(cfg.Logging.RetentionDays) * 24 * time.Hour,
			CleanupInterval:  time.Duration(cfg.Logging.CleanupIntervalHours) * time.Hour,
			MaxDiskUsage:     int64(cfg.Logging.MaxDiskUsageGB) * 1024 * 1024 * 1024,
		},
		Processor: ProcessorConfig{
			EnableParsing:     true,
			EnableLevelDetect: true,
			TimeFormats: []string{
				time.RFC3339,
				time.RFC3339Nano,
				"2006-01-02 15:04:05",
				"2006/01/02 15:04:05",
			},
			DropEmptyLines: true,
			MaxMessageSize: 32 * 1024, // 32KB
		},
	}
}

