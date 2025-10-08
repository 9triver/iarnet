FROM golang:1.25.0 AS builder
WORKDIR /app
COPY go.mod .
COPY go.sum .
COPY main.go .

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.org

RUN go mod download
RUN go build -o runner ./main.go

# ======= 运行环境 1：Python =======
FROM python:3.11-slim AS python_3.11
RUN apt-get update && apt-get install -y \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 安装 actorc 及其依赖
COPY py/actorc /tmp/actorc
RUN cd /tmp/actorc && pip install -e .

# 安装 lucas 及其依赖
COPY py/lucas /tmp/lucas
RUN cd /tmp/lucas && pip install -e .

COPY --from=builder /app/runner /usr/bin/runner

# 创建应用代码目录
RUN mkdir -p /iarnet/app

WORKDIR /iarnet/app

CMD ["runner"]
