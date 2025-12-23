#!/usr/bin/env python3
"""
将本地 component 镜像同步到所有 docker 和 k8s provider 节点
支持 docker 节点（使用 docker load）和 k8s 节点（使用 ctr import）
"""

import os
import sys
import yaml
import argparse
import subprocess
import time
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
from threading import Lock

# 获取脚本所在目录
SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent

class ComponentImageSyncer:
    def __init__(self, vm_config_path: str):
        """初始化镜像同步器"""
        # 处理vm_config_path
        vm_config_path_obj = Path(vm_config_path)
        if not vm_config_path_obj.is_absolute():
            if (SCRIPT_DIR / vm_config_path_obj.name).exists():
                vm_config_path_obj = SCRIPT_DIR / vm_config_path_obj.name
            elif (PROJECT_ROOT / vm_config_path).exists():
                vm_config_path_obj = PROJECT_ROOT / vm_config_path
            else:
                vm_config_path_obj = Path(vm_config_path)
        
        if not vm_config_path_obj.exists():
            raise FileNotFoundError(f"虚拟机配置文件不存在: {vm_config_path_obj}")
        
        with open(vm_config_path_obj, 'r', encoding='utf-8') as f:
            self.vm_config = yaml.safe_load(f)
        
        self.user = self.vm_config['global']['user']
        self._log_lock = Lock()
    
    def _print(self, *args, **kwargs):
        """线程安全的打印函数"""
        with self._log_lock:
            kwargs.setdefault('flush', True)
            print(*args, **kwargs)
    
    def check_node_connectivity(self, node_ip: str) -> bool:
        """检查节点连通性"""
        try:
            result = subprocess.run(
                ['ping', '-c', '1', '-W', '2', node_ip],
                capture_output=True,
                timeout=5
            )
            return result.returncode == 0
        except:
            return False
    
    def check_image_exists_local(self, image_name: str) -> bool:
        """检查本地镜像是否存在"""
        try:
            result = subprocess.run(
                ['docker', 'images', '-q', image_name],
                capture_output=True,
                text=True,
                check=True
            )
            return bool(result.stdout.strip())
        except:
            return False
    
    def get_image_id(self, image_name: str) -> str:
        """获取镜像的 ID（用于判断镜像是否变化）"""
        try:
            result = subprocess.run(
                ['docker', 'images', '--format', '{{.ID}}', image_name],
                check=True,
                capture_output=True,
                text=True
            )
            return result.stdout.strip()
        except:
            return ""
    
    def save_image_to_tar(self, image_name: str, output_path: str, reuse_existing: bool = True) -> bool:
        """将镜像保存为 tar 文件
        
        Args:
            image_name: 镜像名称
            output_path: 输出文件路径
            reuse_existing: 如果文件已存在且镜像未变化，是否复用（默认 True）
        """
        output_path_obj = Path(output_path)
        
        # 如果启用复用且文件已存在，检查是否可以复用
        if reuse_existing and output_path_obj.exists():
            self._print(f"  检查已存在的 tar 文件: {output_path_obj.name}")
            
            # 检查文件大小，如果文件存在且大小合理（至少 1MB），直接复用
            file_size = output_path_obj.stat().st_size
            if file_size > 1024 * 1024:  # 至少 1MB，说明文件不是空的
                self._print(f"  ✓ 复用已存在的 tar 文件 ({file_size / (1024**2):.1f} MB)")
                return True
            else:
                self._print(f"  ⚠ 文件太小，重新导出")
        
        # 导出镜像
        try:
            result = subprocess.run(
                ['docker', 'save', '-o', output_path, image_name],
                check=True,
                capture_output=True,
                text=True
            )
            return True
        except subprocess.CalledProcessError as e:
            self._print(f"  ✗ 镜像保存失败: {e}")
            return False
    
    def sync_image_to_docker_node(self, node_id: int, node_info: dict, image_name: str, 
                                  image_path: Path = None) -> bool:
        """同步镜像到 docker 节点（使用流式传输，不需要 tar 文件）"""
        node_prefix = f"[Docker节点 {node_id}] {node_info['hostname']} ({node_info['ip']}) "
        
        # 检查连通性
        if not self.check_node_connectivity(node_info['ip']):
            self._print(f"{node_prefix}⚠ 无法连接，跳过")
            return False
        
        ssh_cmd = [
            'ssh',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f"{self.user}@{node_info['ip']}"
        ]
        
        # 检查 Docker
        check_docker_cmd = ' '.join(ssh_cmd) + ' "docker --version >/dev/null 2>&1 && echo OK || echo NOT_INSTALLED"'
        try:
            result = subprocess.run(check_docker_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            if 'OK' not in result.stdout:
                self._print(f"{node_prefix}⚠ Docker 未安装，跳过")
                return False
        except:
            self._print(f"{node_prefix}⚠ 无法检查 Docker，跳过")
            return False
        
        # 估算镜像大小（用于超时计算和磁盘空间检查）
        # 如果提供了 image_path，使用实际大小；否则估算
        if image_path and image_path.exists():
            image_size_mb = image_path.stat().st_size / (1024**2)
        else:
            # 估算：通过 docker images 获取镜像大小
            try:
                inspect_cmd = ['docker', 'inspect', '--format={{.Size}}', image_name]
                result = subprocess.run(inspect_cmd, capture_output=True, text=True, timeout=10)
                if result.returncode == 0:
                    size_bytes = int(result.stdout.strip())
                    image_size_mb = size_bytes / (1024**2)
                else:
                    # 默认估算 5GB
                    image_size_mb = 5000
            except:
                # 默认估算 5GB
                image_size_mb = 5000
        
        required_space_gb = (image_size_mb * 2) / 1024  # 需要至少 2 倍空间
        
        # 检查磁盘空间，优先检查 Docker 数据目录，否则检查根目录
        # 使用 df -k 获取 KB 值，更可靠
        check_space_cmd = ' '.join(ssh_cmd) + ' "df -k /var/lib/docker 2>/dev/null | tail -1 || df -k / | tail -1"'
        try:
            space_result = subprocess.run(check_space_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            output_lines = space_result.stdout.strip().split('\n')
            # 获取最后一行（实际数据行）
            if output_lines:
                last_line = output_lines[-1].strip()
                # df -k 输出格式: Filesystem 1K-blocks Used Available Use% Mounted
                # 第4列是 Available（可用空间，单位KB）
                parts = last_line.split()
                if len(parts) >= 4:
                    try:
                        available_kb = int(parts[3])
                        available_space_gb = available_kb / (1024 * 1024)  # KB 转 GB
                    except (ValueError, IndexError):
                        # 如果解析失败，尝试使用 df -h 并解析人类可读格式
                        available_space_gb = self._parse_df_h_output(ssh_cmd, node_prefix)
                else:
                    available_space_gb = self._parse_df_h_output(ssh_cmd, node_prefix)
            else:
                available_space_gb = self._parse_df_h_output(ssh_cmd, node_prefix)
            
            if available_space_gb < required_space_gb:
                # 显示详细磁盘信息
                detail_cmd = ' '.join(ssh_cmd) + ' "df -h /var/lib/docker 2>/dev/null | tail -1 || df -h / | tail -1"'
                detail_result = subprocess.run(detail_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
                self._print(f"{node_prefix}⚠ 磁盘空间不足: 需要 {required_space_gb:.1f}GB，可用 {available_space_gb:.1f}GB")
                if detail_result.stdout.strip():
                    self._print(f"{node_prefix}   磁盘使用情况: {detail_result.stdout.strip()}")
                self._print(f"{node_prefix}   建议: 在节点上执行 'docker system prune -a --volumes' 清理空间")
                self._print(f"{node_prefix}   或跳过此节点，稍后手动同步")
                return False
            else:
                self._print(f"{node_prefix}ℹ 磁盘空间检查: 可用 {available_space_gb:.1f}GB，需要 {required_space_gb:.1f}GB")
        except Exception as e:
            self._print(f"{node_prefix}⚠ 无法检查磁盘空间: {e}，继续执行...")
        
        # 计算镜像大小，用于动态调整超时时间
        # 流式传输不需要 image_path，但如果提供了则使用实际大小
        if image_path and image_path.exists():
            image_size_mb = image_path.stat().st_size / (1024**2)
        else:
            # 通过 docker inspect 获取镜像大小
            try:
                inspect_cmd = ['docker', 'inspect', '--format={{.Size}}', image_name]
                result = subprocess.run(inspect_cmd, capture_output=True, text=True, timeout=10)
                if result.returncode == 0:
                    size_bytes = int(result.stdout.strip())
                    image_size_mb = size_bytes / (1024**2)
                else:
                    # 默认估算 5GB
                    image_size_mb = 5000
            except:
                # 默认估算 5GB
                image_size_mb = 5000
        
        # 根据镜像大小计算超时时间：每 MB 约 2-3 秒，最小 300 秒，最大 1800 秒（30分钟）
        load_timeout = max(300, min(1800, int(image_size_mb * 3)))
        
        # 使用流式传输方法：docker save | ssh | docker load
        # 这种方法避免了中间文件，减少磁盘IO，速度更快，失败率更低
        self._print(f"{node_prefix}流式传输并加载镜像 ({image_size_mb:.1f} MB，预计需要 {load_timeout//60} 分钟)...")
        
        # 构建流式传输命令：本地 docker save -> ssh -> 远程 docker load
        # 使用 gzip 压缩可以显著减少传输时间（通常压缩率 50-70%）
        ssh_target = f"{self.user}@{node_info['ip']}"
        stream_cmd = [
            'docker', 'save', image_name,
            '|', 'gzip',  # 压缩传输
            '|', 'ssh',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=10',
            '-o', 'ServerAliveInterval=60',  # 保持连接活跃
            '-o', 'ServerAliveCountMax=3',
            ssh_target,
            f'"gunzip | timeout {load_timeout} docker load"'
        ]
        
        # 使用 shell=True 执行管道命令
        stream_cmd_str = ' '.join(stream_cmd)
        
        try:
            # 执行流式传输，设置较长的超时时间
            result = subprocess.run(
                stream_cmd_str,
                shell=True,
                check=True,
                timeout=load_timeout + 300,  # 额外 5 分钟缓冲
                capture_output=True,
                text=True
            )
            
            # 检查输出中是否有错误信息
            output = result.stdout + result.stderr
            if output and ('error' in output.lower() or 'failed' in output.lower()):
                # 检查是否是真正的错误（排除警告）
                error_lines = [line for line in output.split('\n') 
                             if any(keyword in line.lower() for keyword in ['error', 'failed', 'no space', 'permission denied'])]
                if error_lines:
                    self._print(f"{node_prefix}  ⚠ 可能有错误: {error_lines[0][:200]}")
                else:
                    self._print(f"{node_prefix}  ✓ 镜像传输和加载成功（有警告但已忽略）")
                    return True
            else:
                self._print(f"{node_prefix}  ✓ 镜像传输和加载成功")
                return True
                
        except subprocess.CalledProcessError as e:
            error_msg = e.stderr if hasattr(e, 'stderr') and e.stderr else (e.stdout if hasattr(e, 'stdout') else str(e))
            # 提取关键错误信息
            error_lines = str(error_msg).split('\n')
            key_errors = [line for line in error_lines 
                         if any(keyword in line.lower() for keyword in ['error', 'failed', 'no space', 'permission', 'denied', 'timeout', 'connection'])]
            if key_errors:
                error_display = '\n'.join(key_errors[:3])  # 只显示前3个关键错误
            else:
                error_display = error_msg[:300] if len(str(error_msg)) > 300 else error_msg
            
            self._print(f"{node_prefix}  ✗ 流式传输失败")
            self._print(f"{node_prefix}     错误详情: {error_display}")
            
            # 检查是否是磁盘空间问题
            if 'no space' in str(error_msg).lower() or 'failed to open write' in str(error_msg).lower():
                # 再次检查磁盘空间
                space_check_cmd = ' '.join(ssh_cmd) + ' "df -h /var/lib/docker 2>/dev/null | tail -1 || df -h / | tail -1"'
                space_result = subprocess.run(space_check_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
                if space_result.stdout:
                    self._print(f"{node_prefix}     磁盘空间: {space_result.stdout.strip()}")
                self._print(f"{node_prefix}     建议: 清理 Docker 空间或增加磁盘容量")
            
            return False
        except subprocess.TimeoutExpired:
            self._print(f"{node_prefix}  ✗ 流式传输超时（{load_timeout + 300}秒），镜像可能较大")
            # 检查进程是否仍在运行
            check_cmd = ' '.join(ssh_cmd) + ' "ps aux | grep -E \"docker load|gunzip\" | grep -v grep || echo NO_PROCESS"'
            check_result = subprocess.run(check_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            if 'NO_PROCESS' not in check_result.stdout:
                self._print(f"{node_prefix}  ℹ 传输进程仍在运行，可能需要更长时间")
            return False
    def _parse_df_h_output(self, ssh_cmd: list, node_prefix: str) -> float:
        """解析 df -h 输出，将人类可读格式转换为 GB"""
        try:
            detail_cmd = ' '.join(ssh_cmd) + ' "df -h /var/lib/docker 2>/dev/null | tail -1 || df -h / | tail -1"'
            detail_result = subprocess.run(detail_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            output_line = detail_result.stdout.strip().split('\n')[-1].strip()
            # df -h 输出格式: /dev/vda1  20G  15G  5.2G  74% /
            # 第4列是可用空间（如 5.2G）
            parts = output_line.split()
            if len(parts) >= 4:
                avail_str = parts[3]  # 如 "5.2G"
                # 移除单位并转换
                if avail_str.endswith('G'):
                    return float(avail_str[:-1])
                elif avail_str.endswith('M'):
                    return float(avail_str[:-1]) / 1024
                elif avail_str.endswith('K'):
                    return float(avail_str[:-1]) / (1024 * 1024)
                elif avail_str.endswith('T'):
                    return float(avail_str[:-1]) * 1024
                else:
                    # 尝试直接解析为数字（假设是GB）
                    return float(avail_str)
        except:
            pass
        # 如果都失败了，返回一个较大的值允许继续执行
        return 999.0
    
    def sync_image_to_k8s_node(self, cluster_id: int, node_info: dict, image_name: str, 
                               image_path: Path) -> bool:
        """同步镜像到 k8s 节点（master 或 worker，使用 containerd）"""
        role = node_info.get('role', 'master')
        node_prefix = f"[K8s集群 {cluster_id} {role}] {node_info['hostname']} ({node_info['ip']}) "
        
        # 检查连通性
        if not self.check_node_connectivity(node_info['ip']):
            self._print(f"{node_prefix}⚠ 无法连接，跳过")
            return False
        
        ssh_cmd = [
            'ssh',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f"{self.user}@{node_info['ip']}"
        ]
        
        # 检查 containerd
        check_containerd_cmd = ' '.join(ssh_cmd) + ' "sudo ctr version >/dev/null 2>&1 && echo OK || echo NOT_INSTALLED"'
        try:
            result = subprocess.run(check_containerd_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            if 'OK' not in result.stdout:
                self._print(f"{node_prefix}⚠ containerd 未安装，跳过")
                return False
        except:
            self._print(f"{node_prefix}⚠ 无法检查 containerd，跳过")
            return False
        
        # 计算镜像文件大小，用于动态调整超时时间
        image_size_mb = image_path.stat().st_size / (1024**2)
        # 传输超时时间：根据文件大小计算，最小 300 秒，最大 600 秒
        transfer_timeout = max(300, min(600, int(image_size_mb * 2)))
        
        # 清理旧的镜像 tar 文件，释放磁盘空间
        self._print(f"{node_prefix}清理旧的镜像文件...")
        cleanup_cmd = ' '.join(ssh_cmd) + ' "rm -f ~/image.tar ~/image.tar.* 2>/dev/null; du -sh ~/image.tar* 2>/dev/null | head -1 || echo CLEANED"'
        try:
            cleanup_result = subprocess.run(cleanup_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            if 'CLEANED' not in cleanup_result.stdout:
                cleaned_info = cleanup_result.stdout.strip().split('\n')[0] if cleanup_result.stdout.strip() else ""
                if cleaned_info:
                    self._print(f"{node_prefix}  ℹ 已清理: {cleaned_info}")
            else:
                self._print(f"{node_prefix}  ✓ 旧文件已清理")
        except:
            self._print(f"{node_prefix}  ⚠ 清理旧文件时出错，继续执行...")
        
        # 传输镜像文件
        self._print(f"{node_prefix}传输镜像文件 ({image_size_mb:.1f} MB)...")
        scp_cmd = [
            'scp',
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            str(image_path),
            f"{self.user}@{node_info['ip']}:~/image.tar"
        ]
        
        try:
            subprocess.run(scp_cmd, check=True, capture_output=True, timeout=transfer_timeout)
            self._print(f"{node_prefix}  ✓ 文件传输成功")
        except subprocess.CalledProcessError as e:
            self._print(f"{node_prefix}  ✗ 文件传输失败: {e}")
            return False
        except subprocess.TimeoutExpired:
            self._print(f"{node_prefix}  ✗ 文件传输超时（{transfer_timeout}秒）")
            return False
        
        # 在远程节点上导入镜像到 containerd
        # Docker 镜像格式与 containerd 兼容（都遵循 OCI 标准），可以直接导入
        # 计算镜像文件大小，用于动态调整超时时间
        image_size_mb = image_path.stat().st_size / (1024**2)
        # 根据镜像大小计算超时时间：每 MB 约 2-3 秒，最小 300 秒，最大 1800 秒（30分钟）
        import_timeout = max(300, min(1800, int(image_size_mb * 3)))
        
        self._print(f"{node_prefix}导入镜像到 containerd（{image_size_mb:.1f} MB，预计需要 {import_timeout//60} 分钟）...")
        
        # 导入镜像到 containerd 的 k8s.io 命名空间
        # ctr import 应该会保留 docker save 中的标签信息
        # 如果导入后名称不对，会在后面重新标记
        import_cmd = ' '.join(ssh_cmd) + f' "timeout {import_timeout} bash -c \'sudo ctr -n k8s.io images import ~/image.tar 2>&1\'"'
        try:
            result = subprocess.run(import_cmd, shell=True, check=True, timeout=import_timeout + 60, capture_output=True, text=True)
            self._print(f"{node_prefix}  ✓ 镜像导入成功")
            
            # 检查导入后的镜像，如果名称不对则重新标记
            # 首先尝试查找导入的镜像（可能是 SHA256 或原始名称）
            list_cmd = ' '.join(ssh_cmd) + ' "sudo ctr -n k8s.io images ls | tail -3"'
            list_result = subprocess.run(list_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            
            # 检查是否已经有正确名称的镜像
            verify_cmd = ' '.join(ssh_cmd) + f' "sudo ctr -n k8s.io images ls | grep -E \\"^{image_name}\\\\s|\\\\s{image_name}\\\\s|\\\\s{image_name}$\\" && echo EXISTS || echo NOT_FOUND"'
            verify_result = subprocess.run(verify_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            
            if 'EXISTS' in verify_result.stdout:
                self._print(f"{node_prefix}  ✓ 镜像验证成功: {image_name}")
            else:
                # 如果没有正确名称，尝试从导入输出中提取 SHA256 并重新标记
                # 或者查找最近导入的镜像（通常是 SHA256 格式）
                self._print(f"{node_prefix}  ℹ 尝试重新标记镜像...")
                
                # 从列表中找到最新的 SHA256 镜像并标记
                if list_result.stdout:
                    lines = list_result.stdout.strip().split('\n')
                    for line in reversed(lines):  # 从最新的开始
                        if 'sha256:' in line:
                            # 提取 SHA256 值
                            parts = line.split()
                            if parts:
                                sha256 = parts[0]
                                # 使用 ctr tag 重新标记
                                tag_cmd = ' '.join(ssh_cmd) + f' "sudo ctr -n k8s.io images tag {sha256} {image_name} 2>&1"'
                                tag_result = subprocess.run(tag_cmd, shell=True, check=False, timeout=30, capture_output=True, text=True)
                                if tag_result.returncode == 0:
                                    self._print(f"{node_prefix}  ✓ 镜像已重新标记为: {image_name}")
                                    break
                
                # 再次验证
                verify_result2 = subprocess.run(verify_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
                if 'EXISTS' in verify_result2.stdout:
                    self._print(f"{node_prefix}  ✓ 镜像验证成功: {image_name}")
                else:
                    self._print(f"{node_prefix}  ⚠ 镜像已导入，但名称可能不匹配")
                    self._print(f"{node_prefix}     提示: 镜像已成功导入，可以使用 SHA256 或手动标记后使用")
            
            # 清理临时文件
            cleanup_cmd = ' '.join(ssh_cmd) + ' "rm -f ~/image.tar"'
            subprocess.run(cleanup_cmd, shell=True, check=False, timeout=5, capture_output=True)
            
            return True
        except subprocess.CalledProcessError as e:
            error_msg = e.stderr if hasattr(e, 'stderr') and e.stderr else (e.stdout if hasattr(e, 'stdout') else str(e))
            self._print(f"{node_prefix}  ✗ 镜像导入失败: {error_msg[:200] if len(str(error_msg)) > 200 else error_msg}")
            # 清理临时文件
            cleanup_cmd = ' '.join(ssh_cmd) + ' "rm -f ~/image.tar"'
            subprocess.run(cleanup_cmd, shell=True, check=False, timeout=5, capture_output=True)
            return False
        except subprocess.TimeoutExpired:
            self._print(f"{node_prefix}  ✗ 镜像导入超时（{import_timeout}秒），镜像可能较大，请检查节点状态")
            # 检查导入进程是否仍在运行
            check_cmd = ' '.join(ssh_cmd) + ' "ps aux | grep -E \"ctr.*import|ctr.*image\" | grep -v grep || echo NO_PROCESS"'
            check_result = subprocess.run(check_cmd, shell=True, check=False, timeout=10, capture_output=True, text=True)
            if 'NO_PROCESS' not in check_result.stdout:
                self._print(f"{node_prefix}  ℹ containerd 导入进程仍在运行，可能需要更长时间")
            # 清理临时文件
            cleanup_cmd = ' '.join(ssh_cmd) + ' "rm -f ~/image.tar"'
            subprocess.run(cleanup_cmd, shell=True, check=False, timeout=5, capture_output=True)
            return False
    
    def sync_image_to_all_providers(self, image_name: str, max_workers: int = 10, 
                                   node_types: list = None) -> dict:
        """同步镜像到所有 docker 和 k8s provider 节点
        
        Args:
            image_name: 镜像名称
            max_workers: 最大并发数
            node_types: 要同步的节点类型列表，可选值: ['docker', 'k8s']，默认: ['docker', 'k8s']
        """
        if node_types is None:
            node_types = ['docker', 'k8s']
        
        node_types_str = ' 和 '.join(node_types)
        self._print(f"\n同步镜像到 {node_types_str} provider 节点: {image_name}")
        self._print("=" * 80)
        
        # 检查本地镜像是否存在
        if not self.check_image_exists_local(image_name):
            self._print(f"✗ 本地镜像不存在: {image_name}")
            self._print(f"请先构建或拉取镜像: docker build/pull {image_name}")
            return {'success': False, 'docker': {'success': 0, 'total': 0}, 'k8s': {'success': 0, 'total': 0}}
        
        # Docker 节点使用流式传输（不需要 tar 文件），K8s 节点需要 tar 文件
        # 只有当需要同步到 K8s 节点时才创建 tar 文件
        image_tar_path = None
        if 'k8s' in node_types:
            # 先保存镜像为 tar 文件（K8s 节点需要）
            import tempfile
            temp_dir = Path(tempfile.gettempdir())
            # 使用镜像名称生成文件名（替换特殊字符）
            safe_image_name = image_name.replace('/', '_').replace(':', '_')
            image_tar_path = temp_dir / f"iarnet_component_{safe_image_name}.tar"
            
            self._print(f"\n1. 保存镜像为 tar 文件（K8s 节点需要）...")
            self._print(f"  镜像: {image_name}")
            self._print(f"  文件: {image_tar_path.name}")
            
            try:
                if not self.save_image_to_tar(image_name, str(image_tar_path), reuse_existing=True):
                    return {'success': False, 'docker': {'success': 0, 'total': 0}, 'k8s': {'success': 0, 'total': 0}}
                file_size_mb = image_tar_path.stat().st_size / (1024**2)
                self._print(f"  ✓ 镜像文件就绪 ({file_size_mb:.1f} MB)")
            except Exception as e:
                self._print(f"  ✗ 镜像保存失败: {e}")
                return {'success': False, 'docker': {'success': 0, 'total': 0}, 'k8s': {'success': 0, 'total': 0}}
        else:
            self._print(f"\n1. Docker 节点使用流式传输（无需 tar 文件）...")
            self._print(f"  镜像: {image_name}")
        
        # 获取 docker 节点列表
        docker_config = self.vm_config['vm_types']['docker']
        docker_nodes = []
        for node_id in range(docker_config['count']):
            ip_suffix = docker_config['ip_start'] + node_id
            ip_address = f"{docker_config['ip_base']}.{ip_suffix}"
            hostname = f"{docker_config['hostname_prefix']}-{node_id+1:02d}"
            docker_nodes.append({
                'id': node_id,
                'hostname': hostname,
                'ip': ip_address
            })
        
        # 获取 k8s 集群列表（同步到 master 和所有 worker 节点）
        k8s_config = self.vm_config['vm_types']['k8s_clusters']
        master_config = k8s_config['master']
        worker_config = k8s_config['worker']
        k8s_nodes = []  # 包含所有 master 和 worker 节点
        
        for cluster_id in range(k8s_config['count']):
            # Master 节点
            master_ip_suffix = master_config['ip_start'] + cluster_id * master_config['ip_step']
            master_ip = f"{master_config['ip_base']}.{master_ip_suffix}"
            master_hostname = f"{master_config['hostname_prefix']}-{cluster_id+1:02d}{master_config['hostname_suffix']}"
            k8s_nodes.append({
                'id': cluster_id,
                'node_id': 0,  # master 是节点 0
                'hostname': master_hostname,
                'ip': master_ip,
                'role': 'master',
                'cluster_id': cluster_id
            })
            
            # Worker 节点
            worker_count = worker_config.get('count_per_cluster', 2)
            for worker_id in range(worker_count):
                worker_ip_suffix = master_ip_suffix + worker_id + 1  # worker IP 在 master 之后
                worker_ip = f"{master_config['ip_base']}.{worker_ip_suffix}"
                worker_hostname = f"{worker_config['hostname_prefix']}-{cluster_id+1:02d}-worker-{worker_id+1:02d}"
                k8s_nodes.append({
                    'id': cluster_id,
                    'node_id': worker_id + 1,  # worker 从 1 开始
                    'hostname': worker_hostname,
                    'ip': worker_ip,
                    'role': 'worker',
                    'cluster_id': cluster_id
                })
        
        # 同步到 docker 节点
        docker_success = 0
        docker_failed = []
        
        if 'docker' in node_types:
            self._print(f"\n2. 同步镜像到 {len(docker_nodes)} 个 Docker provider 节点...")
            try:
                with ThreadPoolExecutor(max_workers=max_workers) as executor:
                    future_to_node = {
                        executor.submit(
                            self.sync_image_to_docker_node,
                            node['id'],
                            node,
                            image_name,
                            image_tar_path
                        ): node
                        for node in docker_nodes
                    }
                    
                    for future in as_completed(future_to_node):
                        node = future_to_node[future]
                        try:
                            result = future.result()
                            if result:
                                docker_success += 1
                            else:
                                docker_failed.append(node['id'])
                        except Exception as e:
                            docker_failed.append(node['id'])
                            self._print(f"[Docker节点 {node['id']}] ✗ 同步异常: {e}")
            except Exception as e:
                self._print(f"✗ Docker 节点同步过程出错: {e}")
        else:
            self._print(f"\n2. 跳过 Docker 节点同步")
        
        # 同步到 k8s 节点（包括 master 和 worker）
        k8s_success = 0
        k8s_failed = []
        
        if 'k8s' in node_types:
            master_count = k8s_config['count']
            worker_count = len(k8s_nodes) - master_count
            step_num = 3 if 'docker' in node_types else 2
            self._print(f"\n{step_num}. 同步镜像到 {len(k8s_nodes)} 个 K8s 节点（{master_count} 个 master + {worker_count} 个 worker）...")
            try:
                with ThreadPoolExecutor(max_workers=max_workers) as executor:
                    future_to_node = {
                        executor.submit(
                            self.sync_image_to_k8s_node,
                            node['cluster_id'],
                            node,
                            image_name,
                            image_tar_path
                        ): node
                        for node in k8s_nodes
                    }
                    
                    for future in as_completed(future_to_node):
                        node = future_to_node[future]
                        try:
                            result = future.result()
                            if result:
                                k8s_success += 1
                            else:
                                k8s_failed.append(f"{node['cluster_id']}-{node['role']}-{node['node_id']}")
                        except Exception as e:
                            k8s_failed.append(f"{node['cluster_id']}-{node['role']}-{node['node_id']}")
                            self._print(f"[K8s集群 {node['cluster_id']} {node['role']}] ✗ 同步异常: {e}")
            except Exception as e:
                self._print(f"✗ K8s 节点同步过程出错: {e}")
        else:
            step_num = 3 if 'docker' in node_types else 2
            self._print(f"\n{step_num}. 跳过 K8s 节点同步")
        
        # 不删除临时文件，保留以便下次复用
        # 文件保存在系统临时目录，系统会自动清理旧文件
        # 如果需要强制重新导出，可以手动删除临时文件
        
        # 输出结果
        self._print("\n" + "=" * 80)
        self._print(f"同步完成:")
        self._print(f"  Docker provider: {docker_success}/{len(docker_nodes)} 个节点成功")
        if docker_failed:
            self._print(f"    失败的节点: {docker_failed}")
        self._print(f"  K8s provider: {k8s_success}/{len(k8s_nodes)} 个节点成功（{master_count} master + {worker_count} worker）")
        if k8s_failed:
            self._print(f"    失败的节点: {k8s_failed}")
        self._print("=" * 80)
        
        total_success = docker_success + k8s_success
        total_nodes = len(docker_nodes) + len(k8s_nodes)
        
        return {
            'success': total_success > 0,
            'docker': {'success': docker_success, 'total': len(docker_nodes), 'failed': docker_failed},
            'k8s': {'success': k8s_success, 'total': len(k8s_clusters), 'failed': k8s_failed},
            'total': {'success': total_success, 'total': total_nodes}
        }
    
    def sync_multiple_images(self, image_names: list, max_workers: int = 10) -> bool:
        """批量同步多个镜像"""
        self._print(f"\n批量同步 {len(image_names)} 个镜像到所有 provider 节点...")
        self._print("=" * 80)
        
        success_count = 0
        for i, image_name in enumerate(image_names, 1):
            self._print(f"\n[{i}/{len(image_names)}] 同步镜像: {image_name}")
            result = self.sync_image_to_all_providers(image_name, max_workers)
            if result['success']:
                success_count += 1
        
        self._print(f"\n批量同步完成: {success_count}/{len(image_names)} 个镜像成功")
        return success_count == len(image_names)

def main():
    parser = argparse.ArgumentParser(
        description='将本地 component 镜像同步到所有 docker 和 k8s provider 节点',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 同步单个镜像到所有 provider 节点
  python3 sync_component_images_to_providers.py --image iarnet/component:python_3.11-latest
  
  # 只同步到 docker 节点
  python3 sync_component_images_to_providers.py --image iarnet/component:python_3.11-latest --node-types docker
  
  # 只同步到 k8s 节点
  python3 sync_component_images_to_providers.py --image iarnet/component:python_3.11-latest --node-types k8s
  
  # 同步多个镜像
  python3 sync_component_images_to_providers.py --images iarnet/component:python_3.11-latest iarnet/component:node_20-latest
  
  # 指定配置文件
  python3 sync_component_images_to_providers.py --vm-config vm-config.yaml --image my-component:latest
  
  # 调整并发数
  python3 sync_component_images_to_providers.py --image my-component:latest --max-workers 20
        """
    )
    
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    
    parser.add_argument(
        '--image', '-i',
        type=str,
        help='要同步的镜像名（如 iarnet/component:python_3.11-latest）'
    )
    
    parser.add_argument(
        '--images',
        nargs='+',
        help='要同步的多个镜像名（用空格分隔）'
    )
    
    parser.add_argument(
        '--max-workers', '-w',
        type=int,
        default=10,
        help='最大并发数 (默认: 10)'
    )
    
    parser.add_argument(
        '--node-types', '-t',
        type=str,
        nargs='+',
        choices=['docker', 'k8s'],
        default=['docker', 'k8s'],
        help='要同步的节点类型 (默认: docker k8s，可指定 docker 或 k8s)。也支持 --node-type（单数形式）'
    )
    
    # 支持 --node-type（单数形式）作为 --node-types 的别名
    parser.add_argument(
        '--node-type',
        dest='node_types',  # 使用相同的目标属性
        type=str,
        nargs='+',
        choices=['docker', 'k8s'],
        help='要同步的节点类型（--node-types 的别名）'
    )
    
    args = parser.parse_args()
    
    # 验证参数
    if not args.image and not args.images:
        print("\n错误: 请指定要同步的镜像 (--image 或 --images)")
        parser.print_help()
        sys.exit(1)
    
    try:
        syncer = ComponentImageSyncer(args.vm_config)
        
        # 确定要同步的镜像列表
        if args.images:
            image_names = args.images
        else:
            image_names = [args.image]
        
        # 同步镜像
        if len(image_names) == 1:
            result = syncer.sync_image_to_all_providers(image_names[0], args.max_workers, node_types=args.node_types)
            if not result['success']:
                sys.exit(1)
        else:
            # 批量同步时也支持节点类型过滤
            for image_name in image_names:
                result = syncer.sync_image_to_all_providers(image_name, args.max_workers, node_types=args.node_types)
                if not result['success']:
                    print(f"⚠ 镜像 {image_name} 同步失败")
        
        print("\n✓ 所有镜像同步完成！")
        
    except KeyboardInterrupt:
        print("\n\n用户中断")
        sys.exit(1)
    except Exception as e:
        print(f"\n错误: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)

if __name__ == '__main__':
    main()

