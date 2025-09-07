"use client"

import { useState, useEffect } from "react"
import { useParams, useRouter } from "next/navigation"
import { Sidebar } from "@/components/sidebar"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Progress } from "@/components/ui/progress"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  ArrowLeft,
  Box,
  Cpu,
  Database,
  Globe,
  HardDrive,
  MemoryStick,
  Network,
  Play,
  Square,
  RefreshCw,
  GitBranch,
  Activity,
} from "lucide-react"

// 组件类型定义
interface Component {
  id: string
  name: string
  type: "frontend" | "backend" | "database" | "cache" | "queue" | "gateway"
  status: "running" | "stopped" | "deploying" | "error"
  image: string
  ports: number[]
  dependencies: string[] // 依赖的组件ID
  resources: {
    cpu: number
    memory: number
    gpu?: number
  }
  providerID: string
  containerRef?: {
    id: string
    name: string
  }
  createdAt: string
  updatedAt: string
}

// DAG图边定义
interface DAGEdge {
  from: string
  to: string
  type: "http" | "grpc" | "database" | "queue" | "file"
}

// 应用DAG定义
interface ApplicationDAG {
  applicationID: string
  components: Component[]
  edges: DAGEdge[]
  createdAt: string
  updatedAt: string
}

// 组件状态指示器
function ComponentStatusIndicator({ status }: { status: Component["status"] }) {
  const statusConfig = {
    running: { color: "bg-green-500", text: "运行中" },
    stopped: { color: "bg-gray-500", text: "已停止" },
    deploying: { color: "bg-blue-500", text: "部署中" },
    error: { color: "bg-red-500", text: "错误" },
  }

  const config = statusConfig[status]
  return (
    <div className="flex items-center space-x-2">
      <div className={`w-2 h-2 rounded-full ${config.color}`} />
      <span className="text-sm">{config.text}</span>
    </div>
  )
}

// 组件类型图标
function ComponentTypeIcon({ type }: { type: Component["type"] }) {
  const iconMap = {
    frontend: Globe,
    backend: Box,
    database: Database,
    cache: HardDrive,
    queue: Network,
    gateway: GitBranch,
  }

  const Icon = iconMap[type]
  return <Icon className="w-4 h-4" />
}

// DAG图可视化组件
function DAGVisualization({ dag }: { dag: ApplicationDAG }) {
  const [selectedComponent, setSelectedComponent] = useState<string | null>(null)

  // 简化的DAG布局算法
  const getComponentPosition = (componentId: string, index: number) => {
    const component = dag.components.find(c => c.id === componentId)
    if (!component) return { x: 0, y: 0 }

    // 根据组件类型和依赖关系计算位置
    const typeOrder = { frontend: 0, gateway: 1, backend: 2, cache: 3, database: 4, queue: 5 }
    const layer = typeOrder[component.type] || 0
    const x = 50 + layer * 150
    const y = 50 + (index % 3) * 100

    return { x, y }
  }

  return (
    <div className="relative w-full h-96 border rounded-lg bg-gray-50 overflow-auto">
      <svg className="absolute inset-0 w-full h-full">
        {/* 绘制连接线 */}
        {dag.edges.map((edge, index) => {
          const fromComponent = dag.components.find(c => c.id === edge.from)
          const toComponent = dag.components.find(c => c.id === edge.to)
          if (!fromComponent || !toComponent) return null

          const fromPos = getComponentPosition(edge.from, dag.components.indexOf(fromComponent))
          const toPos = getComponentPosition(edge.to, dag.components.indexOf(toComponent))

          return (
            <line
              key={index}
              x1={fromPos.x + 60}
              y1={fromPos.y + 30}
              x2={toPos.x}
              y2={toPos.y + 30}
              stroke="#6b7280"
              strokeWidth="2"
              markerEnd="url(#arrowhead)"
            />
          )
        })}

        {/* 箭头标记 */}
        <defs>
          <marker
            id="arrowhead"
            markerWidth="10"
            markerHeight="7"
            refX="9"
            refY="3.5"
            orient="auto"
          >
            <polygon points="0 0, 10 3.5, 0 7" fill="#6b7280" />
          </marker>
        </defs>
      </svg>

      {/* 绘制组件节点 */}
      {dag.components.map((component, index) => {
        const position = getComponentPosition(component.id, index)
        return (
          <div
            key={component.id}
            className={`absolute w-32 h-16 border-2 rounded-lg bg-white shadow-sm cursor-pointer transition-all ${
              selectedComponent === component.id ? "border-blue-500 shadow-md" : "border-gray-300"
            }`}
            style={{ left: position.x, top: position.y }}
            onClick={() => setSelectedComponent(component.id)}
          >
            <div className="p-2 h-full flex flex-col justify-between">
              <div className="flex items-center space-x-1">
                <ComponentTypeIcon type={component.type} />
                <span className="text-xs font-medium truncate">{component.name}</span>
              </div>
              <ComponentStatusIndicator status={component.status} />
            </div>
          </div>
        )
      })}
    </div>
  )
}

