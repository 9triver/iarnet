# IAR.NET HTTP API 文档

[TOC]

## 概述

IAR.NET 提供两套API接口：
- **后端Go服务API**：运行在端口8080，提供核心业务逻辑
- **前端Next.js API**：运行在端口3000，提供代理接口和前端特定功能

## 通用响应格式

### 后端Go服务响应格式
```json
{
  "code": 200,
  "message": "success",
  "data": {...}
}
```

### 前端Next.js响应格式
```json
{
  "success": true,
  "data": {...}
}
```

## 错误处理

### HTTP状态码
- `200`: 成功
- `400`: 请求参数错误
- `401`: 未授权
- `403`: 禁止访问
- `404`: 资源不存在
- `500`: 服务器内部错误

### 错误响应格式

**后端Go服务：**
```json
{
  "code": 404,
  "message": "Resource not found",
  "data": null
}
```

**前端Next.js：**
```json
{
  "success": false,
  "error": {
    "code": "RESOURCE_NOT_FOUND",
    "message": "指定的资源不存在",
    "details": {
      "resource_id": "123",
      "timestamp": "2024-01-15T14:30:00Z"
    }
  }
}
```

---

## 后端Go服务API (端口8080)

### 1. 资源管理API

#### 1.1 获取资源容量信息

**请求：**
```
GET /resource/capacity
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "total_cpu": 8,
    "used_cpu": 2,
    "total_memory": 16,
    "used_memory": 4,
    "total_storage": 500,
    "used_storage": 120
  }
}
```

#### 1.2 获取所有资源提供者

**请求：**
```
GET /resource/providers
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "local_provider": {
      "id": "local_docker",
      "name": "Local Docker",
      "type": "docker",
      "host": "localhost",
      "port": 2376,
      "status": "connected",
      "cpu_usage": {"total": 8, "used": 2},
      "memory_usage": {"total": 16, "used": 4},
      "last_update_time": "2024-01-15T14:30:00Z"
    },
    "managed_providers": [],
    "collaborative_providers": []
  }
}
```

#### 1.3 注册新的资源提供者

**请求：**
```
POST /resource/providers
Content-Type: application/json

// Docker提供者
{
  "type": "docker",
  "config": {
    "host": "tcp://192.168.1.100:2376",
    "tlsCertPath": "/path/to/certs",
    "tlsVerify": true,
    "apiVersion": "1.41"
  }
}

// Kubernetes提供者
{
  "type": "k8s",
  "config": {
    "kubeConfigContent": "apiVersion: v1\nkind: Config...",
    "namespace": "default",
    "context": "my-context"
  }
}
```

**响应：**
```json
{
  "code": 200,
  "message": "Provider registered successfully",
  "data": {
    "id": "provider_123",
    "type": "docker",
    "status": "connected"
  }
}
```

#### 1.4 注销资源提供者

**请求：**
```
DELETE /resource/providers/{id}
```

**响应：**
```json
{
  "code": 200,
  "message": "Provider unregistered successfully",
  "data": null
}
```

### 2. 应用管理API

#### 2.1 获取所有应用

**请求：**
```
GET /application/apps
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "applications": [
      {
        "id": "app_123",
        "name": "用户管理系统",
        "git_url": "https://github.com/company/user-management",
        "branch": "main",
        "type": "web",
        "description": "基于React和Node.js的用户管理后台系统",
        "ports": [3000],
        "health_check": "/health",
        "status": "running",
        "last_deployed": "2024-01-15 14:30:00",
        "running_on": ["local_docker"]
      }
    ]
  }
}
```

#### 2.2 创建新应用

**请求：**
```
POST /application/apps
Content-Type: application/json

{
  "name": "用户管理系统",
  "description": "基于React和Node.js的用户管理后台系统",
  "git_url": "https://github.com/company/user-management",
  "branch": "main",
  "type": "web",
  "ports": [3000],
  "health_check": "/health"
}
```

