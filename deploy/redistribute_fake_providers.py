#!/usr/bin/env python3
# 将 iarnet-1 中的部分 fake provider 重新分配到其他 iarnet 节点

import re
import random
import os
import shutil

def extract_providers_from_config(config_path):
    """从配置文件中提取所有 fake provider"""
    with open(config_path, 'r', encoding='utf-8') as f:
        lines = f.readlines()
    
    providers = []
    current_provider = []
    in_fake_providers = False
    indent_level = None
    
    for i, line in enumerate(lines):
        if 'fake_providers:' in line:
            in_fake_providers = True
            # 找到 fake_providers 的缩进级别
            indent_level = len(line) - len(line.lstrip())
            continue
        
        if in_fake_providers:
            # 检查是否到了下一个顶级键（非缩进或相同缩进）
            stripped = line.lstrip()
            if stripped and not line.startswith(' ' * (indent_level + 2)):
                # 如果遇到非缩进行（除了空行），说明 fake_providers 部分结束
                if stripped and not stripped.startswith('#'):
                    break
            
            # 收集 provider 行
            if line.strip().startswith('- name:'):
                if current_provider:
                    providers.append(''.join(current_provider))
                current_provider = [line]
            elif current_provider:
                current_provider.append(line)
    
    if current_provider:
        providers.append(''.join(current_provider))
    
    return providers

def get_fake_providers_section(config_path):
    """获取 fake_providers 部分的开始和结束位置"""
    with open(config_path, 'r', encoding='utf-8') as f:
        lines = f.readlines()
    
    start_idx = None
    end_idx = None
    indent_level = None
    
    for i, line in enumerate(lines):
        if 'fake_providers:' in line:
            start_idx = i
            indent_level = len(line) - len(line.lstrip())
            continue
        
        if start_idx is not None:
            # 检查是否到了下一个顶级键
            stripped = line.lstrip()
            if stripped and not line.startswith(' ' * (indent_level + 2)):
                if stripped and not stripped.startswith('#'):
                    end_idx = i
                    break
    
    if end_idx is None:
        end_idx = len(lines)
    
    return start_idx, end_idx, lines

def update_i1_config(config_path, providers_to_keep):
    """更新 i1 的配置，只保留指定的 provider"""
    start_idx, end_idx, lines = get_fake_providers_section(config_path)
    
    # 构建新的 fake_providers 部分
    new_section = ['  fake_providers:\n']
    for provider in providers_to_keep:
        # 确保格式正确
        provider_lines = provider.strip().split('\n')
        for pline in provider_lines:
            if pline.strip():
                new_section.append('  ' + pline + '\n')
    
    # 替换
    new_lines = lines[:start_idx + 1] + new_section + lines[end_idx:]
    
    # 备份
    shutil.copy(config_path, config_path + '.bak')
    
    with open(config_path, 'w', encoding='utf-8') as f:
        f.writelines(new_lines)

def add_providers_to_config(config_path, providers_to_add):
    """将 provider 添加到配置文件的 fake_providers 部分"""
    start_idx, end_idx, lines = get_fake_providers_section(config_path)
    
    # 在 fake_providers 部分末尾添加新的 provider
    new_providers = []
    for provider in providers_to_add:
        provider_lines = provider.strip().split('\n')
        for pline in provider_lines:
            if pline.strip():
                new_providers.append('  ' + pline + '\n')
    
    # 插入到 fake_providers 部分末尾（在 end_idx 之前）
    new_lines = lines[:end_idx] + new_providers + lines[end_idx:]
    
    # 备份
    shutil.copy(config_path, config_path + '.bak')
    
    with open(config_path, 'w', encoding='utf-8') as f:
        f.writelines(new_lines)

def main():
    base_dir = os.path.dirname(os.path.abspath(__file__))
    i1_config = os.path.join(base_dir, 'iarnet/i1/config.yaml')
    
    if not os.path.exists(i1_config):
        print(f"错误: 找不到 {i1_config}")
        return
    
    print("读取 i1 的配置文件...")
    providers = extract_providers_from_config(i1_config)
    
    if not providers:
        print("i1 中没有 fake provider")
        return
    
    print(f"i1 共有 {len(providers)} 个 fake provider")
    
    # 随机选择要移动的 provider（移动 30-50%）
    num_to_move = random.randint(max(1, len(providers) // 3), len(providers) // 2)
    providers_to_move = random.sample(providers, num_to_move)
    providers_to_keep = [p for p in providers if p not in providers_to_move]
    
    print(f"将移动 {len(providers_to_move)} 个 provider 到其他节点")
    print(f"i1 将保留 {len(providers_to_keep)} 个 provider")
    
    # 显示要移动的 provider 名称
    for i, provider in enumerate(providers_to_move, 1):
        name_match = re.search(r'name:\s*(.+)', provider)
        if name_match:
            print(f"  {i}. {name_match.group(1).strip()}")
    
    # 随机选择目标节点（i2-i10）
    target_nodes = random.sample(range(2, 11), min(len(providers_to_move), 9))
    # 如果 provider 数量超过目标节点数，循环分配
    provider_assignments = {}
    for i, provider in enumerate(providers_to_move):
        target = target_nodes[i % len(target_nodes)]
        if target not in provider_assignments:
            provider_assignments[target] = []
        provider_assignments[target].append(provider)
    
    print("\n分配计划:")
    for target, assigned_providers in provider_assignments.items():
        print(f"  i{target}: {len(assigned_providers)} 个 provider")
        for provider in assigned_providers:
            name_match = re.search(r'name:\s*(.+)', provider)
            if name_match:
                print(f"    - {name_match.group(1).strip()}")
    
    # 更新 i1 的配置（移除要移动的 provider）
    print("\n更新 i1 的配置...")
    update_i1_config(i1_config, providers_to_keep)
    print(f"✓ i1 配置已更新（保留 {len(providers_to_keep)} 个 provider）")
    
    # 将 provider 添加到目标节点
    print("\n更新目标节点的配置...")
    for target, assigned_providers in provider_assignments.items():
        target_config = os.path.join(base_dir, f'iarnet/i{target}/config.yaml')
        if not os.path.exists(target_config):
            print(f"警告: 跳过 {target_config}（文件不存在）")
            continue
        
        add_providers_to_config(target_config, assigned_providers)
        print(f"✓ i{target} 配置已更新（添加 {len(assigned_providers)} 个 provider）")
    
    print("\n重新分配完成！")
    print(f"i1: {len(providers_to_keep)} 个 provider（减少了 {len(providers_to_move)} 个）")
    for target, assigned_providers in provider_assignments.items():
        print(f"i{target}: +{len(assigned_providers)} 个 provider")

if __name__ == "__main__":
    main()
