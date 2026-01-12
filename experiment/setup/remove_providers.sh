#!/bin/bash

# 移除所有 Provider 容器脚本
# 使用方法: ./remove_providers.sh
# 注意: 此脚本会删除容器，但不会删除数据卷

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  移除所有 Provider 容器${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 定义所有 provider 服务名称
PROVIDERS=(
    "provider-i1-p1"
    "provider-i1-p2"
    "provider-i1-p3"
    "provider-i2-p1"
    "provider-i2-p2"
    "provider-i2-p3"
    "provider-i3-p1"
    "provider-i3-p2"
    "provider-i3-p3"
    "provider-i4-p1"
    "provider-i4-p2"
    "provider-i4-p3"
    "provider-i5-p1"
    "provider-i5-p2"
    "provider-i5-p3"
    "provider-i6-p1"
    "provider-i6-p2"
    "provider-i6-p3"
    "provider-i7-p1"
    "provider-i7-p2"
    "provider-i7-p3"
    "provider-i8-p1"
    "provider-i8-p2"
    "provider-i8-p3"
)

# 确认操作
echo -e "${YELLOW}警告: 此操作将删除所有 Provider 容器!${NC}"
echo -e "${YELLOW}容器数据将被删除，但数据卷会保留。${NC}"
echo ""
read -p "确认继续? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo -e "${YELLOW}操作已取消${NC}"
    exit 0
fi

echo ""

# 先停止所有 provider（如果正在运行）
echo -e "${YELLOW}停止所有 Provider 容器...${NC}"
docker-compose stop "${PROVIDERS[@]}" 2>/dev/null || true

# 移除所有 provider 容器
echo -e "${YELLOW}移除所有 Provider 容器...${NC}"
for provider in "${PROVIDERS[@]}"; do
    echo -n "Removing $provider ... "
    if docker-compose rm -f "$provider" > /dev/null 2>&1; then
        echo -e "${GREEN}done${NC}"
    else
        # 如果 docker-compose rm 失败，尝试直接使用 docker rm
        if docker rm -f "$provider" > /dev/null 2>&1; then
            echo -e "${GREEN}done${NC}"
        else
            echo -e "${YELLOW}not found${NC}"
        fi
    fi
done

if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✅ 所有 Provider 容器已移除!${NC}"
else
    echo -e "${RED}❌ Provider 容器移除失败!${NC}"
    exit 1
fi
