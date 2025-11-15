"use client"

import { useState, useEffect } from "react"
import { Sidebar } from "@/components/sidebar"
import { resourcesAPI } from "@/lib/api"
import type {
  GetResourceCapacityResponse,
  GetResourceProvidersResponse,
  ProviderItem,
  RegisterResourceProviderRequest,
} from "@/lib/model"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { useForm } from "react-hook-form"
import { Plus, Server, Cpu, HardDrive, Activity, Trash2, Edit, RefreshCw, MemoryStick } from "lucide-react"
import { formatMemory, formatNumber } from "@/lib/utils"
import { Skeleton } from "@/components/ui/skeleton"

// 本地使用的资源类型（包含 CPU 和内存使用情况）
// 注意：后端 ProviderItem 不包含 cpu_usage 和 memory_usage
// 这些数据需要从其他地方获取或计算
interface Resource {
  id: string
  name: string
  type: "kubernetes" | "docker" | "vm"
  host: string
  port: number
  category?: "local_providers" | "remote_providers" // 资源分类：本地、远程
  status: "connected" | "disconnected" | "error"
  cpu: {
    total: number
    used: number
  }
  memory: {
    total: number
    used: number
  }
  lastUpdated: string
}


interface ResourceFormData {
  name: string
  type: "kubernetes" | "docker" | "vm"
  host: string
  port: number
  token: string
  description?: string
}

interface NodeFormData {
  name: string
  host: string
  port: number
  description?: string
}

interface Usage {
  cpu: number
  memory: number
  gpu: number
}

interface Capacity {
  total: Usage
  used: Usage
  available: Usage
}

