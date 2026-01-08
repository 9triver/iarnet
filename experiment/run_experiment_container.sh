#!/bin/bash

# 实验容器启动脚本
# 使用方法: ./run_experiment_container.sh [命令]
# 如果不提供命令，则进入交互式 shell

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IARNET_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 容器名称
CONTAINER_NAME="iarnet-experiment"

# 镜像名称
IMAGE_NAME="iarnet-experiment:latest"

# 固定 IP 地址
FIXED_IP="172.30.0.128"

# 检查 IP 是否已被占用
check_ip_available() {
    local ip=$1
    local network=$2
    if [ -z "$network" ] || [ -z "$ip" ]; then
        return 1
    fi
    # 检查网络中是否已有容器使用该 IP
    docker network inspect "$network" --format '{{range $k, $v := .Containers}}{{$v.IPv4Address}}{{"\n"}}{{end}}' 2>/dev/null | grep -q "^${ip}/" && return 1 || return 0
}

# 尝试连接网络，优先使用固定 IP，失败则自动分配
connect_to_network() {
    local network=$1
    local container=$2
    local preferred_ip=$3
    
    if [ -z "$network" ] || [ -z "$container" ]; then
        return 1
    fi
    
    # 如果指定了固定 IP 且可用，尝试使用
    if [ -n "$preferred_ip" ] && check_ip_available "$preferred_ip" "$network"; then
        echo -e "${YELLOW}使用固定 IP: $preferred_ip${NC}"
        if docker network connect --ip "$preferred_ip" --alias experiment-runner "$network" "$container" 2>/dev/null; then
            return 0
        fi
        echo -e "${YELLOW}固定 IP 连接失败，尝试自动分配 IP...${NC}"
    fi
    
    # 尝试自动分配 IP
    if docker network connect --alias experiment-runner "$network" "$container" 2>/dev/null; then
        return 0
    fi
    
    return 1
}

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Iarnet 实验容器${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}错误: Docker daemon 未运行${NC}"
    exit 1
fi

# 构建镜像（如果不存在）
if ! docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${IMAGE_NAME}$"; then
    echo -e "${YELLOW}构建实验容器镜像...${NC}"
    docker build -t "$IMAGE_NAME" -f "$SCRIPT_DIR/Dockerfile" "$IARNET_ROOT"
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ 镜像构建成功!${NC}"
    else
        echo -e "${RED}❌ 镜像构建失败!${NC}"
        exit 1
    fi
    echo ""
fi

# 查找实验网络
NETWORK_NAME=""
if docker network ls --format "{{.Name}}" | grep -q "iarnet.*testing.*network"; then
    NETWORK_NAME=$(docker network ls --format "{{.Name}}" | grep "iarnet.*testing.*network" | head -1)
elif docker network ls --format "{{.Name}}" | grep -q "iarnet-testing-network"; then
    NETWORK_NAME="iarnet-testing-network"
fi

# 检查容器是否已存在
if docker ps -a --format "{{.Names}}" | grep -q "^${CONTAINER_NAME}$"; then
    echo -e "${YELLOW}容器已存在${NC}"
    
    # 检查端口映射是否正确（4000:3000）
    PORT_MAPPING=$(docker inspect "$CONTAINER_NAME" --format '{{range $p, $conf := .HostConfig.PortBindings}}{{$p}}={{index $conf 0 "HostPort"}}{{end}}' 2>/dev/null | grep -q "3000/tcp=4000" && echo "ok" || echo "")
    if [ -z "$PORT_MAPPING" ]; then
        echo -e "${YELLOW}⚠️  容器未配置端口映射 4000:3000${NC}"
        echo -e "${YELLOW}提示: 如需使用端口映射，请先删除容器: docker rm -f $CONTAINER_NAME${NC}"
    fi
    
    # 如果网络存在且容器未连接到网络，则连接
    if [ -n "$NETWORK_NAME" ]; then
        if ! docker inspect "$CONTAINER_NAME" --format '{{range $net, $conf := .NetworkSettings.Networks}}{{$net}}{{end}}' | grep -q "$NETWORK_NAME"; then
            echo -e "${YELLOW}连接到实验网络: $NETWORK_NAME${NC}"
            if ! connect_to_network "$NETWORK_NAME" "$CONTAINER_NAME" "$FIXED_IP"; then
                echo -e "${RED}❌ 无法连接到网络!${NC}"
                echo -e "${RED}提示: 请检查网络状态和容器状态${NC}"
            fi
        else
            # 检查当前 IP 是否为固定 IP
            CURRENT_IP=$(docker inspect "$CONTAINER_NAME" --format "{{range \$net, \$conf := .NetworkSettings.Networks}}{{if eq \$net \"$NETWORK_NAME\"}}{{.IPAddress}}{{end}}{{end}}")
            if [ -n "$CURRENT_IP" ] && [ "$CURRENT_IP" != "$FIXED_IP" ] && check_ip_available "$FIXED_IP" "$NETWORK_NAME"; then
                echo -e "${YELLOW}当前 IP ($CURRENT_IP) 与固定 IP ($FIXED_IP) 不一致，尝试重新连接...${NC}"
                docker network disconnect "$NETWORK_NAME" "$CONTAINER_NAME" 2>/dev/null || true
                if ! connect_to_network "$NETWORK_NAME" "$CONTAINER_NAME" "$FIXED_IP"; then
                    echo -e "${YELLOW}重新连接失败，保持当前 IP${NC}"
                fi
            fi
        fi
    fi
    
    # 启动容器（如果未运行）
    if ! docker ps --format "{{.Names}}" | grep -q "^${CONTAINER_NAME}$"; then
        echo -e "${YELLOW}启动容器...${NC}"
        docker start "$CONTAINER_NAME" > /dev/null 2>&1 || true
    fi
