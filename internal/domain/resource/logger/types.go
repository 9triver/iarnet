package logger

import "time"

// LogLevel 日志级别，对应 logrus 的日志级别
type LogLevel string

const (
	LogLevelUnknown LogLevel = "unknown"
	LogLevelTrace   LogLevel = "trace"
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarn    LogLevel = "warn"
	LogLevelError   LogLevel = "error"
	LogLevelFatal   LogLevel = "fatal"
	LogLevelPanic   LogLevel = "panic"
)

// LogField 日志字段，用于存储 logrus Fields
type LogField struct {
	Key   string
	Value string // JSON 编码的值，支持各种类型
}

// CallerInfo 调用栈信息
type CallerInfo struct {
	File     string
	Line     int
	Function string
}

// Entry 单条日志条目
type Entry struct {
	// 基础信息
	Timestamp time.Time
	Level     LogLevel
	Message   string

	// 元数据
	Fields []LogField
	Caller *CallerInfo // 调用栈信息（可选）
}

// SubmitLogResult 单条日志提交结果
type SubmitLogResult struct {
	Success bool
	Error   string // 错误信息（如果 success = false）
	LogID   string // 日志 ID（用于追踪，可选）
}

type LogID = string

// BatchSubmitLogResult 批量日志提交结果
type BatchSubmitLogResult struct {
	Success       bool
	Error         string
	AcceptedCount int      // 成功接受的日志数量
	RejectedCount int      // 被拒绝的日志数量
	LogIDs        []string // 日志 ID 列表（可选）
}

// QueryOptions 日志查询选项
type QueryOptions struct {
	Limit     int        // 每页数量，默认 100
	Offset    int        // 偏移量，默认 0
	Level     LogLevel   // 日志级别过滤（可选）
	StartTime *time.Time // 开始时间（可选）
	EndTime   *time.Time // 结束时间（可选）
}

// QueryResult 日志查询结果
type QueryResult struct {
	Entries []*Entry // 日志条目列表
	Total   int      // 总数量（如果支持）
	HasMore bool     // 是否还有更多数据
}
