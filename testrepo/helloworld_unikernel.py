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
    name="hello_function_unikernel",
    backend="unikernel",
    cpu=1000,  # millicores
    memory="1024Mi",
    gpu=0,
    replicas=1,
)
def hello_function_unikernel(name: str):
    """open Yojson.Basic.Util

let hello_function value =
  try
    let name = value |> member "name" |> to_string in
    let message = Printf.sprintf "Hello, %s! This is from Unikernel function." name in
    Ok (`String message)
  with Type_error (msg, _) -> Error ("input type error: " ^ msg)

let handlers = [
  "hello_function_unikernel", hello_function;
]
"""
    pass  # OCaml代码在docstring中


@function(
    wrapper=ActorFunction,
    dependency=[],
    provider="actor",
    name="process_data_unikernel",
    backend="unikernel",
    cpu=1000,  # millicores
    memory="1024Mi",
    gpu=0,
    replicas=2,
)
def process_data_unikernel(data: str):
    """open Yojson.Basic.Util

let process_data value =
  try
    let data = value |> member "data" |> to_string in
    let processed = String.uppercase_ascii data ^ " - PROCESSED BY UNIKERNEL" in
    Ok (`String processed)
  with Type_error (msg, _) -> Error ("input type error: " ^ msg)

let handlers = [
  "process_data_unikernel", process_data;
]
"""
    pass  # OCaml代码在docstring中


@workflow(executor=ActorExecutor)
def simple_workflow_unikernel(wf: Workflow):
    """简单的工作流：调用两个Unikernel函数"""
    _in = wf.input()
    
    # 调用第一个Unikernel函数
    greeting = wf.call("hello_function_unikernel", {"name": _in["name"]})
    
    # 调用第二个Unikernel函数
    result = wf.call("process_data_unikernel", {"data": greeting})
    
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
    simple_workflow_unikernel.set_runtime(rt)
    workflow = simple_workflow_unikernel.generate()
    return workflow.execute()


# 导出工作流
demo_workflow_unikernel = simple_workflow_unikernel.export(actorWorkflowExportFunc)

# 执行示例
if __name__ == "__main__":
    log.info("执行Unikernel backend的workflow示例:")
    result = demo_workflow_unikernel({"name": "World"})
    log.info(f"工作流结果: {result}")

