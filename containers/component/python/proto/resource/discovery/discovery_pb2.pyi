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
NODE_STATUS_UNKNOWN: NodeStatus
NODE_STATUS_ONLINE: NodeStatus
NODE_STATUS_OFFLINE: NodeStatus
NODE_STATUS_ERROR: NodeStatus

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

class PeerNodeInfo(_message.Message):
    __slots__ = ("node_id", "node_name", "address", "domain_id", "scheduler_address", "resource_capacity", "resource_tags", "status", "last_seen", "last_updated", "version", "gossip_count")
    NODE_ID_FIELD_NUMBER: _ClassVar[int]
    NODE_NAME_FIELD_NUMBER: _ClassVar[int]
    ADDRESS_FIELD_NUMBER: _ClassVar[int]
    DOMAIN_ID_FIELD_NUMBER: _ClassVar[int]
    SCHEDULER_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_CAPACITY_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_TAGS_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    LAST_SEEN_FIELD_NUMBER: _ClassVar[int]
    LAST_UPDATED_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    GOSSIP_COUNT_FIELD_NUMBER: _ClassVar[int]
    node_id: str
    node_name: str
    address: str
    domain_id: str
    scheduler_address: str
    resource_capacity: ResourceCapacity
    resource_tags: ResourceTags
    status: NodeStatus
    last_seen: int
    last_updated: int
    version: int
    gossip_count: int
    def __init__(self, node_id: _Optional[str] = ..., node_name: _Optional[str] = ..., address: _Optional[str] = ..., domain_id: _Optional[str] = ..., scheduler_address: _Optional[str] = ..., resource_capacity: _Optional[_Union[ResourceCapacity, _Mapping]] = ..., resource_tags: _Optional[_Union[ResourceTags, _Mapping]] = ..., status: _Optional[_Union[NodeStatus, str]] = ..., last_seen: _Optional[int] = ..., last_updated: _Optional[int] = ..., version: _Optional[int] = ..., gossip_count: _Optional[int] = ...) -> None: ...

class NodeInfoGossipMessage(_message.Message):
    __slots__ = ("sender_node_id", "sender_address", "sender_domain_id", "nodes", "message_id", "timestamp", "ttl", "max_hops")
    SENDER_NODE_ID_FIELD_NUMBER: _ClassVar[int]
    SENDER_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    SENDER_DOMAIN_ID_FIELD_NUMBER: _ClassVar[int]
    NODES_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_ID_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    TTL_FIELD_NUMBER: _ClassVar[int]
    MAX_HOPS_FIELD_NUMBER: _ClassVar[int]
    sender_node_id: str
    sender_address: str
    sender_domain_id: str
    nodes: _containers.RepeatedCompositeFieldContainer[PeerNodeInfo]
    message_id: str
    timestamp: int
    ttl: int
    max_hops: int
    def __init__(self, sender_node_id: _Optional[str] = ..., sender_address: _Optional[str] = ..., sender_domain_id: _Optional[str] = ..., nodes: _Optional[_Iterable[_Union[PeerNodeInfo, _Mapping]]] = ..., message_id: _Optional[str] = ..., timestamp: _Optional[int] = ..., ttl: _Optional[int] = ..., max_hops: _Optional[int] = ...) -> None: ...

class NodeInfoGossipResponse(_message.Message):
    __slots__ = ("nodes", "message_id", "timestamp")
    NODES_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_ID_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    nodes: _containers.RepeatedCompositeFieldContainer[PeerNodeInfo]
    message_id: str
    timestamp: int
    def __init__(self, nodes: _Optional[_Iterable[_Union[PeerNodeInfo, _Mapping]]] = ..., message_id: _Optional[str] = ..., timestamp: _Optional[int] = ...) -> None: ...

class ResourceRequest(_message.Message):
    __slots__ = ("cpu", "memory", "gpu")
    CPU_FIELD_NUMBER: _ClassVar[int]
    MEMORY_FIELD_NUMBER: _ClassVar[int]
    GPU_FIELD_NUMBER: _ClassVar[int]
    cpu: int
    memory: int
    gpu: int
    def __init__(self, cpu: _Optional[int] = ..., memory: _Optional[int] = ..., gpu: _Optional[int] = ...) -> None: ...

