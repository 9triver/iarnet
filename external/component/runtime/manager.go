package runtime

import (
	"context"
	"sync"

	"github.com/9triver/ignis/configs"
	"github.com/9triver/ignis/objects"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/proto/executor"
	"github.com/9triver/ignis/utils"
)

// Manager 抽象不同语言函数的运行时管理器
// 负责根据函数定义进行环境准备与可选的执行器启动
type Manager interface {
	// Language 返回该运行时支持的语言
	Language() proto.Language
	// Setup 根据函数定义准备运行环境
	Setup(fn *cluster.Function) error
	// Execute 执行指定对象的指定方法，返回 Future[objects.Interface]
	Execute(name, method string, args map[string]objects.Interface) utils.Future[objects.Interface]
	Run(fn *cluster.Function) error
}

type UnimplementedManager struct {
	mu      sync.Mutex
	ctx     context.Context
	addr    string
	conn    *Connection
	started bool
	futures map[string]utils.Future[objects.Interface]
	streams map[string]*objects.Stream
}

func NewUnimplementedManager(ctx context.Context, addr string, connId string) *UnimplementedManager {
	conn := NewConnection(addr, connId)
	return &UnimplementedManager{
		ctx:     ctx,
		addr:    addr,
		conn:    conn,
		started: false,
		futures: make(map[string]utils.Future[objects.Interface]),
		streams: make(map[string]*objects.Stream),
	}
}

func (um *UnimplementedManager) Addr() string {
	return um.addr
}

func (um *UnimplementedManager) Language() proto.Language {
	panic("unimplemented")
}

func (um *UnimplementedManager) Setup(fn *cluster.Function) error {
	panic("unimplemented")
}

func (um *UnimplementedManager) Execute(name, method string, args map[string]objects.Interface) utils.Future[objects.Interface] {
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
	um.futures[corrId] = fut

	msg := executor.NewExecute(um.conn.Id(), corrId, name, method, encoded)
	um.conn.SendChan() <- msg

	for _, arg := range args {
		if stream, ok := arg.(*objects.Stream); ok {
			chunks := stream.ToChan()
			go func() {
				defer func() {
					um.conn.SendChan() <- executor.NewStreamEnd(um.conn.Id(), stream.GetID())
				}()
				for chunk := range chunks {
					encoded, err := chunk.Encode()
					um.conn.SendChan() <- executor.NewStreamChunk(um.conn.Id(), stream.GetID(), encoded, err)
				}
			}()
		}
	}
	return fut
}

func (um *UnimplementedManager) onReturn(ret *executor.Return) {
	fut, ok := um.futures[ret.CorrID]
	if !ok {
		return
	}
	defer delete(um.futures, ret.CorrID)

	obj, err := ret.Object()
	if err != nil {
		fut.Reject(err)
		return
	}

	var o objects.Interface
	if obj.Stream { // return a stream from python
		values := make(chan objects.Interface)
		ls := objects.NewStream(values, obj.GetLanguage())
		um.streams[ret.CorrID] = ls
		o = ls
	} else {
		o = obj
	}
	fut.Resolve(o)
}

func (um *UnimplementedManager) onStreamChunk(chunk *proto.StreamChunk) {
	stream, ok := um.streams[chunk.StreamID]
	if !ok {
		return
	}

	if chunk.EoS {
		stream.EnqueueChunk(nil)
	} else {
		stream.EnqueueChunk(chunk.GetValue())
	}
}

func (um *UnimplementedManager) onReceive(msg *executor.Message) {
	switch cmd := msg.Command.(type) {
	case *executor.Message_StreamChunk:
		um.onStreamChunk(cmd.StreamChunk)
	case *executor.Message_Return:
		um.onReturn(cmd.Return)
	}
}

func (um *UnimplementedManager) Run(fn *cluster.Function) error {
	um.mu.Lock()
	defer um.mu.Unlock()
	if um.started {
		return nil
	}
	um.started = true

	go func() {
		for msg := range um.conn.RecvChan() {
			um.onReceive(msg)
		}
	}()
	if err := um.Setup(fn); err != nil {
		return err
	}
	addHandler := executor.NewAddHandler(
		um.conn.Id(), fn.Name,
		fn.PickledObject, fn.Language, nil,
	)
	um.conn.SendChan() <- addHandler
	<-um.conn.Ready()
	return nil
}
