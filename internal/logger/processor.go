package logger

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Processor 日志处理器接口
type Processor interface {
	// Process 处理单个日志条目
	Process(entry *LogEntry) (*ProcessedEntry, error)

	// ProcessBatch 批量处理日志条目
	ProcessBatch(entries []*LogEntry) ([]*ProcessedEntry, error)
}

// processor 处理器实现
type processor struct {
	config *ProcessorConfig

	// 级别检测正则
	levelPatterns map[LogLevel]*regexp.Regexp

	// 时间解析格式
	timeFormats []string
}

// NewProcessor 创建新的处理器
func NewProcessor(config *ProcessorConfig) Processor {
	if config == nil {
		config = &DefaultConfig().Processor
	}

	p := &processor{
		config:        config,
		levelPatterns: make(map[LogLevel]*regexp.Regexp),
		timeFormats:   config.TimeFormats,
	}

	// 初始化级别检测正则
	if config.EnableLevelDetect {
		p.levelPatterns[LogLevelDebug] = regexp.MustCompile(`(?i)\b(debug|trace)\b`)
		p.levelPatterns[LogLevelInfo] = regexp.MustCompile(`(?i)\b(info|information)\b`)
		p.levelPatterns[LogLevelWarning] = regexp.MustCompile(`(?i)\b(warn|warning)\b`)
		p.levelPatterns[LogLevelError] = regexp.MustCompile(`(?i)\b(error|err|exception)\b`)
		p.levelPatterns[LogLevelFatal] = regexp.MustCompile(`(?i)\b(fatal|critical|panic)\b`)
	}

	return p
}

// Process 处理单个日志条目
func (p *processor) Process(entry *LogEntry) (*ProcessedEntry, error) {
	processed := &ProcessedEntry{
		LogEntry:   entry,
		StreamID:   "", // 将由存储层填充
		ChunkID:    "", // 将由存储层填充
		ParsedAt:   time.Now(),
		Structured: false,
	}

	// 解析结构化日志
	if p.config.EnableParsing {
		if parsed := p.tryParseStructured(entry.Raw); parsed != nil {
			p.enrichFromStructured(entry, parsed)
			processed.Structured = true
		}
	}

	// 检测日志级别
	if p.config.EnableLevelDetect && entry.Level == LogLevelUnknown {
		entry.Level = p.detectLevel(entry.Message)
	}

	// 规范化时间戳
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// 限制消息大小
	if p.config.MaxMessageSize > 0 && len(entry.Message) > p.config.MaxMessageSize {
		entry.Message = entry.Message[:p.config.MaxMessageSize] + "... [truncated]"
	}

	return processed, nil
}

// ProcessBatch 批量处理
func (p *processor) ProcessBatch(entries []*LogEntry) ([]*ProcessedEntry, error) {
	processed := make([]*ProcessedEntry, 0, len(entries))

	for _, entry := range entries {
		if p.config.DropEmptyLines && strings.TrimSpace(entry.Message) == "" {
			continue
		}

		result, err := p.Process(entry)
		if err != nil {
			logrus.Debugf("Failed to process log entry: %v", err)
			continue
		}

		processed = append(processed, result)
	}

	return processed, nil
}

// tryParseStructured 尝试解析结构化日志
func (p *processor) tryParseStructured(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "{") {
		return nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil
	}

	return data
}

// enrichFromStructured 从结构化数据中提取信息
func (p *processor) enrichFromStructured(entry *LogEntry, data map[string]interface{}) {
	// 提取级别
	if level, ok := data["level"].(string); ok {
		entry.Level = p.normalizeLevel(level)
	} else if level, ok := data["lvl"].(string); ok {
		entry.Level = p.normalizeLevel(level)
	} else if severity, ok := data["severity"].(string); ok {
		entry.Level = p.normalizeLevel(severity)
	}

	// 提取消息
	if msg, ok := data["message"].(string); ok && msg != "" {
		entry.Message = msg
	} else if msg, ok := data["msg"].(string); ok && msg != "" {
		entry.Message = msg
	}

	// 提取时间戳
	if ts, ok := data["timestamp"].(string); ok {
		if parsed := p.parseTimestamp(ts); !parsed.IsZero() {
			entry.Timestamp = parsed
		}
	} else if ts, ok := data["ts"].(string); ok {
		if parsed := p.parseTimestamp(ts); !parsed.IsZero() {
			entry.Timestamp = parsed
		}
	} else if ts, ok := data["time"].(string); ok {
		if parsed := p.parseTimestamp(ts); !parsed.IsZero() {
			entry.Timestamp = parsed
		}
	}

	// 提取其他字段作为标签
	if entry.Labels == nil {
		entry.Labels = make(map[string]string)
	}

	for key, value := range data {
		if key == "level" || key == "message" || key == "timestamp" ||
			key == "lvl" || key == "msg" || key == "ts" || key == "time" {
			continue
		}

		if strVal, ok := value.(string); ok {
			entry.Labels[key] = strVal
		}
	}
}

// parseTimestamp 解析时间戳
func (p *processor) parseTimestamp(ts string) time.Time {
	for _, format := range p.timeFormats {
		if t, err := time.Parse(format, ts); err == nil {
			return t
		}
	}
	return time.Time{}
}

// detectLevel 检测日志级别
func (p *processor) detectLevel(message string) LogLevel {
	// 按优先级检测
	levels := []LogLevel{
		LogLevelFatal,
		LogLevelError,
		LogLevelWarning,
		LogLevelInfo,
		LogLevelDebug,
	}

	for _, level := range levels {
		if pattern, exists := p.levelPatterns[level]; exists {
			if pattern.MatchString(message) {
				return level
			}
		}
	}

	return LogLevelInfo // 默认为 info
}

// normalizeLevel 规范化日志级别
func (p *processor) normalizeLevel(level string) LogLevel {
	level = strings.ToLower(strings.TrimSpace(level))

	switch level {
	case "debug", "trace", "dbg":
		return LogLevelDebug
	case "info", "information", "inf":
		return LogLevelInfo
	case "warn", "warning", "wrn":
		return LogLevelWarning
	case "error", "err", "exception":
		return LogLevelError
	case "fatal", "critical", "crit", "panic":
		return LogLevelFatal
	default:
		return LogLevelUnknown
	}
}
