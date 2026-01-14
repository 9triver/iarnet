#!/bin/bash
# 将 runner 镜像同步到 demo-iarnet-1 的 dind

set -e

# 配置
DIND_HOST="localhost"
DIND_PORT="23771"
DIND_URL="tcp://${DIND_HOST}:${DIND_PORT}"

# Runner 镜像
RUNNER_IMAGE="iarnet/runner:python_3.11-latest"

echo "同步 runner 镜像到 demo-iarnet-1 的 dind..."
echo "镜像: $RUNNER_IMAGE"
echo "目标: $DIND_URL"
echo ""

# 检查宿主机是否有该镜像
if ! docker images "$RUNNER_IMAGE" --format "{{.Repository}}:{{.Tag}}" | grep -q "^${RUNNER_IMAGE}$"; then
    echo "错误: 宿主机上不存在镜像 $RUNNER_IMAGE"
    echo "请先构建或拉取该镜像"
    exit 1
fi

# 检查 dind 是否可访问
echo "检查 dind 连接..."
if ! docker -H "$DIND_URL" info >/dev/null 2>&1; then
    echo "错误: 无法连接到 dind ($DIND_URL)"
    echo "请确保 demo-iarnet-1 容器正在运行"
    exit 1
fi
echo "✓ dind 连接成功"

# 检查 dind 中是否已有该镜像
if docker -H "$DIND_URL" images "$RUNNER_IMAGE" --format "{{.Repository}}:{{.Tag}}" | grep -q "^${RUNNER_IMAGE}$"; then
    echo "镜像 $RUNNER_IMAGE 已存在于 dind 中"
    read -p "是否重新同步？(y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "跳过同步"
        exit 0
    fi
fi

# 导出镜像
echo ""
echo "导出镜像..."
TEMP_FILE=$(mktemp)
docker save "$RUNNER_IMAGE" -o "$TEMP_FILE"

# 导入到 dind
echo "导入到 dind..."
docker -H "$DIND_URL" load -i "$TEMP_FILE"

# 清理临时文件
rm -f "$TEMP_FILE"

echo ""
echo "✓ 镜像 $RUNNER_IMAGE 同步完成！"

# 验证
echo ""
echo "验证镜像..."
docker -H "$DIND_URL" images "$RUNNER_IMAGE"
