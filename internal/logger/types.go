package logger

import (
	"time"
)

// LogLevel 日志级别
type LogLevel string

const (
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
	LogLevelFatal   LogLevel = "fatal"
	LogLevelUnknown LogLevel = "unknown"
)

// ContainerType 容器类型
type ContainerType string

const (
	ContainerTypeRunner    ContainerType = "runner"
	ContainerTypeComponent ContainerType = "component"
)

// LogSource 日志来源
type LogSource string

const (
	LogSourceStdout LogSource = "stdout"
	LogSourceStderr LogSource = "stderr"
)

// LogEntry 原始日志条目
type LogEntry struct {
	Timestamp     time.Time         `json:"timestamp"`
	ContainerID   string            `json:"container_id"`
	ContainerType ContainerType     `json:"container_type"`
	Level         LogLevel          `json:"level"`
	Message       string            `json:"message"`
	Source        LogSource         `json:"source"`
	Labels        map[string]string `json:"labels"`
	Raw           string            `json:"raw,omitempty"` // 原始日志行
}

// ProcessedEntry 处理后的日志条目
type ProcessedEntry struct {
	*LogEntry
	StreamID   string    `json:"stream_id"`  // 日志流ID
	ChunkID    string    `json:"chunk_id"`   // 所属块ID
	ParsedAt   time.Time `json:"parsed_at"`  // 解析时间
	Structured bool      `json:"structured"` // 是否结构化日志
}

// LogStream 日志流（唯一的标签组合）
type LogStream struct {
	StreamID  string            `json:"stream_id"`
	Labels    map[string]string `json:"labels"`
	FirstSeen time.Time         `json:"first_seen"`
	LastSeen  time.Time         `json:"last_seen"`
}

// LogChunk 日志块
type LogChunk struct {
	ChunkID          string    `json:"chunk_id"`
	StreamID         string    `json:"stream_id"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	FilePath         string    `json:"file_path"`
	CompressedSize   int64     `json:"compressed_size"`
	UncompressedSize int64     `json:"uncompressed_size"`
	LineCount        int       `json:"line_count"`
}

// QueryRequest 查询请求
type QueryRequest struct {
	Labels     map[string]string `json:"labels"`      // 标签过滤
	StartTime  time.Time         `json:"start_time"`  // 开始时间
	EndTime    time.Time         `json:"end_time"`    // 结束时间
	Grep       string            `json:"grep"`        // 内容搜索（可选）
	Regex      string            `json:"regex"`       // 正则匹配（可选）
	Limit      int               `json:"limit"`       // 返回行数限制
	Direction  QueryDirection    `json:"direction"`   // 查询方向
	Level      LogLevel          `json:"level"`       // 日志级别过滤
	IncludeRaw bool              `json:"include_raw"` // 是否包含原始日志
}

// QueryDirection 查询方向
type QueryDirection string

const (
	QueryDirectionForward  QueryDirection = "forward"  // 从旧到新
	QueryDirectionBackward QueryDirection = "backward" // 从新到旧
)

// QueryResult 查询结果
type QueryResult struct {
	Entries   []*LogEntry   `json:"entries"`
	Total     int           `json:"total"`      // 匹配的总数（可能被限制）
	Limited   bool          `json:"limited"`    // 是否达到限制
	QueryTime time.Duration `json:"query_time"` // 查询耗时
	Stats     *QueryStats   `json:"stats"`
}

// QueryStats 查询统计
type QueryStats struct {
	ChunksScanned int   `json:"chunks_scanned"` // 扫描的块数
	BytesScanned  int64 `json:"bytes_scanned"`  // 扫描的字节数
	LinesScanned  int   `json:"lines_scanned"`  // 扫描的行数
}

// TailRequest 尾随请求
type TailRequest struct {
	Labels map[string]string `json:"labels"`
	Lines  int               `json:"lines"`  // 初始行数
	Level  LogLevel          `json:"level"`  // 日志级别过滤
	Follow bool              `json:"follow"` // 是否持续跟随
}

// LogStats 日志统计
type LogStats struct {
	TotalStreams    int                `json:"total_streams"`
	TotalChunks     int                `json:"total_chunks"`
	TotalLines      int64              `json:"total_lines"`
	TotalBytes      int64              `json:"total_bytes"`
	OldestLog       time.Time          `json:"oldest_log"`
	NewestLog       time.Time          `json:"newest_log"`
	LevelCounts     map[LogLevel]int64 `json:"level_counts"`
	ContainerCounts map[string]int64   `json:"container_counts"`
}

// CollectorConfig 收集器配置
type CollectorConfig struct {
	BufferSize    int           `yaml:"buffer_size"`    // 缓冲区大小
	FlushInterval time.Duration `yaml:"flush_interval"` // 刷新间隔
	BatchSize     int           `yaml:"batch_size"`     // 批量大小
	MaxLineSize   int           `yaml:"max_line_size"`  // 最大行大小
	FollowTimeout time.Duration `yaml:"follow_timeout"` // 跟随超时
}

// StorageConfig 存储配置
type StorageConfig struct {
	DataDir          string        `yaml:"data_dir"`          // 数据目录
	DBPath           string        `yaml:"db_path"`           // 数据库路径
	ChunkDuration    time.Duration `yaml:"chunk_duration"`    // 块时间长度
	ChunkMaxLines    int           `yaml:"chunk_max_lines"`   // 块最大行数
	ChunkMaxSize     int64         `yaml:"chunk_max_size"`    // 块最大大小（字节）
	CompressionLevel int           `yaml:"compression_level"` // 压缩级别（1-9）
	RetentionPeriod  time.Duration `yaml:"retention_period"`  // 保留期限
	CleanupInterval  time.Duration `yaml:"cleanup_interval"`  // 清理间隔
	MaxDiskUsage     int64         `yaml:"max_disk_usage"`    // 最大磁盘使用（字节）
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	EnableParsing     bool     `yaml:"enable_parsing"`      // 是否解析结构化日志
	EnableLevelDetect bool     `yaml:"enable_level_detect"` // 是否检测日志级别
	TimeFormats       []string `yaml:"time_formats"`        // 时间格式列表
	DropEmptyLines    bool     `yaml:"drop_empty_lines"`    // 是否丢弃空行
	MaxMessageSize    int      `yaml:"max_message_size"`    // 最大消息大小
}

// LogSystemConfig 日志系统总配置
type LogSystemConfig struct {
	Enabled   bool            `yaml:"enabled"`
	Collector CollectorConfig `yaml:"collector"`
	Storage   StorageConfig   `yaml:"storage"`
	Processor ProcessorConfig `yaml:"processor"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *LogSystemConfig {
	return &LogSystemConfig{
		Enabled: true,
		Collector: CollectorConfig{
			BufferSize:    10000,
			FlushInterval: 5 * time.Second,
			BatchSize:     1000,
			MaxLineSize:   64 * 1024, // 64KB
			FollowTimeout: 30 * time.Second,
		},
		Storage: StorageConfig{
			DataDir:          "./data/logs",
			DBPath:           "./data/logs.db",
			ChunkDuration:    5 * time.Minute,
			ChunkMaxLines:    10000,
			ChunkMaxSize:     10 * 1024 * 1024, // 10MB
			CompressionLevel: 6,
			RetentionPeriod:  7 * 24 * time.Hour, // 7 days
			CleanupInterval:  1 * time.Hour,
			MaxDiskUsage:     10 * 1024 * 1024 * 1024, // 10GB
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
