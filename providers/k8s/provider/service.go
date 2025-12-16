package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned"
)

const providerType = "kubernetes"

// Service Kubernetes provider 服务实现
type Service struct {
	providerpb.UnimplementedServiceServer
	mu            sync.RWMutex
	clientset     *kubernetes.Clientset
	metricsClient *metricsv1beta1.Clientset
	manager       *Manager // 健康检查状态管理器
	resourceTags  *providerpb.ResourceTags
	namespace     string // 部署 Pod 的命名空间
	labelSelector string // 用于筛选管理的 Pod 的标签选择器

	// 资源容量管理（从配置文件读取）
	totalCapacity *resourcepb.Info // 配置的总容量
	allocated     *resourcepb.Info // 当前已分配的容量（内存中动态维护）
}

// NewService 创建新的 Kubernetes provider 服务
func NewService(kubeconfig string, inCluster bool, namespace string, labelSelector string, resourceTags []string, totalCapacity *resourcepb.Info, allowConnectionFailure bool) (*Service, error) {
	var config *rest.Config
	var err error

	// 获取 Kubernetes 配置
	if inCluster {
		// 在 Pod 内运行，使用 in-cluster 配置
		config, err = rest.InClusterConfig()
		if err != nil {
			if allowConnectionFailure {
				logrus.Warnf("Failed to get in-cluster config: %v (continuing in test mode)", err)
				config = nil
			} else {
				return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
			}
		} else {
			logrus.Info("Using in-cluster Kubernetes configuration")
		}
	} else {
		// 在 Pod 外运行，使用 kubeconfig 文件
		if kubeconfig == "" {
			// 尝试使用默认的 kubeconfig 路径
			home, err := os.UserHomeDir()
			if err == nil {
				kubeconfig = filepath.Join(home, ".kube", "config")
			}
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			if allowConnectionFailure {
				logrus.Warnf("Failed to build config from kubeconfig: %v (continuing in test mode)", err)
				config = nil
			} else {
				return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
			}
		} else {
			logrus.Infof("Using kubeconfig: %s", kubeconfig)
		}
	}

	var clientset *kubernetes.Clientset
	var metricsClient *metricsv1beta1.Clientset

	// 创建 Kubernetes 客户端
	if config != nil {
		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			if allowConnectionFailure {
				logrus.Warnf("Failed to create Kubernetes client: %v (continuing in test mode)", err)
				clientset = nil
			} else {
				return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
			}
		}

		// 创建 metrics 客户端（用于获取实时资源使用情况）
		if clientset != nil {
			metricsClient, err = metricsv1beta1.NewForConfig(config)
			if err != nil {
				logrus.Warnf("Failed to create metrics client (metrics-server may not be installed): %v", err)
				// metrics 客户端创建失败不是致命错误，继续运行
			}
		}
	} else if allowConnectionFailure {
		logrus.Warnf("Kubernetes config is nil, provider will start in test mode without Kubernetes connection")
		clientset = nil
		metricsClient = nil
	}

	// 测试连接
	if clientset != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err = clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			if allowConnectionFailure {
				logrus.Warnf("Failed to access namespace %s: %v (continuing in test mode)", namespace, err)
				clientset = nil
				metricsClient = nil
			} else {
				return nil, fmt.Errorf("failed to access namespace %s: %w", namespace, err)
			}
		} else {
			logrus.Infof("Successfully connected to Kubernetes cluster, namespace: %s", namespace)
		}
	} else if allowConnectionFailure {
		logrus.Warnf("Kubernetes client is nil, provider will start in test mode without Kubernetes connection")
	}

	// 创建健康检查管理器
	// 健康检测超时时间：90 秒（假设 iarnet 每 30 秒检测一次，允许 3 次失败）
	// 检查间隔：10 秒
	manager := NewManager(
		90*time.Second,
		10*time.Second,
		func() {
			// 超时回调：清除 provider ID 的逻辑已经在 manager 中处理
			logrus.Debug("Provider ID cleared due to health check timeout")
		},
	)

	// 初始化已分配容量为 0
	allocated := &resourcepb.Info{
		Cpu:    0,
		Memory: 0,
		Gpu:    0,
	}

	service := &Service{
		clientset:     clientset,
		metricsClient: metricsClient,
		manager:       manager,
		namespace:     namespace,
		labelSelector: labelSelector,
		resourceTags: &providerpb.ResourceTags{
			Cpu:    slices.Contains(resourceTags, "cpu"),
			Memory: slices.Contains(resourceTags, "memory"),
			Gpu:    slices.Contains(resourceTags, "gpu"),
			Camera: slices.Contains(resourceTags, "camera"),
		},
		totalCapacity: totalCapacity,
		allocated:     allocated,
	}

	// 启动健康检测超时监控
	manager.Start()

	return service, nil
}

