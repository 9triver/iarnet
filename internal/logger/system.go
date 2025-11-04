package logger

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

// System 日志系统主接口
type System interface {
	// StartCollectingContainer 开始收集容器日志
	StartCollectingContainer(containerID string, containerType string, labels map[string]string) error

	// StopCollectingContainer 停止收集容器日志
	StopCollectingContainer(containerID string) error

	// QueryApplicationLogs 查询应用日志（runner 容器）
	QueryApplicationLogs(ctx context.Context, appID string, lines int, levelStr string) ([]*LogEntry, error)

	// QueryComponentLogs 查询组件日志
	QueryComponentLogs(ctx context.Context, componentID string, lines int, levelStr string) ([]*LogEntry, error)

	// TailApplicationLogs 实时尾随应用日志
	TailApplicationLogs(ctx context.Context, appID string, lines int) ([]*LogEntry, error)

	// StreamApplicationLogs 实时流式读取应用日志
	StreamApplicationLogs(ctx context.Context, appID string) (<-chan *LogEntry, error)

	// GetStats 获取统计信息
	GetStats(ctx context.Context) (*LogStats, error)

	// Start 启动日志系统
	Start() error

	// Stop 停止日志系统
	Stop() error
}

// system 日志系统实现
type system struct {
	config    *LogSystemConfig
	collector Collector
	processor Processor
	storage   Storage

	// 清理任务
	cleanupTicker *time.Ticker
	cleanupDone   chan struct{}

	mu      sync.RWMutex
	running bool
}

// NewSystem 创建新的日志系统
func NewSystem(dockerClient *client.Client, config *LogSystemConfig) (System, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if !config.Enabled {
		logrus.Info("Log system is disabled")
		return &noopSystem{}, nil
	}

	// 初始化存储
	storage, err := NewStorage(&config.Storage)
	if err != nil {
		return nil, err
	}

	// 初始化处理器
	processor := NewProcessor(&config.Processor)

	// 初始化收集器
	collector := NewCollector(dockerClient, &config.Collector, processor, storage)

	sys := &system{
		config:      config,
		collector:   collector,
		processor:   processor,
		storage:     storage,
		cleanupDone: make(chan struct{}),
	}

	return sys, nil
}

// Start 启动日志系统
func (s *system) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	// 启动清理任务
	if s.config.Storage.CleanupInterval > 0 {
		s.cleanupTicker = time.NewTicker(s.config.Storage.CleanupInterval)
		go s.cleanupLoop()
	}

	s.running = true
	logrus.Info("Log system started")
	return nil
}

// Stop 停止日志系统
func (s *system) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	// 停止清理任务
	if s.cleanupTicker != nil {
		s.cleanupTicker.Stop()
		close(s.cleanupDone)
	}

	// 停止收集器
	if err := s.collector.Stop(); err != nil {
		logrus.Errorf("Failed to stop collector: %v", err)
	}

	// 关闭存储
	if err := s.storage.Close(); err != nil {
		logrus.Errorf("Failed to close storage: %v", err)
	}

	s.running = false
	logrus.Info("Log system stopped")
	return nil
}

// cleanupLoop 清理循环
func (s *system) cleanupLoop() {
	for {
		select {
		case <-s.cleanupTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			if err := s.storage.Cleanup(ctx); err != nil {
				logrus.Errorf("Failed to cleanup old logs: %v", err)
			}
			cancel()

		case <-s.cleanupDone:
			return
		}
	}
}

// StartCollectingContainer 开始收集容器日志
func (s *system) StartCollectingContainer(containerID string, containerType string, labels map[string]string) error {
	// 添加容器类型标签
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["container_type"] = containerType
	labels["container_id"] = containerID

	ctx := context.Background()
	return s.collector.StartCollecting(ctx, containerID, labels)
}

// StopCollectingContainer 停止收集容器日志
func (s *system) StopCollectingContainer(containerID string) error {
	return s.collector.StopCollecting(containerID)
}

// QueryApplicationLogs 查询应用日志（runner 容器）
func (s *system) QueryApplicationLogs(ctx context.Context, appID string, lines int, levelStr string) ([]*LogEntry, error) {
	labels := map[string]string{
		"app_id":         appID,
		"container_type": "runner",
	}

	level := LogLevel(levelStr)
	req := &QueryRequest{
		Labels:    labels,
		Level:     level,
		Limit:     lines,
		Direction: QueryDirectionBackward,
		EndTime:   time.Now(),
		StartTime: time.Now().Add(-24 * time.Hour), // 查询最近24小时
	}

	result, err := s.storage.Query(ctx, req)
	if err != nil {
		return nil, err
	}

	return result.Entries, nil
}

// QueryComponentLogs 查询组件日志
func (s *system) QueryComponentLogs(ctx context.Context, componentID string, lines int, levelStr string) ([]*LogEntry, error) {
	labels := map[string]string{
		"component_id":   componentID,
		"container_type": "component",
	}

	level := LogLevel(levelStr)
	req := &QueryRequest{
		Labels:    labels,
		Level:     level,
		Limit:     lines,
		Direction: QueryDirectionBackward,
		EndTime:   time.Now(),
		StartTime: time.Now().Add(-24 * time.Hour),
	}

	result, err := s.storage.Query(ctx, req)
	if err != nil {
		return nil, err
	}

	return result.Entries, nil
}

// GetStats 获取统计信息
func (s *system) GetStats(ctx context.Context) (*LogStats, error) {
	return s.storage.GetStats(ctx)
}

// TailApplicationLogs 实时尾随应用日志
func (s *system) TailApplicationLogs(ctx context.Context, appID string, lines int) ([]*LogEntry, error) {
	labels := map[string]string{
		"app_id":         appID,
		"container_type": "runner",
	}

	req := &TailRequest{
		Labels: labels,
		Lines:  lines,
		Follow: false,
	}

	return s.storage.Tail(ctx, req)
}

// StreamApplicationLogs 实时流式读取应用日志
func (s *system) StreamApplicationLogs(ctx context.Context, appID string) (<-chan *LogEntry, error) {
	// 需要通过 containerID 来流式读取
	// 这里需要先查询 app 的 containerID，暂时返回错误
	return nil, fmt.Errorf("stream by app_id not yet implemented, use container_id instead")
}

// noopSystem 空操作实现（当日志系统被禁用时）
type noopSystem struct{}

func (n *noopSystem) StartCollectingContainer(containerID string, containerType string, labels map[string]string) error {
	return nil
}
func (n *noopSystem) StopCollectingContainer(containerID string) error { return nil }
func (n *noopSystem) QueryApplicationLogs(ctx context.Context, appID string, lines int, level string) ([]*LogEntry, error) {
	return []*LogEntry{}, nil
}
func (n *noopSystem) QueryComponentLogs(ctx context.Context, componentID string, lines int, level string) ([]*LogEntry, error) {
	return []*LogEntry{}, nil
}
func (n *noopSystem) TailApplicationLogs(ctx context.Context, appID string, lines int) ([]*LogEntry, error) {
	return []*LogEntry{}, nil
}
func (n *noopSystem) StreamApplicationLogs(ctx context.Context, appID string) (<-chan *LogEntry, error) {
	ch := make(chan *LogEntry)
	close(ch)
	return ch, nil
}
func (n *noopSystem) GetStats(ctx context.Context) (*LogStats, error) {
	return &LogStats{}, nil
}
func (n *noopSystem) Start() error { return nil }
func (n *noopSystem) Stop() error  { return nil }
