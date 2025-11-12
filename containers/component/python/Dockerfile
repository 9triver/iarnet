# Python Component Dockerfile
# 用于运行 Python 组件的容器镜像

FROM python:3.11-alpine

# 安装系统依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    zeromq \
    czmq \
    zeromq-dev \
    czmq-dev \
    pkgconfig \
    gcc \
    musl-dev \
    python3-dev

# 设置工作目录
WORKDIR /app

# 创建虚拟环境
RUN python -m venv /app/venv

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

# 复制主程序
COPY main.py /app/main.py

# 创建非 root 用户
RUN addgroup -S appuser && adduser -S -G appuser appuser
RUN chown -R appuser:appuser /app

# 切换到非 root 用户
USER appuser

# 设置环境变量
ENV PYTHONUNBUFFERED=1
ENV PYTHONPATH=/app
ENV PATH="/app/venv/bin:$PATH"

# 入口点 - 使用虚拟环境中的 Python
ENTRYPOINT ["/app/venv/bin/python", "main.py"]

