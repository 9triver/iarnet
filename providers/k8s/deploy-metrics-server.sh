#!/bin/bash

# Metrics Server 部署脚本
# 部署禁用 TLS 验证的 metrics-server（适用于测试环境）

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}  Metrics Server 部署脚本 (禁用 TLS)    ${NC}"
echo -e "${YELLOW}========================================${NC}"

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
METRICS_YAML="$SCRIPT_DIR/metrics-server.yaml"

# 检查 kubectl 是否可用
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}错误: kubectl 未安装或不在 PATH 中${NC}"
    exit 1
fi

# 检查是否能连接到集群
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}错误: 无法连接到 Kubernetes 集群${NC}"
    echo "请确保 kubectl 已正确配置"
    exit 1
fi

echo -e "${GREEN}✓ kubectl 已连接到集群${NC}"

# 检查 metrics-server.yaml 是否存在
if [ ! -f "$METRICS_YAML" ]; then
    echo -e "${RED}错误: 找不到 metrics-server.yaml${NC}"
    echo "请确保在 providers/k8s 目录下运行此脚本"
    exit 1
fi

# 检查是否已部署 metrics-server
echo -e "${YELLOW}检查现有的 metrics-server 部署...${NC}"
if kubectl get deployment metrics-server -n kube-system &> /dev/null; then
    echo -e "${YELLOW}发现现有的 metrics-server 部署${NC}"
    read -p "是否删除并重新部署? (y/N): " confirm
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}删除现有的 metrics-server...${NC}"
        kubectl delete -f "$METRICS_YAML" --ignore-not-found=true
        sleep 3
    else
        echo -e "${YELLOW}跳过部署${NC}"
        exit 0
    fi
fi

# 部署 metrics-server
echo -e "${YELLOW}部署 metrics-server (禁用 TLS)...${NC}"
kubectl apply -f "$METRICS_YAML"

# 等待 metrics-server 就绪
echo -e "${YELLOW}等待 metrics-server 就绪...${NC}"
kubectl rollout status deployment/metrics-server -n kube-system --timeout=120s

# 验证部署
echo -e "${YELLOW}验证部署...${NC}"
sleep 10

# 检查 API 是否可用
echo -e "${YELLOW}检查 metrics API 是否可用...${NC}"
for i in {1..30}; do
    if kubectl get --raw /apis/metrics.k8s.io/v1beta1/nodes &> /dev/null; then
        echo -e "${GREEN}✓ Metrics API 已就绪${NC}"
        break
    fi
    echo "等待 metrics API 就绪... ($i/30)"
    sleep 2
done

# 测试获取 metrics
echo -e "${YELLOW}测试获取 Node Metrics...${NC}"
if kubectl top nodes 2>/dev/null; then
    echo -e "${GREEN}✓ Node Metrics 获取成功${NC}"
else
    echo -e "${YELLOW}! Node Metrics 可能还需要等待一段时间${NC}"
    echo "  请稍后手动运行: kubectl top nodes"
fi

echo ""
echo -e "${YELLOW}测试获取 Pod Metrics...${NC}"
if kubectl top pods -A 2>/dev/null | head -5; then
    echo -e "${GREEN}✓ Pod Metrics 获取成功${NC}"
else
    echo -e "${YELLOW}! Pod Metrics 可能还需要等待一段时间${NC}"
    echo "  请稍后手动运行: kubectl top pods -A"
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Metrics Server 部署完成!              ${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${YELLOW}使用说明:${NC}"
echo "  - 查看节点资源使用: kubectl top nodes"
echo "  - 查看 Pod 资源使用: kubectl top pods -A"
echo "  - 查看 metrics-server 状态: kubectl get pods -n kube-system -l k8s-app=metrics-server"
echo ""
echo -e "${YELLOW}注意:${NC}"
echo "  - 此部署禁用了 kubelet TLS 验证，仅适用于测试环境"
echo "  - 生产环境请使用正确配置的 TLS 证书"

