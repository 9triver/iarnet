package component

import (
	"context"
	"sync"

	clusterpb "github.com/9triver/iarnet/internal/proto/ignis/cluster"
	"github.com/9triver/iarnet/internal/resource/types"
)

type Sender func(componentID string, msg *clusterpb.Message)

type Component struct {
	mu            sync.RWMutex
	id            string
	name          string
	image         string
	resourceUsage *types.Info
	buffer        chan *clusterpb.Message
	sender        Sender
}

func NewComponent(id, name, image string, resourceUsage *types.Info) *Component {
	comp := &Component{
		id:            id,
		name:          name,
		image:         image,
		resourceUsage: resourceUsage,
		buffer:        make(chan *clusterpb.Message, 100), // Buffered channel to avoid blocking
	}

	return comp
}

func (c *Component) GetID() string {
	return c.id
}

func (c *Component) GetName() string {
	return c.name
}

func (c *Component) GetImage() string {
	return c.image
}

func (c *Component) SetSender(sender Sender) {
	c.sender = sender
}

func (c *Component) Send(msg *clusterpb.Message) {
	c.sender(c.id, msg)
}

func (c *Component) Receive(ctx context.Context) *clusterpb.Message {
	select {
	case <-ctx.Done():
		return nil
	case msg := <-c.buffer:
		return msg
	}
}

func (c *Component) Push(msg *clusterpb.Message) {
	c.buffer <- msg
}
