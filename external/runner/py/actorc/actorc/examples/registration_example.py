#!/usr/bin/env python3
"""
ApplicationRegistrationService 调用示例
展示如何在 actorc 中使用应用注册服务
"""

import sys
import os

# 添加 actorc 到 Python 路径
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from controller.context import ActorContext
from client.registration_client import ApplicationRegistrationClient, register_application_simple


def example_using_actor_context():
    """使用 ActorContext 进行应用注册的示例"""
    print("=== 使用 ActorContext 进行应用注册 ===")
    
    # 创建 ActorContext
    context = ActorContext.createContext()
    
    try:
        # 注册应用
        app_id = "actorc-example-app-001"
        response = context.register_application(app_id)
        
        print(f"应用 {app_id} 注册结果:")
        print(f"  成功: {response.success}")
        if response.error:
            print(f"  错误: {response.error}")
        
        return response.success
        
    except Exception as e:
        print(f"注册失败: {e}")
        return False


def example_using_dedicated_client():
    """使用专用客户端进行应用注册的示例"""
    print("\n=== 使用专用客户端进行应用注册 ===")
    
    try:
        # 方式1: 使用上下文管理器
        with ApplicationRegistrationClient() as client:
            app_id = "actorc-example-app-002"
            response = client.register_application(app_id)
            print(f"应用 {app_id} 注册结果:")
            print(f"  成功: {response.success}")
            if response.error:
                print(f"  错误: {response.error}")
        
        # 方式2: 使用简单函数
        app_id = "actorc-example-app-003"
        success = register_application_simple(app_id)
        print(f"应用 {app_id} 简单注册结果: {success}")
        
        return True
        
    except Exception as e:
        print(f"注册失败: {e}")
        return False


def example_batch_registration():
    """批量应用注册示例"""
    print("\n=== 批量应用注册 ===")
    
    app_ids = [
        "actorc-batch-app-001",
        "actorc-batch-app-002", 
        "actorc-batch-app-003"
    ]
    
    success_count = 0
    
    try:
        with ApplicationRegistrationClient() as client:
            for app_id in app_ids:
                try:
                    response = client.register_application(app_id)
                    if response.success:
                        success_count += 1
                        print(f"✓ {app_id} 注册成功")
                    else:
                        print(f"✗ {app_id} 注册失败: {response.error}")
                except Exception as e:
                    print(f"✗ {app_id} 注册异常: {e}")
        
        print(f"\n批量注册完成: {success_count}/{len(app_ids)} 成功")
        return success_count == len(app_ids)
        
    except Exception as e:
        print(f"批量注册失败: {e}")
        return False


def main():
    """主函数"""
    print("ApplicationRegistrationService 调用示例")
    print("=" * 50)
    
    # 检查环境变量
    ignis_addr = os.getenv("IGNIS_ADDR", "localhost:50051")
    print(f"Ignis 服务地址: {ignis_addr}")
    print()
    
    # 运行示例
    examples = [
        ("ActorContext 示例", example_using_actor_context),
        ("专用客户端示例", example_using_dedicated_client),
        ("批量注册示例", example_batch_registration)
    ]
    
    results = []
    for name, func in examples:
        try:
            result = func()
            results.append((name, result))
        except Exception as e:
            print(f"{name} 执行失败: {e}")
            results.append((name, False))
    
    # 总结
    print("\n" + "=" * 50)
    print("执行总结:")
    for name, success in results:
        status = "✓ 成功" if success else "✗ 失败"
        print(f"  {name}: {status}")


if __name__ == "__main__":
    main()