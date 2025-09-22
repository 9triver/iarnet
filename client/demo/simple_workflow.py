#!/usr/bin/env python3
"""
简单的 Lucas + ActorC 应用部署示例

这个示例展示了如何使用 Lucas 和 ActorC 创建一个基础的分布式计算工作流。
工作流包含三个步骤：
1. 生成数据
2. 处理数据
3. 汇总结果
"""

import sys
import time
from typing import List, Dict, Any
from lucas import workflow, function, Workflow
from lucas.serverless_function import Metadata
from actorc.controller.context import (
    ActorContext,
    ActorFunction,
    ActorExecutor,
    ActorRuntime,
)

# 创建 ActorC 上下文连接
# 注意：确保 ActorC 服务运行在 localhost:8082
context = ActorContext.createContext("localhost:8082")


@function(
    wrapper=ActorFunction,
    dependency=[],
    provider="actor",
    name="generate_data",
    venv="demo_env",
)
def generate_data(count: int) -> List[int]:
    """生成指定数量的随机数据"""
    import random

    print(f"生成 {count} 个随机数...")
    data = [random.randint(1, 100) for _ in range(count)]
    print(f"生成完成: {data[:5]}..." if len(data) > 5 else f"生成完成: {data}")
    return data


@function(
    wrapper=ActorFunction,
    dependency=[],
    provider="actor",
    name="process_data",
    venv="demo_env",
    replicas=2,  # 使用2个副本并行处理
)
def process_data(data: List[int]) -> Dict[str, Any]:
    """处理数据：计算基本统计信息"""
    print(f"处理数据，共 {len(data)} 个元素...")

    if not data:
        return {"error": "空数据"}

    result = {
        "count": len(data),
        "sum": sum(data),
        "avg": sum(data) / len(data),
        "min": min(data),
        "max": max(data),
        "processed_at": time.time(),
    }

    print(
        f"处理完成: 平均值={result['avg']:.2f}, 最小值={result['min']}, 最大值={result['max']}"
    )
    return result


@function(
    wrapper=ActorFunction,
    dependency=[],
    provider="actor",
    name="summarize_results",
    venv="demo_env",
)
def summarize_results(stats: Dict[str, Any]) -> str:
    """汇总处理结果"""
    if "error" in stats:
        return f"处理失败: {stats['error']}"

    summary = f"""
        数据处理报告
        ============
        数据量: {stats['count']} 个
        总和: {stats['sum']}
        平均值: {stats['avg']:.2f}
        最小值: {stats['min']}
        最大值: {stats['max']}
        处理时间: {time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(stats['processed_at']))}
    """

    print("生成汇总报告完成")
    return summary


@workflow(executor=ActorExecutor)
def simple_workflow_func(wf: Workflow):
    """定义简单的数据处理工作流"""
    # 步骤1: 生成数据
    data = wf.submit(generate_data, wf.get_input()["count"])

    # 步骤2: 处理数据
    stats = wf.submit(process_data, data)

    # 步骤3: 汇总结果
    summary = wf.submit(summarize_results, stats)

    # 返回最终结果
    wf.set_output(summary)


# 生成工作流实例
workflow_instance = simple_workflow_func.generate()

# 验证工作流
dag = workflow_instance.valicate()
print("工作流验证成功!")

# 打印工作流元数据（可选）
import json

print("\n工作流元数据:")
print(json.dumps(dag.metadata(fn_export=True), indent=2, ensure_ascii=False))


def workflow_export_func(input_dict: Dict[str, Any]) -> str:
    """工作流导出函数"""
    print(f"开始执行工作流，输入参数: {input_dict}")

    # 执行工作流
    result = workflow_instance.run(input_dict)

    print("工作流执行完成!")
    return result


# 导出可执行的工作流函数
simple_workflow = simple_workflow_func.export(workflow_export_func)


def main():
    """主函数：演示工作流的使用"""
    print("=== Lucas + ActorC 简单工作流示例 ===\n")

    # 示例1: 处理10个数据
    print("示例1: 处理10个随机数")
    result1 = simple_workflow({"count": 10})
    print("结果1:")
    print(result1)
    print("\n" + "=" * 50 + "\n")

    # 示例2: 处理更多数据
    print("示例2: 处理50个随机数")
    result2 = simple_workflow({"count": 50})
    print("结果2:")
    print(result2)
    print("\n" + "=" * 50 + "\n")

    # 示例3: 边界情况测试
    print("示例3: 处理0个数据（边界情况）")
    result3 = simple_workflow({"count": 0})
    print("结果3:")
    print(result3)


if __name__ == "__main__":
    main()
