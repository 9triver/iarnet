"use client"

import { useState, useEffect } from "react"
import { Sidebar } from "@/components/sidebar"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import {
  Activity,
  Cpu,
  AlertTriangle,
  CheckCircle,
  XCircle,
  RefreshCw,
  Play,
  Square,
  RotateCcw,
  Eye,
  TrendingUp,
  TrendingDown,
} from "lucide-react"

interface ApplicationStatus {
  id: string
  name: string
  status: "running" | "stopped" | "error" | "warning"
  uptime: string
  cpu: number
  memory: number
  network: number
  storage: number
  instances: number
  healthCheck: "healthy" | "unhealthy" | "unknown"
  lastRestart: string
  runningOn: string[]
  logs: LogEntry[]
  metrics: MetricData[]
}

interface LogEntry {
  timestamp: string
  level: "info" | "warn" | "error"
  message: string
}

interface MetricData {
  timestamp: string
  cpu: number
  memory: number
  network: number
  requests: number
}

const chartConfig = {
  cpu: {
    label: "CPU使用率",
    color: "hsl(var(--chart-1))",
  },
  memory: {
    label: "内存使用率",
    color: "hsl(var(--chart-2))",
  },
  network: {
    label: "网络流量",
    color: "hsl(var(--chart-3))",
  },
  requests: {
    label: "请求数",
    color: "hsl(var(--chart-4))",
  },
}

