"use client"

import { useState, useEffect, useRef, useMemo } from "react"
import { useParams, useRouter } from "next/navigation"
import { applicationsAPI, APIError } from "@/lib/api"
import { getWebSocketManager, disconnectWebSocketManager } from "@/lib/websocket"
import type { LogEntry, Application, DAG, DAGNode, DAGEdge, ControlNode, DataNode, DAGNodeStatus, GetApplicationActorsResponse, ActorRecord } from "@/lib/model"
import { formatDateTime, formatMemory } from "@/lib/utils"
import { Sidebar } from "@/components/sidebar"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Input } from "@/components/ui/input"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form"
import { Textarea } from "@/components/ui/textarea"
import { useForm } from "react-hook-form"
import { toast } from "sonner"
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
  Database,
  Network,
  Cpu,
  MemoryStick,
  HardDrive,
  FileText,
  Search,
  Filter,
  X,
  ExternalLink,
  Settings,
  Folder,
} from "lucide-react"
import { AutoSizer, CellMeasurer, CellMeasurerCache, List, type ListRowProps } from "react-virtualized"
import { Graph } from '@antv/g6'
import { ExtensionCategory, register } from '@antv/g6'
import { ReactNode } from '@antv/g6-extension-react'


// 组件类型定义 - 表示分布式部署的actor类型
// DAG节点显示信息
interface NodeDisplayInfo {
  id: string
  name: string
  type: "control" | "data"
  status: DAGNodeStatus | "unknown"
}

interface ActorViewItem {
  id: string
  displayName: string
  componentId?: string
  providerId?: string
  image?: string
  resourceUsage?: {
    cpu?: number
    memory?: number
    gpu?: number
  }
  calcLatency?: number
  linkLatency?: number
}

interface ActorGroupView {
  functionName: string
  actors: ActorViewItem[]
}

const pickString = (...values: unknown[]): string | undefined => {
  for (const value of values) {
    if (typeof value === "string" && value.trim().length > 0) {
      return value
    }
  }
  return undefined
}

const pickNumber = (...values: unknown[]): number | undefined => {
  for (const value of values) {
    if (typeof value === "number" && !Number.isNaN(value)) {
      return value
    }
  }
  return undefined
}

const normalizeActor = (actor: ActorRecord | undefined, functionName: string, index: number): ActorViewItem => {
  const fallbackId = `${functionName || "actor"}-${index + 1}`
  if (!actor || typeof actor !== "object") {
    return {
      id: fallbackId,
      displayName: fallbackId,
    }
  }

  const resolvedId =
    pickString(actor.id, actor.ID, actor.actor_id, actor.actorId, actor.actorID, actor.name) || fallbackId

  const component = (actor.component && typeof actor.component === "object") ? actor.component : undefined
  const componentId = component ? pickString(
    component.id,
    component.ID,
    component.component_id,
    component.componentId
  ) : undefined
  const providerId = component ? pickString(component.provider_id, component.providerId, component.providerID) : undefined
  const image = component && typeof component.image === "string" ? component.image : undefined

  const rawUsage =
    component && typeof component.resource_usage === "object" ? component.resource_usage :
    component && typeof component.resourceUsage === "object" ? component.resourceUsage :
    undefined

  const rawCpu = pickNumber(rawUsage?.cpu)
  const rawMemory = pickNumber(rawUsage?.memory)
  const rawGpu = pickNumber(rawUsage?.gpu)
  
  const resourceUsage = rawUsage ? {
    cpu: rawCpu !== undefined ? rawCpu / 1000 : undefined, // 转换为核
    memory: rawMemory, // 保持字节，前端格式化
    gpu: rawGpu,
  } : undefined

  const info = actor.info && typeof actor.info === "object" ? actor.info : undefined
  // 延迟信息：0也是有效值，需要特殊处理
  const calcLatency = (() => {
    if (info?.calc_latency !== undefined && typeof info.calc_latency === "number") {
      return info.calc_latency
    }
    if (info?.calcLatency !== undefined && typeof info.calcLatency === "number") {
      return info.calcLatency
    }
    if (info?.CalcLatency !== undefined && typeof info.CalcLatency === "number") {
      return info.CalcLatency
    }
    if ((actor as any)?.calcLatency !== undefined && typeof (actor as any).calcLatency === "number") {
      return (actor as any).calcLatency
    }
    return undefined
  })()
  
  const linkLatency = (() => {
    if (info?.link_latency !== undefined && typeof info.link_latency === "number") {
      return info.link_latency
    }
    if (info?.linkLatency !== undefined && typeof info.linkLatency === "number") {
      return info.linkLatency
    }
    if (info?.LinkLatency !== undefined && typeof info.LinkLatency === "number") {
      return info.LinkLatency
    }
    if ((actor as any)?.linkLatency !== undefined && typeof (actor as any).linkLatency === "number") {
      return (actor as any).linkLatency
    }
    return undefined
  })()

  const displayName = componentId ? `${componentId} (${resolvedId})` : resolvedId

  return {
    id: resolvedId,
    displayName,
    componentId,
    providerId,
    image,
    resourceUsage,
    calcLatency,
    linkLatency,
  }
}

