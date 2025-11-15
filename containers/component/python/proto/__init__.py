"""
Protobuf 模块根目录
添加 proto 目录到 sys.path，使生成的 protobuf 文件可以正确导入 common 等模块
"""
import sys
import os

# 添加 proto 目录到 sys.path，使 from common import ... 可以工作
_current_dir = os.path.dirname(os.path.abspath(__file__))
if _current_dir not in sys.path:
    sys.path.insert(0, _current_dir)

