from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Language(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    LANGUAGE_UNKNOWN: _ClassVar[Language]
    LANGUAGE_JSON: _ClassVar[Language]
    LANGUAGE_GO: _ClassVar[Language]
    LANGUAGE_PYTHON: _ClassVar[Language]
LANGUAGE_UNKNOWN: Language
LANGUAGE_JSON: Language
LANGUAGE_GO: Language
LANGUAGE_PYTHON: Language

class ObjectRef(_message.Message):
    __slots__ = ("ID", "Source")
    ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    ID: str
    Source: str
    def __init__(self, ID: _Optional[str] = ..., Source: _Optional[str] = ...) -> None: ...

class EncodedObject(_message.Message):
    __slots__ = ("ID", "Data", "Language", "IsStream")
    ID_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    ISSTREAM_FIELD_NUMBER: _ClassVar[int]
    ID: str
    Data: bytes
    Language: Language
    IsStream: bool
    def __init__(self, ID: _Optional[str] = ..., Data: _Optional[bytes] = ..., Language: _Optional[_Union[Language, str]] = ..., IsStream: bool = ...) -> None: ...

class StreamChunk(_message.Message):
    __slots__ = ("ObjectID", "Offset", "EoS", "Value", "Error")
    OBJECTID_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    EOS_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    ObjectID: str
    Offset: str
    EoS: bool
    Value: EncodedObject
    Error: str
    def __init__(self, ObjectID: _Optional[str] = ..., Offset: _Optional[str] = ..., EoS: bool = ..., Value: _Optional[_Union[EncodedObject, _Mapping]] = ..., Error: _Optional[str] = ...) -> None: ...

class SaveObjectRequest(_message.Message):
    __slots__ = ("Object",)
    OBJECT_FIELD_NUMBER: _ClassVar[int]
    Object: EncodedObject
    def __init__(self, Object: _Optional[_Union[EncodedObject, _Mapping]] = ...) -> None: ...

class SaveObjectResponse(_message.Message):
    __slots__ = ("ObjectRef", "Success", "Error")
    OBJECTREF_FIELD_NUMBER: _ClassVar[int]
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    ObjectRef: ObjectRef
    Success: bool
    Error: str
    def __init__(self, ObjectRef: _Optional[_Union[ObjectRef, _Mapping]] = ..., Success: bool = ..., Error: _Optional[str] = ...) -> None: ...

class GetObjectRequest(_message.Message):
    __slots__ = ("ObjectRef",)
    OBJECTREF_FIELD_NUMBER: _ClassVar[int]
    ObjectRef: ObjectRef
    def __init__(self, ObjectRef: _Optional[_Union[ObjectRef, _Mapping]] = ...) -> None: ...

class GetObjectResponse(_message.Message):
    __slots__ = ("Object",)
    OBJECT_FIELD_NUMBER: _ClassVar[int]
    Object: EncodedObject
    def __init__(self, Object: _Optional[_Union[EncodedObject, _Mapping]] = ...) -> None: ...

class GetStreamChunkRequest(_message.Message):
    __slots__ = ("ObjectID", "Offset")
    OBJECTID_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    ObjectID: str
    Offset: str
    def __init__(self, ObjectID: _Optional[str] = ..., Offset: _Optional[str] = ...) -> None: ...

class GetStreamChunkResponse(_message.Message):
    __slots__ = ("Chunk",)
    CHUNK_FIELD_NUMBER: _ClassVar[int]
    Chunk: StreamChunk
    def __init__(self, Chunk: _Optional[_Union[StreamChunk, _Mapping]] = ...) -> None: ...
