#!/bin/bash
# 部署 iarnet-global 服务到虚拟机
# 用法: ./deploy_iarnet_global.sh [--build] [--restart]

set -e

# 配置
GLOBAL_VM_IP="192.168.100.5"
GLOBAL_VM_USER="ubuntu"
GLOBAL_PROJECT_DIR="/home/zhangyx/iarnet-global"
REMOTE_DIR="~/iarnet-global"

# 参数解析
BUILD=false
RESTART=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --build)
            BUILD=true
            shift
            ;;
        --restart)
            RESTART=true
            shift
            ;;
        *)
            echo "未知参数: $1"
            exit 1
            ;;
    esac
done

echo "=========================================="
echo "部署 iarnet-global 到 $GLOBAL_VM_IP"
echo "=========================================="

# 1. 本地构建（如果需要）
if [ "$BUILD" = true ]; then
    echo ""
    echo ">>> 本地构建后端..."
    cd "$GLOBAL_PROJECT_DIR"
    GOPROXY=https://goproxy.cn,direct go build -o iarnet-global ./cmd
    
    echo ""
    echo ">>> 本地构建前端..."
    cd "$GLOBAL_PROJECT_DIR/web"
    npm install
    npm run build
fi

# 2. 创建远程目录
echo ""
echo ">>> 创建远程目录..."
ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "mkdir -p $REMOTE_DIR/web"

# 3. 上传二进制文件
echo ""
echo ">>> 上传后端二进制文件..."
scp "$GLOBAL_PROJECT_DIR/iarnet-global" "$GLOBAL_VM_USER@$GLOBAL_VM_IP:$REMOTE_DIR/"

# 4. 上传配置文件
echo ""
echo ">>> 上传配置文件..."
scp "$GLOBAL_PROJECT_DIR/config.yaml" "$GLOBAL_VM_USER@$GLOBAL_VM_IP:$REMOTE_DIR/"

# 5. 上传前端构建产物
echo ""
echo ">>> 上传前端文件..."
scp -r "$GLOBAL_PROJECT_DIR/web/.next" "$GLOBAL_VM_USER@$GLOBAL_VM_IP:$REMOTE_DIR/web/"
# 如果 public 目录存在则上传
if [ -d "$GLOBAL_PROJECT_DIR/web/public" ]; then
    scp -r "$GLOBAL_PROJECT_DIR/web/public" "$GLOBAL_VM_USER@$GLOBAL_VM_IP:$REMOTE_DIR/web/"
fi
scp "$GLOBAL_PROJECT_DIR/web/package.json" "$GLOBAL_VM_USER@$GLOBAL_VM_IP:$REMOTE_DIR/web/"
scp "$GLOBAL_PROJECT_DIR/web/package-lock.json" "$GLOBAL_VM_USER@$GLOBAL_VM_IP:$REMOTE_DIR/web/"
# 上传 next.config（支持 .ts 和 .mjs）
if [ -f "$GLOBAL_PROJECT_DIR/web/next.config.ts" ]; then
    scp "$GLOBAL_PROJECT_DIR/web/next.config.ts" "$GLOBAL_VM_USER@$GLOBAL_VM_IP:$REMOTE_DIR/web/"
elif [ -f "$GLOBAL_PROJECT_DIR/web/next.config.mjs" ]; then
    scp "$GLOBAL_PROJECT_DIR/web/next.config.mjs" "$GLOBAL_VM_USER@$GLOBAL_VM_IP:$REMOTE_DIR/web/"
fi

# 6. 安装前端依赖
echo ""
echo ">>> 安装前端依赖..."
ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "cd $REMOTE_DIR/web && npm install --production"

# 7. 重启服务（如果需要）
if [ "$RESTART" = true ]; then
    echo ""
    echo ">>> 停止现有服务..."
    # 临时禁用 set -e，因为 pkill 在找不到进程时会返回非零状态
    set +e
    ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "pkill -f 'iarnet-global'"
    ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "pkill -f 'next start'"
    set -e
    sleep 2
    
    echo ""
    echo ">>> 启动后端服务..."
    ssh -f "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "cd $REMOTE_DIR && nohup ./iarnet-global --config=./config.yaml > iarnet-global.log 2>&1 &"
    sleep 3
    
    echo ""
    echo ">>> 启动前端服务..."
    ssh -f "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "cd $REMOTE_DIR/web && nohup npm run start -- -p 3000 > web.log 2>&1 &"
    sleep 3
    
    echo ""
    echo ">>> 检查服务状态..."
    ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "ps aux | grep -E 'iarnet-global|next' | grep -v grep || echo '服务未运行'"
fi

echo ""
echo "=========================================="
echo "部署完成!"
echo "=========================================="
echo ""
echo "服务地址:"
echo "  - 后端 RPC: $GLOBAL_VM_IP:50010"
echo "  - 后端 HTTP: http://$GLOBAL_VM_IP:8080"
echo "  - 前端 Web: http://$GLOBAL_VM_IP:3000"
echo ""
echo "查看日志:"
echo "  ssh $GLOBAL_VM_USER@$GLOBAL_VM_IP 'tail -f ~/iarnet-global/iarnet-global.log'"
echo "  ssh $GLOBAL_VM_USER@$GLOBAL_VM_IP 'tail -f ~/iarnet-global/web/web.log'"

