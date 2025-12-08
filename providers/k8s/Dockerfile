# 多阶段构建 Dockerfile for kubernetes provider
# 支持 in-cluster 和 out-of-cluster 两种运行模式

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

# 切换到 k8s provider 目录
WORKDIR /build/iarnet/providers/k8s

# 下载依赖
RUN go mod download

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s' \
    -a \
    -o k8s-provider ./cmd/main.go

# 运行阶段 - 使用 alpine 作为基础镜像
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    kubectl  # 可选：用于调试

# 创建非 root 用户
RUN addgroup -S appuser && adduser -S -G appuser appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/iarnet/providers/k8s/k8s-provider /app/k8s-provider

# 复制配置文件（如果存在）
COPY --from=builder /build/iarnet/providers/k8s/config.yaml /app/config.yaml

# 设置权限
RUN chown -R appuser:appuser /app

# 暴露 gRPC 端口（默认 50052）
EXPOSE 50052

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep -f k8s-provider || exit 1

# 运行 kubernetes provider
# 在 Pod 内运行时，会自动使用 in-cluster 配置
# 在 Pod 外运行时，需要挂载 kubeconfig 文件
CMD ["/app/k8s-provider", "--config", "/app/config.yaml"]

