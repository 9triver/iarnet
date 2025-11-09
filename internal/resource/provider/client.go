package provider

import (
	"context"
	"fmt"

	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client provider RPC 客户端
type Client struct {
	conn   *grpc.ClientConn
	client providerpb.ProviderServiceClient
}

// NewClient 创建新的 provider 客户端
func NewClient(host string, port int) (*Client, error) {
	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to provider service: %w", err)
	}

	return &Client{
		conn:   conn,
		client: providerpb.NewProviderServiceClient(conn),
	}, nil
}

// Close 关闭客户端连接
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// RegisterProvider 注册 provider 并获取分配的 ID
func (c *Client) RegisterProvider(ctx context.Context, name, host, providerType string, port int32, config string) (string, error) {
	req := &providerpb.RegisterProviderRequest{
		Name:   name,
		Host:   host,
		Port:   port,
		Type:   providerType,
		Config: config,
	}

	resp, err := c.client.RegisterProvider(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to register provider: %w", err)
	}

	if !resp.Success {
		return "", fmt.Errorf("provider registration failed: %s", resp.Error)
	}

	return resp.ProviderId, nil
}
