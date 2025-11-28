import logging
import queue
import threading
import time
import traceback
from typing import Optional

import grpc

from proto.common import logger_pb2 as common_logger_pb2
from proto.resource.logger import logger_pb2, logger_pb2_grpc

# logging 级别到 proto LogLevel 的映射
_LEVEL_MAP = {
    logging.DEBUG: common_logger_pb2.LOG_LEVEL_DEBUG,
    logging.INFO: common_logger_pb2.LOG_LEVEL_INFO,
    logging.WARNING: common_logger_pb2.LOG_LEVEL_WARN,
    logging.ERROR: common_logger_pb2.LOG_LEVEL_ERROR,
    logging.CRITICAL: common_logger_pb2.LOG_LEVEL_FATAL,
}


_GLOBAL_HANDLER: Optional["RemoteLogHandler"] = None
_GLOBAL_HANDLER_LOCK = threading.Lock()


def setup_global_logging(component_id: str, logger_addr: str, level: int = logging.INFO):
    """
    将 RemoteLogHandler 安装到 root logger，确保所有日志都发送到远程服务。
    """
    global _GLOBAL_HANDLER

    with _GLOBAL_HANDLER_LOCK:
        root_logger = logging.getLogger()
        root_logger.setLevel(level)

        if _GLOBAL_HANDLER:
            if (
                _GLOBAL_HANDLER.component_id == component_id
                and _GLOBAL_HANDLER.logger_addr == logger_addr
            ):
                return _GLOBAL_HANDLER
            root_logger.removeHandler(_GLOBAL_HANDLER)
            _GLOBAL_HANDLER.close()
            _GLOBAL_HANDLER = None

        handler = RemoteLogHandler(component_id, logger_addr)

        # 移除已存在的 RemoteLogHandler，避免重复发送
        for existing in list(root_logger.handlers):
            if isinstance(existing, RemoteLogHandler):
                root_logger.removeHandler(existing)
                existing.close()

        root_logger.addHandler(handler)
        _GLOBAL_HANDLER = handler
        return handler


class RemoteLogHandler(logging.Handler):

    def __init__(self, component_id: str, logger_addr: str):
        super().__init__()

        self._logger_addr = logger_addr
        self._component_id = component_id

        # 创建 gRPC 连接
        self._channel = grpc.insecure_channel(
            logger_addr,
            options=[("grpc.max_receive_message_length", 512 * 1024 * 1024)],
        )
        self._stub = logger_pb2_grpc.LoggerServiceStub(self._channel)
        self._q = queue.Queue()
        self._response_stream = self._stub.StreamLogs(self._generate())
        self._thread = threading.Thread(target=self._run, daemon=True)
        self._thread.start()

    def _generate(self):
        while True:
            msg = self._q.get()
            yield msg

    def _run(self):
        while True:
            try:
                for response in self._response_stream:
                    if not response.success and response.error:
                        print(f"Log service error: {response.error}")
            except Exception as e:
                print(f"Stream error: {e}")
                time.sleep(1)

    def emit(self, record: logging.LogRecord):
        try:
            proto_level = _LEVEL_MAP.get(
                record.levelno, common_logger_pb2.LOG_LEVEL_UNKNOWN)

            fields = []
            standard_attrs = {
                'name', 'msg', 'args', 'created', 'filename', 'funcName',
                'levelname', 'levelno', 'lineno', 'module', 'msecs', 'message',
                'pathname', 'process', 'processName', 'relativeCreated', 'thread',
                'threadName', 'exc_info', 'exc_text', 'stack_info', 'getMessage', "asctime"
            }

            # 遍历 LogRecord 的所有属性，找出自定义字段
            import json
            for key, value in record.__dict__.items():
                # 跳过标准属性和私有属性
                if key in standard_attrs or key.startswith('_'):
                    continue
                # 将自定义字段添加到 fields
                field = common_logger_pb2.LogField(
                    key=key,
                    value=json.dumps(value) if not isinstance(
                        value, str) else value
                )
                fields.append(field)

            # 添加异常信息（如果有）
            if record.exc_info:
                exc_text = self.formatException(record.exc_info)
                fields.append(common_logger_pb2.LogField(
                    key='exception',
                    value=exc_text
                ))

            stream_msg = logger_pb2.LogStreamMessage(
                component_id=self._component_id,
                entry=common_logger_pb2.LogEntry(
                    timestamp=int(time.time_ns()),
                    level=proto_level,
                    message=record.getMessage(),
                    fields=fields,
                    caller=common_logger_pb2.CallerInfo(
                        file=record.filename,
                        line=record.lineno,
                        function=record.funcName,
                    ),
                )
            )
            self._q.put(stream_msg)
        except Exception as e:
            print(f"Log emit error: {e}")

    def formatException(self, exc_info):
        if self.formatter:
            return self.formatter.formatException(exc_info)
        return "".join(traceback.format_exception(*exc_info))
