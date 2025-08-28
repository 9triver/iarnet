# Container Peer Service

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