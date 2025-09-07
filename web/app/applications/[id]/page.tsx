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
import { CodeEditor } from "@/components/code-editor"
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
  Code,
  Globe,
  Box,
  Database,
  Network,
  Cpu,
  MemoryStick,
  HardDrive,
} from "lucide-react"

interface CodeBrowserInfo {
  status: string
  port?: number
  start_time?: string
  work_dir?: string
}

// 组件类型定义 - 表示分布式部署的actor类型
interface Component {
  id: string
  name: string
  type: "web" | "api" | "worker" | "compute" | "gateway"
  status: "running" | "stopped" | "deploying" | "error" | "pending" | "unknown"
  image: string
  ports: number[]
  dependencies: string[]
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
  from_component: string
  to_component: string
  connection_type: "http" | "grpc" | "stream" | "queue" | "file"
}

interface ApplicationDAG {
  applicationID: string
  components: { [key: string]: Component }
  edges?: DAGEdge[]
  globalConfig?: { [key: string]: string }
  analysisMetadata?: { [key: string]: any }
  createdAt?: string
  updatedAt?: string
}

export default function ApplicationDetailPage() {
  const params = useParams()
  const router = useRouter()
  const [application, setApplication] = useState<Application | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [components, setComponents] = useState<Component[]>([])
  const [isLoadingComponents, setIsLoadingComponents] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [codeBrowserStatus, setCodeBrowserStatus] = useState<CodeBrowserInfo | null>(null)
  const [isStartingCodeBrowser, setIsStartingCodeBrowser] = useState(false)

  const applicationId = params.id as string

  useEffect(() => {
    loadApplicationDetail()
    loadComponents()
  }, [applicationId])

  // 组件类型图标
  const getComponentTypeIcon = (type: Component["type"]) => {
    const iconMap = {
      web: Globe,
      api: Box,
      worker: Cpu,
      compute: Activity,
      gateway: GitBranch,
    }
    const Icon = iconMap[type] || Package
    return <Icon className="w-4 h-4" />
  }

  // 组件类型标签
  const getComponentTypeLabel = (type: Component["type"]) => {
    const labelMap = {
      web: "Web服务Actor",
      api: "API服务Actor",
      worker: "工作处理Actor",
      compute: "计算处理Actor",
      gateway: "网关代理Actor",
    }
    return labelMap[type] || type
  }

  // 组件状态指示器
  const ComponentStatusIndicator = ({ status }: { status: Component["status"] }) => {
    const statusConfig = {
      running: { color: "bg-green-500", text: "运行中" },
      stopped: { color: "bg-gray-500", text: "已停止" },
      deploying: { color: "bg-blue-500", text: "部署中" },
      error: { color: "bg-red-500", text: "错误" },
      pending: { color: "bg-yellow-500", text: "待部署" },
      unknown: { color: "bg-gray-400", text: "未知" },
    }
    const config = statusConfig[status] || { color: "bg-gray-400", text: "未知" }
    return (
      <div className="flex items-center space-x-2">
        <div className={`w-2 h-2 rounded-full ${config.color}`} />
        <span className="text-sm">{config.text}</span>
      </div>
    )
  }

  // DAG图可视化组件
  const DAGVisualization = ({ dag }: { dag: ApplicationDAG }) => {
    const [selectedComponent, setSelectedComponent] = useState<string | null>(null)
    const componentsArray = Object.values(dag.components)

    // 简化的DAG布局算法
    const getComponentPosition = (componentId: string, index: number) => {
      const component = dag.components[componentId]
      if (!component) return { x: 0, y: 0 }

      // 根据Actor组件类型和依赖关系计算位置
      const typeOrder = { web: 0, gateway: 1, api: 2, worker: 3, compute: 4 }
      const layer = typeOrder[component.type] || 0
      const x = 50 + layer * 150
      const y = 50 + (index % 3) * 100

      return { x, y }
    }

    return (
      <div className="relative w-full h-96 border rounded-lg bg-gray-50 overflow-auto">
        <svg className="absolute inset-0 w-full h-full">
          {/* 绘制连接线 */}
          {dag.edges?.map((edge, index) => {
            const fromComponent = dag.components[edge.from_component]
            const toComponent = dag.components[edge.to_component]
            if (!fromComponent || !toComponent) return null

            const fromIndex = componentsArray.findIndex(c => c.id === edge.from_component)
            const toIndex = componentsArray.findIndex(c => c.id === edge.to_component)
            const fromPos = getComponentPosition(edge.from_component, fromIndex)
            const toPos = getComponentPosition(edge.to_component, toIndex)

            return (
              <line
                key={index}
                x1={fromPos.x + 60}
                y1={fromPos.y + 30}
                x2={toPos.x}
                y2={toPos.y + 30}
                stroke="#94a3b8"
                strokeWidth="1.5"
                markerEnd="url(#arrowhead)"
                strokeDasharray="none"
                opacity="0.8"
              />
            )
          })}

          {/* 箭头标记 */}
          <defs>
            <marker
              id="arrowhead"
              markerWidth="8"
              markerHeight="6"
              refX="7"
              refY="3"
              orient="auto"
              markerUnits="strokeWidth"
            >
              <polygon points="0 0, 8 3, 0 6" fill="#94a3b8" opacity="0.8" />
            </marker>
          </defs>
        </svg>

        {/* 绘制组件节点 */}
        {componentsArray.map((component, index) => {
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
                  {getComponentTypeIcon(component.type)}
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

  const loadCodeBrowserStatus = async () => {
    if (!applicationId) return
    
    try {
      const result = await applicationsAPI.getCodeBrowserStatus(applicationId)
      setCodeBrowserStatus(result.browser)
    } catch (err) {
      console.error('Failed to load code browser status:', err)
      setCodeBrowserStatus(null)
    }
  }

  const handleStartCodeBrowser = async () => {
    if (!application) return
    
    try {
      setIsStartingCodeBrowser(true)
      const result = await applicationsAPI.startCodeBrowser(application.id)
      await loadCodeBrowserStatus()
      
      // 打开新窗口访问代码浏览器
      if (result.url) {
        window.open(result.url, '_blank')
      }
    } catch (err) {
      console.error('Failed to start code browser:', err)
    } finally {
      setIsStartingCodeBrowser(false)
    }
  }

  const handleStopCodeBrowser = async () => {
    if (!application) return
    
    try {
      await applicationsAPI.stopCodeBrowser(application.id)
      await loadCodeBrowserStatus()
    } catch (err) {
      console.error('Failed to stop code browser:', err)
    }
  }

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

  const [applicationDAG, setApplicationDAG] = useState<ApplicationDAG | null>(null)

  const loadComponents = async () => {
    if (!applicationId) return
    
    setIsLoadingComponents(true)
    try {
      const dag = await applicationsAPI.getComponents(applicationId) as ApplicationDAG
      // 保存完整的DAG数据
      setApplicationDAG(dag)
      // 将DAG中的components对象转换为数组
      const componentsArray = dag.components ? Object.values(dag.components) as Component[] : []
      setComponents(componentsArray)
    } catch (error) {
      console.error('Failed to load components:', error)
      setComponents([])
      setApplicationDAG(null)
    } finally {
      setIsLoadingComponents(false)
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
              
              <div className="grid grid-cols-1 md:grid-cols-[2fr_1fr] lg:grid-cols-[2fr_1fr_1fr_1fr] gap-4">
                <div>
                  <h4 className="text-sm font-medium text-muted-foreground mb-1">Git仓库</h4>
                  <div className="space-y-2">
                    {application.gitUrl && (
                      <div className="flex items-center space-x-2 text-sm">
                        <Package className="h-4 w-4" />
                        <span className="font-mono text-xs break-all">{application.gitUrl}</span>
                      </div>
                    )}
                    {application.branch && (
                      <div className="flex items-center space-x-2 text-sm">
                        <GitBranch className="h-4 w-4" />
                        <span className="font-mono">{application.branch}</span>
                      </div>
                    )}
                  </div>
                </div>
                
                <div>
                  <h4 className="text-sm font-medium text-muted-foreground mb-1">组件数量</h4>
                  <div className="flex items-center space-x-2 text-sm">
                    <Package className="h-4 w-4" />
                    <span>{components.length} 个组件</span>
                    {isLoadingComponents && (
                      <RefreshCw className="h-3 w-3 animate-spin" />
                    )}
                  </div>
                </div>
                
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
          <Tabs defaultValue="components" className="space-y-4">
            <TabsList>
              <TabsTrigger value="components" className="flex items-center space-x-2">
                <Package className="h-4 w-4" />
                <span>组件管理</span>
              </TabsTrigger>
              <TabsTrigger value="code" className="flex items-center space-x-2">
                <Code className="h-4 w-4" />
                <span>代码浏览</span>
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

            <TabsContent value="components">
              <Card>
                <CardHeader>
                  <div className="flex items-center justify-between">
                    <CardTitle className="flex items-center space-x-2">
                      <Package className="h-5 w-5" />
                      <span>组件管理</span>
                    </CardTitle>
                    <div className="flex items-center space-x-2">
                      <Button variant="outline" size="sm" onClick={loadComponents} disabled={isLoadingComponents}>
                        <RefreshCw className={`h-4 w-4 mr-2 ${isLoadingComponents ? 'animate-spin' : ''}`} />
                        刷新
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => applicationsAPI.analyzeApplication(applicationId)}>
                        <Activity className="h-4 w-4 mr-2" />
                        分析
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => applicationsAPI.deployComponents(applicationId)}>
                        <Play className="h-4 w-4 mr-2" />
                        部署
                      </Button>
                    </div>
                  </div>
                  <CardDescription>
                    管理应用的Actor组件，包括Web服务、API服务、工作处理等可分布式部署的执行单元
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {components.length === 0 ? (
                    <div className="flex items-center justify-center h-64 text-gray-500">
                      {isLoadingComponents ? (
                        <div className="flex items-center space-x-2">
                          <RefreshCw className="h-4 w-4 animate-spin" />
                          <span>加载组件中...</span>
                        </div>
                      ) : (
                        <div className="text-center">
                          <Package className="h-8 w-8 mx-auto mb-2 opacity-50" />
                          <p>暂无组件数据</p>
                          <Button variant="link" onClick={loadComponents} className="mt-2">
                            点击加载组件
                          </Button>
                        </div>
                      )}
                    </div>
                  ) : (
                    <Tabs defaultValue="dag" className="space-y-4">
                      <TabsList>
                        <TabsTrigger value="dag" className="flex items-center space-x-2">
                          <GitBranch className="h-4 w-4" />
                          <span>DAG图</span>
                        </TabsTrigger>
                        <TabsTrigger value="list" className="flex items-center space-x-2">
                          <Box className="h-4 w-4" />
                          <span>组件列表</span>
                        </TabsTrigger>
                      </TabsList>

                      <TabsContent value="dag" className="space-y-4">
                        <Card>
                          <CardHeader>
                            <CardTitle className="flex items-center space-x-2">
                              <GitBranch className="w-5 h-5" />
                              <span>组件依赖图</span>
                            </CardTitle>
                            <CardDescription>
                              显示应用Actor组件之间的依赖关系和数据流向
                            </CardDescription>
                          </CardHeader>
                          <CardContent>
                            {applicationDAG ? (
                              <DAGVisualization dag={applicationDAG} />
                            ) : (
                              <div className="flex items-center justify-center h-64 text-muted-foreground">
                                {isLoadingComponents ? "加载组件数据中..." : "暂无组件数据"}
                              </div>
                            )}
                          </CardContent>
                        </Card>
                      </TabsContent>

                      <TabsContent value="list" className="space-y-4">
                        <div className="space-y-2">
                          {components.map((component) => (
                            <div key={component.id} className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50">
                              <div className="flex items-center space-x-4 flex-1">
                                <div className="flex items-center space-x-2">
                                  {getComponentTypeIcon(component.type)}
                                  <div>
                                    <h4 className="font-semibold">{component.name}</h4>
                                    <p className="text-sm text-muted-foreground">
                                      {getComponentTypeLabel(component.type)}
                                    </p>
                                  </div>
                                </div>
                                
                                <div className="flex items-center space-x-6 text-sm">
                                  <div className="flex items-center space-x-1">
                                    <span className="text-muted-foreground">镜像:</span>
                                    <span className="font-mono text-xs">{component.image}</span>
                                  </div>
                                  
                                  {component.ports.length > 0 && (
                                    <div className="flex items-center space-x-1">
                                      <span className="text-muted-foreground">端口:</span>
                                      <span className="font-mono text-xs">{component.ports.join(', ')}</span>
                                    </div>
                                  )}
                                  
                                  <div className="flex items-center space-x-1">
                                    <span className="text-muted-foreground">CPU:</span>
                                    <span>{component.resources.cpu} 核</span>
                                  </div>
                                  
                                  <div className="flex items-center space-x-1">
                                    <span className="text-muted-foreground">内存:</span>
                                    <span>{component.resources.memory} MB</span>
                                  </div>
                                  
                                  {(component.dependencies || []).length > 0 && (
                                    <div className="flex items-center space-x-1">
                                      <span className="text-muted-foreground">依赖:</span>
                                      <span className="text-xs">{(component.dependencies || []).length} 个</span>
                                    </div>
                                  )}
                                </div>
                              </div>
                              
                              <div className="flex items-center space-x-2">
                                <ComponentStatusIndicator status={component.status} />
                                
                                {component.status === "running" ? (
                                  <Button variant="outline" size="sm">
                                    <Square className="h-3 w-3 mr-1" />
                                    停止
                                  </Button>
                                ) : (
                                  <Button size="sm">
                                    <Play className="h-3 w-3 mr-1" />
                                    启动
                                  </Button>
                                )}
                                
                                <Button variant="outline" size="sm">
                                  <Terminal className="h-3 w-3" />
                                </Button>
                              </div>
                            </div>
                          ))}
                        </div>
                      </TabsContent>
                    </Tabs>
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="code">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center space-x-2">
                    <Code className="h-5 w-5" />
                    <span>代码浏览</span>
                  </CardTitle>
                  <CardDescription>
                    在线浏览和编辑应用源代码，基于 Monaco Editor 的现代化代码编辑体验
                  </CardDescription>
                </CardHeader>
                <CardContent className="p-0">
                  <CodeEditor 
                    appId={params.id as string} 
                    className="h-[600px]" 
                  />
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