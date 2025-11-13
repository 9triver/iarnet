package component

import (
	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"
)

// NewPayloadWithError creates a Message with the given proto.Message as payload.
// Returns error if conversion fails.
func NewPayload(message proto.Message) (*Message, error) {
	any, err := anypb.New(message)
	if err != nil {
		return nil, err
	}
	return &Message{
		Type:    MessageType_PAYLOAD,
		Message: &Message_Payload{Payload: any},
	}, nil
}

// GetPayloadMessage extracts the proto.Message from Payload field.
// Returns nil if Payload is not set or unmarshal fails.
func (m *Message) GetPayloadMessage() proto.Message {
	if m == nil {
		return nil
	}
	payload := m.GetPayload()
	if payload == nil {
		return nil
	}
	msg, err := payload.UnmarshalNew()
	if err != nil {
		return nil
	}
	return msg
}
