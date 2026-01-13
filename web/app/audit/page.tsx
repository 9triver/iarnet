"use client"

import { useState, useEffect, useMemo, useRef } from "react"
import { useRouter } from "next/navigation"
import { cn } from "@/lib/utils"
import { Sidebar } from "@/components/sidebar"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Badge } from "@/components/ui/badge"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Calendar } from "@/components/ui/calendar"
import { DateTimePicker } from "@/components/ui/date-time-picker"
import { AuthGuard } from "@/components/auth-guard"
import { formatDateTime } from "@/lib/utils"
import { auditAPI } from "@/lib/api"
import { canAccessAudit } from "@/lib/permissions"
import { useIARNetStore } from "@/lib/store"
import { format } from "date-fns"
import { zhCN } from "date-fns/locale"
import {
  FileText,
  RefreshCw,
  Search,
  Filter,
  X,
  Activity,
  User,
  Clock,
  Calendar as CalendarIcon,
  Download,
  ArrowRight,
  Globe,
} from "lucide-react"
import { AutoSizer, CellMeasurer, CellMeasurerCache, List, type ListRowProps } from "react-virtualized"

const LOG_LEVEL_STYLES: Record<string, { badge: string; dot: string; label: string }> = {
  error: { badge: "bg-red-100 text-red-800", dot: "bg-red-500", label: "错误" },
  warn: { badge: "bg-amber-100 text-amber-800", dot: "bg-amber-500", label: "警告" },
  debug: { badge: "bg-blue-100 text-blue-800", dot: "bg-blue-500", label: "调试" },
  trace: { badge: "bg-slate-100 text-slate-800", dot: "bg-slate-400", label: "追踪" },
  info: { badge: "bg-emerald-100 text-emerald-800", dot: "bg-emerald-500", label: "信息" },
}

// 系统日志条目
type SystemLogEntry = {
  id: string
  timestamp?: string
  level?: string
  message: string
  details?: string
  caller?: {
    file?: string
    line?: number
    function?: string
  }
}

// 操作日志条目
type OperationLogEntry = {
  id: string
  user: string
  operation: string
  resource_id: string
  resource_type: string
  action: string
  before?: Record<string, any>
  after?: Record<string, any>
  timestamp: string
  ip?: string
}

