package provider

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	commonpb "github.com/9triver/iarnet/internal/proto/common"
	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/sirupsen/logrus"
)

const providerType = "mock"

// TaskType 任务类型
type TaskType string

const (
	TaskTypeSmall  TaskType = "small"
	TaskTypeMedium TaskType = "medium"
	TaskTypeLarge  TaskType = "large"
)

// TaskDurationConfig 任务执行时间配置
type TaskDurationConfig struct {
	SmallMinMs  int `yaml:"small_min_ms" json:"small_min_ms"`
	SmallMaxMs  int `yaml:"small_max_ms" json:"small_max_ms"`
	MediumMinMs int `yaml:"medium_min_ms" json:"medium_min_ms"`
	MediumMaxMs int `yaml:"medium_max_ms" json:"medium_max_ms"`
	LargeMinMs  int `yaml:"large_min_ms" json:"large_min_ms"`
	LargeMaxMs  int `yaml:"large_max_ms" json:"large_max_ms"`
}

// taskInfo 存储已部署任务的信息
type taskInfo struct {
	cancel          context.CancelFunc
	resourceRequest *resourcepb.Info
}

// DefaultTaskDurationConfig 返回默认的任务执行时间配置
func DefaultTaskDurationConfig() TaskDurationConfig {
	return TaskDurationConfig{
		SmallMinMs:  50,
		SmallMaxMs:  200,
		MediumMinMs: 200,
		MediumMaxMs: 800,
		LargeMinMs:  800,
		LargeMaxMs:  2000,
	}
}

// Service Mock provider 服务实现
type Service struct {
	providerpb.UnimplementedServiceServer
	mu           sync.RWMutex
	manager      *Manager
	resourceTags *providerpb.ResourceTags

	// 资源容量管理（从配置文件读取）
	totalCapacity *resourcepb.Info // 配置的总容量
	allocated     *resourcepb.Info // 当前已分配的容量（内存中动态维护）

	// 任务执行时间配置
	durationConfig TaskDurationConfig

	// 已部署的任务映射：instanceID -> taskInfo
	// 用于在 Undeploy 时取消自动释放资源的 goroutine 并释放资源
	deployedTasks map[string]*taskInfo
	tasksMu       sync.RWMutex

	// 随机数生成器
	rand *rand.Rand
}

