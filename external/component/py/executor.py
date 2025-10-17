#!/usr/bin/env python3
"""
Container-based Python Function Executor
基于容器的 Python 函数执行器，参考 ignis 的设计实现
"""

import sys
import json
import traceback
import importlib.util
import inspect
import threading
import queue
import zmq
from typing import Any, Dict, Callable, Optional
from pathlib import Path


class ContainerExecutor:
    """容器内的函数执行器，支持ZMQ通信"""
    
    def __init__(self, conn_name: str):
        self.conn_name = conn_name
        self.functions: Dict[str, Callable] = {}
        self.function_metadata: Dict[str, Dict] = {}
        self.send_queue = queue.Queue()
        self.context = None
        self.socket = None
    
    def register_function(self, name: str, func: Callable, metadata: Optional[Dict] = None):
        """注册函数到执行器"""
        self.functions[name] = func
        self.function_metadata[name] = metadata or {}
        
    def load_function_from_file(self, func_path: str, func_name: str):
        """从文件加载函数"""
        try:
            # 加载模块
            spec = importlib.util.spec_from_file_location("user_function", func_path)
            if spec is None or spec.loader is None:
                raise ImportError(f"Cannot load module from {func_path}")
            
            module = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(module)
            
            # 获取函数
            if not hasattr(module, func_name):
                raise AttributeError(f"Function '{func_name}' not found in {func_path}")
            
            func = getattr(module, func_name)
            if not callable(func):
                raise TypeError(f"'{func_name}' is not callable")
            
            # 获取函数签名信息
            sig = inspect.signature(func)
            metadata = {
                'parameters': list(sig.parameters.keys()),
                'source_file': func_path,
                'return_annotation': str(sig.return_annotation) if sig.return_annotation != inspect.Signature.empty else None
            }
            
            self.register_function(func_name, func, metadata)
            return True
            
        except Exception as e:
            print(f"Error loading function {func_name} from {func_path}: {e}", file=sys.stderr)
            return False
    
    def execute_function(self, func_name: str, args: Dict[str, Any]) -> Dict[str, Any]:
        """执行指定的函数"""
        try:
            if func_name not in self.functions:
                return {
                    'success': False,
                    'error': f"Function '{func_name}' not found",
                    'error_type': 'FunctionNotFound'
                }
            
            func = self.functions[func_name]
            
            # 检查参数
            sig = inspect.signature(func)
            bound_args = sig.bind(**args)
            bound_args.apply_defaults()
            
            # 执行函数
            result = func(**bound_args.arguments)
            
            return {
                'success': True,
                'result': result,
                'function_name': func_name
            }
            
        except TypeError as e:
            return {
                'success': False,
                'error': f"Parameter error: {str(e)}",
                'error_type': 'ParameterError',
                'function_name': func_name
            }
        except Exception as e:
            return {
                'success': False,
                'error': str(e),
                'error_type': type(e).__name__,
                'traceback': traceback.format_exc(),
                'function_name': func_name
            }
    
    def list_functions(self) -> Dict[str, Dict]:
        """列出所有已注册的函数及其元数据"""
        return {
            name: {
                'metadata': self.function_metadata.get(name, {}),
                'signature': str(inspect.signature(func)) if callable(func) else None
            }
            for name, func in self.functions.items()
        }
    
    def _start_send_thread(self, socket):
        """发送消息的线程"""
        while True:
            try:
                msg = self.send_queue.get()
                if msg is None:  # 停止信号
                    break
                socket.send(msg)
                self.send_queue.task_done()
            except Exception as e:
                print(f"Send thread error: {e}", file=sys.stderr)
                
    def _handle_message(self, msg_data: bytes):
        """处理接收到的消息"""
        try:
            # 这里应该解析protobuf消息，暂时用JSON模拟
            msg = json.loads(msg_data.decode('utf-8'))
            
            if msg.get('type') == 'execute':
                func_name = msg.get('function')
                params = msg.get('params', {})
                corr_id = msg.get('corr_id')
                
                try:
                    result = self.execute_function(func_name, params)
                    response = {
                        'type': 'return',
                        'corr_id': corr_id,
                        'result': result,
                        'success': True
                    }
                except Exception as e:
                    response = {
                        'type': 'return',
                        'corr_id': corr_id,
                        'error': str(e),
                        'success': False
                    }
                
                # 发送响应
                response_data = json.dumps(response).encode('utf-8')
                self.send_queue.put(response_data)
                
        except Exception as e:
            print(f"Message handling error: {e}", file=sys.stderr)
    
    def serve(self, ipc_addr: str):
        """启动ZMQ服务"""
        self.context = zmq.Context()
        self.socket = self.context.socket(zmq.DEALER)
        self.socket.connect(ipc_addr)
        
        # 发送Ready消息
        ready_msg = {
            'type': 'ready',
            'conn': self.conn_name
        }
        ready_data = json.dumps(ready_msg).encode('utf-8')
        self.socket.send(ready_data)
        
        print(f"Connected to {ipc_addr}", file=sys.stderr)
        
        # 启动发送线程
        send_thread = threading.Thread(target=self._start_send_thread, args=(self.socket,))
        send_thread.start()
        
        try:
            # 主循环接收消息
            while True:
                msg_data = self.socket.recv()
                self._handle_message(msg_data)
        except KeyboardInterrupt:
            print("Executor stopping...", file=sys.stderr)
        except Exception as e:
            print(f"Executor error: {e}", file=sys.stderr)
        finally:
            # 停止发送线程
            self.send_queue.put(None)
            send_thread.join()
            self.socket.close()
            self.context.term()


