"use client"

import { useState, useEffect, useRef } from "react"
import { useParams, useRouter } from "next/navigation"
import { applicationsAPI, componentsAPI, APIError } from "@/lib/api"
import { getWebSocketManager, disconnectWebSocketManager } from "@/lib/websocket"
import type { LogEntry, Application, DAG, DAGNode, DAGEdge, ControlNode, DataNode, Component } from "@/lib/model"
import { Sidebar } from "@/components/sidebar"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Input } from "@/components/ui/input"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
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
  FileText,
  Info,
  AlertTriangle,
  AlertCircle,
  CheckCircle,
  Search,
  Filter,
  X,
} from "lucide-react"
import { Graph } from '@antv/g6'
import { ExtensionCategory, register } from '@antv/g6'
import { ReactNode } from '@antv/g6-extension-react'

interface CodeBrowserInfo {
  status: string
  port?: number
  start_time?: string
  work_dir?: string
}

// 组件类型定义 - 表示分布式部署的actor类型
// DAG节点显示信息
interface NodeDisplayInfo {
  id: string
  name: string
  type: "control" | "data"
  status: "running" | "pending" | "ready" | "unknown"
  done?: boolean
  ready?: boolean
}

// 注册G6 React节点扩展
register(ExtensionCategory.NODE, 'dag-react-node', ReactNode)

// DAG节点React组件
const DAGNodeComponent = ({ g6Node }: { g6Node: any }) => {
  console.log("DAGNodeComponent data:", g6Node)
  const { nodeType, nodeName, node } = g6Node.data
  
  const isControl = nodeType === "ControlNode"
  const isData = nodeType === "DataNode"
  
  const getStatusColor = () => {
    if (isControl) {
      return (node as ControlNode)?.done ? "bg-green-500" : "bg-gray-400"
    } else {
      return (node as DataNode)?.done ? "bg-green-500" : "bg-gray-400"
    }
  }

  const getStatusText = () => {
    if (isControl) {
      return (node as ControlNode)?.done ? "已完成" : "未开始"
    } else {
      return (node as DataNode)?.done ? "已完成" : "未开始"
    }
  }

  return (
    <div className="bg-white border-2 border-gray-300 rounded-lg shadow-sm p-3 min-w-[160px] min-h-[64px] flex flex-col justify-between hover:border-blue-400 transition-colors">
      <div className="flex items-center space-x-2">
        {isControl ? <Cpu className="w-3 h-3 text-blue-600 flex-shrink-0" /> : <Database className="w-3 h-3 text-green-600 flex-shrink-0" />}
        <span className="text-xs font-medium text-gray-800 truncate" title={nodeName}>
          {nodeName}
        </span>
      </div>
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-1">
          <div className={`w-2 h-2 rounded-full ${getStatusColor()}`} />
          <span className="text-xs text-gray-600">{getStatusText()}</span>
        </div>
        <span className="text-xs text-gray-500">
          {isControl ? "控制" : "数据"}
        </span>
      </div>
    </div>
  )
}

