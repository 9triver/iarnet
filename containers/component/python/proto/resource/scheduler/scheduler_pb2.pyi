from resource import resource_pb2 as _resource_pb2
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
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
    __slots__ = ("runtime_env", "resource_request", "target_node_id", "target_node_address")
    RUNTIME_ENV_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_REQUEST_FIELD_NUMBER: _ClassVar[int]
    TARGET_NODE_ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_NODE_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    runtime_env: str
    resource_request: _resource_pb2.Info
    target_node_id: str
    target_node_address: str
    def __init__(self, runtime_env: _Optional[str] = ..., resource_request: _Optional[_Union[_resource_pb2.Info, _Mapping]] = ..., target_node_id: _Optional[str] = ..., target_node_address: _Optional[str] = ...) -> None: ...

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
