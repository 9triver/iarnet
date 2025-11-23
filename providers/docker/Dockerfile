# 多阶段构建 Dockerfile for docker provider
# 使用 socket 文件映射方式，连接宿主机的 Docker daemon

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

# 运行阶段 - 使用 alpine 作为基础镜像
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    docker-cli  # 可选：用于调试，实际通过 API 调用

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

# 暴露 gRPC 端口（默认 50051）
EXPOSE 50051

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep -f docker-provider || exit 1

# 直接运行 docker provider
# 注意：运行时需要挂载宿主机的 Docker socket: -v /var/run/docker.sock:/var/run/docker.sock
CMD ["/app/docker-provider", "--config", "/app/config.yaml"]
