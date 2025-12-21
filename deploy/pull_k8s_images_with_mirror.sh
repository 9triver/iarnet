#!/bin/bash
# 使用国内镜像源拉取 Kubernetes 镜像（适用于 containerd）
# 在 K8s 节点上执行此脚本，预先拉取所有 kubeadm 所需的镜像

set -e

echo "=========================================="
echo "使用国内镜像源拉取 Kubernetes 镜像"
echo "=========================================="

# 定义国内镜像仓库（阿里云）
REGISTRY_MIRROR="registry.aliyuncs.com/google_containers"

# 获取 kubeadm 所需的镜像列表
echo "获取 kubeadm 镜像列表..."
IMAGES=$(kubeadm config images list 2>/dev/null)

if [ -z "$IMAGES" ]; then
    echo "错误: 无法获取镜像列表，请确保 kubeadm 已正确安装"
    exit 1
fi

echo "找到以下镜像需要拉取:"
echo "$IMAGES"
echo ""

# 循环每个镜像
for image in $IMAGES; do
    echo "处理镜像: $image"
    
    # 将 registry.k8s.io 或 k8s.gcr.io 替换为国内镜像仓库
    # 注意：coredns 镜像的路径在阿里云上是分开的，需要特别处理
    if [[ $image == *"coredns"* ]]; then
        # 阿里云上 coredns 的镜像路径为：registry.aliyuncs.com/google_containers/coredns
        # 原路径可能是：registry.k8s.io/coredns/coredns:v1.10.1
        # 转换为：registry.aliyuncs.com/google_containers/coredns:v1.10.1
        mirror_image=${image/registry.k8s.io\/coredns\/coredns/$REGISTRY_MIRROR/coredns}
        mirror_image=${mirror_image/k8s.gcr.io\/coredns\/coredns/$REGISTRY_MIRROR/coredns}
    else
        # 其他镜像直接替换仓库地址
        mirror_image=${image/registry.k8s.io/$REGISTRY_MIRROR}
        mirror_image=${mirror_image/k8s.gcr.io/$REGISTRY_MIRROR}
    fi
    
    echo "  从国内镜像仓库拉取: $mirror_image"
    
    # 使用 ctr (containerd 客户端) 拉取镜像
    # -n k8s.io 指定命名空间
    if sudo ctr -n k8s.io images pull "$mirror_image" 2>/dev/null; then
        echo "  ✓ 镜像拉取成功: $mirror_image"
        
        # 将镜像重新打标签为原始标签
        echo "  重新打标签为: $image"
        if sudo ctr -n k8s.io images tag "$mirror_image" "$image" 2>/dev/null; then
            echo "  ✓ 标签设置成功"
            
            # 删除国内镜像标签，保留原始标签即可
            sudo ctr -n k8s.io images remove "$mirror_image" 2>/dev/null || true
        else
            echo "  ⚠ 标签设置失败，但镜像已拉取"
        fi
    else
        echo "  ✗ 从国内镜像仓库拉取失败，尝试从原始仓库拉取..."
        # 如果国内镜像失败，尝试从原始仓库拉取
        if sudo ctr -n k8s.io images pull "$image" 2>/dev/null; then
            echo "  ✓ 从原始仓库拉取成功"
        else
            echo "  ✗ 镜像拉取失败: $image"
            echo "    继续处理下一个镜像..."
            continue
        fi
    fi
    
    echo ""
done

echo "=========================================="
echo "验证拉取的镜像..."
echo "=========================================="

# 验证镜像是否已拉取
echo "使用 ctr 查看镜像（推荐，无警告）:"
sudo ctr -n k8s.io images list | grep -E "k8s\.gcr\.io|registry\.k8s\.io" || echo "未找到镜像"

echo ""
echo "使用 crictl 查看镜像（可能有警告，可忽略）:"
# 配置 crictl 使用 containerd 端点，避免警告
export CONTAINER_RUNTIME_ENDPOINT=unix:///run/containerd/containerd.sock
sudo -E crictl images 2>/dev/null | grep -E "k8s\.gcr\.io|registry\.k8s\.io" || echo "未找到镜像（或使用上面的 ctr 命令查看）"

echo ""
echo "=========================================="
echo "镜像拉取完成！"
echo "=========================================="
echo ""
echo "现在可以使用 kubeadm init 初始化集群，镜像已预先拉取。"
echo "或者导出此虚拟机为镜像，新创建的虚拟机将包含这些镜像。"

