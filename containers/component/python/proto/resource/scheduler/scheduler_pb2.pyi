from resource import resource_pb2 as _resource_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ComponentStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    COMPONENT_STATUS_UNKNOWN: _ClassVar[ComponentStatus]
    COMPONENT_STATUS_DEPLOYING: _ClassVar[ComponentStatus]
    COMPONENT_STATUS_RUNNING: _ClassVar[ComponentStatus]
    COMPONENT_STATUS_STOPPED: _ClassVar[ComponentStatus]
    COMPONENT_STATUS_ERROR: _ClassVar[ComponentStatus]
COMPONENT_STATUS_UNKNOWN: ComponentStatus
COMPONENT_STATUS_DEPLOYING: ComponentStatus
COMPONENT_STATUS_RUNNING: ComponentStatus
COMPONENT_STATUS_STOPPED: ComponentStatus
COMPONENT_STATUS_ERROR: ComponentStatus

class DeployComponentRequest(_message.Message):
    __slots__ = ("runtime_env", "resource_request", "target_node_id", "target_node_address", "upstream_zmq_address", "upstream_store_address", "upstream_logger_address")
    RUNTIME_ENV_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_REQUEST_FIELD_NUMBER: _ClassVar[int]
    TARGET_NODE_ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_NODE_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    UPSTREAM_ZMQ_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    UPSTREAM_STORE_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    UPSTREAM_LOGGER_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    runtime_env: str
    resource_request: _resource_pb2.Info
    target_node_id: str
    target_node_address: str
    upstream_zmq_address: str
    upstream_store_address: str
    upstream_logger_address: str
    def __init__(self, runtime_env: _Optional[str] = ..., resource_request: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ..., target_node_id: _Optional[str] = ..., target_node_address: _Optional[str] = ..., upstream_zmq_address: _Optional[str] = ..., upstream_store_address: _Optional[str] = ..., upstream_logger_address: _Optional[str] = ...) -> None: ...

class DeployComponentResponse(_message.Message):
    __slots__ = ("success", "error", "component", "node_id", "node_name", "provider_id")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    COMPONENT_FIELD_NUMBER: _ClassVar[int]
    NODE_ID_FIELD_NUMBER: _ClassVar[int]
    NODE_NAME_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    success: bool
    error: str
    component: ComponentInfo
    node_id: str
    node_name: str
    provider_id: str
    def __init__(self, success: bool = ..., error: _Optional[str] = ..., component: _Optional[_Union[ComponentInfo, _Mapping]] = ..., node_id: _Optional[str] = ..., node_name: _Optional[str] = ..., provider_id: _Optional[str] = ...) -> None: ...

class ComponentInfo(_message.Message):
    __slots__ = ("component_id", "image", "resource_usage", "provider_id")
    COMPONENT_ID_FIELD_NUMBER: _ClassVar[int]
    IMAGE_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_USAGE_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    component_id: str
    image: str
    resource_usage: _resource_pb2.Info
    provider_id: str
    def __init__(self, component_id: _Optional[str] = ..., image: _Optional[str] = ..., resource_usage: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ..., provider_id: _Optional[str] = ...) -> None: ...

class GetDeploymentStatusRequest(_message.Message):
    __slots__ = ("component_id", "node_id")
    COMPONENT_ID_FIELD_NUMBER: _ClassVar[int]
    NODE_ID_FIELD_NUMBER: _ClassVar[int]
    component_id: str
    node_id: str
    def __init__(self, component_id: _Optional[str] = ..., node_id: _Optional[str] = ...) -> None: ...

class GetDeploymentStatusResponse(_message.Message):
    __slots__ = ("success", "error", "status", "component")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    COMPONENT_FIELD_NUMBER: _ClassVar[int]
    success: bool
    error: str
    status: ComponentStatus
    component: ComponentInfo
    def __init__(self, success: bool = ..., error: _Optional[str] = ..., status: _Optional[_Union[ComponentStatus, str]] = ..., component: _Optional[_Union[ComponentInfo, _Mapping]] = ...) -> None: ...

class ProposeLocalScheduleRequest(_message.Message):
    __slots__ = ("resource_request",)
    RESOURCE_REQUEST_FIELD_NUMBER: _ClassVar[int]
    resource_request: _resource_pb2.Info
    def __init__(self, resource_request: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ...) -> None: ...

class ProposeLocalScheduleResponse(_message.Message):
    __slots__ = ("success", "error", "node_id", "node_name", "provider_id", "available")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    NODE_ID_FIELD_NUMBER: _ClassVar[int]
    NODE_NAME_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    success: bool
    error: str
    node_id: str
    node_name: str
    provider_id: str
    available: _resource_pb2.Info
    def __init__(self, success: bool = ..., error: _Optional[str] = ..., node_id: _Optional[str] = ..., node_name: _Optional[str] = ..., provider_id: _Optional[str] = ..., available: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ...) -> None: ...

class CommitLocalScheduleRequest(_message.Message):
    __slots__ = ("runtime_env", "resource_request", "provider_id", "upstream_zmq_address", "upstream_store_address", "upstream_logger_address")
    RUNTIME_ENV_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_REQUEST_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    UPSTREAM_ZMQ_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    UPSTREAM_STORE_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    UPSTREAM_LOGGER_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    runtime_env: str
    resource_request: _resource_pb2.Info
    provider_id: str
    upstream_zmq_address: str
    upstream_store_address: str
    upstream_logger_address: str
    def __init__(self, runtime_env: _Optional[str] = ..., resource_request: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ..., provider_id: _Optional[str] = ..., upstream_zmq_address: _Optional[str] = ..., upstream_store_address: _Optional[str] = ..., upstream_logger_address: _Optional[str] = ...) -> None: ...

class ListProvidersRequest(_message.Message):
    __slots__ = ("include_resources",)
    INCLUDE_RESOURCES_FIELD_NUMBER: _ClassVar[int]
    include_resources: bool
    def __init__(self, include_resources: bool = ...) -> None: ...

class ListProvidersResponse(_message.Message):
    __slots__ = ("success", "error", "node_id", "node_name", "providers")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    NODE_ID_FIELD_NUMBER: _ClassVar[int]
    NODE_NAME_FIELD_NUMBER: _ClassVar[int]
    PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    success: bool
    error: str
    node_id: str
    node_name: str
    providers: _containers.RepeatedCompositeFieldContainer[ProviderInfo]
    def __init__(self, success: bool = ..., error: _Optional[str] = ..., node_id: _Optional[str] = ..., node_name: _Optional[str] = ..., providers: _Optional[_Iterable[_Union[ProviderInfo, _Mapping]]] = ...) -> None: ...

class ProviderInfo(_message.Message):
    __slots__ = ("provider_id", "provider_name", "provider_type", "status", "available", "total_capacity", "used", "resource_tags")
    PROVIDER_ID_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_NAME_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_TYPE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    TOTAL_CAPACITY_FIELD_NUMBER: _ClassVar[int]
    USED_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_TAGS_FIELD_NUMBER: _ClassVar[int]
    provider_id: str
    provider_name: str
    provider_type: str
    status: str
    available: _resource_pb2.Info
    total_capacity: _resource_pb2.Info
    used: _resource_pb2.Info
    resource_tags: ResourceTags
    def __init__(self, provider_id: _Optional[str] = ..., provider_name: _Optional[str] = ..., provider_type: _Optional[str] = ..., status: _Optional[str] = ..., available: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ..., total_capacity: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ..., used: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ..., resource_tags: _Optional[_Union[ResourceTags, _Mapping]] = ...) -> None: ...

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