def main():
    """主函数，处理命令行参数"""
    import argparse
    
    parser = argparse.ArgumentParser(description='Container Function Executor')
    parser.add_argument('--serve', action='store_true', help='Start ZMQ server mode')
    parser.add_argument('--remote', type=str, help='ZMQ remote address (for serve mode)')
    parser.add_argument('--conn', type=str, help='Connection name (for serve mode)')
    parser.add_argument('--load', nargs=2, metavar=('FILE', 'FUNC'), help='Load function from file')
    parser.add_argument('--execute', nargs='+', metavar=('FUNC', 'PARAMS'), help='Execute function')
    parser.add_argument('--list', action='store_true', help='List functions')
    
    args = parser.parse_args()
    
    if args.serve:
        if not args.remote or not args.conn:
            print("Error: --serve mode requires --remote and --conn arguments")
            sys.exit(1)
        
        executor = ContainerExecutor(args.conn)
        executor.serve(args.remote)
        return
    
    # 非服务模式，使用默认执行器
    executor = ContainerExecutor("default")
    
    if args.load:
        file_path, func_name = args.load
        try:
            executor.load_function_from_file(file_path, func_name)
            print(f"Successfully loaded function {func_name} from {file_path}")
        except Exception as e:
            print(f"Failed to load function: {e}")
            sys.exit(1)
    
    elif args.execute:
        if len(args.execute) < 1:
            print("Error: --execute requires function name")
            sys.exit(1)
        
        func_name = args.execute[0]
        params = {}
        
        if len(args.execute) > 1:
            try:
                params = json.loads(args.execute[1])
            except json.JSONDecodeError as e:
                print(f"Invalid JSON parameters: {e}")
                sys.exit(1)
        
        try:
            result = executor.execute_function(func_name, params)
            print(json.dumps({"result": result, "success": True}))
        except Exception as e:
            print(json.dumps({"error": str(e), "success": False}))
            sys.exit(1)
    
    elif args.list:
        functions = executor.list_functions()
        print(json.dumps({"functions": functions}))
    
    else:
        parser.print_help()
        sys.exit(1)


if __name__ == "__main__":
    main()