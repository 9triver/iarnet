package controller

import (
	"context"
	"errors"

	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Service = ctrlpb.ServiceServer

type service struct {
	ctrlpb.UnimplementedServiceServer
	manager Manager
}

func NewService(manager Manager) Service {
	return &service{
		manager: manager,
	}
}

func (s *service) Session(stream grpc.BidiStreamingServer[ctrlpb.Message, ctrlpb.Message]) error {
	ctx := stream.Context()
	var controller *Controller
	var releaseSession func()
	var expectedAppID string
	var responseErrCh chan error
	var cancelResponse context.CancelFunc

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

	controller, release, err := s.manager.AcquireControllerSession(expectedAppID)
	if err != nil {
		logrus.Errorf("failed to acquire controller session: %v", err)
		return err
	}
	releaseSession = release

	responseErrCh = make(chan error, 1)
	var responseCtx context.Context
	responseCtx, cancelResponse = context.WithCancel(ctx)
	go func() {
		responseErrCh <- forwardResponses(responseCtx, controller, stream)
	}()

	defer func() {
		releaseSession()
		cancelResponse()
		if err := <-responseErrCh; err != nil && !errors.Is(err, context.Canceled) {
			logrus.WithError(err).Warn("controller response loop exited with error")
		}
	}()

	if err := controller.HandleMessage(ctx, firstMsg); err != nil {
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			logrus.Errorf("context error: %v", err)
			return err
		}

		if responseErrCh != nil {
			select {
			case err := <-responseErrCh:
				if err != nil && !errors.Is(err, context.Canceled) {
					logrus.Errorf("response error: %v", err)
					return err
				}
			default:
			}
		}

		msg, err := stream.Recv()
		if err != nil {
			logrus.Errorf("failed to receive message: %v", err)
			if responseErrCh != nil {
				if forwardErr := <-responseErrCh; forwardErr != nil && !errors.Is(forwardErr, context.Canceled) {
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

		if err := controller.HandleMessage(ctx, msg); err != nil {
			return err
		}
	}
}

func forwardResponses(ctx context.Context, controller *Controller, stream grpc.BidiStreamingServer[ctrlpb.Message, ctrlpb.Message]) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-controller.ResponseChan():
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
