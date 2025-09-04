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