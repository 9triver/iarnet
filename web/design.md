# IARNet 算力网络资源管理与智能应用部署平台设计文档

## 1. 项目概述

### 1.1 项目简介
IARNet（Intelligent Application Resource Network）是一个现代化的算力网络资源管理与智能应用部署平台。该平台旨在统一管理分布式算力资源，并提供便捷的应用部署和运行状态监控功能。

### 1.2 核心价值
- **资源统一管理**：将分散的算力资源（CPU/GPU/存储/网络）纳入统一管理
- **应用快速部署**：支持从Git仓库快速导入和部署应用
- **智能资源调度**：自动选择合适的资源运行应用
- **实时状态监控**：提供应用运行状态和资源使用情况的实时监控

### 1.3 目标用户
- 企业IT运维人员
- 开发团队
- 系统管理员
- DevOps工程师

## 2. 系统架构

### 2.1 技术栈

#### 前端技术栈
- **框架**: Next.js 15 + React 19
- **语言**: TypeScript
- **样式**: Tailwind CSS
- **UI组件**: shadcn/ui
- **状态管理**: Zustand
- **表单处理**: React Hook Form + Zod
- **图标**: Lucide React
- **字体**: Geist Sans + Playfair Display

#### 后端技术栈
- **API**: Next.js API Routes
- **架构**: RESTful API
- **数据持久化**: localStorage（演示版本）

### 2.2 系统架构图

```
┌─────────────────────────────────────────────────────────────┐
│                    IARNet 前端界面                          │
├─────────────────┬─────────────────┬─────────────────────────┤
│   资源管理页面   │   应用管理页面   │   运行状态监控页面        │
└─────────────────┴─────────────────┴─────────────────────────┘
                           │
┌─────────────────────────────────────────────────────────────┐
│                    Next.js API Routes                      │
├─────────────────┬─────────────────┬─────────────────────────┤
│  /api/resources │ /api/applications│    /api/status          │
└─────────────────┴─────────────────┴─────────────────────────┘
                           │
┌─────────────────────────────────────────────────────────────┐
│                    算力资源层                                │
├─────────────────┬─────────────────┬─────────────────────────┤
│  Kubernetes集群  │   Docker环境     │      虚拟机             │
└─────────────────┴─────────────────┴─────────────────────────┘
```

### 2.3 模块架构

#### 2.3.1 前端模块结构
```
app/
├── layout.tsx              # 根布局组件
├── page.tsx               # 首页
├── resources/             # 资源管理模块
│   └── page.tsx
├── applications/          # 应用管理模块
│   └── page.tsx
├── status/               # 状态监控模块
│   └── page.tsx
└── api/                  # API路由
    ├── resources/
    ├── applications/
    └── status/

components/
├── sidebar.tsx           # 侧边栏导航
├── providers.tsx         # 全局状态提供者
└── ui/                   # UI组件库

lib/
├── store.ts             # Zustand状态管理
├── api.ts               # API客户端
└── utils.ts             # 工具函数
```

## 3. 功能模块设计

### 3.1 算力资源管理模块

#### 3.1.1 功能概述
负责算力资源的接入、配置和管理，支持多种资源类型的统一管理。

#### 3.1.2 核心功能
- **资源接入**：通过API Server URL和Token接入资源
- **资源类型支持**：
  - Kubernetes集群
  - Docker环境
  - 虚拟机
- **资源监控**：实时监控CPU、内存、存储使用情况
- **连接状态管理**：监控资源连接状态

#### 3.1.3 数据模型
```typescript
interface Resource {
  id: string
  name: string                    // 资源名称
  type: "kubernetes" | "docker" | "vm"  // 资源类型
  url: string                     // API Server URL
  status: "connected" | "disconnected" | "error"  // 连接状态
  cpu: {
    total: number                 // 总CPU核心数
    used: number                  // 已使用CPU核心数
  }
  memory: {
    total: number                 // 总内存(GB)
    used: number                  // 已使用内存(GB)
  }
  storage: {
    total: number                 // 总存储(GB)
    used: number                  // 已使用存储(GB)
  }
  lastUpdated: string             // 最后更新时间
}
```

#### 3.1.4 用户界面设计
- **统计卡片**：显示总资源数、在线资源、CPU核心数、总存储
- **资源列表**：表格形式展示所有资源及其状态
- **添加资源对话框**：表单形式收集资源信息
- **操作按钮**：编辑、删除、刷新资源

