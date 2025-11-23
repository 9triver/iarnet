package provider

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/types"
	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/9triver/iarnet/internal/util"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type EnvVariables struct {
	IarnetHost string
	ZMQPort    int
	StorePort  int
	LoggerPort int
}

// ResourceTags 资源标签（描述 provider 支持的计算资源类型）
type ResourceTags struct {
	CPU    bool
	GPU    bool
	Memory bool
	Camera bool
}

// Provider 资源提供者
type Provider struct {
	id             string
	name           string
	host           string
	port           int
	providerType   types.ProviderType
	lastUpdateTime time.Time
	status         types.ProviderStatus

	conn   *grpc.ClientConn
	client providerpb.ServiceClient

	envVariables *EnvVariables

	// 资源缓存（从健康检测响应中获取）
	cachedCapacity *types.Capacity
	cachedTags     *ResourceTags
	cacheTimestamp time.Time
	cacheMu        sync.RWMutex
}

// NewProvider 创建新的 provider，如果未提供 ID，将通过 RPC 服务注册并获取分配的 ID
func NewProvider(name string, host string, port int, envVariables *EnvVariables) *Provider {
	return &Provider{
		id:             util.GenIDWith("provider."),
		name:           name,
		envVariables:   envVariables,
		host:           host,
		port:           port,
		lastUpdateTime: time.Now(),
		status:         types.ProviderStatusDisconnected,
	}
}

// NewProviderWithID 从持久化数据重建 Provider（使用保存的 ID）
// 用于从数据库恢复 Provider 对象
func NewProviderWithID(id, name string, host string, port int, envVariables *EnvVariables) *Provider {
	return &Provider{
		id:             id,
		name:           name,
		envVariables:   envVariables,
		host:           host,
		port:           port,
		lastUpdateTime: time.Now(),
		status:         types.ProviderStatusDisconnected,
		// conn 和 client 保持为 nil，需要在业务层重新连接
	}
}

func (p *Provider) Connect(ctx context.Context) error {
	// 如果未提供 ID，通过 RPC 服务注册并获取分配的 ID
	if p.host == "" || p.port == 0 {
		return fmt.Errorf("service host and port are required when ID is not provided")
	}

	conn, err := grpc.NewClient(fmt.Sprintf("%s:%d", p.host, p.port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create provider connection: %w", err)
	}
	client := providerpb.NewServiceClient(conn)

	req := &providerpb.ConnectRequest{
		ProviderId: p.id,
	}
	resp, err := client.Connect(ctx, req)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to assign ID: %w", err)
	}
	if !resp.Success {
		conn.Close()
		return fmt.Errorf("failed to assign ID: %s", resp.Error)
	}

	p.providerType = types.ProviderType(resp.ProviderType.Name)
	p.client = client
	p.conn = conn
	p.status = types.ProviderStatusConnected
	return nil
}

func (p *Provider) GetID() string {
	return p.id
}

func (p *Provider) GetName() string {
	return p.name
}

func (p *Provider) SetName(name string) {
	p.name = name
	p.lastUpdateTime = time.Now()
}

func (p *Provider) GetHost() string {
	return p.host
}

func (p *Provider) GetPort() int {
	return p.port
}

func (p *Provider) GetType() types.ProviderType {
	return p.providerType
}

func (p *Provider) GetLastUpdateTime() time.Time {
	return p.lastUpdateTime
}

func (p *Provider) GetStatus() types.ProviderStatus {
	return p.status
}

// SetStatus 设置 provider 状态
func (p *Provider) SetStatus(status types.ProviderStatus) {
	p.status = status
}

// Disconnect 断开连接但不清除 ID，仅更新状态
// 用于健康检测失败时，让 provider 感知到 iarnet 的管理状态
func (p *Provider) Disconnect() {
	if p.status != types.ProviderStatusConnected {
		return
	}
	p.status = types.ProviderStatusDisconnected
	_, err := p.client.Disconnect(context.Background(), &providerpb.DisconnectRequest{
		ProviderId: p.id,
	})
	if err != nil {
		logrus.Warnf("Failed to disconnect from provider %s: %v", p.id, err)
	}
	if p.conn != nil {

		p.conn.Close()
		p.conn = nil
	}
	p.client = nil
}

