# Python 基础镜像 - 包含 actorc 和 lucas 库
FROM python:3.11-slim

# 安装系统依赖
RUN apt-get update && apt-get install -y \
    git \
    ca-certificates \
    gcc \
    g++ \
    && rm -rf /var/lib/apt/lists/*

# 升级 pip
RUN pip install --upgrade pip

# 安装通用 Python 依赖
RUN pip install numpy

# 复制并安装 actorc
COPY ../shared/python/libs/actorc /tmp/actorc
RUN cd /tmp/actorc && pip install -e .

# 复制并安装 lucas
COPY ../shared/python/libs/lucas /tmp/lucas
RUN cd /tmp/lucas && pip install -e .

# 清理临时文件
RUN rm -rf /tmp/actorc /tmp/lucas

# 创建非 root 用户
RUN addgroup --system appuser && adduser --system --group appuser

# 设置工作目录
WORKDIR /app

# 切换到非 root 用户
USER appuser