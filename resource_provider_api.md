# 资源提供者 API 文档

本文档描述了如何使用 HTTP API 接入 Docker 或 Kubernetes 资源提供者。

## 接口列表

### 1. 注册资源提供者

**接口**: `POST /resource/providers`

**描述**: 注册一个新的资源提供者（Docker 或 Kubernetes）

**请求体**:
```json
{
  "type": "docker|k8s",
  "config": {
    // 配置信息，根据类型不同而不同
  }
}
```

#### Docker 提供者配置

```json
{
  "type": "docker",
  "config": {
    "host": "tcp://192.168.1.100:2376",
    "tlsCertPath": "/path/to/certs",
    "tlsVerify": true,
    "apiVersion": "1.41"
  }
}
```

**配置字段说明**:
- `host`: Docker daemon 主机地址（必填）
- `tlsCertPath`: TLS 证书路径（可选）
- `tlsVerify`: 是否启用 TLS 验证（可选）
- `apiVersion`: Docker API 版本（可选）

#### Kubernetes 提供者配置

```json
{
  "type": "k8s",
  "config": {
    "kubeConfigContent": "apiVersion: v1\nkind: Config\nclusters:\n- cluster:\n    certificate-authority-data: LS0t...\n    server: https://kubernetes.example.com:6443\n  name: my-cluster\ncontexts:\n- context:\n    cluster: my-cluster\n    user: my-user\n  name: my-context\ncurrent-context: my-context\nusers:\n- name: my-user\n  user:\n    token: eyJhbGciOiJSUzI1NiIs...",
    "namespace": "default",
    "context": "my-context"
  }
}
```

**配置字段说明**:
- `kubeConfigContent`: kubeconfig 文件内容（必需，完整的 kubeconfig YAML 内容字符串，包含集群、用户、上下文等完整配置信息，为空时使用 in-cluster 配置）
- `namespace`: Kubernetes 命名空间（可选，默认为 "default"）
- `context`: kubeconfig 上下文（可选）

**获取 kubeconfig 内容的方法：**
```bash
# 获取默认 kubeconfig 内容
cat ~/.kube/config

# 或者获取指定路径的 kubeconfig 内容
cat /path/to/your/kubeconfig
```

**响应**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "providerId": "docker-1",
    "message": "Provider registered successfully"
  }
}
```

### 2. 注销资源提供者

**接口**: `DELETE /resource/providers/{id}`

**描述**: 注销指定的资源提供者

**路径参数**:
- `id`: 提供者 ID

**响应**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "message": "Provider unregistered successfully"
  }
}
```

### 3. 获取资源提供者列表

**接口**: `GET /resource/providers`

**描述**: 获取所有已注册的资源提供者信息

**响应**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "providers": [
      {
        "id": "docker-1",
        "name": "standalone",
        "url": "tcp://192.168.1.100:2376",
        "type": "docker",
        "status": 1,
        "cpu_usage": {
          "used": 2.5,
          "total": 8.0
        },
        "memory_usage": {
          "used": 4294967296,
          "total": 17179869184
        },
        "last_update_time": "2024-01-01T12:00:00Z"
      }
    ]
  }
}
```

### 4. 获取资源容量信息

**接口**: `GET /resource/capacity`

**描述**: 获取所有资源提供者的总容量信息

**响应**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "total": {
      "cpu": 16.0,
      "memory": 34359738368,
      "gpu": 2.0
    },
    "used": {
      "cpu": 4.5,
      "memory": 8589934592,
      "gpu": 0.0
    },
    "available": {
      "cpu": 11.5,
      "memory": 25769803776,
      "gpu": 2.0
    }
  }
}
```

## 使用示例

### 注册 Docker 提供者

```bash
curl -X POST http://localhost:8080/resource/providers \
  -H "Content-Type: application/json" \
  -d '{
    "type": "docker",
    "config": {
      "host": "tcp://192.168.1.100:2376",
      "tlsVerify": false
    }
  }'
```

### 注册 Kubernetes 提供者

```bash
# 方法1: 使用 shell 变量（推荐）
# 首先获取 kubeconfig 内容
KUBE_CONFIG=$(cat ~/.kube/config | sed 's/"/\\"/g' | tr '\n' '\\n')

# 然后发送注册请求
curl -X POST http://localhost:8080/resource/providers \
  -H "Content-Type: application/json" \
  -d "{
    \"type\": \"k8s\",
    \"config\": {
      \"kubeConfigContent\": \"$KUBE_CONFIG\",
      \"namespace\": \"iarnet\"
    }
  }"

# 方法2: 使用文件（适用于复杂的 kubeconfig）
# 创建请求文件
cat > k8s_register.json << 'EOF'
{
  "type": "k8s",
  "config": {
    "kubeConfigContent": "",
    "namespace": "iarnet"
  }
}
EOF

# 使用 jq 插入 kubeconfig 内容
jq --rawfile kubeconfig ~/.kube/config '.config.kubeConfigContent = $kubeconfig' k8s_register.json > k8s_register_final.json

# 发送请求
curl -X POST http://localhost:8080/resource/providers \
  -H "Content-Type: application/json" \
  -d @k8s_register_final.json
```

### 获取提供者列表

```bash
curl http://localhost:8080/resource/providers
```

### 注销提供者

```bash
curl -X DELETE http://localhost:8080/resource/providers/docker-1
```

## 错误处理

所有接口在出错时会返回相应的 HTTP 状态码和错误信息：

```json
{
  "code": 400,
  "message": "bad request",
  "error": "invalid request body"
}
```

常见错误码：
- `400`: 请求参数错误
- `404`: 资源未找到
- `500`: 内部服务器错误

## 注意事项

1. **Docker 提供者**: 确保 Docker daemon 可访问，并且网络连接正常
2. **Kubernetes 提供者**: 确保 kubeconfig 文件有效，或者在集群内运行时有正确的 RBAC 权限
3. **资源监控**: 系统会定期更新资源使用情况，可能存在轻微延迟
4. **并发安全**: 所有接口都是线程安全的，支持并发调用