package component

import (
	"context"
	"fmt"
	"sync"

	componentpb "github.com/9triver/iarnet/internal/proto/resource/component"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

type Manager interface {
	AddComponent(ctx context.Context, component *Component) error
	Start(ctx context.Context) error
	SetChanneler(channeler Channeler) // 用于后续注入真正的 channeler
}

type manager struct {
	mu         sync.RWMutex
	channeler  Channeler // 使用接口而不是具体实现
	components map[string]*Component
}

func NewManager(channeler Channeler) Manager {
	return &manager{
		mu:         sync.RWMutex{},
		components: make(map[string]*Component),
		channeler:  channeler,
	}
}

func (m *manager) Start(ctx context.Context) error {
	// Start receiver that marks components as connected and processes messages
	m.channeler.StartReceiver(ctx, func(componentID string, data []byte) {
		m.mu.RLock()
		component, ok := m.components[componentID]
		m.mu.RUnlock()

		if !ok {
			logrus.Warnf("component %s not found", componentID)
			return
		}

		message := &componentpb.Message{}
		if err := proto.Unmarshal(data, message); err != nil {
			logrus.Errorf("failed to unmarshal message: %v", err)
			return
		}

		if message.GetType() == componentpb.MessageType_READY {
			// TODO: mark component as connected 暂时不用实现，请忽略
		} else {
			component.Push(message)
		}
	})
	return nil
}

func (m *manager) AddComponent(ctx context.Context, component *Component) error {
	if component == nil {
		return fmt.Errorf("component is nil")
	}
	component.SetSender(func(componentID string, msg *componentpb.Message) {
		data, err := proto.Marshal(msg)
		if err != nil {
			logrus.Errorf("failed to marshal message: %v, err: %v", msg, err)
			return
		}
		m.channeler.Send(componentID, data)
	})
	m.mu.Lock()
	defer m.mu.Unlock()
	m.components[component.GetID()] = component

	return nil
}

func (m *manager) SetChanneler(channeler Channeler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channeler = channeler
}
