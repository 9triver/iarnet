package logger

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

// Collector 日志收集器接口
type Collector interface {
	// StartCollecting 开始收集容器日志
	StartCollecting(ctx context.Context, containerID string, labels map[string]string) error

	// StopCollecting 停止收集容器日志
	StopCollecting(containerID string) error

	// StreamLogs 实时流式读取日志
	StreamLogs(ctx context.Context, containerID string) (<-chan *LogEntry, error)

	// IsCollecting 检查是否正在收集
	IsCollecting(containerID string) bool

	// Stop 停止所有收集器
	Stop() error
}

// collector 收集器实现
type collector struct {
	dockerClient *client.Client
	config       *CollectorConfig
	processor    Processor
	storage      Storage

	// 活动的收集任务
	collectors map[string]*containerCollector
	mu         sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// containerCollector 单个容器的收集器
type containerCollector struct {
	containerID string
	labels      map[string]string
	cancel      context.CancelFunc
	active      bool
}

// NewCollector 创建新的收集器
func NewCollector(dockerClient *client.Client, config *CollectorConfig, processor Processor, storage Storage) Collector {
	if config == nil {
		config = &DefaultConfig().Collector
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &collector{
		dockerClient: dockerClient,
		config:       config,
		processor:    processor,
		storage:      storage,
		collectors:   make(map[string]*containerCollector),
		ctx:          ctx,
		cancel:       cancel,
	}

	return c
}

// StartCollecting 开始收集容器日志
func (c *collector) StartCollecting(ctx context.Context, containerID string, labels map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否已经在收集
	if _, exists := c.collectors[containerID]; exists {
		return fmt.Errorf("already collecting logs for container %s", containerID)
	}

	// 创建收集器上下文
	collectorCtx, cancel := context.WithCancel(c.ctx)

	cc := &containerCollector{
		containerID: containerID,
		labels:      labels,
		cancel:      cancel,
		active:      true,
	}

	c.collectors[containerID] = cc

	// 启动收集协程
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer func() {
			c.mu.Lock()
			if cc, exists := c.collectors[containerID]; exists {
				cc.active = false
			}
			c.mu.Unlock()
		}()

		if err := c.collectContainerLogs(collectorCtx, cc); err != nil {
			logrus.Errorf("Error collecting logs for container %s: %v", containerID, err)
		}
	}()

	logrus.Infof("Started log collection for container %s", containerID)
	return nil
}

// StopCollecting 停止收集容器日志
func (c *collector) StopCollecting(containerID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cc, exists := c.collectors[containerID]
	if !exists {
		return fmt.Errorf("not collecting logs for container %s", containerID)
	}

	cc.cancel()
	delete(c.collectors, containerID)

	logrus.Infof("Stopped log collection for container %s", containerID)
	return nil
}

// collectContainerLogs 收集容器日志的核心逻辑
func (c *collector) collectContainerLogs(ctx context.Context, cc *containerCollector) error {
	// 获取容器日志流
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Since:      time.Now().Add(-1 * time.Minute).Format(time.RFC3339), // 获取最近1分钟的日志
	}

	reader, err := c.dockerClient.ContainerLogs(ctx, cc.containerID, options)
	if err != nil {
		return fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	// 缓冲区
	buffer := make([]*LogEntry, 0, c.config.BatchSize)
	flushTicker := time.NewTicker(c.config.FlushInterval)
	defer flushTicker.Stop()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, c.config.MaxLineSize), c.config.MaxLineSize)

	for {
		select {
		case <-ctx.Done():
			// 刷新剩余的日志
			if len(buffer) > 0 {
				c.flushBuffer(buffer, cc.labels)
			}
			return nil

		case <-flushTicker.C:
			// 定时刷新
			if len(buffer) > 0 {
				c.flushBuffer(buffer, cc.labels)
				buffer = make([]*LogEntry, 0, c.config.BatchSize)
			}

		default:
			if !scanner.Scan() {
				// 读取结束或错误
				if err := scanner.Err(); err != nil && err != io.EOF {
					logrus.Errorf("Error reading container logs: %v", err)
				}
				// 刷新剩余的日志
				if len(buffer) > 0 {
					c.flushBuffer(buffer, cc.labels)
				}
				return nil
			}

			line := scanner.Text()
			if line == "" {
				continue
			}

			// 解析日志行
			entry := c.parseDockerLogLine(line, cc.containerID, cc.labels)
			if entry != nil {
				buffer = append(buffer, entry)

				// 检查是否达到批量大小
				if len(buffer) >= c.config.BatchSize {
					c.flushBuffer(buffer, cc.labels)
					buffer = make([]*LogEntry, 0, c.config.BatchSize)
				}
			}
		}
	}
}