// NewService 创建 Mock provider 服务
func NewService(resourceTags []string, totalCapacity *resourcepb.Info, durationConfig *TaskDurationConfig) (*Service, error) {
	// 创建健康检查管理器
	manager := NewManager(
		90*time.Second,
		10*time.Second,
		func() {
			logrus.Debug("Provider ID cleared due to health check timeout")
		},
	)

	// 初始化已分配容量为 0
	allocated := &resourcepb.Info{
		Cpu:    0,
		Memory: 0,
		Gpu:    0,
	}

	// 使用默认配置（如果未提供）
	config := DefaultTaskDurationConfig()
	if durationConfig != nil {
		config = *durationConfig
	}

	service := &Service{
		manager: manager,
		resourceTags: &providerpb.ResourceTags{
			Cpu:    contains(resourceTags, "cpu"),
			Memory: contains(resourceTags, "memory"),
			Gpu:    contains(resourceTags, "gpu"),
			Camera: contains(resourceTags, "camera"),
		},
		totalCapacity:  totalCapacity,
		allocated:      allocated,
		durationConfig: config,
		deployedTasks:  make(map[string]*taskInfo),
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// 启动健康检测超时监控
	manager.Start()

	return service, nil
}

// contains 检查字符串切片中是否包含指定字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Close 关闭服务
func (s *Service) Close() error {
	if s.manager != nil {
		s.manager.Stop()
	}
	return nil
}

// checkAuth 检查认证
func (s *Service) checkAuth(requestProviderID string, allowEmpty bool) error {
	if s.manager == nil {
		return fmt.Errorf("provider not initialized")
	}

	// 检查是否已连接（provider ID 不为空）
	hasID := s.manager.GetProviderID() != ""
	if !hasID {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("provider not connected, please call Connect first")
	}

	if requestProviderID == "" {
		return fmt.Errorf("provider_id is required for authenticated requests")
	}

	if requestProviderID != s.manager.GetProviderID() {
		return fmt.Errorf("unauthorized: provider_id mismatch, expected %s, got %s", s.manager.GetProviderID(), requestProviderID)
	}

	return nil
}

// classifyTaskType 根据资源需求判断任务类型
func (s *Service) classifyTaskType(req *resourcepb.Info) TaskType {
	// 大任务：CPU >= 4000mC 或 Memory >= 4GB 或 GPU >= 1
	const largeCPUThreshold = 4000
	const largeMemoryThreshold = 4 * 1024 * 1024 * 1024 // 4GB
	const largeGPUThreshold = int64(1)

	if req.Cpu >= largeCPUThreshold || req.Memory >= largeMemoryThreshold || req.Gpu >= largeGPUThreshold {
		return TaskTypeLarge
	}

	// 中任务：CPU >= 2000mC 或 Memory >= 2GB
	const mediumCPUThreshold = 2000
	const mediumMemoryThreshold = 2 * 1024 * 1024 * 1024 // 2GB

	if req.Cpu >= mediumCPUThreshold || req.Memory >= mediumMemoryThreshold {
		return TaskTypeMedium
	}

	// 小任务：其他情况
	return TaskTypeSmall
}

// getTaskDuration 根据任务类型和配置获取随机执行时间
func (s *Service) getTaskDuration(taskType TaskType) time.Duration {
	var minMs, maxMs int

	switch taskType {
	case TaskTypeSmall:
		minMs, maxMs = s.durationConfig.SmallMinMs, s.durationConfig.SmallMaxMs
	case TaskTypeMedium:
		minMs, maxMs = s.durationConfig.MediumMinMs, s.durationConfig.MediumMaxMs
	case TaskTypeLarge:
		minMs, maxMs = s.durationConfig.LargeMinMs, s.durationConfig.LargeMaxMs
	default:
		minMs, maxMs = 100, 200
	}

	if maxMs <= minMs {
		return time.Duration(minMs) * time.Millisecond
	}

	// 生成 [minMs, maxMs] 区间的随机值
	delta := maxMs - minMs
	randomMs := minMs + s.rand.Intn(delta+1)
	return time.Duration(randomMs) * time.Millisecond
}

// Deploy 部署组件到 mock provider
func (s *Service) Deploy(ctx context.Context, req *providerpb.DeployRequest) (*providerpb.DeployResponse, error) {
	// 鉴权
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return &providerpb.DeployResponse{
			Error: fmt.Sprintf("authentication failed: %v", err),
		}, nil
	}

	if req.ResourceRequest == nil {
		return &providerpb.DeployResponse{
			Error: "resource request is required",
		}, nil
	}

	// 检查资源是否足够
	s.mu.RLock()
	availableCPU := s.totalCapacity.Cpu - s.allocated.Cpu
	availableMemory := s.totalCapacity.Memory - s.allocated.Memory
	availableGPU := s.totalCapacity.Gpu - s.allocated.Gpu
	s.mu.RUnlock()

	if req.ResourceRequest.Cpu > availableCPU ||
		req.ResourceRequest.Memory > availableMemory ||
		req.ResourceRequest.Gpu > availableGPU {
		return &providerpb.DeployResponse{
			Error: fmt.Sprintf("insufficient resources: requested CPU=%d Memory=%d GPU=%d, available CPU=%d Memory=%d GPU=%d",
				req.ResourceRequest.Cpu, req.ResourceRequest.Memory, req.ResourceRequest.Gpu,
				availableCPU, availableMemory, availableGPU),
		}, nil
	}

	// 判断任务类型
	taskType := s.classifyTaskType(req.ResourceRequest)
	logrus.WithFields(logrus.Fields{
		"instance_id": req.InstanceId,
		"task_type":   taskType,
		"cpu":         req.ResourceRequest.Cpu,
		"memory":      req.ResourceRequest.Memory,
		"gpu":         req.ResourceRequest.Gpu,
		"language":    req.Language,
	}).Info("mock provider deploy component")

	// 更新已分配的资源容量
	s.mu.Lock()
	s.allocated.Cpu += req.ResourceRequest.Cpu
	s.allocated.Memory += req.ResourceRequest.Memory
	s.allocated.Gpu += req.ResourceRequest.Gpu
	s.mu.Unlock()

	// 获取任务执行时间
	duration := s.getTaskDuration(taskType)

	// 创建 context 用于取消自动释放
	taskCtx, cancel := context.WithCancel(context.Background())

	// 保存资源请求信息的副本
	resourceReqCopy := &resourcepb.Info{
		Cpu:    req.ResourceRequest.Cpu,
		Memory: req.ResourceRequest.Memory,
		Gpu:    req.ResourceRequest.Gpu,
	}

	// 记录已部署的任务
	taskInfo := &taskInfo{
		cancel:          cancel,
		resourceRequest: resourceReqCopy,
	}
	s.tasksMu.Lock()
	s.deployedTasks[req.InstanceId] = taskInfo
	s.tasksMu.Unlock()

	// 创建 goroutine 等待时间后自动释放资源
	go func() {
		select {
		case <-taskCtx.Done():
			// 任务被手动取消（通过 Undeploy）
			logrus.Debugf("Task %s auto-release cancelled", req.InstanceId)
			return
		case <-time.After(duration):
			// 时间到达，自动释放资源
			logrus.Infof("Task %s completed after %v, auto-releasing resources", req.InstanceId, duration)

			// 释放资源
			s.mu.Lock()
			s.allocated.Cpu -= resourceReqCopy.Cpu
			s.allocated.Memory -= resourceReqCopy.Memory
			s.allocated.Gpu -= resourceReqCopy.Gpu
			s.mu.Unlock()

			// 从已部署任务列表中移除
			s.tasksMu.Lock()
			delete(s.deployedTasks, req.InstanceId)
			s.tasksMu.Unlock()

			logrus.Debugf("Resources released for task %s: CPU=%d Memory=%d GPU=%d",
				req.InstanceId, resourceReqCopy.Cpu, resourceReqCopy.Memory, resourceReqCopy.Gpu)
		}
	}()

	return &providerpb.DeployResponse{
		Error: "",
	}, nil
}

