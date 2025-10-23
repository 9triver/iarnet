package stub

import (
	"github.com/9triver/ignis/configs"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
	"google.golang.org/grpc"
	pb "google.golang.org/protobuf/proto"
)

type RpcStub struct {
	send chan *cluster.Message
	recv chan pb.Message
}

func (s *RpcStub) SendTo(remoteRef *proto.StoreRef, msg pb.Message) {
	s.send <- cluster.NewMessage(msg)
}

func (s *RpcStub) RecvChan() <-chan pb.Message {
	return nil
}

func (s *RpcStub) Close() error {
	close(s.send)
	return nil
}

func NewRpcStub(stream grpc.BidiStreamingClient[cluster.Message, cluster.Message]) *RpcStub {
	send := make(chan *cluster.Message, configs.ChannelBufferSize)

	go func() {
		select {
		case <-stream.Context().Done():
			close(send)
		case msg := <-send:
			stream.Send(msg)
		}
	}()

	return &RpcStub{
		send: send,
		recv: nil,
	}
}
