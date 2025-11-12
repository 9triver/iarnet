import inspect
import json
import logging
import os
import queue
import subprocess
import sys
import threading
import uuid
from collections.abc import Callable, Iterable
from typing import Any, NamedTuple, Optional

import cloudpickle
import grpc
import zmq

from proto.ignis.cluster import cluster_pb2 as cluster
from proto.ignis import platform_pb2 as platform
from proto.resource.store import store_pb2 as store_pb
from proto.resource.store import store_pb2_grpc as store_grpc

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    datefmt='%Y-%m-%d %H:%M:%S'
)
logger = logging.getLogger(__name__)


class RemoteFunction(NamedTuple):
    language: platform.Language
    fn: Callable[..., Any]

    def call(self, *args, **kwargs):
        return self.fn(**kwargs)


class StoreClient:
    """gRPC client for store service"""

    def __init__(self, store_addr: str):
        # self.channel = grpc.insecure_channel(store_addr)
        # self.stub = store_grpc.ServiceStub(self.channel)

        self.store_addr = store_addr

    def close(self):
        self.channel.close()

    def get_object(self, object_id: str, source: str = "") -> Optional[store_pb.EncodedObject]:
        """Get object from store by ObjectRef"""
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
        self, data: bytes, language: store_pb.Language, object_id: str = "", is_stream: bool = False
    ) -> Optional[store_pb.ObjectRef]:
        """Save object to store and return ObjectRef"""
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
    """Encode/decode objects with language support"""

    @staticmethod
    def next_id() -> str:
        return f"obj.{uuid.uuid4()}"

    @staticmethod
    def decode(obj: store_pb.EncodedObject) -> Any:
        """Decode EncodedObject from store"""
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
        """Encode object to bytes and return (data, is_stream)"""
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
        """Convert platform.Language to store.Language"""
        match lang:
            case platform.LANG_JSON:
                return store_pb.LANGUAGE_JSON
            case platform.LANG_PYTHON:
                return store_pb.LANGUAGE_PYTHON
            case platform.LANG_GO:
                return store_pb.LANGUAGE_GO
            case _:
                return store_pb.LANGUAGE_UNKNOWN


