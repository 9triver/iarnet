FROM golang:1.25.0 AS builder
WORKDIR /app
COPY containers/images/runner .

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.org

RUN go mod download
RUN go build -o runner ./main.go

# ======= 运行环境 1：Python =======
FROM python:3.11-alpine AS python_3.11
RUN apk update && apk add --no-cache \
    git \
    ca-certificates \
    bash \
    && ln -sf /usr/local/bin/python3 /usr/bin/python \
    && ln -sf /usr/local/bin/pip3 /usr/bin/pip

# 安装 actorc 及其依赖
COPY containers/envs/python/libs/actorc /tmp/actorc
RUN cd /tmp/actorc && pip install -e .

# 安装 lucas 及其依赖
COPY containers/envs/python/libs/lucas /tmp/lucas
RUN cd /tmp/lucas && pip install -e .

COPY --from=builder /app/runner /usr/bin/runner

# 创建应用代码目录
RUN mkdir -p /iarnet/app

WORKDIR /iarnet/app

CMD ["runner"]
