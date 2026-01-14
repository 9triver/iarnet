package util

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestLoggerRotation(t *testing.T) {
	// 创建临时目录用于测试
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	// 初始化日志系统，保留3天
	logPath, err := InitLoggerWithFileAndRetention(logDir, 3)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	defer CloseLogFile()

	// 验证日志文件已创建
	if logPath == "" {
		t.Error("Log path should not be empty")
	}

	// 验证日志文件存在
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Log file should exist at %s", logPath)
	}

	// 写入一些日志
	logrus.Info("Test log message 1")
	logrus.Info("Test log message 2")

	// 验证日志文件有内容
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}
	if fileInfo.Size() == 0 {
		t.Error("Log file should not be empty")
	}

	// 验证日志文件命名格式正确（iarnet-YYYYMMDD.log）
	expectedDate := time.Now().Format("20060102")
	expectedName := "iarnet-" + expectedDate + ".log"
	if filepath.Base(logPath) != expectedName {
		t.Errorf("Log file name should be %s, got %s", expectedName, filepath.Base(logPath))
	}
}

func TestLogRotation(t *testing.T) {
	// 创建临时目录用于测试
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	// 初始化日志系统
	logPath, err := InitLoggerWithFileAndRetention(logDir, 3)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	defer CloseLogFile()

	// 等待一小段时间，确保初始化完成
	time.Sleep(10 * time.Millisecond)

	// 获取初始日期和文件路径
	logFileMu.Lock()
	initialDate := currentDate
	initialPath := logFilePath
	logFileMu.Unlock()

	// 验证初始状态
	if initialPath == "" {
		// 如果为空，使用返回的 logPath
		initialPath = logPath
		if initialPath == "" {
			t.Fatal("Initial log path should not be empty")
		}
	}
	if logPath == "" {
		t.Fatal("Returned log path should not be empty")
	}

	// 验证初始文件存在
	if _, err := os.Stat(initialPath); os.IsNotExist(err) {
		t.Fatalf("Initial log file should exist at %s", initialPath)
	}

	// 模拟日期变化（通过直接修改 currentDate）
	// 注意：在实际使用中，日期变化由定时任务检测
	logFileMu.Lock()
	// 设置一个过去的日期来触发轮换（使用昨天）
	yesterday := time.Now().AddDate(0, 0, -1).Format("20060102")
	currentDate = yesterday
	logFileMu.Unlock()

	// 手动触发轮换（应该检测到今天并创建今天的文件）
	err = rotateLogFile(logDir)
	if err != nil {
		t.Fatalf("Failed to rotate log file: %v", err)
	}

	// 验证新文件已创建
	logFileMu.Lock()
	newPath := logFilePath
	newDate := currentDate
	logFileMu.Unlock()

	// 验证日期已更新为今天
	today := time.Now().Format("20060102")
	if newDate != today {
		t.Errorf("Current date should be today (%s), got %s", today, newDate)
	}

	// 验证路径已更新（如果日期不同，路径应该不同）
	if initialDate == today {
		// 如果初始日期就是今天，那么路径可能相同，这是正常的
		t.Logf("Initial date was already today, path may be the same")
	} else if newPath == initialPath {
		t.Error("Log file path should change after rotation when date changes")
	}

	// 验证新文件存在
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Errorf("New log file should exist at %s", newPath)
	}

	// 验证旧文件仍然存在（因为只是创建了新文件，没有删除旧文件）
	if initialDate != today {
		// 只有当日期不同时，旧文件才应该存在
		if _, err := os.Stat(initialPath); os.IsNotExist(err) {
			t.Errorf("Old log file should still exist at %s", initialPath)
		}
	}

	// 写入日志到新文件
	logrus.Info("Test log message after rotation")
}

