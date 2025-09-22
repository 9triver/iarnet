#!/usr/bin/env python3
"""
计算模式使用示例

展示如何使用Ray式交互模式和Flink式批处理模式
"""

import os
import time
import asyncio
from actorc.controller import (
    context_factory, 
    ComputeMode, 
    ActorContextConfig
)


def example_ray_interactive():
    """Ray式交互模式示例"""
    print("=== Ray式交互模式示例 ===")
    
    # 设置环境变量强制使用Ray模式
    os.environ["IARNET_COMPUTE_MODE"] = "ray"
    
    # 创建上下文
    context = context_factory.create_context()
    print(f"创建的上下文类型: {context.context_type}")
    print(f"计算模式: {context._config.compute_mode.value}")
    
    try:
        # 实时交互示例
        print("\n发送实时消息...")
        response = context.send({
            "action": "process_image",
            "image_path": "/path/to/image.jpg",
            "model": "yolo"
        })
        print(f"实时响应: {response}")
        
        # 流式处理示例
        print("\n开始流式处理...")
        for i in range(3):
            result = context.send({
                "action": "stream_process",
                "data": f"chunk_{i}",
                "stream_id": "test_stream"
            })
            print(f"流式结果 {i}: {result}")
            time.sleep(0.5)
            
    except Exception as e:
        print(f"Ray模式执行错误: {e}")
    finally:
        context.close()


def example_flink_batch():
    """Flink式批处理模式示例"""
    print("\n=== Flink式批处理模式示例 ===")
    
    # 设置环境变量强制使用Flink模式
    os.environ["IARNET_COMPUTE_MODE"] = "flink"
    
    # 创建上下文
    context = context_factory.create_context()
    print(f"创建的上下文类型: {context.context_type}")
    print(f"计算模式: {context._config.compute_mode.value}")
    
    try:
        # 提交批处理任务
        print("\n提交批处理任务...")
        task_result = context.submit_task({
            "action": "batch_process",
            "input_data": [f"item_{i}" for i in range(100)],
            "model": "bert",
            "batch_size": 32
        })
        
        task_id = task_result.task_id
        print(f"任务已提交，ID: {task_id}")
        
        # 轮询任务状态
        print("\n监控任务状态...")
        while True:
            status = context.get_task_status(task_id)
            print(f"任务状态: {status.status.value}")
            
            if status.status.value in ['completed', 'failed']:
                break
                
            time.sleep(2)
        
        # 获取最终结果
        if status.status.value == 'completed':
            final_result = context.get_task_result(task_id)
            print(f"任务完成，结果: {final_result}")
        else:
            print(f"任务失败: {status.error}")
            
    except Exception as e:
        print(f"Flink模式执行错误: {e}")
    finally:
        context.close()


def example_auto_detection():
    """自动检测模式示例"""
    print("\n=== 自动检测模式示例 ===")
    
    # 清除环境变量，让系统自动检测
    os.environ.pop("IARNET_COMPUTE_MODE", None)
    
    # 模拟不同环境
    scenarios = [
        ("本地开发环境", {"IARNET_DEV_MODE": "true"}),
        ("生产环境", {"IARNET_PROD_MODE": "true"}),
        ("默认环境", {})
    ]
    
    for scenario_name, env_vars in scenarios:
        print(f"\n--- {scenario_name} ---")
        
        # 设置环境变量
        for key, value in env_vars.items():
            os.environ[key] = value
        
        # 清除缓存
        context_factory.clear_cache()
        
        # 检测计算模式
        detected_mode = context_factory.detect_compute_mode()
        print(f"检测到的计算模式: {detected_mode.value}")
        
        # 清理环境变量
        for key in env_vars:
            os.environ.pop(key, None)


def example_explicit_mode():
    """显式指定模式示例"""
    print("\n=== 显式指定模式示例 ===")
    
    # 显式创建Ray模式上下文
    ray_context = context_factory.create_context(
        compute_mode=ComputeMode.RAY_INTERACTIVE
    )
    print(f"Ray上下文: {ray_context.context_type}")
    ray_context.close()
    
    # 显式创建Flink模式上下文
    flink_context = context_factory.create_context(
        compute_mode=ComputeMode.FLINK_BATCH
    )
    print(f"Flink上下文: {flink_context.context_type}")
    flink_context.close()


if __name__ == "__main__":
    print("ActorContext 计算模式示例")
    print("=" * 50)
    
    try:
        # 运行各种示例
        example_auto_detection()
        example_explicit_mode()
        
        # 注意：以下示例需要实际的服务端支持
        # example_ray_interactive()
        # example_flink_batch()
        
        print("\n示例运行完成！")
        
    except Exception as e:
        print(f"示例运行出错: {e}")
        import traceback
        traceback.print_exc()