**响应：**
```json
{
  "code": 200,
  "message": "Application created successfully",
  "data": {
    "id": "app_123",
    "name": "用户管理系统",
    "status": "idle",
    "created_at": "2024-01-15T14:30:00Z"
  }
}
```

#### 2.3 获取单个应用详情

**请求：**
```
GET /application/apps/{id}
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "app_123",
    "name": "用户管理系统",
    "git_url": "https://github.com/company/user-management",
    "branch": "main",
    "type": "web",
    "description": "基于React和Node.js的用户管理后台系统",
    "ports": [3000],
    "health_check": "/health",
    "status": "running",
    "created_at": "2024-01-15T14:30:00Z",
    "last_deployed": "2024-01-15 14:30:00",
    "running_on": ["local_docker"]
  }
}
```

#### 2.4 删除应用

**请求：**
```
DELETE /application/apps/{id}
```

**响应：**
```json
{
  "code": 200,
  "message": "Application deleted successfully",
  "data": null
}
```

#### 2.5 获取应用统计信息

**请求：**
```
GET /application/stats
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "total_applications": 5,
    "running_applications": 3,
    "stopped_applications": 2,
    "total_actor_components": 15,
    "running_actor_components": 10,
    "actor_types": {
      "web": 5,
      "api": 4,
      "worker": 3,
      "compute": 2,
      "gateway": 1
    }
  }
}
```

### 3. 文件管理API

#### 3.1 获取应用文件树

**请求：**
```
GET /application/apps/{id}/files
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "files": [
      {
        "name": "src",
        "type": "directory",
        "children": [
          {
            "name": "index.js",
            "type": "file",
            "size": 1024
          }
        ]
      }
    ]
  }
}
```

#### 3.2 获取文件内容

**请求：**
```
GET /application/apps/{id}/files/content?path=/src/index.js
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "content": "const express = require('express');\nconst app = express();\n...",
    "encoding": "utf-8"
  }
}
```

### 4. 组件管理API

#### 4.1 获取应用组件

**请求：**
```
GET /application/apps/{id}/components
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "components": [
      {
        "id": "comp_123",
        "name": "web-server",
        "type": "service",
        "status": "running",
        "port": 3000,
        "health_check": "/health"
      }
    ]
  }
}
```

#### 4.2 分析应用架构

**请求：**
```
POST /application/apps/{id}/analyze
```

**响应：**
```json
{
  "code": 200,
  "message": "Analysis completed",
  "data": {
    "components": [
      {
        "name": "web-server",
        "type": "web",
        "dockerfile": "Dockerfile",
        "port": 3000,
        "actor_type": "Web服务Actor"
      }
    ]
  }
}
```

#### 4.3 部署Actor组件

**请求：**
```
POST /application/apps/{id}/deploy-components
```

**响应：**
```json
{
  "code": 200,
  "message": "Actor components deployed successfully",
  "data": {
    "deployment_id": "deploy_123",
    "status": "deploying",
    "total_components": 5,
    "deployed_components": ["web-actor", "api-actor", "worker-actor"]
  }
}
```

#### 4.4 启动Actor组件

**请求：**
```
POST /application/apps/{id}/components/{componentId}/start
```

**响应：**
```json
{
  "code": 200,
  "message": "Actor component started successfully",
  "data": {
    "component_id": "comp_123",
    "actor_type": "web",
    "status": "running",
    "container_ref": {
      "id": "container_456",
      "name": "web-actor-container"
    }
  }
}
```

#### 4.5 停止Actor组件

**请求：**
```
POST /application/apps/{id}/components/{componentId}/stop
```

**响应：**
```json
{
  "code": 200,
  "message": "Actor component stopped successfully",
  "data": {
    "component_id": "comp_123",
    "actor_type": "web",
    "status": "stopped"
  }
}
```

#### 4.6 获取Actor组件状态

**请求：**
```
GET /application/apps/{id}/components/{componentId}/status
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "component_id": "comp_123",
    "actor_type": "web",
    "status": "running",
    "uptime": "2h 30m",
    "health_check": "healthy",
    "container_ref": {
      "id": "container_456",
      "name": "web-actor-container"
    }
  }
}
```