export default function ApplicationDetailPage() {
  const params = useParams()
  const router = useRouter()
  const [application, setApplication] = useState<Application | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const [isLoadingComponents, setIsLoadingComponents] = useState(false)// 组件列表状态
  const [components, setComponents] = useState<Component[]>([])
  const [isLoadingComponentsList, setIsLoadingComponentsList] = useState(false)
  
  // 组件日志状态
  const [selectedComponent, setSelectedComponent] = useState<string | null>(null)
  const [componentLogs, setComponentLogs] = useState<string>("")
  const [isLoadingComponentLogs, setIsLoadingComponentLogs] = useState(false)
  const [showLogsDialog, setShowLogsDialog] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [codeBrowserStatus, setCodeBrowserStatus] = useState<CodeBrowserInfo | null>(null)
  const [isStartingCodeBrowser, setIsStartingCodeBrowser] = useState(false)
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [isLoadingAppLogs, setIsLoadingAppLogs] = useState(false)
  const [logLines, setLogLines] = useState(100)
  const [activeTab, setActiveTab] = useState("components")
  const [logSearchTerm, setLogSearchTerm] = useState("")
  const [logLevelFilter, setLogLevelFilter] = useState<string>("all")

  const applicationId = params.id as string

  // 处理刷新DAG按钮点击
  const handleRefreshDAG = () => {
    loadAppDAG()
    loadComponents()
  }

  // 标记 WebSocket 是否已初始化，避免重复连接
  const wsInitializedRef = useRef(false)
  const updateTimerRef = useRef<NodeJS.Timeout | null>(null)

  useEffect(() => {
    loadApplicationDetail()
    loadAppDAG()
    loadComponents()
  }, [applicationId])

  // WebSocket 连接独立管理，避免与数据加载冲突
  useEffect(() => {
    // 建立WebSocket连接以接收实时DAG状态更新
    if (applicationId && !wsInitializedRef.current) {
      wsInitializedRef.current = true
      
      const wsManager = getWebSocketManager(applicationId)
      
      // 监听DAG状态变化事件 - 添加防抖避免频繁更新
      const handleDAGStateChange = (event: any) => {
        console.log('收到 DAG 状态变化事件:', event)
        
        // 清除之前的定时器
        if (updateTimerRef.current) {
          clearTimeout(updateTimerRef.current)
        }
        
        // 防抖：300ms 内的多次更新只执行最后一次
        updateTimerRef.current = setTimeout(() => {
          console.log('执行 DAG 数据更新')
          loadAppDAG()
          loadComponents()
        }, 300)
      }
      
      wsManager.addHandler(handleDAGStateChange)
      
      // 延迟连接，确保组件已完全挂载
      const connectTimer = setTimeout(() => {
        wsManager.connect().catch((error) => {
          console.log('WebSocket 连接失败（这不影响应用的其他功能）:', error.message)
        })
      }, 100)
      
      // 清理函数
      return () => {
        clearTimeout(connectTimer)
        if (updateTimerRef.current) {
          clearTimeout(updateTimerRef.current)
        }
        wsManager.removeHandler(handleDAGStateChange)
        disconnectWebSocketManager(applicationId)
        wsInitializedRef.current = false
      }
    }
  }, [applicationId])

  // 当日志行数改变时重新加载日志
  useEffect(() => {
    if (applicationId && activeTab === "logs") {
      loadLogs()
    }
  }, [logLines])

  // 处理标签页切换
  const handleTabChange = (value: string) => {
    setActiveTab(value)
    if (value === "logs" && applicationId) {
      loadLogs()
    }
  }



  // DAG图可视化组件 - 使用G6图可视化库
  const DAGVisualization = ({ dag }: { dag: DAG }) => {
    const containerRef = useRef<HTMLDivElement>(null)
    const graphRef = useRef<Graph | null>(null)

    useEffect(() => {
      if (!containerRef.current || !dag.nodes.length) return

      // 获取节点ID辅助函数
      const getNodeId = (node: DAGNode, index: number): string => {
        if (!node || !node.node) {
          return `node-${index}`
        }

        try {
          if (node.type === "ControlNode") {
            const controlNode = node.node as ControlNode
            return controlNode.id || `control-${index}`
          } else if (node.type === "DataNode") {
            const dataNode = node.node as DataNode
            return dataNode.id || `data-${index}`
          }
        } catch (error) {
          console.error(`Error getting node ID for index ${index}:`, error)
        }
        
        return `node-${index}`
      }

      // 获取节点名称辅助函数
      const getNodeName = (node: DAGNode, index: number): string => {
        if (!node || !node.node) return `Node ${index}`

        try {
          if (node.type === "ControlNode") {
            const controlNode = node.node as ControlNode
            return controlNode.functionName || `Control ${index}`
          } else if (node.type === "DataNode") {
            const dataNode = node.node as DataNode
            return `Data ${dataNode.lambda || index}`
          }
        } catch (error) {
          console.error(`Error getting node name for index ${index}:`, error)
        }
        
        return `Node ${index}`
      }

      // 转换DAG数据为G6格式
      const g6Data = {
        nodes: dag.nodes.map((node, index) => {
          const nodeId = getNodeId(node, index)
          const nodeName = getNodeName(node, index)
          
          return {
            id: nodeId,
            data: {
              id: nodeId,
              nodeType: node.type,
              nodeName: nodeName,
              node: node.node,
              status: node.type === "ControlNode" 
                ? ((node.node as ControlNode)?.done ? "done" : "pending")
                : ((node.node as DataNode)?.done ? "done" : "pending")
            },
          }
        }),
        edges: dag.edges.map((edge, index) => {
          let edgeLabel = ''
          if (edge.info) {
            if (typeof edge.info === 'string') {
              edgeLabel = edge.info
            } else if (typeof edge.info === 'object') {
              edgeLabel = Object.entries(edge.info)
                .map(([key, value]) => `${key}: ${value}`)
                .join(', ')
            } else {
              edgeLabel = String(edge.info)
            }
          }

          return {
            id: `edge-${index}`,
            source: edge.fromNodeId,
            target: edge.toNodeId,
            data: {
              label: edgeLabel
            },
            style: {
              stroke: '#94a3b8',
              lineWidth: 1.5,
              endArrow: true
            }
          }
        })
      }

      // 创建G6图实例
      const graph = new Graph({
        container: containerRef.current,
        padding: 20,
        data: g6Data,
        node: {
          type: 'dag-react-node',
          style: {
            size: [160, 64],
            component: (node: any) => <DAGNodeComponent g6Node={node} />
          }
        },
        edge: {
          style: {
            stroke: '#94a3b8',
            lineWidth: 1.5,
            endArrow: true,
            labelText: (d: any) => d.data?.label || '',
            labelFill: '#475569',
            labelFontSize: 10,
            labelTextAlign: 'center',
            labelTextBaseline: 'middle',
            labelOffsetX: -2,
            labelOffsetY: 0,
            labelPosition: 'center',
            labelBackground: true,
            labelBackgroundFill: '#ffffff',
            labelBackgroundOpacity: 0.8,
            labelBackgroundRadius: 4,
            labelPadding: [2, 4]
          }
        },
        layout: {
          type: 'dagre',
          rankdir: 'LR', // 从左到右
          nodesep: 160,
          ranksep: 60,
          controlPoints: true
        },
        behaviors: [
          // 'drag-element',
          {
            type: 'zoom-canvas',
            key: 'zoom-canvas-1', // 为交互指定标识符，方便动态更新
            sensitivity: 0.5, // 设置灵敏度
            // trigger: ['Control']
          },
          'drag-canvas'
        ],
        autoFit: {
          type: 'view',
          options: {
            // 仅适用于 'view' 类型
            when: 'always', // 何时适配：'overflow'(仅当内容溢出时) 或 'always'(总是适配)
            direction: 'both', // 适配方向：'x'、'y' 或 'both'
          },
        },
        autoResize: true
      })

      // 渲染图形 - 使用 try-catch 捕获可能的错误
      try {
        graph.render()
      } catch (error) {
        console.error('图表渲染失败:', error)
        return
      }

      // 保存图实例引用
      graphRef.current = graph

      // 清理函数
      return () => {
        if (graphRef.current) {
          try {
            graphRef.current.destroy()
            graphRef.current = null
          } catch (error) {
            // 忽略销毁时的错误，这通常不是问题
            console.debug('图表销毁时出错:', error)
          }
        }
      }
    }, [dag])

    return (
      <div 
        ref={containerRef} 
        className="w-full h-[500px] border rounded-lg bg-gray-50"
        style={{ minHeight: '500px' }}
      />
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

  const loadLogs = async () => {
    if (!applicationId) return

    try {
      setIsLoadingAppLogs(true)
      const response = await applicationsAPI.getLogsParsed(applicationId, logLines)
      setLogs(response.logs || [])
    } catch (err) {
      console.error('Failed to load logs:', err)
      setLogs([])
    } finally {
      setIsLoadingAppLogs(false)
    }
  }

  const loadApplicationDetail = async () => {
    try {
      setIsLoading(true)
      setError(null)

      // 获取应用详情
      const response = await applicationsAPI.getAll()
      const app = response.applications.find((app: Application) => app.id === applicationId)

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

  const [appDAG, setAppDAG] = useState<DAG | null>(null)

  const loadAppDAG = async () => {
    if (!applicationId) return

    setIsLoadingComponents(true)
    try {
      const dagResponse = await applicationsAPI.getAppDAG(applicationId)

      setAppDAG(dagResponse.dag)
    } catch (error) {
      // DAG不存在是正常现象，应用只有在运行时才会有DAG
      // 只在非404错误时才记录错误日志
      if (error instanceof APIError && error.status === 404) {
        // 404错误是正常的，不记录日志
      } else {
        console.error('Failed to load DAG:', error)
      }
      setAppDAG(null)
    } finally {
      setIsLoadingComponents(false)
    }
  }

  const loadComponents = async () => {
    if (!applicationId) return

    setIsLoadingComponentsList(true)
    try {
      const response = await applicationsAPI.getComponents(applicationId) as { components: Component[] }
      setComponents(response.components || [])
    } catch (error) {
      console.error('Failed to load components:', error)
      setComponents([])
    } finally {
      setIsLoadingComponentsList(false)
    }
  }

  // 加载组件日志
  const loadComponentLogs = async (componentName: string) => {
    if (!applicationId || !componentName) return
    
    setIsLoadingComponentLogs(true)
    try {
      const response = await componentsAPI.getLogs(applicationId, componentName) as { logs: string }
      setComponentLogs(response.logs || "")
    } catch (error) {
      console.error('Failed to load component logs:', error)
      setComponentLogs("获取日志失败: " + (error instanceof Error ? error.message : String(error)))
    } finally {
      setIsLoadingComponentLogs(false)
    }
  }

  // 处理查看组件日志
  const handleViewComponentLogs = (componentName: string) => {
    setSelectedComponent(componentName)
    setShowLogsDialog(true)
    loadComponentLogs(componentName)
  }

  const handleStart = async () => {
    if (!application) return

    try {
      await applicationsAPI.run(application.id)
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
                    <span>{appDAG?.nodes.length || 0} 个节点</span>
                    {isLoadingComponents && (
                      <RefreshCw className="h-3 w-3 animate-spin" />
                    )}
                  </div>
                </div>

                {application.ports && application.ports.length > 0 && (
                  <div>
                    <h4 className="text-sm font-medium text-muted-foreground mb-1">端口</h4>
                    <div className="flex flex-wrap gap-1">
                      {application.ports.map((port: number, index: number) => (
                        <Badge key={index} variant="outline" className="text-xs font-mono">
                          {port}
                        </Badge>
                      ))}
                    </div>
                  </div>
                )}

                {application.executeCmd && (
                  <div>
                    <h4 className="text-sm font-medium text-muted-foreground mb-1">执行命令</h4>
                    <div className="flex items-center space-x-2 text-sm">
                      <Terminal className="h-4 w-4" />
                      <span className="font-mono text-xs break-all">{application.executeCmd}</span>
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
                      {application.runningOn.map((resource: string, index: number) => (
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
          <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-4">
            <TabsList>
              <TabsTrigger value="components" className="flex items-center space-x-2">
                <Package className="h-4 w-4" />
                <span>组件管理</span>
              </TabsTrigger>
              <TabsTrigger value="logs" className="flex items-center space-x-2">
                <FileText className="h-4 w-4" />
                <span>应用日志</span>
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
                      <Button variant="outline" size="sm" onClick={handleRefreshDAG} disabled={isLoadingComponents}>
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
                  {!appDAG || appDAG.nodes.length === 0 ? (
                    <div className="flex items-center justify-center h-64 text-gray-500">
                      {isLoadingComponents ? (
                        <div className="flex items-center space-x-2">
                          <RefreshCw className="h-4 w-4 animate-spin" />
                          <span>加载DAG中...</span>
                        </div>
                      ) : (
                        <div className="text-center">
                          <Package className="h-8 w-8 mx-auto mb-2 opacity-50" />
                          <p>暂无DAG数据</p>
                          <Button variant="link" onClick={loadAppDAG} className="mt-2">
                            点击加载DAG
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
                            {appDAG ? (
                              <DAGVisualization 
                                key={`dag-${appDAG.nodes.length}-${appDAG.edges.length}`} 
                                dag={appDAG} 
                              />
                            ) : (
                              <div className="flex items-center justify-center h-64 text-muted-foreground">
                                {isLoadingComponents ? "加载组件数据中..." : "暂无组件数据"}
                              </div>
                            )}
                          </CardContent>
                        </Card>
                      </TabsContent>

                      <TabsContent value="list" className="space-y-4">
                        {isLoadingComponentsList ? (
                          <div className="flex items-center justify-center h-32">
                            <RefreshCw className="h-6 w-6 animate-spin mr-2" />
                            <span>加载组件列表中...</span>
                          </div>
                        ) : components.length > 0 ? (
                          <div className="space-y-2">
                            {components.map((component, index) => {
                              const getStatusColor = (status: string) => {
                                switch (status) {
                                  case "running": return "bg-green-500"
                                  case "deploying": return "bg-blue-500"
                                  case "stopped": return "bg-gray-500"
                                  case "failed": return "bg-red-500"
                                  case "pending": return "bg-yellow-500"
                                  default: return "bg-gray-400"
                                }
                              }

                              const getStatusText = (status: string) => {
                                switch (status) {
                                  case "running": return "运行中"
                                  case "deploying": return "部署中"
                                  case "stopped": return "已停止"
                                  case "failed": return "失败"
                                  case "pending": return "等待中"
                                  default: return "未知"
                                }
                              }

                              return (
                                <div key={component.name || `component-${index}`} className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50">
                                  <div className="flex items-center space-x-4 flex-1">
                                    <div className="flex items-center space-x-2">
                                      <Package className="w-4 h-4" />
                                      <div>
                                        <h4 className="font-semibold">{component.name}</h4>
                                        <p className="text-sm text-muted-foreground font-mono text-xs">
                                          {component.image}
                                        </p>
                                      </div>
                                    </div>

                                    <div className="flex items-center space-x-6 text-sm">
                                      <div className="flex items-center space-x-1">
                                        <span className="text-muted-foreground">Provider:</span>
                                        <span className="font-mono text-xs">{component.provider_id}</span>
                                      </div>

                                      <div className="flex items-center space-x-1">
                                        <Cpu className="w-3 h-3" />
                                        <span className="text-muted-foreground">CPU:</span>
                                        <span className="font-mono text-xs">{component.resource_usage.cpu}</span>
                                      </div>

                                      <div className="flex items-center space-x-1">
                                        <MemoryStick className="w-3 h-3" />
                                        <span className="text-muted-foreground">内存:</span>
                                        <span className="font-mono text-xs">{component.resource_usage.memory}MB</span>
                                      </div>

                                      {component.resource_usage.gpu > 0 && (
                                        <div className="flex items-center space-x-1">
                                          <HardDrive className="w-3 h-3" />
                                          <span className="text-muted-foreground">GPU:</span>
                                          <span className="font-mono text-xs">{component.resource_usage.gpu}</span>
                                        </div>
                                      )}
                                    </div>
                                  </div>

                                  <div className="flex items-center space-x-2">
                                    <div className="flex items-center space-x-1">
                                      <div className={`w-2 h-2 rounded-full ${getStatusColor(component.status)}`} />
                                      <span className="text-xs text-muted-foreground">
                                        {getStatusText(component.status)}
                                      </span>
                                    </div>
                                    
                                    <Button 
                                      variant="outline" 
                                      size="sm"
                                      onClick={() => handleViewComponentLogs(component.name)}
                                    >
                                      <FileText className="w-3 h-3 mr-1" />
                                      日志
                                    </Button>
                                  </div>
                                </div>
                              )
                            })}
                          </div>
                        ) : (
                          <div className="flex items-center justify-center h-32 text-muted-foreground">
                            <Package className="h-8 w-8 mr-2 opacity-50" />
                            <span>暂无组件数据</span>
                          </div>
                        )}
                      </TabsContent>
                    </Tabs>
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="logs">
              <Card>
                <CardHeader>
                  <div className="flex items-center justify-between">
                    <div>
                      <CardTitle className="flex items-center space-x-2">
                        <FileText className="h-5 w-5" />
                        <span>应用日志</span>
                      </CardTitle>
                      <CardDescription>
                        查看应用运行时的实时日志输出
                      </CardDescription>
                    </div>
                    <div className="flex items-center space-x-2">
                      <Select value={logLines.toString()} onValueChange={(value) => setLogLines(Number(value))}>
                        <SelectTrigger className="w-32">
                          <SelectValue placeholder="选择行数" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="50">最近 50 行</SelectItem>
                          <SelectItem value="100">最近 100 行</SelectItem>
                          <SelectItem value="200">最近 200 行</SelectItem>
                          <SelectItem value="500">最近 500 行</SelectItem>
                        </SelectContent>
                      </Select>
                      <Button variant="outline" size="sm" onClick={loadLogs} disabled={isLoadingAppLogs}>
                        <RefreshCw className={`h-4 w-4 mr-2 ${isLoadingAppLogs ? 'animate-spin' : ''}`} />
                        刷新
                      </Button>
                    </div>
                  </div>
                  <div className="flex items-center space-x-2 mt-4">
                    <div className="relative flex-1">
                      <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" />
                      <Input
                        placeholder="搜索日志内容..."
                        value={logSearchTerm}
                        onChange={(e) => setLogSearchTerm(e.target.value)}
                        className="pl-10"
                      />
                      {logSearchTerm && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="absolute right-1 top-1/2 transform -translate-y-1/2 h-6 w-6 p-0"
                          onClick={() => setLogSearchTerm("")}
                        >
                          <X className="h-3 w-3" />
                        </Button>
                      )}
                    </div>
                    <Select value={logLevelFilter} onValueChange={setLogLevelFilter}>
                      <SelectTrigger className="w-32">
                        <Filter className="h-4 w-4 mr-2" />
                        <SelectValue placeholder="级别" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">所有级别</SelectItem>
                        <SelectItem value="error">错误</SelectItem>
                        <SelectItem value="warn">警告</SelectItem>
                        <SelectItem value="info">信息</SelectItem>
                        <SelectItem value="debug">调试</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </CardHeader>
                <CardContent>
                  <ScrollArea className="h-[500px] w-full border rounded-md p-4 bg-gray-50 dark:bg-gray-900">
                    {isLoadingAppLogs ? (
                      <div className="flex items-center justify-center h-32">
                        <RefreshCw className="h-6 w-6 animate-spin mr-2" />
                        <span>加载日志中...</span>
                      </div>
                    ) : logs.length > 0 ? (
                      (() => {
                        // 过滤日志
                        const filteredLogs = logs.filter(log => {
                          // 级别过滤
                          if (logLevelFilter !== "all" && log.level.toLowerCase() !== logLevelFilter) {
                            return false
                          }
                          // 搜索过滤
                          if (logSearchTerm) {
                            const searchLower = logSearchTerm.toLowerCase()
                            return log.message.toLowerCase().includes(searchLower) ||
                              (log.details && log.details.toLowerCase().includes(searchLower))
                          }
                          return true
                        })

                        // 高亮搜索文本的函数
                        const highlightText = (text: string, searchTerm: string) => {
                          if (!searchTerm) return text
                          const regex = new RegExp(`(${searchTerm.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi')
                          const parts = text.split(regex)
                          return parts.map((part, index) =>
                            regex.test(part) ?
                              <span key={index} className="bg-yellow-200 dark:bg-yellow-800 px-1 rounded">{part}</span> :
                              part
                          )
                        }

                        const getLevelIcon = (level: string) => {
                          switch (level.toLowerCase()) {
                            case 'error':
                              return <AlertCircle className="h-4 w-4 text-red-500" />
                            case 'warn':
                            case 'warning':
                              return <AlertTriangle className="h-4 w-4 text-yellow-500" />
                            case 'info':
                              return <Info className="h-4 w-4 text-blue-500" />
                            case 'debug':
                              return <Terminal className="h-4 w-4 text-gray-500" />
                            default:
                              return <CheckCircle className="h-4 w-4 text-green-500" />
                          }
                        }

                        const getLevelColor = (level: string) => {
                          switch (level.toLowerCase()) {
                            case 'error':
                              return 'text-red-600 dark:text-red-400'
                            case 'warn':
                            case 'warning':
                              return 'text-yellow-600 dark:text-yellow-400'
                            case 'info':
                              return 'text-blue-600 dark:text-blue-400'
                            case 'debug':
                              return 'text-gray-600 dark:text-gray-400'
                            default:
                              return 'text-green-600 dark:text-green-400'
                          }
                        }

                        return (
                          <div className="space-y-2">
                            {(logSearchTerm || logLevelFilter !== "all") && (
                              <div className="mb-3 text-sm text-gray-600 dark:text-gray-400 border-b pb-2">
                                显示 {filteredLogs.length} / {logs.length} 条日志
                                {logSearchTerm && <span className="ml-2">搜索: "{logSearchTerm}"</span>}
                                {logLevelFilter !== "all" && <span className="ml-2">级别: {logLevelFilter}</span>}
                              </div>
                            )}
                            {filteredLogs.length > 0 ? (
                              filteredLogs.map((log, index) => (
                                <div key={log.id || index} className="border-l-2 border-gray-200 dark:border-gray-700 pl-4 py-2 hover:bg-gray-50 dark:hover:bg-gray-800 rounded-r">
                                  <div className="flex items-start space-x-2">
                                    <div className="flex items-center space-x-2 min-w-0 flex-1">
                                      {getLevelIcon(log.level)}
                                      <span className={`text-xs font-medium uppercase tracking-wide ${getLevelColor(log.level)}`}>
                                        {log.level}
                                      </span>
                                      <span className="text-xs text-gray-500 dark:text-gray-400">
                                        {new Date(log.timestamp).toLocaleString()}
                                      </span>
                                    </div>
                                  </div>
                                  <div className="mt-1 text-sm font-mono text-gray-800 dark:text-gray-200">
                                    {highlightText(log.message, logSearchTerm)}
                                  </div>
                                  {log.details && (
                                    <div className="mt-1 text-xs text-gray-600 dark:text-gray-400 font-mono bg-gray-100 dark:bg-gray-800 p-2 rounded">
                                      {highlightText(log.details, logSearchTerm)}
                                    </div>
                                  )}
                                </div>
                              ))
                            ) : (
                              <div className="flex items-center justify-center h-32 text-muted-foreground">
                                <Search className="h-8 w-8 mr-2 opacity-50" />
                                <span>没有找到匹配的日志</span>
                              </div>
                            )}
                          </div>
                        )
                      })()
                    ) : (
                      <div className="flex items-center justify-center h-32 text-muted-foreground">
                        <FileText className="h-8 w-8 mr-2 opacity-50" />
                        <span>暂无日志数据</span>
                      </div>
                    )}
                  </ScrollArea>
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

      {/* 组件日志对话框 */}
      <Dialog open={showLogsDialog} onOpenChange={setShowLogsDialog}>
        <DialogContent className="max-w-4xl max-h-[80vh]">
          <DialogHeader>
            <DialogTitle>组件日志 - {selectedComponent}</DialogTitle>
            <DialogDescription>
              查看组件的运行日志信息
            </DialogDescription>
          </DialogHeader>
          <div className="flex-1 overflow-hidden">
            {isLoadingComponentLogs ? (
              <div className="flex items-center justify-center h-64">
                <RefreshCw className="h-6 w-6 animate-spin mr-2" />
                <span>加载日志中...</span>
              </div>
            ) : (
              <ScrollArea className="h-96 w-full rounded border">
                <div className="p-4">
                  <pre className="text-sm font-mono whitespace-pre-wrap break-words">
                    {componentLogs || "暂无日志数据"}
                  </pre>
                </div>
              </ScrollArea>
            )}
          </div>
          <div className="flex justify-end space-x-2 pt-4">
            <Button 
              variant="outline" 
              onClick={() => selectedComponent && loadComponentLogs(selectedComponent)}
              disabled={isLoadingComponentLogs}
            >
              <RefreshCw className="w-4 h-4 mr-2" />
              刷新
            </Button>
            <Button variant="outline" onClick={() => setShowLogsDialog(false)}>
              关闭
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}