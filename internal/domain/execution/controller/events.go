package controller

import (
	"context"
	"sync"
)

type EventType string

const (
	EventTypeDAGNodeStatusChanged EventType = "dag_node_status_changed"
)

type Event interface {
	Type() EventType
}

type EventHandler func(context.Context, Event)

type EventHub struct {
	mu       sync.RWMutex
	handlers map[EventType][]EventHandler
}

func NewEventHub() *EventHub {
	return &EventHub{
		handlers: make(map[EventType][]EventHandler),
	}
}

func (h *EventHub) Subscribe(eventType EventType, handler EventHandler) {
	if handler == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.handlers[eventType] = append(h.handlers[eventType], handler)
}

func (h *EventHub) Publish(ctx context.Context, event Event) {
	if h == nil || event == nil {
		return
	}

	h.mu.RLock()
	handlers := append([]EventHandler(nil), h.handlers[event.Type()]...)
	h.mu.RUnlock()

	for _, handler := range handlers {
		handler(ctx, event)
	}
}

type DAGNodeStatus string

const (
	DAGNodeStatusDone DAGNodeStatus = "done"
)

type DAGNodeStatusChangedEvent struct {
	AppID     string
	NodeID    string
	SessionID string
	Status    DAGNodeStatus
}

// Type 实现 Event 接口。
func (e *DAGNodeStatusChangedEvent) Type() EventType {
	return EventTypeDAGNodeStatusChanged
}
