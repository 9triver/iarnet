package controller

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/9triver/iarnet/internal/domain/resource/component"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	"github.com/sirupsen/logrus"
)

type Manager interface {
	Add(controller *Controller) error
	Get(appID string) *Controller
	Remove(appID string) error
	On(eventType EventType, handler EventHandler)
	HandleSession(ctx context.Context, recv func() (*ctrlpb.Message, error), send func(*ctrlpb.Message) error) error
}

type manager struct {
	mu               sync.RWMutex
	componentService component.Service
	controllers      map[string]*Controller
	events           *EventHub
}

func NewManager(componentService component.Service) Manager {
	return &manager{
		componentService: componentService,
		controllers:      make(map[string]*Controller),
		events:           NewEventHub(),
	}
}

func (m *manager) Add(controller *Controller) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.controllers[controller.AppID()]; ok {
		return errors.New("controller already exists")
	}
	controller.SetEvents(m.events)
	m.controllers[controller.AppID()] = controller
	return nil
}

func (m *manager) Get(appID string) *Controller {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.controllers[appID]
}

func (m *manager) Remove(appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.controllers[appID]; !ok {
		return fmt.Errorf("controller for app %s not found", appID)
	}
	delete(m.controllers, appID)
	logrus.Infof("Removed controller for application %s", appID)
	return nil
}

func (m *manager) On(eventType EventType, handler EventHandler) {
	m.events.Subscribe(eventType, handler)
}

func (m *manager) HandleSession(ctx context.Context, recv func() (*ctrlpb.Message, error), send func(*ctrlpb.Message) error) error {
	var controller *Controller
	var expectedAppID string
	var toClientErrCh chan error
	var cancelToClient context.CancelFunc

	firstMsg, err := recv()
	if err != nil {
		logrus.Errorf("failed to receive first message: %v", err)
		return err
	}

	expectedAppID = firstMsg.GetAppID()
	if expectedAppID == "" {
		logrus.Errorf("application id is empty")
		return errors.New("application id is empty")
	}

	m.mu.RLock()
	controller = m.controllers[expectedAppID]
	m.mu.RUnlock()
	if controller == nil {
		logrus.Errorf("controller not found")
		return errors.New("controller not found")
	}

	if controller.GetToClientChan() != nil {
		logrus.Errorf("session already exists for application %s", expectedAppID)
		return errors.New("session already exists for application " + expectedAppID)
	}

	toClientChan := make(chan *ctrlpb.Message, 100)
	controller.SetToClientChan(toClientChan)

	toClientErrCh = make(chan error, 1)
	var toClientCtx context.Context
	toClientCtx, cancelToClient = context.WithCancel(ctx)

	go func() {
		err := forwardResponses(toClientCtx, toClientChan, send)
		toClientErrCh <- err
		close(toClientErrCh)
	}()

	defer func() {
		controller.ClearToClientChan()
		close(toClientChan)
		cancelToClient()
		if toClientErrCh != nil {
			for err := range toClientErrCh {
				if err != nil && !errors.Is(err, context.Canceled) {
					logrus.WithError(err).Warn("controller response loop exited with error")
				}
			}
		}
	}()

	if err := controller.HandleClientMessage(ctx, firstMsg); err != nil {
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			logrus.Errorf("context error: %v", err)
			return err
		}

		if toClientErrCh != nil {
			select {
			case err, ok := <-toClientErrCh:
				if ok {
					if err != nil && !errors.Is(err, context.Canceled) {
						logrus.Errorf("response error: %v", err)
						return err
					}
				}
				toClientErrCh = nil
			default:
			}
		}

		msg, err := recv()
		if err != nil {
			logrus.Errorf("failed to receive message: %v", err)
			if toClientErrCh != nil {
				if forwardErr := <-toClientErrCh; forwardErr != nil && !errors.Is(forwardErr, context.Canceled) {
					logrus.Errorf("forward error: %v", forwardErr)
					return forwardErr
				}
			}
			return err
		}

		appID := msg.GetAppID()
		if appID == "" {
			logrus.Errorf("application id is empty")
			return errors.New("application id is empty")
		}

		if appID != expectedAppID {
			logrus.Errorf("controller app id mismatch: %s != %s", expectedAppID, appID)
			return errors.New("controller app id mismatch")
		}

		if err := controller.HandleClientMessage(ctx, msg); err != nil {
			return err
		}
	}
}

func forwardResponses(ctx context.Context, toClientChan <-chan *ctrlpb.Message, send func(*ctrlpb.Message) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-toClientChan:
			if !ok {
				return nil
			}
			if msg == nil {
				continue
			}
			if err := send(msg); err != nil {
				return err
			}
		}
	}
}
