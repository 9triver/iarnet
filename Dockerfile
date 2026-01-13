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

# 将 monaco-editor 的静态资源复制到 public，以便在无网络环境下本地加载
RUN mkdir -p public/monaco-editor && \
    cp -r node_modules/monaco-editor/min/vs public/monaco-editor/vs

# 构建前端（生产模式）
RUN npm run build

# ============================================================================
# 阶段 3: 运行阶段
# ============================================================================
FROM docker:dind

# 安装必要的运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    nodejs \
    npm \
    docker-cli \
    netcat-openbsd \
    zeromq \
    czmq \
    git \
    coreutils

# 创建非 root 用户
RUN addgroup -S appuser && adduser -S -G appuser appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制后端二进制文件（直接设置所有权）
COPY --chown=appuser:appuser --from=backend-builder /build/iarnet/iarnet /app/iarnet

# 从构建阶段复制前端构建产物（直接设置所有权）
COPY --chown=appuser:appuser --from=frontend-builder /build/web/.next /app/web/.next
COPY --chown=appuser:appuser --from=frontend-builder /build/web/public /app/web/public
COPY --chown=appuser:appuser --from=frontend-builder /build/web/package.json /app/web/package.json
COPY --chown=appuser:appuser --from=frontend-builder /build/web/next.config.mjs /app/web/next.config.mjs
COPY --chown=appuser:appuser --from=frontend-builder /build/web/node_modules /app/web/node_modules

# 复制配置文件（直接设置所有权）
COPY --chown=appuser:appuser --from=backend-builder /build/iarnet/config.yaml /app/config.yaml

# 创建数据目录（只对新建的目录设置所有权，避免递归整个 /app）
RUN mkdir -p /app/data /app/workspaces && \
    chown appuser:appuser /app/data /app/workspaces

# 创建启动脚本
RUN cat > /app/entrypoint.sh << 'EOF'
#!/bin/sh
set -e

# 设置环境变量
export BACKEND_URL="${BACKEND_URL:-http://localhost:8083}"
export DOCKER_HOST="${DOCKER_HOST:-unix:///var/run/docker.sock}"
export DOCKER_TLS_CERTDIR=""

# 检查是否使用主机 Docker socket（通过环境变量或检查 socket 是否已挂载）
USE_HOST_DOCKER="${USE_HOST_DOCKER:-0}"
DOCKERD_PID=""

# 检查 Docker socket 是否已挂载（来自主机）
if [ -S /var/run/docker.sock ] && [ "$USE_HOST_DOCKER" = "1" ]; then
    echo "[启动脚本] 检测到主机 Docker socket，使用主机 Docker 引擎..."
    # 测试连接
    if docker info >/dev/null 2>&1; then
        echo "[启动脚本] 主机 Docker 连接成功，跳过启动内置 dockerd"
    else
        echo "[启动脚本] 警告: 无法连接到主机 Docker，尝试启动内置 dockerd..."
        USE_HOST_DOCKER="0"
    fi
fi

# 如果没有使用主机 Docker，启动容器内 dockerd（dind）
if [ "$USE_HOST_DOCKER" != "1" ]; then
echo "[启动脚本] 启动内置 dockerd..."
mkdir -p /var/lib/docker
dockerd --host=tcp://0.0.0.0:2375 --host=unix:///var/run/docker.sock &
DOCKERD_PID=$!

# 等待 dockerd 就绪
echo "[启动脚本] 等待 dockerd 就绪..."
for i in $(seq 1 30); do
    if docker info >/dev/null 2>&1; then
        echo "[启动脚本] dockerd 已就绪"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "[启动脚本] 警告: dockerd 启动超时"
    fi
    sleep 1
done
fi

# 启动后端服务（实时输出到日志文件，同时输出到 stdout/stderr）
echo "[启动脚本] 启动 iarnet 后端服务..."
cd /app
# 如果日志文件已挂载，使用 tee 实时输出到文件和控制台；否则只输出到控制台
if [ -w /app/iarnet.log ]; then
    /app/iarnet -config /app/config.yaml 2>&1 | tee -a /app/iarnet.log &
else
/app/iarnet -config /app/config.yaml &
fi
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

# 启动前端服务（实时输出到日志文件，同时输出到 stdout/stderr）
echo "[启动脚本] 启动 iarnet 前端服务..."
cd /app/web
# 如果日志文件已挂载，使用 tee 实时输出到文件和控制台；否则只输出到控制台
if [ -w /app/iarnet.log ]; then
    npm start 2>&1 | tee -a /app/iarnet.log &
else
npm start &
fi
FRONTEND_PID=$!

# 定义清理函数
cleanup() {
    echo "[启动脚本] 收到退出信号，正在停止服务..."
    kill $BACKEND_PID $FRONTEND_PID 2>/dev/null || true
    if [ -n "$DOCKERD_PID" ]; then
        kill $DOCKERD_PID 2>/dev/null || true
    fi
    wait $BACKEND_PID $FRONTEND_PID 2>/dev/null || true
    if [ -n "$DOCKERD_PID" ]; then
        wait $DOCKERD_PID 2>/dev/null || true
    fi
    exit 0
}

# 捕获退出信号
trap cleanup SIGTERM SIGINT

# 等待进程退出（如果任一进程退出，脚本也会退出）
wait $BACKEND_PID $FRONTEND_PID
EOF

RUN chmod +x /app/entrypoint.sh && \
    chown appuser:appuser /app/entrypoint.sh

# 暴露端口
# 8083: 后端 HTTP API
# 3000: 前端 Next.js
EXPOSE 8083 3000

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD pgrep -f iarnet && pgrep -f "next start" || exit 1

# 使用启动脚本作为入口点
ENTRYPOINT ["/app/entrypoint.sh"]

