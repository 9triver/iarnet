package util

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	logFile     *os.File
	logFilePath string
	logFileMu   sync.Mutex
)

// FileHook logrus Hook 实现，用于将日志写入文件
type FileHook struct {
	file      *os.File
	formatter logrus.Formatter
	mu        sync.Mutex
}

// Levels 返回 Hook 要处理的日志级别
func (hook *FileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire 写入日志到文件
func (hook *FileHook) Fire(entry *logrus.Entry) error {
	hook.mu.Lock()
	defer hook.mu.Unlock()

	if hook.file == nil {
		return nil
	}

	// 使用配置的格式化器格式化日志
	if hook.formatter == nil {
		// 如果没有配置格式化器，使用默认的
		hook.formatter = &logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.DateTime,
			DisableColors:   true,
		}
	}
	line, err := hook.formatter.Format(entry)
	if err != nil {
		return err
	}

	_, err = hook.file.Write(line)
	return err
}

// Close 关闭文件
func (hook *FileHook) Close() error {
	hook.mu.Lock()
	defer hook.mu.Unlock()

	if hook.file != nil {
		err := hook.file.Close()
		hook.file = nil
		return err
	}
	return nil
}

// InitLogger 初始化日志系统，将日志输出到文件
func InitLogger() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.DateTime,
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			return frame.Function, "" // TODO: 生成包的简写
		},
	})
	logrus.SetReportCaller(true)
}

// InitLoggerWithFile 初始化日志系统并设置文件输出
// logDir: 日志文件目录
// 返回日志文件路径
func InitLoggerWithFile(logDir string) (string, error) {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create log directory: %w", err)
	}

	// 使用启动时间命名日志文件
	startTime := time.Now()
	logFileName := fmt.Sprintf("iarnet-%s.log", startTime.Format("20060102-150405"))
	logFilePath = filepath.Join(logDir, logFileName)

	// 打开日志文件（追加模式）
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return "", fmt.Errorf("failed to open log file: %w", err)
	}

	logFile = file

	// 创建与标准输出相同的格式化器（但禁用颜色）
	fileFormatter := &logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.DateTime,
		DisableColors:   true, // 文件输出不需要颜色
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			return frame.Function, "" // 与 InitLogger 保持一致
		},
	}

	// 创建 FileHook 并添加到 logrus
	fileHook := &FileHook{
		file:      file,
		formatter: fileFormatter,
	}
	logrus.AddHook(fileHook)
	logrus.SetLevel(logrus.DebugLevel)

	// 使用 fmt.Printf 输出到标准错误，避免触发 logrus（可能导致循环）
	fmt.Fprintf(os.Stderr, "Logging to file: %s\n", logFilePath)
	return logFilePath, nil
}

// GetLogFilePath 获取当前日志文件路径
func GetLogFilePath() string {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	return logFilePath
}

// CloseLogFile 关闭日志文件
func CloseLogFile() error {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	if logFile != nil {
		err := logFile.Close()
		logFile = nil
		return err
	}
	return nil
}
