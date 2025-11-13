"""
Component protobuf types
"""
import sys
import os

# 添加 proto 目录到 sys.path，使 from common import ... 可以工作
_current_dir = os.path.dirname(os.path.abspath(__file__))
_proto_root = os.path.dirname(os.path.dirname(_current_dir))
if _proto_root not in sys.path:
    sys.path.insert(0, _proto_root)

