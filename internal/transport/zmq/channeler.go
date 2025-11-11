package zmq

import (
	"fmt"

	"gopkg.in/zeromq/goczmq.v4"
)

func NewChanneler(zmqAddr string) (*goczmq.Channeler, error) {
	if zmqAddr == "" {
		return nil, fmt.Errorf("ZMQ address not set")
	}

	channeler := goczmq.NewRouterChanneler(zmqAddr)
	if channeler == nil {
		return nil, fmt.Errorf("failed to create ZMQ router channeler")
	}

	return channeler, nil
}