const normalizeActorGroups = (data: GetApplicationActorsResponse | null | undefined): ActorGroupView[] => {
  if (!data || typeof data !== "object") {
    return []
  }

  return Object.entries(data)
    .map(([functionName, actorList]) => {
      const actorsArray = Array.isArray(actorList) ? actorList : []
      const normalizedActors = actorsArray.map((actor, index) => normalizeActor(actor, functionName, index))
      // 按actor id排序
      normalizedActors.sort((a, b) => a.id.localeCompare(b.id))
      return {
        functionName,
        actors: normalizedActors,
      }
    })
    .sort((a, b) => a.functionName.localeCompare(b.functionName))
}

// 注册G6 React节点扩展
register(ExtensionCategory.NODE, 'dag-react-node', ReactNode)

// DAG节点React组件
const DAGNodeComponent = ({ g6Node }: { g6Node: any }) => {
  console.log("DAGNodeComponent data:", g6Node)
  const { nodeType, nodeName, node } = g6Node.data
  
  const isControl = nodeType === "ControlNode"
  const isData = nodeType === "DataNode"
  
  const statusValue: DAGNodeStatus | "unknown" = (() => {
    if (isControl) {
      return (node as ControlNode)?.status ?? "unknown"
    }
    if (isData) {
      return (node as DataNode)?.status ?? "unknown"
    }
    return "unknown"
  })()

  const getStatusColor = () => {
    switch (statusValue) {
      case "done":
        return "bg-green-500"
      case "running":
        return "bg-blue-500"
      case "ready":
        return "bg-amber-400"
      case "failed":
        return "bg-red-500"
      default:
        return "bg-gray-400"
    }
  }

  const getStatusText = () => {
    switch (statusValue) {
      case "done":
        return "已完成"
      case "running":
        return "运行中"
      case "ready":
        return "就绪"
      case "failed":
        return "失败"
      case "pending":
        return "等待中"
      default:
        return "未知"
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

  const [isLoadingComponents, setIsLoadingComponents] = useState(false)// DAG 加载状态
  const [actorGroups, setActorGroups] = useState<ActorGroupView[]>([])
  const [isLoadingActorsList, setIsLoadingActorsList] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [isLoadingAppLogs, setIsLoadingAppLogs] = useState(false)
  const [logLines, setLogLines] = useState(100)
  const [activeTab, setActiveTab] = useState("files")
  const [logSearchTerm, setLogSearchTerm] = useState("")
  const [logLevelFilter, setLogLevelFilter] = useState<string>("all")
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const isSubmittingRef = useRef(false) // 使用 ref 跟踪提交状态，避免状态更新导致的重新渲染
  const [runnerEnvironments, setRunnerEnvironments] = useState<string[]>([])

  const applicationId = params.id as string
  const logLevelStyles: Record<string, { badge: string; dot: string; label: string }> = {
    error: { badge: "bg-red-100 text-red-800", dot: "bg-red-500", label: "错误" },
    warn: { badge: "bg-amber-100 text-amber-800", dot: "bg-amber-500", label: "警告" },
    debug: { badge: "bg-blue-100 text-blue-800", dot: "bg-blue-500", label: "调试" },
    trace: { badge: "bg-slate-100 text-slate-800", dot: "bg-slate-400", label: "追踪" },
    info: { badge: "bg-emerald-100 text-emerald-800", dot: "bg-emerald-500", label: "信息" },
  }

  // 编辑表单
  interface ApplicationFormData {
    name: string
    description?: string
    executeCmd: string
    envInstallCmd?: string
    runnerEnv?: string
  }

  const form = useForm<ApplicationFormData>({
    defaultValues: {
      name: "",
      description: "",
      executeCmd: "",
      envInstallCmd: "",
      runnerEnv: "",
    },
  })

  // 标记 WebSocket 是否已初始化，避免重复连接
  const wsInitializedRef = useRef(false)
  const updateTimerRef = useRef<NodeJS.Timeout | null>(null)
  const logViewerCacheRef = useRef(
    new CellMeasurerCache({
      fixedWidth: true,
      defaultHeight: 72,
    })
  )

  const fetchRunnerEnvironments = async () => {
    try {
      const response = await applicationsAPI.getRunnerEnvironments()
      setRunnerEnvironments(response.environments)
    } catch (fetchError) {
      console.error('Failed to fetch runner environments:', fetchError)
    }
  }

  useEffect(() => {
    loadApplicationDetail()
    loadAppDAG()
    loadActors()
    fetchRunnerEnvironments()
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
          loadActors()
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
    if (value === "dag" && applicationId) {
      loadAppDAG()
    }
    if (value === "components" && applicationId) {
      loadActors()
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

      const EDGE_LABEL_MAX_LENGTH = 10
      const EDGE_LABEL_BASE_RANKSEP = 32
      const EDGE_LABEL_PER_CHAR = 6

      let maxEdgeLabelLength = 0

      // 转换DAG数据为G6格式
      const g6Data = {
        nodes: dag.nodes.map((node, index) => {
          const nodeId = getNodeId(node, index)
          const nodeName = getNodeName(node, index)
          const nodeStatus: DAGNodeStatus | "unknown" =
            node.type === "ControlNode"
              ? (node.node as ControlNode)?.status ?? "unknown"
              : (node.node as DataNode)?.status ?? "unknown"
          
          return {
            id: nodeId,
            data: {
              id: nodeId,
              nodeType: node.type,
              nodeName: nodeName,
              node: node.node,
              status: nodeStatus,
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
          const fullLabel = edgeLabel
          let displayLabel = fullLabel
          if (fullLabel.length > EDGE_LABEL_MAX_LENGTH) {
            displayLabel = `${fullLabel.slice(0, EDGE_LABEL_MAX_LENGTH - 1)}…`
          }
          if (edgeLabel.length > maxEdgeLabelLength) {
            maxEdgeLabelLength = displayLabel.length
          }

          return {
            id: `edge-${index}`,
            source: edge.fromNodeId,
            target: edge.toNodeId,
            data: {
              displayLabel,
              fullLabel
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
      const dynamicRanksep = EDGE_LABEL_BASE_RANKSEP + Math.max(0, maxEdgeLabelLength * EDGE_LABEL_PER_CHAR)

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
            labelText: (d: any) => d.data?.displayLabel || '',
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
          ranksep: dynamicRanksep,
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


      // TODO: 添加tooltip，当前实现无效
      const tooltipEl = document.createElement('div')
      tooltipEl.style.position = 'fixed'
      tooltipEl.style.pointerEvents = 'none'
      tooltipEl.style.padding = '6px 10px'
      tooltipEl.style.background = 'rgba(15, 23, 42, 0.9)'
      tooltipEl.style.color = '#fff'
      tooltipEl.style.borderRadius = '6px'
      tooltipEl.style.fontSize = '12px'
      tooltipEl.style.lineHeight = '16px'
      tooltipEl.style.maxWidth = '320px'
      tooltipEl.style.wordBreak = 'break-all'
      tooltipEl.style.zIndex = '9999'
      tooltipEl.style.boxShadow = '0 8px 16px rgba(15, 23, 42, 0.35)'
      tooltipEl.style.opacity = '0'
      tooltipEl.style.transition = 'opacity 0.15s ease'
      document.body.appendChild(tooltipEl)

      const showTooltip = (content: string, x: number, y: number) => {
        tooltipEl.textContent = content
        tooltipEl.style.left = `${x + 16}px`
        tooltipEl.style.top = `${y + 16}px`
        tooltipEl.style.opacity = '1'
      }

      const hideTooltip = () => {
        tooltipEl.style.opacity = '0'
      }

      const handleEdgeTooltip = (evt: any) => {
        const item = evt?.item || evt?.target?.get?.('item')
        const model = item?.getModel?.()
        const displayLabel = model?.data?.displayLabel
        const fullLabel = model?.data?.fullLabel
        if (!fullLabel || fullLabel === displayLabel) {
          hideTooltip()
          return
        }
        const clientX = evt?.clientX ?? evt?.canvasX ?? 0
        const clientY = evt?.clientY ?? evt?.canvasY ?? 0
        showTooltip(fullLabel, clientX, clientY)
      }

      graph.on('edge:mouseenter', handleEdgeTooltip)
      graph.on('edge:mousemove', handleEdgeTooltip)
      graph.on('edge-label:mouseenter', handleEdgeTooltip)
      graph.on('edge-label:mousemove', handleEdgeTooltip)

      const handleLeave = () => {
        hideTooltip()
      }

      graph.on('edge:mouseleave', handleLeave)
      graph.on('edge-label:mouseleave', handleLeave)

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
        hideTooltip()
        if (tooltipEl.parentNode) {
          tooltipEl.parentNode.removeChild(tooltipEl)
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

  const filteredLogs = useMemo(() => {
    const searchTerm = logSearchTerm.trim().toLowerCase()
    const levelFilter = logLevelFilter.toLowerCase()

    return logs.filter((log) => {
      if (levelFilter !== "all" && log.level?.toLowerCase() !== levelFilter) {
        return false
      }
      if (searchTerm) {
        const content = `${log.message ?? ""} ${log.details ?? ""}`.toLowerCase()
        return content.includes(searchTerm)
      }
      return true
    })
  }, [logs, logLevelFilter, logSearchTerm])

  useEffect(() => {
    logViewerCacheRef.current.clearAll()
  }, [filteredLogs])

  const loadApplicationDetail = async () => {
    try {
      setIsLoading(true)
      setError(null)

      // 获取应用详情
      const appData: any = await applicationsAPI.getById(applicationId)

      if (!appData) {
        setError("应用不存在")
        return
      }

      // 转换后端返回的下划线字段名为驼峰格式
      const app: Application = {
        id: appData.id,
        name: appData.name,
        description: appData.description || "",
        gitUrl: appData.git_url,
        branch: appData.branch,
        status: (appData.status === "idle" ? "idle" : appData.status === "running" ? "running" : appData.status === "stopped" ? "stopped" : appData.status === "error" ? "error" : appData.status === "deploying" ? "deploying" : appData.status === "cloning" ? "cloning" : "idle") as Application["status"],
        lastDeployed: appData.last_deployed && appData.last_deployed !== "" && appData.last_deployed !== "0001-01-01T00:00:00Z" ? new Date(appData.last_deployed).toISOString() : undefined,
        runnerEnv: appData.runner_env,
        containerId: appData.container_id,
        executeCmd: appData.execute_cmd,
        envInstallCmd: appData.env_install_cmd,
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
  const [dagSessions, setDagSessions] = useState<string[]>([])
  const [selectedDagSession, setSelectedDagSession] = useState<string | null>(null)

  const loadAppDAG = async (sessionId?: string) => {
    if (!applicationId) return

    setIsLoadingComponents(true)
    try {
      const requestSession = sessionId ?? selectedDagSession ?? undefined
      const dagResponse = await applicationsAPI.getAppDAG(applicationId, requestSession)

      setDagSessions(dagResponse.sessions || [])
      const resolvedSession =
        dagResponse.selectedSessionId ||
        requestSession ||
        (dagResponse.sessions && dagResponse.sessions.length > 0 ? dagResponse.sessions[dagResponse.sessions.length - 1] : null)
      setSelectedDagSession(resolvedSession || null)
      setAppDAG(dagResponse.dag)
    } catch (error) {
      // DAG不存在是正常现象，应用只有在运行时才会有DAG
      // 只在非404错误时才记录错误日志
      if (error instanceof APIError && error.status === 404) {
        // 404错误是正常的，不记录日志
      } else {
        console.error('Failed to load DAG:', error)
      }
      setDagSessions([])
      setSelectedDagSession(null)
      setAppDAG(null)
    } finally {
      setIsLoadingComponents(false)
    }
  }

  const loadActors = async () => {
    if (!applicationId) return

    setIsLoadingActorsList(true)
    try {
      const response = await applicationsAPI.getActors(applicationId)
      setActorGroups(normalizeActorGroups(response))
    } catch (error) {
      console.error('Failed to load actors:', error)
      setActorGroups([])
    } finally {
      setIsLoadingActorsList(false)
    }
  }

  const handleDagSessionChange = (sessionId: string) => {
    setSelectedDagSession(sessionId)
    loadAppDAG(sessionId)
  }

  const handleRefreshDAG = () => {
    loadAppDAG(selectedDagSession ?? undefined)
  }

  const handleRefreshActors = () => {
    loadActors()
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

  const handleEdit = () => {
    if (!application) return
    isSubmittingRef.current = false // 重置提交标志
    form.setValue("name", application.name)
    form.setValue("description", application.description || "")
    form.setValue("executeCmd", application.executeCmd || "")
    form.setValue("envInstallCmd", application.envInstallCmd || "")
    form.setValue("runnerEnv", application.runnerEnv || "")
    setIsEditDialogOpen(true)
  }

  const handleEditDialogOpenChange = (open: boolean) => {
    setIsEditDialogOpen(open)
    // 延迟清除表单数据，等待关闭动画完成
    // 如果是提交关闭，不清除表单（由 handleUpdate 处理）
    if (!open && !isSubmittingRef.current) {
      setTimeout(() => {
        form.reset({
          name: "",
          description: "",
          executeCmd: "",
          envInstallCmd: "",
          runnerEnv: "",
        })
        form.clearErrors()
      }, 200) // 等待对话框关闭动画完成
    }
  }

  const handleUpdate = async (data: ApplicationFormData) => {
    if (!application) return

    // 先设置提交标志，确保在关闭对话框时不会被清除
    isSubmittingRef.current = true

    try {
      const updateData = {
        name: data.name,
        description: data.description || "",
        execute_cmd: data.executeCmd,
        env_install_cmd: data.envInstallCmd,
        runner_env: data.runnerEnv,
      }

      await applicationsAPI.update(application.id, updateData)
      
      toast.success(`应用 "${data.name}" 已成功更新`)
      
      // 使用 requestAnimationFrame 确保在下一帧再关闭对话框
      // 这样可以确保 isSubmittingRef.current 已经被正确设置
      requestAnimationFrame(() => {
        setIsEditDialogOpen(false)
        
        // 延迟清除表单、加载详情和重置标志，等待关闭动画完成
        // 对话框关闭动画通常需要 200-300ms，我们延迟 500ms 确保完全关闭
        setTimeout(async () => {
          // 先加载详情，再清除表单
          await loadApplicationDetail()
          
          // 再延迟一点清除表单，确保详情已经加载完成
          setTimeout(() => {
            form.reset({
              name: "",
              description: "",
              executeCmd: "",
              envInstallCmd: "",
              runnerEnv: "",
            })
            form.clearErrors()
            isSubmittingRef.current = false // 重置提交标志
          }, 100)
        }, 500)
      })
      
    } catch (error) {
      console.error('Failed to update application:', error)
      toast.error("应用更新时发生错误，请稍后重试")
      isSubmittingRef.current = false // 提交失败时重置标志
      // 提交失败时不关闭对话框，让用户可以看到错误并重试
    }
  }



  // 将 Git URL 转换为可在浏览器中打开的 HTTPS 格式
  const convertGitUrlToHttps = (url: string): string | null => {
    if (!url) return null

    // 如果已经是 HTTPS 格式，直接返回
    if (url.startsWith('http://') || url.startsWith('https://')) {
      // 移除 .git 后缀（如果有）
      return url.replace(/\.git$/, '')
    }

    // 处理 SSH 格式：git@github.com:user/repo.git
    const sshPattern = /^git@([^:]+):([\w\-.]+\/[\w\-.]+)(?:\.git)?$/
    const match = url.match(sshPattern)
    if (match) {
      const [, host, repo] = match
      // 将常见的主机名映射到 HTTPS URL
      if (host === 'github.com') {
        return `https://github.com/${repo}`
      } else if (host === 'gitlab.com') {
        return `https://gitlab.com/${repo}`
      } else if (host === 'bitbucket.org') {
        return `https://bitbucket.org/${repo}`
      } else {
        // 对于其他主机，尝试使用 HTTPS
        return `https://${host}/${repo}`
      }
    }

    // 无法转换，返回 null
    return null
  }

  const getStatusBadge = (status: string) => {
    const statusConfig = {
      running: { variant: "default" as const, label: "运行中", color: "bg-green-500" },
      stopped: { variant: "secondary" as const, label: "已停止", color: "bg-gray-500" },
      error: { variant: "destructive" as const, label: "错误", color: "bg-red-500" },
      deploying: { variant: "outline" as const, label: "部署中", color: "bg-blue-500" },
      cloning: { variant: "outline" as const, label: "克隆中", color: "bg-yellow-500" },
      idle: { variant: "secondary" as const, label: "未部署", color: "bg-gray-500" },
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
                <Package className="h-5 w-5" />
                <div>
                  <h1 className="text-2xl font-bold">{application.name}</h1>
                  <div className="flex items-center space-x-2 mt-1">
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
              <Button variant="outline" onClick={handleEdit}>
                <Settings className="h-4 w-4 mr-2" />
                编辑
              </Button>
            </div>
          </div>

          {/* 编辑应用对话框 */}
          <Dialog open={isEditDialogOpen} onOpenChange={handleEditDialogOpenChange}>
            <DialogContent className="sm:max-w-[550px]">
              <DialogHeader>
                <DialogTitle>编辑应用</DialogTitle>
                <DialogDescription>
                  修改应用配置信息
                </DialogDescription>
              </DialogHeader>

              <Form {...form}>
                <form onSubmit={form.handleSubmit(handleUpdate)} className="space-y-4">
                  <FormField
                    control={form.control}
                    name="name"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>应用名称</FormLabel>
                        <FormControl>
                          <Input placeholder="例如：用户管理系统" {...field} />
                        </FormControl>
                        <FormDescription>为这个应用起一个易识别的名称</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="envInstallCmd"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>环境安装命令（可选）</FormLabel>
                        <FormControl>
                          <Input 
                            placeholder="例如：pip install -r requirements.txt"
                            {...field} 
                          />
                        </FormControl>
                        <FormDescription>在运行应用前执行的依赖安装命令。若需要多行命令，请放在脚本文件中。</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="executeCmd"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>执行命令</FormLabel>
                        <FormControl>
                          <Input 
                            placeholder="例如：python app.py"
                            {...field} 
                          />
                        </FormControl>
                        <FormDescription>应用启动时执行的命令。若需要多行命令，请放在脚本文件中。</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="runnerEnv"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>运行环境</FormLabel>
                        <Select onValueChange={field.onChange} value={field.value}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="选择运行环境" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            {runnerEnvironments.map((env) => (
                              <SelectItem key={env} value={env}>
                                {env}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <FormDescription>选择应用的运行环境</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="description"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>描述（可选）</FormLabel>
                        <FormControl>
                          <Textarea placeholder="应用描述信息..." {...field} />
                        </FormControl>
                        <FormDescription>添加关于此应用的描述信息</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <DialogFooter>
                    <Button type="button" variant="outline" onClick={() => handleEditDialogOpenChange(false)}>
                      取消
                    </Button>
                    <Button type="submit">更新应用</Button>
                  </DialogFooter>
                </form>
              </Form>
            </DialogContent>
          </Dialog>

          {/* Application Info */}
          <Card>
            <CardHeader>
              <CardTitle>应用信息</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <h4 className="text-sm font-medium text-muted-foreground mb-1">描述</h4>
                <p className="text-sm">{application.description || "无描述"}</p>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-[2fr_1fr] lg:grid-cols-[2fr_1fr_1fr_1fr] gap-4">
                <div>
                  <h4 className="text-sm font-medium text-muted-foreground mb-1">Git仓库</h4>
                  <div className="space-y-2">
                    {application.gitUrl ? (
                      <div className="flex items-center space-x-2 text-sm">
                        <Package className="h-4 w-4" />
                        <span className="font-mono text-xs break-all">{application.gitUrl}</span>
                        {(() => {
                          const httpsUrl = convertGitUrlToHttps(application.gitUrl || '')
                          return httpsUrl ? (
                            <a 
                              href={httpsUrl} 
                              target="_blank" 
                              rel="noopener noreferrer"
                              className="ml-2 text-primary hover:text-primary/80"
                              title="在新标签页中打开仓库"
                            >
                              <ExternalLink className="h-3 w-3" />
                            </a>
                          ) : null
                        })()}
                      </div>
                    ) : (
                      <span className="text-sm text-muted-foreground">未设置</span>
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
                  <h4 className="text-sm font-medium text-muted-foreground mb-1">运行环境</h4>
                  <div className="flex items-center space-x-2 text-sm">
                    <Cpu className="h-4 w-4" />
                    <span>{application.runnerEnv || "未设置"}</span>
                  </div>
                </div>

                <div>
                  <h4 className="text-sm font-medium text-muted-foreground mb-1">函数数量</h4>
                  <div className="flex items-center space-x-2 text-sm">
                    <Package className="h-4 w-4" />
                    <span>{actorGroups.length} 个函数</span>
                    {isLoadingActorsList && (
                      <RefreshCw className="h-3 w-3 animate-spin" />
                    )}
                  </div>
                </div>

                <div>
                  <h4 className="text-sm font-medium text-muted-foreground mb-1">Actor数量</h4>
                  <div className="flex items-center space-x-2 text-sm">
                    <Package className="h-4 w-4" />
                    <span>{actorGroups.reduce((sum, group) => sum + group.actors.length, 0)} 个Actor</span>
                    {isLoadingActorsList && (
                      <RefreshCw className="h-3 w-3 animate-spin" />
                    )}
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div>
                  <h4 className="text-sm font-medium text-muted-foreground mb-1">环境安装命令</h4>
                  <div className="flex items-center space-x-2 text-sm">
                    <Terminal className="h-4 w-4" />
                    <span className="font-mono text-xs break-all whitespace-pre-wrap">{application.envInstallCmd || "未设置"}</span>
                  </div>
                </div>

                <div>
                  <h4 className="text-sm font-medium text-muted-foreground mb-1">执行命令</h4>
                  <div className="flex items-center space-x-2 text-sm">
                    <Terminal className="h-4 w-4" />
                    <span className="font-mono text-xs break-all whitespace-pre-wrap">{application.executeCmd || "未设置"}</span>
                  </div>
                </div>

                <div>
                  <h4 className="text-sm font-medium text-muted-foreground mb-1">最后部署</h4>
                  <div className="flex items-center space-x-2 text-sm">
                    <Clock className="h-4 w-4" />
                    <span className="text-xs">
                      {application.lastDeployed ? formatDateTime(application.lastDeployed) : "未部署"}
                    </span>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Tabs */}
          <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-4">
            <TabsList>
              <TabsTrigger value="files" className="flex items-center space-x-2">
                <Folder className="h-4 w-4" />
                <span>文件管理</span>
              </TabsTrigger>
              <TabsTrigger value="components" className="flex items-center space-x-2">
                <Package className="h-4 w-4" />
                <span>Actor管理</span>
              </TabsTrigger>
              <TabsTrigger value="dag" className="flex items-center space-x-2">
                <GitBranch className="h-4 w-4" />
                <span>DAG</span>
              </TabsTrigger>
              <TabsTrigger value="logs" className="flex items-center space-x-2">
                <FileText className="h-4 w-4" />
                <span>应用日志</span>
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

            <TabsContent value="files">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center space-x-2">
                    <Folder className="h-5 w-5" />
                    <span>文件管理</span>
                  </CardTitle>
                  <CardDescription>
                    浏览和编辑应用源代码文件
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

            <TabsContent value="dag">
              <Card>
                <CardHeader>
                  <div className="flex items-center justify-between">
                    <CardTitle className="flex items-center space-x-2">
                      <GitBranch className="h-5 w-5" />
                      <span>DAG</span>
                    </CardTitle>
                    <div className="flex items-center space-x-4">
                      {dagSessions.length > 0 && (
                        <div className="flex items-center space-x-2">
                          <label className="text-sm font-semibold text-foreground whitespace-nowrap">
                            执行会话：
                          </label>
                          <Select
                            value={selectedDagSession ?? dagSessions[dagSessions.length - 1]}
                            onValueChange={handleDagSessionChange}
                          >
                            <SelectTrigger className="w-64 font-mono text-sm bg-white">
                              <SelectValue placeholder="选择 Session" />
                            </SelectTrigger>
                            <SelectContent>
                              {dagSessions.map((sessionId) => (
                                <SelectItem key={sessionId} value={sessionId} className="font-mono">
                                  {sessionId}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>
                      )}
                      <Button variant="outline" size="sm" onClick={handleRefreshDAG} disabled={isLoadingComponents}>
                        <RefreshCw className={`h-4 w-4 mr-2 ${isLoadingComponents ? 'animate-spin' : ''}`} />
                        刷新
                      </Button>
                    </div>
                  </div>
                  <CardDescription>
                    显示应用 Actor 组件之间的依赖关系和数据流向
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
                          <Button variant="link" onClick={() => loadAppDAG(selectedDagSession ?? undefined)} className="mt-2">
                            点击加载DAG
                          </Button>
                        </div>
                      )}
                    </div>
                  ) : (
                    <DAGVisualization 
                      key={`dag-${appDAG.nodes.length}-${appDAG.edges.length}`} 
                      dag={appDAG} 
                    />
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="components">
              <Card>
                <CardHeader>
                  <div className="flex items-center justify-between">
                    <CardTitle className="flex items-center space-x-2">
                      <Package className="h-5 w-5" />
                      <span>Actor 管理</span>
                    </CardTitle>
                    <div className="flex items-center space-x-2">
                      <Button variant="outline" size="sm" onClick={handleRefreshActors} disabled={isLoadingActorsList}>
                        <RefreshCw className={`h-4 w-4 mr-2 ${isLoadingActorsList ? 'animate-spin' : ''}`} />
                        刷新
                      </Button>
                    </div>
                  </div>
                  <CardDescription>
                    按函数视角查看 Actor 运行实例、资源占用与实时延迟，便于跟踪调度效果
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {isLoadingActorsList ? (
                    <div className="flex items-center justify-center h-32">
                      <RefreshCw className="h-6 w-6 animate-spin mr-2" />
                      <span>加载组件列表中...</span>
                    </div>
                  ) : actorGroups.length > 0 ? (
                    <div className="space-y-4">
                      {actorGroups.map((group) => (
                        <div key={group.functionName} className="border rounded-lg p-4 bg-background">
                          <div className="flex flex-col gap-1 md:flex-row md:items-center md:justify-between">
                            <div className="flex items-center space-x-3">
                              <Package className="h-5 w-5 text-primary" />
                              <div>
                                <p className="font-semibold text-lg">{group.functionName}</p>
                                <p className="text-sm text-muted-foreground">
                                  绑定 {group.actors.length} 个 Actor 实例
                                </p>
                              </div>
                            </div>
                            <Badge variant="secondary" className="w-fit">
                              {group.actors.length} 个 Actor
                            </Badge>
                          </div>

                          {group.actors.length > 0 ? (
                            <div className="mt-4 space-y-2">
                              {group.actors.map((actor) => (
                                <div
                                  key={`${group.functionName}-${actor.id}`}
                                  className="p-4 border rounded-md bg-muted/30 flex flex-col gap-3 md:flex-row md:items-center md:justify-between"
                                >
                                  <div className="flex items-start space-x-3 flex-1">
                                    <div className="p-2 rounded-md bg-white border shadow-sm">
                                      <Package className="h-4 w-4 text-primary" />
                                    </div>
                                    <div className="flex-1">
                                      <p className="font-medium font-mono mb-2">{actor.id}</p>
                                      <div className="space-y-1">
                                        {actor.componentId && (
                                          <p className="text-xs text-muted-foreground font-mono flex items-center space-x-1">
                                            <Package className="h-3 w-3" />
                                            <span>组件 {actor.componentId}</span>
                                          </p>
                                        )}
                                        {actor.providerId && (
                                          <p className="text-xs text-muted-foreground font-mono flex items-center space-x-1">
                                            <Network className="h-3 w-3" />
                                            <span>运行在 {actor.providerId}</span>
                                          </p>
                                        )}
                                      </div>
                                    </div>
                                  </div>

                                  <div className="flex flex-col items-end gap-2">
                                    <div className="flex flex-wrap gap-4 text-xs text-muted-foreground md:justify-end md:text-right">
                                      {actor.resourceUsage?.cpu !== undefined && (
                                        <span className="flex items-center space-x-1">
                                          <Cpu className="h-3 w-3" />
                                          <span>CPU {actor.resourceUsage.cpu.toFixed(3)} 核</span>
                                        </span>
                                      )}
                                      {actor.resourceUsage?.memory !== undefined && (
                                        <span className="flex items-center space-x-1">
                                          <MemoryStick className="h-3 w-3" />
                                          <span>内存 {formatMemory(actor.resourceUsage.memory)}</span>
                                        </span>
                                      )}
                                      {actor.resourceUsage?.gpu !== undefined && actor.resourceUsage.gpu > 0 && (
                                        <span className="flex items-center space-x-1">
                                          <HardDrive className="h-3 w-3" />
                                          <span>GPU {actor.resourceUsage.gpu}</span>
                                        </span>
                                      )}
                                      <span className="flex items-center space-x-1">
                                        <Activity className="h-3 w-3" />
                                        <span>
                                          计算延迟 {actor.calcLatency !== undefined && actor.calcLatency !== null ? `${actor.calcLatency}ms` : "未知"}
                                        </span>
                                      </span>
                                      <span className="flex items-center space-x-1">
                                        <Globe className="h-3 w-3" />
                                        <span>
                                          链路延迟 {actor.linkLatency !== undefined && actor.linkLatency !== null ? `${actor.linkLatency}ms` : "未知"}
                                        </span>
                                      </span>
                                    </div>
                                    {actor.image && (
                                      <p className="text-xs text-muted-foreground font-mono break-all text-right">
                                        {actor.image}
                                      </p>
                                    )}
                                  </div>
                                </div>
                              ))}
                            </div>
                          ) : (
                            <div className="mt-4 text-sm text-muted-foreground">
                              尚未收到 {group.functionName} 的 Actor 运行实例
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="flex flex-col items-center justify-center h-32 text-muted-foreground space-y-2 text-sm">
                      <Package className="h-8 w-8 opacity-50" />
                      <span>尚未获取到 Actor 实例数据</span>
                      <span className="text-xs">启动应用或触发任务后，Actor 列表将自动填充</span>
                    </div>
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
                  {isLoadingAppLogs ? (
                    <div className="flex items-center justify-center h-32">
                      <RefreshCw className="h-6 w-6 animate-spin mr-2" />
                      <span>加载日志中...</span>
                    </div>
                  ) : filteredLogs.length === 0 ? (
                    <div className="flex flex-col items-center justify-center h-40 text-muted-foreground space-y-2 text-sm">
                      {logSearchTerm || logLevelFilter !== "all" ? (
                        <Filter className="h-8 w-8 opacity-50" />
                      ) : (
                        <FileText className="h-8 w-8 opacity-50" />
                      )}
                      <span>{logSearchTerm || logLevelFilter !== "all" ? "没有符合条件的日志" : "尚未获取到日志数据"}</span>
                      <span className="text-xs">
                        {logSearchTerm || logLevelFilter !== "all" ? "调整筛选条件或清空搜索后重试" : "启动应用或刷新后再试"}
                      </span>
                    </div>
                  ) : (
                    <div className="h-[500px] w-full border rounded-md bg-gray-50 dark:bg-gray-900">
                      <AutoSizer>
                        {({ height, width }: { height: number; width: number }) => (
                          <List
                            width={width}
                            height={height}
                            rowCount={filteredLogs.length}
                            deferredMeasurementCache={logViewerCacheRef.current}
                            rowHeight={logViewerCacheRef.current.rowHeight}
                            overscanRowCount={6}
                            rowRenderer={({ index, key, parent, style }: ListRowProps) => {
                              const log = filteredLogs[index]
                              const levelKey = (log.level || "info").toLowerCase()
                              const levelStyles = logLevelStyles[levelKey] || logLevelStyles.info

                              return (
                                <CellMeasurer
                                  cache={logViewerCacheRef.current}
                                  columnIndex={0}
                                  key={key}
                                  parent={parent}
                                  rowIndex={index}
                                >
                                  <div
                                    style={style}
                                    className="border-b border-gray-200/80 dark:border-gray-800/80 px-4 py-3 hover:bg-white dark:hover:bg-gray-900 transition-colors"
                                  >
                                    <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
                                      <div className="flex items-center gap-3">
                                        <span
                                          className={`px-2 py-0.5 rounded-full text-[11px] font-semibold uppercase tracking-wide ${levelStyles.badge}`}
                                        >
                                          {log.level?.toUpperCase() || "INFO"}
                                        </span>
                                        <span className="text-xs text-muted-foreground font-mono">
                                          {formatDateTime(log.timestamp)}
                                        </span>
                                      </div>
                                      <div className="text-[11px] text-muted-foreground font-mono flex items-center gap-2">
                                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-white border text-gray-700 dark:text-gray-300 dark:bg-gray-900">
                                          <span className={`w-2 h-2 rounded-full ${levelStyles.dot}`} />
                                          {levelStyles.label}
                                        </span>
                                        <span className="hidden md:inline">
                                          App: {log.app || application?.name || "未知"}
                                        </span>
                                      </div>
                                    </div>
                                    <p className="mt-2 text-sm text-gray-900 dark:text-gray-100 whitespace-pre-wrap break-words font-mono">
                                      {log.message}
                                    </p>
                                    {log.details && (
                                      <pre className="mt-2 bg-gray-100 dark:bg-gray-950 rounded-md p-2 text-xs text-gray-700 dark:text-gray-300 overflow-x-auto whitespace-pre-wrap break-words font-mono">
                                        {log.details}
                                      </pre>
                                    )}
                                  </div>
                                </CellMeasurer>
                              )
                            }}
                          />
                        )}
                      </AutoSizer>
                    </div>
                  )}
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