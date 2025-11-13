"""
数据模型定义
"""

from collections.abc import Callable
from typing import Any, NamedTuple

from proto.ignis import platform_pb2 as platform


class RemoteFunction(NamedTuple):
    """远程函数封装，包含函数对象和语言类型"""
    language: platform.Language  # 函数返回值的语言类型
    fn: Callable[..., Any]        # 可调用的函数对象

    def call(self, *args, **kwargs) -> Any:
        """调用函数，使用关键字参数"""
        return self.fn(**kwargs)

