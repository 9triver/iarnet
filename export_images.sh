#!/bin/bash

# Iarnet 镜像导出脚本
# 使用方法: ./export_images.sh [输出目录] [是否压缩]
#
# 示例：
#   ./export_images.sh                          # 导出到当前目录，不压缩
#   ./export_images.sh /path/to/backup          # 导出到指定目录，不压缩
#   ./export_images.sh /path/to/backup compress # 导出到指定目录并压缩

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 默认镜像标签配置
IARNET_IMAGE="${IARNET_IMAGE:-iarnet:latest}"
RUNNER_IMAGE="${RUNNER_IMAGE:-iarnet/runner:python_3.11-latest}"
COMPONENT_IMAGE="${COMPONENT_IMAGE:-iarnet/component:python_3.11-latest}"
DOCKER_PROVIDER_IMAGE="${DOCKER_PROVIDER_IMAGE:-iarnet/provider:docker}"

# 获取参数
OUTPUT_DIR="${1:-images}"
COMPRESS="${2:-}"

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 创建输出目录
mkdir -p "$OUTPUT_DIR"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Iarnet 镜像导出工具${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}错误: Docker daemon 未运行${NC}"
    exit 1
fi

# 定义要导出的镜像列表
declare -a IMAGES=(
    "$IARNET_IMAGE"
    "$RUNNER_IMAGE"
    "$COMPONENT_IMAGE"
    "$DOCKER_PROVIDER_IMAGE"
)

# 定义镜像名称映射（用于生成文件名）
declare -A IMAGE_NAMES=(
    ["$IARNET_IMAGE"]="iarnet"
    ["$RUNNER_IMAGE"]="runner"
    ["$COMPONENT_IMAGE"]="component"
    ["$DOCKER_PROVIDER_IMAGE"]="docker-provider"
)

# 检查镜像是否存在
echo -e "${YELLOW}检查镜像是否存在...${NC}"
MISSING_IMAGES=()
for img in "${IMAGES[@]}"; do
    if ! docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${img}$"; then
        MISSING_IMAGES+=("$img")
        echo -e "${RED}  ✗ 镜像不存在: $img${NC}"
    else
        echo -e "${GREEN}  ✓ 镜像存在: $img${NC}"
    fi
done

if [ ${#MISSING_IMAGES[@]} -gt 0 ]; then
    echo ""
    echo -e "${RED}错误: 以下镜像不存在，请先构建：${NC}"
    for img in "${MISSING_IMAGES[@]}"; do
        echo -e "${RED}  - $img${NC}"
    done
    echo ""
    echo -e "${YELLOW}构建提示:${NC}"
    echo -e "${YELLOW}  - iarnet: ./build.sh${NC}"
    echo -e "${YELLOW}  - runner: cd containers/runner/python && ./build.sh${NC}"
    echo -e "${YELLOW}  - component: cd containers/component/python && ./build.sh${NC}"
    echo -e "${YELLOW}  - docker provider: cd providers/docker && ./build.sh${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}输出目录: $OUTPUT_DIR${NC}"
if [ "$COMPRESS" = "compress" ]; then
    echo -e "${YELLOW}压缩模式: 启用${NC}"
else
    echo -e "${YELLOW}压缩模式: 禁用${NC}"
fi
echo ""

# 导出镜像
SUCCESS_COUNT=0
FAILED_COUNT=0
TOTAL_SIZE=0

for img in "${IMAGES[@]}"; do
    IMAGE_NAME="${IMAGE_NAMES[$img]}"
    
    # 生成文件名（替换特殊字符）
    SAFE_NAME=$(echo "$img" | sed 's/[\/:]/-/g')
    
    if [ "$COMPRESS" = "compress" ]; then
        OUTPUT_FILE="$OUTPUT_DIR/${IMAGE_NAME}-${SAFE_NAME}.tar.gz"
        echo -e "${YELLOW}正在导出并压缩: $img${NC}"
        echo -e "${YELLOW}  → $OUTPUT_FILE${NC}"
        
        if docker save "$img" | gzip > "$OUTPUT_FILE"; then
            FILE_SIZE=$(du -h "$OUTPUT_FILE" | cut -f1)
            echo -e "${GREEN}  ✓ 导出成功 (大小: $FILE_SIZE)${NC}"
            SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        else
            echo -e "${RED}  ✗ 导出失败${NC}"
            FAILED_COUNT=$((FAILED_COUNT + 1))
            rm -f "$OUTPUT_FILE"
        fi
    else
        OUTPUT_FILE="$OUTPUT_DIR/${IMAGE_NAME}-${SAFE_NAME}.tar"
        echo -e "${YELLOW}正在导出: $img${NC}"
        echo -e "${YELLOW}  → $OUTPUT_FILE${NC}"
        
        if docker save -o "$OUTPUT_FILE" "$img"; then
            FILE_SIZE=$(du -h "$OUTPUT_FILE" | cut -f1)
            echo -e "${GREEN}  ✓ 导出成功 (大小: $FILE_SIZE)${NC}"
            SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        else
            echo -e "${RED}  ✗ 导出失败${NC}"
            FAILED_COUNT=$((FAILED_COUNT + 1))
            rm -f "$OUTPUT_FILE"
        fi
    fi
    echo ""
done

# 汇总结果
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  导出完成${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "${GREEN}成功: $SUCCESS_COUNT${NC}"
if [ $FAILED_COUNT -gt 0 ]; then
    echo -e "${RED}失败: $FAILED_COUNT${NC}"
fi
echo ""
echo -e "${YELLOW}导出文件列表:${NC}"
ls -lh "$OUTPUT_DIR"/*.tar* 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}'
echo ""

# 显示导入命令
echo -e "${YELLOW}导入镜像命令:${NC}"
if [ "$COMPRESS" = "compress" ]; then
    echo -e "${BLUE}# 导入压缩镜像${NC}"
    for img in "${IMAGES[@]}"; do
        IMAGE_NAME="${IMAGE_NAMES[$img]}"
        SAFE_NAME=$(echo "$img" | sed 's/[\/:]/-/g')
        OUTPUT_FILE="$OUTPUT_DIR/${IMAGE_NAME}-${SAFE_NAME}.tar.gz"
        echo "  gunzip -c $OUTPUT_FILE | docker load"
    done
else
    echo -e "${BLUE}# 导入镜像${NC}"
    for img in "${IMAGES[@]}"; do
        IMAGE_NAME="${IMAGE_NAMES[$img]}"
        SAFE_NAME=$(echo "$img" | sed 's/[\/:]/-/g')
        OUTPUT_FILE="$OUTPUT_DIR/${IMAGE_NAME}-${SAFE_NAME}.tar"
        echo "  docker load -i $OUTPUT_FILE"
    done
fi
echo ""

if [ $FAILED_COUNT -eq 0 ]; then
    echo -e "${GREEN}✅ 所有镜像导出成功!${NC}"
    exit 0
else
    echo -e "${RED}❌ 部分镜像导出失败!${NC}"
    exit 1
fi

