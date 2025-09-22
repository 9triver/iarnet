"""
ActorContext 工厂和环境检测

这个模块负责检测当前运行环境和计算模式，并创建相应的 ActorContext 实现。
支持以下计算模式：
- Ray式交互模式：实时交互，适合开发调试
- Flink式批处理模式：提交后分离，适合生产环境

支持以下环境：
- 本地开发环境
- Docker 容器环境  
- Kubernetes 集群环境
- 自定义远程环境
"""

import os
import socket
import logging
from typing import Optional, Dict, Any
from .base import IActorContext, ActorContextConfig, ComputeMode, ConfigurationError


logger = logging.getLogger(__name__)


class EnvironmentDetector:
    """环境检测器"""
    
    @staticmethod
    def detect_environment() -> str:
        """
        检测当前运行环境
        
        Returns:
            str: 环境类型 ('local', 'docker', 'kubernetes', 'remote')
        """
        # 检测 Kubernetes 环境
        if EnvironmentDetector._is_kubernetes():
            return 'kubernetes'
        
        # 检测 Docker 环境
        if EnvironmentDetector._is_docker():
            return 'docker'
        
        # 检测是否有远程配置
        if EnvironmentDetector._has_remote_config():
            return 'remote'
        
        # 默认本地环境
        return 'local'
    
    @staticmethod
    def _is_kubernetes() -> bool:
        """检测是否在 Kubernetes 环境中运行"""
        # 检查 Kubernetes 服务账户文件
        k8s_token_path = "/var/run/secrets/kubernetes.io/serviceaccount/token"
        if os.path.exists(k8s_token_path):
            return True
        
        # 检查环境变量
        k8s_env_vars = [
            "KUBERNETES_SERVICE_HOST",
            "KUBERNETES_SERVICE_PORT",
            "KUBERNETES_PORT"
        ]
        return any(os.getenv(var) for var in k8s_env_vars)
    
    @staticmethod
    def _is_docker() -> bool:
        """检测是否在 Docker 容器中运行"""
        # 检查 .dockerenv 文件
        if os.path.exists("/.dockerenv"):
            return True
        
        # 检查 cgroup 信息
        try:
            with open("/proc/1/cgroup", "r") as f:
                content = f.read()
                return "docker" in content or "containerd" in content
        except (FileNotFoundError, PermissionError):
            pass
        
        # 检查环境变量
        return os.getenv("DOCKER_CONTAINER") == "true"
    
    @staticmethod
    def _has_remote_config() -> bool:
        """检测是否有远程配置"""
        # 检查环境变量中的远程地址配置
        remote_vars = [
            "ACTOR_MASTER_ADDRESS",
            "IARNET_MASTER_ADDRESS", 
            "ACTORC_REMOTE_ADDRESS"
        ]
        return any(os.getenv(var) for var in remote_vars)
    
    @staticmethod
    def get_master_address(env_type: str) -> str:
        """
        根据环境类型获取主节点地址
        
        Args:
            env_type: 环境类型
            
        Returns:
            str: 主节点地址
        """
        # 优先使用环境变量配置
        env_address = (
            os.getenv("ACTOR_MASTER_ADDRESS") or
            os.getenv("IARNET_MASTER_ADDRESS") or 
            os.getenv("ACTORC_REMOTE_ADDRESS")
        )
        if env_address:
            return env_address
        
        if env_type == 'kubernetes':
            # Kubernetes 环境下的服务发现
            service_name = os.getenv("IARNET_SERVICE_NAME", "iarnet-service")
            namespace = os.getenv("IARNET_NAMESPACE", "default")
            port = os.getenv("IARNET_SERVICE_PORT", "50051")
            return f"{service_name}.{namespace}.svc.cluster.local:{port}"
        
        elif env_type == 'docker':
            # Docker 环境下的网络发现
            host = os.getenv("IARNET_HOST", "iarnet-master")
            port = os.getenv("IARNET_PORT", "50051")
            return f"{host}:{port}"
        
        elif env_type == 'remote':
            # 远程环境配置
            return os.getenv("IARNET_REMOTE_ADDRESS", "localhost:50051")
        
        else:  # local
            # 本地开发环境
            return "localhost:50051"


