from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Info(_message.Message):
    __slots__ = ("cpu", "memory", "gpu", "tags")
    CPU_FIELD_NUMBER: _ClassVar[int]
    MEMORY_FIELD_NUMBER: _ClassVar[int]
    GPU_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    cpu: int
    memory: int
    gpu: int
    tags: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, cpu: _Optional[int] = ..., memory: _Optional[int] = ..., gpu: _Optional[int] = ..., tags: _Optional[_Iterable[str]] = ...) -> None: ...

class Capacity(_message.Message):
    __slots__ = ("total", "used", "available")
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    USED_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    total: Info
    used: Info
    available: Info
    def __init__(self, total: _Optional[_Union[Info, _Mapping]] = ..., used: _Optional[_Union[Info, _Mapping]] = ..., available: _Optional[_Union[Info, _Mapping]] = ...) -> None: ...

class ResourceRequest(_message.Message):
    __slots__ = ("info",)
    INFO_FIELD_NUMBER: _ClassVar[int]
    info: Info
    def __init__(self, info: _Optional[_Union[Info, _Mapping]] = ...) -> None: ...