// parseDockerLogLine 解析 Docker 日志行
func (c *collector) parseDockerLogLine(line string, containerID string, labels map[string]string) *LogEntry {
	// Docker 日志格式：8字节头部 + 日志内容
	// 头部：[stdout/stderr: 1字节][padding: 3字节][size: 4字节]

	var source LogSource = LogSourceStdout
	var content string

	// 尝试解析 Docker 日志头部
	if len(line) > 8 {
		streamType := line[0]
		if streamType == 1 {
			source = LogSourceStdout
		} else if streamType == 2 {
			source = LogSourceStderr
		}
		content = line[8:]
	} else {
		content = line
	}

	entry := &LogEntry{
		Timestamp:   time.Now(),
		ContainerID: containerID,
		Message:     content,
		Raw:         content,
		Source:      source,
		Level:       LogLevelUnknown,
		Labels:      make(map[string]string),
	}

	// 复制标签
	for k, v := range labels {
		entry.Labels[k] = v
	}

	// 添加容器标签
	entry.Labels["container_id"] = containerID
	entry.Labels["source"] = string(source)

	return entry
}

// flushBuffer 刷新缓冲区
func (c *collector) flushBuffer(buffer []*LogEntry, labels map[string]string) {
	if len(buffer) == 0 {
		return
	}

	// 处理日志
	processed, err := c.processor.ProcessBatch(buffer)
	if err != nil {
		logrus.Errorf("Failed to process log batch: %v", err)
		return
	}

	// 转换回 LogEntry
	entries := make([]*LogEntry, len(processed))
	for i, p := range processed {
		entries[i] = p.LogEntry
	}

	// 写入存储
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.storage.Write(ctx, entries); err != nil {
		logrus.Errorf("Failed to write logs to storage: %v", err)
	}
}

// StreamLogs 实时流式读取日志
func (c *collector) StreamLogs(ctx context.Context, containerID string) (<-chan *LogEntry, error) {
	ch := make(chan *LogEntry, c.config.BufferSize)

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Tail:       "100",
	}

	reader, err := c.dockerClient.ContainerLogs(ctx, containerID, options)
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}

	// 启动读取协程
	go func() {
		defer close(ch)
		defer reader.Close()

		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, c.config.MaxLineSize), c.config.MaxLineSize)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				line := scanner.Text()
				if line == "" {
					continue
				}

				entry := c.parseDockerLogLine(line, containerID, nil)
				if entry != nil {
					// 处理日志
					processed, err := c.processor.Process(entry)
					if err == nil {
						select {
						case ch <- processed.LogEntry:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}()

	return ch, nil
}

// IsCollecting 检查是否正在收集
func (c *collector) IsCollecting(containerID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cc, exists := c.collectors[containerID]
	return exists && cc.active
}

// Stop 停止所有收集器
func (c *collector) Stop() error {
	c.cancel()
	c.wg.Wait()

	c.mu.Lock()
	c.collectors = make(map[string]*containerCollector)
	c.mu.Unlock()

	logrus.Info("Log collector stopped")
	return nil
}
