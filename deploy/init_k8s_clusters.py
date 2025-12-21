#!/usr/bin/env python3
"""
一键初始化所有 K8s 集群
包括：
1. 在每个 master 节点上执行 kubeadm init
2. 配置 kubectl
3. 安装网络插件（Flannel）
4. 获取 join token
5. 在每个 worker 节点上执行 kubeadm join
"""

import os
import sys
import yaml
import argparse
import subprocess
import time
import re
import shlex
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
from threading import Lock

# 获取脚本所在目录
SCRIPT_DIR = Path(__file__).parent.absolute()

class K8sClusterInitializer:
    _log_lock = Lock()
    
    def __init__(self, vm_config_path: str):
        """初始化集群初始化器"""
        vm_config_path_obj = Path(vm_config_path)
        if not vm_config_path_obj.is_absolute():
            if (SCRIPT_DIR / vm_config_path_obj.name).exists():
                vm_config_path_obj = SCRIPT_DIR / vm_config_path_obj.name
            else:
                vm_config_path_obj = Path(vm_config_path)
        
        if not vm_config_path_obj.exists():
            raise FileNotFoundError(f"配置文件不存在: {vm_config_path_obj}")
        
        with open(vm_config_path_obj, 'r', encoding='utf-8') as f:
            self.vm_config = yaml.safe_load(f)
        
        k8s_config = self.vm_config['vm_types']['k8s_clusters']
        self.master_config = k8s_config['master']
        self.worker_config = k8s_config['worker']
        self.cluster_count = k8s_config['count']
        self.k8s_pod_cidrs = self.vm_config.get('k8s_pod_cidrs', [])
        self.user = self.vm_config['global']['user']
        
        # 处理 SSH 密钥路径
        ssh_key_config = self.vm_config['global'].get('ssh_key_path', '~/.ssh/id_rsa.pub')
        ssh_key_path = os.path.expanduser(ssh_key_config)
        if ssh_key_path.endswith('.pub'):
            self.ssh_key_path = ssh_key_path[:-4]
        else:
            self.ssh_key_path = ssh_key_path
        
        if not os.path.exists(self.ssh_key_path):
            default_keys = [
                os.path.expanduser('~/.ssh/id_rsa'),
                os.path.expanduser('~/.ssh/id_ed25519'),
            ]
            for key_path in default_keys:
                if os.path.exists(key_path):
                    self.ssh_key_path = key_path
                    break
        
        # 构建集群信息
        self.cluster_info = {}
        for cluster_id in range(self.cluster_count):
            master_ip_suffix = self.master_config['ip_start'] + cluster_id * self.master_config['ip_step']
            master_ip = f"{self.master_config['ip_base']}.{master_ip_suffix}"
            master_hostname = f"{self.master_config['hostname_prefix']}-{cluster_id+1:02d}{self.master_config['hostname_suffix']}"
            
            # Worker 节点信息
            workers = []
            for worker_id in range(1, self.worker_config['count_per_cluster'] + 1):
                worker_ip_suffix = master_ip_suffix + worker_id
                worker_ip = f"{self.master_config['ip_base']}.{worker_ip_suffix}"
                worker_hostname = f"{self.worker_config['hostname_prefix']}-{cluster_id+1:02d}-{self.worker_config['hostname_suffix']}-{worker_id}"
                workers.append({
                    'hostname': worker_hostname,
                    'ip': worker_ip
                })
            
            # Pod CIDR（从配置中获取，如果没有则使用默认值）
            pod_cidr = self.k8s_pod_cidrs[cluster_id] if cluster_id < len(self.k8s_pod_cidrs) else f"10.{240 + cluster_id}.0.0/16"
            
            self.cluster_info[cluster_id] = {
                'master': {
                    'hostname': master_hostname,
                    'ip': master_ip
                },
                'workers': workers,
                'pod_cidr': pod_cidr
            }
    
    def _print(self, *args, **kwargs):
        """线程安全的打印函数"""
        with self._log_lock:
            kwargs.setdefault('flush', True)
            print(*args, **kwargs)
    
    def _build_ssh_cmd(self, ip: str) -> list:
        """构建 SSH 命令"""
        return [
            'ssh',
            '-i', self.ssh_key_path,
            '-o', 'StrictHostKeyChecking=no',
            '-o', 'UserKnownHostsFile=/dev/null',
            '-o', 'ConnectTimeout=5',
            f'{self.user}@{ip}'
        ]
    
    def check_node_connectivity(self, ip: str) -> bool:
        """检查节点连通性"""
        try:
            result = subprocess.run(
                ['ping', '-c', '1', '-W', '2', ip],
                capture_output=True,
                timeout=5
            )
            return result.returncode == 0
        except:
            return False
    
    def init_master(self, cluster_id: int) -> tuple[bool, str]:
        """初始化 master 节点
        
        Returns:
            (success, join_command): 成功标志和 join 命令
        """
        cluster = self.cluster_info[cluster_id]
        master = cluster['master']
        cluster_prefix = f"[集群 {cluster_id+1:02d}]"
        
        self._print(f"{cluster_prefix} 初始化 master 节点: {master['hostname']} ({master['ip']})")
        
        # 检查连通性
        if not self.check_node_connectivity(master['ip']):
            self._print(f"{cluster_prefix} ✗ 无法连接到 master 节点")
            return False, ""
        
        ssh_cmd = self._build_ssh_cmd(master['ip'])
        
        # 1. 清理可能存在的旧 K8s 配置（如果之前初始化过）
        self._print(f"{cluster_prefix}   清理可能存在的旧 K8s 配置...")
        cleanup_cmd = '''sudo systemctl stop kubelet 2>/dev/null || true && \\
sudo kubeadm reset -f 2>/dev/null || true && \\
sudo rm -rf /etc/kubernetes/manifests/* 2>/dev/null || true && \\
sudo rm -rf /etc/kubernetes/*.conf 2>/dev/null || true && \\
sudo rm -rf /etc/kubernetes/pki 2>/dev/null || true && \\
sudo rm -rf /home/ubuntu/.kube 2>/dev/null || true && \\
sudo rm -rf /root/.kube 2>/dev/null || true && \\
sudo pkill -9 -f kubelet 2>/dev/null || true && \\
sudo crictl rm -a -f 2>/dev/null || true && \\
sudo systemctl stop containerd 2>/dev/null || true && \\
sudo rm -rf /var/lib/containerd/io.containerd.runtime.v2.task/k8s.io/* 2>/dev/null || true && \\
sudo systemctl daemon-reload 2>/dev/null || true && \\
sudo systemctl restart containerd 2>/dev/null || true && \\
sudo systemctl enable kubelet 2>/dev/null || true && \\
sudo systemctl start kubelet 2>/dev/null || true && \\
sleep 2 && \\
echo "清理完成"
'''
        cleanup_ssh_cmd = ' '.join(ssh_cmd) + f' "{cleanup_cmd}"'
        
        try:
            cleanup_result = subprocess.run(
                cleanup_ssh_cmd,
                shell=True,
                check=False,
                timeout=60,
                capture_output=True,
                text=True
            )
            if '清理完成' in cleanup_result.stdout:
                self._print(f"{cluster_prefix}   ✓ 清理完成")
            else:
                self._print(f"{cluster_prefix}   ⚠ 清理可能未完全成功，但继续执行")
        except:
            self._print(f"{cluster_prefix}   ⚠ 清理超时或失败，但继续执行")
        
        # 2. 执行 kubeadm init（镜像已预拉取，init 会直接使用现有镜像）
        self._print(f"{cluster_prefix}   执行 kubeadm init...")
        init_cmd = f'''sudo kubeadm init \\
  --pod-network-cidr={cluster['pod_cidr']} \\
  --apiserver-advertise-address={master['ip']} \\
  --control-plane-endpoint={master['ip']}:6443 \\
  --ignore-preflight-errors=Swap
'''
        
        # 输出完整的命令，方便手动执行和排查
        self._print(f"{cluster_prefix}   完整 kubeadm init 命令（可在 {master['hostname']} 上手动执行）:")
        self._print(f"{cluster_prefix}   =========================================")
        # 输出原始命令（去掉 sudo，因为用户可能已经以 root 身份登录）
        init_cmd_clean = init_cmd.replace('sudo ', '').strip()
        for line in init_cmd_clean.split('\n'):
            if line.strip():
                self._print(f"{cluster_prefix}   {line.strip()}")
        self._print(f"{cluster_prefix}   =========================================")
        
        # 输出 SSH 命令（用于从本地执行）
        self._print(f"{cluster_prefix}   或通过 SSH 执行（从本地机器）:")
        self._print(f"{cluster_prefix}   ssh -i {self.ssh_key_path} {self.user}@{master['ip']} \"{init_cmd.strip()}\"")
        self._print(f"{cluster_prefix}   =========================================")
        
        init_ssh_cmd = ' '.join(ssh_cmd) + f' "{init_cmd}"'
        
        try:
            result = subprocess.run(
                init_ssh_cmd,
                shell=True,
                check=False,
                timeout=1200,  # 20 分钟超时（如果镜像已预先拉取，应该更快）
                capture_output=True,
                text=True
            )
            
            if result.returncode != 0:
                self._print(f"{cluster_prefix} ✗ kubeadm init 失败")
                # 显示详细错误信息
                error_output = (result.stderr or '') + (result.stdout or '')
                # 过滤 SSH 警告
                error_lines = [line for line in error_output.split('\n') 
                              if line.strip() and 'Warning:' not in line and 'Permanently added' not in line]
                if error_lines:
                    self._print(f"{cluster_prefix}   错误信息:")
                    for line in error_lines[:15]:  # 显示前15行
                        if line.strip():
                            self._print(f"{cluster_prefix}     {line}")
                
                # 诊断 kubelet 状态
                self._print(f"{cluster_prefix}   诊断 kubelet 状态...")
                diagnose_cmd = 'systemctl status kubelet --no-pager -l 2>&1 | head -20 || echo "无法获取 kubelet 状态"'
                diagnose_ssh_cmd = ' '.join(ssh_cmd) + f' "{diagnose_cmd}"'
                try:
                    diagnose_result = subprocess.run(
                        diagnose_ssh_cmd,
                        shell=True,
                        check=False,
                        timeout=10,
                        capture_output=True,
                        text=True
                    )
                    if diagnose_result.stdout:
                        diagnose_lines = [line for line in diagnose_result.stdout.split('\n') 
                                        if line.strip() and 'Warning:' not in line and 'Permanently added' not in line]
                        if diagnose_lines:
                            self._print(f"{cluster_prefix}   kubelet 状态:")
                            for line in diagnose_lines[:8]:
                                if line.strip():
                                    self._print(f"{cluster_prefix}     {line}")
                except:
                    pass
                
                return False, ""
            
            # 从输出中提取 join 命令
            # 注意：从 init 输出中提取的 join 命令可能跨多行，需要完整拼接
            # 优先提取 worker 节点的 join 命令（最后一个，不包含 --control-plane）
            join_command = ""
            lines = result.stdout.split('\n')
            
            # 从后往前查找，找到最后一个 kubeadm join 命令（通常是 worker 节点的）
            for i in range(len(lines) - 1, -1, -1):
                line = lines[i]
                if 'kubeadm join' in line:
                    # 提取完整的 join 命令（可能跨多行）
                    temp_command = line.strip()
                    # 检查后续行是否包含 --control-plane（如果是，则跳过这个命令）
                    is_control_plane = False
                    for j in range(i, min(i + 5, len(lines))):  # 检查后续最多5行
                        if '--control-plane' in lines[j]:
                            is_control_plane = True
                            break
                    
                    if is_control_plane:
                        continue  # 跳过 control-plane 的命令
                    
                    # 这是 worker 节点的命令，开始提取
                    join_command = temp_command
                    # 如果命令以反斜杠结尾，说明跨多行，需要拼接后续行
                    if join_command.endswith('\\'):
                        # 继续读取后续行，直到找到完整的命令
                        for j in range(i + 1, len(lines)):
                            next_line = lines[j].strip()
                            if next_line:  # 跳过空行
                                # 移除行首的缩进（通常是空格或制表符）
                                next_line = next_line.lstrip()
                                # 如果遇到 --control-plane，说明这是 control-plane 命令的一部分，停止
                                if '--control-plane' in next_line:
                                    join_command = ""
                                    break
                                join_command = join_command.rstrip('\\').rstrip() + ' ' + next_line
                                # 如果这一行也以反斜杠结尾，继续读取下一行
                                if not next_line.endswith('\\'):
                                    break
                    
                    # 检查是否包含必要的参数
                    if join_command and '--token' in join_command and '--discovery-token-ca-cert-hash' in join_command:
                        # 清理命令中的多余空格
                        join_command = ' '.join(join_command.split())
                        self._print(f"{cluster_prefix}   ✓ 从 init 输出中提取到完整的 worker join 命令")
                        break
                    else:
                        join_command = ""  # 重置，继续查找
            
            # 如果没找到，尝试从 stderr 中查找（某些版本可能输出到 stderr）
            if not join_command:
                stderr_lines = result.stderr.split('\n')
                # 从后往前查找
                for i in range(len(stderr_lines) - 1, -1, -1):
                    line = stderr_lines[i]
                    if 'kubeadm join' in line:
                        # 检查后续行是否包含 --control-plane
                        is_control_plane = False
                        for j in range(i, min(i + 5, len(stderr_lines))):
                            if '--control-plane' in stderr_lines[j]:
                                is_control_plane = True
                                break
                        
                        if is_control_plane:
                            continue
                        
                        join_command = line.strip()
                        # 处理跨行情况
                        if join_command.endswith('\\'):
                            for j in range(i + 1, len(stderr_lines)):
                                next_line = stderr_lines[j].strip().lstrip()
                                if next_line:
                                    if '--control-plane' in next_line:
                                        join_command = ""
                                        break
                                    join_command = join_command.rstrip('\\').rstrip() + ' ' + next_line
                                    if not next_line.endswith('\\'):
                                        break
                        # 清理命令
                        join_command = ' '.join(join_command.split())
                        if join_command and '--token' in join_command and '--discovery-token-ca-cert-hash' in join_command:
                            self._print(f"{cluster_prefix}   ✓ 从 init stderr 中提取到完整的 worker join 命令")
                            break
                        else:
                            join_command = ""
            
            # 如果提取的 join 命令不完整（缺少 ca-cert-hash），清空它，后续会重新生成
            if join_command and '--discovery-token-ca-cert-hash' not in join_command:
                self._print(f"{cluster_prefix}   ⚠ 从 init 输出中提取的 join 命令不完整，将重新生成")
                self._print(f"{cluster_prefix}     提取的命令: {join_command[:100]}")
                join_command = ""
            elif join_command:
                # 显示成功提取的命令（用于调试）
                self._print(f"{cluster_prefix}   ✓ 成功从 init 输出中提取 join 命令")
                self._print(f"{cluster_prefix}     命令: {join_command[:150]}...")
            
            self._print(f"{cluster_prefix}   ✓ kubeadm init 成功")
            
            # kubeadm init 成功后，配置 crictl 并重启 containerd 和 kubelet 以清理可能的残留状态
            self._print(f"{cluster_prefix}   配置 crictl 并重启 containerd 和 kubelet 以清理状态...")
            restart_cmd = '''echo 'runtime-endpoint: unix:///var/run/containerd/containerd.sock' | sudo tee /etc/crictl.yaml > /dev/null && \\
echo 'image-endpoint: unix:///var/run/containerd/containerd.sock' | sudo tee -a /etc/crictl.yaml > /dev/null && \\
sudo systemctl restart containerd && sudo systemctl restart kubelet && sleep 3'''
            restart_ssh_cmd = ' '.join(ssh_cmd) + f' "{restart_cmd}"'
            try:
                subprocess.run(
                    restart_ssh_cmd,
                    shell=True,
                    check=False,
                    timeout=30,
                    capture_output=True
                )
                self._print(f"{cluster_prefix}   ✓ containerd 和 kubelet 已重启")
            except:
                self._print(f"{cluster_prefix}   ⚠ 重启 containerd/kubelet 可能失败，但继续...")
            
        except subprocess.TimeoutExpired:
            self._print(f"{cluster_prefix} ✗ kubeadm init 超时")
            return False, ""
        except Exception as e:
            self._print(f"{cluster_prefix} ✗ kubeadm init 异常: {e}")
            return False, ""
        
        # 2. 配置 kubectl（为 ubuntu 用户和 root 用户都配置）
        self._print(f"{cluster_prefix}   配置 kubectl...")
        # 使用硬编码路径 /home/ubuntu，因为脚本在本地运行，~ 会在本地解析
        # 同时为 ubuntu 用户和 root 用户配置 kubeconfig，确保 sudo kubectl 也能正常工作
        kubectl_setup_cmd = '''sudo mkdir -p /home/ubuntu/.kube /root/.kube && \\
sudo cp -f /etc/kubernetes/admin.conf /home/ubuntu/.kube/config && \\
sudo cp -f /etc/kubernetes/admin.conf /root/.kube/config && \\
sudo chown $(id -u):$(id -g) /home/ubuntu/.kube/config && \\
sudo chown root:root /root/.kube/config && \\
sudo chmod 600 /home/ubuntu/.kube/config /root/.kube/config'''
        kubectl_ssh_cmd = ' '.join(ssh_cmd) + f' "{kubectl_setup_cmd}"'
        
        try:
            result = subprocess.run(
                kubectl_ssh_cmd,
                shell=True,
                check=False,
                timeout=30,
                capture_output=True,
                text=True
            )
            if result.returncode == 0:
                self._print(f"{cluster_prefix}   ✓ kubectl 配置完成")
            else:
                error_msg = (result.stderr or result.stdout or '')[:200]
                error_lines = [line for line in error_msg.split('\n') 
                              if line.strip() and 'Warning:' not in line and 'Permanently added' not in line]
                if error_lines:
                    self._print(f"{cluster_prefix}   ⚠ kubectl 配置可能失败: {error_lines[0]}")
                else:
                    self._print(f"{cluster_prefix}   ⚠ kubectl 配置可能失败（返回码: {result.returncode}）")
        except Exception as e:
            self._print(f"{cluster_prefix}   ⚠ kubectl 配置异常: {e}")
        
        # 3. 安装网络插件（Flannel）
        self._print(f"{cluster_prefix}   安装网络插件 (Flannel)...")
        flannel_cmd = 'sudo KUBECONFIG=/home/ubuntu/.kube/config kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml'
        flannel_ssh_cmd = ' '.join(ssh_cmd) + f' "{flannel_cmd}"'
        
        try:
            result = subprocess.run(
                flannel_ssh_cmd,
                shell=True,
                check=False,
                timeout=120,
                capture_output=True,
                text=True
            )
            if result.returncode == 0:
                self._print(f"{cluster_prefix}   ✓ Flannel 安装成功")
            else:
                error_msg = (result.stderr or result.stdout or '')[:300]
                # 过滤 SSH 警告信息
                error_lines = [line for line in error_msg.split('\n') 
                              if line.strip() and 'Warning:' not in line and 'Permanently added' not in line]
                if error_lines:
                    self._print(f"{cluster_prefix}   ⚠ Flannel 安装可能失败:")
                    for line in error_lines[:5]:
                        if line.strip():
                            self._print(f"{cluster_prefix}     {line}")
                else:
                    self._print(f"{cluster_prefix}   ⚠ Flannel 安装可能失败（返回码: {result.returncode}）")
        except:
            self._print(f"{cluster_prefix}   ⚠ Flannel 安装超时或失败，但继续...")
        
        # 5. 等待一下让集群完全初始化
        time.sleep(5)
        
        # 6. 如果没有从 init 输出中获取完整的 join 命令，使用 token create 重新生成
        # 优先使用 token create，因为它总是生成完整的命令
        if not join_command or '--discovery-token-ca-cert-hash' not in join_command:
            self._print(f"{cluster_prefix}   获取 join 命令...")
            # 稍作等待，让 API 服务器有时间启动
            time.sleep(5)
            # 使用 sudo 执行，确保有权限
            token_cmd = 'sudo kubeadm token create --print-join-command 2>/dev/null'
            token_ssh_cmd = ' '.join(ssh_cmd) + f' "{token_cmd}"'
            
            max_retries = 5
            for retry in range(max_retries):
                try:
                    result = subprocess.run(
                        token_ssh_cmd,
                        shell=True,
                        check=False,
                        timeout=30,
                        capture_output=True,
                        text=True
                    )
                    if result.returncode == 0 and result.stdout.strip():
                        new_join_command = result.stdout.strip()
                        # 验证命令是否完整
                        if '--token' in new_join_command and '--discovery-token-ca-cert-hash' in new_join_command:
                            join_command = new_join_command
                            self._print(f"{cluster_prefix}   ✓ 成功获取完整的 join 命令")
                            break
                        else:
                            if retry < max_retries - 1:
                                self._print(f"{cluster_prefix}   获取的 join 命令不完整，等待后重试... ({retry+1}/{max_retries})")
                                time.sleep(5)
                    else:
                        if retry < max_retries - 1:
                            error_msg = (result.stderr or result.stdout or '')[:100]
                            self._print(f"{cluster_prefix}   获取 join 命令失败，等待后重试... ({retry+1}/{max_retries})")
                            if error_msg:
                                error_lines = [line for line in error_msg.split('\n') 
                                            if line.strip() and 'Warning:' not in line and 'Permanently added' not in line]
                                if error_lines:
                                    self._print(f"{cluster_prefix}     错误: {error_lines[0]}")
                            time.sleep(5)
                except Exception as e:
                    if retry < max_retries - 1:
                        self._print(f"{cluster_prefix}   获取 join 命令异常: {e}，等待后重试... ({retry+1}/{max_retries})")
                        time.sleep(5)
        
        if not join_command:
            self._print(f"{cluster_prefix}   ⚠ 警告: 无法获取 join 命令，将在后续步骤中重试")
        
        return True, join_command
    
    def join_worker(self, cluster_id: int, worker: dict, join_command: str) -> bool:
        """Worker 节点加入集群"""
        cluster_prefix = f"[集群 {cluster_id+1:02d}]"
        worker_prefix = f"{cluster_prefix} Worker {worker['hostname']}"
        
        self._print(f"{worker_prefix} 加入集群...")
        
        # 检查连通性
        if not self.check_node_connectivity(worker['ip']):
            self._print(f"{worker_prefix} ✗ 无法连接到 worker 节点")
            return False
        
        ssh_cmd = self._build_ssh_cmd(worker['ip'])
        
        # 如果 join_command 不完整，先验证
        if not join_command or '--discovery-token-ca-cert-hash' not in join_command:
            self._print(f"{worker_prefix} ✗ join 命令不完整，无法执行")
            return False
        
        # 直接执行 join 命令
        self._print(f"{worker_prefix}   执行 join 命令...")
        join_ssh_cmd = ' '.join(ssh_cmd) + f' "sudo {join_command}"'
        
        try:
            result = subprocess.run(
                join_ssh_cmd,
                shell=True,
                check=False,
                timeout=300,  # 5 分钟超时
                capture_output=True,
                text=True
            )
            
            full_output = (result.stdout or '') + (result.stderr or '')
            
            if result.returncode == 0:
                self._print(f"{worker_prefix} ✓ 加入集群成功")
                return True
            else:
                # 检查是否是已经加入的错误（idempotent）
                if 'This node has joined the cluster' in full_output or 'already a member' in full_output.lower():
                    self._print(f"{worker_prefix} ✓ 节点已加入集群（之前已加入）")
                    return True
                self._print(f"{worker_prefix} ✗ 加入集群失败")
                # 显示错误信息（过滤 SSH 警告）
                error_lines = [line for line in full_output.split('\n') 
                              if line.strip() and 'Warning:' not in line and 'Permanently added' not in line]
                if error_lines:
                    self._print(f"{worker_prefix}   错误: {' '.join(error_lines)}")
                return False
                
        except subprocess.TimeoutExpired:
            self._print(f"{worker_prefix} ✗ 加入集群超时")
            return False
        except Exception as e:
            self._print(f"{worker_prefix} ✗ 加入集群异常: {e}")
            return False
    
    def init_cluster(self, cluster_id: int) -> bool:
        """初始化整个集群（master + workers）"""
        cluster = self.cluster_info[cluster_id]
        cluster_prefix = f"[集群 {cluster_id+1:02d}]"
        
        self._print(f"\n{cluster_prefix} 开始初始化集群...")
        self._print(f"{cluster_prefix} Master: {cluster['master']['hostname']} ({cluster['master']['ip']})")
        self._print(f"{cluster_prefix} Workers: {len(cluster['workers'])} 个")
        self._print(f"{cluster_prefix} Pod CIDR: {cluster['pod_cidr']}")
        
        # 1. 初始化 master
        success, join_command = self.init_master(cluster_id)
        if not success:
            self._print(f"{cluster_prefix} ✗ Master 初始化失败，跳过此集群")
            return False
        
        if not join_command or '--discovery-token-ca-cert-hash' not in join_command:
            self._print(f"{cluster_prefix} ⚠ 无法获取完整的 join 命令，尝试从 master 节点重新获取...")
            # 尝试从 master 节点重新获取 join 命令
            ssh_cmd = self._build_ssh_cmd(cluster['master']['ip'])
            token_cmd = 'sudo kubeadm token create --print-join-command'
            token_ssh_cmd = ' '.join(ssh_cmd) + f' "{token_cmd}"'
            
            try:
                result = subprocess.run(
                    token_ssh_cmd,
                    shell=True,
                    check=False,
                    timeout=30,
                    capture_output=True,
                    text=True
                )
                if result.returncode == 0 and result.stdout.strip():
                    new_join_command = result.stdout.strip()
                    if '--token' in new_join_command and '--discovery-token-ca-cert-hash' in new_join_command:
                        join_command = new_join_command
                        self._print(f"{cluster_prefix}   ✓ 成功获取完整的 join 命令")
                    else:
                        self._print(f"{cluster_prefix}   ✗ 获取的 join 命令不完整")
                        return False
                else:
                    error_msg = (result.stderr or result.stdout or '')[:200]
                    self._print(f"{cluster_prefix}   ✗ 无法获取 join 命令")
                    if error_msg:
                        error_lines = [line for line in error_msg.split('\n') 
                                      if line.strip() and 'Warning:' not in line and 'Permanently added' not in line]
                        if error_lines:
                            self._print(f"{cluster_prefix}     错误: {error_lines[0]}")
                    return False
            except Exception as e:
                self._print(f"{cluster_prefix}   ✗ 获取 join 命令失败: {e}")
                return False
        
        # 等待 master 节点就绪
        self._print(f"{cluster_prefix}   等待 master 节点就绪...")
        time.sleep(10)
        
        # 验证 master 节点状态
        ssh_cmd = self._build_ssh_cmd(cluster['master']['ip'])
        check_master_cmd = 'sudo KUBECONFIG=/home/ubuntu/.kube/config kubectl get nodes --no-headers 2>/dev/null | grep -q . && echo READY || echo NOT_READY'
        check_master_ssh_cmd = ' '.join(ssh_cmd) + f' "{check_master_cmd}"'
        
        max_wait = 6
        for i in range(max_wait):
            try:
                result = subprocess.run(
                    check_master_ssh_cmd,
                    shell=True,
                    check=False,
                    timeout=10,
                    capture_output=True,
                    text=True
                )
                if 'READY' in result.stdout:
                    self._print(f"{cluster_prefix}   ✓ Master 节点已就绪")
                    break
                else:
                    if i < max_wait - 1:
                        self._print(f"{cluster_prefix}   等待中... ({i+1}/{max_wait})")
                        time.sleep(5)
            except:
                time.sleep(5)
        
        # 2. Worker 节点加入集群
        worker_success_count = 0
        for worker in cluster['workers']:
            if self.join_worker(cluster_id, worker, join_command):
                worker_success_count += 1
            time.sleep(5)  # 每个 worker 之间稍作延迟
        
        # 3. 验证集群状态
        self._print(f"{cluster_prefix}   验证集群状态...")
        ssh_cmd = self._build_ssh_cmd(cluster['master']['ip'])
        check_cmd = 'sudo KUBECONFIG=/home/ubuntu/.kube/config kubectl get nodes'
        check_ssh_cmd = ' '.join(ssh_cmd) + f' "{check_cmd}"'
        
        try:
            result = subprocess.run(
                check_ssh_cmd,
                shell=True,
                check=False,
                timeout=30,
                capture_output=True,
                text=True
            )
            if result.returncode == 0:
                self._print(f"{cluster_prefix}   集群节点状态:")
                for line in result.stdout.split('\n')[:5]:  # 只显示前几行
                    if line.strip():
                        self._print(f"{cluster_prefix}     {line}")
        except:
            pass
        
        if worker_success_count == len(cluster['workers']):
            self._print(f"{cluster_prefix} ✓ 集群初始化完成（{len(cluster['workers'])}/{len(cluster['workers'])} workers 已加入）")
            return True
        else:
            self._print(f"{cluster_prefix} ⚠ 集群部分初始化（{worker_success_count}/{len(cluster['workers'])} workers 已加入）")
            return worker_success_count > 0
    
    def init_all_clusters(self, cluster_ids: list = None, max_workers: int = 5) -> dict:
        """初始化所有或指定集群
        
        Args:
            cluster_ids: 要初始化的集群 ID 列表，如果为 None 则初始化所有集群
            max_workers: 最大并发线程数
        
        Returns:
            dict: {'success': [...], 'failed': [...]}
        """
        if cluster_ids is None:
            cluster_ids = list(range(self.cluster_count))
        
        self._print(f"开始初始化 {len(cluster_ids)} 个 K8s 集群...")
        self._print("=" * 60)
        
        success_clusters = []
        failed_clusters = []
        
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            future_to_cluster = {
                executor.submit(self.init_cluster, cluster_id): cluster_id
                for cluster_id in cluster_ids
            }
            
            for future in as_completed(future_to_cluster):
                cluster_id = future_to_cluster[future]
                try:
                    success = future.result()
                    if success:
                        success_clusters.append(cluster_id)
                    else:
                        failed_clusters.append(cluster_id)
                except Exception as e:
                    self._print(f"[集群 {cluster_id+1:02d}] ✗ 初始化异常: {e}")
                    failed_clusters.append(cluster_id)
        
        # 输出总结
        self._print("\n" + "=" * 60)
        self._print("集群初始化完成")
        self._print(f"  成功: {len(success_clusters)} 个集群")
        self._print(f"  失败: {len(failed_clusters)} 个集群")
        
        if failed_clusters:
            self._print(f"\n失败的集群: {[c+1 for c in failed_clusters]}")
        
        return {
            'success': success_clusters,
            'failed': failed_clusters
        }


