"""
Store 服务的 gRPC 客户端
负责与 Store 服务通信，包括地址解析、对象获取和保存
"""

import logging
import re
import uuid
from collections.abc import Iterator
from typing import Optional

import grpc

from proto.common import types_pb2 as common
from proto.resource.store import store_pb2 as store_pb
from proto.resource.store import store_pb2_grpc as store_grpc

logger = logging.getLogger(__name__)


class StoreClient:
    """Store 服务的 gRPC 客户端，用于获取和保存对象"""
    
    def __init__(self, store_addr: str):
        """
        初始化 Store 客户端
        
        Args:
            store_addr: Store 服务的地址（格式：host:port）
        """
        # 基于 /etc/hosts 手动解析 IPv4 地址
        target = self._resolve_from_hosts(store_addr)
        
        # 配置 channel options，禁用 IPv6
        options = [
            ('grpc.ipv6', 0),  # 禁用 IPv6，强制使用 IPv4
        ]
        self.channel = grpc.insecure_channel(target, options=options)
        self.stub = store_grpc.ServiceStub(self.channel)
    
    @staticmethod
    def _resolve_from_hosts(store_addr: str) -> str:
        """
        从 /etc/hosts 文件解析主机名为 IPv4 地址
        
        如果地址已经是 ipv4: 或 unix: 前缀，直接返回
        否则尝试从 /etc/hosts 查找主机名对应的 IPv4 地址
        
        Args:
            store_addr: 原始地址（格式：host:port 或 ipv4:host:port 或 unix:/path）
            
        Returns:
            规范化后的目标地址（ipv4:ip:port 格式）
        """
        # 如果已经有前缀，直接返回
        if store_addr.startswith(("ipv4:", "unix:")):
            return store_addr
        
        # 解析 host:port
        if ":" not in store_addr:
            logger.warning(f"Store address missing port: {store_addr}")
            return store_addr
        
        host, port = store_addr.rsplit(":", 1)
        
        # 如果 host 已经是 IP 地址，直接使用 ipv4: 前缀
        if re.match(r'^\d+\.\d+\.\d+\.\d+$', host):
            return f"ipv4:{store_addr}"
        
        # 从 /etc/hosts 查找主机名对应的 IPv4 地址
        ipv4 = StoreClient._lookup_hosts(host)
        if ipv4:
            target = f"ipv4:{ipv4}:{port}"
            logger.debug(f"Resolved {store_addr} from /etc/hosts -> {target}")
            return target
        
        # 如果 /etc/hosts 中没有找到，使用原始地址（让 gRPC 自己解析）
        logger.warning(f"Host {host} not found in /etc/hosts, using original address")
        return store_addr
    
    @staticmethod
    def _lookup_hosts(hostname: str) -> Optional[str]:
        """
        从 /etc/hosts 文件查找主机名对应的 IPv4 地址
        
        Args:
            hostname: 主机名
            
        Returns:
            IPv4 地址，如果未找到返回 None
        """
        try:
            with open('/etc/hosts', 'r') as f:
                for line in f:
                    # 跳过注释和空行
                    line = line.strip()
                    if not line or line.startswith('#'):
                        continue
                    
                    # 解析 /etc/hosts 格式：IP地址 主机名1 主机名2 ...
                    parts = line.split()
                    if len(parts) < 2:
                        continue
                    
                    ip = parts[0]
                    # 检查是否为 IPv4 地址
                    if re.match(r'^\d+\.\d+\.\d+\.\d+$', ip):
                        # 检查主机名是否匹配（支持完整匹配或部分匹配）
                        for host in parts[1:]:
                            if host == hostname or host.endswith(f'.{hostname}') or hostname.endswith(f'.{host}'):
                                return ip
        except FileNotFoundError:
            logger.debug("/etc/hosts file not found")
        except Exception as e:
            logger.warning(f"Error reading /etc/hosts: {e}")
        
        return None

    def iter_stream_chunks(self, object_id: str) -> Iterator[common.StreamChunk]:
        """
        迭代获取流对象的所有 chunk。

        Args:
            object_id: 流对象 ID

        Yields:
            common.StreamChunk
        """
        offset = 0
        while True:
            try:
                request = store_pb.GetStreamChunkRequest(
                    ObjectID=object_id,
                    Offset=offset
                )
                response = self.stub.GetStreamChunk(request, timeout=60)
            except Exception as e:
                logger.error(f"Failed to get stream chunk for {object_id}: {e}")
                raise

            chunk = response.Chunk if response else None
            if chunk is None:
                logger.error(f"Stream {object_id} returned empty chunk, stop iteration")
                break

            yield chunk

            if chunk.EoS:
                logger.info(f"Stream {object_id} reached EOS")
                break

            offset = chunk.Offset + 1

    def save_stream_chunk(self, chunk: common.StreamChunk) -> bool:
        """
        保存单个流式 chunk 到 Store。
        """
        try:
            request = store_pb.SaveStreamChunkRequest(Chunk=chunk)
            self.stub.SaveStreamChunk(request)
            return True
        except Exception as e:
            logger.error(
                f"Failed to save stream chunk: obj={chunk.ObjectID}, offset={chunk.Offset}, error={e}"
            )
            return False

    def close(self):
        """关闭 gRPC 连接"""
        if hasattr(self, 'channel'):
            self.channel.close()

    def get_object(self, object_id: str, source: str = "") -> Optional[common.EncodedObject]:
        """
        从 Store 获取对象
        
        Args:
            object_id: 对象 ID
            source: 对象来源（可选）
            
        Returns:
            编码后的对象（common.EncodedObject），如果获取失败返回 None
        """
        try:
            request = store_pb.GetObjectRequest(
                ObjectRef=common.ObjectRef(ID=object_id, Source=source)
            )
            response = self.stub.GetObject(request)
            return response.Object
        except Exception as e:
            logger.error(f"Failed to get object {object_id}: {e}")
            return None

    def save_object(
        self, 
        data: bytes, 
        language: common.Language, 
        object_id: str = "", 
        is_stream: bool = False
    ) -> Optional[common.ObjectRef]:
        """
        保存对象到 Store
        
        Args:
            data: 对象的字节数据
            language: 对象的语言类型（common.Language）
            object_id: 对象 ID（如果为空则自动生成）
            is_stream: 是否为流对象
            
        Returns:
            对象引用（common.ObjectRef），如果保存失败返回 None
        """
        try:
            if not object_id:
                object_id = f"obj.{uuid.uuid4()}"
            
            request = store_pb.SaveObjectRequest(
                Object=common.EncodedObject(
                    ID=object_id,
                    Data=data,
                    Language=language,
                    IsStream=is_stream,
                )
            )
            response = self.stub.SaveObject(request)
            if response.Success:
                return response.ObjectRef
            else:
                logger.error(f"Failed to save object: {response.Error}")
                return None
        except Exception as e:
            logger.error(f"Failed to save object: {e}")
            return None

