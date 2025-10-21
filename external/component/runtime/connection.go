package runtime

import (
	"context"

	"github.com/9triver/ignis/actor/remote"
	"github.com/9triver/ignis/proto/executor"
	"github.com/sirupsen/logrus"
	pb "google.golang.org/protobuf/proto"
	"gopkg.in/zeromq/goczmq.v4"
)

type Connection struct {
	router         *goczmq.Channeler
	connId         string
	done           chan struct{}
	executorStream *remote.ExecutorImpl
}

func NewConnection(addr string, connId string) *Connection {
	// 创建 ZeroMQ router，添加错误处理
	router := goczmq.NewRouterChanneler(addr)
	if router == nil {
		logrus.Fatalf("Failed to create ZeroMQ router for address: %s", addr)
		return nil
	}

	// 验证 router 是否正常工作
	if router.SendChan == nil || router.RecvChan == nil {
		logrus.Fatalf("ZeroMQ router channels are nil for address: %s", addr)
		return nil
	}

	logrus.Infof("Successfully created ZeroMQ router for address: %s", addr)

	return &Connection{
		router:         router,
		connId:         connId,
		done:           make(chan struct{}),
		executorStream: remote.NewExecutorImpl(connId, remote.IPC),
	}
}

func (c *Connection) Id() string {
	return c.connId
}

func (c *Connection) Ready() <-chan struct{} {
	return c.executorStream.Ready()
}

func (c *Connection) SendChan() chan<- *executor.Message {
	return c.executorStream.SendChan()
}

func (c *Connection) RecvChan() <-chan *executor.Message {
	return c.executorStream.RecvChan()
}

func (c *Connection) Send(msg *executor.Message) error {
	data, err := pb.Marshal(msg)
	if err != nil {
		return err
	}
	c.router.SendChan <- [][]byte{[]byte(c.connId), data}
	return nil
}

func (c *Connection) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
			return nil
		case msg := <-c.router.RecvChan:
			if len(msg) < 2 {
				continue
			}
			frame, data := msg[0], msg[1]
			cmd := &executor.Message{}
			if err := pb.Unmarshal(data, cmd); err != nil {
				continue
			}
			c.onReceive(frame, cmd)
		}
	}
}

func (c *Connection) onReceive(frame []byte, msg *executor.Message) {
	conn := msg.Conn
	if conn != c.connId {
		logrus.Warnf("connection %s received message for connection %s", c.connId, conn)
		return
	}

	switch msg.Command.(type) {
	case *executor.Message_Ready:
		c.executorStream.SetSender(func(msg *executor.Message) error {
			data, err := pb.Marshal(msg)
			if err != nil {
				return err
			}
			c.router.SendChan <- [][]byte{frame, data}
			return nil
		})
	case *executor.Message_Return, *executor.Message_StreamChunk:
		c.executorStream.Produce(msg)
	}
}

func (c *Connection) Stop(ctx context.Context) error {
	close(c.done)
	c.executorStream.Close()
	c.router.Destroy()
	return nil
}
