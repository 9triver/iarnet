from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ProviderInfo(_message.Message):
    __slots__ = ("id", "name", "type", "host", "port", "status", "peer_address")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    HOST_FIELD_NUMBER: _ClassVar[int]
    PORT_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    PEER_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    type: str
    host: str
    port: int
    status: int
    peer_address: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., type: _Optional[str] = ..., host: _Optional[str] = ..., port: _Optional[int] = ..., status: _Optional[int] = ..., peer_address: _Optional[str] = ...) -> None: ...

class ExchangeRequest(_message.Message):
    __slots__ = ("known_peers",)
    KNOWN_PEERS_FIELD_NUMBER: _ClassVar[int]
    known_peers: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, known_peers: _Optional[_Iterable[str]] = ...) -> None: ...

class ExchangeResponse(_message.Message):
    __slots__ = ("known_peers",)
    KNOWN_PEERS_FIELD_NUMBER: _ClassVar[int]
    known_peers: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, known_peers: _Optional[_Iterable[str]] = ...) -> None: ...

class ProviderExchangeRequest(_message.Message):
    __slots__ = ("providers",)
    PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    providers: _containers.RepeatedCompositeFieldContainer[ProviderInfo]
    def __init__(self, providers: _Optional[_Iterable[_Union[ProviderInfo, _Mapping]]] = ...) -> None: ...

class ProviderExchangeResponse(_message.Message):
    __slots__ = ("providers",)
    PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    providers: _containers.RepeatedCompositeFieldContainer[ProviderInfo]
    def __init__(self, providers: _Optional[_Iterable[_Union[ProviderInfo, _Mapping]]] = ...) -> None: ...

class ProviderCallRequest(_message.Message):
    __slots__ = ("provider_id", "method", "payload")
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    PAYLOAD_FIELD_NUMBER: _ClassVar[int]
    provider_id: str
    method: str
    payload: bytes
    def __init__(self, provider_id: _Optional[str] = ..., method: _Optional[str] = ..., payload: _Optional[bytes] = ...) -> None: ...

class ProviderCallResponse(_message.Message):
    __slots__ = ("success", "error", "result")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    RESULT_FIELD_NUMBER: _ClassVar[int]
    success: bool
    error: str
    result: bytes
    def __init__(self, success: bool = ..., error: _Optional[str] = ..., result: _Optional[bytes] = ...) -> None: ...