#### 4.7 获取Actor组件日志

**请求：**
```
GET /application/apps/{id}/components/{componentId}/logs?lines=100
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "component_id": "comp_123",
    "actor_type": "web",
    "logs": [
      {
        "timestamp": "2024-01-15T14:30:00Z",
        "level": "info",
        "message": "Web Actor server started on port 3000"
      }
    ]
  }
}
```

#### 4.8 获取Actor组件资源使用

**请求：**
```
GET /application/apps/{id}/components/{componentId}/resource-usage
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "component_id": "comp_123",
    "actor_type": "web",
    "cpu_usage": 45.2,
    "memory_usage": 67.8,
    "network_io": {
      "rx_bytes": 1024000,
      "tx_bytes": 512000
    },
    "disk_io": {
      "read_bytes": 2048000,
      "write_bytes": 1024000
    }
  }
}
```

#### 4.9 获取所有Actor组件资源使用

**请求：**
```
GET /application/components/resource-usage
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "components": [
      {
        "component_id": "comp_123",
        "application_id": "app_123",
        "actor_type": "web",
        "cpu_usage": 45.2,
        "memory_usage": 67.8,
        "container_ref": {
          "id": "container_456",
          "name": "web-actor-container"
        }
      }
    ]
  }
}
```

### 5. Peer节点管理API

#### 5.1 获取Peer节点列表

**请求：**

```
GET /peer/nodes
```

**响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "nodes": [
      {
        "address": "192.168.1.100:8080",
        "status": "unknown"
      },
      {
        "address": "192.168.1.101:8080",
        "status": "unknown"
      }
    ],
    "total": 2
  }
}
```

#### 5.2 添加Peer节点

**请求：**
```
POST /peer/nodes
Content-Type: application/json

{
  "address": "192.168.1.102:8080"
}
```

**响应：**
```json
{
  "code": 200,
  "message": "Peer node added successfully",
  "data": {
    "address": "192.168.1.102:8080"
  }
}
```

#### 5.3 删除Peer节点

**请求：**
```
DELETE /peer/nodes/{address}
```

**响应：**
```json
{
  "code": 200,
  "message": "Peer node removed successfully",
  "data": {
    "address": "192.168.1.102:8080"
  }
}
```

**错误响应：**
```json
{
  "code": 404,
  "message": "peer node not found",
  "data": null
}
```

---

## 前端Next.js API (端口3000)

### 1. 资源管理API

#### 1.1 获取所有资源

**请求：**
```
GET /api/resources
```

**响应：**

```json
{
  "success": true,
  "data": [
    {
      "id": "1",
      "name": "本地Docker",
      "type": "docker",
      "host": "localhost",
      "port": 2376,
      "status": "connected",
      "cpu": {"total": 8, "used": 2},
      "memory": {"total": 16, "used": 4},
      "storage": {"total": 500, "used": 120},
      "lastUpdated": "2024-01-15T14:30:00Z"
    }
  ]
}
```

#### 1.2 创建新资源

**请求：**
```
POST /api/resources
Content-Type: application/json

{
  "name": "生产环境集群",
  "type": "kubernetes",
  "url": "https://k8s-prod.example.com",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "description": "生产环境Kubernetes集群"
}
```

**响应：**
```json
{
  "success": true,
  "data": {
    "id": "2",
    "name": "生产环境集群",
    "type": "kubernetes",
    "url": "https://k8s-prod.example.com",
    "status": "connecting",
    "created_at": "2024-01-15T14:30:00Z"
  }
}
```

### 2. 应用管理API

#### 2.1 获取所有应用

**请求：**
```
GET /api/applications
```

**响应：**
```json
{
  "success": true,
  "data": [
    {
      "id": "1",
      "name": "用户管理系统",
      "description": "基于React和Node.js的用户管理后台系统",
      "status": "running",
      "type": "web",
      "gitUrl": "https://github.com/company/user-management",
      "branch": "main",
      "ports": [3000],
      "healthCheck": "/health",
      "lastDeployed": "2024-01-15 14:30:00",
      "runningOn": ["本地Docker"]
    }
  ]
}
```

#### 2.2 创建新应用

**请求：**
```
POST /api/applications
Content-Type: application/json

