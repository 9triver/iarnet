package resource

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sConfig holds configuration for Kubernetes connection
type K8sConfig struct {
	// KubeConfigContent is the content of kubeconfig file
	// If empty, uses in-cluster config
	KubeConfigContent string

	// Namespace specifies the Kubernetes namespace to operate in
	// If empty, uses "default" namespace
	Namespace string

	// Context specifies the kubeconfig context to use
	Context string

	// Host is the Kubernetes API server host
	Host string

	// Port is the Kubernetes API server port
	Port int
}

// K8sProvider implements Provider interface for Kubernetes
type K8sProvider struct {
	clientset      *kubernetes.Clientset
	providerID     string
	config         K8sConfig
	lastUpdateTime time.Time
	status         Status
	name           string
	namespace      string
	host           string
	port           int
}

// NewK8sProvider creates a new Kubernetes resource provider
func NewK8sProvider(providerID string, name string, config interface{}) (*K8sProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("empty config type for K8s provider")
	}

	// apply config if specified
	k8sConfig, ok := config.(K8sConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for K8s provider")
	}

	var kubeConfig *rest.Config
	var err error

	if k8sConfig.KubeConfigContent != "" {
		// Use kubeconfig content
		configOverrides := &clientcmd.ConfigOverrides{}
		if k8sConfig.Context != "" {
			configOverrides.CurrentContext = k8sConfig.Context
		}

		// Parse kubeconfig content
		config, err := clientcmd.Load([]byte(k8sConfig.KubeConfigContent))
		if err != nil {
			return nil, fmt.Errorf("failed to parse kubeconfig content: %w", err)
		}

		clientConfig := clientcmd.NewDefaultClientConfig(*config, configOverrides)

		kubeConfig, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	} else {
		// Use in-cluster config
		kubeConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	namespace := k8sConfig.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// Parse host and port from kubeconfig or use provided values
	host := "localhost"
	port := 6443
	if k8sConfig.Host != "" {
		host = k8sConfig.Host
	}
	if k8sConfig.Port != 0 {
		port = k8sConfig.Port
	} else if kubeConfig.Host != "" {
		// Parse host and port from kubeconfig
		if u, err := url.Parse(kubeConfig.Host); err == nil {
			host = u.Hostname()
			if u.Port() != "" {
				if p, err := strconv.Atoi(u.Port()); err == nil {
					port = p
				}
			}
		}
	}

	kp := &K8sProvider{
		clientset:  clientset,
		providerID: providerID,
		name:       name,
		config:     k8sConfig,
		namespace:  namespace,
		host:       host,
		port:       port,
	}

	kp.lastUpdateTime = time.Now()
	kp.status = StatusConnected

	return kp, nil
}

// GetCapacity returns the total cluster capacity
func (kp *K8sProvider) GetCapacity(ctx context.Context) (*Capacity, error) {
	nodes, err := kp.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var totalCPU, totalMemory, totalGPU float64

	for _, node := range nodes.Items {
		// Get allocatable resources (what's available for pods)
		cpuQuantity := node.Status.Allocatable[v1.ResourceCPU]
		memoryQuantity := node.Status.Allocatable[v1.ResourceMemory]

		// Convert CPU to cores
		cpuCores := float64(cpuQuantity.MilliValue()) / 1000.0
		totalCPU += cpuCores

		// Convert memory to bytes
		memoryBytes := float64(memoryQuantity.Value())
		totalMemory += memoryBytes

		// Check for GPU resources (nvidia.com/gpu)
		if gpuQuantity, exists := node.Status.Allocatable["nvidia.com/gpu"]; exists {
			totalGPU += float64(gpuQuantity.Value())
		}
	}

	total := &Info{
		CPU:    int64(totalCPU),
		Memory: int64(totalMemory),
		GPU:    int64(totalGPU),
	}

	// Get current usage
	allocated, err := kp.GetAllocated(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current usage: %w", err)
	}

	available := &Info{
		CPU:    total.CPU - int64(allocated.CPU),
		Memory: total.Memory - int64(allocated.Memory),
		GPU:    total.GPU - allocated.GPU,
	}

	return &Capacity{
		Total:     total,
		Used:      allocated,
		Available: available,
	}, nil
}

