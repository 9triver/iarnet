#!/bin/bash
# 修复 K8s 镜像的 backing file 循环引用问题
# 使用方法: sudo bash fix_k8s_image_backing_file.sh

set -e

IMAGE_PATH="/var/lib/libvirt/images/ubuntu-22.04-cloud-k8s.qcow2"
BACKUP_PATH="${IMAGE_PATH}.backup.$(date +%Y%m%d_%H%M%S)"
FIXED_PATH="${IMAGE_PATH}.fixed"

echo "=========================================="
echo "修复 K8s 镜像的 backing file 问题"
echo "=========================================="

# 检查镜像是否存在
if [ ! -f "$IMAGE_PATH" ]; then
    echo "错误: 镜像文件不存在: $IMAGE_PATH"
    exit 1
fi

# 检查当前镜像的 backing file
echo "检查当前镜像状态..."
qemu-img info "$IMAGE_PATH" | grep -E "backing file|virtual size" || true

# 备份原镜像
echo ""
echo "1. 备份原镜像..."
cp "$IMAGE_PATH" "$BACKUP_PATH"
echo "  ✓ 备份完成: $BACKUP_PATH"

# 转换镜像，移除 backing file
echo ""
echo "2. 转换镜像，移除 backing file..."
echo "  这可能需要几分钟，请耐心等待..."
qemu-img convert -O qcow2 -B none -c "$BACKUP_PATH" "$FIXED_PATH"

# 验证修复后的镜像
echo ""
echo "3. 验证修复后的镜像..."
qemu-img info "$FIXED_PATH" | grep -E "backing file|virtual size" || true

# 检查是否还有 backing file
if qemu-img info "$FIXED_PATH" | grep -q "backing file"; then
    echo "  ⚠ 警告: 修复后的镜像仍有 backing file"
else
    echo "  ✓ 修复成功: backing file 已移除"
fi

# 替换原镜像
echo ""
echo "4. 替换原镜像..."
mv "$FIXED_PATH" "$IMAGE_PATH"
echo "  ✓ 镜像已修复"

echo ""
echo "=========================================="
echo "修复完成！"
echo "=========================================="
echo "原镜像备份: $BACKUP_PATH"
echo "修复后的镜像: $IMAGE_PATH"
echo ""
echo "现在可以使用此镜像创建新的虚拟机了。"