### 3.2 应用管理模块

#### 3.2.1 功能概述
提供应用的导入、配置、部署和生命周期管理功能。

#### 3.2.2 核心功能
- **Git仓库导入**：支持GitHub、GitLab、Bitbucket
- **应用类型支持**：
  - Web应用
  - API服务
  - 后台任务
  - 数据库
- **部署管理**：一键部署、停止、重启
- **配置管理**：端口、健康检查、分支管理

#### 3.2.3 数据模型
```typescript
interface Application {
  id: string
  name: string                    // 应用名称
  description: string             // 应用描述
  gitUrl: string                  // Git仓库URL
  branch: string                  // Git分支
  status: "idle" | "running" | "stopped" | "error" | "deploying"  // 运行状态
  type: "web" | "api" | "worker" | "database"  // 应用类型
  lastDeployed?: string           // 最后部署时间
  runningOn?: string[]            // 运行的资源节点
  port?: number                   // 端口号
  healthCheck?: string            // 健康检查路径
}
```

#### 3.2.4 用户界面设计
- **快速导入**：简化的Git URL输入对话框
- **高级配置**：详细的应用配置表单
- **应用卡片**：卡片式展示应用信息和操作按钮
- **统计面板**：应用总数、运行中、未部署等统计信息

### 3.3 运行状态监控模块

#### 3.3.1 功能概述
实时监控应用运行状态、资源使用情况和性能指标。

#### 3.3.2 核心功能
- **实时状态监控**：应用运行状态实时更新
- **资源使用监控**：CPU、内存、网络、存储使用率
- **健康检查**：应用健康状态检测
- **日志管理**：应用运行日志查看
- **性能指标**：图表展示性能趋势
- **拓扑可视化**：应用架构拓扑图

#### 3.3.3 数据模型
```typescript
interface ApplicationStatus {
  id: string
  name: string                    // 应用名称
  status: "running" | "stopped" | "error" | "warning"  // 运行状态
  uptime: string                  // 运行时间
  cpu: number                     // CPU使用率(%)
  memory: number                  // 内存使用率(%)
  network: number                 // 网络使用率(%)
  storage: number                 // 存储使用率(%)
  instances: number               // 实例数量
  healthCheck: "healthy" | "unhealthy" | "unknown"  // 健康状态
  lastRestart: string             // 最后重启时间
  runningOn: string[]             // 运行节点
  logs: LogEntry[]                // 日志条目
  metrics: MetricData[]           // 性能指标
}

interface LogEntry {
  timestamp: string               // 时间戳
  level: "info" | "warn" | "error"  // 日志级别
  message: string                 // 日志消息
}

interface MetricData {
  timestamp: string               // 时间戳
  cpu: number                     // CPU使用率
  memory: number                  // 内存使用率
  network: number                 // 网络流量
  requests: number                // 请求数
}
```

#### 3.3.4 用户界面设计
- **状态概览**：运行中、警告、错误应用数量统计
- **应用状态表**：详细的应用状态列表
- **拓扑图**：可视化应用架构和组件关系
- **自动刷新**：可配置的自动数据刷新

## 4. 用户界面设计

### 4.1 设计原则
- **简洁直观**：界面简洁，操作直观
- **响应式设计**：支持桌面和移动端
- **实时反馈**：提供实时的状态反馈
- **一致性**：保持界面元素的一致性

### 4.2 布局结构
```
┌─────────────────────────────────────────────────────────────┐
│                        顶部标题栏                            │
├─────────────┬───────────────────────────────────────────────┤
│             │                                               │
│             │                                               │
│   侧边栏     │              主内容区域                        │
│   导航      │                                               │
│             │                                               │
│             │                                               │
└─────────────┴───────────────────────────────────────────────┘
```

### 4.3 组件设计

#### 4.3.1 侧边栏导航
- 可折叠设计
- 图标+文字的导航项
- 当前页面高亮显示
- 版本信息显示

#### 4.3.2 统计卡片
- 数值+图标的组合
- 颜色编码状态
- 简洁的描述文字

#### 4.3.3 数据表格
- 排序和筛选功能
- 操作按钮集成
- 状态徽章显示
- 响应式列布局

#### 4.3.4 对话框表单
- 分步骤的表单设计
- 实时验证反馈
- 清晰的错误提示
- 取消和确认操作

