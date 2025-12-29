from resource import resource_pb2 as _resource_pb2
from common import types_pb2 as _types_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ProviderType(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ConnectRequest(_message.Message):
    __slots__ = ("provider_id",)
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    provider_id: str
    def __init__(self, provider_id: _Optional[str] = ...) -> None: ...

class ConnectResponse(_message.Message):
    __slots__ = ("success", "error", "provider_type", "supported_languages")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_TYPE_FIELD_NUMBER: _ClassVar[int]
    SUPPORTED_LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    success: bool
    error: str
    provider_type: ProviderType
    supported_languages: _containers.RepeatedScalarFieldContainer[_types_pb2.Language]
    def __init__(self, success: bool = ..., error: _Optional[str] = ..., provider_type: _Optional[_Union[ProviderType, _Mapping]] = ..., supported_languages: _Optional[_Iterable[_Union[_types_pb2.Language, str]]] = ...) -> None: ...

class GetCapacityRequest(_message.Message):
    __slots__ = ("provider_id",)
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    provider_id: str
    def __init__(self, provider_id: _Optional[str] = ...) -> None: ...

class GetCapacityResponse(_message.Message):
    __slots__ = ("capacity",)
    CAPACITY_FIELD_NUMBER: _ClassVar[int]
    capacity: _resource_pb2.Capacity
    def __init__(self, capacity: _Optional[_Union[_resource_pb2.Capacity, _Mapping]] = ...) -> None: ...

class GetAvailableRequest(_message.Message):
    __slots__ = ("provider_id",)
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    provider_id: str
    def __init__(self, provider_id: _Optional[str] = ...) -> None: ...

class GetAvailableResponse(_message.Message):
    __slots__ = ("available",)
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    available: _resource_pb2.Info
    def __init__(self, available: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ...) -> None: ...

class DeployRequest(_message.Message):
    __slots__ = ("instance_id", "image", "resource_request", "env_vars", "provider_id", "language")
    class EnvVarsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    INSTANCE_ID_FIELD_NUMBER: _ClassVar[int]
    IMAGE_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_REQUEST_FIELD_NUMBER: _ClassVar[int]
    ENV_VARS_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    instance_id: str
    image: str
    resource_request: _resource_pb2.Info
    env_vars: _containers.ScalarMap[str, str]
    provider_id: str
    language: _types_pb2.Language
    def __init__(self, instance_id: _Optional[str] = ..., image: _Optional[str] = ..., resource_request: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ..., env_vars: _Optional[_Mapping[str, str]] = ..., provider_id: _Optional[str] = ..., language: _Optional[_Union[_types_pb2.Language, str]] = ...) -> None: ...

class DeployResponse(_message.Message):
    __slots__ = ("error",)
    ERROR_FIELD_NUMBER: _ClassVar[int]
    error: str
    def __init__(self, error: _Optional[str] = ...) -> None: ...

class UndeployRequest(_message.Message):
    __slots__ = ("instance_id", "provider_id")
    INSTANCE_ID_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    instance_id: str
    provider_id: str
    def __init__(self, instance_id: _Optional[str] = ..., provider_id: _Optional[str] = ...) -> None: ...

class UndeployResponse(_message.Message):
    __slots__ = ("error",)
    ERROR_FIELD_NUMBER: _ClassVar[int]
    error: str
    def __init__(self, error: _Optional[str] = ...) -> None: ...

class HealthCheckRequest(_message.Message):
    __slots__ = ("provider_id",)
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    provider_id: str
    def __init__(self, provider_id: _Optional[str] = ...) -> None: ...

class ResourceTags(_message.Message):
    __slots__ = ("cpu", "gpu", "memory", "camera")
    CPU_FIELD_NUMBER: _ClassVar[int]
    GPU_FIELD_NUMBER: _ClassVar[int]
    MEMORY_FIELD_NUMBER: _ClassVar[int]
    CAMERA_FIELD_NUMBER: _ClassVar[int]
    cpu: bool
    gpu: bool
    memory: bool
    camera: bool
    def __init__(self, cpu: bool = ..., gpu: bool = ..., memory: bool = ..., camera: bool = ...) -> None: ...

class HealthCheckResponse(_message.Message):
    __slots__ = ("capacity", "resource_tags", "supported_languages")
    CAPACITY_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_TAGS_FIELD_NUMBER: _ClassVar[int]
    SUPPORTED_LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    capacity: _resource_pb2.Capacity
    resource_tags: ResourceTags
    supported_languages: _containers.RepeatedScalarFieldContainer[_types_pb2.Language]
    def __init__(self, capacity: _Optional[_Union[_resource_pb2.Capacity, _Mapping]] = ..., resource_tags: _Optional[_Union[ResourceTags, _Mapping]] = ..., supported_languages: _Optional[_Iterable[_Union[_types_pb2.Language, str]]] = ...) -> None: ...

class DisconnectRequest(_message.Message):
    __slots__ = ("provider_id",)
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    provider_id: str
    def __init__(self, provider_id: _Optional[str] = ...) -> None: ...

class DisconnectResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetRealTimeUsageRequest(_message.Message):
    __slots__ = ("provider_id",)
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    provider_id: str
    def __init__(self, provider_id: _Optional[str] = ...) -> None: ...

class GetRealTimeUsageResponse(_message.Message):
    __slots__ = ("usage",)
    USAGE_FIELD_NUMBER: _ClassVar[int]
    usage: _resource_pb2.Info
    def __init__(self, usage: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ...) -> None: ...
