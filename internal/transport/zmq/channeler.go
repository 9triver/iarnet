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
	closed          bool                // whether the channeler is closed
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
	cc.mu.Lock()
	if cc.closed {
		cc.mu.Unlock()
		return nil
	}
	cc.closed = true
	ch := cc.Channeler
	cc.mu.Unlock()

	if ch != nil {
		// Destroy the channeler which will close the underlying socket and release the port
		ch.Destroy()
		cc.mu.Lock()
		cc.Channeler = nil
		cc.mu.Unlock()
		logrus.Info("ZMQ Channeler closed and resources released")
	}
	return nil
}

// Send queues messages for unconnected components, sends immediately for connected ones
func (cc *ComponentChanneler) Send(componentID string, data []byte) {
	logrus.Infof("Send: component %s, data size: %d", componentID, len(data))
	cc.mu.Lock()
	if cc.closed || cc.Channeler == nil {
		cc.mu.Unlock()
		logrus.Warnf("Attempted to send message to component %s but channeler is closed", componentID)
		return
	}
	connected := cc.connected[componentID]
	logrus.Debugf("Send: component %s connected=%v, pending=%d", componentID, connected, len(cc.pendingMessages[componentID]))

	if connected {
		// Component is connected, send immediately
		ch := cc.Channeler
		cc.mu.Unlock()

		logrus.Infof("Component %s is connected, sending message immediately", componentID)
		select {
		case ch.SendChan <- [][]byte{[]byte(componentID), data}:
			logrus.Debugf("Sent message to component %s via ZMQ SendChan", componentID)
		default:
			logrus.Warnf("Failed to send message to component %s: SendChan is full or closed", componentID)
		}
	} else {
		// Component not connected yet, queue the message
		cc.pendingMessages[componentID] = append(cc.pendingMessages[componentID], data)
		pendingCount := len(cc.pendingMessages[componentID])
		cc.mu.Unlock()
		logrus.Infof("Queued message for component %s (pending: %d)", componentID, pendingCount)
	}
}

// MarkConnected marks a component as connected and sends all pending messages
func (cc *ComponentChanneler) MarkConnected(componentID string) {
	cc.mu.Lock()

	if cc.closed || cc.Channeler == nil {
		cc.mu.Unlock()
		logrus.Debugf("MarkConnected: channeler is closed for component %s", componentID)
		return // Channeler is closed, don't process
	}

	if cc.connected[componentID] {
		cc.mu.Unlock()
		logrus.Debugf("MarkConnected: component %s already connected", componentID)
		return // Already connected
	}

	cc.connected[componentID] = true
	logrus.Infof("MarkConnected: component %s marked as connected", componentID)

	// Get pending messages while holding lock
	var pending [][]byte
	if p, ok := cc.pendingMessages[componentID]; ok {
		pending = make([][]byte, len(p))
		copy(pending, p)
		delete(cc.pendingMessages, componentID)
	}
	cc.mu.Unlock()

	if len(pending) > 0 {
		logrus.Infof("Component %s connected, flushing %d pending messages", componentID, len(pending))

		// Send all pending messages in a goroutine to avoid blocking the receiver
		go func(compID string, msgs [][]byte) {
			for i, data := range msgs {
				cc.mu.RLock()
				closed := cc.closed
				ch := cc.Channeler
				cc.mu.RUnlock()

				if closed || ch == nil {
					logrus.Warnf("Channeler closed while sending pending messages to component %s", compID)
					return
				}

				select {
				case ch.SendChan <- [][]byte{[]byte(compID), data}:
					logrus.Debugf("Sent pending message %d/%d to component %s", i+1, len(msgs), compID)
				default:
					logrus.Warnf("Failed to send pending message %d/%d to component %s: SendChan is full or closed", i+1, len(msgs), compID)
					return
				}
			}
			logrus.Infof("Finished sending all %d pending messages to component %s", len(msgs), compID)
		}(componentID, pending)
	}
}

// StartReceiver starts a goroutine that processes received messages from components
func (cc *ComponentChanneler) StartReceiver(ctx context.Context, onMessage func(componentID string, data []byte)) {
	go func() {
		// 检查 Channeler 是否已初始化
		cc.mu.RLock()
		channeler := cc.Channeler
		cc.mu.RUnlock()

		if channeler == nil {
			logrus.Error("ZMQ Channeler is nil, cannot start receiver")
			return
		}

		logrus.Info("ZMQ receiver started, waiting for component connections...")
		for {
			select {
			case <-ctx.Done():
				logrus.Info("ZMQ receiver stopped")
				return
			case msg, ok := <-channeler.RecvChan:
				if !ok {
					logrus.Info("ZMQ RecvChan closed")
					return
				}
				if len(msg) < 2 {
					logrus.Warnf("Received invalid message format, expected at least 2 frames, got %d", len(msg))
					continue
				}
				componentID := string(msg[0])
				data := msg[1]
				logrus.Infof("Received message from component %s (size: %d bytes)", componentID, len(data))

				// Mark component as connected and flush pending messages
				// This must be called before processing the message to ensure
				// that subsequent Send() calls see the component as connected
				cc.MarkConnected(componentID)

				// Call the callback
				if onMessage != nil {
					onMessage(componentID, data)
				}
			}
		}
	}()
}
