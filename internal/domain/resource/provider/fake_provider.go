package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/types"
	common "github.com/9triver/iarnet/internal/proto/common"
	"github.com/9triver/iarnet/internal/util"
	"github.com/sirupsen/logrus"
)

// FakeProvider 假 Provider，用于演示
// 始终显示为完全使用状态，不参与调度决策
type FakeProvider struct {
	id             string
	name           string
	host           string
	port           int
	providerType   types.ProviderType
	lastUpdateTime time.Time
	status         types.ProviderStatus

	// 资源容量（固定值）
	totalCapacity *types.Capacity
	capacityMu    sync.RWMutex

	// 资源标签（可配置）
	resourceTags *types.ResourceTags
	tagsMu       sync.RWMutex

	// 支持的语言列表（默认支持所有语言）
	supportedLanguages []common.Language
	languagesMu        sync.RWMutex
}

// UsageConfig 资源使用状态配置
type UsageConfig struct {
	CPURatio    float64     // CPU 使用率（0.0-1.0），例如 0.8 表示使用 80%
	GPURatio    float64     // GPU 使用率（0.0-1.0），例如 0.8 表示使用 80%
	MemoryRatio float64     // Memory 使用率（0.0-1.0），例如 0.8 表示使用 80%
	Used        *types.Info // 直接指定已使用的资源量（如果提供，优先使用）
}

// NewFakeProvider 创建新的假 Provider
// resourceTags: 资源标签配置，如果为 nil 则使用默认值（CPU=true, GPU=true, Memory=true, Camera=false）
// usage: 资源使用状态配置，如果为 nil 则默认完全使用（100%）
func NewFakeProvider(name string, providerType string, cpu int64, memory int64, gpu int64, host string, port int, resourceTags *types.ResourceTags, usage *UsageConfig) *FakeProvider {
	// 转换为 ProviderType
	var pType types.ProviderType
	switch providerType {
	case "docker":
		pType = types.ProviderType("docker")
	case "k8s", "kubernetes":
		pType = types.ProviderType("kubernetes")
	default:
		pType = types.ProviderType("docker") // 默认使用 docker
	}

	// 计算已使用资源
	var usedCPU, usedMemory, usedGPU int64
	if usage != nil && usage.Used != nil {
		// 如果直接指定了已使用资源，使用指定值
		usedCPU = usage.Used.CPU
		usedMemory = usage.Used.Memory
		usedGPU = usage.Used.GPU
		// 确保不超过总容量
		if usedCPU > cpu {
			usedCPU = cpu
		}
		if usedMemory > memory {
			usedMemory = memory
		}
		if usedGPU > gpu {
			usedGPU = gpu
		}
	} else if usage != nil {
		// 如果指定了使用率，根据分别配置的使用率计算
		// CPU 使用率
		cpuRatio := usage.CPURatio
		if cpuRatio > 1.0 {
			cpuRatio = 1.0
		}
		if cpuRatio < 0.0 {
			cpuRatio = 0.0
		}
		if cpuRatio > 0 {
			usedCPU = int64(float64(cpu) * cpuRatio)
		} else {
			usedCPU = cpu // 默认完全使用
		}

		// Memory 使用率
		memoryRatio := usage.MemoryRatio
		if memoryRatio > 1.0 {
			memoryRatio = 1.0
		}
		if memoryRatio < 0.0 {
			memoryRatio = 0.0
		}
		if memoryRatio > 0 {
			usedMemory = int64(float64(memory) * memoryRatio)
		} else {
			usedMemory = memory // 默认完全使用
		}

		// GPU 使用率
		gpuRatio := usage.GPURatio
		if gpuRatio > 1.0 {
			gpuRatio = 1.0
		}
		if gpuRatio < 0.0 {
			gpuRatio = 0.0
		}
		if gpuRatio > 0 {
			usedGPU = int64(float64(gpu) * gpuRatio)
		} else {
			usedGPU = gpu // 默认完全使用
		}
	} else {
		// 默认完全使用（100%）
		usedCPU = cpu
		usedMemory = memory
		usedGPU = gpu
	}

	// 计算可用资源
	availableCPU := cpu - usedCPU
	availableMemory := memory - usedMemory
	availableGPU := gpu - usedGPU
	if availableCPU < 0 {
		availableCPU = 0
	}
	if availableMemory < 0 {
		availableMemory = 0
	}
	if availableGPU < 0 {
		availableGPU = 0
	}

	// 创建容量信息
	capacity := &types.Capacity{
		Total: &types.Info{
			CPU:    cpu,
			Memory: memory,
			GPU:    gpu,
		},
		Used: &types.Info{
			CPU:    usedCPU,
			Memory: usedMemory,
			GPU:    usedGPU,
		},
		Available: &types.Info{
			CPU:    availableCPU,
			Memory: availableMemory,
			GPU:    availableGPU,
		},
	}

	// 默认支持所有语言
	supportedLanguages := []common.Language{
		common.Language_LANG_PYTHON,
		common.Language_LANG_GO,
		common.Language_LANG_UNIKERNEL,
	}

	// 设置资源标签（如果未提供，使用默认值）
	if resourceTags == nil {
		resourceTags = &types.ResourceTags{
			CPU:    true,
			GPU:    true,
			Memory: true,
			Camera: false,
		}
	}

	return &FakeProvider{
		id:                 util.GenIDWith("fake.provider."),
		name:               name,
		host:               host,
		port:               port,
		providerType:       pType,
		lastUpdateTime:     time.Now(),
		status:             types.ProviderStatusConnected, // 始终显示为已连接
		totalCapacity:      capacity,
		resourceTags:       resourceTags,
		supportedLanguages: supportedLanguages,
	}
}

