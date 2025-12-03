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

FROM python:3.11-alpine
RUN apk update && apk add --no-cache \
    git \
    ca-certificates \
    bash \
    && ln -sf /usr/local/bin/python3 /usr/bin/python \
    && ln -sf /usr/local/bin/pip3 /usr/bin/pip

# 安装通用 Python 依赖
RUN pip install cloudpickle
RUN pip install grpcio
RUN pip install grpcio-tools
RUN pip install numpy

# 安装 lucas 及其依赖（从 third_party/lucas 子模块）
COPY third_party/lucas /tmp/lucas
RUN cd /tmp/lucas && pip install -e .

COPY --from=builder /app/runner /usr/bin/runner

# 创建应用代码目录
RUN mkdir -p /iarnet/app

WORKDIR /iarnet/app

CMD ["runner"]
