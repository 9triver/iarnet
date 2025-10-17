#!/usr/bin/env python3
"""
CloudPickle Function Handler
处理cloudpickle序列化的函数，将其转换为可执行的本地文件
"""

import os
import sys
import json
import base64
import pickle
import hashlib
import tempfile
import importlib.util
from pathlib import Path
from typing import Any, Dict, Optional, Callable, Tuple
import cloudpickle


class CloudPickleFunctionHandler:
    """CloudPickle函数处理器"""
    
    def __init__(self, storage_dir: str = None):
        """
        初始化函数处理器
        
        Args:
            storage_dir: 函数存储目录，默认为 /tmp/cloudpickle_functions
        """
        self.storage_dir = Path(storage_dir or "/tmp/cloudpickle_functions")
        self.storage_dir.mkdir(parents=True, exist_ok=True)
        
    def save_function(self, func_data: bytes, func_name: str, func_id: str = None) -> Tuple[str, str]:
        """
        保存cloudpickle序列化的函数到本地文件
        
        Args:
            func_data: cloudpickle序列化的函数数据
            func_name: 函数名称
            func_id: 函数ID，如果不提供则根据数据生成hash
            
        Returns:
            Tuple[str, str]: (函数文件路径, 函数ID)
        """
        if func_id is None:
            func_id = hashlib.md5(func_data).hexdigest()
            
        # 创建函数专用目录
        func_dir = self.storage_dir / func_id
        func_dir.mkdir(exist_ok=True)
        
        # 保存原始pickle数据
        pickle_file = func_dir / f"{func_name}.pkl"
        with open(pickle_file, 'wb') as f:
            f.write(func_data)
            
        # 反序列化函数并生成可执行的Python文件
        try:
            func = cloudpickle.loads(func_data)
            python_file = self._generate_executable_file(func, func_name, func_dir)
            
            # 保存函数元数据
            metadata = {
                "func_name": func_name,
                "func_id": func_id,
                "pickle_file": str(pickle_file),
                "python_file": str(python_file),
                "created_at": str(Path(pickle_file).stat().st_mtime)
            }
            
            metadata_file = func_dir / "metadata.json"
            with open(metadata_file, 'w') as f:
                json.dump(metadata, f, indent=2)
                
            return str(python_file), func_id
            
        except Exception as e:
            raise RuntimeError(f"Failed to process function {func_name}: {e}")
    
    def _generate_executable_file(self, func: Callable, func_name: str, func_dir: Path) -> str:
        """
        生成可执行的Python文件
        
        Args:
            func: 反序列化后的函数对象
            func_name: 函数名称
            func_dir: 函数目录
            
        Returns:
            str: 生成的Python文件路径
        """
        python_file = func_dir / f"{func_name}_executable.py"
        
        # 获取函数的源代码信息（如果可能）
        func_source = self._extract_function_info(func)
        
        # 生成可执行的Python脚本
        script_content = f'''#!/usr/bin/env python3
"""
Auto-generated executable script for cloudpickle function: {func_name}
Generated from cloudpickle serialized data
"""

import sys
import json
import pickle
import cloudpickle
from pathlib import Path

# 函数信息
FUNCTION_NAME = "{func_name}"
PICKLE_FILE = "{func_dir / f'{func_name}.pkl'}"

def load_function():
    """从pickle文件加载函数"""
    with open(PICKLE_FILE, 'rb') as f:
        return cloudpickle.load(f)

def main():
    """主执行函数"""
    import argparse
    
    parser = argparse.ArgumentParser(description=f'Execute cloudpickle function: {{FUNCTION_NAME}}')
    parser.add_argument('--params', type=str, help='JSON encoded parameters')
    parser.add_argument('--param-file', type=str, help='File containing JSON parameters')
    parser.add_argument('--output', type=str, help='Output file for results')
    
    args = parser.parse_args()
    
    # 加载函数
    try:
        func = load_function()
    except Exception as e:
        print(f"Error loading function: {{e}}", file=sys.stderr)
        sys.exit(1)
    
    # 解析参数
    params = {{}}
    if args.params:
        try:
            params = json.loads(args.params)
        except json.JSONDecodeError as e:
            print(f"Error parsing parameters: {{e}}", file=sys.stderr)
            sys.exit(1)
    elif args.param_file:
        try:
            with open(args.param_file, 'r') as f:
                params = json.load(f)
        except Exception as e:
            print(f"Error reading parameter file: {{e}}", file=sys.stderr)
            sys.exit(1)
    
    # 执行函数
    try:
        if isinstance(params, dict):
            result = func(**params)
        elif isinstance(params, list):
            result = func(*params)
        else:
            result = func(params)
            
        # 输出结果
        output_data = {{
            "success": True,
            "result": result,
            "function": FUNCTION_NAME
        }}
        
        if args.output:
            with open(args.output, 'w') as f:
                json.dump(output_data, f, indent=2, default=str)
        else:
            print(json.dumps(output_data, default=str))
            
    except Exception as e:
        error_data = {{
            "success": False,
            "error": str(e),
            "error_type": e.__class__.__name__,
            "function": FUNCTION_NAME
        }}
        
        if args.output:
            with open(args.output, 'w') as f:
                json.dump(error_data, f, indent=2)
        else:
            print(json.dumps(error_data), file=sys.stderr)
        
        sys.exit(1)

if __name__ == "__main__":
    main()
'''

        with open(python_file, 'w') as f:
            f.write(script_content)
            
        # 设置执行权限
        python_file.chmod(0o755)
        
        return str(python_file)
    
    def _extract_function_info(self, func: Callable) -> Dict[str, Any]:
        """
        提取函数信息
        
        Args:
            func: 函数对象
            
        Returns:
            Dict[str, Any]: 函数信息
        """
        import inspect
        
        info = {
            "name": getattr(func, '__name__', 'unknown'),
            "module": getattr(func, '__module__', 'unknown'),
            "doc": getattr(func, '__doc__', None),
            "annotations": getattr(func, '__annotations__', {}),
        }
        
        try:
            info["signature"] = str(inspect.signature(func))
        except (ValueError, TypeError):
            info["signature"] = "unknown"
            
        try:
            info["source_file"] = inspect.getfile(func)
        except (OSError, TypeError):
            info["source_file"] = "unknown"
            
        return info
    
    def load_function(self, func_id: str) -> Tuple[Callable, str]:
        """
        加载已保存的函数
        
        Args:
            func_id: 函数ID
            
        Returns:
            Tuple[Callable, str]: (函数对象, Python文件路径)
        """
        func_dir = self.storage_dir / func_id
        if not func_dir.exists():
            raise FileNotFoundError(f"Function {func_id} not found")
            
        metadata_file = func_dir / "metadata.json"
        if not metadata_file.exists():
            raise FileNotFoundError(f"Metadata for function {func_id} not found")
            
        with open(metadata_file, 'r') as f:
            metadata = json.load(f)
            
        pickle_file = metadata["pickle_file"]
        python_file = metadata["python_file"]
        
        with open(pickle_file, 'rb') as f:
            func = cloudpickle.load(f)
            
        return func, python_file
    
    def list_functions(self) -> Dict[str, Dict[str, Any]]:
        """
        列出所有已保存的函数
        
        Returns:
            Dict[str, Dict[str, Any]]: 函数ID到元数据的映射
        """
        functions = {}
        
        for func_dir in self.storage_dir.iterdir():
            if func_dir.is_dir():
                metadata_file = func_dir / "metadata.json"
                if metadata_file.exists():
                    try:
                        with open(metadata_file, 'r') as f:
                            metadata = json.load(f)
                        functions[func_dir.name] = metadata
                    except Exception:
                        continue
                        
        return functions
    
    def remove_function(self, func_id: str) -> bool:
        """
        删除已保存的函数
        
        Args:
            func_id: 函数ID
            
        Returns:
            bool: 是否成功删除
        """
        func_dir = self.storage_dir / func_id
        if func_dir.exists():
            import shutil
            shutil.rmtree(func_dir)
            return True
        return False