### 4.4 主题设计
- **主色调**：蓝色系（专业、可靠）
- **辅助色**：绿色（成功）、黄色（警告）、红色（错误）
- **字体**：Geist Sans（现代感）+ Playfair Display（标题）
- **圆角**：适度的圆角设计
- **阴影**：轻微的阴影效果

## 5. API设计

### 5.1 API架构
采用RESTful API设计，后端Go服务提供核心API，前端Next.js提供代理API。

**后端Go服务API（端口8080）：**
- 直接处理业务逻辑
- 提供资源管理、应用管理等核心功能
- 使用统一的响应格式：`{"code": 200, "message": "success", "data": {...}}`

**前端Next.js API（端口3000）：**
- 提供`/api`路径的代理接口
- 处理前端特定的数据格式转换
- 兼容前端组件的数据结构

### 5.2 资源管理API

#### 后端Go服务接口
```
GET    /resource/capacity       # 获取资源容量信息
GET    /resource/providers      # 获取所有资源提供者
POST   /resource/providers      # 注册新的资源提供者
DELETE /resource/providers/{id} # 注销资源提供者
```

#### 前端Next.js代理接口
```
GET    /api/resources           # 获取所有资源（代理到后端）
POST   /api/resources           # 创建新资源（代理到后端）
PUT    /api/resources/{id}      # 更新资源
DELETE /api/resources/{id}      # 删除资源
```

#### 请求/响应示例

**注册Docker资源提供者：**
```json
// POST /resource/providers
{
  "type": "docker",
  "config": {
    "host": "tcp://192.168.1.100:2376",
    "tlsCertPath": "/path/to/certs",
    "tlsVerify": true,
    "apiVersion": "1.41"
  }
}

// Response
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

**注册Kubernetes资源提供者：**
```json
// POST /resource/providers
{
  "type": "k8s",
  "config": {
    "kubeConfigContent": "apiVersion: v1\nkind: Config...",
    "namespace": "default",
    "context": "my-context"
  }
}
```

**获取资源提供者列表：**
```json
// GET /resource/providers
// Response
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

### 5.3 应用管理API

#### 后端Go服务接口
```
GET    /application/apps        # 获取所有应用
POST   /application/apps        # 创建新应用
GET    /application/apps/{id}   # 获取单个应用详情
DELETE /application/apps/{id}   # 删除应用
GET    /application/stats       # 获取应用统计信息

# 文件管理
GET    /application/apps/{id}/files         # 获取应用文件树
GET    /application/apps/{id}/files/content # 获取文件内容

# 组件管理
GET    /application/apps/{id}/components                    # 获取应用组件
POST   /application/apps/{id}/analyze                       # 分析应用架构
POST   /application/apps/{id}/deploy-components             # 部署组件
POST   /application/apps/{id}/components/{componentId}/start # 启动组件
POST   /application/apps/{id}/components/{componentId}/stop  # 停止组件
GET    /application/apps/{id}/components/{componentId}/status # 获取组件状态
GET    /application/apps/{id}/components/{componentId}/logs   # 获取组件日志
GET    /application/apps/{id}/components/{componentId}/resource-usage # 获取组件资源使用
GET    /application/components/resource-usage               # 获取所有组件资源使用
```

#### 前端Next.js代理接口
```
GET    /api/applications        # 获取所有应用
POST   /api/applications        # 创建新应用
POST   /api/applications/{id}/deploy # 部署应用
POST   /api/applications/{id}/stop   # 停止应用
```

#### 请求/响应示例

**创建应用：**
```json
// POST /application/apps
{
  "name": "用户管理系统",
  "description": "基于React和Node.js的用户管理后台系统",
  "git_url": "https://github.com/company/user-management",
  "branch": "main",
  "type": "web",
  "ports": [3000],
  "health_check": "/health"
}

// Response
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

**获取应用列表：**
```json
// GET /application/apps
// Response
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

### 5.4 状态监控API

#### 前端Next.js接口
```
GET    /api/status              # 获取所有应用状态
POST   /api/status/{id}/restart # 重启应用
```

