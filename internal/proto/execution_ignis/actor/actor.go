package actor

import (
	commonpb "github.com/9triver/iarnet/internal/proto/common"
	pb "google.golang.org/protobuf/proto"
)

func NewFunction(name string, params []string, requirements []string, obj []byte, lang commonpb.Language) *Message {
	return NewMessage(&Function{
		Name:          name,
		Params:        params,
		Requirements:  requirements,
		PickledObject: obj,
		Language:      lang,
	})
}

func NewMessage(msg pb.Message) *Message {
	ret := &Message{}

	switch msg := msg.(type) {
	case *commonpb.Ack:
		ret.Type = MessageType_ACK
		ret.Message = &Message_Ack{Ack: msg}
	case *commonpb.Ready:
		ret.Type = MessageType_READY
		ret.Message = &Message_Ready{Ready: msg}
	case *Function:
		ret.Type = MessageType_FUNCTION
		ret.Message = &Message_Function{Function: msg}
	case *InvokeRequest:
		ret.Type = MessageType_INVOKE_REQUEST
		ret.Message = &Message_InvokeRequest{InvokeRequest: msg}
	case *InvokeResponse:
		ret.Type = MessageType_INVOKE_RESPONSE
		ret.Message = &Message_InvokeResponse{InvokeResponse: msg}
	default:
		ret.Type = MessageType_UNSPECIFIED
	}
	return ret
}