export default function ResourcesPage() {
  const [resources, setResources] = useState<Resource[]>([])
  const [capacity, setCapacity] = useState<Capacity | null>(null)
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [isNodeDialogOpen, setIsNodeDialogOpen] = useState(false)
  const [editingResource, setEditingResource] = useState<Resource | null>(null)
  const [loading, setLoading] = useState(true)

  // 状态转换函数
  const convertStatus = (status: string | number | any): "connected" | "disconnected" | "error" => {
    // 确保status是字符串类型
    const statusStr = String(status || '').toLowerCase()
    switch (statusStr) {
      case "connected":
      case "online":
      case "active":
      case "1":
        return "connected"
      case "disconnected":
      case "offline":
      case "inactive":
      case "2":
        return "disconnected"
      default:
        return "error"
    }
  }

  const form = useForm<ResourceFormData>({
    defaultValues: {
      name: "",
      type: "kubernetes",
      host: "",
      port: 2376,
      token: "",
      description: "",
    },
  })

  // 获取资源容量数据
  const fetchCapacity = async () => {
    try {
      const data = await resourcesAPI.getCapacity()
      // 后端返回的CPU单位是毫核（millicores），需要除以1000转换为核（cores）
      // 内存单位是 bytes，需要转换为合适的单位（在显示时转换）
      const capacity: Capacity = {
        total: {
          cpu: data.total.cpu / 1000, // 转换为核
          memory: data.total.memory,    // 保持 bytes，显示时转换
          gpu: data.total.gpu,
        },
        used: {
          cpu: data.used.cpu / 1000,
          memory: data.used.memory,
          gpu: data.used.gpu,
        },
        available: {
          cpu: data.available.cpu / 1000,
          memory: data.available.memory,
          gpu: data.available.gpu,
        },
      }
      setCapacity(capacity)
    } catch (error) {
      console.error('Failed to fetch capacity:', error)
    }
  }

  const fetchData = async () => {
    try {
      setLoading(true)
      await fetchProviders()
      await fetchCapacity()
    } finally {
      setLoading(false)
    }
  }



  // 获取资源提供者数据
  const fetchProviders = async () => {
    try {
      const response = await resourcesAPI.getProviders()
      
      // 转换API数据格式为前端需要的格式
      // 注意：后端 ProviderItem 不包含 cpu_usage 和 memory_usage
      // 这些数据需要从其他地方获取，目前先设置为 0
      const convertProvider = (provider: ProviderItem): Resource => ({
        id: provider.id,
        name: provider.name,
        type: provider.type.toLowerCase() as "kubernetes" | "docker" | "vm",
        host: provider.host,
        port: provider.port,
        category: "local_providers", // 全部作为本地资源处理
        status: convertStatus(provider.status),
        cpu: {
          // TODO: 从 provider 的 GetCapacity 接口获取实际使用情况
          // 目前先设置为 0，需要后续实现
          total: 0,
          used: 0,
        },
        memory: {
          total: 0,
          used: 0,
        },
        lastUpdated: provider.last_update_time,
      })
      
      // 只处理本地资源，远程资源暂不接入数据
      const localResources = (response.providers || []).map(convertProvider)
      
      setResources(localResources)
    } catch (error) {
      console.error('Failed to fetch providers:', error)
      // 如果API调用失败，保持使用模拟数据
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  const onSubmit = async (data: ResourceFormData) => {
    if (editingResource) {
      // 编辑现有资源
      // TODO: 后端暂不支持编辑，先在前端更新
      setResources((prev) =>
        prev.map((resource) =>
          resource.id === editingResource.id
            ? { ...resource, name: data.name, type: data.type, host: data.host, port: data.port }
            : resource,
        ),
      )
    } else {
      // 添加新资源 - 调用后端API
      try {
        const request: RegisterResourceProviderRequest = {
          name: data.name,
          host: data.host,
          port: data.port,
        }
        
        const response = await resourcesAPI.registerProvider(request)
        console.log('Provider registered successfully:', response)
        
        // 重新获取资源列表以显示最新数据
        await fetchProviders()
      } catch (error) {
        console.error('Failed to register provider:', error)
        // 可以在这里添加错误提示
      }
    }

    setIsDialogOpen(false)
    setEditingResource(null)
    form.reset()
  }

  const handleEdit = (resource: Resource) => {
    setEditingResource(resource)
    form.setValue("name", resource.name)
    form.setValue("type", resource.type)
    form.setValue("host", resource.host)
    form.setValue("port", resource.port)
    setIsDialogOpen(true)
  }

  const handleDelete = async (id: string) => {
    try {
      await resourcesAPI.unregisterProvider(id)
      console.log('Provider unregistered successfully:', id)
      
      // 重新获取资源列表以显示最新数据
      await fetchProviders()
    } catch (error) {
      console.error('Failed to unregister provider:', error)
      // 可以在这里添加错误提示
    }
  }

  const getStatusBadge = (status: Resource["status"]) => {
    switch (status) {
      case "connected":
        return (
          <Badge variant="default" className="bg-green-500">
            已连接
          </Badge>
        )
      case "disconnected":
        return <Badge variant="secondary">已断开</Badge>
      case "error":
        return <Badge variant="destructive">错误</Badge>
    }
  }

  const getTypeIcon = (type: Resource["type"]) => {
    switch (type) {
      case "kubernetes":
        return <Server className="h-4 w-4" />
      case "docker":
        return <Cpu className="h-4 w-4" />
      case "vm":
        return <HardDrive className="h-4 w-4" />
    }
  }

  // 骨架行组件
  const ResourceTableSkeleton = () => (
    <TableRow>
      <TableCell className="w-64">
        <div className="flex items-center space-x-2">
          <Skeleton className="h-4 w-4 bg-gray-200 dark:bg-gray-700" />
          <div>
            <Skeleton className="h-4 w-40 mb-1 bg-gray-200 dark:bg-gray-700" />
            <Skeleton className="h-3 w-48 bg-gray-200 dark:bg-gray-700" />
          </div>
        </div>
      </TableCell>
      <TableCell className="w-20">
        <Skeleton className="h-6 w-16 bg-gray-200 dark:bg-gray-700" />
      </TableCell>
      <TableCell className="w-20">
        <Skeleton className="h-6 w-16 bg-gray-200 dark:bg-gray-700" />
      </TableCell>
      <TableCell className="w-32">
        <div className="flex items-center space-x-2">
          <Skeleton className="h-4 w-8 bg-gray-200 dark:bg-gray-700" />
          <Skeleton className="h-3 w-20 bg-gray-200 dark:bg-gray-700" />
        </div>
      </TableCell>
      <TableCell className="w-32">
        <div className="flex items-center space-x-2">
          <Skeleton className="h-4 w-8 bg-gray-200 dark:bg-gray-700" />
          <Skeleton className="h-3 w-20 bg-gray-200 dark:bg-gray-700" />
        </div>
      </TableCell>
      <TableCell className="w-40">
        <Skeleton className="h-3 w-32 bg-gray-200 dark:bg-gray-700" />
      </TableCell>
      <TableCell className="w-32">
        <div className="flex items-center space-x-2">
          <Skeleton className="h-8 w-8 bg-gray-200 dark:bg-gray-700" />
          <Skeleton className="h-8 w-8 bg-gray-200 dark:bg-gray-700" />
          <Skeleton className="h-8 w-8 bg-gray-200 dark:bg-gray-700" />
        </div>
      </TableCell>
    </TableRow>
  )

  return (
    <div className="flex h-screen bg-background">
      <Sidebar />

      <main className="flex-1 overflow-auto">
        <div className="p-8">
          {/* Header */}
          <div className="flex items-center justify-between mb-8">
            <div>
              <h1 className="text-3xl font-playfair font-bold text-foreground mb-2">算力资源管理</h1>
              <p className="text-muted-foreground">接入和管理您的算力资源，包括CPU、GPU、内存和网络资源</p>
            </div>

            <div className="flex items-center space-x-3">
              <Button
                variant="outline"
                onClick={() => {
                  fetchData()
                }}
                disabled={loading}
              >
                <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
                刷新数据
              </Button>
              
              <Dialog open={isNodeDialogOpen} onOpenChange={setIsNodeDialogOpen}>
                <DialogTrigger asChild>
                  <Button variant="outline">
                    <Plus className="mr-2 h-4 w-4" />
                    接入节点
                  </Button>
                </DialogTrigger>
                <DialogContent className="sm:max-w-[425px]">
                  <DialogHeader>
                    <DialogTitle>接入远程节点</DialogTitle>
                    <DialogDescription>连接到远程算力节点，发现并使用其资源</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4">
                    <div className="space-y-2">
                      <label className="text-sm font-medium">节点名称</label>
                      <Input placeholder="输入节点名称" />
                    </div>
                    <div className="space-y-2">
                      <label className="text-sm font-medium">主机地址</label>
                      <Input placeholder="例如: 192.168.1.200" />
                    </div>
                    <div className="space-y-2">
                      <label className="text-sm font-medium">端口</label>
                      <Input type="number" placeholder="例如: 8080" />
                    </div>
                    <div className="space-y-2">
                      <label className="text-sm font-medium">描述</label>
                      <Textarea placeholder="节点描述信息（可选）" className="min-h-[80px]" />
                    </div>
                  </div>
                  <DialogFooter>
                    <Button type="button" variant="outline" onClick={() => setIsNodeDialogOpen(false)}>
                      取消
                    </Button>
                    <Button type="button" onClick={() => setIsNodeDialogOpen(false)}>
                      接入节点
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
              
              <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
                <DialogTrigger asChild>
                  <Button>
                    <Plus className="mr-2 h-4 w-4" />
                    接入资源
                  </Button>
                </DialogTrigger>
                <DialogContent className="sm:max-w-[425px]">
                  <DialogHeader>
                    <DialogTitle>添加新资源</DialogTitle>
                    <DialogDescription>配置新的算力资源节点连接信息</DialogDescription>
                  </DialogHeader>
                  <Form {...form}>
                    <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
                      <FormField
                        control={form.control}
                        name="name"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>资源名称</FormLabel>
                            <FormControl>
                              <Input placeholder="输入资源名称" {...field} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="type"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>资源类型</FormLabel>
                            <Select onValueChange={field.onChange} defaultValue={field.value}>
                              <FormControl>
                                <SelectTrigger>
                                  <SelectValue placeholder="选择资源类型" />
                                </SelectTrigger>
                              </FormControl>
                              <SelectContent>
                                <SelectItem value="kubernetes">Kubernetes</SelectItem>
                                <SelectItem value="docker">Docker</SelectItem>
                                <SelectItem value="vm">虚拟机</SelectItem>
                              </SelectContent>
                            </Select>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="host"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>主机地址</FormLabel>
                            <FormControl>
                              <Input placeholder="例如: 192.168.1.100" {...field} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="port"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>端口</FormLabel>
                            <FormControl>
                              <Input
                                type="number"
                                placeholder="例如: 6443"
                                {...field}
                                onChange={(e) => field.onChange(parseInt(e.target.value))}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="token"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>认证信息</FormLabel>
                            <FormControl>
                              <Textarea
                                placeholder="输入认证Token或配置文件内容"
                                className="min-h-[100px]"
                                {...field}
                              />
                            </FormControl>
                            <FormDescription>
                              Kubernetes: kubeconfig文件内容；Docker: TLS证书路径或留空
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <DialogFooter>
                        <Button type="submit">接入资源</Button>
                      </DialogFooter>
                    </form>
                  </Form>
                </DialogContent>
              </Dialog>
            </div>
          </div>

          {/* Stats Cards */}
          <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">总资源数</CardTitle>
                <Server className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{resources.length}</div>
                <p className="text-xs text-muted-foreground">已接入资源节点</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">在线资源</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-green-600">
                  {resources.filter((r) => r.status === "connected").length}
                </div>
                <p className="text-xs text-muted-foreground">正常连接中</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">总CPU核心</CardTitle>
                <Cpu className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {loading ? "加载中..." : capacity ? formatNumber(capacity.total.cpu) : "--"}
                </div>
                <p className="text-xs text-muted-foreground">
                  已使用 {loading ? "--" : capacity ? formatNumber(capacity.used.cpu) : "--"} 核心
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">总内存</CardTitle>
                <MemoryStick className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {loading ? "加载中..." : capacity ? formatMemory(capacity.total.memory) : "--"}
                </div>
                <p className="text-xs text-muted-foreground">
                  已使用 {loading ? "--" : capacity ? formatMemory(capacity.used.memory) : "--"}
                </p>
              </CardContent>
            </Card>
          </div>

          {/* 本地资源面板 */}
          <Card className="mb-8">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="flex items-center space-x-2">
                    <div className="w-3 h-3 bg-blue-500 rounded-full"></div>
                    <span>本地资源</span>
                  </CardTitle>
                  <CardDescription>手动配置并托管的本地算力资源</CardDescription>
                </div>
                <div className="flex items-center space-x-4">
                  <div className="text-sm text-muted-foreground">
                    {resources.filter(r => r.category === 'local_providers').length} 个资源
                  </div>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-64">资源名称</TableHead>
                    <TableHead className="w-20">类型</TableHead>
                    <TableHead className="w-20">状态</TableHead>
                    <TableHead className="w-32">CPU使用率</TableHead>
                    <TableHead className="w-32">内存使用率</TableHead>
                    <TableHead className="w-40">最后更新</TableHead>
                    <TableHead className="w-32">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    Array.from({ length: 1 }).map((_, index) => (
                      <ResourceTableSkeleton key={`local-skeleton-${index}`} />
                    ))
                  ) : resources.filter(r => r.category === 'local_providers').length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                        暂无本地资源，点击上方"接入资源"按钮开始配置
                      </TableCell>
                    </TableRow>
                  ) : (
                    resources.filter(r => r.category === 'local_providers').map((resource) => (
                      <TableRow key={resource.id}>
                        <TableCell className="w-64">
                          <div className="flex items-center space-x-2">
                            {getTypeIcon(resource.type)}
                            <div>
                              <div className="font-medium">{resource.name}</div>
                              <div className="text-sm text-muted-foreground">{resource.host}:{resource.port}</div>
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="w-20">
                          <Badge variant="outline">
                            {resource.type === "kubernetes" ? "K8s" : resource.type === "docker" ? "Docker" : "VM"}
                          </Badge>
                        </TableCell>
                        <TableCell className="w-20">{getStatusBadge(resource.status)}</TableCell>
                        <TableCell className="w-32">
                          <div className="flex items-center space-x-2">
                            <div className="text-sm">
                              {resource.cpu.total > 0
                                ? `${Math.round((resource.cpu.used / resource.cpu.total) * 100)}%`
                                : "0%"}
                            </div>
                            <div className="text-xs text-muted-foreground">
                              {formatNumber(resource.cpu.used)}/{formatNumber(resource.cpu.total)} 核
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="w-32">
                          <div className="flex items-center space-x-2">
                            <div className="text-sm">
                              {resource.memory.total > 0
                                ? `${Math.round((resource.memory.used / resource.memory.total) * 100)}%`
                                : "0%"}
                            </div>
                            <div className="text-xs text-muted-foreground">
                              {formatMemory(resource.memory.used)}/{formatMemory(resource.memory.total)}
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="w-40 text-xs text-muted-foreground">{resource.lastUpdated}</TableCell>
                        <TableCell className="w-32">
                          <div className="flex items-center space-x-2">
                            <Button variant="ghost" size="sm" onClick={() => handleEdit(resource)}>
                              <Edit className="h-4 w-4" />
                            </Button>
                            <Button variant="ghost" size="sm" onClick={() => handleDelete(resource.id)}>
                              <Trash2 className="h-4 w-4" />
                            </Button>
                            <Button variant="ghost" size="sm">
                              <RefreshCw className="h-4 w-4" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          {/* 远程资源面板 */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="flex items-center space-x-2">
                    <div className="w-3 h-3 bg-purple-500 rounded-full"></div>
                    <span>远程资源</span>
                  </CardTitle>
                  <CardDescription>网络中自动发现的可协作算力资源</CardDescription>
                </div>
                <div className="flex items-center space-x-4">
                  <div className="text-sm text-muted-foreground">
                    {resources.filter(r => r.category === 'remote_providers').length} 个资源
                  </div>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-64">资源名称</TableHead>
                    <TableHead className="w-20">类型</TableHead>
                    <TableHead className="w-20">状态</TableHead>
                    <TableHead className="w-32">CPU使用率</TableHead>
                    <TableHead className="w-32">内存使用率</TableHead>
                    <TableHead className="w-40">最后更新</TableHead>
                    <TableHead className="w-32">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    Array.from({ length: 1 }).map((_, index) => (
                      <ResourceTableSkeleton key={`remote-skeleton-${index}`} />
                    ))
                  ) : resources.filter(r => r.category === 'remote_providers').length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                        暂无远程资源，系统将自动发现网络中节点提供的资源，点击上方"接入节点"按钮可以接入新节点到网络中
                      </TableCell>
                    </TableRow>
                  ) : (
                    resources.filter(r => r.category === 'remote_providers').map((resource) => (
                      <TableRow key={resource.id}>
                        <TableCell className="w-64">
                          <div className="flex items-center space-x-2">
                            {getTypeIcon(resource.type)}
                            <div>
                              <div className="font-medium">{resource.name}</div>
                              <div className="text-sm text-muted-foreground">{resource.host}:{resource.port}</div>
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="w-20">
                          <Badge variant="outline">
                            {resource.type === "kubernetes" ? "K8s" : resource.type === "docker" ? "Docker" : "VM"}
                          </Badge>
                        </TableCell>
                        <TableCell className="w-20">{getStatusBadge(resource.status)}</TableCell>
                        <TableCell className="w-32">
                          <div className="flex items-center space-x-2">
                            <div className="text-sm">
                              {resource.cpu.total > 0
                                ? `${Math.round((resource.cpu.used / resource.cpu.total) * 100)}%`
                                : "0%"}
                            </div>
                            <div className="text-xs text-muted-foreground">
                              {formatNumber(resource.cpu.used)}/{formatNumber(resource.cpu.total)} 核
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="w-32">
                          <div className="flex items-center space-x-2">
                            <div className="text-sm">
                              {resource.memory.total > 0
                                ? `${Math.round((resource.memory.used / resource.memory.total) * 100)}%`
                                : "0%"}
                            </div>
                            <div className="text-xs text-muted-foreground">
                              {formatMemory(resource.memory.used)}/{formatMemory(resource.memory.total)}
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="w-40 text-xs text-muted-foreground">{resource.lastUpdated}</TableCell>
                        <TableCell className="w-32">
                          <div className="flex items-center space-x-2">
                            <Button variant="ghost" size="sm" onClick={() => handleEdit(resource)}>
                              <Edit className="h-4 w-4" />
                            </Button>
                            <Button variant="ghost" size="sm" onClick={() => handleDelete(resource.id)}>
                              <Trash2 className="h-4 w-4" />
                            </Button>
                            <Button variant="ghost" size="sm">
                              <RefreshCw className="h-4 w-4" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>


        </div>
      </main>
    </div>
  )
}
