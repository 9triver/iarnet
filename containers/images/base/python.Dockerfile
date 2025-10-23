# =============================================================================
# 多阶段构建 Dockerfile - 支持 Python 基础镜像、Component 和 Runner
# 使用方法:
#   docker build --target python-base -t iarnet/python-base .
#   docker build --target component -t iarnet/component .
#   docker build --target runner -t iarnet/runner .
# =============================================================================

# -----------------------------------------------------------------------------
# 阶段 1: Go 构建环境
# -----------------------------------------------------------------------------
FROM golang:1.24-alpine AS go-builder

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.org

# 安装必要的工具和 ZeroMQ 依赖
RUN apk add --no-cache git ca-certificates tzdata \
    gcc musl-dev \
    zeromq-dev czmq-dev pkgconfig

# 设置工作目录
WORKDIR /build

# 复制整个项目到构建容器中
COPY iarnet /build/iarnet

# -----------------------------------------------------------------------------
# 阶段 2: Python 基础镜像 - 包含 actorc 和 lucas 库
# -----------------------------------------------------------------------------
FROM python:3.11-alpine AS python-base

# 安装系统依赖和编译工具
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

# 升级 pip
RUN pip install --upgrade pip

# 安装通用 Python 依赖
RUN pip install numpy

# 复制并安装 actorc
COPY iarnet/containers/envs/python/libs/actorc /tmp/actorc
RUN cd /tmp/actorc && pip install -e .

# 复制并安装 lucas
COPY iarnet/containers/envs/python/libs/lucas /tmp/lucas
RUN cd /tmp/lucas && pip install -e .

# 清理临时文件
RUN rm -rf /tmp/actorc /tmp/lucas

# 创建非 root 用户
RUN addgroup -S appuser && adduser -S -G appuser appuser

# 设置工作目录
WORKDIR /app

# 切换到非 root 用户
USER appuser

# -----------------------------------------------------------------------------
# 阶段 3: Component 镜像
# -----------------------------------------------------------------------------
FROM python-base AS component

# 切换回 root 用户进行文件复制
USER root

# 构建 component 二进制文件
WORKDIR /build/iarnet/containers/images/component
RUN --mount=from=go-builder,source=/build,target=/build \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s' \
    -a \
    -o /app/component ./main.go

# 复制 Python 运行时文件
COPY iarnet/containers/envs/python/runtime/executor.py /app/py/

# 设置权限
RUN chown -R appuser:appuser /app

# 切换回非 root 用户
USER appuser

# 设置环境变量
ENV APP_ID=""
ENV IGNIS_ADDR=""
ENV FUNC_NAME=""
ENV VENV_PATH="/tmp/venv"
ENV EXECUTOR_PATH="/app/py/executor.py"
ENV IPC_ADDR="ipc:///app/executor.sock"

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep -f component || exit 1

# 启动命令
CMD ["/app/component"]

# -----------------------------------------------------------------------------
# 阶段 4: Runner 镜像
# -----------------------------------------------------------------------------
FROM python-base AS runner

# 切换回 root 用户进行文件复制
USER root

# 构建 runner 二进制文件
WORKDIR /build/iarnet/containers/images/runner
RUN --mount=from=go-builder,source=/build,target=/build \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s' \
    -a \
    -o /app/runner ./main.go

# 设置权限
RUN chown -R appuser:appuser /app

# 切换回非 root 用户
USER appuser

# 设置环境变量
ENV APP_ID=""
ENV IGNIS_ADDR=""

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep -f runner || exit 1

# 启动命令
CMD ["/app/runner"]