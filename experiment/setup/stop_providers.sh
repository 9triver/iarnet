#!/bin/bash

# 停止所有 Provider 服务脚本
# 使用方法: ./stop_providers.sh

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
echo -e "${BLUE}  停止所有 Provider 服务${NC}"
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

# 停止所有 provider
docker-compose stop "${PROVIDERS[@]}"

if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✅ 所有 Provider 服务已停止!${NC}"
else
    echo -e "${RED}❌ Provider 服务停止失败!${NC}"
    exit 1
fi