export default function ApplicationComponentsPage() {
  const params = useParams()
  const router = useRouter()
  const applicationId = params.id as string

  const [dag, setDAG] = useState<ApplicationDAG | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [selectedComponent, setSelectedComponent] = useState<Component | null>(null)

  // Mock数据 - 实际应用中应该从API获取
  useEffect(() => {
    const mockDAG: ApplicationDAG = {
      applicationID: applicationId,
      components: [
        {
          id: "comp-1",
          name: "前端界面",
          type: "frontend",
          status: "running",
          image: "nginx:alpine",
          ports: [80, 443],
          dependencies: ["comp-2"],
          resources: { cpu: 0.5, memory: 512 },
          providerID: "local-docker",
          containerRef: { id: "container-1", name: "frontend-container" },
          createdAt: "2024-01-15T10:00:00Z",
          updatedAt: "2024-01-15T10:00:00Z",
        },
        {
          id: "comp-2",
          name: "API网关",
          type: "gateway",
          status: "running",
          image: "traefik:v2.9",
          ports: [8080, 8443],
          dependencies: ["comp-3", "comp-4"],
          resources: { cpu: 0.3, memory: 256 },
          providerID: "local-docker",
          containerRef: { id: "container-2", name: "gateway-container" },
          createdAt: "2024-01-15T10:01:00Z",
          updatedAt: "2024-01-15T10:01:00Z",
        },
        {
          id: "comp-3",
          name: "用户服务",
          type: "backend",
          status: "running",
          image: "node:18-alpine",
          ports: [3000],
          dependencies: ["comp-5"],
          resources: { cpu: 1.0, memory: 1024 },
          providerID: "k8s-cluster",
          containerRef: { id: "container-3", name: "user-service-container" },
          createdAt: "2024-01-15T10:02:00Z",
          updatedAt: "2024-01-15T10:02:00Z",
        },
        {
          id: "comp-4",
          name: "订单服务",
          type: "backend",
          status: "running",
          image: "python:3.9-slim",
          ports: [8000],
          dependencies: ["comp-5", "comp-6"],
          resources: { cpu: 1.5, memory: 2048 },
          providerID: "k8s-cluster",
          containerRef: { id: "container-4", name: "order-service-container" },
          createdAt: "2024-01-15T10:03:00Z",
          updatedAt: "2024-01-15T10:03:00Z",
        },
        {
          id: "comp-5",
          name: "主数据库",
          type: "database",
          status: "running",
          image: "postgres:14",
          ports: [5432],
          dependencies: [],
          resources: { cpu: 2.0, memory: 4096 },
          providerID: "k8s-cluster",
          containerRef: { id: "container-5", name: "postgres-container" },
          createdAt: "2024-01-15T10:04:00Z",
          updatedAt: "2024-01-15T10:04:00Z",
        },
        {
          id: "comp-6",
          name: "Redis缓存",
          type: "cache",
          status: "running",
          image: "redis:7-alpine",
          ports: [6379],
          dependencies: [],
          resources: { cpu: 0.5, memory: 512 },
          providerID: "local-docker",
          containerRef: { id: "container-6", name: "redis-container" },
          createdAt: "2024-01-15T10:05:00Z",
          updatedAt: "2024-01-15T10:05:00Z",
        },
      ],
      edges: [
        { from: "comp-1", to: "comp-2", type: "http" },
        { from: "comp-2", to: "comp-3", type: "http" },
        { from: "comp-2", to: "comp-4", type: "http" },
        { from: "comp-3", to: "comp-5", type: "database" },
        { from: "comp-4", to: "comp-5", type: "database" },
        { from: "comp-4", to: "comp-6", type: "queue" },
      ],
      createdAt: "2024-01-15T10:00:00Z",
      updatedAt: "2024-01-15T10:05:00Z",
    }

    setTimeout(() => {
      setDAG(mockDAG)
      setIsLoading(false)
    }, 1000)
  }, [applicationId])

  const handleComponentAction = (componentId: string, action: "start" | "stop" | "restart") => {
    if (!dag) return

    setDAG(prev => {
      if (!prev) return prev
      return {
        ...prev,
        components: prev.components.map(comp => {
          if (comp.id === componentId) {
            let newStatus: Component["status"]
            switch (action) {
              case "start":
                newStatus = "deploying"
                setTimeout(() => {
                  setDAG(current => {
                    if (!current) return current
                    return {
                      ...current,
                      components: current.components.map(c => 
                        c.id === componentId ? { ...c, status: "running" } : c
                      )
                    }
                  })
                }, 2000)
                break
              case "stop":
                newStatus = "stopped"
                break
              case "restart":
                newStatus = "deploying"
                setTimeout(() => {
                  setDAG(current => {
                    if (!current) return current
                    return {
                      ...current,
                      components: current.components.map(c => 
                        c.id === componentId ? { ...c, status: "running" } : c
                      )
                    }
                  })
                }, 3000)
                break
              default:
                newStatus = comp.status
            }
            return { ...comp, status: newStatus, updatedAt: new Date().toISOString() }
          }
          return comp
        })
      }
    })
  }

  if (isLoading) {
    return (
      <div className="flex h-screen bg-gray-100">
        <Sidebar />
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center">
            <RefreshCw className="w-8 h-8 animate-spin mx-auto mb-4" />
            <p>加载组件信息中...</p>
          </div>
        </div>
      </div>
    )
  }

  if (!dag) {
    return (
      <div className="flex h-screen bg-gray-100">
        <Sidebar />
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center">
            <Box className="w-8 h-8 mx-auto mb-4 text-gray-400" />
            <p>未找到应用组件信息</p>
            <Button onClick={() => router.back()} className="mt-4">
              返回
            </Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-screen bg-gray-100">
      <Sidebar />
      <div className="flex-1 overflow-auto">
        <div className="p-6">
          {/* 页面头部 */}
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center space-x-4">
              <Button variant="ghost" size="sm" onClick={() => router.back()}>
                <ArrowLeft className="w-4 h-4 mr-2" />
                返回
              </Button>
              <div>
                <h1 className="text-2xl font-bold">应用组件</h1>
                <p className="text-gray-600">应用ID: {applicationId}</p>
              </div>
            </div>
            <div className="flex items-center space-x-2">
              <Badge variant="outline">
                {dag.components.length} 个组件
              </Badge>
              <Badge variant="outline">
                {dag.components.filter(c => c.status === "running").length} 个运行中
              </Badge>
            </div>
          </div>

          {/* 主要内容 */}
          <Tabs defaultValue="dag" className="space-y-6">
            <TabsList>
              <TabsTrigger value="dag">DAG图</TabsTrigger>
              <TabsTrigger value="components">组件列表</TabsTrigger>
              <TabsTrigger value="resources">资源使用</TabsTrigger>
            </TabsList>

            {/* DAG图视图 */}
            <TabsContent value="dag" className="space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center space-x-2">
                    <GitBranch className="w-5 h-5" />
                    <span>组件依赖图</span>
                  </CardTitle>
                  <CardDescription>
                    显示应用组件之间的依赖关系和数据流向
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <DAGVisualization dag={dag} />
                </CardContent>
              </Card>
            </TabsContent>

            {/* 组件列表视图 */}
            <TabsContent value="components" className="space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center space-x-2">
                    <Box className="w-5 h-5" />
                    <span>组件列表</span>
                  </CardTitle>
                  <CardDescription>
                    管理应用的所有组件
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>组件名称</TableHead>
                        <TableHead>类型</TableHead>
                        <TableHead>状态</TableHead>
                        <TableHead>镜像</TableHead>
                        <TableHead>端口</TableHead>
                        <TableHead>资源</TableHead>
                        <TableHead>提供者</TableHead>
                        <TableHead>操作</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {dag.components.map((component) => (
                        <TableRow key={component.id}>
                          <TableCell className="font-medium">
                            <div className="flex items-center space-x-2">
                              <ComponentTypeIcon type={component.type} />
                              <span>{component.name}</span>
                            </div>
                          </TableCell>
                          <TableCell>
                            <Badge variant="secondary">{component.type}</Badge>
                          </TableCell>
                          <TableCell>
                            <ComponentStatusIndicator status={component.status} />
                          </TableCell>
                          <TableCell className="font-mono text-sm">
                            {component.image}
                          </TableCell>
                          <TableCell>
                            {component.ports.join(", ")}
                          </TableCell>
                          <TableCell>
                            <div className="text-sm">
                              <div>CPU: {component.resources.cpu} 核</div>
                              <div>内存: {component.resources.memory} MB</div>
                              {component.resources.gpu && (
                                <div>GPU: {component.resources.gpu} 卡</div>
                              )}
                            </div>
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline">{component.providerID}</Badge>
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center space-x-1">
                              {component.status === "stopped" && (
                                <Button
                                  size="sm"
                                  variant="outline"
                                  onClick={() => handleComponentAction(component.id, "start")}
                                >
                                  <Play className="w-3 h-3" />
                                </Button>
                              )}
                              {component.status === "running" && (
                                <Button
                                  size="sm"
                                  variant="outline"
                                  onClick={() => handleComponentAction(component.id, "stop")}
                                >
                                  <Square className="w-3 h-3" />
                                </Button>
                              )}
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => handleComponentAction(component.id, "restart")}
                              >
                                <RefreshCw className="w-3 h-3" />
                              </Button>
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            </TabsContent>

            {/* 资源使用视图 */}
            <TabsContent value="resources" className="space-y-6">
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                <Card>
                  <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                    <CardTitle className="text-sm font-medium">总CPU使用</CardTitle>
                    <Cpu className="h-4 w-4 text-muted-foreground" />
                  </CardHeader>
                  <CardContent>
                    <div className="text-2xl font-bold">
                      {dag.components.reduce((sum, comp) => sum + comp.resources.cpu, 0)} 核
                    </div>
                    <p className="text-xs text-muted-foreground">
                      跨 {dag.components.length} 个组件
                    </p>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                    <CardTitle className="text-sm font-medium">总内存使用</CardTitle>
                    <MemoryStick className="h-4 w-4 text-muted-foreground" />
                  </CardHeader>
                  <CardContent>
                    <div className="text-2xl font-bold">
                      {(dag.components.reduce((sum, comp) => sum + comp.resources.memory, 0) / 1024).toFixed(1)} GB
                    </div>
                    <p className="text-xs text-muted-foreground">
                      跨 {dag.components.length} 个组件
                    </p>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                    <CardTitle className="text-sm font-medium">运行中组件</CardTitle>
                    <Activity className="h-4 w-4 text-muted-foreground" />
                  </CardHeader>
                  <CardContent>
                    <div className="text-2xl font-bold">
                      {dag.components.filter(c => c.status === "running").length}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      总共 {dag.components.length} 个组件
                    </p>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                    <CardTitle className="text-sm font-medium">提供者数量</CardTitle>
                    <Network className="h-4 w-4 text-muted-foreground" />
                  </CardHeader>
                  <CardContent>
                    <div className="text-2xl font-bold">
                      {new Set(dag.components.map(c => c.providerID)).size}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      分布式部署
                    </p>
                  </CardContent>
                </Card>
              </div>

              {/* 按提供者分组的组件 */}
              <Card>
                <CardHeader>
                  <CardTitle>按提供者分组</CardTitle>
                  <CardDescription>
                    查看组件在不同资源提供者上的分布情况
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="space-y-4">
                    {Array.from(new Set(dag.components.map(c => c.providerID))).map(providerId => {
                      const providerComponents = dag.components.filter(c => c.providerID === providerId)
                      const runningCount = providerComponents.filter(c => c.status === "running").length
                      
                      return (
                        <div key={providerId} className="border rounded-lg p-4">
                          <div className="flex items-center justify-between mb-3">
                            <div className="flex items-center space-x-2">
                              <Badge variant="outline">{providerId}</Badge>
                              <span className="text-sm text-gray-600">
                                {providerComponents.length} 个组件
                              </span>
                            </div>
                            <div className="text-sm text-gray-600">
                              {runningCount}/{providerComponents.length} 运行中
                            </div>
                          </div>
                          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2">
                            {providerComponents.map(component => (
                              <div key={component.id} className="flex items-center space-x-2 p-2 bg-gray-50 rounded">
                                <ComponentTypeIcon type={component.type} />
                                <span className="text-sm font-medium">{component.name}</span>
                                <ComponentStatusIndicator status={component.status} />
                              </div>
                            ))}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  )
}