{
  "name": "博客系统",
  "description": "基于Next.js的个人博客系统",
  "gitUrl": "https://github.com/user/blog",
  "branch": "main",
  "type": "web",
  "ports": [3000],
  "healthCheck": "/api/health"
}
```

**响应：**
```json
{
  "success": true,
  "data": {
    "id": "2",
    "name": "博客系统",
    "status": "idle",
    "created_at": "2024-01-15T14:30:00Z"
  }
}
```

#### 2.3 部署应用

**请求：**
```
POST /api/applications/{id}/deploy
```

**响应：**
```json
{
  "success": true,
  "data": {
    "deploymentId": "deploy_123",
    "status": "deploying",
    "message": "应用正在部署中..."
  }
}
```

#### 2.4 停止应用

**请求：**
```
POST /api/applications/{id}/stop
```

**响应：**
```json
{
  "success": true,
  "data": {
    "status": "stopping",
    "message": "应用正在停止中..."
  }
}
```

### 3. Actor组件管理API

#### 3.1 获取应用Actor组件列表

**请求：**
```
GET /api/applications/{id}/components
```

**响应：**
```json
{
  "success": true,
  "data": {
    "components": [
      {
        "id": "comp_123",
        "name": "web-server",
        "actor_type": "web",
        "status": "running",
        "image": "node:18-alpine",
        "ports": [3000],
        "dependencies": ["api-server"],
        "resources": {
          "cpu": 1.0,
          "memory": 1.0,
          "gpu": 0
        }
      }
    ]
  }
}
```

#### 3.2 启动Actor组件

**请求：**
```
POST /api/components/{componentId}/start
```

**响应：**
```json
{
  "success": true,
  "data": {
    "component_id": "comp_123",
    "actor_type": "web",
    "status": "starting",
    "message": "Actor组件正在启动中..."
  }
}
```

#### 3.3 停止Actor组件

**请求：**
```
POST /api/components/{componentId}/stop
```

**响应：**
```json
{
  "success": true,
  "data": {
    "component_id": "comp_123",
    "actor_type": "web",
    "status": "stopping",
    "message": "Actor组件正在停止中..."
  }
}
```

#### 3.4 获取Actor组件状态

**请求：**
```
GET /api/components/{componentId}/status
```

**响应：**
```json
{
  "success": true,
  "data": {
    "component_id": "comp_123",
    "actor_type": "web",
    "status": "running",
    "uptime": "2h 30m",
    "health_check": "healthy"
  }
}
```

#### 3.5 获取Actor组件日志

**请求：**
```
GET /api/components/{componentId}/logs?lines=100
```

**响应：**
```json
{
  "success": true,
  "data": {
    "component_id": "comp_123",
    "actor_type": "web",
    "logs": [
      {
        "timestamp": "2024-01-15T14:30:00Z",
        "level": "info",
        "message": "Web Actor server started on port 3000"
      }
    ]
  }
}
```

### 4. 状态监控API

#### 4.1 获取所有应用状态

**请求：**
```
GET /api/status
```

**响应：**
```json
{
  "success": true,
  "data": [
    {
      "id": "1",
      "name": "用户管理系统",
      "status": "running",
      "uptime": "7天 12小时 30分钟",
      "cpu": 45,
      "memory": 67,
      "network": 23,
      "storage": 34,
      "instances": 3,
      "healthCheck": "healthy",
      "lastRestart": "2024-01-08 09:15:00",
      "runningOn": ["生产环境集群"],
      "logs": [
        {
          "timestamp": "2024-01-15T14:30:00Z",
          "level": "info",
          "message": "Application is running normally"
        }
      ]
    }
  ]
}
```

#### 4.2 重启应用

**请求：**
```
POST /api/status/{id}/restart
```

**响应：**
```json
{
  "success": true,
  "data": {
    "status": "restarting",
    "message": "应用正在重启中..."
  }
}
```

### 5. Peer节点管理API

#### 5.1 获取Peer节点列表

**请求：**
```
GET /api/peer/nodes
```

**响应：**
```json
{
  "success": true,
  "data": {
    "nodes": [
      {
        "address": "192.168.1.100:8080",
        "status": "unknown"
      },
      {
        "address": "192.168.1.101:8080",
        "status": "unknown"
      }
    ],
    "total": 2
  }
}
```

#### 5.2 添加Peer节点

**请求：**
```
POST /api/peer/nodes
Content-Type: application/json

