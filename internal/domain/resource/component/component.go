package component

import (
	"context"
	"sync"

	"github.com/9triver/iarnet/internal/domain/resource/types"
	componentpb "github.com/9triver/iarnet/internal/proto/resource/component"
)

type Sender func(componentID string, msg *componentpb.Message)

type Component struct {
	mu            sync.RWMutex
	id            string
	providerID    string
	image         string
	resourceUsage *types.Info
	buffer        chan *componentpb.Message
	sender        Sender
}

func NewComponent(id, image string, resourceUsage *types.Info) *Component {
	comp := &Component{
		id:            id,
		image:         image,
		resourceUsage: resourceUsage,
		buffer:        make(chan *componentpb.Message, 100), // Buffered channel to avoid blocking
	}

	return comp
}

func (c *Component) GetProviderID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.providerID
}

func (c *Component) SetProviderID(providerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providerID = providerID
}

func (c *Component) GetID() string {
	return c.id
}

func (c *Component) GetImage() string {
	return c.image
}

func (c *Component) GetResourceUsage() *types.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.resourceUsage
}

func (c *Component) SetSender(sender Sender) {
	c.sender = sender
}

func (c *Component) Send(msg *componentpb.Message) {
	c.sender(c.id, msg)
}

func (c *Component) Receive(ctx context.Context) *componentpb.Message {
	select {
	case <-ctx.Done():
		return nil
	case msg := <-c.buffer:
		return msg
	}
}

func (c *Component) Push(msg *componentpb.Message) {
	c.buffer <- msg
}