// HealthCheck 健康检测，检查 provider 是否仍然连接，并更新资源缓存
func (p *Provider) HealthCheck(ctx context.Context) error {
	if p.client == nil || p.id == "" {
		return fmt.Errorf("provider not connected")
	}
	req := &providerpb.HealthCheckRequest{
		ProviderId: p.id,
	}
	resp, err := p.client.HealthCheck(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to health check: %w", err)
	}

	// 更新资源缓存
	p.updateCacheFromHealthCheckResponse(resp)
	return nil
}

// updateCacheFromHealthCheckResponse 从健康检测响应更新缓存
func (p *Provider) updateCacheFromHealthCheckResponse(resp *providerpb.HealthCheckResponse) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()

	if resp.Capacity != nil {
		p.cachedCapacity = &types.Capacity{
			Total: &types.Info{
				CPU:    resp.Capacity.Total.Cpu,
				Memory: resp.Capacity.Total.Memory,
				GPU:    resp.Capacity.Total.Gpu,
			},
			Used: &types.Info{
				CPU:    resp.Capacity.Used.Cpu,
				Memory: resp.Capacity.Used.Memory,
				GPU:    resp.Capacity.Used.Gpu,
			},
			Available: &types.Info{
				CPU:    resp.Capacity.Available.Cpu,
				Memory: resp.Capacity.Available.Memory,
				GPU:    resp.Capacity.Available.Gpu,
			},
		}
	}

	if resp.ResourceTags != nil {
		p.cachedTags = &ResourceTags{
			CPU:    resp.ResourceTags.Cpu,
			GPU:    resp.ResourceTags.Gpu,
			Memory: resp.ResourceTags.Memory,
			Camera: resp.ResourceTags.Camera,
		}
	}

	p.cacheTimestamp = time.Now()
	logrus.Debugf("Updated resource cache for provider %s at %v", p.id, p.cacheTimestamp)
}

// GetResourceTags 获取缓存的资源标签（返回副本以避免并发问题）
func (p *Provider) GetResourceTags() *ResourceTags {
	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()

	if p.cachedTags == nil {
		return nil
	}

	// 返回副本以避免并发修改
	return &ResourceTags{
		CPU:    p.cachedTags.CPU,
		GPU:    p.cachedTags.GPU,
		Memory: p.cachedTags.Memory,
		Camera: p.cachedTags.Camera,
	}
}

// getCachedCapacity 获取缓存的资源容量（返回副本以避免并发问题）
func (p *Provider) getCachedCapacity() *types.Capacity {
	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()

	if p.cachedCapacity == nil {
		return nil
	}

	// 返回副本以避免并发修改
	return &types.Capacity{
		Total: &types.Info{
			CPU:    p.cachedCapacity.Total.CPU,
			Memory: p.cachedCapacity.Total.Memory,
			GPU:    p.cachedCapacity.Total.GPU,
		},
		Used: &types.Info{
			CPU:    p.cachedCapacity.Used.CPU,
			Memory: p.cachedCapacity.Used.Memory,
			GPU:    p.cachedCapacity.Used.GPU,
		},
		Available: &types.Info{
			CPU:    p.cachedCapacity.Available.CPU,
			Memory: p.cachedCapacity.Available.Memory,
			GPU:    p.cachedCapacity.Available.GPU,
		},
	}
}

// refreshCapacityCache 刷新资源容量缓存（从 provider 实时获取）
func (p *Provider) refreshCapacityCache(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("provider not connected")
	}

	// 实时获取容量
	capacity, err := p.fetchCapacityFromProvider(ctx)
	if err != nil {
		return err
	}

	// 更新缓存
	p.cacheMu.Lock()
	p.cachedCapacity = capacity
	p.cacheTimestamp = time.Now()
	p.cacheMu.Unlock()

	logrus.Debugf("Refreshed capacity cache for provider %s", p.id)
	return nil
}