#### 响应示例
```json
// GET /api/status
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

### 5.5 Peer节点管理API

#### 后端Go服务接口
```
GET    /peer/nodes              # 获取peer节点列表
POST   /peer/nodes              # 添加peer节点
DELETE /peer/nodes/{address}    # 删除peer节点
```

#### 前端Next.js代理接口
```
GET    /api/peer/nodes          # 获取peer节点列表
POST   /api/peer/nodes          # 添加peer节点
DELETE /api/peer/nodes/{address} # 删除peer节点
```

#### 请求/响应示例

**获取peer节点列表：**
```bash
curl -X GET http://localhost:8080/peer/nodes
```

```json
// Response
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

**添加peer节点：**
```bash
curl -X POST http://localhost:8080/peer/nodes \
  -H "Content-Type: application/json" \
  -d '{"address": "192.168.1.102:8080"}'
```

```json
// Response
{
  "code": 200,
  "message": "Peer node added successfully",
  "data": {
    "address": "192.168.1.102:8080"
  }
}
```

**删除peer节点：**
```bash
curl -X DELETE http://localhost:8080/peer/nodes/192.168.1.102:8080
```

```json
// Response
{
  "code": 200,
  "message": "Peer node removed successfully",
  "data": {
    "address": "192.168.1.102:8080"
  }
}
```

### 5.6 统一错误处理

#### 后端Go服务错误格式
```json
{
  "code": 404,
  "message": "Resource not found",
  "data": null
}
```

#### 前端Next.js错误格式
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

#### 常见错误码
- `200`: 成功
- `400`: 请求参数错误
- `401`: 未授权
- `403`: 禁止访问
- `404`: 资源不存在
- `500`: 服务器内部错误

## 6. 状态管理

### 6.1 Zustand Store设计
使用Zustand进行全局状态管理，支持数据持久化。

### 6.2 状态结构
```typescript
interface IARNetStore {
  // 数据状态
  resources: Resource[]
  applications: Application[]
  applicationStatuses: ApplicationStatus[]
  
  // 加载状态
  loadingStates: Record<string, boolean>
  
  // 错误状态
  errors: Record<string, string>
  
  // 操作方法
  addResource: (resource: Omit<Resource, "id">) => void
  updateResource: (id: string, resource: Partial<Resource>) => void
  deleteResource: (id: string) => void
  
  addApplication: (app: Omit<Application, "id">) => void
  updateApplication: (id: string, app: Partial<Application>) => void
  deleteApplication: (id: string) => void
  deployApplication: (id: string) => Promise<void>
  stopApplication: (id: string) => void
  
  updateApplicationStatus: (id: string, status: Partial<ApplicationStatus>) => void
  restartApplication: (id: string) => void
  
  // 异步操作
  fetchResources: () => Promise<void>
  fetchApplications: () => Promise<void>
  fetchApplicationStatuses: () => Promise<void>
  refreshData: () => Promise<void>
}
```

### 6.3 数据持久化
使用Zustand的persist中间件，将关键数据持久化到localStorage。

## 7. 部署和运维

### 7.1 开发环境
```bash
# 安装依赖
pnpm install

# 启动开发服务器
pnpm dev

# 构建生产版本
pnpm build

# 启动生产服务器
pnpm start
```

### 7.2 环境要求
- Node.js 18+
- pnpm 8+
- 现代浏览器支持

### 7.3 配置文件
- `next.config.mjs`：Next.js配置
- `tsconfig.json`：TypeScript配置
- `tailwind.config.js`：Tailwind CSS配置
- `components.json`：shadcn/ui配置

## 8. 安全考虑

### 8.1 认证和授权
- API Token验证
- 资源访问权限控制
- 敏感信息加密存储

### 8.2 数据安全
- HTTPS通信
- 输入验证和清理
- XSS和CSRF防护

### 8.3 网络安全
- API访问频率限制
- 网络隔离
- 日志审计

## 9. 性能优化

### 9.1 前端优化
- 组件懒加载
- 图片优化
- 代码分割
- 缓存策略

### 9.2 后端优化
- API响应缓存
- 数据库查询优化
- 连接池管理

### 9.3 监控指标
- 页面加载时间
- API响应时间
- 资源使用率
- 错误率统计

## 10. 扩展性设计

### 10.1 模块化架构
- 松耦合的模块设计
- 插件化扩展机制
- 标准化的接口定义

### 10.2 水平扩展
- 微服务架构支持
- 负载均衡
- 分布式部署

### 10.3 功能扩展
- 多租户支持
- 权限管理系统
- 审计日志
- 报表系统