// Close 关闭服务
func (s *Service) Close() error {
	// 停止健康检测监控
	if s.manager != nil {
		s.manager.Stop()
	}
	return nil
}

// Connect 处理控制端连接请求
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

	return &providerpb.ConnectResponse{
		Success: true,
		ProviderType: &providerpb.ProviderType{
			Name: providerType,
		},
	}, nil
}

// GetCapacity 获取资源容量
func (s *Service) GetCapacity(ctx context.Context, req *providerpb.GetCapacityRequest) (*providerpb.GetCapacityResponse, error) {
	// 鉴权：如果 provider 已连接，需要验证 provider_id；如果未连接，允许访问
	if err := s.checkAuth(req.ProviderId, true); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	s.mu.RLock()
	total := s.totalCapacity
	allocated := s.allocated
	s.mu.RUnlock()

	// 必须从配置文件获取容量，如果未配置则返回错误
	if total == nil {
		return nil, fmt.Errorf("resource capacity not configured, please set resource capacity in config file")
	}

	available := &resourcepb.Info{
		Cpu:    total.Cpu - allocated.Cpu,
		Memory: total.Memory - allocated.Memory,
		Gpu:    total.Gpu - allocated.Gpu,
	}

	capacity := &resourcepb.Capacity{
		Total:     total,
		Used:      allocated,
		Available: available,
	}
	logrus.Infof("k8s provider get capacity, capacity: %v", capacity)

	return &providerpb.GetCapacityResponse{
		Capacity: capacity,
	}, nil
}

// GetAvailable 获取可用资源
func (s *Service) GetAvailable(ctx context.Context, req *providerpb.GetAvailableRequest) (*providerpb.GetAvailableResponse, error) {
	// 鉴权：如果 provider 已连接，需要验证 provider_id；如果未连接，允许访问
	if err := s.checkAuth(req.ProviderId, true); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	s.mu.RLock()
	total := s.totalCapacity
	allocated := s.allocated
	s.mu.RUnlock()

	// 必须从配置文件获取容量，如果未配置则返回错误
	if total == nil {
		return nil, fmt.Errorf("resource capacity not configured, please set resource capacity in config file")
	}

	return &providerpb.GetAvailableResponse{
		Available: &resourcepb.Info{
			Cpu:    total.Cpu - allocated.Cpu,
			Memory: total.Memory - allocated.Memory,
			Gpu:    total.Gpu - allocated.Gpu,
		},
	}, nil
}

// GetAllocated 返回当前已分配的资源
func (s *Service) GetAllocated(ctx context.Context) (*resourcepb.Info, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回内存中维护的已分配资源
	return &resourcepb.Info{
		Cpu:    s.allocated.Cpu,
		Memory: s.allocated.Memory,
		Gpu:    s.allocated.Gpu,
	}, nil
}

