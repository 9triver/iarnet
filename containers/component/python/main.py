"""
Python Component Main Entry
每个组件中运行着一个 Actor，负责接收消息、执行函数、返回响应。

组件通过以下方式与系统通信：
- ZMQ: 与控制器通信，接收 Function 和 InvokeRequest 消息，发送响应
- gRPC: 与 Store 服务通信，获取参数和保存结果
"""

import logging
import os
import sys

# 添加当前目录到 Python 路径，以便导入模块
_current_dir = os.path.dirname(os.path.abspath(__file__))
if _current_dir not in sys.path:
    sys.path.insert(0, _current_dir)

from actor import Actor
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
    application_id = os.getenv("APPLICATION_ID")
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

    logger.info(f"Starting actor: zmq={zmq_addr}, store={store_addr}, component_id={component_id}")
    
    # 创建 Store 客户端和 Actor
    store_client = StoreClient(store_addr)
    actor = Actor(store_client)
    
    # 启动 Actor，开始接收和处理消息
    actor.run(zmq_addr, component_id)


if __name__ == "__main__":
    main()
