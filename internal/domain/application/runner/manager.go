package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/domain/application/types"
	"github.com/sirupsen/logrus"
)

// Manager 运行器管理器
// 负责在运行时持有内存中的运行器领域对象，并持续监听容器状态
type Manager struct {
	mu            sync.RWMutex
	runners       map[string]*Runner // appID -> Runner
	ctx           context.Context
	cancel        context.CancelFunc
	watchCtx      map[string]context.CancelFunc // appID -> cancel function for watch goroutine
	watchCtxMu    sync.RWMutex
	watchInterval time.Duration // 监听间隔
}

// NewManager 创建运行器管理器
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		mu:            sync.RWMutex{},
		runners:       make(map[string]*Runner),
		ctx:           ctx,
		cancel:        cancel,
		watchCtx:      make(map[string]context.CancelFunc),
		watchCtxMu:    sync.RWMutex{},
		watchInterval: 5 * time.Second, // 默认每 5 秒检查一次
	}
}

// Add 添加运行器到管理器，并启动状态监听
func (m *Manager) Add(appID string, runner *Runner) {
	m.mu.Lock()
	m.runners[appID] = runner
	m.mu.Unlock()

	// 启动状态监听
	m.startWatch(appID, runner)
}

// startWatch 启动对指定运行器的状态监听
func (m *Manager) startWatch(appID string, runner *Runner) {
	// 为每个 runner 创建独立的 context
	watchCtx, cancel := context.WithCancel(m.ctx)

	m.watchCtxMu.Lock()
	// 如果已存在监听，先取消旧的
	if oldCancel, exists := m.watchCtx[appID]; exists {
		oldCancel()
	}
	m.watchCtx[appID] = cancel
	m.watchCtxMu.Unlock()

	// 启动监听 goroutine
	go m.watchRunner(watchCtx, appID, runner)
}

// watchRunner 持续监听运行器状态
func (m *Manager) watchRunner(ctx context.Context, appID string, runner *Runner) {
	ticker := time.NewTicker(m.watchInterval)
	defer ticker.Stop()

	logrus.Debugf("Started watching runner for app %s", appID)

	for {
		select {
		case <-ctx.Done():
			logrus.Debugf("Stopped watching runner for app %s", appID)
			return
		case <-ticker.C:
			// 检查容器状态并更新 runner 状态
			m.updateRunnerStatus(ctx, appID, runner)
		}
	}
}

// updateRunnerStatus 检查容器状态并更新 runner 状态
func (m *Manager) updateRunnerStatus(ctx context.Context, appID string, runner *Runner) {
	// 如果容器 ID 为空，说明容器还未创建
	if runner.containerID == "" {
		return
	}

	// 检查容器状态
	inspect, err := runner.dockerClient.ContainerInspect(ctx, runner.containerID)
	if err != nil {
		// 容器可能已被删除
		logrus.Warnf("Failed to inspect container %s for app %s: %v", runner.containerID, appID, err)
		m.updateStatus(runner, types.RunnerStatusFailed)
		return
	}

	// 根据容器实际状态更新 runner 状态
	// 使用线程安全的方法获取当前状态
	currentStatus, _ := runner.GetStatus(ctx)

	// 如果容器未运行
	if !inspect.State.Running {
		if inspect.State.Status == "exited" {
			// 容器已退出
			if currentStatus != types.RunnerStatusStopped && currentStatus != types.RunnerStatusStopping {
				m.updateStatus(runner, types.RunnerStatusStopped)
				logrus.Infof("Container %s for app %s has exited", runner.containerID, appID)
			}
		} else {
			// 其他状态（如 created, removing 等）
			if currentStatus != types.RunnerStatusStopped {
				m.updateStatus(runner, types.RunnerStatusStopped)
			}
		}
		return
	}

	// 容器正在运行
	if inspect.State.Status == "running" {
		// 检查健康状态
		if inspect.State.Health != nil {
			switch inspect.State.Health.Status {
			case "healthy":
				if currentStatus != types.RunnerStatusRunning {
					m.updateStatus(runner, types.RunnerStatusRunning)
					logrus.Debugf("Container %s for app %s is healthy", runner.containerID, appID)
				}
			case "unhealthy":
				if currentStatus != types.RunnerStatusFailed {
					m.updateStatus(runner, types.RunnerStatusFailed)
					logrus.Warnf("Container %s for app %s is unhealthy", runner.containerID, appID)
				}
			case "starting":
				// 健康检查还在进行中，保持 starting 状态
				if currentStatus != types.RunnerStatusStarting {
					m.updateStatus(runner, types.RunnerStatusStarting)
				}
			}
		} else {
			// 没有健康检查，只要容器在运行就认为正常
			if currentStatus != types.RunnerStatusRunning && currentStatus != types.RunnerStatusStarting {
				m.updateStatus(runner, types.RunnerStatusRunning)
				logrus.Debugf("Container %s for app %s is running (no health check)", runner.containerID, appID)
			}
		}
	}
}

// updateStatus 更新运行器状态（线程安全）
func (m *Manager) updateStatus(runner *Runner, newStatus types.RunnerStatus) {
	// 使用 Runner 的线程安全方法更新状态
	runner.setStatus(newStatus)
}

// Get 获取运行器，如果不存在则返回错误
func (m *Manager) Get(appID string) (*Runner, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runner, ok := m.runners[appID]
	if !ok {
		return nil, fmt.Errorf("runner not found for app %s", appID)
	}

	return runner, nil
}

// Remove 移除运行器并停止监听
func (m *Manager) Remove(appID string) {
	// 停止监听
	m.watchCtxMu.Lock()
	if cancel, exists := m.watchCtx[appID]; exists {
		cancel()
		delete(m.watchCtx, appID)
	}
	m.watchCtxMu.Unlock()

	// 移除运行器
	m.mu.Lock()
	delete(m.runners, appID)
	m.mu.Unlock()

	logrus.Debugf("Removed runner and stopped watching for app %s", appID)
}

// Stop 停止所有监听并清理资源
func (m *Manager) Stop() {
	// 取消所有监听
	m.cancel()

	// 清理所有监听 context
	m.watchCtxMu.Lock()
	for appID, cancel := range m.watchCtx {
		cancel()
		delete(m.watchCtx, appID)
	}
	m.watchCtxMu.Unlock()

	logrus.Info("Runner manager stopped")
}

// SetWatchInterval 设置监听间隔
func (m *Manager) SetWatchInterval(interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watchInterval = interval
}
