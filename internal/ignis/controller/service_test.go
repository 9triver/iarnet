package controller

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	"google.golang.org/grpc/metadata"
)

func TestServiceSession_FirstMessageRequiresAppID(t *testing.T) {
	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()

	first := &ctrlpb.Message{
		Type: ctrlpb.CommandType_FR_REGISTER_REQUEST,
		// AppID intentionally left empty.
		Command: &ctrlpb.Message_RegisterRequest{
			RegisterRequest: &ctrlpb.RegisterRequest{},
		},
	}

	stream := newTestStream(streamCtx, streamCancel, []*ctrlpb.Message{first})

	var acquireCalled atomic.Bool
	mgr := &stubManager{
		acquire: func(appID string) (*Controller, func(), error) {
			acquireCalled.Store(true)
			return nil, nil, errors.New("should not be called")
		},
	}

	svc := &service{manager: mgr}
	err := svc.Session(stream)
	if err == nil || err.Error() != "application id is empty" {
		t.Fatalf("expected application id error, got %v", err)
	}
	if acquireCalled.Load() {
		t.Fatalf("manager should not be asked to acquire session when AppID is empty")
	}
}

func TestServiceSession_HandlesResponsesAndReleasesSession(t *testing.T) {
	parentCtx := context.Background()
	streamCtx, streamCancel := context.WithCancel(parentCtx)
	defer streamCancel()

	appID := "app-1"
	first := &ctrlpb.Message{
		Type:  ctrlpb.CommandType_FR_REGISTER_REQUEST,
		AppID: appID,
		Command: &ctrlpb.Message_RegisterRequest{
			RegisterRequest: &ctrlpb.RegisterRequest{ApplicationID: appID},
		},
	}
	second := &ctrlpb.Message{
		Type:  ctrlpb.CommandType_ACK,
		AppID: appID,
		Command: &ctrlpb.Message_Ack{
			Ack: &ctrlpb.Ack{},
		},
	}

	stream := newTestStream(streamCtx, streamCancel, []*ctrlpb.Message{first, second})
	controller := NewController(appID, nil)

	var acquireCalled atomic.Bool
	var releaseCalled atomic.Bool
	acquired := make(chan struct{})

	mgr := &stubManager{
		acquire: func(requestedAppID string) (*Controller, func(), error) {
			if requestedAppID != appID {
				return nil, nil, errors.New("unexpected app id")
			}
			if acquireCalled.Swap(true) {
				return nil, nil, errors.New("acquire called more than once")
			}
			close(acquired)
			return controller, func() { releaseCalled.Store(true) }, nil
		},
	}

	svc := &service{manager: mgr}

	done := make(chan error, 1)
	go func() {
		done <- svc.Session(stream)
	}()

	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for controller session acquisition")
	}

	response := &ctrlpb.Message{
		Type:  ctrlpb.CommandType_ACK,
		AppID: appID,
	}
	if err := controller.PushResponse(context.Background(), response); err != nil {
		t.Fatalf("push response: %v", err)
	}

	waitForCondition(t, time.Second, func() bool {
		return stream.SentCount() == 1
	})

	err := <-done
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF, got %v", err)
	}

	if !releaseCalled.Load() {
		t.Fatalf("expected release function to be invoked")
	}
}

type stubManager struct {
	acquire func(string) (*Controller, func(), error)
}

func (m *stubManager) CreateController(context.Context, string) (*Controller, error) {
	return nil, errors.New("not implemented")
}

func (m *stubManager) AcquireControllerSession(appID string) (*Controller, func(), error) {
	if m.acquire == nil {
		return nil, nil, errors.New("acquire not configured")
	}
	return m.acquire(appID)
}

func (m *stubManager) On(EventType, EventHandler) {}

type testStream struct {
	ctx     context.Context
	cancel  context.CancelFunc
	recvMu  sync.Mutex
	recvIdx int
	recvMsg []*ctrlpb.Message

	sendMu sync.Mutex
	sent   []*ctrlpb.Message
}

func newTestStream(ctx context.Context, cancel context.CancelFunc, messages []*ctrlpb.Message) *testStream {
	return &testStream{
		ctx:    ctx,
		cancel: cancel,
		recvMsg: func() []*ctrlpb.Message {
			out := make([]*ctrlpb.Message, len(messages))
			copy(out, messages)
			return out
		}(),
	}
}

func (s *testStream) Context() context.Context { return s.ctx }

func (s *testStream) Recv() (*ctrlpb.Message, error) {
	s.recvMu.Lock()
	defer s.recvMu.Unlock()

	if s.recvIdx < len(s.recvMsg) {
		msg := s.recvMsg[s.recvIdx]
		s.recvIdx++
		return msg, nil
	}

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	return nil, io.EOF
}

func (s *testStream) Send(msg *ctrlpb.Message) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.sent = append(s.sent, msg)
	return nil
}

func (s *testStream) SentCount() int {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return len(s.sent)
}

func (s *testStream) SetHeader(metadata.MD) error  { return nil }
func (s *testStream) SendHeader(metadata.MD) error { return nil }
func (s *testStream) SetTrailer(metadata.MD)       {}
func (s *testStream) SendMsg(interface{}) error    { return nil }
func (s *testStream) RecvMsg(interface{}) error    { return nil }

func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(1 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}
