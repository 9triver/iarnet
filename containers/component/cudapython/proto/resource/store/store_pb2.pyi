from common import types_pb2 as _types_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SaveObjectRequest(_message.Message):
    __slots__ = ("Object",)
    OBJECT_FIELD_NUMBER: _ClassVar[int]
    Object: _types_pb2.EncodedObject
    def __init__(self, Object: _Optional[_Union[_types_pb2.EncodedObject, _Mapping]] = ...) -> None: ...

class SaveObjectResponse(_message.Message):
    __slots__ = ("ObjectRef", "Success", "Error")
    OBJECTREF_FIELD_NUMBER: _ClassVar[int]
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    ObjectRef: _types_pb2.ObjectRef
    Success: bool
    Error: str
    def __init__(self, ObjectRef: _Optional[_Union[_types_pb2.ObjectRef, _Mapping]] = ..., Success: bool = ..., Error: _Optional[str] = ...) -> None: ...

class GetObjectRequest(_message.Message):
    __slots__ = ("ObjectRef",)
    OBJECTREF_FIELD_NUMBER: _ClassVar[int]
    ObjectRef: _types_pb2.ObjectRef
    def __init__(self, ObjectRef: _Optional[_Union[_types_pb2.ObjectRef, _Mapping]] = ...) -> None: ...

class GetObjectResponse(_message.Message):
    __slots__ = ("Object",)
    OBJECT_FIELD_NUMBER: _ClassVar[int]
    Object: _types_pb2.EncodedObject
    def __init__(self, Object: _Optional[_Union[_types_pb2.EncodedObject, _Mapping]] = ...) -> None: ...

class GetStreamChunkRequest(_message.Message):
    __slots__ = ("ObjectID", "Offset")
    OBJECTID_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    ObjectID: str
    Offset: int
    def __init__(self, ObjectID: _Optional[str] = ..., Offset: _Optional[int] = ...) -> None: ...

class GetStreamChunkResponse(_message.Message):
    __slots__ = ("Chunk",)
    CHUNK_FIELD_NUMBER: _ClassVar[int]
    Chunk: _types_pb2.StreamChunk
    def __init__(self, Chunk: _Optional[_Union[_types_pb2.StreamChunk, _Mapping]] = ...) -> None: ...

class SaveStreamChunkRequest(_message.Message):
    __slots__ = ("Chunk",)
    CHUNK_FIELD_NUMBER: _ClassVar[int]
    Chunk: _types_pb2.StreamChunk
    def __init__(self, Chunk: _Optional[_Union[_types_pb2.StreamChunk, _Mapping]] = ...) -> None: ...

class SaveStreamChunkResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
