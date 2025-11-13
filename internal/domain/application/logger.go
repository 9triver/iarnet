package application

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// LoggerService 日志服务接口
type LoggerService interface {
	// GetLogs 获取容器日志
	GetLogs(ctx context.Context, containerID string, lines int) ([]string, error)

	// GetLogsParsed 获取解析后的日志
	GetLogsParsed(ctx context.Context, containerID, appName string, lines int) ([]*LogEntry, error)

	// StreamLogs 实时日志流（未来扩展）
	StreamLogs(ctx context.Context, containerID string) (<-chan *LogEntry, error)
}

// LogEntry 日志条目
type LogEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	App       string `json:"app"`
	Message   string `json:"message"`
	Details   string `json:"details"`
	RawLine   string `json:"raw_line"`
}

// logger 日志服务实现
type logger struct {
	dockerClient *client.Client
}

// NewLogger 创建日志服务
func NewLogger(dockerClient *client.Client) LoggerService {
	return &logger{
		dockerClient: dockerClient,
	}
}

// GetLogs 获取容器日志
func (l *logger) GetLogs(ctx context.Context, containerID string, lines int) ([]string, error) {
	if l.dockerClient == nil {
		return nil, fmt.Errorf("docker client not available")
	}

	// 获取 Docker 容器日志
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", lines),
	}

	reader, err := l.dockerClient.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %v", err)
	}
	defer reader.Close()

	// 读取日志内容
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read logs: %v", err)
	}

	// 分割为行
	logLines := strings.Split(string(content), "\n")

	// 过滤空行
	var filteredLines []string
	for _, line := range logLines {
		if strings.TrimSpace(line) != "" {
			filteredLines = append(filteredLines, line)
		}
	}

	return filteredLines, nil
}

// GetLogsParsed 获取解析后的日志
func (l *logger) GetLogsParsed(ctx context.Context, containerID, appName string, lines int) ([]*LogEntry, error) {
	rawLogs, err := l.GetLogs(ctx, containerID, lines)
	if err != nil {
		return nil, err
	}

	entries := make([]*LogEntry, 0)
	for _, line := range rawLogs {
		if entry := parseDockerLogLine(line, appName); entry != nil {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// StreamLogs 实时日志流（未来扩展）
func (l *logger) StreamLogs(ctx context.Context, containerID string) (<-chan *LogEntry, error) {
	ch := make(chan *LogEntry)
	close(ch)
	return ch, nil
}

// parseDockerLogLine 解析 Docker 日志行
func parseDockerLogLine(line, appName string) *LogEntry {
	// 移除 Docker 日志前缀
	line = strings.TrimSpace(line)
	if len(line) < 8 {
		return nil
	}

	// Docker 日志格式通常以 8 字节的头部开始，跳过它
	if line[0] < 32 {
		line = line[8:]
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	level := detectLogLevel(line)
	message := line
	details := ""

	// 尝试解析结构化日志
	if strings.Contains(line, "{") && strings.Contains(line, "}") {
		msg, det := parseStructuredLog(line)
		if msg != "" {
			message = msg
			details = det
		}
	}

	// 移除嵌入的时间戳
	message = removeEmbeddedTimestamp(message)

	return &LogEntry{
		ID:        fmt.Sprintf("%s-%d", appName, time.Now().UnixNano()),
		Timestamp: timestamp,
		Level:     level,
		App:       appName,
		Message:   message,
		Details:   details,
		RawLine:   line,
	}
}

// parseStructuredLog 解析结构化日志
func parseStructuredLog(message string) (string, string) {
	// 简化实现：提取主要消息和详细信息
	if idx := strings.Index(message, "{"); idx != -1 {
		mainMsg := strings.TrimSpace(message[:idx])
		details := strings.TrimSpace(message[idx:])
		return mainMsg, details
	}
	return message, ""
}

// removeEmbeddedTimestamp 移除嵌入的时间戳
func removeEmbeddedTimestamp(message string) string {
	// 常见时间戳格式
	patterns := []string{
		`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}[.\d]*Z?`,
		`\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`,
		`\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\]`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		message = re.ReplaceAllString(message, "")
	}

	return strings.TrimSpace(message)
}

// detectLogLevel 检测日志级别
func detectLogLevel(message string) string {
	messageLower := strings.ToLower(message)

	if strings.Contains(messageLower, "error") || strings.Contains(messageLower, "fatal") || strings.Contains(messageLower, "critical") {
		return "error"
	}
	if strings.Contains(messageLower, "warn") || strings.Contains(messageLower, "warning") {
		return "warn"
	}
	if strings.Contains(messageLower, "debug") || strings.Contains(messageLower, "trace") {
		return "debug"
	}
	return "info"
}