// GetProviderID 获取当前分配的 provider ID
func (s *Service) GetProviderID() string {
	// 优先从 manager 获取（因为 manager 可能已经清除了 ID）
	if s.manager != nil {
		return s.manager.GetProviderID()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.manager.GetProviderID()
}

// checkAuth 检查鉴权
func (s *Service) checkAuth(requestProviderID string, allowUnconnected bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 如果 provider 还没有被连接
	if s.manager.GetProviderID() == "" {
		if allowUnconnected {
			// 允许未连接的 provider 访问（用于 GetCapacity 和 GetAvailable）
			return nil
		}
		// 其他方法需要先连接
		return fmt.Errorf("provider not connected, please call Connect first")
	}

	// 如果 provider 已经被连接，必须验证 provider_id
	if requestProviderID == "" {
		return fmt.Errorf("provider_id is required for authenticated requests")
	}

	if requestProviderID != s.manager.GetProviderID() {
		return fmt.Errorf("unauthorized: provider_id mismatch, expected %s, got %s", s.manager.GetProviderID(), requestProviderID)
	}

	return nil
}

// Deploy 部署一个 Pod
func (s *Service) Deploy(ctx context.Context, req *providerpb.DeployRequest) (*providerpb.DeployResponse, error) {
	// 鉴权：Deploy 必须验证 provider_id，不允许未连接的 provider 部署
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return &providerpb.DeployResponse{
			Error: fmt.Sprintf("authentication failed: %v", err),
		}, nil
	}

	logrus.WithFields(logrus.Fields{
		"image":            req.Image,
		"env_vars":         req.EnvVars,
		"resource_request": req.ResourceRequest,
		"instance_id":      req.InstanceId,
	}).Info("k8s provider deploy component")

	// 获取 provider ID 用于标记 Pod
	providerID := s.manager.GetProviderID()

	// 构建 Pod 规格
	// 检查 Kubernetes 客户端是否可用
	if s.clientset == nil {
		logrus.Warnf("Kubernetes client is not available (test mode), cannot deploy Pod")
		return &providerpb.DeployResponse{
			Error: "Kubernetes client is not available (test mode)",
		}, nil
	}

	pod := s.buildPodSpec(req, providerID)

	// 创建 Pod
	createdPod, err := s.clientset.CoreV1().Pods(s.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		logrus.Errorf("Failed to create Pod: %v", err)
		return &providerpb.DeployResponse{
			Error: err.Error(),
		}, nil
	}

	// 更新已分配的资源容量（在内存中维护）
	s.mu.Lock()
	s.allocated.Cpu += req.ResourceRequest.Cpu
	s.allocated.Memory += req.ResourceRequest.Memory
	s.allocated.Gpu += req.ResourceRequest.Gpu
	s.mu.Unlock()

	logrus.Infof("Pod deployed successfully: %s/%s, allocated resources: CPU=%d, Memory=%d, GPU=%d",
		s.namespace, createdPod.Name, req.ResourceRequest.Cpu, req.ResourceRequest.Memory, req.ResourceRequest.Gpu)

	return &providerpb.DeployResponse{
		Error: "",
	}, nil
}

// sanitizePodName 将名称转换为符合 RFC 1123 规范的 Kubernetes 资源名称
// RFC 1123 规范：只能包含小写字母、数字、'-' 或 '.'，必须以字母或数字开头和结尾
func sanitizePodName(name string) string {
	// 转换为小写
	name = strings.ToLower(name)

	// 将不允许的字符替换为 '-'
	reg := regexp.MustCompile(`[^a-z0-9\-.]`)
	name = reg.ReplaceAllString(name, "-")

	// 确保以字母或数字开头
	for len(name) > 0 && !isAlphanumeric(name[0]) {
		name = name[1:]
	}

	// 确保以字母或数字结尾
	for len(name) > 0 && !isAlphanumeric(name[len(name)-1]) {
		name = name[:len(name)-1]
	}

	// 限制长度（Kubernetes 资源名称最长 253 字符）
	if len(name) > 253 {
		name = name[:253]
	}

	// 如果名称为空，生成一个默认名称
	if name == "" {
		name = "pod"
	}

	return name
}

// isAlphanumeric 检查字符是否为字母或数字
func isAlphanumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