{
  "address": "192.168.1.102:8080"
}
```

**响应：**
```json
{
  "success": true,
  "data": {
    "message": "Peer node added successfully",
    "address": "192.168.1.102:8080"
  }
}
```

#### 4.3 删除Peer节点

**请求：**
```
DELETE /api/peer/nodes/{address}
```

**响应：**
```json
{
  "success": true,
  "data": {
    "message": "Peer node removed successfully",
    "address": "192.168.1.102:8080"
  }
}
```

**错误响应：**
```json
{
  "success": false,
  "error": {
    "code": 404,
    "message": "peer node not found"
  }
}
```

## 数据模型

### Resource（资源）
```typescript
interface Resource {
  id: string;
  name: string;
  type: 'docker' | 'kubernetes' | 'vm';
  host?: string;
  port?: number;
  url?: string;
  token?: string;
  status: 'connected' | 'disconnected' | 'connecting' | 'error';
  cpu: {
    total: number;
    used: number;
  };
  memory: {
    total: number; // GB
    used: number;  // GB
  };
  storage: {
    total: number; // GB
    used: number;  // GB
  };
  description?: string;
  lastUpdated: string;
  createdAt: string;
}
```

### Application（应用）
```typescript
interface Application {
  id: string;
  name: string;
  description?: string;
  gitUrl: string;
  branch: string;
  type: 'web' | 'api' | 'service' | 'database';
  ports: number[];
  healthCheck?: string;
  status: 'idle' | 'building' | 'deploying' | 'running' | 'stopped' | 'error';
  lastDeployed?: string;
  runningOn: string[]; // Resource IDs
  createdAt: string;
  updatedAt: string;
}
```

### ApplicationStatus（应用状态）
```typescript
interface ApplicationStatus {
  id: string;
  name: string;
  status: 'running' | 'stopped' | 'error' | 'restarting';
  uptime: string;
  cpu: number;     // 百分比
  memory: number;  // 百分比
  network: number; // 百分比
  storage: number; // 百分比
  instances: number;
  healthCheck: 'healthy' | 'unhealthy' | 'unknown';
  lastRestart: string;
  runningOn: string[];
  logs: LogEntry[];
}

interface LogEntry {
  timestamp: string;
  level: 'info' | 'warn' | 'error' | 'debug';
  message: string;
}
```

### PeerNode（Peer节点）
```typescript
interface PeerNode {
  address: string; // peer节点地址，格式: host:port
  status: 'connected' | 'disconnected' | 'unknown'; // 节点状态
}

interface PeerNodesResponse {
  nodes: PeerNode[];
  total: number;
}

interface AddPeerNodeRequest {
  address: string; // peer节点地址，格式: host:port
}

interface AddPeerNodeResponse {
  message: string; // 响应消息
  address: string; // 添加的节点地址
}

