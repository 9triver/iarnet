package controller

import (
	"context"
	"errors"
	"sync"
)

type Manager interface {
	CreateController(ctx context.Context, appID string) (*Controller, error)
	GetController(appID string) (*Controller, error)
	AcquireControllerSession(appID string) (*Controller, func(), error)
	On(eventType EventType, handler EventHandler)
}

type manager struct {
	mu          sync.RWMutex
	controllers map[string]*controllerEntry
	events      *EventHub
}

type controllerEntry struct {
	controller    *Controller
	sessionActive bool
}

func NewManager() Manager {
	return &manager{
		controllers: make(map[string]*controllerEntry),
		events:      NewEventHub(),
	}
}

func (m *manager) CreateController(ctx context.Context, appID string) (*Controller, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	controller := NewController(appID, m.events)
	m.controllers[appID] = &controllerEntry{
		controller: controller,
	}
	return controller, nil
}

func (m *manager) On(eventType EventType, handler EventHandler) {
	m.events.Subscribe(eventType, handler)
}

func (m *manager) GetController(appID string) (*Controller, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.controllers[appID]
	if !ok {
		return nil, errors.New("controller not found")
	}
	return entry.controller, nil
}

func (m *manager) AcquireControllerSession(appID string) (*Controller, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.controllers[appID]
	if !ok {
		return nil, nil, errors.New("controller not found")
	}

	if entry.sessionActive {
		return nil, nil, errSessionAlreadyActive
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