// GetAllocated returns current resource usage by all pods in the namespace
func (kp *K8sProvider) GetAllocated(ctx context.Context) (*Info, error) {
	pods, err := kp.clientset.CoreV1().Pods(kp.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	logrus.Infof("k8s provider get allocated, pod count: %d", len(pods.Items))

	var totalCPU, totalMemory, totalGPU int64

	for _, pod := range pods.Items {
		// Skip pods that are not running
		if pod.Status.Phase != v1.PodRunning {
			continue
		}

		for _, container := range pod.Spec.Containers {
			// Get CPU requests/limits
			if cpuRequest, exists := container.Resources.Requests[v1.ResourceCPU]; exists {
				cpuCores := int64(cpuRequest.MilliValue()) / 1000
				totalCPU += cpuCores
				logrus.Infof("Pod %s, Container %s: CPU request %d cores", pod.Name, container.Name, cpuCores)
			} else if cpuLimit, exists := container.Resources.Limits[v1.ResourceCPU]; exists {
				cpuCores := int64(cpuLimit.MilliValue()) / 1000
				totalCPU += cpuCores
				logrus.Infof("Pod %s, Container %s: CPU limit %d cores", pod.Name, container.Name, cpuCores)
			} else {
				// If no CPU request/limit is set, assume 0.1 cores per container
				cpuCores := int64(1000)
				totalCPU += cpuCores
				logrus.Infof("Pod %s, Container %s: No CPU request/limit set, assuming %d cores", pod.Name, container.Name, cpuCores)
			}

			// Get memory requests/limits
			if memRequest, exists := container.Resources.Requests[v1.ResourceMemory]; exists {
				memoryBytes := int64(memRequest.Value())
				totalMemory += memoryBytes
				logrus.Infof("Pod %s, Container %s: Memory request %d bytes", pod.Name, container.Name, memoryBytes)
			} else if memLimit, exists := container.Resources.Limits[v1.ResourceMemory]; exists {
				memoryBytes := int64(memLimit.Value())
				totalMemory += memoryBytes
				logrus.Infof("Pod %s, Container %s: Memory limit %d bytes", pod.Name, container.Name, memoryBytes)
			} else {
				// If no memory request/limit is set, assume 128MB per container
				memoryBytes := int64(128 * 1024 * 1024) // 128MB in bytes
				totalMemory += memoryBytes
				logrus.Infof("Pod %s, Container %s: No memory request/limit set, assuming %d bytes", pod.Name, container.Name, memoryBytes)
			}

			// Get GPU requests/limits
			if gpuRequest, exists := container.Resources.Requests["nvidia.com/gpu"]; exists {
				gpuCount := int64(gpuRequest.Value())
				totalGPU += gpuCount
				logrus.Infof("Pod %s, Container %s: GPU request %d", pod.Name, container.Name, gpuCount)
			} else if gpuLimit, exists := container.Resources.Limits["nvidia.com/gpu"]; exists {
				gpuCount := int64(gpuLimit.Value())
				totalGPU += gpuCount
				logrus.Infof("Pod %s, Container %s: GPU limit %d", pod.Name, container.Name, gpuCount)
			}
		}
	}

	logrus.Infof("k8s provider get allocated, allocatedCPU: %d, allocatedMemory: %d, allocatedGPU: %d", totalCPU, totalMemory, totalGPU)

	return &Info{
		CPU:    totalCPU,
		Memory: totalMemory,
		GPU:    int64(totalGPU),
	}, nil
}

// GetType returns the provider type
func (kp *K8sProvider) GetType() string {
	return "k8s"
}

// GetID returns the provider ID
func (kp *K8sProvider) GetID() string {
	return kp.providerID
}

func (kp *K8sProvider) GetName() string {
	return kp.name
}

// GetHost returns the Kubernetes API server host
func (kp *K8sProvider) GetHost() string {
	return kp.host
}

// GetPort returns the Kubernetes API server port
func (kp *K8sProvider) GetPort() int {
	return kp.port
}

// GetLocalK8sProvider creates a new local Kubernetes provider instance
func GetLocalK8sProvider() (*K8sProvider, error) {
	// Try in-cluster config first
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to default kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules, &clientcmd.ConfigOverrides{})

		kubeConfig, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	kp := &K8sProvider{
		clientset:  clientset,
		providerID: "local-k8s",
		namespace:  "default",
	}

	kp.name = "standalone"
	kp.lastUpdateTime = time.Now()
	kp.status = StatusConnected

	return kp, nil
}

