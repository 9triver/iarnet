"""
本地 ActorContext 实现

这个模块实现了本地开发环境下的 ActorContext，
主要用于本地测试和开发，不需要连接远程服务。
"""

import logging
import threading
import time
from typing import Optional, Dict, Any
from .base import IActorContext, ActorContextConfig, ConnectionError
from ..protos.controller import controller_pb2
from ..protos import platform_pb2


logger = logging.getLogger(__name__)


class LocalActorContext(IActorContext):
    """本地 ActorContext 实现"""
    
    def __init__(self, config: ActorContextConfig):
        self._config = config
        self._result_map: Dict[str, platform_pb2.Flow] = {}
        self._message_queue: list = []
        self._is_connected = True
        self._lock = threading.Lock()
        
        logger.info(f"初始化本地 ActorContext: {config.master_address}")
    
    def get_result(self, key: str) -> Optional[platform_pb2.Flow]:
        """获取执行结果"""
        with self._lock:
            return self._result_map.get(key)
    
    def send(self, message: controller_pb2.Message):
        """发送消息（本地模拟）"""
        with self._lock:
            self._message_queue.append(message)
            logger.debug(f"本地模拟发送消息: {message.Type}")
            
            # 本地模拟处理消息
            self._process_message_locally(message)
    
    def _process_message_locally(self, message: controller_pb2.Message):
        """本地模拟处理消息"""
        if message.Type == controller_pb2.CommandType.FR_APPEND_PY_FUNC:
            func_info = message.AppendPyFunc
            logger.info(f"本地注册函数: {func_info.Name}")
            
        elif message.Type == controller_pb2.CommandType.FR_APPEND_ARG:
            arg_info = message.AppendArg
            key = f"{arg_info.SessionID}-{arg_info.InstanceID}-{arg_info.Name}"
            
            # 本地模拟执行并返回结果
            mock_result = platform_pb2.Flow()
            mock_result.Type = platform_pb2.Flow.FlowType.FLOW_ENCODED
            mock_result.Encoded.Data = b"mock_local_result"
            mock_result.Encoded.Language = platform_pb2.LANG_PYTHON
            
            self._result_map[key] = mock_result
            logger.info(f"本地模拟执行函数: {arg_info.Name}")
    
    def close(self):
        """关闭上下文连接"""
        with self._lock:
            self._is_connected = False
            self._result_map.clear()
            self._message_queue.clear()
        logger.info("本地 ActorContext 已关闭")
    
    @property
    def is_connected(self) -> bool:
        """检查连接状态"""
        return self._is_connected
    
    @property
    def context_type(self) -> str:
        """返回上下文类型"""
        return "local"
    
    def get_message_count(self) -> int:
        """获取消息队列长度（调试用）"""
        with self._lock:
            return len(self._message_queue)
    
    def get_result_count(self) -> int:
        """获取结果数量（调试用）"""
        with self._lock:
            return len(self._result_map)