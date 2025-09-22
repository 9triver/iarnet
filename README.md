# Container Peer Service

问题记录

1. 前端的运行时应该放在哪？
2. 前端的运行时会持续与 ignis 通信，获取任务信息和执行结果，这部分通信是否需要重构？
3. store 是管理同一组 actor 通信的吗？
4. ignis 如何与部署在远程容器中的 store 进行通信？
5. 部署 actor 时需要确认是否部署 store
6. actor 和 store 的部署位置有无要求？
7. 前端的运行时需要为其分配资源吗？
8. 由于是将应用导入平台运行，而不是应用运行时连接平台，需要减少应用代码中框架的侵入性。（参考 flink）
9. DAG 其实是前端结构，后端不感知。


## Build
go build -o cps ./cmd

## Run Standalone
./cps --config=config.yaml

## Run in K8s
kubectl apply -f k8s-deployment.yaml

## API Usage
POST /run
Body: {"image": "nginx", "command": ["nginx"], "cpu": 1.0, "memory": 0.5, "gpu": 0}

## Extend
- Add real resource monitoring (poll Docker/K8s).
- Implement load balancing: If local full, forward to peers.
- Add error handling, logging, metrics (Prometheus).
- Secure gRPC/HTTP with TLS.
- Handle container completion (deallocate resources).

# Web UI

## Quick Start

```shell
npm install # npm install --legacy-peer-deps
npm run dev
```

## Environment Configuration

The web application uses environment variables for configuration. Create a `.env.local` file in the `web/` directory:

```shell
# Backend API URL (required)
BACKEND_URL=http://localhost:8083
```

### Environment Variables

- `BACKEND_URL`: Backend API server URL
  - Local development: `http://localhost:8083`
  - Docker environment: `http://workspace:8083`
  - Production: Your production backend URL

**Note**: Environment files (`.env*`) are ignored by git to keep sensitive configuration out of the repository.