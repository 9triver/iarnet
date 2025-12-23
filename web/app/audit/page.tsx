"use client"

import { useState, useEffect, useMemo, useRef } from "react"
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
  Shield,
  Calendar as CalendarIcon,
} from "lucide-react"
import { AutoSizer, CellMeasurer, CellMeasurerCache, List, type ListRowProps } from "react-virtualized"

const LOG_LEVEL_STYLES: Record<string, { badge: string; dot: string; label: string }> = {
  error: { badge: "bg-red-100 text-red-800", dot: "bg-red-500", label: "错误" },
  warn: { badge: "bg-amber-100 text-amber-800", dot: "bg-amber-500", label: "警告" },
  debug: { badge: "bg-blue-100 text-blue-800", dot: "bg-blue-500", label: "调试" },
  trace: { badge: "bg-slate-100 text-slate-800", dot: "bg-slate-400", label: "追踪" },
  info: { badge: "bg-emerald-100 text-emerald-800", dot: "bg-emerald-500", label: "信息" },
}

type LogEntry = {
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
  user?: string
  action?: string
  resource?: string
}

const LogListViewer = ({ logs }: { logs: LogEntry[] }) => {
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
                      {log.user && (
                        <div className="flex items-center gap-1 text-xs text-muted-foreground">
                          <User className="h-3 w-3" />
                          <span>{log.user}</span>
                        </div>
                      )}
                      {log.action && (
                        <Badge variant="outline" className="text-xs">
                          {log.action}
                        </Badge>
                      )}
                      {log.resource && (
                        <span className="text-xs text-muted-foreground font-mono">
                          {log.resource}
                        </span>
                      )}
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

  const defaultTimeRange = getDefaultTimeRange()
  const [logs, setLogs] = useState<LogEntry[]>([])
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

      // 根据当前标签页获取对应的时间范围
      const timeRange = logType === "operations" ? operationsTimeRange : allLogsTimeRange

      // 准备查询参数
      const params: any = {}
      
      // 添加时间范围参数（RFC3339 格式）
      if (timeRange.startTime) {
        params.startTime = timeRange.startTime
      }
      if (timeRange.endTime) {
        params.endTime = timeRange.endTime
      }
      
      // 如果没有时间范围，使用默认的 limit
      if (!timeRange.startTime && !timeRange.endTime) {
        params.limit = 100
      }
      
      // 根据日志类型选择不同的 API
      const response = logType === "operations"
        ? await auditAPI.getOperations({ limit: 100 })
        : await auditAPI.getLogs(params)
      
      // apiRequest 应该提取 data.data，但如果返回的是完整响应，需要访问 response.data.logs
      // 兼容两种情况：response.logs（已提取）或 response.data.logs（完整响应）
      const logs = (response as any).logs || (response as any).data?.logs || []
      
      const normalizedLogs: LogEntry[] = logs.map((log: any, index: number) => {
        if (index < 3) {
          console.log(`Processing log ${index}:`, log)
        }
        // 处理 timestamp：后端返回的是纳秒时间戳（int64），需要转换为 ISO 字符串
        let timestampStr: string | undefined = undefined
        if (log.timestamp !== undefined && log.timestamp !== null) {
          // 纳秒时间戳转换为毫秒
          const timestampMs = Number(log.timestamp) / 1000000
          timestampStr = new Date(timestampMs).toISOString()
        }
        
        // 处理 level：后端返回的是数字（0-7），需要转换为字符串
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
        
        // 处理 fields 字段
        let details: string | undefined = undefined
        if (log.fields && Array.isArray(log.fields) && log.fields.length > 0) {
          details = log.fields.map((f: any) => `${f.key}: ${f.value}`).join("\n")
        }
        
        // 处理 caller 字段
        let caller: { file?: string; line?: number; function?: string } | undefined = undefined
        if (log.caller) {
          // 完整保留函数名，不进行截取
          const file = log.caller.file && log.caller.file.trim() ? log.caller.file.trim() : undefined
          const line = log.caller.line !== undefined && log.caller.line !== 0 ? Number(log.caller.line) : undefined
          const func = log.caller.function && log.caller.function.trim() ? log.caller.function.trim() : undefined
          
          // 如果至少有一个字段有值，才创建 caller 对象
          if (file || line || func) {
            caller = {
              file,
              line,
              function: func,
            }
          }
        }
        
        return {
          id: `${log.timestamp || Date.now()}-${index}`,
          timestamp: timestampStr,
          level: levelStr,
          message: log.message || log.content || "",
          details,
          caller,
          user: log.user || log.username || log.user_name,
          action: log.action || log.operation,
          resource: log.component_id || log.resource || log.resource_id || log.resource_type,
        }
      })
      
      console.log("Normalized logs count:", normalizedLogs.length)
      if (normalizedLogs.length > 0) {
        console.log("First normalized log:", normalizedLogs[0])
      }
      setLogs(normalizedLogs)
    } catch (err: any) {
      console.error('Failed to load logs:', err)
      setError(err.message || "加载日志失败")
      setLogs([])
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    loadLogs()
  }, [logType, operationsTimeRange.startTime, operationsTimeRange.endTime, allLogsTimeRange.startTime, allLogsTimeRange.endTime])

  const filteredLogs = useMemo(() => {
    const searchTerm = logSearchTerm.trim().toLowerCase()
    const levelFilter = logLevelFilter.toLowerCase()

    return logs.filter((log) => {
      if (levelFilter !== "all" && log.level?.toLowerCase() !== levelFilter) {
        return false
      }
      if (searchTerm) {
        const content = `${log.message ?? ""} ${log.details ?? ""} ${log.user ?? ""} ${log.action ?? ""} ${log.resource ?? ""}`.toLowerCase()
        return content.includes(searchTerm)
      }
      return true
    })
  }, [logs, logLevelFilter, logSearchTerm])

  return (
    <AuthGuard>
      <div className="flex h-screen bg-gray-50">
        <Sidebar />
        <main className="flex-1 overflow-auto">
          <div className="p-8 space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-3">
                <Shield className="h-6 w-6" />
                <div>
                  <h1 className="text-2xl font-bold">日志审计</h1>
                  <p className="text-sm text-muted-foreground mt-1">
                    查看系统日志和用户操作记录
                  </p>
                </div>
              </div>
              <Button variant="outline" onClick={loadLogs} disabled={isLoading}>
                <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
                刷新
              </Button>
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
                  <span>所有日志</span>
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
                              <span>所有日志</span>
                            </>
                          )}
                        </CardTitle>
                        <CardDescription>
                          {logType === "operations" 
                            ? "查看用户操作记录和系统事件" 
                            : "查看系统所有日志记录"}
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
                    ) : filteredLogs.length === 0 ? (
                      <div className="flex flex-col items-center justify-center h-40 text-muted-foreground space-y-2 text-sm">
                        {logSearchTerm || logLevelFilter !== "all" || startTime || endTime ? (
                          <Filter className="h-8 w-8 opacity-50" />
                        ) : (
                          <FileText className="h-8 w-8 opacity-50" />
                        )}
                        <span>{logSearchTerm || logLevelFilter !== "all" || startTime || endTime ? "没有符合条件的日志" : "暂无日志数据"}</span>
                        <span className="text-xs">
                          {logSearchTerm || logLevelFilter !== "all" || startTime || endTime ? "调整筛选条件或清空搜索后重试" : "请稍后再试或刷新页面"}
                        </span>
                      </div>
                    ) : (
                      <div className="h-[600px] w-full border rounded-md bg-gray-50 dark:bg-gray-900">
                        <LogListViewer logs={filteredLogs} />
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

