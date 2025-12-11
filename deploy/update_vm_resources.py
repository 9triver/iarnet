#!/usr/bin/env python3
"""
更新已存在虚拟机的 CPU 和内存资源（不删除虚拟机）
"""

import yaml
import subprocess
import sys
import argparse
from pathlib import Path

# 获取脚本所在目录
SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent

def update_vm_resources(vm_name: str, cpu: int, memory_mb: int, dry_run: bool = False) -> bool:
    """更新单个虚拟机的 CPU 和内存资源"""
    print(f"\n处理虚拟机: {vm_name}")
    
    # 检查虚拟机是否存在
    try:
        result = subprocess.run(
            ['virsh', 'dominfo', vm_name],
            capture_output=True,
            text=True,
            check=True
        )
    except subprocess.CalledProcessError:
        print(f"  ⚠️  虚拟机 {vm_name} 不存在，跳过")
        return False
    
    # 检查虚拟机状态
    is_running = 'running' in result.stdout.lower()
    
    if dry_run:
        print(f"  [DRY RUN] 将更新: CPU={cpu}, Memory={memory_mb}MB")
        if is_running:
            print(f"  [DRY RUN] 需要关闭虚拟机以应用更改")
        return True
    
    # 获取当前虚拟机 XML
    try:
        xml_result = subprocess.run(
            ['virsh', 'dumpxml', vm_name],
            capture_output=True,
            text=True,
            check=True
        )
        xml_content = xml_result.stdout
    except subprocess.CalledProcessError as e:
        print(f"  ✗ 无法获取虚拟机 XML: {e}")
        return False
    
    # 如果虚拟机正在运行，需要先关闭
    if is_running:
        print(f"  正在关闭虚拟机...")
        try:
            subprocess.run(
                ['virsh', 'shutdown', vm_name],
                check=True,
                timeout=60
            )
            # 等待虚拟机完全关闭
            print(f"  等待虚拟机关闭...")
            for i in range(30):  # 最多等待30秒
                result = subprocess.run(
                    ['virsh', 'dominfo', vm_name],
                    capture_output=True,
                    text=True
                )
                if 'shut off' in result.stdout.lower():
                    break
                import time
                time.sleep(1)
            else:
                print(f"  ⚠️  虚拟机未在30秒内关闭，尝试强制关闭...")
                subprocess.run(['virsh', 'destroy', vm_name], check=False)
        except subprocess.CalledProcessError as e:
            print(f"  ✗ 关闭虚拟机失败: {e}")
            return False
        except subprocess.TimeoutExpired:
            print(f"  ⚠️  关闭虚拟机超时，尝试强制关闭...")
            subprocess.run(['virsh', 'destroy', vm_name], check=False)
    
    # 修改 XML 中的 CPU 和内存
    import re
    
    # 更新 CPU
    # 查找 <vcpu placement='static'>X</vcpu>
    xml_content = re.sub(
        r'<vcpu[^>]*>\d+</vcpu>',
        f'<vcpu placement=\'static\'>{cpu}</vcpu>',
        xml_content
    )
    
    # 更新内存（转换为 KB）
    memory_kb = memory_mb * 1024
    # 查找 <memory unit='KiB'>XXX</memory> 和 <currentMemory unit='KiB'>XXX</currentMemory>
    xml_content = re.sub(
        r'<memory[^>]*>\d+</memory>',
        f'<memory unit=\'KiB\'>{memory_kb}</memory>',
        xml_content
    )
    xml_content = re.sub(
        r'<currentMemory[^>]*>\d+</currentMemory>',
        f'<currentMemory unit=\'KiB\'>{memory_kb}</currentMemory>',
        xml_content
    )
    
    # 保存临时 XML 文件
    import tempfile
    with tempfile.NamedTemporaryFile(mode='w', suffix='.xml', delete=False) as f:
        f.write(xml_content)
        temp_xml = f.name
    
    try:
        # 重新定义虚拟机
        print(f"  更新虚拟机配置: CPU={cpu}, Memory={memory_mb}MB")
        subprocess.run(
            ['virsh', 'define', temp_xml],
            check=True,
            capture_output=True
        )
        print(f"  ✓ 虚拟机配置已更新")
        
        # 如果虚拟机之前是运行状态，尝试启动
        if is_running:
            print(f"  启动虚拟机...")
            subprocess.run(
                ['virsh', 'start', vm_name],
                check=True,
                capture_output=True
            )
            print(f"  ✓ 虚拟机已启动")
        
        return True
    except subprocess.CalledProcessError as e:
        print(f"  ✗ 更新失败: {e}")
        if e.stderr:
            print(f"  错误信息: {e.stderr.decode()}")
        return False
    finally:
        # 清理临时文件
        import os
        try:
            os.unlink(temp_xml)
        except:
            pass

