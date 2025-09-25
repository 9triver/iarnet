# IARNet 环境变量配置指南

## 概述

IARNet 支持通过环境变量进行配置，这样可以在不同的部署环境中灵活地调整应用程序的行为。

## 配置文件

### `.env.example`
这是环境变量配置的模板文件，包含了所有可配置的环境变量及其说明。

### `.env`
这是实际使用的环境变量配置文件。请根据你的具体需求修改其中的值。

**注意**: `.env` 文件已被添加到 `.gitignore` 中，不会被提交到版本控制系统。

## 使用方法

### 1. 初始化配置
```bash
# 复制模板文件
cp .env.example .env

# 编辑配置文件
nano .env  # 或使用你喜欢的编辑器
```

### 2. 加载环境变量
```bash
# 方法1: 使用提供的脚本
source ./load-env.sh

# 方法2: 手动加载
set -a
source .env
set +a
```

### 3. 验证配置
```bash
# 查看已加载的 IARNet 环境变量
env | grep "^IARNET_\|^IGNIS_" | sort
```

## 主要配置项

### 服务器配置
- `IARNET_MODE`: 运行模式 (standalone/k8s)
- `IARNET_LISTEN_ADDR`: HTTP 服务监听地址
- `IARNET_PEER_LISTEN_ADDR`: 节点间通信监听地址

### 资源限制
- `IARNET_CPU_LIMIT`: CPU 核心数限制
- `IARNET_MEMORY_LIMIT`: 内存限制
- `IARNET_GPU_LIMIT`: GPU 数量限制

### Ignis 集成
- `IGNIS_MASTER_ADDRESS`: Ignis 主节点地址

### 日志配置
- `IARNET_LOG_LEVEL`: 日志级别 (debug/info/warn/error)
- `IARNET_LOG_FORMAT`: 日志格式 (json/text)

## 在 Go 代码中使用

```go
import "os"

// 获取环境变量，如果不存在则使用默认值
mode := os.Getenv("IARNET_MODE")
if mode == "" {
    mode = "standalone"
}

listenAddr := os.Getenv("IARNET_LISTEN_ADDR")
if listenAddr == "" {
    listenAddr = ":8083"
}
```

## 在 Docker 中使用

### Dockerfile
```dockerfile
# 复制环境变量文件
COPY .env /app/.env

# 在启动脚本中加载环境变量
COPY load-env.sh /app/load-env.sh
RUN chmod +x /app/load-env.sh
```

### docker-compose.yml
```yaml
version: '3.8'
services:
  iarnet:
    build: .
    env_file:
      - .env
    # 或者直接指定环境变量
    environment:
      - IARNET_MODE=standalone
      - IARNET_LISTEN_ADDR=:8083
```

## 在 Kubernetes 中使用

### 创建 ConfigMap
```bash
kubectl create configmap iarnet-config --from-env-file=.env
```

### 在 Deployment 中使用
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: iarnet
spec:
  template:
    spec:
      containers:
      - name: iarnet
        image: iarnet:latest
        envFrom:
        - configMapRef:
            name: iarnet-config
```

## 安全注意事项

1. **不要提交 `.env` 文件**: 该文件可能包含敏感信息，已被添加到 `.gitignore`
2. **使用 Secrets**: 在生产环境中，敏感信息应使用 Kubernetes Secrets 或其他安全存储方式
3. **权限控制**: 确保 `.env` 文件的读取权限仅限于必要的用户

## 故障排除

### 环境变量未生效
1. 确认已正确加载 `.env` 文件
2. 检查变量名是否正确（区分大小写）
3. 确认应用程序代码中正确读取了环境变量

### 配置冲突
环境变量的优先级通常为：
1. 系统环境变量（最高优先级）
2. `.env` 文件中的变量
3. 应用程序默认值（最低优先级）