class ActorContextFactory:
    """ActorContext 工厂类"""
    
    _context_cache: Optional[IActorContext] = None
    
    @classmethod
    def detect_compute_mode(cls) -> ComputeMode:
        """
        检测计算模式
        
        Returns:
            ComputeMode: 检测到的计算模式
        """
        # 检查环境变量配置
        mode_env = os.getenv("IARNET_COMPUTE_MODE", "").lower()
        if mode_env == "ray":
            return ComputeMode.RAY_INTERACTIVE
        elif mode_env == "flink":
            return ComputeMode.FLINK_BATCH
        
        # 检查是否在开发环境（倾向于交互模式）
        if os.getenv("IARNET_DEV_MODE") == "true":
            return ComputeMode.RAY_INTERACTIVE
        
        # 检查是否在生产环境（倾向于批处理模式）
        if os.getenv("IARNET_PROD_MODE") == "true":
            return ComputeMode.FLINK_BATCH
        
        # 根据环境类型推断
        env_type = EnvironmentDetector.detect_environment()
        if env_type == 'local':
            # 本地环境默认使用交互模式
            return ComputeMode.RAY_INTERACTIVE
        else:
            # 远程环境默认使用批处理模式
            return ComputeMode.FLINK_BATCH
    
    @classmethod
    def create_context(
        cls,
        master_address: Optional[str] = None,
        force_type: Optional[str] = None,
        compute_mode: Optional[ComputeMode] = None,
        config: Optional[ActorContextConfig] = None
    ) -> IActorContext:
        """
        创建 ActorContext 实例
        
        Args:
            master_address: 主节点地址（可选，会自动检测）
            force_type: 强制指定上下文类型（可选）
            compute_mode: 计算模式（可选，会自动检测）
            config: 配置对象（可选）
            
        Returns:
            IActorContext: ActorContext 实例
        """
        # 检测计算模式
        if not compute_mode:
            compute_mode = cls.detect_compute_mode()
        
        # 如果已有缓存的上下文且配置匹配，直接返回
        if (cls._context_cache and 
            cls._context_cache.is_connected and
            hasattr(cls._context_cache, '_config') and
            cls._context_cache._config.compute_mode == compute_mode):
            return cls._context_cache
        
        # 检测环境类型
        env_type = force_type or EnvironmentDetector.detect_environment()
        logger.info(f"检测到环境类型: {env_type}, 计算模式: {compute_mode.value}")
        
        # 获取主节点地址
        if not master_address:
            master_address = EnvironmentDetector.get_master_address(env_type)
        
        # 创建配置
        if not config:
            config = ActorContextConfig(
                master_address=master_address,
                compute_mode=compute_mode
            )
        else:
            config.master_address = master_address
            config.compute_mode = compute_mode
        
        # 根据计算模式创建相应的上下文
        if compute_mode == ComputeMode.RAY_INTERACTIVE:
            from .ray_context import RayInteractiveContext
            context = RayInteractiveContext(config)
        elif compute_mode == ComputeMode.FLINK_BATCH:
            # 根据环境选择具体的Flink实现
            if env_type in ['kubernetes', 'docker', 'remote']:
                from .flink_context import FlinkBatchContextWithGRPC
                context = FlinkBatchContextWithGRPC(config)
            else:
                from .flink_context import FlinkBatchContext
                context = FlinkBatchContext(config)
        else:
            raise ConfigurationError(f"不支持的计算模式: {compute_mode}")
        
        # 缓存上下文
        cls._context_cache = context
        logger.info(f"创建 {context.context_type} ActorContext: {master_address}")
        
        return context
    
    @classmethod
    def get_current_context(cls) -> Optional[IActorContext]:
        """获取当前缓存的上下文"""
        return cls._context_cache
    
    @classmethod
    def clear_cache(cls):
        """清除缓存的上下文"""
        if cls._context_cache:
            try:
                cls._context_cache.close()
            except Exception as e:
                logger.warning(f"关闭缓存上下文时出错: {e}")
            cls._context_cache = None


# 全局工厂实例
context_factory = ActorContextFactory()