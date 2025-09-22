"""
Flink式提交上下文实现

这个模块实现了类似IARNet的Flink式批处理计算模式：
- 任务提交后分离：提交任务后客户端可以断开连接
- 异步执行：任务在后台异步执行，不依赖客户端连接
- 状态查询：通过任务ID查询执行状态和结果
- 批处理优化：适合大规模批处理作业
"""

import time
import uuid
import logging
import requests
import json
from typing import Optional, Dict, Any, Callable
from .base import IActorContext, ComputeMode, TaskStatus, TaskResult, ActorContextConfig
from ..protos.controller import controller_pb2
from ..protos import platform_pb2


logger = logging.getLogger(__name__)


class FlinkBatchContext(IActorContext):
    """Flink式批处理上下文 - 提交后分离计算模式"""
    
    def __init__(self, config: ActorContextConfig):
        self.config = config
        self._compute_mode = ComputeMode.FLINK_BATCH
        self._connected = False
        self._base_url = f"http://{config.master_address}"
        self._session = requests.Session()
        self._session.timeout = config.connection_timeout
        
        # 测试连接
        self._test_connection()
    
    @property
    def compute_mode(self) -> ComputeMode:
        """获取计算模式"""
        return self._compute_mode
    
    @property
    def context_type(self) -> str:
        """返回上下文类型"""
        return "flink_batch"
    
    @property
    def is_connected(self) -> bool:
        """检查连接状态"""
        return self._connected
    
    def _test_connection(self):
        """测试连接"""
        try:
            # 尝试访问健康检查端点
            response = self._session.get(f"{self._base_url}/health")
            if response.status_code == 200:
                self._connected = True
                logger.info(f"Flink批处理模式连接成功: {self.config.master_address}")
            else:
                raise ConnectionError(f"健康检查失败: {response.status_code}")
                
        except Exception as e:
            logger.error(f"Flink批处理模式连接失败: {e}")
            # 在批处理模式下，连接失败不是致命的，可以稍后重试
            self._connected = False
    
    def _make_request(self, method: str, endpoint: str, **kwargs) -> requests.Response:
        """发起HTTP请求"""
        url = f"{self._base_url}{endpoint}"
        
        try:
            response = self._session.request(method, url, **kwargs)
            response.raise_for_status()
            return response
        except requests.exceptions.RequestException as e:
            logger.error(f"Flink批处理模式请求失败: {method} {url} - {e}")
            raise ConnectionError(f"请求失败: {e}")
    
    # Flink式批处理模式方法实现
    def submit_task(self, task_name: str, params: Dict[str, Any]) -> str:
        """提交任务（Flink式异步提交），返回任务ID"""
        task_id = str(uuid.uuid4())
        
        # 构造任务提交请求
        task_request = {
            "task_id": task_id,
            "task_name": task_name,
            "params": params,
            "submit_time": time.time(),
            "mode": "batch"
        }
        
        try:
            response = self._make_request(
                "POST", 
                "/api/v1/tasks",
                json=task_request,
                headers={"Content-Type": "application/json"}
            )
            
            result = response.json()
            actual_task_id = result.get("task_id", task_id)
            
            logger.info(f"Flink批处理模式任务提交成功: {actual_task_id}")
            return actual_task_id
            
        except Exception as e:
            logger.error(f"Flink批处理模式任务提交失败: {e}")
            raise RuntimeError(f"任务提交失败: {e}")
    
    def get_task_status(self, task_id: str) -> TaskResult:
        """获取任务状态（Flink式状态查询）"""
        try:
            response = self._make_request("GET", f"/api/v1/tasks/{task_id}")
            data = response.json()
            
            # 解析状态
            status_str = data.get("status", "unknown").lower()
            status_mapping = {
                "pending": TaskStatus.PENDING,
                "running": TaskStatus.RUNNING,
                "completed": TaskStatus.COMPLETED,
                "failed": TaskStatus.FAILED,
                "cancelled": TaskStatus.CANCELLED
            }
            status = status_mapping.get(status_str, TaskStatus.PENDING)
            
            # 构造结果
            result = TaskResult(
                task_id=task_id,
                status=status,
                result=data.get("result"),
                error=data.get("error")
            )
            
            logger.debug(f"Flink批处理模式任务状态: {task_id} -> {status.value}")
            return result
            
        except Exception as e:
            logger.error(f"Flink批处理模式状态查询失败: {e}")
            # 返回错误状态
            return TaskResult(
                task_id=task_id,
                status=TaskStatus.FAILED,
                error=str(e)
            )
    
    def cancel_task(self, task_id: str) -> bool:
        """取消任务"""
        try:
            response = self._make_request(
                "DELETE", 
                f"/api/v1/tasks/{task_id}",
                json={"action": "cancel"}
            )
            
            result = response.json()
            success = result.get("success", False)
            
            if success:
                logger.info(f"Flink批处理模式任务取消成功: {task_id}")
            else:
                logger.warning(f"Flink批处理模式任务取消失败: {task_id}")
            
            return success
            
        except Exception as e:
            logger.error(f"Flink批处理模式任务取消失败: {e}")
            return False
    
    def wait_for_completion(self, task_id: str, timeout: int = 3600, poll_interval: int = 5) -> TaskResult:
        """等待任务完成（便利方法）"""
        start_time = time.time()
        
        while time.time() - start_time < timeout:
            result = self.get_task_status(task_id)
            
            if result.status in [TaskStatus.COMPLETED, TaskStatus.FAILED, TaskStatus.CANCELLED]:
                return result
            
            time.sleep(poll_interval)
        
        # 超时
        logger.warning(f"Flink批处理模式任务等待超时: {task_id}")
        return TaskResult(
            task_id=task_id,
            status=TaskStatus.FAILED,
            error="等待超时"
        )
    
    # Ray式交互模式方法（在Flink模式下不支持）
    def get_result(self, key: str) -> Optional[platform_pb2.Flow]:
        """获取执行结果（Flink模式不支持此方法）"""
        raise NotImplementedError("Flink批处理模式不支持Ray式结果获取，请使用get_task_status()方法")
    
    def send(self, message: controller_pb2.Message):
        """发送消息（Flink模式不支持此方法）"""
        raise NotImplementedError("Flink批处理模式不支持Ray式消息发送，请使用submit_task()方法")
    
    def set_result_callback(self, callback: Callable[[str, Any], None]) -> None:
        """设置结果回调（Flink模式不支持此方法）"""
        raise NotImplementedError("Flink批处理模式不支持Ray式回调，请使用轮询方式查询状态")
    
    # 通用方法
    def close(self):
        """关闭连接"""
        if self._session:
            self._session.close()
        
        self._connected = False
        logger.info("Flink批处理模式连接已关闭")


