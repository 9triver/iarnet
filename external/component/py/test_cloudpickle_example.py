#!/usr/bin/env python3
"""
CloudPickle Function Test Example
演示如何使用cloudpickle序列化函数并通过我们的系统执行
"""

import os
import sys
import json
import base64
import cloudpickle
from pathlib import Path

# 添加当前目录到Python路径
sys.path.insert(0, str(Path(__file__).parent))

from cloudpickle_handler import CloudPickleFunctionHandler


def create_test_functions():
    """创建一些测试函数"""
    
    # 简单的数学函数
    def add_numbers(a, b):
        """简单的加法函数"""
        return a + b
    
    # 带有闭包的函数
    def create_multiplier(factor):
        """创建一个乘法器函数"""
        def multiply(x):
            return x * factor
        return multiply
    
    # 使用外部库的函数
    def process_data(data):
        """处理数据的函数"""
        import json
        import math
        
        if isinstance(data, str):
            data = json.loads(data)
        
        if isinstance(data, list):
            return {
                "sum": sum(data),
                "mean": sum(data) / len(data) if data else 0,
                "sqrt_sum": math.sqrt(sum(x**2 for x in data))
            }
        else:
            return {"error": "Invalid data type"}
    
    # Lambda函数
    square = lambda x: x ** 2
    
    return {
        "add_numbers": add_numbers,
        "multiplier_by_3": create_multiplier(3),
        "process_data": process_data,
        "square": square
    }


def test_cloudpickle_workflow():
    """测试完整的cloudpickle工作流程"""
    
    print("=== CloudPickle Function Test ===\n")
    
    # 1. 创建测试函数
    print("1. 创建测试函数...")
    test_functions = create_test_functions()
    
    # 2. 初始化处理器
    print("2. 初始化CloudPickle处理器...")
    handler = CloudPickleFunctionHandler("/tmp/test_cloudpickle_functions")
    
    # 3. 序列化并保存函数
    print("3. 序列化并保存函数...")
    saved_functions = {}
    
    for func_name, func in test_functions.items():
        try:
            # 使用cloudpickle序列化函数
            func_data = cloudpickle.dumps(func)
            print(f"   - 序列化函数 {func_name}: {len(func_data)} bytes")
            
            # 保存函数
            python_file, func_id = handler.save_function(func_data, func_name)
            saved_functions[func_name] = {
                "func_id": func_id,
                "python_file": python_file
            }
            print(f"   - 保存成功: {func_id}")
            
        except Exception as e:
            print(f"   - 保存失败 {func_name}: {e}")
    
    # 4. 列出已保存的函数
    print("\n4. 列出已保存的函数...")
    functions_list = handler.list_functions()
    for func_id, metadata in functions_list.items():
        print(f"   - {func_id}: {metadata['func_name']}")
    
    # 5. 测试函数执行
    print("\n5. 测试函数执行...")
    
    # 测试add_numbers
    if "add_numbers" in saved_functions:
        try:
            func_info = saved_functions["add_numbers"]
            python_file = func_info["python_file"]
            
            # 通过命令行执行
            import subprocess
            result = subprocess.run([
                "python3", python_file, 
                "--params", json.dumps({"a": 10, "b": 20})
            ], capture_output=True, text=True)
            
            if result.returncode == 0:
                output = json.loads(result.stdout)
                print(f"   - add_numbers(10, 20) = {output['result']}")
            else:
                print(f"   - add_numbers 执行失败: {result.stderr}")
                
        except Exception as e:
            print(f"   - add_numbers 测试失败: {e}")
    
    # 测试multiplier_by_3
    if "multiplier_by_3" in saved_functions:
        try:
            func_info = saved_functions["multiplier_by_3"]
            python_file = func_info["python_file"]
            
            # 通过命令行执行
            result = subprocess.run([
                "python3", python_file, 
                "--params", json.dumps([7])  # 位置参数
            ], capture_output=True, text=True)
            
            if result.returncode == 0:
                output = json.loads(result.stdout)
                print(f"   - multiplier_by_3(7) = {output['result']}")
            else:
                print(f"   - multiplier_by_3 执行失败: {result.stderr}")
                
        except Exception as e:
            print(f"   - multiplier_by_3 测试失败: {e}")
    
    # 测试process_data
    if "process_data" in saved_functions:
        try:
            func_info = saved_functions["process_data"]
            python_file = func_info["python_file"]
            
            test_data = [1, 2, 3, 4, 5]
            result = subprocess.run([
                "python3", python_file, 
                "--params", json.dumps({"data": test_data})
            ], capture_output=True, text=True)
            
            if result.returncode == 0:
                output = json.loads(result.stdout)
                print(f"   - process_data({test_data}) = {output['result']}")
            else:
                print(f"   - process_data 执行失败: {result.stderr}")
                
        except Exception as e:
            print(f"   - process_data 测试失败: {e}")
    
    # 测试square
    if "square" in saved_functions:
        try:
            func_info = saved_functions["square"]
            python_file = func_info["python_file"]
            
            result = subprocess.run([
                "python3", python_file, 
                "--params", json.dumps([8])  # lambda函数的参数
            ], capture_output=True, text=True)
            
            if result.returncode == 0:
                output = json.loads(result.stdout)
                print(f"   - square(8) = {output['result']}")
            else:
                print(f"   - square 执行失败: {result.stderr}")
                
        except Exception as e:
            print(f"   - square 测试失败: {e}")
    
    # 6. 测试通过处理器直接加载和执行
    print("\n6. 测试通过处理器直接加载...")
    for func_name, func_info in saved_functions.items():
        try:
            func_id = func_info["func_id"]
            loaded_func, python_file = handler.load_function(func_id)
            print(f"   - 成功加载 {func_name}: {loaded_func.__name__}")
        except Exception as e:
            print(f"   - 加载失败 {func_name}: {e}")
    
    print("\n=== 测试完成 ===")
    
    return saved_functions


