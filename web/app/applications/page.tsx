"use client"

import { useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import { applicationsAPI } from "@/lib/api"
import { useToast } from "@/hooks/use-toast"
import { Sidebar } from "@/components/sidebar"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Progress } from "@/components/ui/progress"
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
import {
  Plus,
  Package,
  Play,
  Square,
  GitBranch,
  Clock,
  Settings,
  Trash2,
  ExternalLink,
  Activity,
  Cpu,
  Database,
  RefreshCw,
} from "lucide-react"

interface ApplicationFormData {
  name: string
  gitUrl?: string
  branch?: string
  type: "web" | "api" | "worker" | "database"
  description?: string
  ports?: string
  healthCheck?: string
  executeCmd: string
}

interface ApplicationStats {
  total: number
  running: number
  stopped: number
  undeployed: number
  failed: number
  unknown: number
}

export default function ApplicationsPage() {
  const router = useRouter()
  const { toast } = useToast()
  const [stats, setStats] = useState<ApplicationStats>({
    total: 0,
    running: 0,
    stopped: 0,
    undeployed: 0,
    failed: 0,
    unknown: 0,
  })
  const [isLoadingStats, setIsLoadingStats] = useState(true)
  const [applications, setApplications] = useState<Application[]>([
    // {
    //   id: "1",
    //   name: "用户管理系统",
    //   description: "基于React和Node.js的用户管理后台系统",
    
    //   gitUrl: "https://github.com/company/user-management",
    //   branch: "main",
    //   status: "running",
    //   type: "web",
    //   lastDeployed: "2024-01-15 14:30:00",
    //   runningOn: ["生产环境集群"],
    //   ports: [3000],
    //   healthCheck: "/health",
    // },
    // {
    //   id: "2",
    //   name: "数据处理服务",
    //   description: "Python数据处理和分析服务",
    
    //   gitUrl: "https://github.com/company/data-processor",
    //   branch: "develop",
    //   status: "idle",
    //   type: "worker",
    //   lastDeployed: "2024-01-14 10:15:00",
    //   ports: [8080],
    // },
    // {
    //   id: "3",
    //   name: "API网关",
    //   description: "微服务API网关和路由服务",
    
    //   gitUrl: "https://github.com/company/api-gateway",
    //   branch: "main",
    //   status: "running",
    //   type: "api",
    //   lastDeployed: "2024-01-15 09:45:00",
    //   runningOn: ["生产环境集群", "开发环境"],
    //   ports: [8000],
    //   healthCheck: "/api/health",
    // },
    // {
    //   id: "4",
    //   name: "Nginx代理服务",
    //   description: "基于Docker的Nginx反向代理服务",
    
    //   status: "running",
    //   type: "web",
    //   lastDeployed: "2024-01-15 16:20:00",
    //   runningOn: ["生产环境集群"],
    //   ports: [80, 31],
    //   healthCheck: "/",
    // },
  ])

  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [editingApp, setEditingApp] = useState<Application | null>(null)
  const [isImporting, setIsImporting] = useState(false)
  const [importProgress, setImportProgress] = useState(0)


  // 获取应用统计数据
  const fetchStats = async () => {
    try {
      setIsLoadingStats(true)
      const statsData = await applicationsAPI.getStats() as ApplicationStats
      setStats(statsData)
    } catch (error) {
      console.error('获取应用统计数据失败:', error)
    } finally {
      setIsLoadingStats(false)
    }
  }

  const fetchApplications = async () => {
    try {
      const updatedApps = await applicationsAPI.getAll()
      setApplications(updatedApps.applications)
    } catch (fetchError) {
      console.error('Failed to fetch updated applications:', fetchError)
    }
  }

  useEffect(() => {
    fetchStats()
    fetchApplications()
  }, [])

  // 刷新数据
  const handleRefreshData = () => {
    fetchStats()
    fetchApplications()
  }

  const form = useForm<ApplicationFormData>({
    defaultValues: {
      name: "",
      gitUrl: "",
      branch: "main",
      type: "web",
      description: "",
      ports: "3000",
      healthCheck: "",
      executeCmd: "",
    },
  })

  const isValidGitUrl = (url: string): boolean => {
    const gitUrlPattern = /^https?:\/\/(github\.com|gitlab\.com|bitbucket\.org)\/[\w\-.]+\/[\w\-.]+(?:\.git)?$/i
    return (
      gitUrlPattern.test(url) || /^git@(github\.com|gitlab\.com|bitbucket\.org):[\w\-.]+\/[\w\-.]+\.git$/i.test(url)
    )
  }

  const onSubmit = async (data: ApplicationFormData) => {
    // 解析端口字符串为数字数组
    const parsePorts = (portsStr?: string): number[] => {
      if (!portsStr) return []
      return portsStr
        .split(',')
        .map(port => parseInt(port.trim()))
        .filter(port => !isNaN(port) && port > 0 && port <= 65535)
    }

    const ports = parsePorts(data.ports)

    if (editingApp) {
      // 编辑现有应用 - 调用后端API
      try {
        const updateData = {
          name: data.name,
          type: data.type,
          description: data.description || "",
          ports: ports,
          healthCheck: data.healthCheck,
          executeCmd: data.executeCmd,
        }

        if (await applicationsAPI.update(editingApp.id, updateData)) {
          // 更新成功后，重新获取所有应用数据
          handleRefreshData()
          toast({
            title: "更新成功",
            description: `应用 "${data.name}" 已成功更新`,
            variant: "default",
          })
        } else {
          toast({
            title: "更新失败",
            description: "应用更新失败，请稍后重试",
            variant: "destructive",
          })
          return
        }
      } catch (error) {
        console.error('Failed to update application:', error)
        toast({
          title: "更新失败",
          description: "应用更新时发生错误，请稍后重试",
          variant: "destructive",
        })
        return
      }
    } else {
      // 创建新应用 - 调用后端API
      try {
        const createData = {
          name: data.name,
          gitUrl: data.gitUrl,
          branch: data.branch,
          type: data.type,
          description: data.description || "",
          ports: ports,
          healthCheck: data.healthCheck,
          executeCmd: data.executeCmd,
        }

        if (await applicationsAPI.create(createData)) { // TODO: fix
          // 创建成功后，重新获取所有应用数据
          handleRefreshData()
          toast({
            title: "创建成功",
            description: `应用 "${data.name}" 已成功创建`,
            variant: "default",
          })
        } else {
          toast({
            title: "创建失败",
            description: "应用创建失败，请稍后重试",
            variant: "destructive",
          })
        }
      } catch (error) {
        console.error('Failed to create application:', error)
        toast({
          title: "创建失败",
          description: "应用创建时发生错误，请稍后重试",
          variant: "destructive",
        })
        return
      }
    }

    setIsDialogOpen(false)
    setEditingApp(null)
    form.reset()
  }

  const handleEdit = (app: Application) => {
    setEditingApp(app)
    form.setValue("name", app.name)
    form.setValue("gitUrl", app.gitUrl || "")
    form.setValue("branch", app.branch || "main")
    form.setValue("type", app.type)
    form.setValue("description", app.description)
    form.setValue("ports", app.ports ? app.ports.join(", ") : "")
    form.setValue("healthCheck", app.healthCheck)
    form.setValue("executeCmd", app.executeCmd || "")
    setIsDialogOpen(true)
  }

  const handleDelete = async (id: string) => {
    try {
      // 调用后端API删除应用
      await applicationsAPI.delete(id)
      // 删除成功后，重新获取应用列表
      handleRefreshData()
    } catch (error) {
      console.error('Failed to delete application:', error)
      // 可以在这里添加错误提示
    }
  }

  const handleRun = (id: string) => {
    setApplications((prev) => prev.map((app) => (app.id === id ? { ...app, status: "deploying" } : app)))

    setTimeout(() => {
      setApplications((prev) =>
        prev.map((app) =>
          app.id === id
            ? {
              ...app,
              status: "running",
              lastDeployed: new Date().toLocaleString(),
              runningOn: ["生产环境集群"],
            }
            : app,
        ),
      )
    }, 3000)
  }

  const handleStop = (id: string) => {
    setApplications((prev) =>
      prev.map((app) => (app.id === id ? { ...app, status: "stopped", runningOn: undefined } : app)),
    )
  }

  const getStatusBadge = (status: Application["status"]) => {
    switch (status) {
      case "running":
        return (
          <Badge variant="default" className="bg-green-500">
            运行中
          </Badge>
        )
      case "idle":
        return <Badge variant="secondary">未部署</Badge>
      case "stopped":
        return <Badge variant="outline">已停止</Badge>
      case "error":
        return <Badge variant="destructive">错误</Badge>
      case "deploying":
        return (
          <Badge variant="default" className="bg-blue-500">
            部署中
          </Badge>
        )
    }
  }

  const getTypeIcon = (type: Application["type"]) => {
    switch (type) {
      case "web":
        return <Package className="h-4 w-4" />
      case "api":
        return <Activity className="h-4 w-4" />
      case "worker":
        return <Cpu className="h-4 w-4" />
      case "database":
        return <Database className="h-4 w-4" />
    }
  }

  const getTypeLabel = (type: Application["type"]) => {
    switch (type) {
      case "web":
        return "Web应用"
      case "api":
        return "API服务"
      case "worker":
        return "后台任务"
      case "database":
        return "数据库"
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
              <h1 className="text-3xl font-playfair font-bold text-foreground mb-2">应用管理</h1>
              <p className="text-muted-foreground">从Git仓库导入应用，并在算力资源上部署运行</p>
            </div>

            <div className="flex space-x-2">
              <Button
                variant="outline"
                onClick={handleRefreshData}
                disabled={isLoadingStats}
              >
                <RefreshCw className={`h-4 w-4 ${isLoadingStats ? 'animate-spin' : ''}`} />
                刷新数据
              </Button>

              <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
                <DialogTrigger asChild>
                  <Button
                    onClick={() => {
                      setEditingApp(null)
                      form.reset()
                    }}
                  >
                    <Plus className="h-4 w-4" />
                    导入应用
                  </Button>
                </DialogTrigger>
                <DialogContent className="sm:max-w-[550px]">
                  <DialogHeader>
                    <DialogTitle>{editingApp ? "编辑应用" : "导入新应用"}</DialogTitle>
                    <DialogDescription>
                      {editingApp ? "修改应用配置信息" : "从Git仓库导入应用到IARNet平台"}
                    </DialogDescription>
                  </DialogHeader>

                  <Form {...form}>
                    <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
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
                        name="gitUrl"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>Git仓库URL</FormLabel>
                            <FormControl>
                              <Input 
                                placeholder="https://github.com/username/repo" 
                                disabled={!!editingApp}
                                {...field} 
                              />
                            </FormControl>
                            <FormDescription>
                              {editingApp ? "Git仓库地址不支持修改" : "应用的Git仓库地址"}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <div className="grid grid-cols-2 gap-4">
                        <FormField
                          control={form.control}
                          name="branch"
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>分支</FormLabel>
                              <FormControl>
                                <Input 
                                  placeholder="main" 
                                  disabled={!!editingApp}
                                  {...field} 
                                />
                              </FormControl>
                              <FormDescription>
                                {editingApp ? "分支不支持修改" : "要部署的Git分支"}
                              </FormDescription>
                              <FormMessage />
                            </FormItem>
                          )}
                        />

                        <FormField
                          control={form.control}
                          name="type"
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>应用类型</FormLabel>
                              <Select onValueChange={field.onChange} defaultValue={field.value}>
                                <FormControl>
                                  <SelectTrigger>
                                    <SelectValue placeholder="选择应用类型" />
                                  </SelectTrigger>
                                </FormControl>
                                <SelectContent>
                                  <SelectItem value="web">Web应用</SelectItem>
                                  <SelectItem value="api">API服务</SelectItem>
                                  <SelectItem value="worker">后台任务</SelectItem>
                                  <SelectItem value="database">数据库</SelectItem>
                                </SelectContent>
                              </Select>
                              <FormDescription>应用的类型</FormDescription>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                      </div>

                      <div className="grid grid-cols-2 gap-4">
                        <FormField
                          control={form.control}
                          name="ports"
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>端口号</FormLabel>
                              <FormControl>
                                <Input
                                  placeholder="3000, 8080, 9000"
                                  {...field}
                                />
                              </FormControl>
                              <FormDescription>应用运行端口，多个端口用逗号分隔</FormDescription>
                              <FormMessage />
                            </FormItem>
                          )}
                        />

                        <FormField
                          control={form.control}
                          name="healthCheck"
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>健康检查路径</FormLabel>
                              <FormControl>
                                <Input placeholder="/health" {...field} />
                              </FormControl>
                              <FormDescription>健康检查端点</FormDescription>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                      </div>

                      <FormField
                        control={form.control}
                        name="executeCmd"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>执行命令</FormLabel>
                            <FormControl>
                              <Input placeholder="npm start, python app.py, ./start.sh" {...field} />
                            </FormControl>
                            <FormDescription>应用启动时执行的命令</FormDescription>
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
                        <Button type="button" variant="outline" onClick={() => setIsDialogOpen(false)}>
                          取消
                        </Button>
                        <Button type="submit">{editingApp ? "更新应用" : "导入应用"}</Button>
                      </DialogFooter>
                    </form>
                  </Form>
                </DialogContent>
              </Dialog>
            </div>
          </div>

          {isImporting && (
            <div className="mb-8">
              <Card>
                <CardContent className="pt-6">
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-medium">正在导入应用...</span>
                      <span className="text-sm text-muted-foreground">{importProgress}%</span>
                    </div>
                    <Progress value={importProgress} className="w-full" />
                    <p className="text-xs text-muted-foreground">正在从Git仓库获取应用信息并进行初始化配置</p>
                  </div>
                </CardContent>
              </Card>
            </div>
          )}

          {/* Stats Cards */}
          <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">总应用数</CardTitle>
                <Package className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {isLoadingStats ? "..." : stats.total}
                </div>
                <p className="text-xs text-muted-foreground">已导入应用</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">运行中</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-green-600">
                  {isLoadingStats ? "..." : stats.running}
                </div>
                <p className="text-xs text-muted-foreground">正在运行</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">未部署</CardTitle>
                <Clock className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-orange-600">
                  {isLoadingStats ? "..." : stats.undeployed}
                </div>
                <p className="text-xs text-muted-foreground">等待部署</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">已停止</CardTitle>
                <Square className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-gray-600">
                  {isLoadingStats ? "..." : stats.stopped}
                </div>
                <p className="text-xs text-muted-foreground">已停止运行</p>
              </CardContent>
            </Card>
          </div>

          {/* Applications Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {applications.map((app) => (
              <Card 
                key={app.id} 
                className="hover:shadow-lg transition-shadow cursor-pointer"
                onClick={() => router.push(`/applications/${app.id}`)}
              >
                <CardHeader>
                  <div className="flex items-start justify-between">
                    <div className="flex items-center space-x-2">
                      {getTypeIcon(app.type)}
                      <div>
                        <CardTitle className="text-lg">{app.name}</CardTitle>
                        <CardDescription className="flex items-center space-x-2 mt-1">
                          <Badge variant="outline">{getTypeLabel(app.type)}</Badge>
                          {getStatusBadge(app.status)}
                        </CardDescription>
                      </div>
                    </div>
                  </div>
                </CardHeader>

                <CardContent className="space-y-4">
                  <p className="text-sm text-muted-foreground line-clamp-2">{app.description}</p>

                  <div className="space-y-2">
                    {app.gitUrl && (
                      <div className="flex items-center space-x-2 text-sm">
                        <Package className="h-4 w-4 text-muted-foreground" />
                        <span className="text-muted-foreground">仓库:</span>
                        <span className="font-mono text-xs truncate flex-1">{app.gitUrl}</span>
                      </div>
                    )}
                    {app.branch && (
                      <div className="flex items-center space-x-2 text-sm">
                        <GitBranch className="h-4 w-4 text-muted-foreground" />
                        <span className="text-muted-foreground">分支:</span>
                        <span className="font-mono">{app.branch}</span>
                      </div>
                    )}

                    {app.ports && app.ports.length > 0 && (
                      <div className="flex items-center space-x-2 text-sm">
                        <Activity className="h-4 w-4 text-muted-foreground" />
                        <span className="text-muted-foreground">端口:</span>
                        <div className="flex flex-wrap gap-1">
                          {app.ports.map((port, index) => (
                            <Badge key={index} variant="outline" className="text-xs font-mono">
                              {port}
                            </Badge>
                          ))}
                        </div>
                      </div>
                    )}

                    {app.lastDeployed && (
                      <div className="flex items-center space-x-2 text-sm">
                        <Clock className="h-4 w-4 text-muted-foreground" />
                        <span className="text-muted-foreground">最后部署:</span>
                        <span className="text-xs">{app.lastDeployed}</span>
                      </div>
                    )}

                    {app.runningOn && app.runningOn.length > 0 && (
                      <div className="space-y-1">
                        <div className="text-sm text-muted-foreground">运行在:</div>
                        <div className="flex flex-wrap gap-1">
                          {app.runningOn.map((resource, index) => (
                            <Badge key={index} variant="secondary" className="text-xs">
                              {resource}
                            </Badge>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>

                  <div className="flex items-center space-x-2 pt-2 border-t">
                    {app.status === "running" ? (
                      <Button size="sm" variant="outline" onClick={(e) => { e.stopPropagation(); handleStop(app.id); }}>
                        <Square className="h-4 w-4" />
                        停止
                      </Button>
                    ) : (
                      <Button size="sm" onClick={(e) => { e.stopPropagation(); handleRun(app.id); }} disabled={app.status === "deploying"}>
                        <Play className="h-4 w-4" />
                        {app.status === "deploying" ? "部署中..." : "运行"}
                      </Button>
                    )}



                    <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); handleEdit(app); }}>
                      <Settings className="h-4 w-4" />
                    </Button>

                    <Button size="sm" variant="ghost" asChild>
                      <a href={app.gitUrl} target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()}>
                        <ExternalLink className="h-4 w-4" />
                      </a>
                    </Button>

                    <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); handleDelete(app.id); }}>
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      </main>
    </div>
  )
}
