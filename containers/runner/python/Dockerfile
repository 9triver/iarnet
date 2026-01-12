FROM golang:1.25.0 AS builder
WORKDIR /app
# 通用 Runner Core（与具体语言运行环境无关）
COPY containers/runner/core .

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.org
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN go mod download
RUN go build -o runner .

FROM python:3.11-slim
RUN apt update && apt install -y \
    git \
    ca-certificates \
    bash \
    && ln -sf /usr/local/bin/python3 /usr/bin/python \
    && ln -sf /usr/local/bin/pip3 /usr/bin/pip

# 安装通用 Python 依赖
RUN pip install --no-cache-dir \
-i https://pypi.mirrors.ustc.edu.cn/simple \
cloudpickle \
torchvision \
grpcio \
grpcio-tools \
pyzmq \
numpy \
pandas \
datasets \
modelscope \
transformers \
peft \
bitsandbytes \
river \
ultralytics \
opencv-python-headless && pip install --no-cache-dir -i https://download.pytorch.org/whl/cpu torch 

RUN apt-get update && apt-get install -y \
    libgl1 \
    libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*

# 安装 lucas 及其依赖（从 third_party/lucas 子模块）
COPY third_party/lucas /tmp/lucas
RUN cd /tmp/lucas && pip install -e .

COPY --from=builder /app/runner /usr/bin/runner

# 创建应用代码目录
RUN mkdir -p /iarnet/app

WORKDIR /iarnet/app

CMD ["runner"]
