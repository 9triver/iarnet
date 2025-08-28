FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o cps ./cmd/main.go

FROM ubuntu:22.04
COPY --from=builder /app/cps /usr/bin/cps
CMD ["cps", "--config=/config.yaml"]