interface RemovePeerNodeResponse {
  message: string; // 响应消息
  address: string; // 删除的节点地址
}
```

### Component（组件）
```typescript
interface Component {
  id: string;
  name: string;
  type: 'service' | 'database' | 'cache' | 'queue';
  applicationId: string;
  status: 'running' | 'stopped' | 'error' | 'starting' | 'stopping';
  port?: number;
  healthCheck?: string;
  dockerfile?: string;
  resourceUsage: {
    cpu: number;
    memory: number;
    network: {
      rxBytes: number;
      txBytes: number;
    };
    disk: {
      readBytes: number;
      writeBytes: number;
    };
  };
  createdAt: string;
  updatedAt: string;
}
```
---

## gRPC接口协议

### 概述

IARNet系统中的peer节点之间通过gRPC协议进行通信，主要用于peer发现、资源提供者交换和远程方法调用。gRPC服务定义在proto文件中，包含两个主要服务：

- **PeerService**: 处理peer节点间的通信
- **CodeAnalysisService**: 处理代码分析相关的服务

### 1. PeerService gRPC接口

#### 1.1 ExchangePeers - 交换Peer节点信息

**服务定义：**
```protobuf
rpc ExchangePeers(ExchangeRequest) returns (ExchangeResponse);
```

**请求消息：**
```protobuf
message ExchangeRequest {
  repeated string known_peers = 1;  // 已知peer地址列表 (例如: "host:port")
}
```

**响应消息：**
```protobuf
message ExchangeResponse {
  repeated string known_peers = 1;  // 返回的peer地址列表
}
```

**使用示例：**
```json
// 请求
{
  "known_peers": [
    "192.168.1.100:8080",
    "192.168.1.101:8080"
  ]
}

// 响应
{
  "known_peers": [
    "192.168.1.100:8080",
    "192.168.1.101:8080",
    "192.168.1.102:8080",
    "192.168.1.103:8080"
  ]
}
```

#### 1.2 ExchangeProviders - 交换资源提供者信息

**服务定义：**
```protobuf
rpc ExchangeProviders(ProviderExchangeRequest) returns (ProviderExchangeResponse);
```

**请求消息：**
```protobuf
message ProviderExchangeRequest {
  repeated ignis.ProviderInfo providers = 1;  // 本地提供者信息列表
}
```

**响应消息：**
```protobuf
message ProviderExchangeResponse {
  repeated ignis.ProviderInfo providers = 1;  // 远程提供者信息列表
}
```

**ProviderInfo结构：**
```protobuf
message ProviderInfo {
  string id = 1;           // 提供者唯一标识
  string name = 2;         // 提供者名称
  string type = 3;         // 提供者类型 (docker, kubernetes等)
  string host = 4;         // 主机地址
  int32 port = 5;          // 端口号
  int32 status = 6;        // 状态码
  string peer_address = 7; // 管理此提供者的peer地址
}
```

**使用示例：**
```json
// 请求
{
  "providers": [
    {
      "id": "docker-local-001",
      "name": "本地Docker",
      "type": "docker",
      "host": "localhost",
      "port": 2376,
      "status": 1,
      "peer_address": "192.168.1.100:8080"
    }
  ]
}

// 响应
{
  "providers": [
    {
      "id": "k8s-cluster-001",
      "name": "生产环境集群",
      "type": "kubernetes",
      "host": "k8s-master.example.com",
      "port": 6443,
      "status": 1,
      "peer_address": "192.168.1.101:8080"
    }
  ]
}
```

#### 1.3 CallProvider - 远程提供者方法调用

**服务定义：**
```protobuf
rpc CallProvider(ProviderCallRequest) returns (ProviderCallResponse);
```

**请求消息：**
```protobuf
message ProviderCallRequest {
  string provider_id = 1;  // 目标提供者ID
  string method = 2;       // 调用的方法名
  bytes payload = 3;       // 序列化的方法参数
}
```

**响应消息：**
```protobuf
message ProviderCallResponse {
  bool success = 1;   // 调用是否成功
  string error = 2;   // 错误信息（如果有）
  bytes result = 3;   // 序列化的方法结果
}
```

**使用示例：**
```json
// 请求
{
  "provider_id": "docker-remote-001",
  "method": "CreateContainer",
  "payload": "<base64编码的参数数据>"
}

