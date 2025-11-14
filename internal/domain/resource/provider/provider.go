package provider

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/lithammer/shortuuid/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Provider interface {
	Connect(ctx context.Context) error
	GetCapacity(ctx context.Context) (*types.Capacity, error)
	GetAvailable(ctx context.Context) (*types.Info, error)
	GetType() types.ProviderType
	GetID() string
	GetName() string
	GetHost() string
	GetPort() int
	GetLastUpdateTime() time.Time
	GetStatus() types.ProviderStatus
	GetLogs(d string, lines int) ([]string, error)
	Deploy(ctx context.Context, id, image string, resourceRequest *types.Info) error
}

type provider struct {
	id             string
	name           string
	host           string
	port           int
	providerType   types.ProviderType
	lastUpdateTime time.Time
	status         types.ProviderStatus

	conn   *grpc.ClientConn
	client providerpb.ProviderServiceClient
	cfg    *config.Config // 配置引用
}

// NewProvider 创建新的 provider，如果未提供 ID，将通过 RPC 服务注册并获取分配的 ID
func NewProvider(name string, host string, port int, cfg *config.Config) Provider {
	return &provider{
		name:           name,
		cfg:            cfg,
		host:           host,
		port:           port,
		lastUpdateTime: time.Now(),
		status:         types.ProviderStatusDisconnected,
	}
}

func (p *provider) Connect(ctx context.Context) error {
	// 如果未提供 ID，通过 RPC 服务注册并获取分配的 ID
	if p.host == "" || p.port == 0 {
		return fmt.Errorf("service host and port are required when ID is not provided")
	}

	conn, err := grpc.NewClient(fmt.Sprintf("%s:%d", p.host, p.port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create provider connection: %w", err)
	}
	client := providerpb.NewProviderServiceClient(conn)

	providerID := shortuuid.New()
	req := &providerpb.AssignIDRequest{
		ProviderId: providerID,
	}
	resp, err := client.AssignID(ctx, req)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to assign ID: %w", err)
	}
	if !resp.Success {
		conn.Close()
		return fmt.Errorf("failed to assign ID: %s", resp.Error)
	}

	p.id = providerID
	p.providerType = types.ProviderType(resp.ProviderType.Name)
	p.client = client
	p.conn = conn
	p.status = types.ProviderStatusConnected
	return nil
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

func (p *provider) GetType() types.ProviderType {
	return p.providerType
}

func (p *provider) GetLastUpdateTime() time.Time {
	return p.lastUpdateTime
}

func (p *provider) GetStatus() types.ProviderStatus {
	return p.status
}

func (p *provider) GetCapacity(ctx context.Context) (*types.Capacity, error) {
	if p.client == nil {
		return nil, fmt.Errorf("provider not connected")
	}
	req := &providerpb.GetCapacityRequest{}
	resp, err := p.client.GetCapacity(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get capacity: %w", err)
	}
	return &types.Capacity{
		Total:     &types.Info{CPU: resp.Capacity.Total.Cpu, Memory: resp.Capacity.Total.Memory, GPU: resp.Capacity.Total.Gpu},
		Used:      &types.Info{CPU: resp.Capacity.Used.Cpu, Memory: resp.Capacity.Used.Memory, GPU: resp.Capacity.Used.Gpu},
		Available: &types.Info{CPU: resp.Capacity.Available.Cpu, Memory: resp.Capacity.Available.Memory, GPU: resp.Capacity.Available.Gpu},
	}, nil
}

func (p *provider) GetAvailable(ctx context.Context) (*types.Info, error) {
	if p.client == nil {
		return nil, fmt.Errorf("provider not connected")
	}
	req := &providerpb.GetAvailableRequest{}
	resp, err := p.client.GetAvailable(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get available: %w", err)
	}
	return &types.Info{CPU: resp.Available.Cpu, Memory: resp.Available.Memory, GPU: resp.Available.Gpu}, nil
}

func (p *provider) GetLogs(d string, lines int) ([]string, error) {
	// TODO: 实现获取日志的逻辑
	return nil, fmt.Errorf("not implemented")
}

func (p *provider) Deploy(ctx context.Context, id, image string, resourceRequest *types.Info) error {
	if p.client == nil {
		return fmt.Errorf("provider not connected")
	}
	req := &providerpb.DeployComponentRequest{
		ComponentId: id,
		Image:       image,
		ResourceRequest: &resourcepb.Info{
			Cpu:    resourceRequest.CPU,
			Memory: resourceRequest.Memory,
			Gpu:    resourceRequest.GPU,
		},
		EnvVars: map[string]string{
			"COMPONENT_ID": id,
			"ZMQ_ADDR":     net.JoinHostPort(p.cfg.Host, strconv.Itoa(p.cfg.Transport.ZMQ.Port)),
			"STORE_ADDR":   net.JoinHostPort(p.cfg.Host, strconv.Itoa(p.cfg.Transport.RPC.Store.Port)),
		},
	}
	resp, err := p.client.DeployComponent(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to deploy component: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("failed to deploy component: %s", resp.Error)
	}
	return nil
}

func (p *provider) Close() error {
	if p.client != nil {
		return p.conn.Close()
	}
	return nil
}
