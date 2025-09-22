# Lucas + ActorC 应用部署示例

这个目录包含了使用 Lucas 和 ActorC 进行分布式应用部署的基础示例。

## 文件说明

- `simple_workflow.py` - 主要示例文件，展示了一个简单的数据处理工作流
- `requirements.txt` - 项目依赖文件
- `README.md` - 本说明文档

## 示例概述

`simple_workflow.py` 展示了一个包含三个步骤的简单数据处理工作流：

1. **数据生成** (`generate_data`) - 生成指定数量的随机数据
2. **数据处理** (`process_data`) - 计算数据的基本统计信息（平均值、最小值、最大值等）
3. **结果汇总** (`summarize_results`) - 生成格式化的处理报告

## 前置条件

在运行示例之前，请确保：

1. **ActorC 服务运行中**
   ```bash
   # 确保 ActorC 服务在 localhost:8082 上运行
   # 具体启动方法请参考 ActorC 文档
   ```

2. **安装必要依赖**
   ```bash
   # 安装 Lucas 框架（从项目根目录）
   cd /home/zyx/cfn-proj/Lucas
   pip install -e .
   
   # 安装 ActorC 客户端（从项目根目录）
   cd /home/zyx/cfn-proj/iarnet/client/py
   pip install -e .
   
   # 安装其他依赖
   cd /home/zyx/cfn-proj/iarnet/client/demo
   pip install -r requirements.txt
   ```

## 运行示例

```bash
cd /home/zyx/cfn-proj/iarnet/client/demo
python simple_workflow.py
```

## 示例输出

运行成功后，你将看到类似以下的输出：

```
=== Lucas + ActorC 简单工作流示例 ===

工作流验证成功!

工作流元数据:
{
  "functions": [...],
  "dependencies": [...]
}

示例1: 处理10个随机数
生成 10 个随机数...
生成完成: [42, 17, 89, 23, 56]...
处理数据，共 10 个元素...
处理完成: 平均值=45.30, 最小值=17, 最大值=89
生成汇总报告完成
结果1:

数据处理报告
============
数据量: 10 个
总和: 453
平均值: 45.30
最小值: 17
最大值: 89
处理时间: 2024-01-20 15:30:45
```

## 代码结构说明

### 1. 函数定义

每个处理步骤都使用 `@function` 装饰器定义：

```python
@function(
    wrapper=ActorFunction,      # 使用 ActorC 包装器
    dependency=[],              # 依赖列表
    provider="actor",           # 使用 actor 提供者
    name="function_name",       # 函数名称
    venv="demo_env",           # 虚拟环境名称
    replicas=2,                # 副本数量（可选）
)
def your_function(input_data):
    # 函数实现
    return result
```

### 2. 工作流定义

使用 `@workflow` 装饰器定义工作流：

```python
@workflow(executor=ActorExecutor)
def workflow_func(wf: Workflow):
    # 定义工作流步骤
    step1 = wf.submit(function1, wf.get_input())
    step2 = wf.submit(function2, step1)
    wf.set_output(step2)
```

### 3. 工作流执行

```python
# 生成工作流实例
workflow_instance = workflow_func.generate()

# 验证工作流
dag = workflow_instance.valicate()

# 导出可执行函数
executable_workflow = workflow_func.export(export_function)

# 执行工作流
result = executable_workflow({"input": "data"})
```

## 自定义示例

你可以基于这个示例创建自己的工作流：

1. **修改函数逻辑** - 在现有函数中实现你的业务逻辑
2. **添加新函数** - 使用相同的装饰器模式添加新的处理步骤
3. **调整工作流** - 在 `workflow_func` 中重新组织步骤顺序
4. **配置参数** - 调整副本数量、虚拟环境等配置

## 故障排除

### 常见问题

1. **连接错误**
   ```
   错误: 无法连接到 ActorC 服务
   解决: 确保 ActorC 服务在 localhost:8082 上运行
   ```

2. **导入错误**
   ```
   错误: ModuleNotFoundError: No module named 'lucas'
   解决: 确保已正确安装 Lucas 和 ActorC 客户端
   ```

3. **虚拟环境错误**
   ```
   错误: 虚拟环境 'demo_env' 不存在
   解决: 在 ActorC 中创建相应的虚拟环境
   ```

### 调试技巧

1. **启用详细日志** - 在代码中添加更多 print 语句
2. **单步测试** - 单独测试每个函数
3. **检查服务状态** - 确认 ActorC 服务正常运行

## 进阶用法

- 查看 `ignis/clients/demo/` 目录下的更复杂示例
- 参考 Lucas 和 ActorC 的官方文档
- 尝试添加更多的并行处理步骤
- 实验不同的数据类型和处理逻辑

## 相关资源

- [Lucas 框架文档](../../../Lucas/README.md)
- [ActorC 文档](../py/README.md)
- [IARNet 架构设计](../../doc/IARNet混合交互模式架构设计.md)