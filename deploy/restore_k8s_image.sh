#!/bin/bash
# 恢复 K8s 镜像的备份

IMAGE_PATH="/var/lib/libvirt/images/ubuntu-22.04-cloud-k8s.qcow2"

echo "可用的备份文件:"
ls -lh ${IMAGE_PATH}.backup.* 2>/dev/null | awk '{print $9, "(" $5 ")"}'

if [ -z "$(ls ${IMAGE_PATH}.backup.* 2>/dev/null)" ]; then
    echo "错误: 没有找到备份文件"
    exit 1
fi

echo ""
echo "请选择要恢复的备份文件（输入完整路径）:"
read -r backup_file

if [ ! -f "$backup_file" ]; then
    echo "错误: 备份文件不存在: $backup_file"
    exit 1
fi

echo "恢复镜像..."
sudo cp "$backup_file" "$IMAGE_PATH"
echo "✓ 恢复完成"

echo ""
echo "验证镜像:"
qemu-img info "$IMAGE_PATH" | head -10

