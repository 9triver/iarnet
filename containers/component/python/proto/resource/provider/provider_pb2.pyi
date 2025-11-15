import resource_pb2 as _resource_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ProviderType(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class AssignIDRequest(_message.Message):
    __slots__ = ("provider_id",)
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    provider_id: str
    def __init__(self, provider_id: _Optional[str] = ...) -> None: ...

class AssignIDResponse(_message.Message):
    __slots__ = ("success", "error", "provider_type")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_TYPE_FIELD_NUMBER: _ClassVar[int]
    success: bool
    error: str
    provider_type: ProviderType
    def __init__(self, success: bool = ..., error: _Optional[str] = ..., provider_type: _Optional[_Union[ProviderType, _Mapping]] = ...) -> None: ...

class GetCapacityRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetCapacityResponse(_message.Message):
    __slots__ = ("capacity",)
    CAPACITY_FIELD_NUMBER: _ClassVar[int]
    capacity: _resource_pb2.Capacity
    def __init__(self, capacity: _Optional[_Union[_resource_pb2.Capacity, _Mapping]] = ...) -> None: ...

class GetAvailableRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetAvailableResponse(_message.Message):
    __slots__ = ("available",)
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    available: _resource_pb2.Info
    def __init__(self, available: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ...) -> None: ...

class DeployComponentRequest(_message.Message):
    __slots__ = ("component_id", "image", "resource_request", "env_vars")
    class EnvVarsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    COMPONENT_ID_FIELD_NUMBER: _ClassVar[int]
    IMAGE_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_REQUEST_FIELD_NUMBER: _ClassVar[int]
    ENV_VARS_FIELD_NUMBER: _ClassVar[int]
    component_id: str
    image: str
    resource_request: _resource_pb2.Info
    env_vars: _containers.ScalarMap[str, str]
    def __init__(self, component_id: _Optional[str] = ..., image: _Optional[str] = ..., resource_request: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ..., env_vars: _Optional[_Mapping[str, str]] = ...) -> None: ...

class DeployComponentResponse(_message.Message):
    __slots__ = ("error",)
    ERROR_FIELD_NUMBER: _ClassVar[int]
    error: str
    def __init__(self, error: _Optional[str] = ...) -> None: ...
