"""
Ray式交互上下文实现

这个模块实现了类似ignis的Ray式交互计算模式：
- 实时交互：支持动态任务提交和实时结果获取
- 长连接维持：保持与计算集群的持续连接
- 流式处理：支持数据流的实时处理
- 交互式调试：适合开发和调试场景
"""

import time
import threading
import queue
import uuid
import logging
from typing import Optional, Dict, Any, Callable
from .base import IActorContext, ComputeMode, TaskStatus, TaskResult, ActorContextConfig
from ..protos.controller import controller_pb2
from ..protos import platform_pb2
import grpc


logger = logging.getLogger(__name__)


class RayInteractiveContext(IActorContext):
    """Ray式交互上下文 - 实时交互计算模式"""
    
    def __init__(self, config: ActorContextConfig):
        self.config = config
        self._compute_mode = ComputeMode.RAY_INTERACTIVE
        self._connected = False
        self._channel = None
        self._stub = None
        self._response_stream = None
        self._message_queue = queue.Queue()
        self._result_map = {}
        self._result_callback = None
        self._background_thread = None
        self._shutdown_event = threading.Event()
        
        # 连接到远程服务
        self._connect()
    
    @property
    def compute_mode(self) -> ComputeMode:
        """获取计算模式"""
        return self._compute_mode
    
    @property
    def context_type(self) -> str:
        """返回上下文类型"""
        return "ray_interactive"
    
    @property
    def is_connected(self) -> bool:
        """检查连接状态"""
        return self._connected
    
    def _connect(self):
        """建立gRPC连接"""
        try:
            # 创建gRPC通道
            options = [
                ('grpc.max_send_message_length', self.config.max_message_length),
                ('grpc.max_receive_message_length', self.config.max_message_length),
                ('grpc.keepalive_time_ms', 30000),
                ('grpc.keepalive_timeout_ms', 5000),
                ('grpc.keepalive_permit_without_calls', True),
                ('grpc.http2.max_pings_without_data', 0),
                ('grpc.http2.min_time_between_pings_ms', 10000),
                ('grpc.http2.min_ping_interval_without_data_ms', 300000)
            ]
            
            self._channel = grpc.insecure_channel(self.config.master_address, options=options)
            self._stub = controller_pb2_grpc.ControllerStub(self._channel)
            
            # 创建双向流
            self._response_stream = self._stub.MessageStream(self._generate_messages())
            
            # 启动后台线程处理响应
            self._background_thread = threading.Thread(target=self._process_responses, daemon=True)
            self._background_thread.start()
            
            self._connected = True
            logger.info(f"Ray交互模式连接成功: {self.config.master_address}")
            
        except Exception as e:
            logger.error(f"Ray交互模式连接失败: {e}")
            raise ConnectionError(f"无法连接到 {self.config.master_address}: {e}")
    
    def _generate_messages(self):
        """生成消息流"""
        while not self._shutdown_event.is_set():
            try:
                # 从队列获取消息，超时1秒
                message = self._message_queue.get(timeout=1.0)
                yield message
            except queue.Empty:
                continue
            except Exception as e:
                logger.error(f"消息生成错误: {e}")
                break
    
    def _process_responses(self):
        """处理响应流"""
        try:
            for response in self._response_stream:
                if self._shutdown_event.is_set():
                    break
                
                self._handle_response(response)
                
        except Exception as e:
            if not self._shutdown_event.is_set():
                logger.error(f"响应处理错误: {e}")
                self._connected = False
    
    def _handle_response(self, response: controller_pb2.Message):
        """处理单个响应"""
        try:
            if response.Type == controller_pb2.CommandType.BK_RETURN_RESULT:
                result = response.ReturnResult
                session_id = result.SessionID
                instance_id = result.InstanceID
                name = result.Name
                value = result.Value
                
                # 构造结果键
                key = f"{session_id}-{instance_id}-{name}"
                self._result_map[key] = value
                
                # 调用回调函数
                if self._result_callback:
                    try:
                        self._result_callback(key, value)
                    except Exception as e:
                        logger.error(f"结果回调执行错误: {e}")
                
                logger.debug(f"Ray模式收到结果: {key}")
                
        except Exception as e:
            logger.error(f"响应处理错误: {e}")
    
    # Ray式交互模式方法实现
    def get_result(self, key: str) -> Optional[platform_pb2.Flow]:
        """获取执行结果（Ray式同步获取）"""
        # 实时获取结果，支持轮询等待
        max_wait_time = 60  # 最大等待60秒
        start_time = time.time()
        
        while time.time() - start_time < max_wait_time:
            if key in self._result_map:
                return self._result_map[key]
            time.sleep(0.1)  # 100ms轮询间隔
        
        logger.warning(f"Ray模式获取结果超时: {key}")
        return None
    
    def send(self, message: controller_pb2.Message):
        """发送消息（Ray式实时发送）"""
        if not self._connected:
            raise ConnectionError("Ray交互模式未连接")
        
        try:
            self._message_queue.put(message, timeout=5.0)
            logger.debug(f"Ray模式发送消息: {message.Type}")
        except queue.Full:
            raise RuntimeError("Ray交互模式消息队列已满")
    
    def set_result_callback(self, callback: Callable[[str, Any], None]) -> None:
        """设置结果回调（Ray式实时回调）"""
        self._result_callback = callback
        logger.info("Ray模式设置结果回调")
    
    # Flink式批处理模式方法（在Ray模式下不支持）
    def submit_task(self, task_name: str, params: Dict[str, Any]) -> str:
        """提交任务（Ray模式不支持此方法）"""
        raise NotImplementedError("Ray交互模式不支持Flink式任务提交，请使用send()方法")
    
    def get_task_status(self, task_id: str) -> TaskResult:
        """获取任务状态（Ray模式不支持此方法）"""
        raise NotImplementedError("Ray交互模式不支持Flink式任务状态查询")
    
    def cancel_task(self, task_id: str) -> bool:
        """取消任务（Ray模式不支持此方法）"""
        raise NotImplementedError("Ray交互模式不支持Flink式任务取消")
    
    # 通用方法
    def close(self):
        """关闭连接"""
        if not self._connected:
            return
        
        logger.info("关闭Ray交互模式连接")
        
        # 设置关闭标志
        self._shutdown_event.set()
        
        # 等待后台线程结束
        if self._background_thread and self._background_thread.is_alive():
            self._background_thread.join(timeout=5.0)
        
        # 关闭gRPC连接
        if self._channel:
            self._channel.close()
        
        self._connected = False
        self._result_map.clear()
        
        logger.info("Ray交互模式连接已关闭")


# 为了兼容性，需要导入gRPC stub
try:
    from ..protos.controller import controller_pb2_grpc
except ImportError:
    logger.warning("无法导入controller_pb2_grpc，Ray交互模式可能无法正常工作")
    controller_pb2_grpc = None