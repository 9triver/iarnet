package runtime

import (
	"github.com/9triver/ignis/actor/remote"
	"github.com/9triver/ignis/proto/executor"
	"github.com/sirupsen/logrus"
)

type Connection struct {
	addr           string
	connId         string
	executorStream remote.Executor
}

func NewConnection(addr string, connId string, stream remote.Executor) *Connection {
	logrus.Infof("Creating connection for address: %s, connId: %s", addr, connId)

	return &Connection{
		addr:           addr,
		connId:         connId,
		executorStream: stream,
	}
}

func (c *Connection) Id() string {
	return c.connId
}

func (c *Connection) Addr() string {
	return c.addr
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
