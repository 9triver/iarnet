#!/bin/bash
# 修复 fake provider 的 CPU 和内存使用率，使其不同（更符合真实场景）

# 为每个 iarnet 实例修复配置
for i in {1..10}; do
  CONFIG_FILE="iarnet/i${i}/config.yaml"
  
  if [ ! -f "$CONFIG_FILE" ]; then
    echo "跳过 $CONFIG_FILE（文件不存在）"
    continue
  fi
  
  echo "处理 $CONFIG_FILE..."
  
  # 创建临时文件
  TEMP_FILE=$(mktemp)
  
  # 使用 Python 脚本处理
  python3 << 'PYTHON_SCRIPT' > "$TEMP_FILE"
import re
import random
import sys

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

if __name__ == "__main__":
    filename = sys.argv[1]
    with open(filename, 'r', encoding='utf-8') as f:
        content = f.read()
    
    fixed_content = fix_usage_ratios(content)
    print(fixed_content, end='')
PYTHON_SCRIPT
  
  # 将处理后的内容写回原文件
  python3 "$TEMP_FILE" "$CONFIG_FILE" > "${CONFIG_FILE}.new"
  mv "${CONFIG_FILE}.new" "$CONFIG_FILE"
  rm -f "$TEMP_FILE"
  
  echo "完成 $CONFIG_FILE"
done

echo "所有配置文件已修复完成！"

