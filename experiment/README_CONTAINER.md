# 实验容器使用说明

## 概述

实验容器用于在隔离的网络环境中运行实验代码，避免本地网络环境对实验结果的影响。

## 快速开始

### 1. 构建并启动容器

```bash
cd /home/zhangyx/iarnet/experiment
./run_experiment_container.sh
```

这将：
- 自动构建实验容器镜像（如果不存在）
- 创建并启动容器
- 挂载 iarnet 目录到容器内
- 连接到实验网络（如果 docker-compose 服务已启动）
- 进入交互式 shell

### 2. 在容器内运行实验

进入容器后，你可以：

```bash
# 查看工作目录
pwd  # /workspace/experiment

# 查看 iarnet 源码
ls -la /workspace/iarnet

# 编译实验代码
go build -o experiment main.go

# 运行实验
./experiment

# 或直接运行（无需编译）
go run main.go
```

### 3. 执行单个命令

你也可以在不进入交互式 shell 的情况下执行命令：

```bash
# 编译实验代码
./run_experiment_container.sh go build -o experiment main.go

# 运行实验
./run_experiment_container.sh ./experiment

# 查看 Go 版本
./run_experiment_container.sh go version
```

## 容器配置

### 挂载目录

- **iarnet 源码**: `/workspace/iarnet` - 完整的 iarnet 项目目录
- **实验目录**: `/workspace/experiment` - 当前实验代码目录

### 网络连接

容器会自动连接到 `iarnet-testing-network` 网络（如果存在），这样容器内的实验代码可以访问到：
- 所有 iarnet 节点（iarnet-1 到 iarnet-7）
- 所有 provider 服务（provider-i1-p1 到 provider-i8-p3）

### 固定 IP 配置

容器配置了固定 IP 地址：**172.30.0.20**

- 确保容器每次启动都使用相同的 IP，避免网络环境变化对实验的影响
- IP 地址在脚本中定义，可以根据需要修改 `run_experiment_container.sh` 中的 `FIXED_IP` 变量
- 如果固定 IP 已被占用，脚本会尝试使用动态 IP 并显示警告

### 环境变量

容器内已配置：
- `GOPROXY=https://goproxy.cn,direct` - 使用国内 Go 代理
- `GOSUMDB=sum.golang.org` - Go 校验和数据库
- `CGO_ENABLED=1` - 启用 CGO（用于 sqlite3 等）

## 工作流程

### 完整实验流程

1. **启动实验环境**（在宿主机）
   ```bash
   cd /home/zhangyx/iarnet/experiment/setup
   docker-compose up -d
   ```

2. **进入实验容器**（在宿主机）
   ```bash
   cd /home/zhangyx/iarnet/experiment
   ./run_experiment_container.sh
   ```

3. **在容器内运行实验**（在容器内）
   ```bash
   go run main.go
   ```

4. **查看实验结果**（在宿主机）
   ```bash
   ls -la /home/zhangyx/iarnet/experiment/results/
   ```

## 容器管理

### 停止容器

```bash
docker stop iarnet-experiment
```

### 删除容器

```bash
docker rm iarnet-experiment
```

### 查看容器日志

```bash
docker logs iarnet-experiment
```

### 查看容器状态

```bash
docker ps -a | grep iarnet-experiment
```

## 网络连接验证

在容器内，你可以验证网络连接：

```bash
# 查看容器 IP（应该是 172.30.0.20）
ip addr show

# 测试连接到 iarnet-1
ping -c 3 iarnet-1

# 测试连接到 provider
ping -c 3 provider-i1-p1

# 查看网络配置
ip addr show
```

### 验证固定 IP

```bash
# 在宿主机查看容器 IP
docker inspect iarnet-experiment --format "{{range \$net, \$conf := .NetworkSettings.Networks}}{{if eq \$net \"iarnet-testing-network\"}}IP: {{.IPAddress}}{{end}}{{end}}"

# 应该显示: IP: 172.30.0.20
```

## 注意事项

1. **网络依赖**: 容器会自动连接到实验网络，但需要先启动 docker-compose 服务以创建网络

2. **配置文件**: 确保实验配置文件 `config.yaml` 在 `/workspace/experiment` 目录下

3. **数据持久化**: 实验结果会保存在挂载的目录中，容器删除后数据不会丢失

4. **Go 依赖**: 首次运行可能需要下载 Go 依赖，容器内已配置国内代理加速

5. **权限问题**: 如果遇到权限问题，确保挂载的目录有适当的读写权限

## 故障排查

### 容器无法连接到网络

```bash
# 检查网络是否存在
docker network ls | grep iarnet

# 手动连接容器到网络
docker network connect iarnet-testing-network iarnet-experiment
```

### 无法访问 iarnet 服务

```bash
# 在容器内测试连接
ping iarnet-1
curl http://iarnet-1:3000

# 检查服务是否运行
docker ps | grep iarnet
```

### Go 编译错误

```bash
# 清理 Go 模块缓存
go clean -modcache

# 重新下载依赖
go mod download
```

