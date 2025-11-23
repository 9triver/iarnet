# 多阶段构建 Dockerfile for docker provider
# 使用 docker:dind 作为基础镜像，包含 Docker daemon

# 构建阶段
FROM golang:1.25-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.org

# 安装必要的工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /build

# 复制整个项目到构建容器中（因为有跨目录依赖）
# 注意：这需要从项目根目录运行 docker build
COPY . /build/iarnet

# 切换到 docker provider 目录
WORKDIR /build/iarnet/providers/docker

# 下载依赖
RUN go mod download

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s' \
    -a \
    -o docker-provider ./cmd/main.go

# 运行阶段 - 使用 docker:dind 作为基础镜像
FROM docker:dind

# 安装必要的运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata

# 创建非 root 用户
RUN addgroup -S appuser && adduser -S -G appuser appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/iarnet/providers/docker/docker-provider /app/docker-provider

# 复制配置文件（如果存在）
COPY --from=builder /build/iarnet/providers/docker/config.yaml /app/config.yaml

# 设置权限
RUN chown -R appuser:appuser /app

# 创建启动脚本，同时启动 Docker daemon 和 docker provider
RUN cat > /app/entrypoint.sh << 'EOF'
#!/bin/sh
set -e

# 启动 Docker daemon 在后台
dockerd-entrypoint.sh &
DOCKERD_PID=$!

# 等待 Docker daemon 启动
echo "Waiting for Docker daemon to start..."
for i in $(seq 1 30); do
    if docker info > /dev/null 2>&1; then
        echo "Docker daemon is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "Failed to start Docker daemon"
        exit 1
    fi
    sleep 1
done

# 启动 docker provider
exec /app/docker-provider -config /app/config.yaml
EOF

RUN chmod +x /app/entrypoint.sh

# 暴露 gRPC 端口（默认 50051）
EXPOSE 50051

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD pgrep -f docker-provider || exit 1

# 使用启动脚本作为入口点
ENTRYPOINT ["/app/entrypoint.sh"]
