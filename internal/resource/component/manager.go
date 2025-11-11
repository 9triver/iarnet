package component

import (
	"context"
	"fmt"
	"sync"

	clusterpb "github.com/9triver/ignis/proto/cluster"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"gopkg.in/zeromq/goczmq.v4"
)

type Manager interface {
	AddComponent(ctx context.Context, component *Component) error
}

type manager struct {
	mu         sync.RWMutex
	channeler  *goczmq.Channeler
	components map[string]*Component
}

func NewManager(channeler *goczmq.Channeler) Manager {
	return &manager{
		mu:         sync.RWMutex{},
		components: make(map[string]*Component),
		channeler:  channeler,
	}
}

func (m *manager) Run(ctx context.Context) error {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-m.channeler.RecvChan:
				m.mu.RLock()
				defer m.mu.RUnlock()
				component, ok := m.components[string(msg[0])]
				if !ok {
					continue
				}
				message := &clusterpb.Message{}
				if err := proto.Unmarshal(msg[1], message); err != nil {
					continue
				}
				component.Push(message)
			}
		}
	}()
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
		m.channeler.SendChan <- [][]byte{[]byte(componentID), data}
	})
	m.mu.Lock()
	defer m.mu.Unlock()
	m.components[component.GetID()] = component

	return nil
}
