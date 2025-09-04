"use client"

import { useState, useEffect } from "react"
import { useParams, useRouter } from "next/navigation"
import { applicationsAPI } from "@/lib/api"
import { Sidebar } from "@/components/sidebar"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  ArrowLeft,
  Package,
  GitBranch,
  Clock,
  Activity,
  Play,
  Square,
  RefreshCw,
  Terminal,
  Download,
} from "lucide-react"

interface LogEntry {
  timestamp: string
  level: string
  message: string
}

export default function ApplicationDetailPage() {
  const params = useParams()
  const router = useRouter()
  const [application, setApplication] = useState<Application | null>(null)
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isLoadingLogs, setIsLoadingLogs] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const applicationId = params.id as string

  useEffect(() => {
    loadApplicationDetail()
  }, [applicationId])

  const loadApplicationDetail = async () => {
    try {
      setIsLoading(true)
      setError(null)
      
      // 获取应用详情
      const response = await applicationsAPI.getAll()
      const app = response.applications.find(app => app.id === applicationId)
      
      if (!app) {
        setError("应用不存在")
        return
      }
      
      setApplication(app)
    } catch (err) {
      console.error('Failed to load application detail:', err)
      setError("加载应用详情失败")
    } finally {
      setIsLoading(false)
    }
  }

  const loadLogs = async () => {
    if (!application) return
    
    try {
      setIsLoadingLogs(true)
      
      // 模拟日志数据，实际应该调用真实的日志API
      const mockLogs: LogEntry[] = [
        {
          timestamp: "2025-01-09 10:30:15",
          level: "INFO",
          message: "Application started successfully"
        },
        {
          timestamp: "2025-01-09 10:30:20",
          level: "INFO",
          message: "Server listening on port 3000"
        },
        {
          timestamp: "2025-01-09 10:31:05",
          level: "DEBUG",
          message: "Database connection established"
        },
        {
          timestamp: "2025-01-09 10:32:10",
          level: "WARN",
          message: "High memory usage detected: 85%"
        },
        {
          timestamp: "2025-01-09 10:33:15",
          level: "ERROR",
          message: "Failed to connect to external service: timeout"
        },
        {
          timestamp: "2025-01-09 10:34:20",
          level: "INFO",
          message: "Health check passed"
        }
      ]
      
      setLogs(mockLogs)
    } catch (err) {
      console.error('Failed to load logs:', err)
    } finally {
      setIsLoadingLogs(false)
    }
  }

  const handleStart = async () => {
    if (!application) return
    
    try {
      await applicationsAPI.deploy(application.id)
      await loadApplicationDetail()
    } catch (err) {
      console.error('Failed to start application:', err)
    }
  }

  const handleStop = async () => {
    if (!application) return
    
    try {
      await applicationsAPI.stop(application.id)
      await loadApplicationDetail()
    } catch (err) {
      console.error('Failed to stop application:', err)
    }
  }

  const getStatusBadge = (status: string) => {
    const statusConfig = {
      running: { variant: "default" as const, label: "运行中", color: "bg-green-500" },
      stopped: { variant: "secondary" as const, label: "已停止", color: "bg-gray-500" },
      error: { variant: "destructive" as const, label: "错误", color: "bg-red-500" },
      deploying: { variant: "outline" as const, label: "部署中", color: "bg-blue-500" },
      idle: { variant: "outline" as const, label: "未部署", color: "bg-orange-500" },
    }
    
    const config = statusConfig[status as keyof typeof statusConfig] || statusConfig.idle
    return (
      <Badge variant={config.variant} className="flex items-center space-x-1">
        <div className={`w-2 h-2 rounded-full ${config.color}`} />
        <span>{config.label}</span>
      </Badge>
    )
  }

  const getTypeIcon = (type: string) => {
    switch (type) {
      case "web":
        return <Activity className="h-5 w-5 text-blue-500" />
      case "api":
        return <Package className="h-5 w-5 text-green-500" />
      case "worker":
        return <RefreshCw className="h-5 w-5 text-purple-500" />
      case "database":
        return <Package className="h-5 w-5 text-orange-500" />
      default:
        return <Package className="h-5 w-5 text-gray-500" />
    }
  }

  const getTypeLabel = (type: string) => {
    const typeLabels = {
      web: "Web应用",
      api: "API服务",
      worker: "后台任务",
      database: "数据库"
    }
    return typeLabels[type as keyof typeof typeLabels] || type
  }

  const getLevelColor = (level: string) => {
    switch (level.toUpperCase()) {
      case "ERROR":
        return "text-red-500"
      case "WARN":
        return "text-yellow-500"
      case "INFO":
        return "text-blue-500"
      case "DEBUG":
        return "text-gray-500"
      default:
        return "text-gray-700"
    }
  }

  if (isLoading) {
    return (
      <div className="flex h-screen bg-gray-50">
        <Sidebar />
        <main className="flex-1 p-8">
          <div className="flex items-center justify-center h-full">
            <div className="text-center">
              <RefreshCw className="h-8 w-8 animate-spin mx-auto mb-4" />
              <p>加载中...</p>
            </div>
          </div>
        </main>
      </div>
    )
  }

  if (error || !application) {
    return (
      <div className="flex h-screen bg-gray-50">
        <Sidebar />
        <main className="flex-1 p-8">
          <div className="flex items-center justify-center h-full">
            <div className="text-center">
              <p className="text-red-500 mb-4">{error || "应用不存在"}</p>
              <Button onClick={() => router.back()}>
                <ArrowLeft className="h-4 w-4 mr-2" />
                返回
              </Button>
            </div>
          </div>
        </main>
      </div>
    )
  }

  return (
    <div className="flex h-screen bg-gray-50">
      <Sidebar />
      <main className="flex-1 overflow-auto">
        <div className="p-8 space-y-6">
          {/* Header */}
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-4">
              <Button variant="ghost" onClick={() => router.back()}>
                <ArrowLeft className="h-4 w-4 mr-2" />
                返回
              </Button>
              <div className="flex items-center space-x-3">
                {getTypeIcon(application.type)}
                <div>
                  <h1 className="text-2xl font-bold">{application.name}</h1>
                  <div className="flex items-center space-x-2 mt-1">
                    <Badge variant="outline">{getTypeLabel(application.type)}</Badge>
                    {getStatusBadge(application.status)}
                  </div>
                </div>
              </div>
            </div>
            
            <div className="flex items-center space-x-2">
              {application.status === "running" ? (
                <Button variant="outline" onClick={handleStop}>
                  <Square className="h-4 w-4 mr-2" />
                  停止
                </Button>
              ) : (
                <Button onClick={handleStart} disabled={application.status === "deploying"}>
                  <Play className="h-4 w-4 mr-2" />
                  {application.status === "deploying" ? "部署中..." : "启动"}
                </Button>
              )}
              <Button variant="outline" onClick={loadApplicationDetail}>
                <RefreshCw className="h-4 w-4 mr-2" />
                刷新
              </Button>
            </div>
          </div>

          {/* Application Info */}
          <Card>
            <CardHeader>
              <CardTitle>应用信息</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {application.description && (
                <div>
                  <h4 className="text-sm font-medium text-muted-foreground mb-1">描述</h4>
                  <p className="text-sm">{application.description}</p>
                </div>
              )}
              
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {application.importType === "git" ? (
                  <div>
                    <h4 className="text-sm font-medium text-muted-foreground mb-1">Git仓库</h4>
                    <div className="flex items-center space-x-2 text-sm">
                      <GitBranch className="h-4 w-4" />
                      <span className="font-mono">{application.branch}</span>
                    </div>
                  </div>
                ) : (
                  <div>
                    <h4 className="text-sm font-medium text-muted-foreground mb-1">Docker镜像</h4>
                    <div className="flex items-center space-x-2 text-sm">
                      <Package className="h-4 w-4" />
                      <span className="font-mono">{application.dockerImage}:{application.dockerTag}</span>
                    </div>
                  </div>
                )}
                
                {application.ports && application.ports.length > 0 && (
                  <div>
                    <h4 className="text-sm font-medium text-muted-foreground mb-1">端口</h4>
                    <div className="flex flex-wrap gap-1">
                      {application.ports.map((port, index) => (
                        <Badge key={index} variant="outline" className="text-xs font-mono">
                          {port}
                        </Badge>
                      ))}
                    </div>
                  </div>
                )}
                
                {application.lastDeployed && (
                  <div>
                    <h4 className="text-sm font-medium text-muted-foreground mb-1">最后部署</h4>
                    <div className="flex items-center space-x-2 text-sm">
                      <Clock className="h-4 w-4" />
                      <span>{application.lastDeployed}</span>
                    </div>
                  </div>
                )}
                
                {application.runningOn && application.runningOn.length > 0 && (
                  <div>
                    <h4 className="text-sm font-medium text-muted-foreground mb-1">运行节点</h4>
                    <div className="flex flex-wrap gap-1">
                      {application.runningOn.map((resource, index) => (
                        <Badge key={index} variant="secondary" className="text-xs">
                          {resource}
                        </Badge>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>

          {/* Tabs */}
          <Tabs defaultValue="logs" className="space-y-4">
            <TabsList>
              <TabsTrigger value="logs" className="flex items-center space-x-2">
                <Terminal className="h-4 w-4" />
                <span>容器日志</span>
              </TabsTrigger>
              <TabsTrigger value="metrics" disabled>
                <Activity className="h-4 w-4 mr-2" />
                监控指标
              </TabsTrigger>
              <TabsTrigger value="events" disabled>
                <Clock className="h-4 w-4 mr-2" />
                事件历史
              </TabsTrigger>
            </TabsList>

            <TabsContent value="logs">
              <Card>
                <CardHeader>
                  <div className="flex items-center justify-between">
                    <CardTitle className="flex items-center space-x-2">
                      <Terminal className="h-5 w-5" />
                      <span>容器日志</span>
                    </CardTitle>
                    <div className="flex items-center space-x-2">
                      <Button variant="outline" size="sm" onClick={loadLogs} disabled={isLoadingLogs}>
                        <RefreshCw className={`h-4 w-4 mr-2 ${isLoadingLogs ? 'animate-spin' : ''}`} />
                        刷新
                      </Button>
                      <Button variant="outline" size="sm">
                        <Download className="h-4 w-4 mr-2" />
                        下载
                      </Button>
                    </div>
                  </div>
                  <CardDescription>
                    查看应用容器的实时日志输出
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <ScrollArea className="h-96 w-full border rounded-md p-4 bg-black text-green-400 font-mono text-sm">
                    {logs.length === 0 ? (
                      <div className="flex items-center justify-center h-full text-gray-500">
                        {isLoadingLogs ? (
                          <div className="flex items-center space-x-2">
                            <RefreshCw className="h-4 w-4 animate-spin" />
                            <span>加载日志中...</span>
                          </div>
                        ) : (
                          <div className="text-center">
                            <Terminal className="h-8 w-8 mx-auto mb-2 opacity-50" />
                            <p>暂无日志数据</p>
                            <Button variant="link" onClick={loadLogs} className="text-green-400 mt-2">
                              点击加载日志
                            </Button>
                          </div>
                        )}
                      </div>
                    ) : (
                      <div className="space-y-1">
                        {logs.map((log, index) => (
                          <div key={index} className="flex space-x-3">
                            <span className="text-gray-400 shrink-0">{log.timestamp}</span>
                            <span className={`shrink-0 font-bold ${getLevelColor(log.level)}`}>
                              [{log.level}]
                            </span>
                            <span className="break-all">{log.message}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </ScrollArea>
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="metrics">
              <Card>
                <CardHeader>
                  <CardTitle>监控指标</CardTitle>
                  <CardDescription>应用的性能监控数据（即将推出）</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="text-center py-8 text-muted-foreground">
                    <Activity className="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p>监控指标功能正在开发中</p>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="events">
              <Card>
                <CardHeader>
                  <CardTitle>事件历史</CardTitle>
                  <CardDescription>应用的部署和运行事件记录（即将推出）</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="text-center py-8 text-muted-foreground">
                    <Clock className="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p>事件历史功能正在开发中</p>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>
      </main>
    </div>
  )
}