// fetchCapacityFromProvider 从 provider 实时获取资源容量并更新缓存
func (p *Provider) fetchCapacityFromProvider(ctx context.Context) (*types.Capacity, error) {
	var client providerpb.ServiceClient
	var conn *grpc.ClientConn
	var err error

	if p.client != nil {
		client = p.client
	} else {
		// 创建临时连接（用于测试场景）
		if p.host == "" || p.port == 0 {
			return nil, fmt.Errorf("provider host and port are required")
		}
		conn, err = grpc.NewClient(fmt.Sprintf("%s:%d", p.host, p.port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("failed to create provider connection: %w", err)
		}
		client = providerpb.NewServiceClient(conn)
		defer conn.Close()
	}

	req := &providerpb.GetCapacityRequest{
		ProviderId: p.id,
	}
	resp, err := client.GetCapacity(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get capacity: %w", err)
	}

	capacity := &types.Capacity{
		Total:     &types.Info{CPU: resp.Capacity.Total.Cpu, Memory: resp.Capacity.Total.Memory, GPU: resp.Capacity.Total.Gpu},
		Used:      &types.Info{CPU: resp.Capacity.Used.Cpu, Memory: resp.Capacity.Used.Memory, GPU: resp.Capacity.Used.Gpu},
		Available: &types.Info{CPU: resp.Capacity.Available.Cpu, Memory: resp.Capacity.Available.Memory, GPU: resp.Capacity.Available.Gpu},
	}

	// 更新缓存
	p.cacheMu.Lock()
	p.cachedCapacity = capacity
	p.cacheTimestamp = time.Now()
	p.cacheMu.Unlock()

	return capacity, nil
}

// GetCapacity 获取资源容量，优先使用缓存
// forceRefresh: 如果为 true，强制从 provider 实时获取
func (p *Provider) GetCapacity(ctx context.Context, forceRefresh ...bool) (*types.Capacity, error) {
	shouldRefresh := len(forceRefresh) > 0 && forceRefresh[0]

	// 如果不需要强制刷新，尝试使用缓存
	if !shouldRefresh {
		if cached := p.getCachedCapacity(); cached != nil {
			return cached, nil
		}
	}

	// 缓存不存在或需要强制刷新，从 provider 实时获取
	return p.fetchCapacityFromProvider(ctx)
}

// GetAvailable 获取可用资源，优先使用缓存
// forceRefresh: 如果为 true，强制从 provider 实时获取
func (p *Provider) GetAvailable(ctx context.Context, forceRefresh ...bool) (*types.Info, error) {
	shouldRefresh := len(forceRefresh) > 0 && forceRefresh[0]

	// 如果不需要强制刷新，尝试使用缓存
	if !shouldRefresh {
		if cached := p.getCachedCapacity(); cached != nil && cached.Available != nil {
			// 返回缓存的可用资源副本
			return &types.Info{
				CPU:    cached.Available.CPU,
				Memory: cached.Available.Memory,
				GPU:    cached.Available.GPU,
			}, nil
		}
	}

	// 缓存不存在或需要强制刷新，从 provider 实时获取
	capacity, err := p.fetchCapacityFromProvider(ctx)
	if err != nil {
		return nil, err
	}
	if capacity.Available == nil {
		return nil, fmt.Errorf("available resources not found in capacity response")
	}
	return capacity.Available, nil
}

func (p *Provider) GetLogs(d string, lines int) ([]string, error) {
	// TODO: 实现获取日志的逻辑
	return nil, fmt.Errorf("not implemented")
}

func (p *Provider) Deploy(ctx context.Context, id, image string, resourceRequest *types.Info) error {
	if p.client == nil {
		return fmt.Errorf("provider not connected")
	}
	if p.id == "" {
		return fmt.Errorf("provider not connected, please call Connect first")
	}
	req := &providerpb.DeployRequest{
		InstanceId: id,
		Image:      image,
		ResourceRequest: &resourcepb.Info{
			Cpu:    resourceRequest.CPU,
			Memory: resourceRequest.Memory,
			Gpu:    resourceRequest.GPU,
		},
		EnvVars: map[string]string{
			"COMPONENT_ID": id,
			"ZMQ_ADDR":     net.JoinHostPort(p.envVariables.IarnetHost, strconv.Itoa(p.envVariables.ZMQPort)),
			"STORE_ADDR":   net.JoinHostPort(p.envVariables.IarnetHost, strconv.Itoa(p.envVariables.StorePort)),
			"LOGGER_ADDR":  net.JoinHostPort(p.envVariables.IarnetHost, strconv.Itoa(p.envVariables.LoggerPort)),
		},
		ProviderId: p.id, // 必须传递 provider_id
	}
	resp, err := p.client.Deploy(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to deploy component: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("failed to deploy component: %s", resp.Error)
	}

	// 部署成功后，立即刷新资源缓存以确保数据准确性
	if err := p.refreshCapacityCache(ctx); err != nil {
		logrus.Warnf("Failed to refresh capacity cache after deployment for provider %s: %v", p.id, err)
		// 不返回错误，因为部署已经成功，缓存刷新失败不应该影响部署结果
	}

	return nil
}

func (p *Provider) Close() error {
	if p.client != nil {
		return p.conn.Close()
	}
	return nil
}