## 11. 测试策略

### 11.1 单元测试
- 组件测试
- 工具函数测试
- API测试

### 11.2 集成测试
- 页面流程测试
- API集成测试
- 数据流测试

### 11.3 端到端测试
- 用户场景测试
- 浏览器兼容性测试
- 性能测试

## 12. 关键性代码说明与解释

### 12.1 核心组件实现

#### 12.1.1 侧边栏导航组件 (components/sidebar.tsx)

```typescript
"use client"

import { useState } from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { cn } from "@/lib/utils"

const navigation = [
  {
    name: "算力资源管理",
    href: "/resources",
    icon: Server,
    description: "接入和管理算力资源",
  },
  // ... 其他导航项
]

export function Sidebar() {
  const [isCollapsed, setIsCollapsed] = useState(false)
  const pathname = usePathname()
  
  return (
    <div className={cn(
      "flex flex-col h-screen bg-sidebar border-r border-sidebar-border transition-all duration-300",
      isCollapsed ? "w-16" : "w-64",
    )}>
      {/* 侧边栏内容 */}
    </div>
  )
}
```

**关键特性说明**:
- **响应式折叠**: 使用`isCollapsed`状态控制侧边栏宽度
- **路径高亮**: 通过`usePathname`获取当前路径，实现导航项高亮
- **动画过渡**: 使用Tailwind CSS的`transition-all`实现平滑动画
- **条件渲染**: 根据折叠状态动态显示/隐藏文字内容

#### 12.1.2 资源管理页面 (app/resources/page.tsx)

```typescript
"use client"

import { useState } from "react"
import { useForm } from "react-hook-form"

interface Resource {
  id: string
  name: string
  type: "kubernetes" | "docker" | "vm"
  url: string
  status: "connected" | "disconnected" | "error"
  // ... 其他属性
}

export default function ResourcesPage() {
  const [resources, setResources] = useState<Resource[]>([])
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  
  const form = useForm<ResourceFormData>({
    defaultValues: {
      name: "",
      type: "kubernetes",
      url: "",
      token: "",
    },
  })
  
  const onSubmit = (data: ResourceFormData) => {
    const newResource: Resource = {
      id: Date.now().toString(),
      ...data,
      status: "connected",
      lastUpdated: new Date().toLocaleString(),
    }
    setResources(prev => [...prev, newResource])
    setIsDialogOpen(false)
    form.reset()
  }
  
  return (
    <div className="flex h-screen bg-background">
      <Sidebar />
      <main className="flex-1 overflow-auto">
        {/* 页面内容 */}
      </main>
    </div>
  )
}
```

**关键特性说明**:
- **表单管理**: 使用React Hook Form进行表单状态管理和验证
- **状态更新**: 通过`setResources`更新资源列表，使用函数式更新确保不可变性
- **对话框控制**: 通过`isDialogOpen`状态控制模态框的显示/隐藏
- **数据持久化**: 新增资源后自动生成ID和时间戳

### 12.2 状态管理实现

#### 12.2.1 Zustand Store (lib/store.ts)

```typescript
import { create } from "zustand"
import { persist } from "zustand/middleware"

interface IARNetStore {
  resources: Resource[]
  applications: Application[]
  applicationStatuses: ApplicationStatus[]
  
  addResource: (resource: Omit<Resource, "id">) => void
  updateResource: (id: string, resource: Partial<Resource>) => void
  deleteResource: (id: string) => void
  
  deployApplication: (id: string) => Promise<void>
  fetchResources: () => Promise<void>
}

export const useIARNetStore = create<IARNetStore>()()
  persist(
    (set, get) => ({
      resources: [],
      applications: [],
      applicationStatuses: [],
      
      addResource: (resource) => {
        const newResource: Resource = {
          ...resource,
          id: Date.now().toString(),
        }
        set((state) => ({
          resources: [...state.resources, newResource],
        }))
      },
      
      deployApplication: async (id) => {
        const { setLoadingState, setError } = get()
        try {
          setLoadingState(`deploy-${id}`, true)
          
          set((state) => ({
            applications: state.applications.map((app) =>
              app.id === id
                ? { ...app, status: "deploying" as const }
                : app
            ),
          }))
          
          // 模拟异步部署过程
          setTimeout(() => {
            set((state) => ({
              applications: state.applications.map((app) =>
                app.id === id ? { ...app, status: "running" as const } : app
              ),
            }))
            setLoadingState(`deploy-${id}`, false)
          }, 3000)
        } catch (error) {
          setError(`deploy-${id}`, error.message)
        }
      },
    }),
    {
      name: "iarnet-storage",
      partialize: (state) => ({
        resources: state.resources,
        applications: state.applications,
      }),
    }
  )
)
```

