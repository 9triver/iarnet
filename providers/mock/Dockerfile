# 多阶段构建 Dockerfile for mock provider
# Mock provider 用于实验和测试，模拟资源提供者行为

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

# 切换到 mock provider 目录
WORKDIR /build/iarnet/providers/mock

# 下载依赖并同步 go.sum
RUN go mod tidy && go mod download

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s' \
    -a \
    -o mock-provider ./cmd/main.go

# 运行阶段 - 使用 alpine 作为基础镜像
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata

# 创建非 root 用户
RUN addgroup -S appuser && adduser -S -G appuser appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/iarnet/providers/mock/mock-provider /app/mock-provider

# 复制配置文件（如果存在）
COPY --from=builder /build/iarnet/providers/mock/config.yaml /app/config.yaml

# 设置权限
RUN chown -R appuser:appuser /app

# 切换到非 root 用户
USER appuser

# 暴露 gRPC 端口（默认 50051）
EXPOSE 50051

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep -f mock-provider || exit 1

# 运行 mock provider
CMD ["/app/mock-provider", "--config", "/app/config.yaml"]