class ResourceQueryRequest(_message.Message):
    __slots__ = ("query_id", "requester_node_id", "requester_address", "requester_domain_id", "resource_request", "required_tags", "timestamp", "max_hops", "ttl", "current_hops")
    QUERY_ID_FIELD_NUMBER: _ClassVar[int]
    REQUESTER_NODE_ID_FIELD_NUMBER: _ClassVar[int]
    REQUESTER_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    REQUESTER_DOMAIN_ID_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_REQUEST_FIELD_NUMBER: _ClassVar[int]
    REQUIRED_TAGS_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    MAX_HOPS_FIELD_NUMBER: _ClassVar[int]
    TTL_FIELD_NUMBER: _ClassVar[int]
    CURRENT_HOPS_FIELD_NUMBER: _ClassVar[int]
    query_id: str
    requester_node_id: str
    requester_address: str
    requester_domain_id: str
    resource_request: ResourceRequest
    required_tags: ResourceTags
    timestamp: int
    max_hops: int
    ttl: int
    current_hops: int
    def __init__(self, query_id: _Optional[str] = ..., requester_node_id: _Optional[str] = ..., requester_address: _Optional[str] = ..., requester_domain_id: _Optional[str] = ..., resource_request: _Optional[_Union[ResourceRequest, _Mapping]] = ..., required_tags: _Optional[_Union[ResourceTags, _Mapping]] = ..., timestamp: _Optional[int] = ..., max_hops: _Optional[int] = ..., ttl: _Optional[int] = ..., current_hops: _Optional[int] = ...) -> None: ...

class ResourceQueryResponse(_message.Message):
    __slots__ = ("query_id", "responder_node_id", "responder_address", "available_nodes", "timestamp", "is_final")
    QUERY_ID_FIELD_NUMBER: _ClassVar[int]
    RESPONDER_NODE_ID_FIELD_NUMBER: _ClassVar[int]
    RESPONDER_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_NODES_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    IS_FINAL_FIELD_NUMBER: _ClassVar[int]
    query_id: str
    responder_node_id: str
    responder_address: str
    available_nodes: _containers.RepeatedCompositeFieldContainer[PeerNodeInfo]
    timestamp: int
    is_final: bool
    def __init__(self, query_id: _Optional[str] = ..., responder_node_id: _Optional[str] = ..., responder_address: _Optional[str] = ..., available_nodes: _Optional[_Iterable[_Union[PeerNodeInfo, _Mapping]]] = ..., timestamp: _Optional[int] = ..., is_final: bool = ...) -> None: ...

class PeerListExchangeRequest(_message.Message):
    __slots__ = ("requester_node_id", "requester_address", "requester_domain_id", "known_peers", "timestamp")
    REQUESTER_NODE_ID_FIELD_NUMBER: _ClassVar[int]
    REQUESTER_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    REQUESTER_DOMAIN_ID_FIELD_NUMBER: _ClassVar[int]
    KNOWN_PEERS_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    requester_node_id: str
    requester_address: str
    requester_domain_id: str
    known_peers: _containers.RepeatedScalarFieldContainer[str]
    timestamp: int
    def __init__(self, requester_node_id: _Optional[str] = ..., requester_address: _Optional[str] = ..., requester_domain_id: _Optional[str] = ..., known_peers: _Optional[_Iterable[str]] = ..., timestamp: _Optional[int] = ...) -> None: ...

class PeerListExchangeResponse(_message.Message):
    __slots__ = ("known_peers", "timestamp")
    KNOWN_PEERS_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    known_peers: _containers.RepeatedScalarFieldContainer[str]
    timestamp: int
    def __init__(self, known_peers: _Optional[_Iterable[str]] = ..., timestamp: _Optional[int] = ...) -> None: ...

class GetLocalNodeInfoRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetLocalNodeInfoResponse(_message.Message):
    __slots__ = ("node_info",)
    NODE_INFO_FIELD_NUMBER: _ClassVar[int]
    node_info: PeerNodeInfo
    def __init__(self, node_info: _Optional[_Union[PeerNodeInfo, _Mapping]] = ...) -> None: ...
