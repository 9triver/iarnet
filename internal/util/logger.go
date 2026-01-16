package util

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	logFile     *os.File
	logFilePath string
	logFileMu   sync.Mutex
	currentDate string        // 当前日志文件对应的日期（格式：20060102）
	stopCleanup chan struct{} // 用于停止日志清理 goroutine
	fileHook    *FileHook     // 全局 FileHook 实例引用，用于日志轮换
)

// FileHook logrus Hook 实现，用于将日志写入文件
type FileHook struct {
	file      *os.File
	formatter logrus.Formatter
	mu        sync.Mutex
}

// UpdateFile 更新文件句柄（用于日志轮换）
func (hook *FileHook) UpdateFile(newFile *os.File) {
	hook.mu.Lock()
	defer hook.mu.Unlock()

	// 关闭旧文件
	if hook.file != nil {
		hook.file.Close()
	}

	// 设置新文件
	hook.file = newFile
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

// createLogFile 创建指定日期的日志文件
func createLogFile(logDir string, date string) (*os.File, string, error) {
	logFileName := fmt.Sprintf("iarnet-%s.log", date)
	logFilePath := filepath.Join(logDir, logFileName)

	// 打开日志文件（追加模式）
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open log file: %w", err)
	}

	return file, logFilePath, nil
}

// rotateLogFile 轮换日志文件（如果日期变化）
func rotateLogFile(logDir string) error {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	today := time.Now().Format("20060102")

	// 如果日期没有变化，不需要轮换
	if currentDate == today && logFile != nil {
		return nil
	}

	// 创建新文件
	file, newLogFilePath, err := createLogFile(logDir, today)
	if err != nil {
		return err
	}

	// 更新 FileHook 的文件句柄（如果已存在）
	if fileHook != nil {
		fileHook.UpdateFile(file)
	} else {
		// 如果 FileHook 不存在，创建新的
		fileFormatter := &logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.DateTime,
			DisableColors:   true, // 文件输出不需要颜色
			CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
				return frame.Function, "" // 与 InitLogger 保持一致
			},
		}

		fileHook = &FileHook{
			file:      file,
			formatter: fileFormatter,
		}
		logrus.AddHook(fileHook)
	}

	// 更新全局变量
	oldFile := logFile
	logFile = file
	logFilePath = newLogFilePath
	currentDate = today

	// 关闭旧文件（在更新 hook 之后）
	if oldFile != nil {
		oldFile.Close()
	}

	return nil
}

// cleanupOldLogs 清理超过指定天数的旧日志文件
func cleanupOldLogs(logDir string, keepDays int) error {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	cutoffTime := time.Now().AddDate(0, 0, -keepDays)
	cutoffDate := cutoffTime.Format("20060102")

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// 只处理 iarnet-YYYYMMDD.log 格式的文件
		if !strings.HasPrefix(name, "iarnet-") || !strings.HasSuffix(name, ".log") {
			continue
		}

		// 提取日期部分
		dateStr := strings.TrimPrefix(strings.TrimSuffix(name, ".log"), "iarnet-")
		if len(dateStr) != 8 { // YYYYMMDD 格式应该是8位
			continue
		}

		// 比较日期字符串（因为格式是 YYYYMMDD，可以直接字符串比较）
		if dateStr < cutoffDate {
			filePath := filepath.Join(logDir, name)
			if err := os.Remove(filePath); err != nil {
				logrus.Warnf("Failed to delete old log file %s: %v", filePath, err)
			} else {
				logrus.Infof("Deleted old log file: %s", filePath)
			}
		}
	}

	return nil
}

// startLogRotation 启动日志轮换和清理的定时任务
func startLogRotation(logDir string, keepDays int) {
	stopCleanup = make(chan struct{})

	// 启动定时任务
	go func() {
		// 每小时检查一次是否需要轮换日志文件
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		// 每天凌晨执行一次日志清理
		cleanupTicker := time.NewTicker(24 * time.Hour)
		defer cleanupTicker.Stop()

		// 立即执行一次清理
		if err := cleanupOldLogs(logDir, keepDays); err != nil {
			logrus.Warnf("Failed to cleanup old logs: %v", err)
		}

		for {
			select {
			case <-ticker.C:
				// 检查是否需要轮换日志文件
				if err := rotateLogFile(logDir); err != nil {
					logrus.Errorf("Failed to rotate log file: %v", err)
				}
			case <-cleanupTicker.C:
				// 清理旧日志文件
				if err := cleanupOldLogs(logDir, keepDays); err != nil {
					logrus.Warnf("Failed to cleanup old logs: %v", err)
				}
			case <-stopCleanup:
				return
			}
		}
	}()
}

// stopLogRotation 停止日志轮换和清理的定时任务
func stopLogRotation() {
	if stopCleanup != nil {
		close(stopCleanup)
		stopCleanup = nil
	}
}

// InitLoggerWithFile 初始化日志系统并设置文件输出
// logDir: 日志文件目录
// keepDays: 保留日志文件的天数（默认3天）
// 返回日志文件路径
func InitLoggerWithFile(logDir string) (string, error) {
	return InitLoggerWithFileAndRetention(logDir, 3)
}

// InitLoggerWithFileAndRetention 初始化日志系统并设置文件输出和保留天数
// logDir: 日志文件目录
// keepDays: 保留日志文件的天数
// 返回日志文件路径
func InitLoggerWithFileAndRetention(logDir string, keepDays int) (string, error) {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create log directory: %w", err)
	}

	// 使用当前日期命名日志文件（格式：iarnet-20060102.log）
	today := time.Now().Format("20060102")
	file, newLogFilePath, err := createLogFile(logDir, today)
	if err != nil {
		return "", err
	}

	logFile = file
	logFilePath = newLogFilePath // 设置全局变量 logFilePath
	currentDate = today

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
	fileHook = &FileHook{
		file:      file,
		formatter: fileFormatter,
	}
	logrus.AddHook(fileHook)
	// 设置控制台输出级别为 Info，文件输出会通过 Hook 记录所有级别（包括 Debug）
	logrus.SetLevel(logrus.InfoLevel)

	// 启动日志轮换和清理任务
	startLogRotation(logDir, keepDays)

	// 使用 fmt.Printf 输出到标准错误，避免触发 logrus（可能导致循环）
	fmt.Fprintf(os.Stderr, "Logging to file: %s (keeping %d days of logs)\n", newLogFilePath, keepDays)
	return newLogFilePath, nil
}

// GetLogFilePath 获取当前日志文件路径
func GetLogFilePath() string {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	return logFilePath
}

// CloseLogFile 关闭日志文件
func CloseLogFile() error {
	// 停止日志轮换任务
	stopLogRotation()

	logFileMu.Lock()
	defer logFileMu.Unlock()

	if logFile != nil {
		err := logFile.Close()
		logFile = nil
		logFilePath = ""
		currentDate = ""
		return err
	}
	return nil
}
