# 实验环境 Docker 容器
# 用于在隔离的网络环境中运行实验代码

FROM golang:1.25-alpine

# 安装必要的工具
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    bash \
    curl \
    netcat-openbsd \
    docker-cli \
    make \
    nodejs \
    npm

# 设置工作目录
WORKDIR /workspace

# 设置 Go 环境变量
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.org
ENV CGO_ENABLED=1

# 安装必要的 C 库（用于 sqlite3 等）
RUN apk add --no-cache \
    gcc \
    musl-dev \
    sqlite-dev

# 安装 ZeroMQ 依赖（goczmq 需要 CGO）
RUN apk add --no-cache \
    zeromq-dev \
    czmq-dev \
    libsodium-dev \
    pkgconfig \
    zeromq \
    czmq \
    libsodium

# 设置默认命令为 bash，保持容器运行
CMD ["/bin/bash"]

