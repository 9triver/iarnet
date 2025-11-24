# 多阶段构建 Dockerfile for iarnet
# 在同一个容器中启动前后端，同时映射宿主机 Docker socket

# ============================================================================
# 阶段 1: 构建 Go 后端
# ============================================================================
FROM golang:1.25-alpine AS backend-builder

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.org

# 安装必要的工具和 ZeroMQ 依赖（goczmq 需要 CGO）
RUN apk add --no-cache git ca-certificates tzdata \
    gcc musl-dev \
    zeromq-dev czmq-dev pkgconfig

# 设置工作目录
WORKDIR /build

# 复制整个项目
COPY . /build/iarnet

# 切换到项目根目录
WORKDIR /build/iarnet

# 下载依赖
RUN go mod download

# 构建后端应用（启用 CGO，因为 goczmq 需要）
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s' \
    -a \
    -o iarnet ./cmd/main.go

# ============================================================================
# 阶段 2: 构建 Next.js 前端
# ============================================================================
FROM node:20-alpine AS frontend-builder

# 设置工作目录
WORKDIR /build

# 复制前端项目
COPY web /build/web

# 切换到前端目录
WORKDIR /build/web

# 安装依赖
RUN npm install --legacy-peer-deps

# 构建前端（生产模式）
RUN npm run build

# ============================================================================
# 阶段 3: 运行阶段
# ============================================================================
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    nodejs \
    npm \
    docker-cli \
    netcat-openbsd \
    zeromq \
    czmq  # ZeroMQ 运行时库（goczmq 需要）

# 创建非 root 用户
RUN addgroup -S appuser && adduser -S -G appuser appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制后端二进制文件
COPY --from=backend-builder /build/iarnet/iarnet /app/iarnet

# 从构建阶段复制前端构建产物
COPY --from=frontend-builder /build/web/.next /app/web/.next
COPY --from=frontend-builder /build/web/public /app/web/public
COPY --from=frontend-builder /build/web/package.json /app/web/package.json
COPY --from=frontend-builder /build/web/next.config.mjs /app/web/next.config.mjs
COPY --from=frontend-builder /build/web/node_modules /app/web/node_modules

# 复制配置文件
COPY --from=backend-builder /build/iarnet/config.yaml /app/config.yaml

# 创建数据目录
RUN mkdir -p /app/data /app/workspaces && \
    chown -R appuser:appuser /app

# 创建启动脚本
RUN cat > /app/entrypoint.sh << 'EOF'
#!/bin/sh
set -e

# 设置环境变量
export BACKEND_URL="${BACKEND_URL:-http://localhost:8083}"
export DOCKER_HOST="${DOCKER_HOST:-unix:///var/run/docker.sock}"

# 检查 Docker socket 是否存在
if [ ! -S /var/run/docker.sock ]; then
    echo "警告: Docker socket 不存在，请确保挂载了 /var/run/docker.sock"
fi

# 启动后端服务（后台运行，输出到 stdout/stderr，这样 docker logs 可以看到）
echo "[启动脚本] 启动 iarnet 后端服务..."
cd /app
/app/iarnet -config /app/config.yaml &
BACKEND_PID=$!

# 等待后端启动
echo "[启动脚本] 等待后端服务启动..."
for i in $(seq 1 30); do
    if nc -z localhost 8083 2>/dev/null; then
        echo "[启动脚本] 后端服务已启动"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "[启动脚本] 警告: 后端服务启动超时，继续启动前端..."
    fi
    sleep 1
done

# 启动前端服务（后台运行，输出到 stdout/stderr，这样 docker logs 可以看到）
echo "[启动脚本] 启动 iarnet 前端服务..."
cd /app/web
npm start &
FRONTEND_PID=$!

# 定义清理函数
cleanup() {
    echo "[启动脚本] 收到退出信号，正在停止服务..."
    kill $BACKEND_PID $FRONTEND_PID 2>/dev/null || true
    wait $BACKEND_PID $FRONTEND_PID 2>/dev/null || true
    exit 0
}

# 捕获退出信号
trap cleanup SIGTERM SIGINT

# 等待进程退出（如果任一进程退出，脚本也会退出）
wait $BACKEND_PID $FRONTEND_PID
EOF

RUN chmod +x /app/entrypoint.sh

# 暴露端口
# 8083: 后端 HTTP API
# 3000: 前端 Next.js
EXPOSE 8083 3000

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD pgrep -f iarnet && pgrep -f "next start" || exit 1

# 使用启动脚本作为入口点
ENTRYPOINT ["/app/entrypoint.sh"]