def main():
    parser = argparse.ArgumentParser(description='更新已存在虚拟机的 CPU 和内存资源')
    parser.add_argument(
        '--vm-config', '-v',
        default=str(SCRIPT_DIR / 'vm-config.yaml'),
        help='虚拟机配置文件路径 (默认: deploy/vm-config.yaml)'
    )
    parser.add_argument(
        '--type', '-t',
        choices=['docker', 'iarnet', 'k8s', 'all'],
        default='docker',
        help='要更新的虚拟机类型 (默认: docker)'
    )
    parser.add_argument(
        '--nodes', '-n',
        type=str,
        help='节点范围，格式: start-end 或逗号分隔的列表 (例如: 0-59 或 0,1,2)。如果不指定，则更新该类型的所有节点'
    )
    parser.add_argument(
        '--dry-run',
        action='store_true',
        help='仅显示将要执行的操作，不实际修改'
    )
    
    args = parser.parse_args()
    
    # 读取虚拟机配置
    vm_config_path = Path(args.vm_config)
    if not vm_config_path.is_absolute():
        vm_config_path = SCRIPT_DIR / vm_config_path
    
    if not vm_config_path.exists():
        print(f"错误: 配置文件不存在: {vm_config_path}")
        sys.exit(1)
    
    with open(vm_config_path, 'r', encoding='utf-8') as f:
        vm_config = yaml.safe_load(f)
    
    # 确定要更新的节点
    if args.type == 'docker':
        vm_type_config = vm_config['vm_types']['docker']
        hostname_prefix = vm_type_config['hostname_prefix']
        cpu = vm_type_config['cpu']
        memory = vm_type_config['memory']
        count = vm_type_config['count']
    elif args.type == 'iarnet':
        vm_type_config = vm_config['vm_types']['iarnet']
        hostname_prefix = vm_type_config['hostname_prefix']
        cpu = vm_type_config['cpu']
        memory = vm_type_config['memory']
        count = vm_type_config['count']
    elif args.type == 'k8s':
        print("错误: K8s 集群节点更新暂不支持（结构较复杂）")
        sys.exit(1)
    elif args.type == 'all':
        print("错误: 批量更新所有类型暂不支持，请分别指定类型")
        sys.exit(1)
    else:
        print(f"错误: 不支持的虚拟机类型: {args.type}")
        sys.exit(1)
    
    # 解析节点范围
    if args.nodes:
        if '-' in args.nodes:
            start, end = map(int, args.nodes.split('-'))
            node_ids = list(range(start, end + 1))
        else:
            node_ids = [int(x.strip()) for x in args.nodes.split(',')]
    else:
        node_ids = list(range(count))
    
    # 验证节点ID范围
    invalid_nodes = [n for n in node_ids if n < 0 or n >= count]
    if invalid_nodes:
        print(f"错误: 节点ID超出范围: {invalid_nodes}")
        print(f"有效范围: 0-{count-1}")
        sys.exit(1)
    
    print("=" * 60)
    print(f"更新 {args.type} 类型虚拟机的资源")
    print(f"节点范围: {node_ids}")
    print(f"新配置: CPU={cpu}, Memory={memory}MB")
    if args.dry_run:
        print("模式: DRY RUN (仅显示，不实际修改)")
    print("=" * 60)
    
    # 更新每个节点
    success_count = 0
    fail_count = 0
    
    for node_id in node_ids:
        hostname = f"{hostname_prefix}-{node_id+1:02d}"
        if update_vm_resources(hostname, cpu, memory, args.dry_run):
            success_count += 1
        else:
            fail_count += 1
    
    print("\n" + "=" * 60)
    print(f"完成！成功: {success_count}, 失败: {fail_count}")
    print("=" * 60)
    
    if fail_count > 0:
        sys.exit(1)

if __name__ == '__main__':
    main()

