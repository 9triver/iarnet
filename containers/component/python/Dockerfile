# --- 第一阶段：构建者 (Builder) ---
FROM nvidia/cuda:12.9.1-cudnn-runtime-ubuntu24.04 AS builder

# 换源并安装构建依赖
RUN sed -i 's|http://.*.ubuntu.com/ubuntu/|http://mirrors.ustc.edu.cn/ubuntu/|g' /etc/apt/sources.list.d/ubuntu.sources && \
    apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    software-properties-common gpg-agent ca-certificates && \
    add-apt-repository ppa:deadsnakes/ppa -y && \
    apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    python3.11 python3.11-dev python3.11-venv gcc libc6-dev pkg-config libzmq3-dev libczmq-dev

# 创建并安装 Python 依赖
WORKDIR /app
RUN python3.11 -m venv /app/venv
ENV PATH="/app/venv/bin:$PATH"

RUN pip install --upgrade pip && \
    pip install --no-cache-dir -i https://pypi.mirrors.ustc.edu.cn/simple \
    torch==2.8.0 torchvision \
    transformers modelscope ultralytics datasets \
    langchain langchain_openai langchain_chroma langchain_community \
    grpcio grpcio-tools pyzmq numpy pandas peft bitsandbytes river \
    opencv-python-headless faiss-cpu cloudpickle

# 清理虚拟环境中的冗余文件（如测试文件夹、__pycache__）
RUN find /app/venv -type d -name "__pycache__" -exec rm -rf {} + && \
    rm -rf /app/venv/lib/python3.11/site-packages/tests

# --- 第二阶段：运行者 (Runner) ---
FROM nvidia/cuda:12.9.1-cudnn-runtime-ubuntu24.04 AS runner

WORKDIR /app

# 仅安装运行时必须的系统库，剔除编译工具
RUN sed -i 's|http://.*.ubuntu.com/ubuntu/|http://mirrors.ustc.edu.cn/ubuntu/|g' /etc/apt/sources.list.d/ubuntu.sources && \
    apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    software-properties-common gpg-agent ca-certificates && \
    add-apt-repository ppa:deadsnakes/ppa -y && \
    apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    python3.11 python3.11-venv tzdata libzmq5 libczmq4 libgomp1 libgl1 libglib2.0-0 && \
    rm -rf /var/lib/apt/lists/*

# 从构建阶段复制虚拟环境
COPY --from=builder /app/venv /app/venv
# 复制源码
COPY proto /app/proto
COPY *.py /app/

ENV PYTHONPATH=/app
ENV PATH="/app/venv/bin:$PATH"
ENV PYTHONUNBUFFERED=1

ENTRYPOINT ["python", "main.py"]