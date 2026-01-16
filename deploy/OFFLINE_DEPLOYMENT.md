# 离线部署优化方案

## 问题分析

当前环境包含以下镜像：
- `iarnet:latest` - 1.65GB
- `iarnet-global:latest` - 819MB
- `iarnet/provider:docker` - 67.1MB
- `iarnet/runner:python_3.11-latest` - 9.51GB ⚠️
- `iarnet/component:python_3.11-latest` - 13.5GB ⚠️

**总大小约 25GB+**，直接导出导入非常慢。

## 优化方案对比

### 方案 1: 压缩导出（推荐，最简单）⭐

**优点**:
- 实现简单，无需额外工具
- 压缩率通常可达 30-50%
- 兼容性好

**缺点**:
- 仍需要传输较大文件
- 导入时需要解压时间

**使用方法**:
```bash
cd /home/zhangyx/iarnet/deploy
./export_images.sh
```

这会生成压缩的 `.tar.gz` 文件，通常可以节省 30-50% 的传输时间。

---

### 方案 2: 使用私有 Registry（推荐，最专业）⭐⭐⭐

**优点**:
- 支持增量更新（只传输变更层）
- 可以分批部署
- 支持版本管理
- 可以复用基础镜像层

**缺点**:
- 需要中间服务器搭建 Registry
- 配置稍复杂

**实现步骤**:

#### 2.1 在有网络的服务器上搭建 Harbor

```bash
# 下载 Harbor
wget https://github.com/goharbor/harbor/releases/download/v2.10.0/harbor-offline-installer-v2.10.0.tgz
tar xvf harbor-offline-installer-v2.10.0.tgz
cd harbor

# 配置 harbor.yml
# 修改 hostname 和端口

# 安装
./install.sh
```

#### 2.2 推送镜像到 Harbor

```bash
# 登录 Harbor
docker login your-harbor-server:port

# 标记并推送镜像
for img in iarnet:latest iarnet-global:latest iarnet/provider:docker; do
    docker tag $img your-harbor-server:port/iarnet/$img
    docker push your-harbor-server:port/iarnet/$img
done
```

#### 2.3 导出 Harbor 数据（离线传输）

```bash
# 导出 Harbor 数据目录
tar -czf harbor-data.tar.gz /data/harbor

# 传输到目标服务器
# 在目标服务器上恢复 Harbor
```

#### 2.4 在目标服务器上使用

```bash
# 从 Harbor 拉取镜像
docker pull your-harbor-server:port/iarnet/iarnet:latest
```

---

### 方案 3: 镜像分层优化（减少重复传输）

**原理**: Docker 镜像使用分层存储，相同的基础层可以复用。

**实现**:
```bash
# 分析镜像依赖关系
docker image inspect iarnet:latest --format='{{.RootFS.Layers}}'

# 只导出变更层（需要自定义脚本）
```

**优点**:
- 如果多个镜像共享基础层，可以大幅减少传输量

**缺点**:
- 实现复杂
- 需要手动管理层依赖

---

### 方案 4: 使用 Docker BuildKit 缓存导出

**原理**: 导出构建缓存，在目标服务器上复用。

**实现**:
```bash
# 导出构建缓存
docker buildx build --cache-from type=local,src=/tmp/cache --cache-to type=local,dest=/tmp/cache .

# 传输缓存到目标服务器
# 在目标服务器上使用缓存构建
```

**优点**:
- 可以加速目标服务器的镜像构建

**缺点**:
- 仍需要传输镜像本身
- 适合需要重新构建的场景

---

### 方案 5: 优化镜像大小（长期方案）

**针对大镜像的优化**:

#### 5.1 优化 runner 和 component 镜像

这两个镜像特别大（9.51GB 和 13.5GB），可能的原因：
- 包含大量 Python 包
- 未使用多阶段构建
- 包含不必要的文件

