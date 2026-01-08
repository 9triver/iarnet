import os
import uuid
from lucas import workflow, function, Workflow
from lucas.serverless_function import Metadata
from lucas.actorc.actor import (
    ActorContext,
    ActorFunction,
    ActorExecutor,
    ActorRuntime,
)
from lucas.utils.logging import log

# 创建Actor上下文
context = ActorContext.createContext()


@function(
    wrapper=ActorFunction,
    dependency=[],
    provider="actor",
    name="hello_function_go",
    backend="go",
    cpu=1000,  # millicores
    memory="1024Mi",
    gpu=0,
    replicas=1,
)
def hello_function_go(Name: str):
    """package main

import (
    "fmt"
)

type Input struct {
    Name string
}

type Output = string

func Impl(input *Input) (Output, error) {
    return fmt.Sprintf("Hello, %s! This is from Go function.", input.Name), nil
}
"""
    pass  # Go代码在docstring中


@function(
    wrapper=ActorFunction,
    dependency=[],
    provider="actor",
    name="process_data_go",
    backend="go",
    cpu=1000,  # millicores
    memory="1024Mi",
    gpu=0,
    replicas=2,
)
def process_data_go(Data: str):
    """package main

import (
    "strings"
)

type Input struct {
    Data string
}

type Output = string

func Impl(input *Input) (Output, error) {
    processed := strings.ToUpper(input.Data) + " - PROCESSED BY GO"
    return processed, nil
}
"""
    pass  # Go代码在docstring中


@workflow(executor=ActorExecutor)
def simple_workflow_go(wf: Workflow):
    """简单的工作流：调用两个Go函数"""
    _in = wf.input()
    
    # 调用第一个Go函数
    greeting = wf.call("hello_function_go", {"Name": _in["Name"]})
    
    # 调用第二个Go函数
    result = wf.call("process_data_go", {"Data": greeting})
    
    return result


def actorWorkflowExportFunc(dict: dict):
    """导出函数用于本地调用"""
    from lucas import routeBuilder

    route = routeBuilder.build()
    route_dict = {}
    for function in route.functions:
        route_dict[function.name] = function.handler
    for workflow in route.workflows:
        route_dict[workflow.name] = workflow._generate_workflow
        
    metadata = Metadata(
        id=str(uuid.uuid4()),
        params=dict,
        namespace=None,
        router=route_dict,
        request_type="invoke",
        redis_db=None,
        producer=None,
    )
    rt = ActorRuntime(metadata)
    simple_workflow_go.set_runtime(rt)
    workflow = simple_workflow_go.generate()
    return workflow.execute()


# 导出工作流
demo_workflow_go = simple_workflow_go.export(actorWorkflowExportFunc)

# 执行示例
if __name__ == "__main__":
    log.info("执行Go backend的workflow示例:")
    result = demo_workflow_go({"Name": "World"})
    log.info(f"工作流结果: {result}")

