#!/usr/bin/env python3
# 修复 fake provider 的 CPU 和内存使用率，使其不同（更符合真实场景）

import re
import random
import sys
import os

def fix_usage_ratios(content):
    # 匹配 usage 块
    pattern = r'(usage:\s*\n\s*cpu_ratio:\s*)([\d.]+)(\s*\n\s*gpu_ratio:\s*)([\d.]+)(\s*\n\s*memory_ratio:\s*)([\d.]+)'
    
    def replace_usage(match):
        cpu_ratio = float(match.group(2))
        gpu_ratio = float(match.group(4))
        memory_ratio = float(match.group(6))
        
        # 如果 CPU 和内存使用率相同，生成不同的值
        if abs(cpu_ratio - memory_ratio) < 0.01:  # 如果差值小于 0.01，认为相同
            # 在原有值的基础上，CPU 和内存各自随机偏移 ±15%
            base_ratio = cpu_ratio
            cpu_offset = random.uniform(-0.15, 0.15)
            memory_offset = random.uniform(-0.15, 0.15)
            
            new_cpu = max(0.0, min(1.0, base_ratio + cpu_offset))
            new_memory = max(0.0, min(1.0, base_ratio + memory_offset))
            
            # 确保两者不同（至少相差 0.05）
            if abs(new_cpu - new_memory) < 0.05:
                if new_cpu < new_memory:
                    new_cpu = max(0.0, new_memory - 0.05)
                else:
                    new_memory = max(0.0, new_cpu - 0.05)
            
            # 保留 2 位小数
            new_cpu = round(new_cpu, 2)
            new_memory = round(new_memory, 2)
        else:
            # 如果已经不同，保持原值
            new_cpu = cpu_ratio
            new_memory = memory_ratio
        
        return f"{match.group(1)}{new_cpu}{match.group(3)}{gpu_ratio}{match.group(5)}{new_memory}"
    
    return re.sub(pattern, replace_usage, content)

def main():
    base_dir = os.path.dirname(os.path.abspath(__file__))
    
    # 为每个 iarnet 实例修复配置
    for i in range(1, 11):
        config_file = os.path.join(base_dir, f"iarnet/i{i}/config.yaml")
        
        if not os.path.exists(config_file):
            print(f"跳过 {config_file}（文件不存在）")
            continue
        
        print(f"处理 {config_file}...")
        
        try:
            with open(config_file, 'r', encoding='utf-8') as f:
                content = f.read()
            
            fixed_content = fix_usage_ratios(content)
            
            with open(config_file, 'w', encoding='utf-8') as f:
                f.write(fixed_content)
            
            print(f"完成 {config_file}")
        except Exception as e:
            print(f"处理 {config_file} 时出错: {e}")
    
    print("所有配置文件已修复完成！")

if __name__ == "__main__":
    main()