// 响应
{
  "success": true,
  "error": "",
  "result": "<base64编码的结果数据>"
}
```

### 2. CodeAnalysisService gRPC接口

#### 2.1 AnalyzeCode - 代码分析

**服务定义：**
```protobuf
rpc AnalyzeCode(CodeAnalysisRequest) returns (CodeAnalysisResponse);
```

**请求消息：**
```protobuf
message CodeAnalysisRequest {
  string application_id = 1;                    // 应用ID
  string code_content = 2;                      // Base64编码的代码归档
  repeated ProviderInfo available_providers = 3; // 可用的提供者列表
  map<string, string> metadata = 4;            // 额外元数据（语言、框架等）
}
```

**响应消息：**
```protobuf
message CodeAnalysisResponse {
  bool success = 1;                        // 分析是否成功
  string error = 2;                        // 错误信息
  repeated Component components = 3;        // 分析出的组件列表
  repeated DAGEdge edges = 4;              // 组件间的依赖关系
  map<string, string> global_config = 5;   // 全局配置
  string analysis_metadata = 6;            // 分析元数据（JSON格式）
}
```

**Component结构：**
```protobuf
message Component {
  string id = 1;                           // 组件ID
  string name = 2;                         // 组件名称
  string type = 3;                         // 组件类型 (web, api, worker等)
  string image = 4;                        // Docker镜像或构建指令
  repeated string dependencies = 5;        // 依赖的组件ID列表
  repeated int32 ports = 6;               // 端口列表
  map<string, string> environment = 7;     // 环境变量
  ResourceRequirements resources = 8;      // 资源需求
  string provider_type = 9;               // 首选提供者类型
  string provider_id = 10;                // 特定提供者ID
  bytes deployment_config = 11;           // 序列化的部署配置
}
```

**ResourceRequirements结构：**
```protobuf
message ResourceRequirements {
  double cpu = 1;     // CPU需求
  double memory = 2;  // 内存需求（GB）
  double gpu = 3;     // GPU需求
  double storage = 4; // 存储需求（GB）
}
```

**DAGEdge结构：**
```protobuf
message DAGEdge {
  string from_component = 1;               // 源组件
  string to_component = 2;                 // 目标组件
  string connection_type = 3;              // 连接类型 (http, grpc, database等)
  map<string, string> connection_config = 4; // 连接配置
}
```

### 3. gRPC连接配置

#### 3.1 服务端配置

```yaml
grpc:
  port: 8080
  max_recv_msg_size: 4194304  # 4MB
  max_send_msg_size: 4194304  # 4MB
  keepalive:
    time: 30s
    timeout: 5s
    permit_without_stream: true
```

#### 3.2 客户端配置

```yaml
grpc_client:
  timeout: 30s
  keepalive:
    time: 30s
    timeout: 5s
    permit_without_stream: true
  retry:
    max_attempts: 3
    initial_backoff: 1s
    max_backoff: 10s
```

#### 3.3 错误处理

gRPC服务使用标准的gRPC状态码：

- `OK (0)`: 成功
- `INVALID_ARGUMENT (3)`: 无效参数
- `NOT_FOUND (5)`: 资源未找到
- `ALREADY_EXISTS (6)`: 资源已存在
- `PERMISSION_DENIED (7)`: 权限拒绝
- `RESOURCE_EXHAUSTED (8)`: 资源耗尽
- `FAILED_PRECONDITION (9)`: 前置条件失败
- `UNAVAILABLE (14)`: 服务不可用
- `INTERNAL (13)`: 内部错误

---

## 认证和授权

目前系统处于开发阶段，暂未实现认证和授权机制。在生产环境中，建议实现以下安全措施：

1. **API密钥认证**：为每个客户端分配唯一的API密钥
2. **JWT令牌**：使用JWT令牌进行用户身份验证
3. **RBAC权限控制**：基于角色的访问控制
4. **HTTPS加密**：所有API通信使用HTTPS加密
5. **请求限流**：防止API滥用和DDoS攻击

## 版本控制

当前API版本：`v1`

未来版本更新时，将通过以下方式保持向后兼容：
- 在URL中包含版本号：`/api/v2/resources`
- 通过HTTP头指定版本：`Accept: application/vnd.iarnet.v2+json`
- 维护多个版本的并行支持

## 联系信息

如有API相关问题，请联系开发团队或查看项目文档。