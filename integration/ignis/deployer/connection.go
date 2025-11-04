package deployer

import (
	"context"
	"io"

	"github.com/9triver/ignis/actor/remote"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type ClusterStreamImpl = remote.StreamImpl[*cluster.Message, *cluster.Message]

func NewClusterStreamImpl(conn string, protocol remote.Protocol) *ClusterStreamImpl {
	return remote.NewStreamImpl[*cluster.Message, *cluster.Message](conn, protocol)
}

type ConnectionManager struct {
	cluster.UnimplementedServiceServer
	computers map[string]*ClusterStreamImpl
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		computers: make(map[string]*ClusterStreamImpl),
	}
}

func (cm *ConnectionManager) Session(stream grpc.BidiStreamingServer[cluster.Message, cluster.Message]) error {
	msg, err := stream.Recv()
	if err != nil {
		return err
	}
	if _, ok := msg.Message.(*cluster.Message_Ready); !ok {
		logrus.Errorf("expect ready message, but got %+v", msg)
		return nil
	}
	c, ok := cm.computers[msg.ConnID]
	if !ok {
		logrus.Errorf("compute session %s not found", msg.ConnID)
		return nil
	}
	c.SetSender(stream.Send)
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		c.Produce(msg)
	}
}

func (cm *ConnectionManager) NewConn(ctx context.Context, connId string) *ClusterStreamImpl {
	if c, ok := cm.computers[connId]; ok {
		return c
	}

	c := remote.NewComputeStreamImpl(connId, remote.RPC)
	go c.Run(ctx)
	cm.computers[connId] = c
	return c
}

func (cm *ConnectionManager) Close() {
	for _, c := range cm.computers {
		_ = c.Close()
	}
}
