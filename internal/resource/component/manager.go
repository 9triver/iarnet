package component

import (
	"context"
	"fmt"
	"sync"

	clusterpb "github.com/9triver/iarnet/internal/proto/ignis/cluster"
	"github.com/9triver/iarnet/internal/transport/zmq"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

type Manager interface {
	AddComponent(ctx context.Context, component *Component) error
	Run(ctx context.Context) error
}

type manager struct {
	mu         sync.RWMutex
	channeler  *zmq.ComponentChanneler
	components map[string]*Component
}

func NewManager(channeler *zmq.ComponentChanneler) Manager {
	return &manager{
		mu:         sync.RWMutex{},
		components: make(map[string]*Component),
		channeler:  channeler,
	}
}

func (m *manager) Run(ctx context.Context) error {
	// Start receiver that marks components as connected and processes messages
	m.channeler.StartReceiver(ctx, func(componentID string, data []byte) {
		m.mu.RLock()
		component, ok := m.components[componentID]
		m.mu.RUnlock()

		if !ok {
			logrus.Warnf("component %s not found", componentID)
			return
		}

		message := &clusterpb.Message{}
		if err := proto.Unmarshal(data, message); err != nil {
			logrus.Errorf("failed to unmarshal message: %v", err)
			return
		}

		if message.GetType() == clusterpb.MessageType_READY {
			// TODO: mark component as connected 暂时不用实现，请忽略
		}

		component.Push(message)
	})
	return nil
}

func (m *manager) AddComponent(ctx context.Context, component *Component) error {
	if component == nil {
		return fmt.Errorf("component is nil")
	}
	component.SetSender(func(componentID string, msg *clusterpb.Message) {
		data, err := proto.Marshal(msg)
		if err != nil {
			logrus.Errorf("failed to marshal message: %v, err: %v", msg, err)
			return
		}
		// Send message directly to ZMQ SendChan
		m.channeler.Send(componentID, data)
	})
	m.mu.Lock()
	defer m.mu.Unlock()
	m.components[component.GetID()] = component

	return nil
}
