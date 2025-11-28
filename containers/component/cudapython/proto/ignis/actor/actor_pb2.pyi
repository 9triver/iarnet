from common import types_pb2 as _types_pb2
from common import messages_pb2 as _messages_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class MessageType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    UNSPECIFIED: _ClassVar[MessageType]
    ACK: _ClassVar[MessageType]
    READY: _ClassVar[MessageType]
    FUNCTION: _ClassVar[MessageType]
    INVOKE_REQUEST: _ClassVar[MessageType]
    INVOKE_RESPONSE: _ClassVar[MessageType]
UNSPECIFIED: MessageType
ACK: MessageType
READY: MessageType
FUNCTION: MessageType
INVOKE_REQUEST: MessageType
INVOKE_RESPONSE: MessageType

class Function(_message.Message):
    __slots__ = ("Name", "Params", "Requirements", "PickledObject", "Language")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PARAMS_FIELD_NUMBER: _ClassVar[int]
    REQUIREMENTS_FIELD_NUMBER: _ClassVar[int]
    PICKLEDOBJECT_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Params: _containers.RepeatedScalarFieldContainer[str]
    Requirements: _containers.RepeatedScalarFieldContainer[str]
    PickledObject: bytes
    Language: _types_pb2.Language
    def __init__(self, Name: _Optional[str] = ..., Params: _Optional[_Iterable[str]] = ..., Requirements: _Optional[_Iterable[str]] = ..., PickledObject: _Optional[bytes] = ..., Language: _Optional[_Union[_types_pb2.Language, str]] = ...) -> None: ...

class InvokeArg(_message.Message):
    __slots__ = ("Param", "Value")
    PARAM_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    Param: str
    Value: _types_pb2.ObjectRef
    def __init__(self, Param: _Optional[str] = ..., Value: _Optional[_Union[_types_pb2.ObjectRef, _Mapping]] = ...) -> None: ...

class InvokeRequest(_message.Message):
    __slots__ = ("RuntimeID", "Args")
    RUNTIMEID_FIELD_NUMBER: _ClassVar[int]
    ARGS_FIELD_NUMBER: _ClassVar[int]
    RuntimeID: str
    Args: _containers.RepeatedCompositeFieldContainer[InvokeArg]
    def __init__(self, RuntimeID: _Optional[str] = ..., Args: _Optional[_Iterable[_Union[InvokeArg, _Mapping]]] = ...) -> None: ...

class ActorInfo(_message.Message):
    __slots__ = ("CalcLatency", "LinkLatency")
    CALCLATENCY_FIELD_NUMBER: _ClassVar[int]
    LINKLATENCY_FIELD_NUMBER: _ClassVar[int]
    CalcLatency: int
    LinkLatency: int
    def __init__(self, CalcLatency: _Optional[int] = ..., LinkLatency: _Optional[int] = ...) -> None: ...

class InvokeResponse(_message.Message):
    __slots__ = ("RuntimeID", "Result", "Error", "Info")
    RUNTIMEID_FIELD_NUMBER: _ClassVar[int]
    RESULT_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    INFO_FIELD_NUMBER: _ClassVar[int]
    RuntimeID: str
    Result: _types_pb2.ObjectRef
    Error: str
    Info: ActorInfo
    def __init__(self, RuntimeID: _Optional[str] = ..., Result: _Optional[_Union[_types_pb2.ObjectRef, _Mapping]] = ..., Error: _Optional[str] = ..., Info: _Optional[_Union[ActorInfo, _Mapping]] = ...) -> None: ...

class Message(_message.Message):
    __slots__ = ("Type", "Ack", "Ready", "Function", "InvokeRequest", "InvokeResponse")
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ACK_FIELD_NUMBER: _ClassVar[int]
    READY_FIELD_NUMBER: _ClassVar[int]
    FUNCTION_FIELD_NUMBER: _ClassVar[int]
    INVOKEREQUEST_FIELD_NUMBER: _ClassVar[int]
    INVOKERESPONSE_FIELD_NUMBER: _ClassVar[int]
    Type: MessageType
    Ack: _messages_pb2.Ack
    Ready: _messages_pb2.Ready
    Function: Function
    InvokeRequest: InvokeRequest
    InvokeResponse: InvokeResponse
    def __init__(self, Type: _Optional[_Union[MessageType, str]] = ..., Ack: _Optional[_Union[_messages_pb2.Ack, _Mapping]] = ..., Ready: _Optional[_Union[_messages_pb2.Ready, _Mapping]] = ..., Function: _Optional[_Union[Function, _Mapping]] = ..., InvokeRequest: _Optional[_Union[InvokeRequest, _Mapping]] = ..., InvokeResponse: _Optional[_Union[InvokeResponse, _Mapping]] = ...) -> None: ...
