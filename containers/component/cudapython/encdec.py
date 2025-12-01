"""
对象编解码工具类
支持不同语言的序列化/反序列化
"""

import inspect
import json
from typing import Any

import cloudpickle

from proto.common import types_pb2 as common


class EncDec:
    """对象编解码工具类，支持不同语言的序列化/反序列化"""
    
    @staticmethod
    def next_id() -> str:
        """生成下一个对象 ID"""
        import uuid
        return f"obj.{uuid.uuid4()}"

    @staticmethod
    def decode(obj: common.EncodedObject) -> Any:
        """
        解码 Store 中的编码对象
        
        Args:
            obj: 编码后的对象（common.EncodedObject）
            
        Returns:
            解码后的 Python 对象
            
        Raises:
            ValueError: 如果对象是流或语言类型不支持
        """
        if obj.IsStream:
            raise ValueError("Stream objects should be handled separately")

        data = obj.Data
        match obj.Language:
            case common.LANG_PYTHON:
                return cloudpickle.loads(data)
            case common.LANG_JSON:
                return json.loads(data)
            case _:
                raise ValueError(f"Unsupported language: {obj.Language}")

    @classmethod
    def encode(cls, obj: Any, language: common.Language = common.LANG_JSON) -> tuple[bytes, bool]:
        """
        编码 Python 对象为字节数据
        
        Args:
            obj: 要编码的 Python 对象
            language: 目标语言类型（common.Language）
            
        Returns:
            (编码后的字节数据, 是否为流对象)
        """
        if inspect.isgenerator(obj):
            return b"", True

        match language:
            case common.LANG_PYTHON:
                data = cloudpickle.dumps(obj)
            case common.LANG_JSON:
                data = json.dumps(obj).encode()
            case _:
                raise ValueError(f"Unsupported language: {language}")
        return data, False

