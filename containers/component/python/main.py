"""
Python Component Executor
用于执行远程函数的 Python 组件，通过 ZMQ 与 Go 服务端通信，通过 gRPC 与 Store 服务通信
"""

import inspect
import json
import logging
import os
import queue
import subprocess
import sys
import threading
import uuid
from collections.abc import Callable
from typing import Any, NamedTuple, Optional

import cloudpickle
import grpc
import zmq

from proto.ignis.cluster import cluster_pb2 as cluster
from proto.ignis import platform_pb2 as platform
from proto.resource.store import store_pb2 as store_pb
from proto.resource.store import store_pb2_grpc as store_grpc

# ============================================================================
# 日志配置
# ============================================================================
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    datefmt='%Y-%m-%d %H:%M:%S'
)
logger = logging.getLogger(__name__)


# ============================================================================
# 工具类
# ============================================================================

class RemoteFunction(NamedTuple):
    """远程函数封装，包含函数对象和语言类型"""
    language: platform.Language  # 函数返回值的语言类型
    fn: Callable[..., Any]        # 可调用的函数对象

    def call(self, *args, **kwargs) -> Any:
        """调用函数，使用关键字参数"""
        return self.fn(**kwargs)


class StoreClient:
    """Store 服务的 gRPC 客户端，用于获取和保存对象"""
    
    def __init__(self, store_addr: str):
        """
        初始化 Store 客户端
        
        Args:
            store_addr: Store 服务的地址（格式：host:port）
        """
        # TODO: 当前 Store 客户端被注释，需要时取消注释
        # self.channel = grpc.insecure_channel(store_addr)
        # self.stub = store_grpc.ServiceStub(self.channel)
        self.store_addr = store_addr

    def close(self):
        """关闭 gRPC 连接"""
        if hasattr(self, 'channel'):
            self.channel.close()

    def get_object(self, object_id: str, source: str = "") -> Optional[store_pb.EncodedObject]:
        """
        从 Store 获取对象
        
        Args:
            object_id: 对象 ID
            source: 对象来源（可选）
            
        Returns:
            编码后的对象，如果获取失败返回 None
        """
        try:
            request = store_pb.GetObjectRequest(
                ObjectRef=store_pb.ObjectRef(ID=object_id, Source=source)
            )
            response = self.stub.GetObject(request)
            return response.Object
        except Exception as e:
            logger.error(f"Failed to get object {object_id}: {e}")
            return None

    def save_object(
        self, 
        data: bytes, 
        language: store_pb.Language, 
        object_id: str = "", 
        is_stream: bool = False
    ) -> Optional[store_pb.ObjectRef]:
        """
        保存对象到 Store
        
        Args:
            data: 对象的字节数据
            language: 对象的语言类型
            object_id: 对象 ID（如果为空则自动生成）
            is_stream: 是否为流对象
            
        Returns:
            对象引用，如果保存失败返回 None
        """
        try:
            if not object_id:
                object_id = f"obj.{uuid.uuid4()}"
            
            request = store_pb.SaveObjectRequest(
                Object=store_pb.EncodedObject(
                    ID=object_id,
                    Data=data,
                    Language=language,
                    IsStream=is_stream,
                )
            )
            response = self.stub.SaveObject(request)
            if response.Success:
                return response.ObjectRef
            else:
                logger.error(f"Failed to save object: {response.Error}")
                return None
        except Exception as e:
            logger.error(f"Failed to save object: {e}")
            return None


class EncDec:
    """对象编解码工具类，支持不同语言的序列化/反序列化"""
    
    @staticmethod
    def next_id() -> str:
        """生成下一个对象 ID"""
        return f"obj.{uuid.uuid4()}"

    @staticmethod
    def decode(obj: store_pb.EncodedObject) -> Any:
        """
        解码 Store 中的编码对象
        
        Args:
            obj: 编码后的对象
            
        Returns:
            解码后的 Python 对象
            
        Raises:
            ValueError: 如果对象是流或语言类型不支持
        """
        if obj.IsStream:
            raise ValueError("Stream objects should be handled separately")

        data = obj.Data
        match obj.Language:
            case store_pb.LANGUAGE_PYTHON:
                return cloudpickle.loads(data)
            case store_pb.LANGUAGE_JSON:
                return json.loads(data)
            case _:
                raise ValueError(f"Unsupported language: {obj.Language}")

    @classmethod
    def encode(cls, obj: Any, language: store_pb.Language = store_pb.LANGUAGE_JSON) -> tuple[bytes, bool]:
        """
        编码 Python 对象为字节数据
        
        Args:
            obj: 要编码的 Python 对象
            language: 目标语言类型
            
        Returns:
            (编码后的字节数据, 是否为流对象)
        """
        if inspect.isgenerator(obj):
            return b"", True

        match language:
            case store_pb.LANGUAGE_PYTHON:
                data = cloudpickle.dumps(obj)
            case store_pb.LANGUAGE_JSON:
                data = json.dumps(obj).encode()
            case _:
                raise ValueError(f"Unsupported language: {language}")
        return data, False

    @staticmethod
    def platform_to_store_language(lang: platform.Language) -> store_pb.Language:
        """
        将平台语言类型转换为 Store 语言类型
        
        Args:
            lang: 平台语言类型
            
        Returns:
            Store 语言类型
        """
        match lang:
            case platform.LANG_JSON:
                return store_pb.LANGUAGE_JSON
            case platform.LANG_PYTHON:
                return store_pb.LANGUAGE_PYTHON
            case platform.LANG_GO:
                return store_pb.LANGUAGE_GO
            case _:
                return store_pb.LANGUAGE_UNKNOWN


