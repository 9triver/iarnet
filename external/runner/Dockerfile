FROM golang:1.25.0 AS builder
WORKDIR /app
COPY . .

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.org

RUN go mod download
RUN go build -o runner ./main.go

FROM ubuntu:22.04
COPY --from=builder /app/runner /usr/bin/runner

CMD ["runner"]
