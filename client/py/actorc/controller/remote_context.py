"""
远程 ActorContext 实现

这个模块实现了连接远程 ActorC 服务的 ActorContext，
基于原有的 gRPC 连接实现，但增加了接口兼容性和错误处理。
"""

import grpc
import logging
import threading
import time
import queue
from typing import Optional, Dict, Any
from .base import IActorContext, ActorContextConfig, ConnectionError
from ..protos.controller import controller_pb2, controller_pb2_grpc
from ..protos import platform_pb2


logger = logging.getLogger(__name__)


class RemoteActorContext(IActorContext):
    """远程 ActorContext 实现"""
    
    def __init__(self, config: ActorContextConfig):
        self._config = config
        self._master_address = config.master_address
        self._channel = None
        self._stub = None
        self._q = queue.Queue()
        self._response_stream = None
        self._result_map: Dict[str, platform_pb2.Flow] = {}
        self._thread = None
        self._is_connected = False
        self._lock = threading.Lock()
        
        # 尝试建立连接
        self._connect()
    
    def _connect(self):
        """建立 gRPC 连接"""
        try:
            # 创建 gRPC 通道
            self._channel = grpc.insecure_channel(
                self._master_address,
                options=[
                    ("grpc.max_receive_message_length", self._config.max_message_length),
                    ("grpc.keepalive_time_ms", 30000),
                    ("grpc.keepalive_timeout_ms", 5000),
                    ("grpc.keepalive_permit_without_calls", True),
                    ("grpc.http2.max_pings_without_data", 0),
                    ("grpc.http2.min_time_between_pings_ms", 10000),
                    ("grpc.http2.min_ping_interval_without_data_ms", 300000)
                ],
            )
            
            # 创建 stub
            self._stub = controller_pb2_grpc.ServiceStub(self._channel)
            
            # 测试连接
            try:
                # 设置连接超时
                grpc.channel_ready_future(self._channel).result(
                    timeout=self._config.connection_timeout
                )
                
                # 启动响应流
                self._response_stream = self._stub.Session(self._generate())
                
                # 启动后台线程处理响应
                self._thread = threading.Thread(target=self._run, daemon=True)
                self._thread.start()
                
                self._is_connected = True
                logger.info(f"成功连接到远程 ActorC 服务: {self._master_address}")
                
            except grpc.FutureTimeoutError:
                raise ConnectionError(f"连接超时: {self._master_address}")
            except grpc.RpcError as e:
                raise ConnectionError(f"gRPC 连接错误: {e}")
                
        except Exception as e:
            logger.error(f"连接远程 ActorC 服务失败: {e}")
            self._is_connected = False
            raise ConnectionError(f"无法连接到 {self._master_address}: {e}")
    
    def _generate(self):
        """生成消息流"""
        while self._is_connected:
            try:
                msg = self._q.get(timeout=1.0)
                yield msg
            except queue.Empty:
                continue
            except Exception as e:
                logger.error(f"消息生成器错误: {e}")
                break
    
    def _run(self):
        """后台线程处理响应"""
        retry_count = 0
        max_retries = self._config.retry_attempts
        
        while self._is_connected and retry_count < max_retries:
            try:
                for response in self._response_stream:
                    if not self._is_connected:
                        break
                        
                    response: controller_pb2.Message
                    if response.Type == controller_pb2.CommandType.BK_RETURN_RESULT:
                        result: controller_pb2.ReturnResult = response.ReturnResult
                        sessionID = result.SessionID
                        instanceID = result.InstanceID
                        name = result.Name
                        value = result.Value
                        key = f"{sessionID}-{instanceID}-{name}"
                        
                        with self._lock:
                            self._result_map[key] = value
                        
                        logger.debug(f"收到执行结果: {key}")
                
                # 如果正常退出循环，重置重试计数
                retry_count = 0
                
            except grpc.RpcError as e:
                retry_count += 1
                logger.warning(f"gRPC 流错误 (重试 {retry_count}/{max_retries}): {e}")
                
                if retry_count < max_retries:
                    time.sleep(min(2 ** retry_count, 10))  # 指数退避
                    try:
                        # 重新建立流连接
                        self._response_stream = self._stub.Session(self._generate())
                    except Exception as reconnect_error:
                        logger.error(f"重连失败: {reconnect_error}")
                else:
                    logger.error("达到最大重试次数，停止重连")
                    self._is_connected = False
                    
            except Exception as e:
                logger.error(f"响应处理线程异常: {e}")
                self._is_connected = False
                break
        
        logger.info("响应处理线程已退出")
    
    def get_result(self, key: str) -> Optional[platform_pb2.Flow]:
        """获取执行结果"""
        with self._lock:
            return self._result_map.get(key)
    
    def send(self, message: controller_pb2.Message):
        """发送消息"""
        if not self._is_connected:
            raise ConnectionError("ActorContext 未连接")
        
        try:
            self._q.put(message, timeout=5.0)
            logger.debug(f"发送消息: {message.Type}")
        except queue.Full:
            raise ConnectionError("消息队列已满，发送超时")
    
    def close(self):
        """关闭上下文连接"""
        logger.info("正在关闭远程 ActorContext...")
        
        with self._lock:
            self._is_connected = False
        
        # 等待后台线程结束
        if self._thread and self._thread.is_alive():
            self._thread.join(timeout=5.0)
        
        # 关闭 gRPC 连接
        if self._channel:
            try:
                self._channel.close()
            except Exception as e:
                logger.warning(f"关闭 gRPC 通道时出错: {e}")
        
        # 清理资源
        with self._lock:
            self._result_map.clear()
            
        # 清空消息队列
        while not self._q.empty():
            try:
                self._q.get_nowait()
            except queue.Empty:
                break
        
        logger.info("远程 ActorContext 已关闭")
    
    @property
    def is_connected(self) -> bool:
        """检查连接状态"""
        return self._is_connected
    
    @property
    def context_type(self) -> str:
        """返回上下文类型"""
        return "remote"
    
    def get_connection_info(self) -> Dict[str, Any]:
        """获取连接信息（调试用）"""
        return {
            "master_address": self._master_address,
            "is_connected": self._is_connected,
            "result_count": len(self._result_map),
            "queue_size": self._q.qsize()
        }