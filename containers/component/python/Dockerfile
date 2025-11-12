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

# 升级 pip
RUN pip install --upgrade pip

# 安装 Python 依赖
RUN pip install --no-cache-dir \
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

# 入口点
ENTRYPOINT ["python", "main.py"]