else
    echo -e "${YELLOW}创建新容器...${NC}"
    
    # 创建容器并挂载 iarnet 目录
    # 如果网络存在，创建时直接连接到网络（不使用固定 IP，稍后处理）
    CREATE_ARGS=(
        --name "$CONTAINER_NAME"
        -p 4000:3000
        -v "$IARNET_ROOT:/workspace/iarnet:rw"
        -w /workspace/iarnet/experiment
        -it
    )
    
    # 如果网络存在，创建时直接连接到网络
    if [ -n "$NETWORK_NAME" ]; then
        CREATE_ARGS+=(--network "$NETWORK_NAME")
    fi
    
    CREATE_ARGS+=("$IMAGE_NAME" /bin/bash)
    
    docker create "${CREATE_ARGS[@]}"
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ 容器创建成功!${NC}"
        
        # 如果网络存在，处理固定 IP 设置
        if [ -n "$NETWORK_NAME" ]; then
            # 如果固定 IP 可用，尝试设置固定 IP
            if check_ip_available "$FIXED_IP" "$NETWORK_NAME"; then
                echo -e "${YELLOW}尝试设置固定 IP: $FIXED_IP...${NC}"
                # 断开网络（如果已连接）
                docker network disconnect "$NETWORK_NAME" "$CONTAINER_NAME" 2>/dev/null || true
                # 使用固定 IP 重新连接
                if docker network connect --ip "$FIXED_IP" --alias experiment-runner "$NETWORK_NAME" "$CONTAINER_NAME" 2>/dev/null; then
                    echo -e "${GREEN}✅ 固定 IP 设置成功!${NC}"
                else
                    echo -e "${YELLOW}固定 IP 设置失败，使用自动分配 IP${NC}"
                    # 重新连接（自动分配 IP）
                    docker network connect --alias experiment-runner "$NETWORK_NAME" "$CONTAINER_NAME" 2>/dev/null || {
                        echo -e "${RED}❌ 无法连接到网络!${NC}"
                        echo -e "${RED}提示: 请检查网络状态${NC}"
                        docker rm "$CONTAINER_NAME" 2>/dev/null || true
                        exit 1
                    }
                fi
            else
                # 固定 IP 不可用，使用自动分配的 IP（创建时已连接）
                echo -e "${YELLOW}固定 IP ($FIXED_IP) 不可用，使用自动分配 IP${NC}"
                # 如果创建时未连接（网络不存在时创建），尝试连接
                if ! docker inspect "$CONTAINER_NAME" --format '{{range $net, $conf := .NetworkSettings.Networks}}{{$net}}{{end}}' | grep -q "$NETWORK_NAME"; then
                    docker network connect --alias experiment-runner "$NETWORK_NAME" "$CONTAINER_NAME" 2>/dev/null || {
                        echo -e "${RED}❌ 无法连接到网络!${NC}"
                        echo -e "${RED}提示: 请检查网络状态${NC}"
                        docker rm "$CONTAINER_NAME" 2>/dev/null || true
                        exit 1
                    }
                fi
            fi
        else
            echo -e "${YELLOW}警告: 未找到实验网络，容器将无法访问 iarnet 服务${NC}"
            echo -e "${YELLOW}提示: 请先启动 docker-compose 服务以创建网络${NC}"
        fi
        
        docker start "$CONTAINER_NAME" > /dev/null 2>&1
    else
        echo -e "${RED}❌ 容器创建失败!${NC}"
        exit 1
    fi
fi

echo ""
echo -e "${GREEN}容器已启动: $CONTAINER_NAME${NC}"

# 显示容器 IP 信息
if [ -n "$NETWORK_NAME" ]; then
    ACTUAL_IP=$(docker inspect "$CONTAINER_NAME" --format "{{range \$net, \$conf := .NetworkSettings.Networks}}{{if eq \$net \"$NETWORK_NAME\"}}{{.IPAddress}}{{end}}{{end}}" 2>/dev/null || echo "")
    if [ -n "$ACTUAL_IP" ]; then
        echo -e "${GREEN}容器 IP: $ACTUAL_IP${NC}"
        if [ "$ACTUAL_IP" = "$FIXED_IP" ]; then
            echo -e "${GREEN}✅ 固定 IP 配置成功${NC}"
        else
            echo -e "${YELLOW}⚠️  当前 IP ($ACTUAL_IP) 与配置的固定 IP ($FIXED_IP) 不一致${NC}"
        fi
    fi
fi
echo ""

# 如果提供了命令，则执行命令；否则进入交互式 shell
if [ $# -gt 0 ]; then
    echo -e "${YELLOW}执行命令: $@${NC}"
    echo ""
    docker exec -it "$CONTAINER_NAME" "$@"
else
    echo -e "${YELLOW}进入容器交互式 shell...${NC}"
    echo -e "${YELLOW}工作目录: /workspace/iarnet/experiment${NC}"
    echo -e "${YELLOW}iarnet 源码: /workspace/iarnet${NC}"
    if [ -n "$NETWORK_NAME" ] && [ -n "$ACTUAL_IP" ]; then
        echo -e "${YELLOW}容器 IP: $ACTUAL_IP${NC}"
    fi
    echo ""
    echo -e "${BLUE}提示:${NC}"
    echo -e "${BLUE}  - 编译实验代码: go build -o experiment main.go${NC}"
    echo -e "${BLUE}  - 运行实验: ./experiment${NC}"
    echo -e "${BLUE}  - 或直接运行: go run main.go${NC}"
    echo ""
    docker exec -it "$CONTAINER_NAME" /bin/bash
fi