// buildPodSpec 构建 Pod 规格
func (s *Service) buildPodSpec(req *providerpb.DeployRequest, providerID string) *corev1.Pod {
	// 将 instance_id 转换为符合 RFC 1123 规范的 Pod 名称
	podName := sanitizePodName(req.InstanceId)

	// 构建环境变量
	var envVars []corev1.EnvVar
	for k, v := range req.EnvVars {
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	// 构建资源限制
	// CPU: millicores -> Kubernetes 使用 "m" 后缀表示 millicores
	// Memory: bytes -> Kubernetes 使用整数表示字节
	cpuQuantity := resource.NewMilliQuantity(req.ResourceRequest.Cpu, resource.DecimalSI)
	memoryQuantity := resource.NewQuantity(req.ResourceRequest.Memory, resource.BinarySI)

	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    *cpuQuantity,
			corev1.ResourceMemory: *memoryQuantity,
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    *cpuQuantity,
			corev1.ResourceMemory: *memoryQuantity,
		},
	}

	// 如果请求了 GPU，添加 GPU 资源限制
	if req.ResourceRequest.Gpu > 0 {
		gpuQuantity := resource.NewQuantity(req.ResourceRequest.Gpu, resource.DecimalSI)
		resources.Requests["nvidia.com/gpu"] = *gpuQuantity
		resources.Limits["nvidia.com/gpu"] = *gpuQuantity
	}

	// 构建 Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: s.namespace,
			Labels: map[string]string{
				"iarnet.provider_id": providerID,
				"iarnet.managed":     "true",
				"iarnet.instance_id": req.InstanceId, // 保留原始 instance_id
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "main",
					Image:           req.Image,
					ImagePullPolicy: corev1.PullIfNotPresent, // 优先使用本地镜像
					Env:             envVars,
					Resources:       resources,
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	return pod
}

// HealthCheck 健康检查
func (s *Service) HealthCheck(ctx context.Context, req *providerpb.HealthCheckRequest) (*providerpb.HealthCheckResponse, error) {
	// 鉴权：HealthCheck 必须验证 provider_id，不允许未连接的 provider 健康检查
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// 通过 manager 更新最后收到健康检测的时间
	if s.manager != nil {
		s.manager.UpdateHealthCheck()
	}

	s.mu.RLock()
	total := s.totalCapacity
	allocated := s.allocated
	s.mu.RUnlock()

	// 必须从配置文件获取容量，如果未配置则返回错误
	if total == nil {
		return nil, fmt.Errorf("resource capacity not configured, please set resource capacity in config file")
	}

	available := &resourcepb.Info{
		Cpu:    total.Cpu - allocated.Cpu,
		Memory: total.Memory - allocated.Memory,
		Gpu:    total.Gpu - allocated.Gpu,
	}

	capacity := &resourcepb.Capacity{
		Total:     total,
		Used:      allocated,
		Available: available,
	}

	resourceTags := s.resourceTags

	return &providerpb.HealthCheckResponse{
		Capacity:     capacity,
		ResourceTags: resourceTags,
	}, nil
}

// Disconnect 断开连接
func (s *Service) Disconnect(ctx context.Context, req *providerpb.DisconnectRequest) (*providerpb.DisconnectResponse, error) {
	// 鉴权：Disconnect 必须验证 provider_id，不允许未连接的 provider 断开连接
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 通过 manager 清除 provider ID
	if s.manager != nil {
		s.manager.ClearProviderID()
	}
	logrus.Infof("Provider disconnected: %s", req.ProviderId)

	return &providerpb.DisconnectResponse{}, nil
}

// ReleaseResources 释放已分配的资源（当 Pod 停止或删除时调用）
func (s *Service) ReleaseResources(cpu, memory, gpu int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.allocated.Cpu -= cpu
	if s.allocated.Cpu < 0 {
		s.allocated.Cpu = 0
	}
	s.allocated.Memory -= memory
	if s.allocated.Memory < 0 {
		s.allocated.Memory = 0
	}
	s.allocated.Gpu -= gpu
	if s.allocated.Gpu < 0 {
		s.allocated.Gpu = 0
	}

	logrus.Infof("Released resources: CPU=%d, Memory=%d, GPU=%d, remaining allocated: CPU=%d, Memory=%d, GPU=%d",
		cpu, memory, gpu, s.allocated.Cpu, s.allocated.Memory, s.allocated.Gpu)
}