// 系统日志查看器
const SystemLogListViewer = ({ logs }: { logs: SystemLogEntry[] }) => {
  const cacheRef = useRef(
    new CellMeasurerCache({
      fixedWidth: true,
      defaultHeight: 72,
    })
  )

  useEffect(() => {
    cacheRef.current.clearAll()
  }, [logs])

  if (logs.length === 0) {
    return null
  }

  return (
    <AutoSizer>
      {({ height, width }: { height: number; width: number }) => (
        <List
          width={width}
          height={height}
          rowCount={logs.length}
          deferredMeasurementCache={cacheRef.current}
          rowHeight={cacheRef.current.rowHeight}
          overscanRowCount={6}
          rowRenderer={({ index, key, parent, style }: ListRowProps) => {
            const log = logs[index]
            const levelKey = (log.level || "info").toLowerCase()
            const levelStyles = LOG_LEVEL_STYLES[levelKey] || LOG_LEVEL_STYLES.info

            return (
              <CellMeasurer
                cache={cacheRef.current}
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
                    <div className="flex items-center gap-3 flex-wrap">
                      <span
                        className={`px-2 py-0.5 rounded-full text-[11px] font-semibold uppercase tracking-wide ${levelStyles.badge}`}
                      >
                        {(log.level ?? "INFO").toUpperCase()}
                      </span>
                      <span className="text-xs text-muted-foreground font-mono">
                        {log.timestamp ? formatDateTime(log.timestamp) : "—"}
                      </span>
                    </div>
                    <div className="text-[11px] text-muted-foreground font-mono flex items-center gap-2">
                      {log.caller && (
                        <span className="hidden md:inline">
                          {[
                            log.caller.file,
                            log.caller.line !== undefined && log.caller.line !== 0 ? `:${log.caller.line}` : null,
                            log.caller.function ? ` ${log.caller.function}` : null,
                          ]
                            .filter(Boolean)
                            .join("")}
                        </span>
                      )}
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
  )
}

// 操作日志查看器
const OperationLogListViewer = ({ logs }: { logs: OperationLogEntry[] }) => {
  const cacheRef = useRef(
    new CellMeasurerCache({
      fixedWidth: true,
      defaultHeight: 120,
    })
  )

  useEffect(() => {
    cacheRef.current.clearAll()
  }, [logs])

  if (logs.length === 0) {
    return null
  }

  // 操作类型映射
  const operationLabels: Record<string, string> = {
    create_application: "创建应用",
    update_application: "更新应用",
    delete_application: "删除应用",
    run_application: "运行应用",
    stop_application: "停止应用",
    create_file: "创建文件",
    update_file: "更新文件",
    delete_file: "删除文件",
    create_directory: "创建目录",
    delete_directory: "删除目录",
    register_resource: "注册资源",
    update_resource: "更新资源",
    delete_resource: "删除资源",
  }

  return (
    <AutoSizer>
      {({ height, width }: { height: number; width: number }) => (
        <List
          width={width}
          height={height}
          rowCount={logs.length}
          deferredMeasurementCache={cacheRef.current}
          rowHeight={cacheRef.current.rowHeight}
          overscanRowCount={6}
          rowRenderer={({ index, key, parent, style }: ListRowProps) => {
            const log = logs[index]
            const operationLabel = operationLabels[log.operation] || log.operation

            return (
              <CellMeasurer
                cache={cacheRef.current}
                columnIndex={0}
                key={key}
                parent={parent}
                rowIndex={index}
              >
                <div
                  style={style}
                  className="border-b border-gray-200/80 dark:border-gray-800/80 px-4 py-3 hover:bg-white dark:hover:bg-gray-900 transition-colors"
                >
                  <div className="flex flex-col gap-3">
                    <div className="flex items-center justify-between flex-wrap gap-2">
                      <div className="flex items-center gap-3 flex-wrap">
                        <span className="text-xs text-muted-foreground font-mono">
                          {formatDateTime(log.timestamp)}
                        </span>
                        <div className="flex items-center gap-1 text-xs text-muted-foreground">
                          <User className="h-3 w-3" />
                          <span className="font-medium">{log.user}</span>
                        </div>
                        <Badge variant="outline" className="text-xs">
                          {operationLabel}
                        </Badge>
                        {log.ip && (
                          <div className="flex items-center gap-1 text-xs text-muted-foreground">
                            <Globe className="h-3 w-3" />
                            <span>{log.ip}</span>
                          </div>
                        )}
                      </div>
                      <div className="text-xs text-muted-foreground">
                        <span className="font-medium">{log.resource_type}</span>
                        {log.resource_id && (
                          <span className="ml-1 font-mono">: {log.resource_id}</span>
                        )}
                      </div>
                    </div>
                    <div className="text-sm text-gray-900 dark:text-gray-100">
                      {log.action}
                    </div>
                    {(log.before || log.after) && (
                      <div className="flex gap-4 mt-2">
                        {log.before && Object.keys(log.before).length > 0 && (
                          <div className="flex-1">
                            <div className="text-xs text-muted-foreground mb-1">操作前</div>
                            <pre className="bg-red-50 dark:bg-red-950/20 rounded-md p-2 text-xs text-gray-700 dark:text-gray-300 overflow-x-auto whitespace-pre-wrap break-words font-mono border border-red-200 dark:border-red-900">
                              {JSON.stringify(log.before, null, 2)}
                            </pre>
                          </div>
                        )}
                        {log.after && Object.keys(log.after).length > 0 && (
                          <div className="flex-1">
                            <div className="text-xs text-muted-foreground mb-1">操作后</div>
                            <pre className="bg-green-50 dark:bg-green-950/20 rounded-md p-2 text-xs text-gray-700 dark:text-gray-300 overflow-x-auto whitespace-pre-wrap break-words font-mono border border-green-200 dark:border-green-900">
                              {JSON.stringify(log.after, null, 2)}
                            </pre>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              </CellMeasurer>
            )
          }}
        />
      )}
    </AutoSizer>
  )
}

export default function AuditPage() {
  // 计算默认时间范围：最近两小时，结束时间为当前时间+10分钟
  const getDefaultTimeRange = () => {
    const now = new Date()
    const twoHoursAgo = new Date(now.getTime() - 2 * 60 * 60 * 1000)
    const tenMinutesLater = new Date(now.getTime() + 10 * 60 * 1000)
    return {
      startTime: twoHoursAgo.toISOString(),
      endTime: tenMinutesLater.toISOString(),
    }
  }

  const router = useRouter()
  const currentUser = useIARNetStore((state) => state.currentUser)
  const defaultTimeRange = getDefaultTimeRange()
  const [systemLogs, setSystemLogs] = useState<SystemLogEntry[]>([])
  const [operationLogs, setOperationLogs] = useState<OperationLogEntry[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [logType, setLogType] = useState<"operations" | "all">("operations")
  const [logSearchTerm, setLogSearchTerm] = useState("")
  const [logLevelFilter, setLogLevelFilter] = useState<string>("all")
  
  // 为两个标签页分别设置独立的时间状态
  const [operationsTimeRange, setOperationsTimeRange] = useState<{
    startTime: string
    endTime: string
  }>({
    startTime: defaultTimeRange.startTime,
    endTime: defaultTimeRange.endTime,
  })
  
  const [allLogsTimeRange, setAllLogsTimeRange] = useState<{
    startTime: string
    endTime: string
  }>({
    startTime: defaultTimeRange.startTime,
    endTime: defaultTimeRange.endTime,
  })

  // 更新时间范围：结束时间为当前时间+10分钟
  const updateTimeRange = () => {
    const now = new Date()
    const twoHoursAgo = new Date(now.getTime() - 2 * 60 * 60 * 1000)
    const tenMinutesLater = new Date(now.getTime() + 10 * 60 * 1000)
    setOperationsTimeRange({
      startTime: twoHoursAgo.toISOString(),
      endTime: tenMinutesLater.toISOString(),
    })
    setAllLogsTimeRange({
      startTime: twoHoursAgo.toISOString(),
      endTime: tenMinutesLater.toISOString(),
    })
  }
  
  // 检查权限：只有平台管理员和超级管理员可以访问
  const hasPermission = canAccessAudit(currentUser?.role)

  // 无权限时重定向到资源管理页面
  useEffect(() => {
    if (currentUser && !hasPermission) {
      router.replace("/resources")
    }
  }, [currentUser, hasPermission, router])
  
  // 根据当前标签页获取对应的时间范围
  const currentTimeRange = logType === "operations" ? operationsTimeRange : allLogsTimeRange
  const startTime = currentTimeRange.startTime
  const endTime = currentTimeRange.endTime
  
  // 设置时间范围的函数
  const setStartTime = (value: string) => {
    if (logType === "operations") {
      setOperationsTimeRange(prev => ({ ...prev, startTime: value }))
    } else {
      setAllLogsTimeRange(prev => ({ ...prev, startTime: value }))
    }
  }
  
  const setEndTime = (value: string) => {
    if (logType === "operations") {
      setOperationsTimeRange(prev => ({ ...prev, endTime: value }))
    } else {
      setAllLogsTimeRange(prev => ({ ...prev, endTime: value }))
    }
  }
  
  const [error, setError] = useState<string | null>(null)

  // 页面加载时更新时间范围（结束时间为当前时间+10分钟）
  useEffect(() => {
    updateTimeRange()
  }, [])

  const loadLogs = async () => {
    try {
      setIsLoading(true)
      setError(null)

      if (logType === "operations") {
        // 加载操作日志
        const timeRange = operationsTimeRange
        const params: any = {}
        
        if (timeRange.startTime) {
          params.startTime = timeRange.startTime
        }
        if (timeRange.endTime) {
          params.endTime = timeRange.endTime
        }
        if (!timeRange.startTime && !timeRange.endTime) {
          params.limit = 100
        }
        
        const response = await auditAPI.getOperations(params)
        const logs = response.logs || []
        
        const normalizedLogs: OperationLogEntry[] = logs.map((log: any, index: number) => ({
          id: log.id || `${log.timestamp || Date.now()}-${index}`,
          user: log.user || "unknown",
          operation: log.operation || "",
          resource_id: log.resource_id || "",
          resource_type: log.resource_type || "",
          action: log.action || "",
          before: log.before,
          after: log.after,
          timestamp: log.timestamp || new Date().toISOString(),
          ip: log.ip,
        }))
        
        setOperationLogs(normalizedLogs)
      } else {
        // 加载系统日志
        const timeRange = allLogsTimeRange
        const params: any = {}
        
        if (timeRange.startTime) {
          params.startTime = timeRange.startTime
        }
        if (timeRange.endTime) {
          params.endTime = timeRange.endTime
        }
        if (!timeRange.startTime && !timeRange.endTime) {
          params.limit = 100
        }
        
        const response = await auditAPI.getLogs(params)
        const logs = (response as any).logs || (response as any).data?.logs || []
        
        const normalizedLogs: SystemLogEntry[] = logs.map((log: any, index: number) => {
          let timestampStr: string | undefined = undefined
          if (log.timestamp !== undefined && log.timestamp !== null) {
            const timestampMs = Number(log.timestamp) / 1000000
            timestampStr = new Date(timestampMs).toISOString()
          }
          
          const levelMap: Record<number, string> = {
            0: "unknown",
            1: "trace",
            2: "debug",
            3: "info",
            4: "warn",
            5: "error",
            6: "fatal",
            7: "panic",
          }
          const levelNum = typeof log.level === 'number' ? log.level : parseInt(log.level, 10)
          const levelStr = levelMap[levelNum] || log.level || "info"
          
          let details: string | undefined = undefined
          if (log.fields && Array.isArray(log.fields) && log.fields.length > 0) {
            details = log.fields.map((f: any) => `${f.key}: ${f.value}`).join("\n")
          }
          
          let caller: { file?: string; line?: number; function?: string } | undefined = undefined
          if (log.caller) {
            const file = log.caller.file && log.caller.file.trim() ? log.caller.file.trim() : undefined
            const line = log.caller.line !== undefined && log.caller.line !== 0 ? Number(log.caller.line) : undefined
            const func = log.caller.function && log.caller.function.trim() ? log.caller.function.trim() : undefined
            
            if (file || line || func) {
              caller = { file, line, function: func }
            }
          }
          
          return {
            id: `${log.timestamp || Date.now()}-${index}`,
            timestamp: timestampStr,
            level: levelStr,
            message: log.message || log.content || "",
            details,
            caller,
          }
        })
        
        setSystemLogs(normalizedLogs)
      }
    } catch (err: any) {
      console.error('Failed to load logs:', err)
      setError(err.message || "加载日志失败")
      if (logType === "operations") {
        setOperationLogs([])
      } else {
        setSystemLogs([])
      }
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    loadLogs()
  }, [logType, operationsTimeRange.startTime, operationsTimeRange.endTime, allLogsTimeRange.startTime, allLogsTimeRange.endTime])

  const filteredSystemLogs = useMemo(() => {
    const searchTerm = logSearchTerm.trim().toLowerCase()
    const levelFilter = logLevelFilter.toLowerCase()

    return systemLogs.filter((log) => {
      if (levelFilter !== "all" && log.level?.toLowerCase() !== levelFilter) {
        return false
      }
      if (searchTerm) {
        const content = `${log.message ?? ""} ${log.details ?? ""}`.toLowerCase()
        return content.includes(searchTerm)
      }
      return true
    })
  }, [systemLogs, logLevelFilter, logSearchTerm])

  const filteredOperationLogs = useMemo(() => {
    const searchTerm = logSearchTerm.trim().toLowerCase()

    return operationLogs.filter((log) => {
      if (searchTerm) {
        const content = `${log.user ?? ""} ${log.action ?? ""} ${log.operation ?? ""} ${log.resource_type ?? ""} ${log.resource_id ?? ""}`.toLowerCase()
        return content.includes(searchTerm)
      }
      return true
    })
  }, [operationLogs, logSearchTerm])

  const currentLogs = logType === "operations" ? filteredOperationLogs : filteredSystemLogs

  // 导出日志为 CSV 格式
  const exportToCSV = () => {
    if (logType === "operations") {
      const headers = ["时间", "用户", "操作类型", "操作描述", "资源类型", "资源ID", "IP地址"]
      const rows = filteredOperationLogs.map((log: OperationLogEntry) => {
        const timestamp = formatDateTime(log.timestamp)
        const user = log.user || "—"
        const operation = log.operation || "—"
        const action = (log.action || "").replace(/"/g, '""')
        const resourceType = log.resource_type || "—"
        const resourceId = log.resource_id || "—"
        const ip = log.ip || "—"
        
        return [timestamp, user, operation, action, resourceType, resourceId, ip]
      })
      
      const csvContent = [
        headers.join(","),
        ...rows.map((row: string[]) => row.map((cell: string) => `"${cell}"`).join(",")),
      ].join("\n")

      const blob = new Blob(["\uFEFF" + csvContent], { type: "text/csv;charset=utf-8;" })
      const url = URL.createObjectURL(blob)
      const link = document.createElement("a")
      link.href = url
      const fileName = `操作日志_${format(new Date(), "yyyy-MM-dd_HH-mm-ss")}.csv`
      link.download = fileName
      link.click()
      URL.revokeObjectURL(url)
      return
    }

    const headers = ["时间", "级别", "消息", "详情", "调用位置"]
    const rows = filteredSystemLogs.map((log: SystemLogEntry) => {
      const timestamp = log.timestamp ? formatDateTime(log.timestamp) : "—"
      const level = (log.level || "INFO").toUpperCase()
      const message = (log.message || "").replace(/"/g, '""')
      const details = (log.details || "").replace(/"/g, '""')
      const caller = log.caller
        ? `${log.caller.file || ""}${log.caller.line ? `:${log.caller.line}` : ""}${log.caller.function ? ` ${log.caller.function}` : ""}`
        : "—"
      
      return [timestamp, level, message, details, caller]
    })

    const csvContent = [
      headers.join(","),
      ...rows.map((row: string[]) => row.map((cell: string) => `"${cell}"`).join(",")),
    ].join("\n")

    const blob = new Blob(["\uFEFF" + csvContent], { type: "text/csv;charset=utf-8;" })
    const url = URL.createObjectURL(blob)
    const link = document.createElement("a")
    link.href = url
    const fileName = `系统日志_${format(new Date(), "yyyy-MM-dd_HH-mm-ss")}.csv`
    link.download = fileName
    link.click()
    URL.revokeObjectURL(url)
  }

  // 导出日志为 JSON 格式
  const exportToJSON = () => {
    const jsonContent = JSON.stringify(
      {
        type: logType === "operations" ? "操作日志" : "系统日志",
        exportTime: new Date().toISOString(),
        timeRange: {
          startTime: currentTimeRange.startTime,
          endTime: currentTimeRange.endTime,
        },
        total: currentLogs.length,
        logs: currentLogs,
      },
      null,
      2
    )

    const blob = new Blob([jsonContent], { type: "application/json;charset=utf-8;" })
    const url = URL.createObjectURL(blob)
    const link = document.createElement("a")
    link.href = url
    const fileName = `${logType === "operations" ? "操作日志" : "系统日志"}_${format(new Date(), "yyyy-MM-dd_HH-mm-ss")}.json`
    link.download = fileName
    link.click()
    URL.revokeObjectURL(url)
  }

  // 如果没有权限，显示权限不足页面
  if (!hasPermission) {
    return (
      <AuthGuard>
        <div className="flex h-screen bg-background">
          <Sidebar />
          <main className="flex-1 overflow-auto flex items-center justify-center">
            <Card className="max-w-md">
              <CardHeader>
                <CardTitle>权限不足</CardTitle>
                <CardDescription>您没有权限访问日志审计功能</CardDescription>
              </CardHeader>
            </Card>
          </main>
        </div>
      </AuthGuard>
    )
  }

  // 无权限时不展示具体内容，等待重定向到资源管理页面
  if (currentUser && !hasPermission) {
    return (
      <AuthGuard>
        <div className="flex h-screen bg-background">
          <Sidebar />
          <main className="flex-1 overflow-auto" />
        </div>
      </AuthGuard>
    )
  }

  return (
    <AuthGuard>
      <div className="flex h-screen bg-background">
        <Sidebar />
        <main className="flex-1 overflow-auto">
          <div className="p-8">
            {/* Header */}
            <div className="flex items-center justify-between mb-8">
              <div>
                <h1 className="text-3xl font-playfair font-bold text-foreground mb-2">日志审计</h1>
                <p className="text-muted-foreground">查看系统日志和用户操作记录</p>
              </div>
              <div className="flex items-center space-x-2">
                <Button variant="outline" onClick={exportToCSV} disabled={isLoading || currentLogs.length === 0}>
                  <Download className="h-4 w-4 mr-2" />
                  导出 CSV
                </Button>
                <Button variant="outline" onClick={exportToJSON} disabled={isLoading || currentLogs.length === 0}>
                  <Download className="h-4 w-4 mr-2" />
                  导出 JSON
                </Button>
                <Button variant="outline" onClick={loadLogs} disabled={isLoading}>
                  <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
                  刷新
                </Button>
              </div>
            </div>

            {/* Tabs */}
            <Tabs value={logType} onValueChange={(value) => setLogType(value as "operations" | "all")}>
              <TabsList>
                <TabsTrigger value="operations" className="flex items-center space-x-2">
                  <Activity className="h-4 w-4" />
                  <span>操作日志</span>
                </TabsTrigger>
                <TabsTrigger value="all" className="flex items-center space-x-2">
                  <FileText className="h-4 w-4" />
                  <span>系统日志</span>
                </TabsTrigger>
              </TabsList>

              <TabsContent value={logType} className="space-y-4">
                <Card>
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <div>
                        <CardTitle className="flex items-center space-x-2">
                          {logType === "operations" ? (
                            <>
                              <Activity className="h-5 w-5" />
                              <span>操作日志</span>
                            </>
                          ) : (
                            <>
                              <FileText className="h-5 w-5" />
                              <span>系统日志</span>
                            </>
                          )}
                        </CardTitle>
                        <CardDescription>
                          {logType === "operations" 
                            ? "查看用户操作记录和系统事件" 
                            : "查看系统日志记录"}
                        </CardDescription>
                      </div>
                    </div>
                    <div className="flex flex-col space-y-3 mt-4">
                      <div className="flex items-center space-x-2">
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
                        {logType === "all" && (
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
                        )}
                      </div>
                      <div className="flex items-center space-x-2">
                        <DateTimePicker
                          value={startTime}
                          onChange={setStartTime}
                          placeholder="开始时间"
                          maxDate={endTime ? new Date(endTime) : undefined}
                          maxTime={endTime ? format(new Date(endTime), "HH:mm") : undefined}
                        />
                        <span className="text-sm text-muted-foreground">至</span>
                        <DateTimePicker
                          value={endTime}
                          onChange={setEndTime}
                          placeholder="结束时间"
                          minDate={startTime ? new Date(startTime) : undefined}
                          minTime={startTime ? format(new Date(startTime), "HH:mm") : undefined}
                        />
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent>
                    {error ? (
                      <div className="flex items-center justify-center h-32 text-red-500">
                        <div className="text-center">
                          <p className="mb-2">{error}</p>
                          <Button variant="outline" size="sm" onClick={loadLogs}>
                            重试
                          </Button>
                        </div>
                      </div>
                    ) : isLoading ? (
                      <div className="flex items-center justify-center h-32">
                        <RefreshCw className="h-6 w-6 animate-spin mr-2" />
                        <span>加载日志中...</span>
                      </div>
                    ) : currentLogs.length === 0 ? (
                      <div className="flex flex-col items-center justify-center h-40 text-muted-foreground space-y-2 text-sm">
                        {logSearchTerm || (logType === "all" && logLevelFilter !== "all") || startTime || endTime ? (
                          <Filter className="h-8 w-8 opacity-50" />
                        ) : (
                          <FileText className="h-8 w-8 opacity-50" />
                        )}
                        <span>{logSearchTerm || (logType === "all" && logLevelFilter !== "all") || startTime || endTime ? "没有符合条件的日志" : "暂无日志数据"}</span>
                        <span className="text-xs">
                          {logSearchTerm || (logType === "all" && logLevelFilter !== "all") || startTime || endTime ? "调整筛选条件或清空搜索后重试" : "请稍后再试或刷新页面"}
                        </span>
                      </div>
                    ) : (
                      <div className="h-[600px] w-full border rounded-md bg-gray-50 dark:bg-gray-900">
                        {logType === "operations" ? (
                          <OperationLogListViewer logs={filteredOperationLogs} />
                        ) : (
                          <SystemLogListViewer logs={filteredSystemLogs} />
                        )}
                      </div>
                    )}
                  </CardContent>
                </Card>
              </TabsContent>
            </Tabs>
          </div>
        </main>
      </div>
    </AuthGuard>
  )
}

