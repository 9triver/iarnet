from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class PushLogRequest(_message.Message):
    __slots__ = ("timestamp", "container_id", "container_type", "level", "message", "source", "labels", "raw")
    class LabelsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    CONTAINER_ID_FIELD_NUMBER: _ClassVar[int]
    CONTAINER_TYPE_FIELD_NUMBER: _ClassVar[int]
    LEVEL_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    RAW_FIELD_NUMBER: _ClassVar[int]
    timestamp: str
    container_id: str
    container_type: str
    level: str
    message: str
    source: str
    labels: _containers.ScalarMap[str, str]
    raw: str
    def __init__(self, timestamp: _Optional[str] = ..., container_id: _Optional[str] = ..., container_type: _Optional[str] = ..., level: _Optional[str] = ..., message: _Optional[str] = ..., source: _Optional[str] = ..., labels: _Optional[_Mapping[str, str]] = ..., raw: _Optional[str] = ...) -> None: ...

class PushLogsBatchRequest(_message.Message):
    __slots__ = ("logs",)
    LOGS_FIELD_NUMBER: _ClassVar[int]
    logs: _containers.RepeatedCompositeFieldContainer[PushLogRequest]
    def __init__(self, logs: _Optional[_Iterable[_Union[PushLogRequest, _Mapping]]] = ...) -> None: ...

class PushLogResponse(_message.Message):
    __slots__ = ("success", "message", "accepted_count", "rejected_count")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    ACCEPTED_COUNT_FIELD_NUMBER: _ClassVar[int]
    REJECTED_COUNT_FIELD_NUMBER: _ClassVar[int]
    success: bool
    message: str
    accepted_count: int
    rejected_count: int
    def __init__(self, success: bool = ..., message: _Optional[str] = ..., accepted_count: _Optional[int] = ..., rejected_count: _Optional[int] = ...) -> None: ...
