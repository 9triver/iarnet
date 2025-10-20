package integration

import pb "github.com/gogo/protobuf/proto"

type Actor struct {
	AppID   string
	FuncMsg pb.Message
}

func NewActor(appID string, funcMsg pb.Message) *Actor {
	return &Actor{
		AppID:   appID,
		FuncMsg: funcMsg,
	}
}