// GetRealTimeUsage 获取实时资源使用情况
func (s *Service) GetRealTimeUsage(ctx context.Context, req *providerpb.GetRealTimeUsageRequest) (*providerpb.GetRealTimeUsageResponse, error) {
	// 鉴权：必须验证 provider_id
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	providerID := s.manager.GetProviderID()
	if providerID == "" {
		return nil, fmt.Errorf("provider not connected")
	}

	// 如果 metrics 客户端不可用，返回 0
	if s.metricsClient == nil {
		logrus.Warn("Metrics client not available, returning zero usage")
		return &providerpb.GetRealTimeUsageResponse{
			Usage: &resourcepb.Info{
				Cpu:    0,
				Memory: 0,
				Gpu:    0,
			},
		}, nil
	}

	// 获取该 provider 管理的所有 Pod 的 metrics
	labelSelector := fmt.Sprintf("iarnet.provider_id=%s,iarnet.managed=true", providerID)

	podMetricsList, err := s.metricsClient.MetricsV1beta1().PodMetricses(s.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		logrus.Warnf("Failed to get pod metrics: %v", err)
		// 如果获取 metrics 失败，尝试从 Pod 状态获取分配的资源
		return s.getUsageFromPodSpec(ctx, providerID)
	}

	var totalCpu int64    // millicores
	var totalMemory int64 // bytes
	var totalGpu int64    // GPU 数量

	for _, podMetrics := range podMetricsList.Items {
		for _, container := range podMetrics.Containers {
			// CPU 使用量（millicores）
			cpuUsage := container.Usage.Cpu()
			if cpuUsage != nil {
				totalCpu += cpuUsage.MilliValue()
			}

			// 内存使用量（bytes）
			memoryUsage := container.Usage.Memory()
			if memoryUsage != nil {
				totalMemory += memoryUsage.Value()
			}
		}
	}

	// GPU 使用量需要通过其他方式获取（如 DCGM exporter 或 nvidia-smi）
	// 这里暂时通过查询 Pod 的 GPU 请求来估算
	totalGpu = s.getGPUUsageFromPods(ctx, providerID)

	return &providerpb.GetRealTimeUsageResponse{
		Usage: &resourcepb.Info{
			Cpu:    totalCpu,
			Memory: totalMemory,
			Gpu:    totalGpu,
		},
	}, nil
}

// getUsageFromPodSpec 从 Pod 规格获取分配的资源（作为 metrics 不可用时的备选方案）
func (s *Service) getUsageFromPodSpec(ctx context.Context, providerID string) (*providerpb.GetRealTimeUsageResponse, error) {
	// 检查 Kubernetes 客户端是否可用
	if s.clientset == nil {
		logrus.Warnf("Kubernetes client is not available (test mode), returning zero usage")
		return &providerpb.GetRealTimeUsageResponse{
			Usage: &resourcepb.Info{
				Cpu:    0,
				Memory: 0,
				Gpu:    0,
			},
		}, nil
	}

	labelSelector := fmt.Sprintf("iarnet.provider_id=%s,iarnet.managed=true", providerID)

	pods, err := s.clientset.CoreV1().Pods(s.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var totalCpu int64
	var totalMemory int64
	var totalGpu int64

	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			// CPU 请求（millicores）
			if cpuReq, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				totalCpu += cpuReq.MilliValue()
			}
			// 内存请求（bytes）
			if memReq, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
				totalMemory += memReq.Value()
			}
			// GPU 请求
			if gpuReq, ok := container.Resources.Requests["nvidia.com/gpu"]; ok {
				totalGpu += gpuReq.Value()
			}
		}
	}

	return &providerpb.GetRealTimeUsageResponse{
		Usage: &resourcepb.Info{
			Cpu:    totalCpu,
			Memory: totalMemory,
			Gpu:    totalGpu,
		},
	}, nil
}

// getGPUUsageFromPods 从 Pod 获取 GPU 使用量
func (s *Service) getGPUUsageFromPods(ctx context.Context, providerID string) int64 {
	// 检查 Kubernetes 客户端是否可用
	if s.clientset == nil {
		return 0
	}

	labelSelector := fmt.Sprintf("iarnet.provider_id=%s,iarnet.managed=true", providerID)

	pods, err := s.clientset.CoreV1().Pods(s.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		logrus.Warnf("Failed to list pods for GPU usage: %v", err)
		return 0
	}

	var totalGpu int64
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			if gpuReq, ok := container.Resources.Requests["nvidia.com/gpu"]; ok {
				totalGpu += gpuReq.Value()
			}
		}
	}

	return totalGpu
}
