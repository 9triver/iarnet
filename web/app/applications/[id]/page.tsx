"use client"

import { useState, useEffect } from "react"
import { useParams, useRouter } from "next/navigation"
import { applicationsAPI } from "@/lib/api"
import type { LogEntry, Application, DAG, DAGNode, DAGEdge, ControlNode, DataNode } from "@/lib/model"
import { Sidebar } from "@/components/sidebar"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Input } from "@/components/ui/input"
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

export default function ApplicationDetailPage() {
  const params = useParams()
  const router = useRouter()
  const [application, setApplication] = useState<Application | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const [isLoadingComponents, setIsLoadingAppDAG] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [codeBrowserStatus, setCodeBrowserStatus] = useState<CodeBrowserInfo | null>(null)
  const [isStartingCodeBrowser, setIsStartingCodeBrowser] = useState(false)
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [isLoadingLogs, setIsLoadingLogs] = useState(false)
  const [logLines, setLogLines] = useState(100)
  const [activeTab, setActiveTab] = useState("components")
  const [logSearchTerm, setLogSearchTerm] = useState("")
  const [logLevelFilter, setLogLevelFilter] = useState<string>("all")

  const applicationId = params.id as string

  useEffect(() => {
    loadApplicationDetail()
    loadAppDAG()
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



  // DAG图可视化组件
  // TODO: 使用 antv 库的 G6 图可视化库
  const DAGVisualization = ({ dag }: { dag: DAG }) => {
    const [selectedNode, setSelectedNode] = useState<string | null>(null)
    const nodes = dag.nodes
    const edges = dag.edges

    // 拓扑排序算法，计算节点的层级
    const calculateNodeLevels = () => {
      const nodeIds = nodes.map((node, index) => getNodeId(node, index))
      const nodeIndexMap = new Map(nodeIds.map((id, index) => [id, index]))
      
      // 构建邻接表（入度图）
      const inDegree = new Map<string, number>()
      const adjacencyList = new Map<string, string[]>()
      
      // 初始化
      nodeIds.forEach(id => {
        inDegree.set(id, 0)
        adjacencyList.set(id, [])
      })
      
      // 构建图结构
       edges.forEach(edge => {
         const fromId = edge.fromNodeId
         const toId = edge.toNodeId
         
         if (nodeIds.includes(fromId) && nodeIds.includes(toId)) {
           // 数据流向：from -> to 表示数据从 from 流向 to
           // 所以 from 应该在 to 的左侧（更早的层级）
           adjacencyList.get(fromId)?.push(toId)
           inDegree.set(toId, (inDegree.get(toId) || 0) + 1)
         }
       })
      
      // 拓扑排序计算层级
      const levels = new Map<string, number>()
      const queue: string[] = []
      
      // 找到所有入度为0的节点（数据源节点，没有输入数据的节点）
      nodeIds.forEach(id => {
        if (inDegree.get(id) === 0) {
          queue.push(id)
          levels.set(id, 0)
        }
      })
      
      // BFS计算层级
      while (queue.length > 0) {
        const currentId = queue.shift()!
        const currentLevel = levels.get(currentId) || 0
        
        adjacencyList.get(currentId)?.forEach(neighborId => {
          const newInDegree = (inDegree.get(neighborId) || 0) - 1
          inDegree.set(neighborId, newInDegree)
          
          if (newInDegree === 0) {
            queue.push(neighborId)
            levels.set(neighborId, currentLevel + 1)
          }
        })
      }
      
      // 对于没有被处理的节点（可能存在循环依赖），给它们分配默认层级
      nodeIds.forEach(id => {
        if (!levels.has(id)) {
          levels.set(id, 0)
        }
      })
      
      return levels
    }

    // 布局参数
    const nodeWidth = 160  // 节点宽度
    const nodeHeight = 64  // 节点高度
    const levelSpacing = 240  // 层级间距（增加以使边更长）
    const nodeSpacing = 90   // 同层节点间距（稍微增加）
    const startX = 50
    const startY = 50

    // 改进的DAG布局算法
    const getNodePosition = (nodeIndex: number) => {
      const node = nodes[nodeIndex]
      if (!node) return { x: 0, y: 0 }

      const nodeId = getNodeId(node, nodeIndex)
      const levels = calculateNodeLevels()
      const level = levels.get(nodeId) || 0
      
      // 计算每层的节点数量和当前节点在该层的位置
      const nodesInLevel = nodes.filter((n, i) => {
        const nId = getNodeId(n, i)
        return levels.get(nId) === level
      })
      
      const nodePositionInLevel = nodesInLevel.findIndex((n, i) => {
        const nId = getNodeId(n, nodes.indexOf(n))
        return nId === nodeId
      })
      
      // 计算位置：被依赖的节点在左侧，依赖其他节点的在右侧
      const x = startX + level * levelSpacing
      const y = startY + nodePositionInLevel * (nodeHeight + nodeSpacing)

      return { x, y }
    }

    // 获取节点ID - 修复逻辑
    const getNodeId = (node: DAGNode, index: number): string => {
      if (!node || !node.node) {
        console.warn(`Node at index ${index} is missing or has no node data`)
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

    // 获取节点名称
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

    // 计算图形的实际尺寸
    const calculateGraphDimensions = () => {
      if (nodes.length === 0) return { width: 800, height: 400 }
      
      const positions = nodes.map((node, index) => getNodePosition(index))
      const maxX = Math.max(...positions.map(pos => pos.x)) + nodeWidth + 50
      const maxY = Math.max(...positions.map(pos => pos.y)) + nodeHeight + 50
      
      return {
        width: Math.max(maxX, 800),
        height: Math.max(maxY, 400)
      }
    }

    const graphDimensions = calculateGraphDimensions()

    // 调试信息：打印节点和边的信息
    console.log('DAG Visualization Debug Info:')
    console.log('Nodes:', nodes.map((node, index) => ({
      index,
      type: node.type,
      id: getNodeId(node, index),
      name: getNodeName(node, index),
      node: node.node
    })))
    console.log('Edges:', edges.map((edge, index) => ({
      index,
      fromNodeId: edge.fromNodeId,
      toNodeId: edge.toNodeId,
      info: edge.info,
      infoType: typeof edge.info
    })))
    console.log('Graph dimensions:', graphDimensions)

    return (
      <div className="relative w-full h-[500px] border rounded-lg bg-gray-50 overflow-auto">
        <svg 
          width={graphDimensions.width} 
          height={graphDimensions.height}
          className="block"
        >
          {/* 箭头标记定义 */}
          <defs>
            <marker
              id="arrowhead"
              markerWidth="10"
              markerHeight="7"
              refX="9"
              refY="3.5"
              orient="auto"
              markerUnits="strokeWidth"
            >
              <polygon points="0 0, 10 3.5, 0 7" fill="#94a3b8" />
            </marker>
          </defs>
          
          {/* 绘制连接线 */}
          {edges.map((edge, index) => {
            console.log(`Processing edge ${index}:`, edge)
            
            const fromIndex = nodes.findIndex((node, i) => {
              const nodeId = getNodeId(node, i)
              console.log(`Checking node ${i} with ID ${nodeId} against fromNodeId ${edge.fromNodeId}`)
              return nodeId === edge.fromNodeId
            })
            
            const toIndex = nodes.findIndex((node, i) => {
              const nodeId = getNodeId(node, i)
              console.log(`Checking node ${i} with ID ${nodeId} against toNodeId ${edge.toNodeId}`)
              return nodeId === edge.toNodeId
            })

            console.log(`Edge ${index}: fromIndex=${fromIndex}, toIndex=${toIndex}`)

            if (fromIndex === -1 || toIndex === -1) {
              console.warn(`Edge ${index} skipped: fromIndex=${fromIndex}, toIndex=${toIndex}`)
              return null
            }

            const fromPos = getNodePosition(fromIndex)
            const toPos = getNodePosition(toIndex)

            // 计算连接点：数据流从左到右，出边从右侧边缘出发，入边指向左侧边缘
            const fromX = fromPos.x + nodeWidth  // 从节点右侧边缘出发
            const fromY = fromPos.y + nodeHeight / 2  // 节点垂直中点
            const toX = toPos.x  // 指向节点左侧边缘
            const toY = toPos.y + nodeHeight / 2  // 节点垂直中点

            // 计算连线中点位置（用于显示info文本）
            const midX = (fromX + toX) / 2 - 4
            const midY = (fromY + toY) / 2

            // 修复info字段处理 - info是string类型，不是对象
            let infoText = ''
            if (edge.info) {
              if (typeof edge.info === 'string') {
                infoText = edge.info
              } else if (typeof edge.info === 'object') {
                // 如果实际上是对象，则转换为字符串
                infoText = Object.entries(edge.info)
                  .map(([key, value]) => `${key}: ${value}`)
                  .join(', ')
              } else {
                infoText = String(edge.info)
              }
            }

            console.log(`Edge ${index} info text:`, infoText)

            return (
              <g key={index}>
                <line
                  x1={fromX}
                  y1={fromY}
                  x2={toX}
                  y2={toY}
                  stroke="#94a3b8"
                  strokeWidth="1.5"
                  markerEnd="url(#arrowhead)"
                  strokeDasharray="none"
                  opacity="0.8"
                />
                {/* 在连线中间显示info文本 */}
                {infoText && (
                  <g>
                    {/* 背景框 - 使用更精确的文本宽度计算 */}
                    <rect
                      x={midX - (infoText.length * 5.5 + 16) / 2}
                      y={midY - 9}
                      width={infoText.length * 5.5 + 16}
                      height={18}
                      fill="white"
                      stroke="#94a3b8"
                      strokeWidth="0.5"
                      rx="4"
                      opacity="0.95"
                    />
                    {/* 文本内容 */}
                    <text
                      x={midX}
                      y={midY + 1}
                      textAnchor="middle"
                      dominantBaseline="middle"
                      fontSize="10"
                      fill="#475569"
                      className="font-sans"
                    >
                      {infoText}
                    </text>
                  </g>
                )}
              </g>
            )
          })}
        </svg>

        {/* 绘制节点 */}
        {nodes.map((node, index) => {
          const position = getNodePosition(index)
          const nodeId = getNodeId(node, index)
          const nodeName = getNodeName(node, index)
          const isControl = node.type === "ControlNode"

          return (
            <div
              key={nodeId}
              className={`absolute border-2 rounded-lg shadow-sm cursor-pointer transition-all bg-white 
                ${selectedNode === nodeId ?
                  "border-blue-500 shadow-md" : "border-gray-300"
                }`}
              style={{ 
                left: position.x, 
                top: position.y,
                width: '160px',
                height: '64px'
              }}
              onClick={() => setSelectedNode(nodeId)}
            >
              <div className="p-2 h-full flex flex-col justify-between">
                <div className="flex items-center space-x-1">
                  {isControl ? <Cpu className="w-3 h-3" /> : <Database className="w-3 h-3" />}
                  <span className="text-xs font-medium truncate">{nodeName}</span>
                </div>
                <div className="flex items-center space-x-1">
                  <div className={`w-2 h-2 rounded-full ${isControl
                    ? ((node.node as any)?.Done ? "bg-green-500" : "bg-yellow-500")
                    : ((node.node as any)?.ready ? "bg-green-500" : "bg-gray-400")
                    }`} />
                  <span className="text-xs text-muted-foreground">
                    {isControl ? "控制节点" : "数据节点"}
                  </span>
                </div>
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

  const loadLogs = async () => {
    if (!applicationId) return

    try {
      setIsLoadingLogs(true)
      const response = await applicationsAPI.getLogsParsed(applicationId, logLines)
      setLogs(response.logs || [])
    } catch (err) {
      console.error('Failed to load logs:', err)
      setLogs([])
    } finally {
      setIsLoadingLogs(false)
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

    setIsLoadingAppDAG(true)
    try {
      const dagResponse = await applicationsAPI.getAppDAG(applicationId)

      setAppDAG(dagResponse.dag)
    } catch (error) {
      console.error('Failed to load DAG:', error)
      setAppDAG(null)
    } finally {
      setIsLoadingAppDAG(false)
    }
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
                      <Button variant="outline" size="sm" onClick={loadAppDAG} disabled={isLoadingComponents}>
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
                              <DAGVisualization dag={appDAG} />
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
                          {appDAG?.nodes.map((node, index) => {
                            const isControl = node.type === "ControlNode"
                            const isData = node.type === "DataNode"
                            const nodeData = node.node as ControlNode | DataNode
                            const nodeId = nodeData?.id
                            const controlNode = isControl ? nodeData as ControlNode : null
                            const dataNode = isData ? nodeData as DataNode : null
                            const nodeName = isControl
                              ? (controlNode?.functionName || `Control Node ${index}`)
                              : `Data Node ${index}`

                            return (
                              <div key={nodeId || `node-${index}`} className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50">
                                <div className="flex items-center space-x-4 flex-1">
                                  <div className="flex items-center space-x-2">
                                    {isControl ? <Activity className="w-4 h-4" /> : <Database className="w-4 h-4" />}
                                    <div>
                                      <h4 className="font-semibold">{nodeName}</h4>
                                      <p className="text-sm text-muted-foreground">
                                        {isControl ? "控制节点" : "数据节点"}
                                      </p>
                                    </div>
                                  </div>

                                  <div className="flex items-center space-x-6 text-sm">
                                    <div className="flex items-center space-x-1">
                                      <span className="text-muted-foreground">类型:</span>
                                      <span className="font-mono text-xs">{node.type}</span>
                                    </div>

                                    {isControl && controlNode?.functionType && (
                                      <div className="flex items-center space-x-1">
                                        <span className="text-muted-foreground">函数类型:</span>
                                        <span className="font-mono text-xs">{controlNode?.functionType}</span>
                                      </div>
                                    )}

                                    {isData && dataNode?.lambda && (
                                      <div className="flex items-center space-x-1">
                                        <span className="text-muted-foreground">Lambda:</span>
                                        <span className="font-mono text-xs">{dataNode?.lambda}</span>
                                      </div>
                                    )}
                                  </div>
                                </div>

                                <div className="flex items-center space-x-2">
                                  <div className="flex items-center space-x-1">
                                    <div className={`w-2 h-2 rounded-full ${isControl
                                      ? (controlNode?.done ? "bg-green-500" : "bg-yellow-500")
                                      : (dataNode?.ready ? "bg-green-500" : "bg-gray-400")
                                      }`} />
                                    <span className="text-xs text-muted-foreground">
                                      {isControl
                                        ? (controlNode?.done ? "已完成" : "进行中")
                                        : (dataNode?.ready ? "就绪" : "未就绪")
                                      }</span>
                                  </div>
                                </div>
                              </div>
                            )
                          })}
                        </div>
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
                      <Button variant="outline" size="sm" onClick={loadLogs} disabled={isLoadingLogs}>
                        <RefreshCw className={`h-4 w-4 mr-2 ${isLoadingLogs ? 'animate-spin' : ''}`} />
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
                    {isLoadingLogs ? (
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
    </div>
  )
}