"use client"

import { useState, useEffect } from "react"
import { Sidebar } from "@/components/sidebar"
import { resourcesAPI } from "@/lib/api"
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

interface Resource {
  id: string
  name: string
  type: "kubernetes" | "docker" | "vm"
  url: string
  status: "connected" | "disconnected" | "error"
  cpu: {
    total: number
    used: number
  }
  memory: {
    total: number
    used: number
  }
  storage: {
    total: number
    used: number
  }
  lastUpdated: string
}

interface ResourceFormData {
  name: string
  type: "kubernetes" | "docker" | "vm"
  url: string
  token: string
  description?: string
}

interface Usage {
  cpu: number
  memory: number
  gpu: number
}

interface Capacity {
  total: Usage
  allocated: Usage
  available: Usage
}

// 格式化数值，保留三位小数
const formatNumber = (num: number): string => {
  return Number(num.toFixed(3)).toString()
}

export default function ResourcesPage() {
  const [resources, setResources] = useState<Resource[]>([
    {
      id: "1",
      name: "生产环境集群",
      type: "kubernetes",
      url: "https://k8s-prod.example.com",
      status: "connected",
      cpu: { total: 32, used: 18 },
      memory: { total: 128, used: 76 },
      storage: { total: 2048, used: 1024 },
      lastUpdated: "2024-01-15 14:30:00",
    },
    {
      id: "2",
      name: "开发环境",
      type: "docker",
      url: "https://docker-dev.example.com",
      status: "connected",
      cpu: { total: 16, used: 8 },
      memory: { total: 64, used: 32 },
      storage: { total: 1024, used: 256 },
      lastUpdated: "2024-01-15 14:25:00",
    },
  ])

  const [capacity, setCapacity] = useState<Capacity | null>(null)
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [editingResource, setEditingResource] = useState<Resource | null>(null)
  const [loading, setLoading] = useState(false)

  const form = useForm<ResourceFormData>({
    defaultValues: {
      name: "",
      type: "kubernetes",
      url: "",
      token: "",
      description: "",
    },
  })

  // 获取资源容量数据
  const fetchCapacity = async () => {
    try {
      setLoading(true)
      const data = await resourcesAPI.getCapacity()
      setCapacity(data as Capacity)
    } catch (error) {
      console.error('Failed to fetch capacity:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchCapacity()
  }, [])

  const onSubmit = (data: ResourceFormData) => {
    if (editingResource) {
      // 编辑现有资源
      setResources((prev) =>
        prev.map((resource) =>
          resource.id === editingResource.id
            ? { ...resource, name: data.name, type: data.type, url: data.url }
            : resource,
        ),
      )
    } else {
      // 添加新资源
      const newResource: Resource = {
        id: Date.now().toString(),
        name: data.name,
        type: data.type,
        url: data.url,
        status: "connected",
        cpu: { total: 0, used: 0 },
        memory: { total: 0, used: 0 },
        storage: { total: 0, used: 0 },
        lastUpdated: new Date().toLocaleString(),
      }
      setResources((prev) => [...prev, newResource])
    }

    setIsDialogOpen(false)
    setEditingResource(null)
    form.reset()
  }

  const handleEdit = (resource: Resource) => {
    setEditingResource(resource)
    form.setValue("name", resource.name)
    form.setValue("type", resource.type)
    form.setValue("url", resource.url)
    setIsDialogOpen(true)
  }

  const handleDelete = (id: string) => {
    setResources((prev) => prev.filter((resource) => resource.id !== id))
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

  return (
    <div className="flex h-screen bg-background">
      <Sidebar />

      <main className="flex-1 overflow-auto">
        <div className="p-8">
          {/* Header */}
          <div className="flex items-center justify-between mb-8">
            <div>
              <h1 className="text-3xl font-playfair font-bold text-foreground mb-2">算力资源管理</h1>
              <p className="text-muted-foreground">接入和管理您的算力资源，包括CPU、GPU、存储和网络资源</p>
            </div>

            <div className="flex items-center space-x-3">
              <Button
                variant="outline"
                onClick={fetchCapacity}
                disabled={loading}
              >
                <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
                刷新数据
              </Button>
              
              <Button
                variant="outline"
                onClick={() => {
                  // TODO: 实现接入新节点功能
                  console.log('接入新节点')
                }}
              >
                <Plus className="h-4 w-4" />
                接入新节点
              </Button>
              
              <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
                <DialogTrigger asChild>
                  <Button
                    onClick={() => {
                      setEditingResource(null)
                      form.reset()
                    }}
                  >
                    <Plus className="h-4 w-4" />
                    接入新资源
                  </Button>
                </DialogTrigger>
              <DialogContent className="sm:max-w-[500px]">
                <DialogHeader>
                  <DialogTitle>{editingResource ? "编辑资源" : "接入新的算力资源"}</DialogTitle>
                  <DialogDescription>
                    {editingResource ? "修改资源配置信息" : "输入算力资源的API服务器信息以接入管理"}
                  </DialogDescription>
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
                            <Input placeholder="例如：生产环境集群" {...field} />
                          </FormControl>
                          <FormDescription>为这个算力资源起一个易识别的名称</FormDescription>
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
                              <SelectItem value="kubernetes">Kubernetes 集群</SelectItem>
                              <SelectItem value="docker">Docker 环境</SelectItem>
                              <SelectItem value="vm">虚拟机</SelectItem>
                            </SelectContent>
                          </Select>
                          <FormDescription>选择算力资源的部署环境类型</FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="url"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>API Server URL</FormLabel>
                          <FormControl>
                            <Input placeholder="https://api.example.com" {...field} />
                          </FormControl>
                          <FormDescription>算力资源的API服务器地址</FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="token"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>访问令牌</FormLabel>
                          <FormControl>
                            <Input type="password" placeholder="输入访问令牌" {...field} />
                          </FormControl>
                          <FormDescription>用于访问API服务器的认证令牌</FormDescription>
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
                            <Textarea placeholder="资源描述信息..." {...field} />
                          </FormControl>
                          <FormDescription>添加关于此资源的额外描述信息</FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <DialogFooter>
                      <Button type="button" variant="outline" onClick={() => setIsDialogOpen(false)}>
                        取消
                      </Button>
                      <Button type="submit">{editingResource ? "更新资源" : "接入资源"}</Button>
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
                  已分配 {loading ? "--" : capacity ? formatNumber(capacity.allocated.cpu) : "--"} 核心
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
                  {loading ? "加载中..." : capacity ? formatNumber(capacity.total.memory) : "--"} GB
                </div>
                <p className="text-xs text-muted-foreground">
                  已分配 {loading ? "--" : capacity ? formatNumber(capacity.allocated.memory) : "--"} GB
                </p>
              </CardContent>
            </Card>
          </div>

          {/* Resources Table */}
          <Card>
            <CardHeader>
              <CardTitle>资源列表</CardTitle>
              <CardDescription>管理已接入的算力资源</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>资源名称</TableHead>
                    <TableHead>类型</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>CPU使用率</TableHead>
                    <TableHead>内存使用率</TableHead>
                    <TableHead>存储使用率</TableHead>
                    <TableHead>最后更新</TableHead>
                    <TableHead>操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {resources.map((resource) => (
                    <TableRow key={resource.id}>
                      <TableCell>
                        <div className="flex items-center space-x-2">
                          {getTypeIcon(resource.type)}
                          <div>
                            <div className="font-medium">{resource.name}</div>
                            <div className="text-xs text-muted-foreground">{resource.url}</div>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">
                          {resource.type === "kubernetes" ? "K8s" : resource.type === "docker" ? "Docker" : "VM"}
                        </Badge>
                      </TableCell>
                      <TableCell>{getStatusBadge(resource.status)}</TableCell>
                      <TableCell>
                        <div className="flex items-center space-x-2">
                          <div className="text-sm">
                            {resource.cpu.total > 0
                              ? `${Math.round((resource.cpu.used / resource.cpu.total) * 100)}%`
                              : "0%"}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            {resource.cpu.used}/{resource.cpu.total} 核心
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center space-x-2">
                          <div className="text-sm">
                            {resource.memory.total > 0
                              ? `${Math.round((resource.memory.used / resource.memory.total) * 100)}%`
                              : "0%"}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            {resource.memory.used}/{resource.memory.total} GB
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center space-x-2">
                          <div className="text-sm">
                            {resource.storage.total > 0
                              ? `${Math.round((resource.storage.used / resource.storage.total) * 100)}%`
                              : "0%"}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            {resource.storage.used}/{resource.storage.total} GB
                          </div>
                        </div>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">{resource.lastUpdated}</TableCell>
                      <TableCell>
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
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      </main>
    </div>
  )
}
