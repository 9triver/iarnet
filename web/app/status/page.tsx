"use client"

import { useState, useEffect, useCallback, useMemo } from "react"
import { Sidebar } from "@/components/sidebar"
import { AuthGuard } from "@/components/auth-guard"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import {
  Activity,
  Cpu,
  MemoryStick,
  Zap,
  RefreshCw,
  TrendingUp,
  Server,
  BarChart3,
} from "lucide-react"
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, AreaChart, Area } from "recharts"
import { resourcesAPI } from "@/lib/api"
import type { GetResourceCapacityResponse, GetResourceProviderCapacityResponse, GetResourceProviderUsageResponse, ProviderItem } from "@/lib/model"
import { formatMemory } from "@/lib/utils"

// 资源使用率数据点
interface ResourceDataPoint {
  timestamp: string
  cpu: number      // CPU 使用率 (%)
  memory: number   // 内存使用率 (%)
  gpu: number      // GPU 使用率 (%)
  cpuUsed: number  // CPU 已使用 (millicores)
  memoryUsed: number // 内存已使用 (bytes)
  gpuUsed: number  // GPU 已使用 (数量)
  cpuTotal: number // CPU 总量 (millicores)
  memoryTotal: number // 内存总量 (bytes)
  gpuTotal: number // GPU 总量 (数量)
}

// Provider 资源历史数据
interface ProviderResourceHistory {
  providerId: string
  providerName: string
  data: ResourceDataPoint[]
}

// 计算资源使用率
function calculateUsageRate(used: number, total: number): number {
  if (total === 0) return 0
  return Math.round((used / total) * 100 * 1000) / 1000 // 保留三位小数
}

