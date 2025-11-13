"""
Python Component Executor
用于执行远程函数的 Python 组件，通过 ZMQ 与 Go 服务端通信，通过 gRPC 与 Store 服务通信
"""

import logging
import os
import sys

# 添加当前目录到 Python 路径，以便导入模块
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from executor import Executor
from store_client import StoreClient

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
