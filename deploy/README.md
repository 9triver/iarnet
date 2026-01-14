# Iarnet 部署说明

## 前端访问端口

### Iarnet 节点前端界面

| 节点 | 容器名称 | 前端访问地址 | 说明 |
|------|---------|-------------|------|
| iarnet-1 | demo-iarnet-1 | http://localhost:4001 | 智能应用运行平台原型系统节点 1 |
| iarnet-2 | demo-iarnet-2 | http://localhost:4002 | 智能应用运行平台原型系统节点 2 |
| iarnet-3 | demo-iarnet-3 | http://localhost:4003 | 智能应用运行平台原型系统节点 3 |
| iarnet-4 | demo-iarnet-4 | http://localhost:4004 | 智能应用运行平台原型系统节点 4 |
| iarnet-5 | demo-iarnet-5 | http://localhost:4005 | 智能应用运行平台原型系统节点 5 |
| iarnet-6 | demo-iarnet-6 | http://localhost:4006 | 智能应用运行平台原型系统节点 6 |
| iarnet-7 | demo-iarnet-7 | http://localhost:4007 | 智能应用运行平台原型系统节点 7 |
| iarnet-8 | demo-iarnet-8 | http://localhost:4008 | 智能应用运行平台原型系统节点 8 |
| iarnet-9 | demo-iarnet-9 | http://localhost:4009 | 智能应用运行平台原型系统节点 9 |
| iarnet-10 | demo-iarnet-10 | http://localhost:4010 | 智能应用运行平台原型系统节点 10 |

### 全局注册中心前端界面

| 服务 | 容器名称 | 前端访问地址 | 说明 |
|------|---------|-------------|------|
| iarnet-global | demo-iarnet-global | http://localhost:4000 | 智能应用运行平台原型系统注册中心 |

## 快速启动

### 启动所有服务
```bash
docker-compose up -d
```

### 启动指定节点
```bash
docker-compose up -d iarnet-1 iarnet-2
```

### 停止所有服务
```bash
docker-compose down
```

### 查看日志
```bash
# 查看所有服务日志
docker-compose logs -f

# 查看指定节点日志
docker-compose logs -f iarnet-1
```

## 默认账户

所有节点的默认超级管理员账户：
- 用户名：`admin`
- 密码：`123456`

## 端口说明

- **前端端口**：4001-4010（iarnet-1 到 iarnet-10），4000（iarnet-global）
- **后端 HTTP 端口**：所有节点统一使用 8083（容器内部）
- **Docker-in-Docker 端口**：23771-23780（iarnet-1 到 iarnet-10）