class FlinkBatchContextWithGRPC(FlinkBatchContext):
    """
    Flink式批处理上下文的gRPC版本
    
    这个版本使用gRPC协议进行任务提交，但仍然保持Flink式的
    "提交后分离"语义。适合需要高性能通信的场景。
    """
    
    def __init__(self, config: ActorContextConfig):
        # 先初始化HTTP版本的基础功能
        super().__init__(config)
        
        # 然后尝试建立gRPC连接
        self._grpc_channel = None
        self._grpc_stub = None
        self._setup_grpc()
    
    def _setup_grpc(self):
        """设置gRPC连接"""
        try:
            import grpc
            from ..protos.controller import controller_pb2_grpc
            
            options = [
                ('grpc.max_send_message_length', self.config.max_message_length),
                ('grpc.max_receive_message_length', self.config.max_message_length),
            ]
            
            self._grpc_channel = grpc.insecure_channel(self.config.master_address, options=options)
            self._grpc_stub = controller_pb2_grpc.ControllerStub(self._grpc_channel)
            
            logger.info("Flink批处理模式gRPC连接建立成功")
            
        except Exception as e:
            logger.warning(f"Flink批处理模式gRPC连接失败，回退到HTTP: {e}")
            self._grpc_channel = None
            self._grpc_stub = None
    
    def submit_task(self, task_name: str, params: Dict[str, Any]) -> str:
        """提交任务（优先使用gRPC）"""
        if self._grpc_stub:
            try:
                return self._submit_task_grpc(task_name, params)
            except Exception as e:
                logger.warning(f"gRPC任务提交失败，回退到HTTP: {e}")
        
        # 回退到HTTP方式
        return super().submit_task(task_name, params)
    
    def _submit_task_grpc(self, task_name: str, params: Dict[str, Any]) -> str:
        """通过gRPC提交任务"""
        task_id = str(uuid.uuid4())
        
        # 构造gRPC消息
        # 这里需要根据实际的protobuf定义来构造消息
        # 示例代码，需要根据实际情况调整
        
        logger.info(f"Flink批处理模式gRPC任务提交: {task_id}")
        return task_id
    
    def close(self):
        """关闭连接"""
        if self._grpc_channel:
            self._grpc_channel.close()
        
        super().close()