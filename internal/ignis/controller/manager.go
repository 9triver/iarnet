package controller

import (
	"context"
	"errors"
	"sync"

	"github.com/9triver/iarnet/internal/resource"
)

type Manager interface {
	CreateController(ctx context.Context, appID string) (*Controller, error)
	AcquireControllerSession(appID string) (*Controller, func(), error)
	On(eventType EventType, handler EventHandler)
}

type manager struct {
	mu               sync.RWMutex
	componentService resource.ComponentService
	controllers      map[string]*controllerEntry
	events           *EventHub
}

type controllerEntry struct {
	controller    *Controller
	sessionActive bool
}

func NewManager(componentService resource.ComponentService) Manager {
	return &manager{
		componentService: componentService,
		controllers:      make(map[string]*controllerEntry),
		events:           NewEventHub(),
	}
}

func (m *manager) CreateController(ctx context.Context, appID string) (*Controller, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	controller := NewController(m.componentService, appID, m.events)
	m.controllers[appID] = &controllerEntry{
		controller: controller,
	}
	return controller, nil
}

func (m *manager) On(eventType EventType, handler EventHandler) {
	m.events.Subscribe(eventType, handler)
}

func (m *manager) AcquireControllerSession(appID string) (*Controller, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.controllers[appID]
	if !ok {
		return nil, nil, errors.New("controller not found")
	}

	if entry.sessionActive {
		return nil, nil, errors.New("controller session already active")
	}

	entry.sessionActive = true

	var once sync.Once
	release := func() {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			if entry, ok := m.controllers[appID]; ok {
				entry.sessionActive = false
			}
		})
	}

	return entry.controller, release, nil
}