// Undeploy 从 mock provider 移除组件
func (s *Service) Undeploy(ctx context.Context, req *providerpb.UndeployRequest) (*providerpb.UndeployResponse, error) {
	// 鉴权
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return &providerpb.UndeployResponse{
			Error: fmt.Sprintf("authentication failed: %v", err),
		}, nil
	}

	logrus.WithFields(logrus.Fields{
		"instance_id": req.InstanceId,
	}).Info("mock provider undeploy component")

	// 查找并取消自动释放的 goroutine
	s.tasksMu.Lock()
	taskInfo, exists := s.deployedTasks[req.InstanceId]
	if exists {
		delete(s.deployedTasks, req.InstanceId)
	}
	s.tasksMu.Unlock()

	if exists {
		// 取消自动释放的 goroutine
		taskInfo.cancel()

		// 立即释放资源
		s.mu.Lock()
		s.allocated.Cpu -= taskInfo.resourceRequest.Cpu
		s.allocated.Memory -= taskInfo.resourceRequest.Memory
		s.allocated.Gpu -= taskInfo.resourceRequest.Gpu
		s.mu.Unlock()

		logrus.Debugf("Task %s undeployed, resources released: CPU=%d Memory=%d GPU=%d",
			req.InstanceId, taskInfo.resourceRequest.Cpu, taskInfo.resourceRequest.Memory, taskInfo.resourceRequest.Gpu)
	} else {
		logrus.Warnf("Task %s not found in deployed tasks", req.InstanceId)
	}

	return &providerpb.UndeployResponse{
		Error: "",
	}, nil
}