func (kp *K8sProvider) GetLastUpdateTime() time.Time {
	return kp.lastUpdateTime
}

// Close closes the Kubernetes client connection
func (kp *K8sProvider) Close() error {
	// Kubernetes client doesn't need explicit closing
	return nil
}

func (kp *K8sProvider) GetStatus() Status {
	return kp.status
}

func (kp *K8sProvider) Deploy(ctx context.Context, spec ContainerSpec) (string, error) {
	// Create a pod specification
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "iarnet-",
			Namespace:    kp.namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    "main",
					Image:   spec.Image,
					Command: spec.Command,
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewMilliQuantity(int64(spec.Requirements.CPU*1000), resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(int64(spec.Requirements.Memory), resource.BinarySI),
						},
						Limits: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewMilliQuantity(int64(spec.Requirements.CPU*1000), resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(int64(spec.Requirements.Memory), resource.BinarySI),
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	// Add GPU resources if specified
	if spec.Requirements.GPU > 0 {
		gpuQuantity := *resource.NewQuantity(int64(spec.Requirements.GPU), resource.DecimalSI)
		pod.Spec.Containers[0].Resources.Requests["nvidia.com/gpu"] = gpuQuantity
		pod.Spec.Containers[0].Resources.Limits["nvidia.com/gpu"] = gpuQuantity
	}

	// Create the pod
	createdPod, err := kp.clientset.CoreV1().Pods(kp.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create pod: %w", err)
	}

	return createdPod.Name, nil
}

func (kp *K8sProvider) GetLogs(podName string, lines int) ([]string, error) {
	logrus.Debugf("Getting logs for pod %s, lines: %d", podName, lines)

	// Create log options
	logOptions := &v1.PodLogOptions{
		TailLines:  func(i int) *int64 { i64 := int64(i); return &i64 }(lines),
		Timestamps: true,
	}

	// Get pod logs
	ctx := context.Background()
	req := kp.clientset.CoreV1().Pods(kp.namespace).GetLogs(podName, logOptions)
	logsReader, err := req.Stream(ctx)
	if err != nil {
		logrus.Errorf("Failed to get pod logs for %s: %v", podName, err)
		return nil, fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer logsReader.Close()

	// Read log content
	var logLines []string
	buffer := make([]byte, 4096)
	var logContent strings.Builder

	for {
		n, err := logsReader.Read(buffer)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			logrus.Errorf("Error reading pod logs for %s: %v", podName, err)
			return nil, fmt.Errorf("error reading pod logs: %w", err)
		}

		if n == 0 {
			break
		}

		logContent.Write(buffer[:n])
	}

	// Split logs into lines
	logText := logContent.String()
	lines_split := strings.Split(strings.TrimSpace(logText), "\n")
	for _, line := range lines_split {
		if strings.TrimSpace(line) != "" {
			logLines = append(logLines, strings.TrimSpace(line))
		}
	}

	// Limit returned log lines
	if len(logLines) > lines {
		logLines = logLines[len(logLines)-lines:]
	}

	logrus.Debugf("Successfully retrieved %d log lines for pod %s", len(logLines), podName)
	return logLines, nil
}
