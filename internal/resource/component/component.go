package component

import (
	"context"
	"sync"

	actorpb "github.com/9triver/iarnet/internal/proto/execution_ignis/actor"
	"github.com/9triver/iarnet/internal/resource/types"
)

type Sender func(componentID string, msg *actorpb.Message)

type Component struct {
	mu            sync.RWMutex
	id            string
	name          string
	image         string
	resourceUsage *types.Info
	buffer        chan *actorpb.Message
	sender        Sender
}

func NewComponent(id, name, image string, resourceUsage *types.Info) *Component {
	comp := &Component{
		id:            id,
		name:          name,
		image:         image,
		resourceUsage: resourceUsage,
		buffer:        make(chan *actorpb.Message, 100), // Buffered channel to avoid blocking
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

func (c *Component) Send(msg *actorpb.Message) {
	c.sender(c.id, msg)
}

func (c *Component) Receive(ctx context.Context) *actorpb.Message {
	select {
	case <-ctx.Done():
		return nil
	case msg := <-c.buffer:
		return msg
	}
}

func (c *Component) Push(msg *actorpb.Message) {
	c.buffer <- msg
}