// GetCapacity 获取资源容量
func (s *Service) GetCapacity(ctx context.Context, req *providerpb.GetCapacityRequest) (*providerpb.GetCapacityResponse, error) {
	// 鉴权
	if err := s.checkAuth(req.ProviderId, true); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 计算可用资源
	available := &resourcepb.Info{
		Cpu:    s.totalCapacity.Cpu - s.allocated.Cpu,
		Memory: s.totalCapacity.Memory - s.allocated.Memory,
		Gpu:    s.totalCapacity.Gpu - s.allocated.Gpu,
	}

	capacity := &resourcepb.Capacity{
		Total:     s.totalCapacity,
		Used:      s.allocated,
		Available: available,
	}

	return &providerpb.GetCapacityResponse{
		Capacity: capacity,
	}, nil
}

// HealthCheck 健康检查
func (s *Service) HealthCheck(ctx context.Context, req *providerpb.HealthCheckRequest) (*providerpb.HealthCheckResponse, error) {
	if err := s.checkAuth(req.ProviderId, true); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// 更新健康检查时间（必须在返回前调用，确保每次健康检查都被记录）
	if s.manager != nil {
		s.manager.UpdateHealthCheck()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 计算可用资源
	available := &resourcepb.Info{
		Cpu:    s.totalCapacity.Cpu - s.allocated.Cpu,
		Memory: s.totalCapacity.Memory - s.allocated.Memory,
		Gpu:    s.totalCapacity.Gpu - s.allocated.Gpu,
	}

	capacity := &resourcepb.Capacity{
		Total:     s.totalCapacity,
		Used:      s.allocated,
		Available: available,
	}

	// Mock provider 支持所有语言
	supportedLanguages := []commonpb.Language{
		commonpb.Language_LANG_PYTHON,
		commonpb.Language_LANG_GO,
		commonpb.Language_LANG_UNIKERNEL,
	}

	return &providerpb.HealthCheckResponse{
		Capacity:           capacity,
		ResourceTags:       s.resourceTags,
		SupportedLanguages: supportedLanguages,
	}, nil
}

// Connect 连接 provider（设置 Provider ID）
func (s *Service) Connect(ctx context.Context, req *providerpb.ConnectRequest) (*providerpb.ConnectResponse, error) {
	if req == nil {
		return &providerpb.ConnectResponse{
			Success: false,
			Error:   "request is nil",
		}, nil
	}

	if req.ProviderId == "" {
		return &providerpb.ConnectResponse{
			Success: false,
			Error:   "provider ID is required",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.manager.GetProviderID() != "" && s.manager.GetProviderID() != req.ProviderId {
		logrus.Errorf("provider already connected: %s", s.manager.GetProviderID())
		return &providerpb.ConnectResponse{
			Success: false,
			Error:   fmt.Sprintf("provider already connected: %s", s.manager.GetProviderID()),
		}, nil
	}

	// 通过 manager 设置 provider ID（会同时记录健康检测时间）
	if s.manager != nil {
		s.manager.SetProviderID(req.ProviderId)
	}
	logrus.Infof("Provider ID assigned: %s", s.manager.GetProviderID())

	// Mock provider 支持所有语言
	supportedLanguages := []commonpb.Language{
		commonpb.Language_LANG_PYTHON,
		commonpb.Language_LANG_GO,
		commonpb.Language_LANG_UNIKERNEL,
	}

	return &providerpb.ConnectResponse{
		Success: true,
		ProviderType: &providerpb.ProviderType{
			Name: providerType,
		},
		SupportedLanguages: supportedLanguages,
	}, nil
}

// GetRealTimeUsage 获取实时资源使用情况
func (s *Service) GetRealTimeUsage(ctx context.Context, req *providerpb.GetRealTimeUsageRequest) (*providerpb.GetRealTimeUsageResponse, error) {
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return &providerpb.GetRealTimeUsageResponse{
		Usage: s.allocated,
	}, nil
}
