"""
Actor 实现
每个组件中运行着一个 Actor，负责：
1. 接收消息（通过 ZMQ）
2. 执行函数
3. 返回响应

Actor 是组件中的核心执行单元，采用消息驱动的异步处理模式。
"""

import inspect
import logging
import queue
import subprocess
import sys
import threading
import time
from typing import Any, Optional

import cloudpickle
import zmq

# 确保 proto 模块被导入，以便 __init__.py 中的路径设置生效
import proto  # noqa: F401

from proto.ignis.actor import actor_pb2 as actor
from proto.common import types_pb2 as common
from proto.common import messages_pb2 as common_messages
from proto.resource.component import component_pb2 as component

from encdec import EncDec
from models import RemoteFunction
from store_client import StoreClient

logger = logging.getLogger(__name__)


class Actor:
    """
    Actor 实现类
    
    Actor 是组件中的核心执行单元，采用消息驱动模式：
    - 接收消息：通过 ZMQ 接收来自控制器的消息（Function、InvokeRequest 等）
    - 执行函数：根据接收到的函数定义和参数执行计算任务
    - 返回响应：将执行结果通过 ZMQ 发送回控制器
    
    Actor 生命周期：
    1. 启动时发送 READY 消息，标识自身可用
    2. 接收并注册 Function 消息，准备执行函数
    3. 接收 InvokeRequest 消息，执行函数并返回结果
    """
    
    def __init__(self, store_client: StoreClient):
        """
        初始化 Actor
        
        Args:
            store_client: Store 服务客户端，用于获取参数和保存结果
        """
        self.store_client = store_client
        
        # Actor 状态
        self.function: Optional[RemoteFunction] = None      # 当前注册的函数
        self.function_name: Optional[str] = None            # 函数名称
        self.expected_params: set[str] = set()         # 函数期望的参数名集合
        
        # 消息通信
        self.send_queue = queue.Queue[actor.Message | component.Message | None]()  # 发送消息队列（支持 actor.Message 和 component.Message）

    # ========================================================================
    # 函数注册相关方法
    # ========================================================================
    
    def _install_requirements(self, requirements: list[str]) -> bool:
        """
        安装 Python 依赖包（在虚拟环境中）
        
        Args:
            requirements: 依赖包列表
            
        Returns:
            安装是否成功
        """
        if not requirements:
            return True
        
        logger.info(f"Installing {len(requirements)} dependencies: {requirements}")
        try:
            # 使用虚拟环境中的 Python 和 pip
            cmd = [
                sys.executable, "-m", "pip", "install",
                "--quiet",                    # 减少输出
                "--no-cache-dir",             # 不缓存包
                "--no-warn-script-location"   # 抑制警告
            ] + requirements
            
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=300  # 5 分钟超时
            )
            
            if result.returncode == 0:
                logger.info(f"Successfully installed dependencies: {requirements}")
                if result.stdout:
                    logger.debug(f"pip output: {result.stdout}")
                return True
            else:
                logger.error(f"Failed to install dependencies. stderr: {result.stderr}, stdout: {result.stdout}")
                return False
        except subprocess.TimeoutExpired:
            logger.error("Dependency installation timed out after 5 minutes")
            return False
        except Exception as e:
            logger.error(f"Error installing dependencies: {e}", exc_info=True)
            return False

    def handle_function(self, msg: actor.Function) -> bool:
        """
        处理 Function 消息，注册函数
        
        Actor 接收到 Function 消息后，会：
        1. 安装依赖（如果有）
        2. 反序列化函数对象
        3. 注册函数，准备执行
        
        Args:
            msg: Function 消息
            
        Returns:
            注册是否成功
        """
        # 安装依赖（如果有）
        if msg.Requirements:
            if not self._install_requirements(list(msg.Requirements)):
                logger.warning(f"Failed to install some dependencies for {msg.Name}, continuing anyway")
        
        try:
            # 反序列化函数对象
            fn = cloudpickle.loads(msg.PickledObject)
            if not callable(fn):
                logger.error(f"Function {msg.Name} is not callable")
                return False

            # 注册函数
            self.function = RemoteFunction(msg.Language, fn)
            self.function_name = msg.Name
            
            # 获取函数签名，提取参数名
            try:
                sig = inspect.signature(fn)
                self.expected_params = set(sig.parameters.keys())
                logger.info(f"Registered function: {msg.Name}{sig}, expected params: {self.expected_params}")
                return True
            except Exception as e:
                logger.error(f"Cannot get signature for {msg.Name}: {e}")
                return False
        except Exception as e:
            logger.error(f"Failed to load function {msg.Name}: {e}")
            return False

    # ========================================================================
    # 函数调用相关方法
    # ========================================================================
    
    def handle_invoke_request(self, msg: actor.InvokeRequest):
        """
        处理 InvokeRequest 消息
        
        Actor 接收到批量调用请求后，会在新线程中执行，避免阻塞主消息循环。
        
        Args:
            msg: InvokeRequest 消息，包含会话 ID 和参数列表
        """
        logger.info(
            "InvokeRequest received",
            extra={
                "runtime_id": msg.RuntimeID,
                "arg_count": len(msg.Args),
            }
        )

        # 在新线程中执行，避免阻塞主消息循环
        thread = threading.Thread(
            target=self._process_invoke_request,
            args=(msg,),
            daemon=True
        )
        thread.start()

    def _process_invoke_request(self, msg: actor.InvokeRequest):
        """
        处理批量调用请求（在新线程中执行）
        
        流程：
        1. 从 Store 获取所有参数值
        2. 解码参数
        3. 执行函数
        4. 发送响应
        
        Args:
            msg: InvokeRequest 消息，包含会话 ID 和参数列表
        """
        runtime_id = msg.RuntimeID
        
        # 收集所有参数到局部字典（不使用实例变量，避免状态污染）
        invoke_params = {}
        
        for arg in msg.Args:
            # 验证 ObjectRef 引用
            if not arg.Value or not arg.Value.ID:
                logger.error(f"Invalid ObjectRef for param {arg.Param}")
                continue
                
            # 从 Store 获取对象
            store_obj = self.store_client.get_object(
                arg.Value.ID, 
                arg.Value.Source if arg.Value.Source else ""
            )
            if store_obj is None:
                logger.error(f"Failed to get object {arg.Value.ID} from store")
                continue
                
            # 解码对象并添加到参数字典
            try:
                value = self._decode_store_object(store_obj)
                invoke_params[arg.Param] = value
                logger.debug(
                    f"Collected arg: param={arg.Param}, value_type={type(value).__name__}, "
                    f"collected={len(invoke_params)}/{len(msg.Args)}"
                )
            except Exception as e:
                logger.error(f"Failed to decode object {arg.Value.ID}: {e}")
                continue
        
        # 如果所有参数都收集成功，执行函数
        if len(invoke_params) == len(msg.Args):
            logger.info(f"All parameters collected, executing function {self.function_name}")
            self._execute_and_respond(invoke_params, runtime_id)
        else:
            logger.error(f"Failed to collect all parameters: got {len(invoke_params)}/{len(msg.Args)}")

    def _execute_and_respond(self, invoke_params: dict[str, Any], runtime_id: str):
        """
        执行函数并发送响应
        
        Actor 执行函数的核心逻辑：
        1. 执行函数
        2. 编码结果
        3. 保存结果到 Store
        4. 发送 InvokeResponse 消息
        
        Args:
            invoke_params: 函数参数字典
            runtime_id: Runtime ID
        """
        if not self.function:
            logger.error("Function not registered")
            return

        func = self.function
        error_msg = None
        result_ref = None
        calc_latency_ms = 0  # 计算延迟（毫秒）
        
        try:
            # 记录函数执行开始时间
            start_time = time.time()
            
            # 执行函数
            logger.info(f"Executing function {self.function_name} with params: {list(invoke_params.keys())}")
            value = func.call(**invoke_params)
            
            # 计算函数执行时间（毫秒）
            calc_latency_ms = int((time.time() - start_time) * 1000)
            
            # 编码结果
            store_lang = EncDec.platform_to_store_language(func.language)
            data, is_stream = EncDec.encode(value, store_lang)
            
            # 保存结果到 Store
            object_ref = self.store_client.save_object(data, store_lang, is_stream=is_stream)
            if object_ref is None:
                error_msg = "Failed to save result to store"
                logger.error(error_msg)
            else:
                if is_stream:
                    self._stream_result_chunks(object_ref.ID, value, store_lang)
                # 创建 ObjectRef 引用（不再需要 Flow 和 StoreRef）
                result_ref = common.ObjectRef(
                    ID=object_ref.ID,
                    Source=object_ref.Source if object_ref.Source else ""
                )
                logger.info(
                    f"Function {self.function_name} completed, result saved as {object_ref.ID}, "
                    f"calc_latency={calc_latency_ms}ms"
                )
        except Exception as e:
            error_msg = f"{e.__class__.__name__}: {e}"
            logger.error(f"Function {self.function_name} execution failed: {error_msg}", exc_info=True)
        
        # 创建 ActorInfo，包含计算延迟
        # 注意：这里不设置 ActorRef，因为 Python 端不知道 Actor 的引用信息
        # Go 端会在 Complete 方法中更新 Actor 的延迟信息
        actor_info = actor.ActorInfo(
            CalcLatency=calc_latency_ms,
            LinkLatency=0,  # 链路延迟由 Go 端计算
        )
        
        # 发送 InvokeResponse 消息
        invoke_response = actor.InvokeResponse(
            RuntimeID=runtime_id,
            Result=result_ref if result_ref else None,
            Error=error_msg if error_msg else "",
            Info=actor_info,
        )
        
        # 创建 actor.Message 响应，_send_loop 会自动包装为 component.Message
        response_msg = actor.Message(
            Type=actor.MessageType.INVOKE_RESPONSE,
            InvokeResponse=invoke_response
        )
        self.send_queue.put(response_msg)

    def _decode_store_object(self, store_obj: common.EncodedObject) -> Any:
        """
        解码 Store 中的对象，支持流式对象。

        Args:
            store_obj: Store 返回的对象

        Returns:
            Python 对象或生成器
        """
        if store_obj.IsStream:
            return self._create_stream_reader(store_obj)
        return EncDec.decode(store_obj)

    def _create_stream_reader(self, store_obj: common.EncodedObject):
        """
        将流式对象转换为生成器，按需从 Store 拉取数据。
        """
        object_id = store_obj.ID
        logger.info(f"Creating stream reader for object {object_id}")

        def generator():
            try:
                for chunk in self.store_client.iter_stream_chunks(object_id):
                    if chunk.Error:
                        raise RuntimeError(
                            f"Stream {object_id} chunk error: {chunk.Error}"
                        )
                    if chunk.Value is None:
                        continue
                    yield EncDec.decode(chunk.Value)
                    if chunk.EoS:
                        return
            except Exception as exc:
                logger.error(f"Stream {object_id} iteration failed: {exc}")
                raise

        return generator()

    def _stream_result_chunks(self, object_id: str, generator: Any, language: common.Language):
        """
        将函数返回的生成器拆分为多个 chunk，写入 Store。
        """
        offset = 0
        try:
            for item in generator:
                data, nested_stream = EncDec.encode(item, language)
                if nested_stream:
                    raise ValueError("Nested stream results are not supported")
                encoded = common.EncodedObject(
                    ID=EncDec.next_id(),
                    Data=data,
                    Language=language,
                )
                chunk = common.StreamChunk(
                    ObjectID=object_id,
                    Offset=offset,
                    Value=encoded,
                )
                if not self.store_client.save_stream_chunk(chunk):
                    raise RuntimeError(
                        f"Failed to save stream chunk: object={object_id}, offset={offset}"
                    )
                offset += 1

            eos_chunk = common.StreamChunk(
                ObjectID=object_id,
                Offset=offset,
                EoS=True,
            )
            if not self.store_client.save_stream_chunk(eos_chunk):
                raise RuntimeError(f"Failed to save EOS chunk for stream {object_id}")
        except Exception as exc:
            error_chunk = common.StreamChunk(
                ObjectID=object_id,
                Offset=offset,
                Error=str(exc),
            )
            self.store_client.save_stream_chunk(error_chunk)
            raise

    # ========================================================================
    # 消息接收和发送相关方法
    # ========================================================================
    
    def _unwrap_component_message(self, component_msg: component.Message) -> Optional[actor.Message]:
        """
        从 component.Message 中提取 actor.Message
        
        Args:
            component_msg: component 层的包装消息
            
        Returns:
            提取的 actor.Message，如果消息类型不是 PAYLOAD 则返回 None
        """
        if component_msg.Type != component.MessageType.PAYLOAD:
            return None
        
        if not component_msg.HasField("Payload"):
            logger.warning("Component message has PAYLOAD type but Payload field is not set")
            return None
        
        # 从 google.protobuf.Any 中反序列化 actor.Message
        try:
            actor_msg = actor.Message()
            component_msg.Payload.Unpack(actor_msg)
            return actor_msg
        except Exception as e:
            logger.error(f"Failed to unpack Payload: {e}", exc_info=True)
            return None

    def _wrap_actor_message(self, actor_msg: actor.Message) -> component.Message:
        """
        将 actor.Message 包装为 component.Message
        
        Args:
            actor_msg: actor 层的消息
            
        Returns:
            包装后的 component.Message
        """
        from google.protobuf import any_pb2
        any_payload = any_pb2.Any()
        any_payload.Pack(actor_msg)
        return component.Message(
            Type=component.MessageType.PAYLOAD,
            Payload=any_payload
        )

    def wait_for_function(self, socket: zmq.Socket) -> bool:
        """
        等待并注册 Function 消息（启动时调用）
        
        Actor 启动后，首先需要接收 Function 消息来注册要执行的函数。
        从 ZMQ 读取的是 component.Message，需要先提取 Payload 中的 actor.Message。
        
        Args:
            socket: ZMQ socket
            
        Returns:
            注册是否成功
        """
        logger.info("Waiting for Function message...")
        while True:
            try:
                msg_bytes = socket.recv()
                # 首先解析为 component.Message
                component_msg = component.Message.FromString(msg_bytes)
                
                # 提取 Payload 中的 actor.Message
                msg = self._unwrap_component_message(component_msg)
                if msg is None:
                    logger.warning(f"Received non-PAYLOAD component message: {component_msg.Type}")
                    continue
                
                if msg.Type == actor.MessageType.FUNCTION:
                    logger.info(f"Received FUNCTION: {msg.Function.Name}")
                    if self.handle_function(msg.Function):
                        # 发送 ACK 确认（包装为 component.Message）
                        ack_actor_msg = actor.Message(
                            Type=actor.MessageType.ACK,
                            Ack=common_messages.Ack()
                        )
                        ack_component_msg = self._wrap_actor_message(ack_actor_msg)
                        socket.send(ack_component_msg.SerializeToString())
                        return True
                    else:
                        return False
                else:
                    logger.warning(f"Unexpected message type while waiting for Function: {msg.Type}")
            except zmq.ZMQError as e:
                logger.error(f"ZMQ error while waiting for function: {e}")
                return False
            except Exception as e:
                logger.error(f"Error waiting for function: {e}", exc_info=True)
                return False

    def message_loop(self, socket: zmq.Socket):
        """
        Actor 主消息循环
        
        Actor 持续接收并处理消息，采用事件驱动模式。
        从 ZMQ 读取的是 component.Message，需要先提取 Payload 中的 actor.Message。
        
        Args:
            socket: ZMQ socket
        """
        while True:
            try:
                msg_bytes = socket.recv()
                # 首先解析为 component.Message
                component_msg = component.Message.FromString(msg_bytes)
                
                # 提取 Payload 中的 actor.Message
                msg = self._unwrap_component_message(component_msg)
                if msg is None:
                    logger.warning(f"Received non-PAYLOAD component message: {component_msg.Type}")
                    continue
                
                match msg.Type:
                    case actor.MessageType.INVOKE_REQUEST:
                        logger.info(f"Received INVOKE_REQUEST with {len(msg.InvokeRequest.Args)} args")
                        self.handle_invoke_request(msg.InvokeRequest)
                     
                    case _:
                        logger.warning(f"Unknown message type: {msg.Type}")
                        
            except zmq.ZMQError as e:
                logger.error(f"ZMQ error: {e}")
                break
            except Exception as e:
                logger.error(f"Error processing message: {e}", exc_info=True)

    def _send_loop(self, socket: zmq.Socket):
        """
        发送消息循环（后台线程）
        
        Actor 使用独立的发送线程，避免阻塞消息接收。
        发送的消息需要是 component.Message 类型。
        
        Args:
            socket: ZMQ socket
        """
        while True:
            msg = self.send_queue.get()
            self.send_queue.task_done()
            if msg is None:  # 收到 None 表示退出信号
                break
            try:
                # 如果消息是 actor.Message，需要包装为 component.Message
                if isinstance(msg, actor.Message):
                    component_msg = self._wrap_actor_message(msg)
                    socket.send(component_msg.SerializeToString())
                else:
                    logger.error(f"Unknown message type: {type(msg)}")
            except Exception as e:
                logger.error(f"Failed to send message: {e}")

    def run(self, zmq_addr: str, component_id: str = ""):
        """
        启动 Actor，建立 ZMQ 连接并开始处理消息
        
        Actor 启动流程：
        1. 建立 ZMQ 连接
        2. 设置身份标识
        3. 发送 READY 消息
        4. 等待并注册 Function
        5. 启动消息循环
        
        Args:
            zmq_addr: ZMQ 服务器地址
            component_id: 组件 ID（用于 ZMQ 身份标识）
        """
        ctx = zmq.Context()
        socket = ctx.socket(zmq.DEALER)
        
        # 设置 socket 身份标识，以便 Router 能够识别此 Actor
        if component_id:
            socket.setsockopt_string(zmq.IDENTITY, component_id)
            logger.info(f"Set ZMQ socket identity to: {component_id}")
        
        socket.connect(zmq_addr)
        logger.info(f"Connected to ZMQ: {zmq_addr}")
        
        # 发送初始 READY 消息，让 Go 端识别此 Actor 并发送缓存的 Function 消息
        ready_msg = component.Message(
            Type=component.MessageType.READY,
            Ready=common_messages.Ready()
        )
        socket.send(ready_msg.SerializeToString())
        logger.info("Initial READY message sent to identify actor")
        
        try:
            # 等待 Function 消息并注册函数
            if not self.wait_for_function(socket):
                logger.error("Failed to register function, exiting")
                return
            
            # 启动发送线程
            send_thread = threading.Thread(target=self._send_loop, args=(socket,))
            send_thread.daemon = True
            send_thread.start()
            
            # 进入主消息循环
            self.message_loop(socket)
        except Exception as e:
            logger.error(f"Actor stopped: {e}", exc_info=True)
        finally:
            # 清理资源
            self.send_queue.put(None)  # 通知发送线程退出
            socket.close()
            ctx.term()
            self.store_client.close()