def demonstrate_go_integration():
    """演示如何与Go代码集成"""
    
    print("\n=== Go集成示例 ===\n")
    
    # 创建一个简单的函数
    def fibonacci(n):
        """计算斐波那契数列"""
        if n <= 1:
            return n
        return fibonacci(n-1) + fibonacci(n-2)
    
    # 序列化函数
    func_data = cloudpickle.dumps(fibonacci)
    func_data_b64 = base64.b64encode(func_data).decode('utf-8')
    
    print("1. 函数序列化完成")
    print(f"   - 原始大小: {len(func_data)} bytes")
    print(f"   - Base64编码大小: {len(func_data_b64)} bytes")
    
    # 模拟Go代码调用
    print("\n2. 模拟Go代码调用流程:")
    print("   Go代码可以这样调用:")
    print(f"""
   // 在Go中保存函数
   funcData := "{func_data_b64}"
   funcInfo, err := runtimeManager.SaveCloudPickleFunction(
       base64.StdEncoding.DecodeString(funcData),
       "fibonacci",
       "",
   )
   
   // 执行函数
   params := map[string]interface{{"n": 10}}
   result, err := runtimeManager.ExecuteCloudPickleFunction(funcInfo, params)
   """)
    
    # 实际测试保存和执行
    print("\n3. 实际测试:")
    handler = CloudPickleFunctionHandler()
    
    try:
        python_file, func_id = handler.save_function(func_data, "fibonacci")
        print(f"   - 函数保存成功: {func_id}")
        print(f"   - Python文件: {python_file}")
        
        # 测试执行
        import subprocess
        result = subprocess.run([
            "python3", python_file, 
            "--params", json.dumps({"n": 10})
        ], capture_output=True, text=True)
        
        if result.returncode == 0:
            output = json.loads(result.stdout)
            print(f"   - fibonacci(10) = {output['result']}")
        else:
            print(f"   - 执行失败: {result.stderr}")
            
    except Exception as e:
        print(f"   - 测试失败: {e}")


def main():
    """主函数"""
    
    # 检查cloudpickle是否可用
    try:
        import cloudpickle
        print(f"CloudPickle版本: {cloudpickle.__version__}")
    except ImportError:
        print("错误: 需要安装cloudpickle库")
        print("请运行: pip install cloudpickle")
        return
    
    # 运行测试
    try:
        saved_functions = test_cloudpickle_workflow()
        demonstrate_go_integration()
        
        print(f"\n所有测试完成！共保存了 {len(saved_functions)} 个函数。")
        
    except Exception as e:
        print(f"测试过程中出现错误: {e}")
        import traceback
        traceback.print_exc()


if __name__ == "__main__":
    main()