#!/usr/bin/env python3
"""
为实验环境的 iarnet 节点生成 provider 批量导入 CSV 文件
根据 docker-compose.yaml 中的配置生成
"""

import csv
from pathlib import Path

# Provider 到 iarnet 节点的映射（根据 docker-compose.yaml）
# 格式: (provider_name, ip_address, iarnet_node)
PROVIDER_MAPPING = [
    # iarnet-1 的 providers
    ("provider-i1-p1", "172.30.0.20", "iarnet-1"),
    ("provider-i1-p2", "172.30.0.21", "iarnet-1"),
    ("provider-i1-p3", "172.30.0.22", "iarnet-1"),
    
    # iarnet-2 的 providers
    ("provider-i2-p1", "172.30.0.23", "iarnet-2"),
    ("provider-i2-p2", "172.30.0.24", "iarnet-2"),
    ("provider-i2-p3", "172.30.0.25", "iarnet-2"),
    
    # iarnet-3 的 providers
    ("provider-i3-p1", "172.30.0.26", "iarnet-3"),
    ("provider-i3-p2", "172.30.0.27", "iarnet-3"),
    ("provider-i3-p3", "172.30.0.28", "iarnet-3"),
    
    # iarnet-4 的 providers
    ("provider-i4-p1", "172.30.0.29", "iarnet-4"),
    ("provider-i4-p2", "172.30.0.30", "iarnet-4"),
    ("provider-i4-p3", "172.30.0.31", "iarnet-4"),
    
    # iarnet-5 的 providers
    ("provider-i5-p1", "172.30.0.32", "iarnet-5"),
    ("provider-i5-p2", "172.30.0.33", "iarnet-5"),
    ("provider-i5-p3", "172.30.0.34", "iarnet-5"),
    
    # iarnet-6 的 providers
    ("provider-i6-p1", "172.30.0.35", "iarnet-6"),
    ("provider-i6-p2", "172.30.0.36", "iarnet-6"),
    ("provider-i6-p3", "172.30.0.37", "iarnet-6"),
    
    # iarnet-7 的 providers
    ("provider-i7-p1", "172.30.0.38", "iarnet-7"),
    ("provider-i7-p2", "172.30.0.39", "iarnet-7"),
    ("provider-i7-p3", "172.30.0.40", "iarnet-7"),
    
    # iarnet-8 的 providers（虽然 docker-compose 中没有 iarnet-8，但这些 provider 可以分配给任意节点）
    ("provider-i8-p1", "172.30.0.41", "iarnet-8"),
    ("provider-i8-p2", "172.30.0.42", "iarnet-8"),
    ("provider-i8-p3", "172.30.0.43", "iarnet-8"),
]

# 所有 provider 使用统一端口
PROVIDER_PORT = 50051

def main():
    script_dir = Path(__file__).parent.absolute()
    output_dir = script_dir / "provider-import-csvs"
    output_dir.mkdir(parents=True, exist_ok=True)
    
    # 按 iarnet 节点分组
    iarnet_providers = {}
    for provider_name, ip, iarnet_node in PROVIDER_MAPPING:
        if iarnet_node not in iarnet_providers:
            iarnet_providers[iarnet_node] = []
        iarnet_providers[iarnet_node].append((provider_name, ip))
    
    print("生成 provider 批量导入 CSV 文件...")
    print("=" * 60)
    
    # 为每个 iarnet 节点生成 CSV 文件
    for iarnet_node in sorted(iarnet_providers.keys()):
        csv_file = output_dir / f"{iarnet_node}-providers.csv"
        
        with open(csv_file, 'w', encoding='utf-8', newline='') as f:
            writer = csv.writer(f)
            
            # CSV 格式：节点名称,地址:端口（首行就是数据，不是表头）
            for provider_name, ip in iarnet_providers[iarnet_node]:
                writer.writerow([provider_name, f"{ip}:{PROVIDER_PORT}"])
        
        print(f"✓ {iarnet_node}: {csv_file}")
        print(f"  包含 {len(iarnet_providers[iarnet_node])} 个 provider")
        for provider_name, ip in iarnet_providers[iarnet_node]:
            print(f"    - {provider_name}: {ip}:{PROVIDER_PORT}")
    
    print("=" * 60)
    print(f"完成！共生成 {len(iarnet_providers)} 个 CSV 文件")
    print(f"\nCSV 文件位置: {output_dir}")
    print(f"\n使用方式:")
    print(f"  1. 登录到各个 iarnet 节点的前端界面（http://localhost:300X）")
    print(f"  2. 进入资源管理页面")
    print(f"  3. 使用批量导入功能，上传对应的 CSV 文件")
    print(f"\n例如，为 iarnet-1 导入:")
    print(f"  上传文件: {output_dir / 'iarnet-1-providers.csv'}")

if __name__ == '__main__':
    main()
