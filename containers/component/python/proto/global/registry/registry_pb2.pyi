from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class NodeStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    NODE_STATUS_UNKNOWN: _ClassVar[NodeStatus]
    NODE_STATUS_ONLINE: _ClassVar[NodeStatus]
    NODE_STATUS_OFFLINE: _ClassVar[NodeStatus]
    NODE_STATUS_ERROR: _ClassVar[NodeStatus]

class ProviderStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PROVIDER_STATUS_UNKNOWN: _ClassVar[ProviderStatus]
    PROVIDER_STATUS_RUNNING: _ClassVar[ProviderStatus]
    PROVIDER_STATUS_STOPPED: _ClassVar[ProviderStatus]
    PROVIDER_STATUS_ERROR: _ClassVar[ProviderStatus]
NODE_STATUS_UNKNOWN: NodeStatus
NODE_STATUS_ONLINE: NodeStatus
NODE_STATUS_OFFLINE: NodeStatus
NODE_STATUS_ERROR: NodeStatus
PROVIDER_STATUS_UNKNOWN: ProviderStatus
PROVIDER_STATUS_RUNNING: ProviderStatus
PROVIDER_STATUS_STOPPED: ProviderStatus
PROVIDER_STATUS_ERROR: ProviderStatus

class RegisterNodeRequest(_message.Message):
    __slots__ = ("domain_id", "node_id", "node_name", "node_description")
    DOMAIN_ID_FIELD_NUMBER: _ClassVar[int]
    NODE_ID_FIELD_NUMBER: _ClassVar[int]
    NODE_NAME_FIELD_NUMBER: _ClassVar[int]
    NODE_DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    domain_id: str
    node_id: str
    node_name: str
    node_description: str
    def __init__(self, domain_id: _Optional[str] = ..., node_id: _Optional[str] = ..., node_name: _Optional[str] = ..., node_description: _Optional[str] = ...) -> None: ...

class RegisterNodeResponse(_message.Message):
    __slots__ = ("domain_name", "domain_description")
    DOMAIN_NAME_FIELD_NUMBER: _ClassVar[int]
    DOMAIN_DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    domain_name: str
    domain_description: str
    def __init__(self, domain_name: _Optional[str] = ..., domain_description: _Optional[str] = ...) -> None: ...

class ResourceInfo(_message.Message):
    __slots__ = ("cpu", "memory", "gpu")
    CPU_FIELD_NUMBER: _ClassVar[int]
    MEMORY_FIELD_NUMBER: _ClassVar[int]
    GPU_FIELD_NUMBER: _ClassVar[int]
    cpu: int
    memory: int
    gpu: int
    def __init__(self, cpu: _Optional[int] = ..., memory: _Optional[int] = ..., gpu: _Optional[int] = ...) -> None: ...

class ResourceCapacity(_message.Message):
    __slots__ = ("total", "used", "available")
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    USED_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    total: ResourceInfo
    used: ResourceInfo
    available: ResourceInfo
    def __init__(self, total: _Optional[_Union[ResourceInfo, _Mapping]] = ..., used: _Optional[_Union[ResourceInfo, _Mapping]] = ..., available: _Optional[_Union[ResourceInfo, _Mapping]] = ...) -> None: ...

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

class ProviderInfo(_message.Message):
    __slots__ = ("id", "name", "type", "status", "version", "metadata")
    class MetadataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    type: str
    status: ProviderStatus
    version: str
    metadata: _containers.ScalarMap[str, str]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., type: _Optional[str] = ..., status: _Optional[_Union[ProviderStatus, str]] = ..., version: _Optional[str] = ..., metadata: _Optional[_Mapping[str, str]] = ...) -> None: ...

class HealthCheckRequest(_message.Message):
    __slots__ = ("node_id", "domain_id", "status", "resource_capacity", "resource_tags", "address", "timestamp", "is_head", "providers", "node_name", "node_description")
    NODE_ID_FIELD_NUMBER: _ClassVar[int]
    DOMAIN_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_CAPACITY_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_TAGS_FIELD_NUMBER: _ClassVar[int]
    ADDRESS_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    IS_HEAD_FIELD_NUMBER: _ClassVar[int]
    PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    NODE_NAME_FIELD_NUMBER: _ClassVar[int]
    NODE_DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    node_id: str
    domain_id: str
    status: NodeStatus
    resource_capacity: ResourceCapacity
    resource_tags: ResourceTags
    address: str
    timestamp: int
    is_head: bool
    providers: _containers.RepeatedCompositeFieldContainer[ProviderInfo]
    node_name: str
    node_description: str
    def __init__(self, node_id: _Optional[str] = ..., domain_id: _Optional[str] = ..., status: _Optional[_Union[NodeStatus, str]] = ..., resource_capacity: _Optional[_Union[ResourceCapacity, _Mapping]] = ..., resource_tags: _Optional[_Union[ResourceTags, _Mapping]] = ..., address: _Optional[str] = ..., timestamp: _Optional[int] = ..., is_head: bool = ..., providers: _Optional[_Iterable[_Union[ProviderInfo, _Mapping]]] = ..., node_name: _Optional[str] = ..., node_description: _Optional[str] = ...) -> None: ...

class HealthCheckResponse(_message.Message):
    __slots__ = ("server_timestamp", "recommended_interval_seconds", "require_reregister", "status_code", "message")
    SERVER_TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    RECOMMENDED_INTERVAL_SECONDS_FIELD_NUMBER: _ClassVar[int]
    REQUIRE_REREGISTER_FIELD_NUMBER: _ClassVar[int]
    STATUS_CODE_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    server_timestamp: int
    recommended_interval_seconds: int
    require_reregister: bool
    status_code: str
    message: str
    def __init__(self, server_timestamp: _Optional[int] = ..., recommended_interval_seconds: _Optional[int] = ..., require_reregister: bool = ..., status_code: _Optional[str] = ..., message: _Optional[str] = ...) -> None: ...
