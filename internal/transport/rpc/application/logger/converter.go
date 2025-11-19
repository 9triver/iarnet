package logger

import (
	"time"

	domainlogger "github.com/9triver/iarnet/internal/domain/application/logger"
	commonpb "github.com/9triver/iarnet/internal/proto/common"
)

// protoLogLevelToDomain 将 proto LogLevel 转换为 domain LogLevel
func protoLogLevelToDomain(level commonpb.LogLevel) domainlogger.LogLevel {
	switch level {
	case commonpb.LogLevel_LOG_LEVEL_TRACE:
		return domainlogger.LogLevelTrace
	case commonpb.LogLevel_LOG_LEVEL_DEBUG:
		return domainlogger.LogLevelDebug
	case commonpb.LogLevel_LOG_LEVEL_INFO:
		return domainlogger.LogLevelInfo
	case commonpb.LogLevel_LOG_LEVEL_WARN:
		return domainlogger.LogLevelWarn
	case commonpb.LogLevel_LOG_LEVEL_ERROR:
		return domainlogger.LogLevelError
	case commonpb.LogLevel_LOG_LEVEL_FATAL:
		return domainlogger.LogLevelFatal
	case commonpb.LogLevel_LOG_LEVEL_PANIC:
		return domainlogger.LogLevelPanic
	default:
		return domainlogger.LogLevelUnknown
	}
}

// domainLogLevelToProto 将 domain LogLevel 转换为 proto LogLevel
func domainLogLevelToProto(level domainlogger.LogLevel) commonpb.LogLevel {
	switch level {
	case domainlogger.LogLevelTrace:
		return commonpb.LogLevel_LOG_LEVEL_TRACE
	case domainlogger.LogLevelDebug:
		return commonpb.LogLevel_LOG_LEVEL_DEBUG
	case domainlogger.LogLevelInfo:
		return commonpb.LogLevel_LOG_LEVEL_INFO
	case domainlogger.LogLevelWarn:
		return commonpb.LogLevel_LOG_LEVEL_WARN
	case domainlogger.LogLevelError:
		return commonpb.LogLevel_LOG_LEVEL_ERROR
	case domainlogger.LogLevelFatal:
		return commonpb.LogLevel_LOG_LEVEL_FATAL
	case domainlogger.LogLevelPanic:
		return commonpb.LogLevel_LOG_LEVEL_PANIC
	default:
		return commonpb.LogLevel_LOG_LEVEL_UNKNOWN
	}
}

// protoLogEntryToDomain 将 proto LogEntry 转换为 domain Entry
func protoLogEntryToDomain(entry *commonpb.LogEntry) *domainlogger.Entry {
	if entry == nil {
		return nil
	}

	domainEntry := &domainlogger.Entry{
		Timestamp: time.Unix(0, entry.Timestamp),
		Level:     protoLogLevelToDomain(entry.Level),
		Message:   entry.Message,
	}

	// 转换 Fields
	if len(entry.Fields) > 0 {
		domainEntry.Fields = make([]domainlogger.LogField, len(entry.Fields))
		for i, field := range entry.Fields {
			domainEntry.Fields[i] = domainlogger.LogField{
				Key:   field.Key,
				Value: field.Value,
			}
		}
	}

	// 转换 Caller
	if entry.Caller != nil {
		domainEntry.Caller = &domainlogger.CallerInfo{
			File:     entry.Caller.File,
			Line:     int(entry.Caller.Line),
			Function: entry.Caller.Function,
		}
	}

	return domainEntry
}

// domainEntryToProto 将 domain Entry 转换为 proto LogEntry
func domainEntryToProto(entry *domainlogger.Entry) *commonpb.LogEntry {
	if entry == nil {
		return nil
	}

	protoEntry := &commonpb.LogEntry{
		Timestamp: entry.Timestamp.UnixNano(),
		Level:     domainLogLevelToProto(entry.Level),
		Message:   entry.Message,
	}

	// 转换 Fields
	if len(entry.Fields) > 0 {
		protoEntry.Fields = make([]*commonpb.LogField, len(entry.Fields))
		for i, field := range entry.Fields {
			protoEntry.Fields[i] = &commonpb.LogField{
				Key:   field.Key,
				Value: field.Value,
			}
		}
	}

	// 转换 Caller
	if entry.Caller != nil {
		protoEntry.Caller = &commonpb.CallerInfo{
			File:     entry.Caller.File,
			Line:     int32(entry.Caller.Line),
			Function: entry.Caller.Function,
		}
	}

	return protoEntry
}
