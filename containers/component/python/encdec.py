"""
对象编解码工具类
支持不同语言的序列化/反序列化
"""

import inspect
import json
from typing import Any

import cloudpickle

from proto.common import types_pb2 as common
from proto.resource.store import store_pb2 as store_pb


class EncDec:
    """对象编解码工具类，支持不同语言的序列化/反序列化"""
    
    @staticmethod
    def next_id() -> str:
        """生成下一个对象 ID"""
        import uuid
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
    def platform_to_store_language(lang: common.Language) -> store_pb.Language:
        """
        将平台语言类型转换为 Store 语言类型
        
        Args:
            lang: 平台语言类型（common.Language）
            
        Returns:
            Store 语言类型
        """
        match lang:
            case common.LANG_JSON:
                return store_pb.LANGUAGE_JSON
            case common.LANG_PYTHON:
                return store_pb.LANGUAGE_PYTHON
            case common.LANG_GO:
                return store_pb.LANGUAGE_GO
            case _:
                return store_pb.LANGUAGE_UNKNOWN