# ============================================================================
# 执行器主类
# ============================================================================

class Executor:
    """
    函数执行器，负责：
    1. 接收并注册远程函数
    2. 接收批量调用请求
    3. 从 Store 获取参数
    4. 执行函数
    5. 将结果保存到 Store
    6. 发送响应消息
    """
    
    def __init__(self, store_client: StoreClient):
        """
        初始化执行器
        
        Args:
            store_client: Store 客户端实例
        """
        self.store_client = store_client
        self.function: Optional[RemoteFunction] = None      # 当前注册的函数
        self.function_name: Optional[str] = None            # 函数名称
        self.expected_params: set[str] = set()              # 函数期望的参数名集合
        self.send_q = queue.Queue[cluster.Message | None]() # 发送消息队列
        self.session_id: Optional[str] = None               # 当前会话 ID

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

    def on_function(self, msg: cluster.Function) -> bool:
        """
        处理 Function 消息，注册函数
        
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
    
    def on_invoke_request(self, msg: cluster.InvokeRequest):
        """
        处理批量调用请求（InvokeRequest）
        
        在新线程中执行，避免阻塞主消息循环
        
        Args:
            msg: InvokeRequest 消息，包含会话 ID 和参数列表
        """
        logger.info(
            "InvokeRequest received",
            extra={
                "session": msg.SessionID,
                "instance": msg.InstanceID,
                "args": len(msg.Args),
            }
        )

        # 在新线程中执行，避免阻塞主消息循环
        thread = threading.Thread(
            target=self._handle_invoke_request,
            args=(msg,),
            daemon=True
        )
        thread.start()

    def _handle_invoke_request(self, msg: cluster.InvokeRequest):
        """
        在新线程中处理批量调用请求
        
        流程：
        1. 从 Store 获取所有参数值
        2. 解码参数
        3. 执行函数
        4. 发送响应
        
        Args:
            msg: InvokeRequest 消息，包含会话 ID 和参数列表
        """
        session_id = msg.SessionID
        
        # 收集所有参数到局部字典（不使用实例变量，避免状态污染）
        invoke_params = {}
        
        for arg in msg.Args:
            # 验证 Flow 引用
            if not arg.Value or not arg.Value.ID:
                logger.error(f"Invalid flow for param {arg.Param}")
                continue
                
            # 从 Store 获取对象
            store_obj = self.store_client.get_object(
                arg.Value.ID, 
                arg.Value.Source.ID if arg.Value.Source else ""
            )
            if store_obj is None:
                logger.error(f"Failed to get object {arg.Value.ID} from store")
                continue
                
            # 解码对象并添加到参数字典
            try:
                value = EncDec.decode(store_obj)
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
            self._execute_and_respond(invoke_params, session_id)
        else:
            logger.error(f"Failed to collect all parameters: got {len(invoke_params)}/{len(msg.Args)}")

    def _execute_and_respond(self, invoke_params: dict[str, Any], session_id: str):
        """
        执行函数并发送响应
        
        Args:
            invoke_params: 函数参数字典
            session_id: 会话 ID
        """
        if not self.function:
            logger.error("Function not registered")
            return

        func = self.function
        error_msg = None
        result_flow = None
        
        try:
            # 执行函数
            logger.info(f"Executing function {self.function_name} with params: {list(invoke_params.keys())}")
            value = func.call(**invoke_params)
            
            # 编码结果
            store_lang = EncDec.platform_to_store_language(func.language)
            data, is_stream = EncDec.encode(value, store_lang)
            
            # 保存结果到 Store
            object_ref = self.store_client.save_object(data, store_lang, is_stream=is_stream)
            if object_ref is None:
                error_msg = "Failed to save result to store"
                logger.error(error_msg)
            else:
                # 创建 Flow 引用
                result_flow = platform.Flow(
                    ID=object_ref.ID,
                    Source=platform.StoreRef(ID=object_ref.Source)
                )
                logger.info(f"Function {self.function_name} completed, result saved as {object_ref.ID}")
        except Exception as e:
            error_msg = f"{e.__class__.__name__}: {e}"
            logger.error(f"Function {self.function_name} execution failed: {error_msg}", exc_info=True)
        
        # 发送 InvokeResponse
        invoke_response = platform.InvokeResponse(
            SessionID=session_id,
            Result=result_flow if result_flow else None,
            Error=error_msg if error_msg else ""
        )
        
        response_msg = cluster.Message(
            Type=cluster.INVOKE_RESPONSE,
            InvokeResponse=invoke_response
        )
        self.send_q.put(response_msg)

    # ========================================================================
    # ZMQ 通信相关方法
    # ========================================================================
    
    def wait_for_function(self, socket: zmq.Socket) -> bool:
        """
        等待并注册 Function 消息（启动时调用）
        
        Args:
            socket: ZMQ socket
            
        Returns:
            注册是否成功
        """
        logger.info("Waiting for Function message...")
        while True:
            try:
                msg_bytes = socket.recv()
                msg = cluster.Message.FromString(msg_bytes)
                
                if msg.Type == cluster.FUNCTION:
                    logger.info(f"Received FUNCTION: {msg.Function.Name}")
                    if self.on_function(msg.Function):
                        # 发送 ACK 确认
                        ack_msg = cluster.Message(
                            Type=cluster.ACK,
                            Ack=cluster.Ack()
                        )
                        socket.send(ack_msg.SerializeToString())
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

    def loop(self, socket: zmq.Socket):
        """
        主消息循环，持续接收并处理消息
        
        Args:
            socket: ZMQ socket
        """
        while True:
            try:
                msg_bytes = socket.recv()
                msg = cluster.Message.FromString(msg_bytes)
                
                match msg.Type:
                    case cluster.INVOKE_REQUEST:
                        logger.info(f"Received INVOKE_REQUEST with {len(msg.InvokeRequest.Args)} args")
                        self.on_invoke_request(msg.InvokeRequest)
                        # 发送 ACK 确认
                        ack_msg = cluster.Message(
                            Type=cluster.ACK,
                            Ack=cluster.Ack()
                        )
                        self.send_q.put(ack_msg)
                     
                    case _:
                        logger.warning(f"Unknown message type: {msg.Type}")
                        
            except zmq.ZMQError as e:
                logger.error(f"ZMQ error: {e}")
                break
            except Exception as e:
                logger.error(f"Error processing message: {e}", exc_info=True)

    def _start_send(self, socket: zmq.Socket):
        """
        发送消息线程（后台运行）
        
        Args:
            socket: ZMQ socket
        """
        while True:
            msg = self.send_q.get()
            self.send_q.task_done()
            if msg is None:  # 收到 None 表示退出信号
                break
            try:
                socket.send(msg.SerializeToString())
            except Exception as e:
                logger.error(f"Failed to send message: {e}")

    def serve(self, zmq_addr: str, component_id: str = ""):
        """
        启动服务，建立 ZMQ 连接并开始处理消息
        
        Args:
            zmq_addr: ZMQ 服务器地址
            component_id: 组件 ID（用于 ZMQ 身份标识）
        """
        ctx = zmq.Context()
        socket = ctx.socket(zmq.DEALER)
        
        # 设置 socket 身份标识，以便 Router 能够识别此组件
        if component_id:
            socket.setsockopt_string(zmq.IDENTITY, component_id)
            logger.info(f"Set ZMQ socket identity to: {component_id}")
        
        socket.connect(zmq_addr)
        logger.info(f"Connected to ZMQ: {zmq_addr}")
        
        # 发送初始 READY 消息，让 Go 端识别此组件并发送缓存的 Function 消息
        ready_msg = cluster.Message(
            Type=cluster.READY,
            Ready=cluster.Ready()
        )
        socket.send(ready_msg.SerializeToString())
        logger.info("Initial READY message sent to identify component")
        
        try:
            # 等待 Function 消息并注册函数
            if not self.wait_for_function(socket):
                logger.error("Failed to register function, exiting")
                return
            
            # 启动发送线程
            send_thread = threading.Thread(target=self._start_send, args=(socket,))
            send_thread.daemon = True
            send_thread.start()
            
            # 进入主消息循环
            self.loop(socket)
        except Exception as e:
            logger.error(f"Executor stopped: {e}", exc_info=True)
        finally:
            # 清理资源
            self.send_q.put(None)  # 通知发送线程退出
            socket.close()
            ctx.term()
            self.store_client.close()


# ============================================================================
# 主函数
# ============================================================================

def main():
    """主入口函数"""
    # 读取环境变量
    zmq_addr = os.getenv("ZMQ_ADDR")
    store_addr = os.getenv("STORE_ADDR")
    component_id = os.getenv("COMPONENT_ID")
    
    # 验证必需的环境变量
    if not zmq_addr:
        logger.error("ZMQ_ADDR environment variable is required")
        sys.exit(1)
    if not store_addr:
        logger.error("STORE_ADDR environment variable is required")
        sys.exit(1)
    if not component_id:
        logger.error("COMPONENT_ID environment variable is required")
        sys.exit(1)
    
    # 确保 ZMQ 地址包含协议前缀
    if not zmq_addr.startswith(("tcp://", "ipc://", "inproc://")):
        zmq_addr = f"tcp://{zmq_addr}"
    
    logger.info(f"Starting executor: zmq={zmq_addr}, store={store_addr}, component_id={component_id}")
    
    # 创建 Store 客户端和执行器
    store_client = StoreClient(store_addr)
    executor = Executor(store_client)
    
    # 启动服务
    executor.serve(zmq_addr, component_id)


if __name__ == "__main__":
    main()