// 实现 Provider 接口的所有方法

func (p *FakeProvider) GetID() string {
	return p.id
}

func (p *FakeProvider) GetName() string {
	return p.name
}

func (p *FakeProvider) SetName(name string) {
	p.name = name
	p.lastUpdateTime = time.Now()
}

func (p *FakeProvider) GetHost() string {
	return p.host
}

func (p *FakeProvider) GetPort() int {
	return p.port
}

func (p *FakeProvider) GetType() types.ProviderType {
	return p.providerType
}

func (p *FakeProvider) GetLastUpdateTime() time.Time {
	return p.lastUpdateTime
}

func (p *FakeProvider) GetStatus() types.ProviderStatus {
	return p.status
}

func (p *FakeProvider) SetStatus(status types.ProviderStatus) {
	p.status = status
}

// Connect 假 Provider 不需要连接，直接返回成功
func (p *FakeProvider) Connect(ctx context.Context) error {
	p.status = types.ProviderStatusConnected
	logrus.Infof("Fake provider %s connected (simulated)", p.id)
	return nil
}

// Disconnect 假 Provider 的断开连接
func (p *FakeProvider) Disconnect() {
	p.status = types.ProviderStatusDisconnected
	logrus.Infof("Fake provider %s disconnected (simulated)", p.id)
}

// HealthCheck 假 Provider 的健康检查始终成功
func (p *FakeProvider) HealthCheck(ctx context.Context) error {
	// 更新最后更新时间
	p.lastUpdateTime = time.Now()
	return nil
}

// GetResourceTags 获取资源标签（返回配置的资源标签）
func (p *FakeProvider) GetResourceTags() *types.ResourceTags {
	p.tagsMu.RLock()
	defer p.tagsMu.RUnlock()

	if p.resourceTags == nil {
		// 如果未配置，返回默认值
		return &types.ResourceTags{
			CPU:    true,
			GPU:    true,
			Memory: true,
			Camera: false,
		}
	}

	// 返回副本以避免并发修改
	return &types.ResourceTags{
		CPU:    p.resourceTags.CPU,
		GPU:    p.resourceTags.GPU,
		Memory: p.resourceTags.Memory,
		Camera: p.resourceTags.Camera,
	}
}