export default function StatusPage() {
  const [applications, setApplications] = useState<ApplicationStatus[]>([
    {
      id: "1",
      name: "用户管理系统",
      status: "running",
      uptime: "7天 12小时 30分钟",
      cpu: 45,
      memory: 68,
      network: 23,
      storage: 34,
      instances: 3,
      healthCheck: "healthy",
      lastRestart: "2024-01-08 09:15:00",
      runningOn: ["生产环境集群"],
      logs: [
        { timestamp: "2024-01-15 14:30:00", level: "info", message: "用户登录成功: user@example.com" },
        { timestamp: "2024-01-15 14:29:45", level: "warn", message: "数据库连接池使用率达到80%" },
        { timestamp: "2024-01-15 14:29:30", level: "info", message: "处理用户请求: GET /api/users" },
      ],
      metrics: [
        { timestamp: "14:25", cpu: 42, memory: 65, network: 20, requests: 150 },
        { timestamp: "14:26", cpu: 45, memory: 67, network: 22, requests: 165 },
        { timestamp: "14:27", cpu: 48, memory: 69, network: 25, requests: 180 },
        { timestamp: "14:28", cpu: 44, memory: 66, network: 21, requests: 155 },
        { timestamp: "14:29", cpu: 46, memory: 68, network: 24, requests: 170 },
        { timestamp: "14:30", cpu: 45, memory: 68, network: 23, requests: 160 },
      ],
    },
    {
      id: "2",
      name: "数据处理服务",
      status: "warning",
      uptime: "2天 5小时 15分钟",
      cpu: 78,
      memory: 85,
      network: 45,
      storage: 67,
      instances: 2,
      healthCheck: "unhealthy",
      lastRestart: "2024-01-13 08:45:00",
      runningOn: ["开发环境"],
      logs: [
        { timestamp: "2024-01-15 14:30:00", level: "error", message: "数据处理任务失败: timeout" },
        { timestamp: "2024-01-15 14:29:30", level: "warn", message: "内存使用率过高: 85%" },
        { timestamp: "2024-01-15 14:29:00", level: "info", message: "开始处理数据批次: batch_001" },
      ],
      metrics: [
        { timestamp: "14:25", cpu: 75, memory: 82, network: 42, requests: 80 },
        { timestamp: "14:26", cpu: 78, memory: 84, network: 44, requests: 85 },
        { timestamp: "14:27", cpu: 80, memory: 86, network: 46, requests: 90 },
        { timestamp: "14:28", cpu: 76, memory: 83, network: 43, requests: 82 },
        { timestamp: "14:29", cpu: 79, memory: 85, network: 45, requests: 88 },
        { timestamp: "14:30", cpu: 78, memory: 85, network: 45, requests: 85 },
      ],
    },
    {
      id: "3",
      name: "API网关",
      status: "running",
      uptime: "15天 3小时 45分钟",
      cpu: 32,
      memory: 45,
      network: 67,
      storage: 23,
      instances: 5,
      healthCheck: "healthy",
      lastRestart: "2023-12-31 10:30:00",
      runningOn: ["生产环境集群", "开发环境"],
      logs: [
        { timestamp: "2024-01-15 14:30:00", level: "info", message: "API请求处理: POST /api/auth/login" },
        { timestamp: "2024-01-15 14:29:45", level: "info", message: "负载均衡器状态正常" },
        { timestamp: "2024-01-15 14:29:30", level: "info", message: "处理API请求: GET /api/status" },
      ],
      metrics: [
        { timestamp: "14:25", cpu: 30, memory: 42, network: 65, requests: 320 },
        { timestamp: "14:26", cpu: 32, memory: 44, network: 66, requests: 340 },
        { timestamp: "14:27", cpu: 35, memory: 46, network: 68, requests: 360 },
        { timestamp: "14:28", cpu: 31, memory: 43, network: 64, requests: 330 },
        { timestamp: "14:29", cpu: 33, memory: 45, network: 67, requests: 350 },
        { timestamp: "14:30", cpu: 32, memory: 45, network: 67, requests: 345 },
      ],
    },
  ])

  const [selectedApp, setSelectedApp] = useState<ApplicationStatus | null>(null)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [showTopology, setShowTopology] = useState(false)
  const [selectedAppForTopology, setSelectedAppForTopology] = useState<ApplicationStatus | null>(null)

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
        })),
      )
    }, 5000)

    return () => clearInterval(interval)
  }, [autoRefresh])

  const getStatusIcon = (status: ApplicationStatus["status"]) => {
    switch (status) {
      case "running":
        return <CheckCircle className="h-4 w-4 text-green-500" />
      case "warning":
        return <AlertTriangle className="h-4 w-4 text-yellow-500" />
      case "error":
        return <XCircle className="h-4 w-4 text-red-500" />
      case "stopped":
        return <Square className="h-4 w-4 text-gray-500" />
    }
  }

  const getStatusBadge = (status: ApplicationStatus["status"]) => {
    switch (status) {
      case "running":
        return (
          <Badge variant="default" className="bg-green-500">
            运行中
          </Badge>
        )
      case "warning":
        return (
          <Badge variant="default" className="bg-yellow-500">
            警告
          </Badge>
        )
      case "error":
        return <Badge variant="destructive">错误</Badge>
      case "stopped":
        return <Badge variant="secondary">已停止</Badge>
    }
  }

  const getHealthIcon = (health: ApplicationStatus["healthCheck"]) => {
    switch (health) {
      case "healthy":
        return <CheckCircle className="h-4 w-4 text-green-500" />
      case "unhealthy":
        return <XCircle className="h-4 w-4 text-red-500" />
      case "unknown":
        return <AlertTriangle className="h-4 w-4 text-gray-500" />
    }
  }

  const getLogLevelBadge = (level: LogEntry["level"]) => {
    switch (level) {
      case "info":
        return <Badge variant="outline">INFO</Badge>
      case "warn":
        return (
          <Badge variant="default" className="bg-yellow-500">
            WARN
          </Badge>
        )
      case "error":
        return <Badge variant="destructive">ERROR</Badge>
    }
  }

  const handleRestart = (id: string) => {
    setApplications((prev) =>
      prev.map((app) =>
        app.id === id
          ? {
              ...app,
              lastRestart: new Date().toLocaleString(),
              uptime: "0分钟",
            }
          : app,
      ),
    )
  }

  const handleStop = (id: string) => {
    setApplications((prev) => prev.map((app) => (app.id === id ? { ...app, status: "stopped" } : app)))
  }

  const handleStart = (id: string) => {
    setApplications((prev) => prev.map((app) => (app.id === id ? { ...app, status: "running" } : app)))
  }

  const generateTopologyNodes = () => {
    const nodeCount = Math.floor(Math.random() * 6) + 3 // 3-8 nodes
    const components = ["Web服务Actor", "API网关Actor", "计算处理Actor", "缓存代理Actor", "消息队列Actor", "工作处理Actor", "负载均衡Actor", "监控Actor"]
    const nodes = []

    for (let i = 0; i < nodeCount; i++) {
      const angle = (i / nodeCount) * 2 * Math.PI
      const radius = 120
      const x = 200 + radius * Math.cos(angle)
      const y = 150 + radius * Math.sin(angle)

      nodes.push({
        id: i,
        x,
        y,
        component: components[i % components.length],
        cpu: Math.floor(Math.random() * 100),
        memory: Math.floor(Math.random() * 100),
        storage: Math.floor(Math.random() * 100),
      })
    }
    return nodes
  }

  const [topologyNodes, setTopologyNodes] = useState(generateTopologyNodes())

  const handleShowTopology = (app: ApplicationStatus) => {
    setSelectedAppForTopology(app)
    setTopologyNodes(generateTopologyNodes())
    setShowTopology(true)
  }

  const runningApps = applications.filter((app) => app.status === "running").length
  const warningApps = applications.filter((app) => app.status === "warning").length
  const errorApps = applications.filter((app) => app.status === "error").length
  const totalInstances = applications.reduce((sum, app) => sum + app.instances, 0)

  return (
    <div className="flex h-screen bg-background">
      <Sidebar />

      <main className="flex-1 overflow-auto">
        <div className="p-8">
          {/* Header */}
          <div className="flex items-center justify-between mb-8">
            <div>
              <h1 className="text-3xl font-playfair font-bold text-foreground mb-2">运行状态监控</h1>
              <p className="text-muted-foreground">实时监控应用运行状态和资源使用情况</p>
            </div>

            <div className="flex items-center space-x-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setAutoRefresh(!autoRefresh)}
                className={autoRefresh ? "bg-green-50 border-green-200" : ""}
              >
                <RefreshCw className={`h-4 w-4 ${autoRefresh ? "animate-spin" : ""}`} />
                {autoRefresh ? "自动刷新" : "手动刷新"}
              </Button>
            </div>
          </div>

          {/* Stats Cards */}
          <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">运行中应用</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-green-600">{runningApps}</div>
                <p className="text-xs text-muted-foreground">正常运行</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">警告状态</CardTitle>
                <AlertTriangle className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-yellow-600">{warningApps}</div>
                <p className="text-xs text-muted-foreground">需要关注</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">错误状态</CardTitle>
                <XCircle className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-red-600">{errorApps}</div>
                <p className="text-xs text-muted-foreground">需要处理</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">总实例数</CardTitle>
                <Cpu className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{totalInstances}</div>
                <p className="text-xs text-muted-foreground">运行实例</p>
              </CardContent>
            </Card>
          </div>

          {/* Applications Status Table */}
          <Card>
            <CardHeader>
              <CardTitle>应用状态列表</CardTitle>
              <CardDescription>所有应用的实时运行状态</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>应用名称</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>健康检查</TableHead>
                    <TableHead>CPU</TableHead>
                    <TableHead>内存</TableHead>
                    <TableHead>实例数</TableHead>
                    <TableHead>运行时间</TableHead>
                    <TableHead>操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {applications.map((app) => (
                    <TableRow key={app.id} className="cursor-pointer hover:bg-muted/50">
                      <TableCell>
                        <div className="flex items-center space-x-2">
                          {getStatusIcon(app.status)}
                          <div>
                            <div className="font-medium">{app.name}</div>
                            <div className="text-xs text-muted-foreground">运行在: {app.runningOn.join(", ")}</div>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>{getStatusBadge(app.status)}</TableCell>
                      <TableCell>
                        <div className="flex items-center space-x-2">
                          {getHealthIcon(app.healthCheck)}
                          <span className="text-sm capitalize">{app.healthCheck}</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center space-x-2">
                          <div className="text-sm">{app.cpu}%</div>
                          {app.cpu > 80 ? (
                            <TrendingUp className="h-3 w-3 text-red-500" />
                          ) : (
                            <TrendingDown className="h-3 w-3 text-green-500" />
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center space-x-2">
                          <div className="text-sm">{app.memory}%</div>
                          {app.memory > 80 ? (
                            <TrendingUp className="h-3 w-3 text-red-500" />
                          ) : (
                            <TrendingDown className="h-3 w-3 text-green-500" />
                          )}
                        </div>
                      </TableCell>
                      <TableCell>{app.instances}</TableCell>
                      <TableCell className="text-xs">{app.uptime}</TableCell>
                      <TableCell>
                        <div className="flex items-center space-x-1">
                          <Button variant="ghost" size="sm" onClick={() => handleShowTopology(app)} title="查看详情">
                            <Eye className="h-4 w-4" />
                          </Button>
                          {app.status === "running" ? (
                            <Button variant="ghost" size="sm" onClick={() => handleStop(app.id)} title="停止应用">
                              <Square className="h-4 w-4" />
                            </Button>
                          ) : (
                            <Button variant="ghost" size="sm" onClick={() => handleStart(app.id)} title="启动应用">
                              <Play className="h-4 w-4" />
                            </Button>
                          )}
                          <Button variant="ghost" size="sm" onClick={() => handleRestart(app.id)} title="重启应用">
                            <RotateCcw className="h-4 w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      </main>

      {/* Topology Visualization Dialog */}
      <Dialog open={showTopology} onOpenChange={setShowTopology}>
        <DialogContent className="max-w-4xl max-h-[80vh]">
          <DialogHeader>
            <DialogTitle>{selectedAppForTopology?.name} - 应用拓扑图</DialogTitle>
          </DialogHeader>
          <div className="relative w-full h-[500px] bg-slate-50 rounded-lg overflow-hidden">
            <svg width="100%" height="100%" className="absolute inset-0">
              {topologyNodes.map((node, i) =>
                topologyNodes.slice(i + 1).map((targetNode, j) => (
                  <g key={`${i}-${j}`}>
                    <line
                      x1={node.x}
                      y1={node.y}
                      x2={targetNode.x}
                      y2={targetNode.y}
                      stroke="#e2e8f0"
                      strokeWidth="2"
                      className="opacity-60"
                    />
                    <circle r="3" fill="#3b82f6" className="opacity-80">
                      <animateMotion
                        dur="3s"
                        repeatCount="indefinite"
                        path={`M${node.x},${node.y} L${targetNode.x},${targetNode.y}`}
                      />
                    </circle>
                  </g>
                )),
              )}

              {topologyNodes.map((node, i) => (
                <g key={i}>
                  {/* Resource box (bottom rectangle) */}
                  <rect
                    x={node.x - 50}
                    y={node.y + 10}
                    width="100"
                    height="40"
                    fill="#f8fafc"
                    stroke="#cbd5e1"
                    strokeWidth="1"
                  />
                  <text x={node.x} y={node.y + 25} textAnchor="middle" className="text-xs fill-slate-600">
                    CPU: {node.cpu}%
                  </text>
                  <text x={node.x} y={node.y + 38} textAnchor="middle" className="text-xs fill-slate-600">
                    MEM: {node.memory}%
                  </text>

                  {/* Component box (top rounded rectangle) */}
                  <rect
                    x={node.x - 50}
                    y={node.y - 30}
                    width="100"
                    height="35"
                    rx="8"
                    ry="8"
                    fill="#3b82f6"
                    className="opacity-90"
                  />
                  <text x={node.x} y={node.y - 8} textAnchor="middle" className="text-sm fill-white font-medium">
                    {node.component}
                  </text>
                </g>
              ))}
            </svg>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