def main():
    """命令行工具主函数"""
    import argparse
    
    parser = argparse.ArgumentParser(description='CloudPickle Function Handler')
    parser.add_argument('--storage-dir', type=str, help='Function storage directory')
    
    subparsers = parser.add_subparsers(dest='command', help='Available commands')
    
    # save命令
    save_parser = subparsers.add_parser('save', help='Save a cloudpickle function')
    save_parser.add_argument('--data', type=str, required=True, help='Base64 encoded cloudpickle data')
    save_parser.add_argument('--name', type=str, required=True, help='Function name')
    save_parser.add_argument('--id', type=str, help='Function ID (optional)')
    
    # load命令
    load_parser = subparsers.add_parser('load', help='Load a saved function')
    load_parser.add_argument('--id', type=str, required=True, help='Function ID')
    
    # list命令
    list_parser = subparsers.add_parser('list', help='List all saved functions')
    
    # remove命令
    remove_parser = subparsers.add_parser('remove', help='Remove a saved function')
    remove_parser.add_argument('--id', type=str, required=True, help='Function ID')
    
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        return
        
    handler = CloudPickleFunctionHandler(args.storage_dir)
    
    if args.command == 'save':
        try:
            func_data = base64.b64decode(args.data)
            python_file, func_id = handler.save_function(func_data, args.name, args.id)
            print(json.dumps({
                "success": True,
                "function_id": func_id,
                "python_file": python_file
            }))
        except Exception as e:
            print(json.dumps({
                "success": False,
                "error": str(e)
            }), file=sys.stderr)
            sys.exit(1)
            
    elif args.command == 'load':
        try:
            func, python_file = handler.load_function(args.id)
            print(json.dumps({
                "success": True,
                "function_name": getattr(func, '__name__', 'unknown'),
                "python_file": python_file
            }))
        except Exception as e:
            print(json.dumps({
                "success": False,
                "error": str(e)
            }), file=sys.stderr)
            sys.exit(1)
            
    elif args.command == 'list':
        functions = handler.list_functions()
        print(json.dumps({
            "success": True,
            "functions": functions
        }, indent=2))
        
    elif args.command == 'remove':
        success = handler.remove_function(args.id)
        print(json.dumps({
            "success": success
        }))


if __name__ == "__main__":
    main()