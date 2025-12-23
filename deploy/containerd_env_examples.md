# Containerd 启动容器时指定环境变量

## 使用 ctr 命令

### 1. 基本语法

```bash
# 使用 -e 或 --env 参数指定环境变量
sudo ctr -n k8s.io run -d --rm \
  --env KEY1=value1 \
  --env KEY2=value2 \
  <image-name> <container-id> <command>
```

### 2. 完整示例

```bash
# 启动容器并设置多个环境变量
sudo ctr -n k8s.io run -d --rm \
  --env COMPONENT_ID=test-component-001 \
  --env LOGGER_ADDR=localhost:50051 \
  --env ZMQ_ADDR=tcp://localhost:5555 \
  --env STORE_ADDR=localhost:50052 \
  iarnet/component:python_3.11-latest \
  test-container \
  python3 /app/main.py
```

### 3. 从文件读取环境变量

```bash
# 创建环境变量文件
cat > /tmp/env.txt <<EOF
COMPONENT_ID=test-component-001
LOGGER_ADDR=localhost:50051
ZMQ_ADDR=tcp://localhost:5555
STORE_ADDR=localhost:50052
EOF

# 使用 --env-file 参数（如果支持）
# 注意：ctr 可能不支持 --env-file，需要逐个指定
```

### 4. 使用容器规范文件（推荐用于复杂场景）

```bash
# 创建容器规范文件
cat > /tmp/container-spec.json <<EOF
{
  "ociVersion": "1.0.0",
  "process": {
    "env": [
      "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
      "COMPONENT_ID=test-component-001",
      "LOGGER_ADDR=localhost:50051",
      "ZMQ_ADDR=tcp://localhost:5555",
      "STORE_ADDR=localhost:50052"
    ],
    "args": ["python3", "/app/main.py"]
  },
  "root": {
    "path": "rootfs"
  }
}
EOF

# 使用规范文件创建容器
sudo ctr -n k8s.io containers create \
  --config /tmp/container-spec.json \
  iarnet/component:python_3.11-latest \
  test-container
```

### 5. 查看容器环境变量

```bash
# 查看容器配置
sudo ctr -n k8s.io containers info test-container

# 进入容器查看环境变量
sudo ctr -n k8s.io tasks exec --exec-id test-exec -t test-container env
```

## 在 Kubernetes 中使用

如果是在 Kubernetes Pod 中运行，需要在 Pod 定义中指定：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: test-container
    image: iarnet/component:python_3.11-latest
    env:
    - name: COMPONENT_ID
      value: "test-component-001"
    - name: LOGGER_ADDR
      value: "localhost:50051"
    - name: ZMQ_ADDR
      value: "tcp://localhost:5555"
    - name: STORE_ADDR
      value: "localhost:50052"
    command: ["python3", "/app/main.py"]
```

## 常用 ctr 命令参数

- `-n, --namespace`: 指定命名空间（Kubernetes 使用 `k8s.io`）
- `-d, --detach`: 后台运行
- `--rm`: 容器退出后自动删除
- `-e, --env`: 设置环境变量
- `-t, --tty`: 分配伪终端
- `-i, --interactive`: 保持 STDIN 打开

## 注意事项

1. **环境变量格式**：使用 `KEY=value` 格式，等号两边不要有空格
2. **特殊字符**：如果值包含特殊字符，需要用引号包裹
3. **多个环境变量**：每个环境变量需要单独的 `--env` 参数
4. **命名空间**：Kubernetes 环境必须使用 `-n k8s.io` 命名空间

