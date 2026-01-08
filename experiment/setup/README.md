# 实验环境部署脚本说明

本目录包含用于启动和管理实验环境的脚本。

## 脚本列表

### 启动脚本
- `start_iarnets.sh` - 启动所有 Iarnet 节点（7个）
- `start_providers.sh` - 启动所有 Provider 服务（24个）

### 停止脚本
- `stop_iarnets.sh` - 停止所有 Iarnet 节点
- `stop_providers.sh` - 停止所有 Provider 服务

## 使用方法

### 启动所有服务
```bash
# 启动所有服务（iarnet + provider）
docker-compose up -d

# 或分别启动
./start_iarnets.sh      # 先启动 iarnet 节点
./start_providers.sh    # 再启动 provider 服务
```

### 停止所有服务
```bash
# 停止所有服务
docker-compose down

# 或分别停止
./stop_providers.sh     # 先停止 provider
./stop_iarnets.sh       # 再停止 iarnet
```

### 查看服务状态
```bash
# 查看所有服务状态
docker-compose ps

# 查看特定服务日志
docker-compose logs -f iarnet-1
docker-compose logs -f provider-i1-p1
```

## 服务配置

### Iarnet 节点
- **数量**: 7个（iarnet-1 到 iarnet-7）
- **前端端口**: 3001-3007
- **Docker-in-Docker 端口**: 23761-23767
- **IP 地址**: 172.30.0.10-172.30.0.16

### Provider 服务
- **数量**: 24个
- **类型**: Mock Provider（用于实验）
- **资源容量**: 每个 Provider 8 cores CPU, 16GB 内存, 1 GPU
- **gRPC 端口**: 50051-50074

## 配置文件位置

- Iarnet 配置: `./iarnet/i{1-7}/config.yaml`
- Provider 配置: `./provider/i{1-8}/p{1-3}/config.yaml`

## 注意事项

1. 确保已构建所有镜像：
   - `iarnet:latest`
   - `iarnet-mock-provider:latest`

2. 首次启动前，确保配置文件已正确配置

3. Provider 服务不依赖 Iarnet 的启动顺序，可以独立启动

4. 所有服务共享 `iarnet-testing-network` 网络

