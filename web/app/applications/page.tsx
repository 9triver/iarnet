"use client"

import { useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import { applicationsAPI } from "@/lib/api"
import { toast } from "sonner"
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
  Inbox,
} from "lucide-react"
import { Application, ApplicationStats, RunnerEnvironment } from "@/lib/model"
import { formatDateTime } from "@/lib/utils"

interface ApplicationFormData {
  name: string
  gitUrl?: string
  branch?: string
  description?: string
  executeCmd: string
  envInstallCmd?: string
  runnerEnv?: string
}

export default function ApplicationsPage() {
  const router = useRouter()
  const [stats, setStats] = useState<ApplicationStats>({
    total: 0,
    running: 0,
    stopped: 0,
    undeployed: 0,
    failed: 0,
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
  const [runnerEnvironments, setRunnerEnvironments] = useState<string[]>([])


  // 获取应用统计数据
  const fetchStats = async () => {
    try {
      setIsLoadingStats(true)
      const statsData = await applicationsAPI.getStats()
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
      // 转换后端返回的下划线字段名为驼峰格式
      const convertedApps: Application[] = updatedApps.applications.map((app: any) => {
        // 处理时间：检查是否为零值时间（空字符串或 "0001-01-01T00:00:00Z"）
        let lastDeployed: string | undefined = undefined
        if (app.last_deployed && app.last_deployed !== "" && app.last_deployed !== "0001-01-01T00:00:00Z") {
          const date = new Date(app.last_deployed)
          if (!isNaN(date.getTime())) {
            lastDeployed = date.toISOString()
          }
        }
        
        return {
          id: app.id,
          name: app.name,
          description: app.description || "",
          gitUrl: app.git_url,
          branch: app.branch,
          status: (app.status === "idle" ? "idle" : app.status === "running" ? "running" : app.status === "stopped" ? "stopped" : app.status === "error" ? "error" : app.status === "deploying" ? "deploying" : app.status === "cloning" ? "cloning" : "idle") as Application["status"],
          lastDeployed,
          runnerEnv: app.runner_env,
          containerId: app.container_id,
          executeCmd: app.execute_cmd,
          envInstallCmd: app.env_install_cmd,
        }
      })
      setApplications(convertedApps)
    } catch (fetchError) {
      console.error('Failed to fetch updated applications:', fetchError)
    }
  }

  const fetchRunnerEnvironments = async () => {
    try {
      const response = await applicationsAPI.getRunnerEnvironments()
      setRunnerEnvironments(response.environments)
    } catch (fetchError) {
      console.error('Failed to fetch runner environments:', fetchError)
    }
  }

  useEffect(() => {
    fetchStats()
    fetchApplications()
    fetchRunnerEnvironments()
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
      description: "",
      executeCmd: "",
      envInstallCmd: "",
      runnerEnv: "",
    },
  })

  const isValidGitUrl = (url: string): boolean => {
    const gitUrlPattern = /^https?:\/\/(github\.com|gitlab\.com|bitbucket\.org)\/[\w\-.]+\/[\w\-.]+(?:\.git)?$/i
    return (
      gitUrlPattern.test(url) || /^git@(github\.com|gitlab\.com|bitbucket\.org):[\w\-.]+\/[\w\-.]+\.git$/i.test(url)
    )
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

  const onSubmit = async (data: ApplicationFormData) => {
    if (editingApp) {
      // 编辑现有应用 - 调用后端API
      try {
        const updateData = {
          name: data.name,
          description: data.description || "",
          execute_cmd: data.executeCmd,
          env_install_cmd: data.envInstallCmd,
          runner_env: data.runnerEnv,
        }

        console.log('更新应用数据:', updateData)
        await applicationsAPI.update(editingApp.id, updateData)
        // 更新成功后，重新获取所有应用数据
        console.log('更新成功后，重新获取所有应用数据')
        handleRefreshData()
        toast.success(`应用 "${data.name}" 已成功更新`)

      } catch (error) {
        console.error('Failed to update application:', error)
        toast.error("应用更新时发生错误，请稍后重试")
        return
      }
    } else {
      // 创建新应用 - 调用后端API
      try {
        const createData = {
          name: data.name,
          git_url: data.gitUrl,
          branch: data.branch || "main",
          description: data.description || "",
          execute_cmd: data.executeCmd,
          env_install_cmd: data.envInstallCmd,
          runner_env: data.runnerEnv,
        }

        if (await applicationsAPI.create(createData)) { // TODO: fix
          // 创建成功后，重新获取所有应用数据
          handleRefreshData()
          toast.success(`应用 "${data.name}" 已成功创建`)
        } else {
          toast.error("应用创建失败，请稍后重试")
        }
      } catch (error) {
        console.error('Failed to create application:', error)
        toast.error("应用创建时发生错误，请稍后重试")
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
    form.setValue("description", app.description)
    form.setValue("executeCmd", app.executeCmd || "")
    form.setValue("envInstallCmd", app.envInstallCmd || "")
    form.setValue("runnerEnv", app.runnerEnv || "")
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

  const handleRun = async (id: string) => {
    try {
      // 立即更新UI状态为部署中
      setApplications((prev) => prev.map((app) => (app.id === id ? { ...app, status: "deploying" } : app)))
      
      // 调用后端API运行应用
      const updatedApp = await applicationsAPI.run(id)
      
      // 更新应用状态
      setApplications((prev) =>
        prev.map((app) =>
          app.id === id
            ? {
                ...app,
                status: updatedApp.status,
                lastDeployed: updatedApp.lastDeployed || new Date().toISOString(),
              }
            : app,
        ),
      )
      
      toast.success(`应用 "${updatedApp.name}" 已成功启动`)
    } catch (error) {
      console.error('Failed to run application:', error)
      // 如果失败，恢复状态
      setApplications((prev) => prev.map((app) => (app.id === id ? { ...app, status: "stopped" } : app)))
      toast.error("应用启动失败，请稍后重试")
    }
  }

  const handleStop = async (id: string) => {
    try {
      // 调用后端API停止应用
      const updatedApp = await applicationsAPI.stop(id)
      
      // 更新应用状态
      setApplications((prev) =>
        prev.map((app) =>
          app.id === id
            ? {
                ...app,
                status: updatedApp.status,
              }
            : app,
        ),
      )
      
      toast.success(`应用 "${updatedApp.name}" 已成功停止`)
    } catch (error) {
      console.error('Failed to stop application:', error)
      toast.error("应用停止失败，请稍后重试")
    }
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
      case "cloning":
        return (
          <Badge variant="default" className="bg-yellow-500">
            克隆中
          </Badge>
        )
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

                      <div className="grid grid-cols-12 gap-4">
                        <FormField
                          control={form.control}
                          name="gitUrl"
                          render={({ field }) => (
                            <FormItem className="col-span-8">
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

                        <FormField
                          control={form.control}
                          name="branch"
                          render={({ field }) => (
                            <FormItem className="col-span-4">
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
                      </div>

                      <FormField
                        control={form.control}
                        name="envInstallCmd"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>环境安装命令（可选）</FormLabel>
                            <FormControl>
                              <Textarea 
                                placeholder="输入环境安装命令，支持多行：&#10;pip install -r requirements.txt&#10;&#10;或者：&#10;npm install&#10;yarn install" 
                                className="min-h-[80px]"
                                {...field} 
                              />
                            </FormControl>
                            <FormDescription>在运行应用前执行的依赖安装命令</FormDescription>
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
                              <Textarea 
                                placeholder="输入应用启动命令，支持多行：&#10;npm install&#10;npm start&#10;&#10;或者：&#10;pip install -r requirements.txt&#10;python app.py" 
                                className="min-h-[100px]"
                                {...field} 
                              />
                            </FormControl>
                            <FormDescription>应用启动时执行的命令</FormDescription>
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
                            <Select onValueChange={field.onChange} defaultValue={field.value}>
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
                  const lastDeployedDisplay = new Date().toLocaleString()

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
          {applications.length === 0 ? (
            <Card>
              <CardContent className="flex flex-col items-center justify-center py-16">
                <Inbox className="h-16 w-16 text-muted-foreground mb-4" />
                <h3 className="text-lg font-semibold text-foreground mb-2">暂无应用</h3>
                <p className="text-sm text-muted-foreground text-center max-w-md">
                  您还没有导入任何应用。点击右上角的"导入应用"按钮开始导入您的第一个应用。
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {applications.map((app) => {
                const lastDeployedDisplay = app.lastDeployed 
                  ? formatDateTime(app.lastDeployed) 
                  : "未部署"

                return (
                  <Card
                  key={app.id}
                  className="hover:shadow-lg transition-shadow cursor-pointer"
                  onClick={() => router.push(`/applications/${app.id}`)}
                >
                  <CardHeader>
                    <div className="flex items-start justify-between">
                      <div className="flex items-center space-x-2">
                        <Package className="h-4 w-4" />
                        <div>
                          <CardTitle className="text-lg">{app.name}</CardTitle>
                          <CardDescription className="flex items-center space-x-2 mt-1">
                            {getStatusBadge(app.status)}
                          </CardDescription>
                        </div>
                      </div>
                    </div>
                  </CardHeader>

                  <CardContent className="space-y-4">
                    {app.description && (
                      <p className="text-sm text-muted-foreground line-clamp-2">{app.description}</p>
                    )}

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

                      {app.runnerEnv && (
                        <div className="flex items-center space-x-2 text-sm">
                          <Cpu className="h-4 w-4 text-muted-foreground" />
                          <span className="text-muted-foreground">运行环境:</span>
                          <span className="font-mono">{app.runnerEnv}</span>
                        </div>
                      )}

                      <div className="flex items-center space-x-2 text-sm">
                        <Clock className="h-4 w-4 text-muted-foreground" />
                        <span className="text-muted-foreground">最后部署:</span>
                        <span className="text-xs">{lastDeployedDisplay}</span>
                      </div>
                    </div>

                    <div className="flex items-center space-x-2 pt-2 border-t">
                      {app.status === "running" ? (
                        <Button size="sm" variant="outline" onClick={(e) => { e.stopPropagation(); handleStop(app.id); }}>
                          <Square className="h-4 w-4" />
                          停止
                        </Button>
                      ) : (
                        <Button size="sm" onClick={(e) => { e.stopPropagation(); handleRun(app.id); }} disabled={app.status === "deploying" || app.status === "cloning"}>
                          <Play className="h-4 w-4" />
                          {app.status === "deploying" ? "部署中..." : app.status === "cloning" ? "克隆中..." : "运行"}
                        </Button>
                      )}



                      <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); handleEdit(app); }}>
                        <Settings className="h-4 w-4" />
                      </Button>

                      {(() => {
                        const httpsUrl = convertGitUrlToHttps(app.gitUrl || '')
                        return httpsUrl ? (
                          <Button size="sm" variant="ghost" asChild>
                            <a href={httpsUrl} target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()}>
                              <ExternalLink className="h-4 w-4" />
                            </a>
                          </Button>
                        ) : (
                          <Button 
                            size="sm" 
                            variant="ghost" 
                            disabled 
                            title="无法打开此 Git 仓库链接"
                            onClick={(e) => { e.stopPropagation(); }}
                          >
                            <ExternalLink className="h-4 w-4 opacity-50" />
                          </Button>
                        )
                      })()}

                      <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); handleDelete(app.id); }}>
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </CardContent>
                  </Card>
                )
              })}
            </div>
          )}
        </div>
      </main>
    </div>
  )
}
