from common import messages_pb2 as _messages_pb2
from google.protobuf import any_pb2 as _any_pb2
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class MessageType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    UNSPECIFIED: _ClassVar[MessageType]
    READY: _ClassVar[MessageType]
    PAYLOAD: _ClassVar[MessageType]
UNSPECIFIED: MessageType
READY: MessageType
PAYLOAD: MessageType

class Message(_message.Message):
    __slots__ = ("Type", "Ready", "Payload")
    TYPE_FIELD_NUMBER: _ClassVar[int]
    READY_FIELD_NUMBER: _ClassVar[int]
    PAYLOAD_FIELD_NUMBER: _ClassVar[int]
    Type: MessageType
    Ready: _messages_pb2.Ready
    Payload: _any_pb2.Any
    def __init__(self, Type: _Optional[_Union[MessageType, str]] = ..., Ready: _Optional[_Union[_messages_pb2.Ready, _Mapping]] = ..., Payload: _Optional[_Union[_any_pb2.Any, _Mapping]] = ...) -> None: ...
