package zmq

import (
	"context"

	"github.com/9triver/iarnet/internal/ignis/transport"
)

// Client 为占位实现，后续接 ZeroMQ 实现。
type Client struct{}

func NewClient() *Client { return &Client{} }

func (c *Client) Send(ctx context.Context, addr string, env transport.Envelope) error {
	return nil
}

func (c *Client) Request(ctx context.Context, addr string, env transport.Envelope) (transport.Envelope, error) {
	return transport.Envelope{}, nil
}
