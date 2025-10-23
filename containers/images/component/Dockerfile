# 多阶段构建 Dockerfile for component
# 构建阶段
FROM golang:1.24-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.org

# 安装必要的工具和 ZeroMQ 依赖
RUN apk add --no-cache git ca-certificates tzdata \
    gcc musl-dev \
    zeromq-dev czmq-dev pkgconfig

# 设置工作目录
WORKDIR /build

# 复制整个项目到构建容器中（因为有跨目录依赖）
# 注意：这需要从项目根目录运行 docker build
COPY iarnet /build/iarnet

# 切换到 component 目录
WORKDIR /build/iarnet/external/component

# 下载依赖
RUN go mod download

# 构建应用
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s' \
    -a \
    -o component ./main.go

# 运行阶段
FROM python:3.11-alpine

# 安装系统依赖和编译工具（opencv-python-headless 需要）
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    zeromq \
    czmq \
    zeromq-dev \
    czmq-dev \
    pkgconfig \
    # 编译工具
    gcc \
    g++ \
    make \
    cmake \
    ninja \
    # OpenCV 依赖
    musl-dev \
    linux-headers \
    libffi-dev \
    openssl-dev \
    # 图像处理库
    libjpeg-turbo-dev \
    libpng-dev \
    tiff-dev \
    libwebp-dev \
    openjpeg-dev \
    # 数学库
    openblas-dev \
    lapack-dev \
    # 其他依赖
    zlib-dev \
    freetype-dev \
    # Python 开发头文件
    python3-dev

# 创建非 root 用户
RUN addgroup -S appuser && adduser -S -G appuser appuser

# 安装 actorc 及其依赖到系统 Python
COPY --from=builder /build/iarnet/containers/shared/python/libs/actorc /tmp/actorc
RUN pip install --upgrade pip
RUN pip install numpy
RUN cd /tmp/actorc && pip install -e .

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/iarnet/containers/images/component/component /app/component

# 复制 Python 执行器文件
COPY --from=builder /build/iarnet/containers/shared/python/runtime/executor.py /app/py/executor.py

# 设置权限
RUN chown -R appuser:appuser /app
USER appuser

# 设置环境变量
ENV APP_ID=""
ENV IGNIG_ADDR=""
ENV FUNC_NAME=""
ENV VENV_PATH="/tmp/venv"
ENV EXECUTOR_PATH="/app/py/executor.py"
ENV IPC_ADDR="ipc:///app/executor.sock"

# ZeroMQ 相关环境变量
# ENV ZMQ_HAVE_CURVE=1
# ENV PKG_CONFIG_PATH="/usr/lib/pkgconfig"

# 暴露端口（如果需要）
# EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep -f component || exit 1

# 启动命令
CMD ["/app/component"]