**优化建议**:
```dockerfile
# 使用多阶段构建
FROM python:3.11-slim AS builder
# 只安装必要的包

FROM python:3.11-slim AS runtime
# 只复制必要的文件
COPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages
```

#### 5.2 使用 .dockerignore

确保不复制不必要的文件：
```
node_modules/
.git/
*.md
test/
```

#### 5.3 使用 Alpine 基础镜像

```dockerfile
FROM python:3.11-alpine
# Alpine 镜像通常小 50-70%
```

---

## 推荐方案组合

### 短期方案（立即使用）

1. **使用压缩导出**（方案 1）
   ```bash
   ./export_images.sh
   ```
   - 可以立即减少 30-50% 传输时间
   - 实现简单，无需额外配置

2. **分批传输**
   - 先传输小镜像（provider, global）
   - 再传输大镜像（runner, component）
   - 可以并行部署部分服务

### 中期方案（1-2周内）

1. **搭建 Harbor 私有仓库**（方案 2）
   - 在中间服务器（有网络）搭建 Harbor
   - 推送所有镜像到 Harbor
   - 导出 Harbor 数据目录
   - 在目标服务器恢复 Harbor
   - **优势**: 支持增量更新，后续部署更快

### 长期方案（持续优化）

1. **优化镜像大小**（方案 5）
   - 重构 runner 和 component 的 Dockerfile
   - 使用多阶段构建
   - 使用 Alpine 基础镜像
   - **预期**: 可以将镜像大小减少 50-70%

---

## 快速开始

### 使用压缩导出（推荐）

```bash
# 1. 导出镜像（自动压缩）
cd /home/zhangyx/iarnet/deploy
./export_images.sh

# 2. 查看导出结果
ls -lh offline_images/

# 3. 打包传输
tar -czf offline_images.tar.gz offline_images/

# 4. 传输到目标服务器（使用 scp, rsync, 或移动硬盘）

# 5. 在目标服务器上导入
cd offline_images
./import_images.sh

# 6. 部署
cd /path/to/deploy
docker-compose up -d
```

### 使用 Harbor（专业方案）

```bash
# 1. 在有网络的服务器上搭建 Harbor（参考方案 2）

# 2. 推送镜像
./push_to_harbor.sh

# 3. 导出 Harbor 数据
tar -czf harbor-data.tar.gz /data/harbor

# 4. 在目标服务器上恢复 Harbor 并拉取镜像
```

---

## 性能对比

| 方案 | 传输大小 | 传输时间* | 实现难度 | 推荐度 |
|------|---------|----------|---------|--------|
| 原始导出 | 25GB | 100% | ⭐ | ❌ |
| 压缩导出 | 15-18GB | 60-70% | ⭐ | ⭐⭐⭐ |
| Harbor | 15-18GB | 60-70% | ⭐⭐⭐ | ⭐⭐⭐ |
| Harbor + 增量 | 5-10GB | 20-40% | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| 优化镜像 + 压缩 | 8-12GB | 30-50% | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |

*传输时间基于 100Mbps 网络估算

---

## 注意事项

1. **磁盘空间**: 确保目标服务器有足够空间（建议 50GB+）
2. **Docker 版本**: 确保源和目标 Docker 版本兼容
3. **网络配置**: 部署后需要配置网络和 IP
4. **权限问题**: 确保有 Docker 操作权限
5. **数据持久化**: 注意 volumes 和数据的迁移

---

## 故障排查

### 导入失败
```bash
# 检查磁盘空间
df -h

# 检查 Docker 版本
docker version

# 手动导入单个镜像
gunzip -c image.tar.gz | docker load
```

### 镜像标签丢失
```bash
# 重新标记镜像
docker tag old-name:tag new-name:tag
```

### 网络问题
```bash
# 检查网络配置
docker network ls
docker network inspect iarnet-demo-network
```

---

## 联系支持

如有问题，请检查：
1. 导出日志: `export_images.sh` 的输出
2. 导入日志: `import_images.sh` 的输出
3. Docker 日志: `docker logs <container>`