def parse_cluster_range(cluster_range: str) -> list:
    """解析集群范围字符串"""
    cluster_ids = []
    parts = cluster_range.split(',')
    for part in parts:
        part = part.strip()
        if '-' in part:
            start, end = part.split('-', 1)
            cluster_ids.extend(range(int(start), int(end) + 1))
        else:
            cluster_ids.append(int(part))
    return sorted(set(cluster_ids))


def main():
    parser = argparse.ArgumentParser(
        description='一键初始化 K8s 集群',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 初始化所有集群
  python3 init_k8s_clusters.py

  # 初始化指定范围的集群
  python3 init_k8s_clusters.py --clusters 0-9

  # 初始化指定集群
  python3 init_k8s_clusters.py --clusters 0,1,2

  # 指定配置文件
  python3 init_k8s_clusters.py --vm-config /path/to/vm-config.yaml

  # 调整并发数
  python3 init_k8s_clusters.py --max-workers 3
        """
    )
    
    parser.add_argument(
        '--vm-config',
        default='vm-config.yaml',
        help='虚拟机配置文件路径（默认: vm-config.yaml）'
    )
    
    parser.add_argument(
        '--clusters',
        help='集群范围，格式: start-end 或逗号分隔的列表 (默认: 所有集群)'
    )
    
    parser.add_argument(
        '--max-workers',
        type=int,
        default=3,
        help='最大并发线程数（默认: 3，建议不要太大，避免资源竞争）'
    )
    
    args = parser.parse_args()
    
    try:
        initializer = K8sClusterInitializer(args.vm_config)
        
        cluster_ids = None
        if args.clusters:
            # 转换为 0-based 索引
            cluster_ids_raw = parse_cluster_range(args.clusters)
            cluster_ids = [cid - 1 for cid in cluster_ids_raw]  # 转换为 0-based
            print(f"将初始化以下集群: {cluster_ids_raw}")
        
        results = initializer.init_all_clusters(cluster_ids=cluster_ids, max_workers=args.max_workers)
        
        # 根据结果设置退出码
        if results['failed']:
            sys.exit(1)
        else:
            sys.exit(0)
            
    except KeyboardInterrupt:
        print("\n\n操作被用户中断")
        sys.exit(130)
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()