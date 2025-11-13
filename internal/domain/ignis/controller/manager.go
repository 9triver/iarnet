package controller

import (
	"context"
	"errors"
	"sync"

	"github.com/9triver/iarnet/internal/domain/resource/component"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Manager interface {
	ctrlpb.UnsafeServiceServer
	Session(stream grpc.BidiStreamingServer[ctrlpb.Message, ctrlpb.Message]) error
	AddController(controller *Controller) error
	On(eventType EventType, handler EventHandler)
}

type manager struct {
	ctrlpb.UnimplementedServiceServer

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

func (m *manager) AddController(controller *Controller) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.controllers[controller.AppID()]; ok {
		return errors.New("controller already exists")
	}
	controller.SetEvents(m.events)
	m.controllers[controller.AppID()] = controller
	return nil
}

func (m *manager) On(eventType EventType, handler EventHandler) {
	m.events.Subscribe(eventType, handler)
}

func (m *manager) Session(stream grpc.BidiStreamingServer[ctrlpb.Message, ctrlpb.Message]) error {
	ctx := stream.Context()
	var controller *Controller
	var expectedAppID string
	var toClientErrCh chan error
	var cancelToClient context.CancelFunc

	firstMsg, err := stream.Recv()
	if err != nil {
		logrus.Errorf("failed to receive first message: %v", err)
		return err
	}

	expectedAppID = firstMsg.GetAppID()
	if expectedAppID == "" {
		logrus.Errorf("application id is empty")
		return errors.New("application id is empty")
	}

	controller = m.controllers[expectedAppID]
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
		err := forwardResponses(toClientCtx, toClientChan, stream)
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

		msg, err := stream.Recv()
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

		// TODO: 保证同一个 session 流只对应一个应用
		// TODO: 添加鉴权机制，保证发起请求的客户端有权限访问该应用

		if err := controller.HandleClientMessage(ctx, msg); err != nil {
			return err
		}
	}
}

func forwardResponses(ctx context.Context, toClientChan <-chan *ctrlpb.Message, stream grpc.BidiStreamingServer[ctrlpb.Message, ctrlpb.Message]) error {
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
			if err := stream.Send(msg); err != nil {
				return err
			}
		}
	}
}
