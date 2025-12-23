package audit

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/9triver/iarnet/internal/transport/http/util/response"
	"github.com/9triver/iarnet/internal/util"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// API 审计日志相关 API
type API struct {
	resMgr *resource.Manager
}

func NewAPI(resMgr *resource.Manager) *API {
	return &API{resMgr: resMgr}
}

// RegisterRoutes 注册审计相关路由
func RegisterRoutes(router *mux.Router, resMgr *resource.Manager) {
	api := NewAPI(resMgr)
	router.HandleFunc("/audit/logs", api.handleGetAllLogs).Methods("GET")
	logrus.Infof("Audit API routes registered: /audit/logs")
}

// LogLevel 日志级别（参考 common logger.proto）
type LogLevel int32

const (
	LogLevelUnknown LogLevel = 0
	LogLevelTrace   LogLevel = 1
	LogLevelDebug   LogLevel = 2
	LogLevelInfo    LogLevel = 3
	LogLevelWarn    LogLevel = 4
	LogLevelError   LogLevel = 5
	LogLevelFatal   LogLevel = 6
	LogLevelPanic   LogLevel = 7
)

// LogField 日志字段
type LogField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CallerInfo 调用栈信息
type CallerInfo struct {
	File     string `json:"file"`
	Line     int32  `json:"line"`
	Function string `json:"function"`
}

// LogEntry 单条日志条目
type LogEntry struct {
	Timestamp int64       `json:"timestamp"`        // Unix 纳秒
	Level     LogLevel    `json:"level"`            // 日志级别
	Message   string      `json:"message"`          // 日志消息
	Fields    []*LogField `json:"fields,omitempty"` // 其他字段
	Caller    *CallerInfo `json:"caller,omitempty"` // 调用栈
}

// GetAllLogsResponse 获取所有日志的响应
type GetAllLogsResponse struct {
	Logs  []*LogEntry `json:"logs"`
	Total int         `json:"total"`
}

// handleGetAllLogs 从日志文件中读取后端日志
func (api *API) handleGetAllLogs(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("Received request: %s %s", r.Method, r.URL.Path)
	query := r.URL.Query()

	limit, err := parsePositiveInt(query.Get("limit"), 100)
	if err != nil {
		response.BadRequest("invalid limit: " + err.Error()).WriteJSON(w)
		return
	}

	offset, err := parseNonNegativeInt(query.Get("offset"), 0)
	if err != nil {
		response.BadRequest("invalid offset: " + err.Error()).WriteJSON(w)
		return
	}

	levelFilter := strings.TrimSpace(query.Get("level"))
	if levelFilter != "" && strings.ToLower(levelFilter) == "all" {
		levelFilter = ""
	}

	logFilePath := util.GetLogFilePath()
	if logFilePath == "" {
		// 如果当前进程尚未初始化日志文件，返回空结果
		response.Success(GetAllLogsResponse{
			Logs:  []*LogEntry{},
			Total: 0,
		}).WriteJSON(w)
		return
	}

	logs, err := readLogsFromFile(logFilePath, limit, offset, levelFilter)
	if err != nil {
		logrus.Errorf("Failed to read logs from file: %v", err)
		response.InternalError("failed to read logs: " + err.Error()).WriteJSON(w)
		return
	}

	resp := GetAllLogsResponse{
		Logs:  logs,
		Total: len(logs),
	}
	response.Success(resp).WriteJSON(w)
}

// readLogsFromFile 从日志文件读取并解析日志
// 按时间倒序（最新的在前）返回
func readLogsFromFile(logFilePath string, limit, offset int, levelFilter string) ([]*LogEntry, error) {
	file, err := os.Open(logFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	var allLogs []*LogEntry

	// logrus 文本格式示例：
	// time="2025-12-23 15:40:32" level=info msg="xxx" func=github.com/...
	logPattern := regexp.MustCompile(`time="([^"]+)"\s+level=(\w+)\s+msg="([^"]+)"(?:\s+(.*))?$`)

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		matches := logPattern.FindStringSubmatch(line)
		if len(matches) < 4 {
			// 无法解析的行，作为 info 文本返回
			now := time.Now()
			allLogs = append(allLogs, &LogEntry{
				Timestamp: now.UnixNano(),
				Level:     LogLevelInfo,
				Message:   line,
			})
			continue
		}

		timestampStr := matches[1]
		levelStr := strings.ToLower(matches[2])
		message := matches[3]
		extra := ""
		if len(matches) > 4 {
			extra = strings.TrimSpace(matches[4])
		}

		// 级别过滤
		if levelFilter != "" && levelStr != strings.ToLower(levelFilter) {
			continue
		}

		// 解析时间
		timestamp, err := time.Parse("2006-01-02 15:04:05", timestampStr)
		if err != nil {
			timestamp = time.Now()
		}

		entry := &LogEntry{
			Timestamp: timestamp.UnixNano(),
			Level:     convertLogLevel(levelStr),
			Message:   message,
		}

		// 解析额外字段：形如 key=value
		if extra != "" {
			fields := parseLogFields(extra)
			if len(fields) > 0 {
				filtered := make([]*LogField, 0, len(fields))
				for _, f := range fields {
					switch f.Key {
					case "file":
						if entry.Caller == nil {
							entry.Caller = &CallerInfo{}
						}
						entry.Caller.File = f.Value
					case "line":
						if entry.Caller == nil {
							entry.Caller = &CallerInfo{}
						}
						if n, err := strconv.Atoi(f.Value); err == nil {
							entry.Caller.Line = int32(n)
						}
					case "func":
						if entry.Caller == nil {
							entry.Caller = &CallerInfo{}
						}
						entry.Caller.Function = f.Value
					default:
						filtered = append(filtered, f)
					}
				}
				entry.Fields = filtered
			}
		}

		allLogs = append(allLogs, entry)
	}

	// 应用 offset / limit
	start := offset
	if start > len(allLogs) {
		start = len(allLogs)
	}
	end := start + limit
	if end > len(allLogs) {
		end = len(allLogs)
	}
	if start >= len(allLogs) {
		return []*LogEntry{}, nil
	}
	return allLogs[start:end], nil
}

// 将字符串级别转换为 LogLevel
func convertLogLevel(levelStr string) LogLevel {
	switch levelStr {
	case "trace":
		return LogLevelTrace
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	case "fatal":
		return LogLevelFatal
	case "panic":
		return LogLevelPanic
	default:
		return LogLevelUnknown
	}
}

// 解析 key=value 形式的字段列表
func parseLogFields(fieldsStr string) []*LogField {
	var fields []*LogField
	parts := strings.Fields(fieldsStr)
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := kv[0]
		val := strings.Trim(kv[1], `"`)
		fields = append(fields, &LogField{
			Key:   key,
			Value: val,
		})
	}
	return fields
}

func parsePositiveInt(raw string, defaultVal int) (int, error) {
	if raw == "" {
		return defaultVal, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("must be positive integer")
	}
	return value, nil
}

func parseNonNegativeInt(raw string, defaultVal int) (int, error) {
	if raw == "" {
		return defaultVal, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("must be non-negative integer")
	}
	return value, nil
}
