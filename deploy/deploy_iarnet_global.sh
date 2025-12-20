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
if ! ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "mkdir -p $REMOTE_DIR/web && chmod 755 $REMOTE_DIR"; then
    echo "错误: 无法创建远程目录 $REMOTE_DIR"
    exit 1
fi

# 验证远程目录是否存在且可写
echo ">>> 验证远程目录..."
if ! ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "test -w $REMOTE_DIR"; then
    echo "错误: 远程目录 $REMOTE_DIR 不可写"
    exit 1
fi

# 3. 检查本地文件是否存在
if [ ! -f "$GLOBAL_PROJECT_DIR/iarnet-global" ]; then
    echo "错误: 二进制文件 $GLOBAL_PROJECT_DIR/iarnet-global 不存在"
    echo "请先运行: ./deploy_iarnet_global.sh --build"
    exit 1
fi

# 4. 停止正在运行的服务（如果存在）
echo ""
echo ">>> 停止正在运行的服务..."
set +e  # 临时禁用 set -e，因为 pkill 在找不到进程时会返回非零状态

# 停止后端服务
if ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "pgrep -f 'iarnet-global'" > /dev/null 2>&1; then
    echo "  停止 iarnet-global 后端服务..."
    ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "pkill -f 'iarnet-global'"
    sleep 1
else
    echo "  未找到运行中的 iarnet-global 进程"
fi

# 停止前端服务 - 使用多种方式确保停止
if ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "pgrep -f 'next' || pgrep -f 'node.*3000' || lsof -ti:3000" > /dev/null 2>&1; then
    echo "  停止前端服务..."
    # 尝试多种方式停止前端服务
    ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "pkill -f 'next' || true"
    ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "pkill -f 'node.*3000' || true"
    # 如果进程还在运行，尝试通过端口杀死
    ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "lsof -ti:3000 | xargs kill -9 2>/dev/null || true"
    sleep 1
else
    echo "  未找到运行中的前端服务"
fi

set -e  # 重新启用 set -e
sleep 2  # 等待进程完全停止

# 5. 上传二进制文件
echo ""
echo ">>> 上传后端二进制文件..."
if ! scp "$GLOBAL_PROJECT_DIR/iarnet-global" "$GLOBAL_VM_USER@$GLOBAL_VM_IP:$REMOTE_DIR/"; then
    echo "错误: 上传二进制文件失败"
    echo "请检查:"
    echo "  1. 远程目录是否存在: ssh $GLOBAL_VM_USER@$GLOBAL_VM_IP 'ls -ld $REMOTE_DIR'"
    echo "  2. 远程目录权限: ssh $GLOBAL_VM_USER@$GLOBAL_VM_IP 'ls -ld $REMOTE_DIR'"
    echo "  3. SSH 连接是否正常: ssh $GLOBAL_VM_USER@$GLOBAL_VM_IP 'echo test'"
    exit 1
fi

# 设置可执行权限
ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "chmod +x $REMOTE_DIR/iarnet-global"

# 6. 上传配置文件
echo ""
echo ">>> 上传配置文件..."
scp "$GLOBAL_PROJECT_DIR/config.yaml" "$GLOBAL_VM_USER@$GLOBAL_VM_IP:$REMOTE_DIR/"

# 7. 上传前端构建产物
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

# 8. 安装前端依赖
echo ""
echo ">>> 安装前端依赖..."
ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "cd $REMOTE_DIR/web && npm install --production"

# 9. 启动服务（如果需要）
if [ "$RESTART" = true ]; then
    echo ""
    echo ">>> 启动服务..."
    
    echo ""
    echo ">>> 启动后端服务..."
    # 使用后台 SSH 连接启动服务，避免阻塞
    ssh -f "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "cd $REMOTE_DIR && nohup ./iarnet-global --config=./config.yaml > iarnet-global.log 2>&1 < /dev/null &" || true
    sleep 3
    
    # 验证后端服务是否启动
    if ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "pgrep -f 'iarnet-global'" > /dev/null 2>&1; then
        echo "  ✓ 后端服务已启动"
    else
        echo "  ✗ 后端服务启动失败，请查看日志: ssh $GLOBAL_VM_USER@$GLOBAL_VM_IP 'tail -20 $REMOTE_DIR/iarnet-global.log'"
    fi
    
    echo ""
    echo ">>> 启动前端服务..."
    # 检查 .next 目录是否存在
    if ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "test -d $REMOTE_DIR/web/.next"; then
        echo "  找到 .next 构建目录，启动生产服务..."
        # 使用后台 SSH 连接启动服务，避免阻塞
        ssh -f "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "cd $REMOTE_DIR/web && NODE_ENV=production nohup npm run start -- -p 3000 > web.log 2>&1 < /dev/null &" || true
    else
        echo "  警告: 未找到 .next 构建目录，前端服务可能无法启动"
        echo "  请确保已运行构建步骤: ./deploy_iarnet_global.sh --build"
    fi
    sleep 5  # 前端服务启动需要更长时间
    
    # 验证前端服务是否启动
    if ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "pgrep -f 'next' || pgrep -f 'node.*3000' || lsof -ti:3000" > /dev/null 2>&1; then
        echo "  ✓ 前端服务已启动"
    else
        echo "  ✗ 前端服务启动失败，请查看日志: ssh $GLOBAL_VM_USER@$GLOBAL_VM_IP 'tail -20 $REMOTE_DIR/web/web.log'"
        echo "  尝试手动启动: ssh $GLOBAL_VM_USER@$GLOBAL_VM_IP 'cd $REMOTE_DIR/web && npm run start -- -p 3000'"
    fi
    
    echo ""
    echo ">>> 检查服务状态..."
    echo "后端服务:"
    ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "ps aux | grep 'iarnet-global' | grep -v grep || echo '  未运行'"
    echo "前端服务:"
    ssh "$GLOBAL_VM_USER@$GLOBAL_VM_IP" "ps aux | grep -E 'next|node.*3000' | grep -v grep || echo '  未运行'"
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
echo "  ssh $GLOBAL_VM_USER@$GLOBAL_VM_IP 'tail -f $REMOTE_DIR/iarnet-global.log'"
echo "  ssh $GLOBAL_VM_USER@$GLOBAL_VM_IP 'tail -f $REMOTE_DIR/web/web.log'"

