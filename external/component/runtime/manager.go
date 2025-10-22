package runtime

import (
	"context"
	"sync"

	"github.com/9triver/ignis/actor/remote"
	"github.com/9triver/ignis/configs"
	"github.com/9triver/ignis/objects"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/proto/executor"
	"github.com/9triver/ignis/utils"
)

type ExecutorStream = remote.Executor

type Manager struct {
	mu      sync.Mutex
	conn    *Connection
	started bool
	futures map[string]utils.Future[objects.Interface]
	streams map[string]*objects.Stream
}

func NewManager() *Manager {
	return &Manager{
		started: false,
		futures: make(map[string]utils.Future[objects.Interface]),
		streams: make(map[string]*objects.Stream),
	}
}

func (m *Manager) Execute(name, method string, args map[string]objects.Interface) utils.Future[objects.Interface] {
	fut := utils.NewFuture[objects.Interface](configs.ExecutionTimeout)
	encoded := make(map[string]*objects.Remote)
	for param, obj := range args {
		enc, err := obj.Encode()
		if err != nil {
			fut.Reject(err)
			return fut
		}
		encoded[param] = enc
	}

	corrId := utils.GenID()
	m.futures[corrId] = fut

	msg := executor.NewExecute(m.conn.Id(), corrId, name, method, encoded)
	m.conn.SendChan() <- msg

	for _, arg := range args {
		if stream, ok := arg.(*objects.Stream); ok {
			chunks := stream.ToChan()
			go func() {
				defer func() {
					m.conn.SendChan() <- executor.NewStreamEnd(m.conn.Id(), stream.GetID())
				}()
				for chunk := range chunks {
					encoded, err := chunk.Encode()
					m.conn.SendChan() <- executor.NewStreamChunk(m.conn.Id(), stream.GetID(), encoded, err)
				}
			}()
		}
	}
	return fut
}

func (m *Manager) onReturn(ret *executor.Return) {
	fut, ok := m.futures[ret.CorrID]
	if !ok {
		return
	}
	defer delete(m.futures, ret.CorrID)

	obj, err := ret.Object()
	if err != nil {
		fut.Reject(err)
		return
	}

	var o objects.Interface
	if obj.Stream { // return a stream from python
		values := make(chan objects.Interface)
		ls := objects.NewStream(values, obj.GetLanguage())
		m.streams[ret.CorrID] = ls
		o = ls
	} else {
		o = obj
	}
	fut.Resolve(o)
}

func (m *Manager) onStreamChunk(chunk *proto.StreamChunk) {
	stream, ok := m.streams[chunk.StreamID]
	if !ok {
		return
	}

	if chunk.EoS {
		stream.EnqueueChunk(nil)
	} else {
		stream.EnqueueChunk(chunk.GetValue())
	}
}

func (m *Manager) onReceive(msg *executor.Message) {
	switch cmd := msg.Command.(type) {
	case *executor.Message_StreamChunk:
		m.onStreamChunk(cmd.StreamChunk)
	case *executor.Message_Return:
		m.onReturn(cmd.Return)
	}
}

func (m *Manager) Run(ctx context.Context, conn *Connection, fn *cluster.Function, initializer Initializer) (*Funciton, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return nil, nil
	}
	m.started = true

	m.conn = conn

	go func() {
		for msg := range m.conn.RecvChan() {
			m.onReceive(msg)
		}
	}()
	if err := initializer.Initialize(ctx, fn, conn.Addr(), conn.Id()); err != nil {
		return nil, err
	}
	addHandler := executor.NewAddHandler(
		m.conn.Id(), fn.Name,
		fn.PickledObject, fn.Language, nil,
	)
	<-m.conn.Ready()

	m.conn.SendChan() <- addHandler
	return NewFunciton(m, fn), nil
}