**关键特性说明**:
- **持久化存储**: 使用`persist`中间件将状态保存到localStorage
- **不可变更新**: 使用展开运算符确保状态更新的不可变性
- **异步操作**: `deployApplication`展示了如何处理异步操作和加载状态
- **错误处理**: 集成了统一的错误处理机制
- **选择性持久化**: 通过`partialize`只持久化必要的状态

### 12.3 API路由实现

#### 12.3.1 资源管理API (app/api/resources/route.ts)

```typescript
import { type NextRequest, NextResponse } from "next/server"

// GET /api/resources - 获取所有资源
export async function GET() {
  try {
    // 模拟数据库查询
    const resources = [
      {
        id: "1",
        name: "生产环境集群",
        type: "kubernetes",
        url: "https://k8s-prod.example.com",
        status: "connected",
        cpu: { total: 32, used: 18 },
        memory: { total: 128, used: 76 },
        storage: { total: 2048, used: 1024 },
        lastUpdated: new Date().toISOString(),
      },
    ]

    return NextResponse.json({ success: true, data: resources })
  } catch (error) {
    return NextResponse.json(
      { success: false, error: "Failed to fetch resources" },
      { status: 500 }
    )
  }
}

// POST /api/resources - 创建新资源
export async function POST(request: NextRequest) {
  try {
    const body = await request.json()
    const { name, type, url, token, description } = body

    // 验证必填字段
    if (!name || !type || !url || !token) {
      return NextResponse.json(
        { success: false, error: "Missing required fields" },
        { status: 400 }
      )
    }

    // 创建新资源
    const newResource = {
      id: Date.now().toString(),
      name,
      type,
      url,
      status: "connected",
      cpu: { total: 0, used: 0 },
      memory: { total: 0, used: 0 },
      storage: { total: 0, used: 0 },
      lastUpdated: new Date().toISOString(),
    }

    return NextResponse.json(
      { success: true, data: newResource },
      { status: 201 }
    )
  } catch (error) {
    return NextResponse.json(
      { success: false, error: "Failed to create resource" },
      { status: 500 }
    )
  }
}
```

**关键特性说明**:
- **RESTful设计**: 遵循REST API设计原则，使用HTTP方法表示操作
- **错误处理**: 统一的错误响应格式和HTTP状态码
- **数据验证**: 在API层面进行输入数据验证
- **响应格式**: 统一的成功/失败响应格式

### 12.4 UI组件实现

#### 12.4.1 状态徽章组件

```typescript
const getStatusBadge = (status: Resource["status"]) => {
  switch (status) {
    case "connected":
      return (
        <Badge variant="default" className="bg-green-500">
          已连接
        </Badge>
      )
    case "disconnected":
      return <Badge variant="secondary">已断开</Badge>
    case "error":
      return <Badge variant="destructive">错误</Badge>
  }
}
```

**关键特性说明**:
- **条件渲染**: 根据状态值动态渲染不同样式的徽章
- **语义化颜色**: 使用颜色传达状态信息（绿色=正常，红色=错误）
- **组件复用**: 可在多个地方复用的状态显示逻辑

#### 12.4.2 实时数据更新

```typescript
useEffect(() => {
  if (!autoRefresh) return

  const interval = setInterval(() => {
    // 模拟实时数据更新
    setApplications((prev) =>
      prev.map((app) => ({
        ...app,
        cpu: Math.max(0, Math.min(100, app.cpu + (Math.random() - 0.5) * 10)),
        memory: Math.max(0, Math.min(100, app.memory + (Math.random() - 0.5) * 5)),
        network: Math.max(0, Math.min(100, app.network + (Math.random() - 0.5) * 15)),
      }))
    )
  }, 5000)

  return () => clearInterval(interval)
}, [autoRefresh])
```

**关键特性说明**:
- **定时更新**: 使用`setInterval`实现5秒间隔的数据刷新
- **条件执行**: 通过`autoRefresh`状态控制是否启用自动刷新
- **内存清理**: 在effect清理函数中清除定时器，防止内存泄漏
- **数据边界**: 使用`Math.max`和`Math.min`确保数据在合理范围内

