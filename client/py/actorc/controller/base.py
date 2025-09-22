"""
ActorContext 抽象基类和接口定义

这个模块定义了 ActorContext 的抽象接口，支持两种计算模式：
- Ray式交互模式：实时交互，适合开发调试
- Flink式提交模式：批处理提交，适合生产环境
"""

from abc import ABC, abstractmethod
from typing import Optional, Dict, Any, Callable
from enum import Enum
from lucas.serverless_function import Metadata
from ..protos.controller import controller_pb2
from ..protos import platform_pb2


class ComputeMode(Enum):
    """计算模式枚举"""
    RAY_INTERACTIVE = "ray_interactive"  # Ray式交互模式 - 类似ignis的实时交互
    FLINK_BATCH = "flink_batch"         # Flink式批处理模式 - 类似IARNet的提交式


class TaskStatus(Enum):
    """任务状态枚举"""
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"


class TaskResult:
    """任务结果封装"""
    def __init__(self, task_id: str, status: TaskStatus, result: Any = None, error: str = None):
        self.task_id = task_id
        self.status = status
        self.result = result
        self.error = error


class IActorContext(ABC):
    """ActorContext 抽象接口 - 支持双模式计算"""
    
    @property
    @abstractmethod
    def compute_mode(self) -> ComputeMode:
        """获取计算模式"""
        pass
    
    # Ray式交互模式方法
    @abstractmethod
    def get_result(self, key: str) -> Optional[platform_pb2.Flow]:
        """获取执行结果（Ray式同步获取）"""
        pass
    
    @abstractmethod
    def send(self, message: controller_pb2.Message):
        """发送消息（Ray式实时发送）"""
        pass
    
    @abstractmethod
    def set_result_callback(self, callback: Callable[[str, Any], None]) -> None:
        """设置结果回调（Ray式实时回调）"""
        pass
    
    # Flink式批处理模式方法
    @abstractmethod
    def submit_task(self, task_name: str, params: Dict[str, Any]) -> str:
        """提交任务（Flink式异步提交），返回任务ID"""
        pass
    
    @abstractmethod
    def get_task_status(self, task_id: str) -> TaskResult:
        """获取任务状态（Flink式状态查询）"""
        pass
    
    @abstractmethod
    def cancel_task(self, task_id: str) -> bool:
        """取消任务"""
        pass
    
    # 通用方法
    @abstractmethod
    def close(self):
        """关闭上下文连接"""
        pass
    
    @property
    @abstractmethod
    def is_connected(self) -> bool:
        """检查连接状态"""
        pass
    
    @property
    @abstractmethod
    def context_type(self) -> str:
        """返回上下文类型"""
        pass


class ActorContextConfig:
    """ActorContext 配置类"""
    
    def __init__(
        self,
        compute_mode: ComputeMode = ComputeMode.RAY_INTERACTIVE,
        master_address: str = "localhost:50051",
        max_message_length: int = 512 * 1024 * 1024,
        connection_timeout: int = 30,
        retry_attempts: int = 3,
        **kwargs
    ):
        self.compute_mode = compute_mode
        self.master_address = master_address
        self.max_message_length = max_message_length
        self.connection_timeout = connection_timeout
        self.retry_attempts = retry_attempts
        self.extra_config = kwargs
    
    def to_dict(self) -> Dict[str, Any]:
        """转换为字典格式"""
        return {
            "compute_mode": self.compute_mode.value,
            "master_address": self.master_address,
            "max_message_length": self.max_message_length,
            "connection_timeout": self.connection_timeout,
            "retry_attempts": self.retry_attempts,
            **self.extra_config
        }


class ContextError(Exception):
    """ActorContext 相关异常"""
    pass


class ConnectionError(ContextError):
    """连接异常"""
    pass


class ConfigurationError(ContextError):
    """配置异常"""
    pass