// 格式化时间戳
function formatTime(timestamp: Date): string {
  return timestamp.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export default function StatusPage() {
  const [aggregatedData, setAggregatedData] = useState<ResourceDataPoint[]>([])
  const [providers, setProviders] = useState<ProviderItem[]>([])
  const [providerHistories, setProviderHistories] = useState<Map<string, ProviderResourceHistory>>(new Map())
  const [selectedProvider, setSelectedProvider] = useState<string>("all") // "all" 表示聚合视图
  const [isLoading, setIsLoading] = useState(false)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [refreshInterval, setRefreshInterval] = useState(5000) // 默认 5 秒
  const [maxDataPoints, setMaxDataPoints] = useState(60) // 最多保留 60 个数据点（5分钟，5秒间隔）

  // 加载 provider 列表
  const loadProviders = useCallback(async () => {
    try {
      const response = await resourcesAPI.getProviders()
      setProviders(response.providers || [])
    } catch (error) {
      console.error('Failed to load providers:', error)
    }
  }, [])

  // 加载聚合资源数据
  const loadAggregatedCapacity = useCallback(async () => {
    try {
      const response: GetResourceCapacityResponse = await resourcesAPI.getCapacity()
      const now = new Date()
      
      const dataPoint: ResourceDataPoint = {
        timestamp: formatTime(now),
        cpu: calculateUsageRate(response.used.cpu, response.total.cpu),
        memory: calculateUsageRate(response.used.memory, response.total.memory),
        gpu: calculateUsageRate(response.used.gpu, response.total.gpu),
        cpuUsed: response.used.cpu,
        memoryUsed: response.used.memory,
        gpuUsed: response.used.gpu,
        cpuTotal: response.total.cpu,
        memoryTotal: response.total.memory,
        gpuTotal: response.total.gpu,
      }

      setAggregatedData(prev => {
        const newData = [...prev, dataPoint]
        return newData.slice(-maxDataPoints) // 只保留最近的数据点
      })
    } catch (error) {
      console.error('Failed to load aggregated capacity:', error)
    }
  }, [maxDataPoints])

  // 加载单个 provider 的资源数据（使用实时使用量）
  const loadProviderCapacity = useCallback(async (providerId: string, providerName: string) => {
    try {
      // 同时获取容量（总量）和实时使用量
      const [capacityResponse, usageResponse]: [GetResourceProviderCapacityResponse, GetResourceProviderUsageResponse] = await Promise.all([
        resourcesAPI.getProviderCapacity(providerId),
        resourcesAPI.getProviderUsage(providerId),
      ])
      const now = new Date()
      
      // 使用实时使用量（usageResponse.usage）和总量（capacityResponse.total）来计算使用率
      const dataPoint: ResourceDataPoint = {
        timestamp: formatTime(now),
        cpu: calculateUsageRate(usageResponse.usage.cpu, capacityResponse.total.cpu),
        memory: calculateUsageRate(usageResponse.usage.memory, capacityResponse.total.memory),
        gpu: calculateUsageRate(usageResponse.usage.gpu, capacityResponse.total.gpu),
        cpuUsed: usageResponse.usage.cpu,      // 实时使用量
        memoryUsed: usageResponse.usage.memory, // 实时使用量
        gpuUsed: usageResponse.usage.gpu,      // 实时使用量
        cpuTotal: capacityResponse.total.cpu,
        memoryTotal: capacityResponse.total.memory,
        gpuTotal: capacityResponse.total.gpu,
      }

      setProviderHistories(prev => {
        const newMap = new Map(prev)
        const history = newMap.get(providerId) || {
          providerId,
          providerName,
          data: [],
        }
        history.data = [...history.data, dataPoint].slice(-maxDataPoints)
        newMap.set(providerId, history)
        return newMap
      })
    } catch (error) {
      console.error(`Failed to load capacity/usage for provider ${providerId}:`, error)
    }
  }, [maxDataPoints])

  // 加载所有数据
  const loadAllData = useCallback(async () => {
    setIsLoading(true)
    try {
      await loadAggregatedCapacity()
      
      // 只加载已连接的 provider
      const connectedProviders = providers.filter(p => p.status === 'connected')
      await Promise.all(
        connectedProviders.map(p => loadProviderCapacity(p.id, p.name))
      )
    } finally {
      setIsLoading(false)
    }
  }, [loadAggregatedCapacity, loadProviderCapacity, providers])

  // 初始化：加载 provider 列表
  useEffect(() => {
    loadProviders()
  }, [loadProviders])

  // 当 provider 列表变化时，重新加载数据
  useEffect(() => {
    if (providers.length > 0) {
      loadAllData()
    }
  }, [providers.length]) // 只在 provider 数量变化时触发

  // 自动刷新
  useEffect(() => {
    if (!autoRefresh) return

    const interval = setInterval(() => {
      loadAllData()
    }, refreshInterval)

    return () => clearInterval(interval)
  }, [autoRefresh, refreshInterval, loadAllData])

  // 获取当前显示的数据
  const currentData = useMemo(() => {
    if (selectedProvider === "all") {
      return aggregatedData
    }
    const history = providerHistories.get(selectedProvider)
    return history?.data || []
  }, [selectedProvider, aggregatedData, providerHistories])

  // 获取当前资源统计
  const currentStats = useMemo(() => {
    if (currentData.length === 0) {
      return {
        cpu: { current: 0, avg: 0, max: 0 },
        memory: { current: 0, avg: 0, max: 0 },
        gpu: { current: 0, avg: 0, max: 0 },
      }
    }

    const latest = currentData[currentData.length - 1]
    const cpuValues = currentData.map(d => d.cpu)
    const memoryValues = currentData.map(d => d.memory)
    const gpuValues = currentData.map(d => d.gpu)

    return {
      cpu: {
        current: latest.cpu,
        avg: Math.round((cpuValues.reduce((a, b) => a + b, 0) / cpuValues.length) * 1000) / 1000,
        max: Math.max(...cpuValues),
      },
      memory: {
        current: latest.memory,
        avg: Math.round((memoryValues.reduce((a, b) => a + b, 0) / memoryValues.length) * 1000) / 1000,
        max: Math.max(...memoryValues),
      },
      gpu: {
        current: latest.gpu,
        avg: Math.round((gpuValues.reduce((a, b) => a + b, 0) / gpuValues.length) * 1000) / 1000,
        max: Math.max(...gpuValues),
      },
    }
  }, [currentData])

  // 获取当前资源详情（用于显示实际数值）
  const currentResourceDetails = useMemo(() => {
    if (currentData.length === 0) {
      return null
    }
    const latest = currentData[currentData.length - 1]
    return {
      cpu: { used: latest.cpuUsed, total: latest.cpuTotal },
      memory: { used: latest.memoryUsed, total: latest.memoryTotal },
      gpu: { used: latest.gpuUsed, total: latest.gpuTotal },
    }
  }, [currentData])

  const connectedProviders = providers.filter(p => p.status === 'connected')

  return (
    <div className="flex h-screen bg-background">
      <Sidebar />

      <main className="flex-1 overflow-auto">
        <div className="p-8">
          {/* Header */}
          <div className="flex items-center justify-between mb-8">
            <div>
              <h1 className="text-3xl font-playfair font-bold text-foreground mb-2">状态监控</h1>
              <p className="text-muted-foreground">实时监控所有本地资源的 CPU、内存、GPU 资源使用情况</p>
            </div>

            <div className="flex items-center space-x-2">
              <Select value={refreshInterval.toString()} onValueChange={(v) => setRefreshInterval(Number(v))}>
                <SelectTrigger className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="2000">2 秒</SelectItem>
                  <SelectItem value="5000">5 秒</SelectItem>
                  <SelectItem value="10000">10 秒</SelectItem>
                  <SelectItem value="30000">30 秒</SelectItem>
                </SelectContent>
              </Select>
              <Button
                variant="outline"
                size="sm"
                onClick={() => loadAllData()}
                disabled={isLoading}
              >
                <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? "animate-spin" : ""}`} />
                刷新
              </Button>
              <Button
                variant={autoRefresh ? "default" : "outline"}
                size="sm"
                onClick={() => setAutoRefresh(!autoRefresh)}
              >
                <Activity className="h-4 w-4 mr-2" />
                {autoRefresh ? "自动刷新中" : "手动刷新"}
              </Button>
            </div>
          </div>

          {/* Provider 选择 */}
          <Card className="mb-6">
            <CardHeader>
              <CardTitle>选择监控视图</CardTitle>
              <CardDescription>查看聚合视图或单个本地资源的详细数据</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex items-center space-x-4">
                <span className="text-sm font-medium">监控对象：</span>
                <Select value={selectedProvider} onValueChange={setSelectedProvider}>
                  <SelectTrigger className="w-64">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">
                      <div className="flex items-center space-x-2">
                        <Server className="h-4 w-4" />
                        <span>所有本地资源（聚合）</span>
                      </div>
                    </SelectItem>
                    {connectedProviders.map((provider) => (
                      <SelectItem key={provider.id} value={provider.id}>
                        <div className="flex items-center space-x-2">
                          <div className={`w-2 h-2 rounded-full ${provider.status === 'connected' ? 'bg-green-500' : 'bg-gray-400'}`} />
                          <span>{provider.name} ({provider.host}:{provider.port})</span>
                        </div>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {selectedProvider !== "all" && (
                  <Badge variant="outline">
                    {providerHistories.get(selectedProvider)?.data.length || 0} 个数据点
                  </Badge>
                )}
              </div>
            </CardContent>
          </Card>

          {/* 资源统计卡片 */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-6">
            {/* CPU 统计 */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">CPU 使用率</CardTitle>
                <Cpu className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{currentStats.cpu.current.toFixed(3)}%</div>
                <div className="flex items-center space-x-4 mt-2 text-xs text-muted-foreground">
                  <span>平均: {currentStats.cpu.avg.toFixed(3)}%</span>
                  <span>峰值: {currentStats.cpu.max.toFixed(3)}%</span>
                </div>
                {currentResourceDetails && (
                  <div className="mt-2 text-xs text-muted-foreground">
                    {currentResourceDetails.cpu.used / 1000} / {currentResourceDetails.cpu.total / 1000} 核
                  </div>
                )}
              </CardContent>
            </Card>

            {/* 内存统计 */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">内存使用率</CardTitle>
                <MemoryStick className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{currentStats.memory.current.toFixed(3)}%</div>
                <div className="flex items-center space-x-4 mt-2 text-xs text-muted-foreground">
                  <span>平均: {currentStats.memory.avg.toFixed(3)}%</span>
                  <span>峰值: {currentStats.memory.max.toFixed(3)}%</span>
                </div>
                {currentResourceDetails && (
                  <div className="mt-2 text-xs text-muted-foreground">
                    {formatMemory(currentResourceDetails.memory.used)} / {formatMemory(currentResourceDetails.memory.total)}
                  </div>
                )}
              </CardContent>
            </Card>

            {/* GPU 统计 */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">GPU 使用率</CardTitle>
                <Zap className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{currentStats.gpu.current.toFixed(3)}%</div>
                <div className="flex items-center space-x-4 mt-2 text-xs text-muted-foreground">
                  <span>平均: {currentStats.gpu.avg.toFixed(3)}%</span>
                  <span>峰值: {currentStats.gpu.max.toFixed(3)}%</span>
                </div>
                {currentResourceDetails && (
                  <div className="mt-2 text-xs text-muted-foreground">
                    {currentResourceDetails.gpu.used} / {currentResourceDetails.gpu.total} 个
                  </div>
                )}
              </CardContent>
            </Card>
          </div>

          {/* 资源使用曲线图 */}
          <Tabs defaultValue="combined" className="space-y-4">
            <TabsList>
              <TabsTrigger value="combined">综合视图</TabsTrigger>
              <TabsTrigger value="cpu">CPU</TabsTrigger>
              <TabsTrigger value="memory">内存</TabsTrigger>
              <TabsTrigger value="gpu">GPU</TabsTrigger>
            </TabsList>

            <TabsContent value="combined" className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>资源使用趋势（综合）</CardTitle>
                  <CardDescription>CPU、内存、GPU 使用率随时间变化</CardDescription>
                </CardHeader>
                <CardContent>
                  {currentData.length === 0 ? (
                    <div className="h-[400px] flex items-center justify-center text-muted-foreground">
                      {isLoading ? "加载中..." : "暂无数据"}
                    </div>
                  ) : (
                    <ResponsiveContainer width="100%" height={400}>
                      <AreaChart data={currentData}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="timestamp" />
                        <YAxis domain={[0, 100]} />
                        <Tooltip />
                        <Legend />
                        <Area
                          type="monotone"
                          dataKey="cpu"
                          stackId="1"
                          stroke="#8884d8"
                          fill="#8884d8"
                          fillOpacity={0.6}
                          name="CPU (%)"
                        />
                        <Area
                          type="monotone"
                          dataKey="memory"
                          stackId="1"
                          stroke="#82ca9d"
                          fill="#82ca9d"
                          fillOpacity={0.6}
                          name="内存 (%)"
                        />
                        <Area
                          type="monotone"
                          dataKey="gpu"
                          stackId="1"
                          stroke="#ffc658"
                          fill="#ffc658"
                          fillOpacity={0.6}
                          name="GPU (%)"
                        />
                      </AreaChart>
                    </ResponsiveContainer>
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="cpu" className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>CPU 使用率趋势</CardTitle>
                  <CardDescription>CPU 使用率随时间变化</CardDescription>
                </CardHeader>
                <CardContent>
                  {currentData.length === 0 ? (
                    <div className="h-[400px] flex items-center justify-center text-muted-foreground">
                      {isLoading ? "加载中..." : "暂无数据"}
                    </div>
                  ) : (
                    <ResponsiveContainer width="100%" height={400}>
                      <LineChart data={currentData}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="timestamp" />
                        <YAxis domain={[0, 100]} />
                        <Tooltip />
                        <Legend />
                        <Line
                          type="monotone"
                          dataKey="cpu"
                          stroke="#8884d8"
                          strokeWidth={2}
                          dot={{ r: 4 }}
                          activeDot={{ r: 6 }}
                          name="CPU 使用率 (%)"
                        />
                      </LineChart>
                    </ResponsiveContainer>
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="memory" className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>内存使用率趋势</CardTitle>
                  <CardDescription>内存使用率随时间变化</CardDescription>
                </CardHeader>
                <CardContent>
                  {currentData.length === 0 ? (
                    <div className="h-[400px] flex items-center justify-center text-muted-foreground">
                      {isLoading ? "加载中..." : "暂无数据"}
                    </div>
                  ) : (
                    <ResponsiveContainer width="100%" height={400}>
                      <LineChart data={currentData}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="timestamp" />
                        <YAxis domain={[0, 100]} />
                        <Tooltip />
                        <Legend />
                        <Line
                          type="monotone"
                          dataKey="memory"
                          stroke="#82ca9d"
                          strokeWidth={2}
                          dot={{ r: 4 }}
                          activeDot={{ r: 6 }}
                          name="内存使用率 (%)"
                        />
                      </LineChart>
                    </ResponsiveContainer>
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="gpu" className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>GPU 使用率趋势</CardTitle>
                  <CardDescription>GPU 使用率随时间变化</CardDescription>
                </CardHeader>
                <CardContent>
                  {currentData.length === 0 ? (
                    <div className="h-[400px] flex items-center justify-center text-muted-foreground">
                      {isLoading ? "加载中..." : "暂无数据"}
                    </div>
                  ) : (
                    <ResponsiveContainer width="100%" height={400}>
                      <LineChart data={currentData}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="timestamp" />
                        <YAxis domain={[0, 100]} />
                        <Tooltip />
                        <Legend />
                        <Line
                          type="monotone"
                          dataKey="gpu"
                          stroke="#ffc658"
                          strokeWidth={2}
                          dot={{ r: 4 }}
                          activeDot={{ r: 6 }}
                          name="GPU 使用率 (%)"
                        />
                      </LineChart>
                    </ResponsiveContainer>
                  )}
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>
      </main>
    </div>
  )
}
