package zmq

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/zeromq/goczmq.v4"
)

// ComponentChanneler wraps goczmq.Channeler for component communication
// It provides a Router socket that components (Dealer) can connect to
// Messages for unconnected components are queued and sent when they connect
type ComponentChanneler struct {
	*goczmq.Channeler
	mu              sync.RWMutex
	pendingMessages map[string][][]byte // component ID -> pending messages
	connected       map[string]bool     // component ID -> connected status
}

func NewChanneler(port int) *ComponentChanneler {
	base := goczmq.NewRouterChanneler(fmt.Sprintf("tcp://*:%d", port))
	return &ComponentChanneler{
		Channeler:       base,
		pendingMessages: make(map[string][][]byte),
		connected:       make(map[string]bool),
	}
}

// Close destroys the ZMQ Channeler and releases all resources
func (cc *ComponentChanneler) Close() error {
	if cc.Channeler != nil {
		cc.Channeler.Destroy()
		cc.Channeler = nil
		logrus.Info("ZMQ Channeler closed and resources released")
	}
	return nil
}

// Send queues messages for unconnected components, sends immediately for connected ones
func (cc *ComponentChanneler) Send(componentID string, data []byte) {
	cc.mu.RLock()
	connected := cc.connected[componentID]
	cc.mu.RUnlock()

	if connected {
		// Component is connected, send immediately
		cc.Channeler.SendChan <- [][]byte{[]byte(componentID), data}
		logrus.Debugf("Sent message to component %s via ZMQ SendChan", componentID)
	} else {
		// Component not connected yet, queue the message
		cc.mu.Lock()
		cc.pendingMessages[componentID] = append(cc.pendingMessages[componentID], data)
		pendingCount := len(cc.pendingMessages[componentID])
		cc.mu.Unlock()
		logrus.Infof("Queued message for component %s (pending: %d)", componentID, pendingCount)
	}
}

// MarkConnected marks a component as connected and sends all pending messages
func (cc *ComponentChanneler) MarkConnected(componentID string) {
	cc.mu.Lock()

	if cc.connected[componentID] {
		cc.mu.Unlock()
		return // Already connected
	}

	cc.connected[componentID] = true

	// Get pending messages while holding lock
	var pending [][]byte
	if p, ok := cc.pendingMessages[componentID]; ok {
		pending = make([][]byte, len(p))
		copy(pending, p)
		delete(cc.pendingMessages, componentID)
	}
	cc.mu.Unlock()

	logrus.Infof("Component %s connected, flushing %d pending messages", componentID, len(pending))

	// Send all pending messages in a goroutine to avoid blocking the receiver
	go func(compID string, msgs [][]byte) {
		for i, data := range msgs {
			cc.Channeler.SendChan <- [][]byte{[]byte(compID), data}
			logrus.Debugf("Sent pending message %d/%d to component %s", i+1, len(msgs), compID)
		}
		logrus.Infof("Finished sending all %d pending messages to component %s", len(msgs), compID)
	}(componentID, pending)
}

// StartReceiver starts a goroutine that processes received messages from components
func (cc *ComponentChanneler) StartReceiver(ctx context.Context, onMessage func(componentID string, data []byte)) {
	go func() {
		logrus.Info("ZMQ receiver started, waiting for component connections...")
		for {
			select {
			case <-ctx.Done():
				logrus.Info("ZMQ receiver stopped")
				return
			case msg := <-cc.Channeler.RecvChan:
				if len(msg) < 2 {
					logrus.Warnf("Received invalid message format, expected at least 2 frames, got %d", len(msg))
					continue
				}
				componentID := string(msg[0])
				data := msg[1]
				logrus.Infof("Received message from component %s (size: %d bytes)", componentID, len(data))

				// Mark component as connected and flush pending messages
				cc.MarkConnected(componentID)

				// Call the callback
				if onMessage != nil {
					onMessage(componentID, data)
				}
			}
		}
	}()
}