### 12.5 工具函数实现

#### 12.5.1 样式工具函数 (lib/utils.ts)

```typescript
import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
```

**关键特性说明**:
- **条件样式**: 使用`clsx`处理条件样式类名
- **样式合并**: 使用`twMerge`智能合并Tailwind CSS类名
- **类型安全**: 支持多种类型的样式输入

#### 12.5.2 API客户端 (lib/api.ts)

```typescript
class APIError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = "APIError"
  }
}

async function apiRequest<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const url = `${API_BASE}${endpoint}`

  const response = await fetch(url, {
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
    ...options,
  })

  const data = await response.json()

  if (!response.ok) {
    throw new APIError(response.status, data.error || "API request failed")
  }

  return data.data
}
```

**关键特性说明**:
- **自定义错误类**: 扩展Error类，包含HTTP状态码信息
- **泛型支持**: 使用TypeScript泛型确保返回数据的类型安全
- **统一错误处理**: 集中处理API错误，提供一致的错误信息
- **请求封装**: 封装fetch API，简化API调用代码

### 12.6 性能优化实现

#### 12.6.1 组件懒加载

```typescript
// components/lazy-components.tsx
import dynamic from "next/dynamic"
import { LoadingSpinner } from "@/components/ui/loading-spinner"

export const LazyChart = dynamic(
  () => import("@/components/ui/chart").then((mod) => ({ default: mod.Chart })),
  {
    loading: () => <LoadingSpinner />,
    ssr: false,
  }
)
```

**关键特性说明**:
- **动态导入**: 使用Next.js的`dynamic`函数实现组件懒加载
- **加载状态**: 提供加载中的占位组件
- **SSR控制**: 通过`ssr: false`禁用服务端渲染（适用于客户端专用组件）

#### 12.6.2 防抖处理

```typescript
import { useCallback, useRef } from "react"

function useDebounce<T extends (...args: any[]) => void>(
  callback: T,
  delay: number
): T {
  const timeoutRef = useRef<NodeJS.Timeout>()

  return useCallback(
    ((...args) => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current)
      }
      timeoutRef.current = setTimeout(() => callback(...args), delay)
    }) as T,
    [callback, delay]
  )
}
```

**关键特性说明**:
- **防抖机制**: 延迟执行函数，避免频繁调用
- **泛型约束**: 确保返回函数与原函数类型一致
- **内存管理**: 使用useRef存储定时器引用，避免重复创建

### 12.7 错误边界实现

```typescript
// components/ui/error-boundary.tsx
"use client"

import React from "react"

interface ErrorBoundaryState {
  hasError: boolean
  error?: Error
}

export class ErrorBoundary extends React.Component<
  React.PropsWithChildren<{}>,
  ErrorBoundaryState
> {
  constructor(props: React.PropsWithChildren<{}>) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error("Error caught by boundary:", error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex items-center justify-center min-h-screen">
          <div className="text-center">
            <h2 className="text-2xl font-bold text-red-600 mb-4">
              出现了一些问题
            </h2>
            <p className="text-gray-600 mb-4">
              {this.state.error?.message || "未知错误"}
            </p>
            <button
              onClick={() => window.location.reload()}
              className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
            >
              刷新页面
            </button>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
```

**关键特性说明**:
- **错误捕获**: 使用React错误边界捕获组件树中的JavaScript错误
- **优雅降级**: 提供用户友好的错误界面，而不是白屏
- **错误日志**: 将错误信息记录到控制台，便于调试
- **恢复机制**: 提供刷新页面的选项，让用户可以尝试恢复

这些关键代码实现展示了IARNet平台的核心技术架构和最佳实践，包括组件设计、状态管理、API处理、性能优化和错误处理等方面的具体实现方式。

## 13. 维护和支持

### 13.1 文档维护
- API文档更新
- 用户手册维护
- 开发者指南

### 13.2 版本管理
- 语义化版本控制
- 变更日志维护
- 向后兼容性保证

### 13.3 问题跟踪
- Bug报告流程
- 功能请求管理
- 用户反馈收集

---

**文档版本**: v1.0.0  
**最后更新**: 2024年1月15日  
**维护者**: IARNet开发团队