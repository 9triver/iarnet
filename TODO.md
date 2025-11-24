TODO: 拒绝策略（e.g.拒绝非本域的调度请求）

TODO: proto 文件目录重构

TODO: 前端获取函数计算结果

TODO: actor 之间的直接通信
TODO: 代码编辑，在 vscode 中打开
TODO: 跨域和委托调度机制

TODO: ✔ 替换脚本中的 emoji

TODO: 重构type

# Container Peer Service

docker stop $(docker ps -q) && docker rm $(docker ps -aq)

1. gossip 完善前后端交互 跑例子 debug  实现前端点击 dag 节点显示详情 ws调通
2. k8s router git submodule
3. 从 ignis 中获取应用实际执行结果
4. actorc 调度改为异步非阻塞
5. 任务调度、actor调度+通信封装、actor部署+资源管理+应用管理
<!-- 4. dag node 改三状态：等待、进行、完成，更改节点状态的获取机制，如前端拿到ref时才变为已完成 -->
6. 例子重构，文件访问需要换种方式

(并发优化)

两边py版本不一样会报错 segfault

对等体actor通过router进行通信；replyTo 的场景；集成到 store 转发还是 router 转发；兼容hub转发和p2p转发；通信封装，对于actor而言拥有p2p通信体验

proto actor go 远程调用问题手动解决，后续可以在proto actor go基础上进行二开，封装一个actor.Context自动适配远程调用

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

cd ./external/runner
docker build -t iarnet-app-runner:latest .

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