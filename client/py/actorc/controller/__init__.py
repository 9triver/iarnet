"""
ActorContext 控制器模块

提供基于计算模式的ActorContext实现：
- Ray式交互模式：实时交互，适合开发调试
- Flink式批处理模式：提交后分离，适合生产环境
"""

from .base import (
    IActorContext,
    ActorContextConfig,
    ComputeMode,
    TaskStatus,
    TaskResult,
    ContextError,
    ConnectionError,
    ConfigurationError
)

from .factory import (
    ActorContextFactory,
    EnvironmentDetector,
    context_factory
)

from .ray_context import RayInteractiveContext
from .flink_context import FlinkBatchContext, FlinkBatchContextWithGRPC

# 保持向后兼容
from .local_context import LocalActorContext
from .remote_context import RemoteActorContext

__all__ = [
    # 基础接口和类型
    'IActorContext',
    'ActorContextConfig', 
    'ComputeMode',
    'TaskStatus',
    'TaskResult',
    'ContextError',
    'ConnectionError',
    'ConfigurationError',
    
    # 工厂类
    'ActorContextFactory',
    'EnvironmentDetector',
    'context_factory',
    
    # 新的计算模式实现
    'RayInteractiveContext',
    'FlinkBatchContext',
    'FlinkBatchContextWithGRPC',
    
    # 向后兼容的实现
    'LocalActorContext',
    'RemoteActorContext',
]