// GetSupportedLanguages 获取支持的语言列表
func (p *FakeProvider) GetSupportedLanguages() []common.Language {
	p.languagesMu.RLock()
	defer p.languagesMu.RUnlock()

	if p.supportedLanguages == nil {
		return nil
	}

	// 返回副本以避免并发修改
	languages := make([]common.Language, len(p.supportedLanguages))
	copy(languages, p.supportedLanguages)
	return languages
}

// SupportsLanguage 检查 provider 是否支持指定的语言
func (p *FakeProvider) SupportsLanguage(lang common.Language) bool {
	p.languagesMu.RLock()
	defer p.languagesMu.RUnlock()

	for _, supportedLang := range p.supportedLanguages {
		if supportedLang == lang {
			return true
		}
	}
	return false
}

// GetCapacity 获取资源容量（返回配置的使用状态）
func (p *FakeProvider) GetCapacity(ctx context.Context, forceRefresh ...bool) (*types.Capacity, error) {
	p.capacityMu.RLock()
	defer p.capacityMu.RUnlock()

	// 返回副本以避免并发修改
	return &types.Capacity{
		Total: &types.Info{
			CPU:    p.totalCapacity.Total.CPU,
			Memory: p.totalCapacity.Total.Memory,
			GPU:    p.totalCapacity.Total.GPU,
		},
		Used: &types.Info{
			CPU:    p.totalCapacity.Used.CPU,
			Memory: p.totalCapacity.Used.Memory,
			GPU:    p.totalCapacity.Used.GPU,
		},
		Available: &types.Info{
			CPU:    p.totalCapacity.Available.CPU,
			Memory: p.totalCapacity.Available.Memory,
			GPU:    p.totalCapacity.Available.GPU,
		},
	}, nil
}

// GetAvailable 获取可用资源（返回配置的可用资源）
func (p *FakeProvider) GetAvailable(ctx context.Context, forceRefresh ...bool) (*types.Info, error) {
	p.capacityMu.RLock()
	defer p.capacityMu.RUnlock()

	if p.totalCapacity == nil || p.totalCapacity.Available == nil {
		return &types.Info{
			CPU:    0,
			Memory: 0,
			GPU:    0,
		}, nil
	}

	// 返回副本以避免并发修改
	return &types.Info{
		CPU:    p.totalCapacity.Available.CPU,
		Memory: p.totalCapacity.Available.Memory,
		GPU:    p.totalCapacity.Available.GPU,
	}, nil
}

// GetRealTimeUsage 获取实时资源使用情况（返回配置的使用状态）
func (p *FakeProvider) GetRealTimeUsage(ctx context.Context) (*types.Info, error) {
	p.capacityMu.RLock()
	defer p.capacityMu.RUnlock()

	if p.totalCapacity == nil || p.totalCapacity.Used == nil {
		return &types.Info{
			CPU:    0,
			Memory: 0,
			GPU:    0,
		}, nil
	}

	// 返回副本以避免并发修改
	return &types.Info{
		CPU:    p.totalCapacity.Used.CPU,
		Memory: p.totalCapacity.Used.Memory,
		GPU:    p.totalCapacity.Used.GPU,
	}, nil
}

// Deploy 假 Provider 不支持部署，返回错误
func (p *FakeProvider) Deploy(ctx context.Context, id string, language common.Language, resourceRequest *types.Info) error {
	return fmt.Errorf("fake provider does not support deployment")
}

// Undeploy 假 Provider 不支持卸载，返回错误
func (p *FakeProvider) Undeploy(ctx context.Context, componentID string) error {
	return fmt.Errorf("fake provider does not support undeployment")
}

// GetLogs 假 Provider 不支持获取日志
func (p *FakeProvider) GetLogs(d string, lines int) ([]string, error) {
	return nil, fmt.Errorf("fake provider does not support log retrieval")
}

// Close 关闭假 Provider（无操作）
func (p *FakeProvider) Close() error {
	return nil
}

// IsFake 检查是否为假 Provider（用于调度时排除）
func (p *FakeProvider) IsFake() bool {
	return true
}
