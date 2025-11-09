package component

import (
	"context"
	"fmt"
	"time"

	"github.com/9triver/iarnet/internal/resource"
	"github.com/sirupsen/logrus"
)

type Provider interface {
	GetCapacity(ctx context.Context) (*resource.Capacity, error)
	GetAvailable(ctx context.Context) (*resource.Info, error)
	GetType() string
	GetID() string
	GetName() string
	GetHost() string
	GetPort() int
	GetLastUpdateTime() time.Time
	GetStatus() resource.ProviderStatus
	GetLogs(d string, lines int) ([]string, error)
	DeployComponent(ctx context.Context, runtimeEnv resource.RuntimeEnv, resourceRequest *resource.ResourceRequest) (*resource.ContainerRef, error)
}

type provider struct {
	id             string
	name           string
	host           string
	port           int
	providerType   string
	lastUpdateTime time.Time
	status         resource.ProviderStatus
}

// NewProviderOptions provider 创建选项
type NewProviderOptions struct {
	ID           string // 如果提供，将使用此 ID；否则通过 RPC 注册获取
	Name         string
	Host         string
	Port         int
	ProviderType string // provider 类型（如 "docker", "k8s"）
	ServiceHost  string // RPC 服务地址
	ServicePort  int    // RPC 服务端口
	Config       string // 可选配置信息（JSON 格式）
}

// NewProvider 创建新的 provider，如果未提供 ID，将通过 RPC 服务注册并获取分配的 ID
func NewProvider(ctx context.Context, opts NewProviderOptions) (Provider, error) {
	var providerID string
	var err error

	// 如果未提供 ID，通过 RPC 服务注册并获取分配的 ID
	if opts.ID == "" {
		if opts.ServiceHost == "" || opts.ServicePort == 0 {
			return nil, fmt.Errorf("service host and port are required when ID is not provided")
		}

		client, err := provider.NewClient(opts.ServiceHost, opts.ServicePort)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider client: %w", err)
		}
		defer client.Close()

		providerID, err = client.RegisterProvider(
			ctx,
			opts.Name,
			opts.Host,
			opts.ProviderType,
			int32(opts.Port),
			opts.Config,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to register provider via RPC: %w", err)
		}

		logrus.Infof("Successfully registered provider %s with ID %s via RPC", opts.Name, providerID)
	} else {
		providerID = opts.ID
	}

	return &provider{
		id:             providerID,
		name:           opts.Name,
		host:           opts.Host,
		port:           opts.Port,
		providerType:   opts.ProviderType,
		lastUpdateTime: time.Now(),
		status:         resource.ProviderStatusConnected,
	}, nil
}

func (p *provider) GetID() string {
	return p.id
}

func (p *provider) GetName() string {
	return p.name
}

func (p *provider) GetHost() string {
	return p.host
}

func (p *provider) GetPort() int {
	return p.port
}

func (p *provider) GetType() string {
	return p.providerType
}

func (p *provider) GetLastUpdateTime() time.Time {
	return p.lastUpdateTime
}

func (p *provider) GetStatus() resource.ProviderStatus {
	return p.status
}

func (p *provider) GetCapacity(ctx context.Context) (*resource.Capacity, error) {
	// TODO: 实现获取容量的逻辑
	return nil, fmt.Errorf("not implemented")
}

func (p *provider) GetAvailable(ctx context.Context) (*resource.Info, error) {
	// TODO: 实现获取可用资源的逻辑
	return nil, fmt.Errorf("not implemented")
}

func (p *provider) GetLogs(d string, lines int) ([]string, error) {
	// TODO: 实现获取日志的逻辑
	return nil, fmt.Errorf("not implemented")
}

func (p *provider) DeployComponent(ctx context.Context, runtimeEnv resource.RuntimeEnv, resourceRequest *resource.ResourceRequest) (*resource.ContainerRef, error) {
	// TODO: 实现部署组件的逻辑
	return nil, fmt.Errorf("not implemented")
}