func TestLogCleanup(t *testing.T) {
	// 创建临时目录用于测试
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	// 确保目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("Failed to create log directory: %v", err)
	}

	// 创建一些测试日志文件
	now := time.Now()

	// 创建今天的日志文件（应该保留）
	today := now.Format("20060102")
	todayFile := filepath.Join(logDir, "iarnet-"+today+".log")
	os.WriteFile(todayFile, []byte("today log"), 0666)

	// 创建昨天的日志文件（应该保留，因为保留3天）
	yesterday := now.AddDate(0, 0, -1).Format("20060102")
	yesterdayFile := filepath.Join(logDir, "iarnet-"+yesterday+".log")
	os.WriteFile(yesterdayFile, []byte("yesterday log"), 0666)

	// 创建3天前的日志文件（应该保留，因为保留3天）
	threeDaysAgo := now.AddDate(0, 0, -3).Format("20060102")
	threeDaysAgoFile := filepath.Join(logDir, "iarnet-"+threeDaysAgo+".log")
	os.WriteFile(threeDaysAgoFile, []byte("three days ago log"), 0666)

	// 创建4天前的日志文件（应该被删除）
	fourDaysAgo := now.AddDate(0, 0, -4).Format("20060102")
	fourDaysAgoFile := filepath.Join(logDir, "iarnet-"+fourDaysAgo+".log")
	os.WriteFile(fourDaysAgoFile, []byte("four days ago log"), 0666)

	// 创建5天前的日志文件（应该被删除）
	fiveDaysAgo := now.AddDate(0, 0, -5).Format("20060102")
	fiveDaysAgoFile := filepath.Join(logDir, "iarnet-"+fiveDaysAgo+".log")
	os.WriteFile(fiveDaysAgoFile, []byte("five days ago log"), 0666)

	// 创建一个不符合命名格式的文件（不应该被删除）
	otherFile := filepath.Join(logDir, "other.log")
	os.WriteFile(otherFile, []byte("other file"), 0666)

	// 执行清理（保留3天）
	err := cleanupOldLogs(logDir, 3)
	if err != nil {
		t.Fatalf("Failed to cleanup old logs: %v", err)
	}

	// 验证今天的文件仍然存在
	if _, err := os.Stat(todayFile); os.IsNotExist(err) {
		t.Error("Today's log file should not be deleted")
	}

	// 验证昨天的文件仍然存在
	if _, err := os.Stat(yesterdayFile); os.IsNotExist(err) {
		t.Error("Yesterday's log file should not be deleted")
	}

	// 验证3天前的文件仍然存在
	if _, err := os.Stat(threeDaysAgoFile); os.IsNotExist(err) {
		t.Error("Three days ago log file should not be deleted")
	}

	// 验证4天前的文件已被删除
	if _, err := os.Stat(fourDaysAgoFile); !os.IsNotExist(err) {
		t.Error("Four days ago log file should be deleted")
	}

	// 验证5天前的文件已被删除
	if _, err := os.Stat(fiveDaysAgoFile); !os.IsNotExist(err) {
		t.Error("Five days ago log file should be deleted")
	}

	// 验证其他文件仍然存在（不符合命名格式）
	if _, err := os.Stat(otherFile); os.IsNotExist(err) {
		t.Error("Other file should not be deleted")
	}
}

func TestLogFileCreation(t *testing.T) {
	// 创建临时目录用于测试
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	// 确保目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("Failed to create log directory: %v", err)
	}

	// 测试创建日志文件
	date := time.Now().Format("20060102")
	file, path, err := createLogFile(logDir, date)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	defer file.Close()

	// 验证文件已创建
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Log file should exist at %s", path)
	}

	// 验证文件命名格式
	expectedName := "iarnet-" + date + ".log"
	if filepath.Base(path) != expectedName {
		t.Errorf("Log file name should be %s, got %s", expectedName, filepath.Base(path))
	}

	// 验证可以写入文件
	testContent := "test log content\n"
	_, err = file.WriteString(testContent)
	if err != nil {
		t.Fatalf("Failed to write to log file: %v", err)
	}

	// 验证文件内容
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	if string(content) != testContent {
		t.Errorf("Log file content mismatch, expected %q, got %q", testContent, string(content))
	}
}

func TestFileHookUpdateFile(t *testing.T) {
	// 创建临时目录用于测试
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	// 确保目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("Failed to create log directory: %v", err)
	}

	// 创建第一个文件
	date1 := time.Now().Format("20060102")
	file1, path1, err := createLogFile(logDir, date1)
	if err != nil {
		t.Fatalf("Failed to create first log file: %v", err)
	}

	// 创建 FileHook
	hook := &FileHook{
		file: file1,
	}

	// 写入一些内容到第一个文件
	hook.Fire(&logrus.Entry{
		Message: "test message 1",
		Level:   logrus.InfoLevel,
		Time:    time.Now(),
	})

	// 创建第二个文件
	date2 := time.Now().AddDate(0, 0, 1).Format("20060102")
	file2, path2, err := createLogFile(logDir, date2)
	if err != nil {
		t.Fatalf("Failed to create second log file: %v", err)
	}

	// 更新 hook 的文件句柄
	hook.UpdateFile(file2)

	// 写入一些内容到第二个文件
	hook.Fire(&logrus.Entry{
		Message: "test message 2",
		Level:   logrus.InfoLevel,
		Time:    time.Now(),
	})

	// 验证第一个文件有内容
	content1, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("Failed to read first log file: %v", err)
	}
	if len(content1) == 0 {
		t.Error("First log file should have content")
	}

	// 验证第二个文件有内容
	content2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("Failed to read second log file: %v", err)
	}
	if len(content2) == 0 {
		t.Error("Second log file should have content")
	}

	// 验证两个文件内容不同
	if string(content1) == string(content2) {
		t.Error("Log files should have different content")
	}
}
