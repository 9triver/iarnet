# Python Component Dockerfile
# 用于运行 Python 组件的容器镜像

FROM nvidia/cuda:12.9.1-cudnn-runtime-ubuntu24.04 AS python_cuda

# 安装系统依赖
RUN apt-get update && \
    # 1. 先安装添加源所需的工具
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    software-properties-common \
    ca-certificates \
    gpg-agent && \
    # 2. 添加 deadsnakes PPA 源 (包含 python3.11)
    add-apt-repository ppa:deadsnakes/ppa -y && \
    # 3. 再次更新源并安装其余依赖
    apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    python3.11 \
    python3.11-dev \
    python3.11-distutils \
    python3.11-venv \
    python3-pip \
    tzdata \
    libzmq5 \
    libczmq4 \
    libzmq3-dev \
    libczmq-dev \
    pkg-config \
    gcc \
    libc6-dev \
    python3-dev \
    libgomp1 && \
    # 4. 清理缓存
    rm -rf /var/lib/apt/lists/*

# 设置工作目录
WORKDIR /app

# 创建虚拟环境
RUN python3.11 -m venv /app/venv

# 激活虚拟环境并升级 pip
RUN /app/venv/bin/pip install --upgrade pip

# 在虚拟环境中安装 Python 依赖
RUN /app/venv/bin/pip install --no-cache-dir \
    cloudpickle \
    grpcio \
    grpcio-tools \
    pyzmq

# 复制 proto 文件
COPY proto /app/proto

# 复制所有 Python 模块文件
COPY *.py /app/

# 设置环境变量
ENV PYTHONUNBUFFERED=1
ENV PYTHONPATH=/app
ENV PATH="/app/venv/bin:$PATH"

# 入口点 - 使用虚拟环境中的 Python
ENTRYPOINT ["/app/venv/bin/python", "main.py"]