class Executor:
    def __init__(self, store_client: StoreClient):
        self.store_client = store_client
        self.function: Optional[RemoteFunction] = None  # Single function to execute
        self.function_name: Optional[str] = None
        self.expected_params: set[str] = set()  # Expected parameters for the function
        self.send_q = queue.Queue[cluster.Message | None]()
        self.session_id: Optional[str] = None
        self.invoke_params: dict[str, Any] = {}  # Store params for current invoke session
        self.invoke_start_msg: Optional[platform.InvokeStart] = None  # Store InvokeStart for response

    def _install_requirements(self, requirements: list[str]) -> bool:
        """Install Python package requirements using pip in virtual environment"""
        if not requirements:
            return True
        
        logger.info(f"Installing {len(requirements)} dependencies: {requirements}")
        try:
            # Use the same Python executable (which should be from venv)
            # This ensures packages are installed in the virtual environment
            cmd = [
                sys.executable, "-m", "pip", "install",
                "--quiet",  # Reduce output
                "--no-cache-dir",  # Don't cache packages
                "--no-warn-script-location"  # Suppress warnings
            ] + requirements
            
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=300  # 5 minutes timeout
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
        """Handle Function message - register function. Returns True if successful."""
        # Install dependencies if specified
        if msg.Requirements:
            if not self._install_requirements(list(msg.Requirements)):
                logger.warning(f"Failed to install some dependencies for {msg.Name}, continuing anyway")
        
        try:
            fn = cloudpickle.loads(msg.PickledObject)
            if not callable(fn):
                logger.error(f"Function {msg.Name} is not callable")
                return False

            self.function = RemoteFunction(msg.Language, fn)
            self.function_name = msg.Name
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

    def on_invoke_start(self, msg: platform.InvokeStart):
        """Handle InvokeStart message - prepare for invocation"""
        self.session_id = msg.SessionID
        self.invoke_params = {}
        self.invoke_start_msg = msg
        logger.info(f"InvokeStart: session={msg.SessionID}, reply_to={msg.ReplyTo}, function={self.function_name}, expected_params={self.expected_params}")

    def on_invoke(self, msg: platform.Invoke):
        """Handle Invoke message - provide parameter value"""
        if not self.session_id or self.session_id != msg.SessionID:
            logger.warning(f"Session ID mismatch: expected {self.session_id}, got {msg.SessionID}")
            return

        # Get object from store using Flow ref
        flow = msg.Value
        store_obj = self.store_client.get_object(flow.ID, flow.Source.ID if flow.Source else "")
        
        if store_obj is None:
            logger.error(f"Failed to get object {flow.ID} from store")
            return

        # Decode object
        try:
            value = EncDec.decode(store_obj)
            self.invoke_params[msg.Param] = value
            logger.debug(f"Invoke: param={msg.Param}, value_type={type(value).__name__}, collected={len(self.invoke_params)}/{len(self.expected_params)}")
            
            # Check if all parameters are collected
            if self.expected_params and self.expected_params.issubset(set(self.invoke_params.keys())):
                logger.info(f"All parameters collected, executing function {self.function_name}")
                self._execute_and_respond()
        except Exception as e:
            logger.error(f"Failed to decode object {flow.ID}: {e}")

    def _execute_and_respond(self):
        """Execute function and send InvokeResponse"""
        if not self.function:
            logger.error("Function not registered")
            return

        if not self.invoke_start_msg:
            logger.error("No InvokeStart message stored")
            return

        func = self.function
        error_msg = None
        result_flow = None
        
        try:
            logger.info(f"Executing function {self.function_name} with params: {list(self.invoke_params.keys())}")
            value = func.call(**self.invoke_params)
            
            # Encode result
            store_lang = EncDec.platform_to_store_language(func.language)
            data, is_stream = EncDec.encode(value, store_lang)
            
            # Save to store
            object_ref = self.store_client.save_object(data, store_lang, is_stream=is_stream)
            if object_ref is None:
                error_msg = "Failed to save result to store"
                logger.error(error_msg)
            else:
                # Create Flow reference
                result_flow = platform.Flow(
                    ID=object_ref.ID,
                    Source=platform.StoreRef(ID=object_ref.Source)
                )
                logger.info(f"Function {self.function_name} completed, result saved as {object_ref.ID}")
        except Exception as e:
            error_msg = f"{e.__class__.__name__}: {e}"
            logger.error(f"Function {self.function_name} execution failed: {error_msg}", exc_info=True)
        
        # Send InvokeResponse
        invoke_response = platform.InvokeResponse(
            Target="",  # Will be set by controller
            SessionID=self.session_id,
            Result=result_flow if result_flow else None,
            Error=error_msg if error_msg else ""
        )
        
        response_msg = cluster.Message(
            Type=cluster.INVOKE_RESPONSE,
            InvokeResponse=invoke_response
        )
        self.send_q.put(response_msg)
        
        # Reset for next invocation
        self.invoke_params = {}
        self.invoke_start_msg = None

    def wait_for_function(self, socket: zmq.Socket) -> bool:
        """Wait for and register Function message. Returns True if successful."""
        logger.info("Waiting for Function message...")
        while True:
            try:
                msg_bytes = socket.recv()
                msg = cluster.Message.FromString(msg_bytes)
                
                if msg.Type == cluster.FUNCTION:
                    logger.info(f"Received FUNCTION: {msg.Function.Name}")
                    if self.on_function(msg.Function):
                        # Send ACK
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
        """Main message loop"""
        while True:
            try:
                msg_bytes = socket.recv()
                msg = cluster.Message.FromString(msg_bytes)
                
                match msg.Type:
                        
                    case cluster.INVOKE_START:
                        logger.info("Received INVOKE_START")
                        self.on_invoke_start(msg.InvokeStart)
                        # Send ACK
                        ack_msg = cluster.Message(
                            Type=cluster.ACK,
                            Ack=cluster.Ack()
                        )
                        self.send_q.put(ack_msg)
                        
                    case cluster.INVOKE:
                        logger.debug(f"Received INVOKE: param={msg.Invoke.Param}")
                        self.on_invoke(msg.Invoke)
                        # Send ACK
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
        """Send messages from queue"""
        while True:
            msg = self.send_q.get()
            self.send_q.task_done()
            if msg is None:
                break
            try:
                socket.send(msg.SerializeToString())
            except Exception as e:
                logger.error(f"Failed to send message: {e}")

    def serve(self, zmq_addr: str, component_id: str = ""):
        """Start serving"""
        ctx = zmq.Context()
        socket = ctx.socket(zmq.DEALER)
        
        # Set socket identity to component ID so Router can identify this component
        if component_id:
            socket.setsockopt_string(zmq.IDENTITY, component_id)
            logger.info(f"Set ZMQ socket identity to: {component_id}")
        
        socket.connect(zmq_addr)
        
        logger.info(f"Connected to ZMQ: {zmq_addr}")
        
        # Send initial READY message immediately after connection
        # This allows Go side to identify this component and send cached Function message
        ready_msg = cluster.Message(
            Type=cluster.READY,
            Ready=cluster.Ready()
        )
        socket.send(ready_msg.SerializeToString())
        logger.info("Initial READY message sent to identify component")
        
        try:
            # Now wait for Function message and register it
            if not self.wait_for_function(socket):
                logger.error("Failed to register function, exiting")
                return
            
            # Start send thread
            send_thread = threading.Thread(target=self._start_send, args=(socket,))
            send_thread.daemon = True
            send_thread.start()
            
            # Main loop
            self.loop(socket)
        except Exception as e:
            logger.error(f"Executor stopped: {e}", exc_info=True)
        finally:
            self.send_q.put(None)
            socket.close()
            ctx.term()
            self.store_client.close()


def main():
    # Read environment variables
    zmq_addr = os.getenv("ZMQ_ADDR")
    store_addr = os.getenv("STORE_ADDR")
    component_id = os.getenv("COMPONENT_ID")
    
    if not zmq_addr:
        logger.error("ZMQ_ADDR environment variable is required")
        sys.exit(1)
    if not store_addr:
        logger.error("STORE_ADDR environment variable is required")
        sys.exit(1)
    if not component_id:
        logger.error("COMPONENT_ID environment variable is required")
        sys.exit(1)
    
    # Ensure ZMQ address has protocol prefix
    if not zmq_addr.startswith(("tcp://", "ipc://", "inproc://")):
        zmq_addr = f"tcp://{zmq_addr}"
    
    logger.info(f"Starting executor: zmq={zmq_addr}, store={store_addr}, component_id={component_id}")
    
    # Create store client
    store_client = StoreClient(store_addr)
    
    # Create and start executor
    executor = Executor(store_client)
    executor.serve(zmq_addr, component_id)


if __name__ == "__main__":
    main()
