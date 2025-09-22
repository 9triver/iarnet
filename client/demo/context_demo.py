#!/usr/bin/env python3
"""
ActorContext 接口演示

这个示例展示了如何使用新的 ActorContext 接口，包括：
1. 自动环境检测
2. 手动指定上下文类型
3. 配置自定义参数
4. 获取上下文信息
"""

import os
import sys
import logging
from typing import Optional

# 添加 actorc 到路径
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'py'))

from actorc import (
    ActorContext, 
    ActorContextConfig, 
    ActorContextFactory,
    EnvironmentDetector,
    IActorContext
)

# 设置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def demo_environment_detection():
    """演示环境检测功能"""
    print("=== 环境检测演示 ===")
    
    # 检测当前环境
    env_type = EnvironmentDetector.detect_environment()
    print(f"检测到的环境类型: {env_type}")
    
    # 获取推荐的主节点地址
    master_address = EnvironmentDetector.get_master_address(env_type)
    print(f"推荐的主节点地址: {master_address}")
    
    # 检测各种环境特征
    print(f"是否在 Kubernetes 中: {EnvironmentDetector._is_kubernetes()}")
    print(f"是否在 Docker 中: {EnvironmentDetector._is_docker()}")
    print(f"是否有远程配置: {EnvironmentDetector._has_remote_config()}")
    print()


def demo_auto_context_creation():
    """演示自动上下文创建"""
    print("=== 自动上下文创建演示 ===")
    
    try:
        # 使用自动检测创建上下文
        context = ActorContext.createContext()
        
        print(f"上下文类型: {context.context_type}")
        print(f"连接状态: {context.is_connected}")
        
        # 获取当前上下文
        current = ActorContext.getCurrentContext()
        print(f"当前上下文与创建的上下文相同: {current is context}")
        
        return context
        
    except Exception as e:
        logger.error(f"创建上下文失败: {e}")
        return None


def demo_manual_context_creation():
    """演示手动上下文创建"""
    print("=== 手动上下文创建演示 ===")
    
    try:
        # 创建自定义配置
        config = ActorContextConfig(
            master_address="localhost:50051",
            connection_timeout=10,
            retry_attempts=2
        )
        
        # 使用工厂直接创建（强制本地类型）
        factory = ActorContextFactory()
        context = factory.create_context(
            config=config,
            force_type="local"  # 强制使用本地上下文
        )
        
        print(f"上下文类型: {context.context_type}")
        print(f"连接状态: {context.is_connected}")
        
        return context
        
    except Exception as e:
        logger.error(f"手动创建上下文失败: {e}")
        return None


def demo_context_operations(context: IActorContext):
    """演示上下文操作"""
    print("=== 上下文操作演示 ===")
    
    if not context:
        print("没有可用的上下文")
        return
    
    try:
        # 测试获取结果（应该返回 None，因为没有实际执行）
        result = context.get_result("test-key")
        print(f"获取测试结果: {result}")
        
        # 如果是本地上下文，显示调试信息
        if hasattr(context, 'get_message_count'):
            print(f"消息队列长度: {context.get_message_count()}")
            print(f"结果数量: {context.get_result_count()}")
        
        # 如果是远程上下文，显示连接信息
        if hasattr(context, 'get_connection_info'):
            info = context.get_connection_info()
            print(f"连接信息: {info}")
            
    except Exception as e:
        logger.error(f"上下文操作失败: {e}")


def demo_environment_variables():
    """演示环境变量配置"""
    print("=== 环境变量配置演示 ===")
    
    # 显示相关环境变量
    env_vars = [
        "ACTOR_MASTER_ADDRESS",
        "IARNET_MASTER_ADDRESS", 
        "ACTORC_REMOTE_ADDRESS",
        "IARNET_SERVICE_NAME",
        "IARNET_NAMESPACE",
        "IARNET_SERVICE_PORT",
        "IARNET_HOST",
        "IARNET_PORT",
        "DOCKER_CONTAINER"
    ]
    
    print("当前环境变量:")
    for var in env_vars:
        value = os.getenv(var)
        if value:
            print(f"  {var} = {value}")
        else:
            print(f"  {var} = (未设置)")
    print()


def main():
    """主函数"""
    print("ActorContext 接口演示")
    print("=" * 50)
    
    # 演示环境变量
    demo_environment_variables()
    
    # 演示环境检测
    demo_environment_detection()
    
    # 演示自动上下文创建
    auto_context = demo_auto_context_creation()
    if auto_context:
        demo_context_operations(auto_context)
        print()
    
    # 演示手动上下文创建
    manual_context = demo_manual_context_creation()
    if manual_context:
        demo_context_operations(manual_context)
        print()
    
    # 清理资源
    print("=== 清理资源 ===")
    try:
        ActorContext.closeContext()
        print("上下文已关闭")
    except Exception as e:
        logger.error(f"关闭上下文失败: {e}")


if __name__ == "__main__":
    main()