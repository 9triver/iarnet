"use client"

import { Sidebar } from "@/components/sidebar"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Server, Package, Activity, ArrowRight, Cpu, HardDrive, Wifi } from "lucide-react"
import { AuthGuard } from "@/components/auth-guard"

export default function HomePage() {
  return (
    <AuthGuard>
      <div className="flex h-screen bg-background">
        <Sidebar />

        <main className="flex-1 overflow-auto">
          <div className="p-8">
          {/* Header */}
          <div className="mb-8">
            <h1 className="text-4xl font-playfair font-bold text-foreground mb-2">欢迎使用 IARNet</h1>
            <p className="text-lg text-muted-foreground">算力网络资源管理与智能应用部署平台</p>
          </div>

          {/* Quick Stats */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">算力资源</CardTitle>
                <Server className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">0</div>
                <p className="text-xs text-muted-foreground">已接入资源节点</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">应用数量</CardTitle>
                <Package className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">0</div>
                <p className="text-xs text-muted-foreground">已部署应用</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">运行状态</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">0</div>
                <p className="text-xs text-muted-foreground">正在运行</p>
              </CardContent>
            </Card>
          </div>

          {/* Quick Actions */}
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <Card className="hover:shadow-lg transition-shadow">
              <CardHeader>
                <div className="flex items-center space-x-2">
                  <Server className="h-6 w-6 text-primary" />
                  <CardTitle>算力资源管理</CardTitle>
                </div>
                <CardDescription>接入和管理您的算力资源，包括CPU、GPU、存储和网络资源</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-3 mb-4">
                  <div className="flex items-center space-x-2 text-sm text-muted-foreground">
                    <Cpu className="h-4 w-4" />
                    <span>CPU/GPU 资源</span>
                  </div>
                  <div className="flex items-center space-x-2 text-sm text-muted-foreground">
                    <HardDrive className="h-4 w-4" />
                    <span>存储资源</span>
                  </div>
                  <div className="flex items-center space-x-2 text-sm text-muted-foreground">
                    <Wifi className="h-4 w-4" />
                    <span>网络资源</span>
                  </div>
                </div>
                <Button className="w-full" asChild>
                  <a href="/resources">
                    开始管理资源
                    <ArrowRight className="ml-2 h-4 w-4" />
                  </a>
                </Button>
              </CardContent>
            </Card>

            <Card className="hover:shadow-lg transition-shadow">
              <CardHeader>
                <div className="flex items-center space-x-2">
                  <Package className="h-6 w-6 text-primary" />
                  <CardTitle>应用管理</CardTitle>
                </div>
                <CardDescription>从Git仓库导入应用，并在算力资源上部署运行</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-3 mb-4">
                  <div className="flex items-center space-x-2 text-sm text-muted-foreground">
                    <Package className="h-4 w-4" />
                    <span>Git 仓库导入</span>
                  </div>
                  <div className="flex items-center space-x-2 text-sm text-muted-foreground">
                    <Activity className="h-4 w-4" />
                    <span>自动部署</span>
                  </div>
                  <div className="flex items-center space-x-2 text-sm text-muted-foreground">
                    <Server className="h-4 w-4" />
                    <span>资源调度</span>
                  </div>
                </div>
                <Button className="w-full" asChild>
                  <a href="/applications">
                    管理应用
                    <ArrowRight className="ml-2 h-4 w-4" />
                  </a>
                </Button>
              </CardContent>
            </Card>

            {/* 状态监控功能暂时移除（功能有问题） */}
            {/* <Card className="hover:shadow-lg transition-shadow">
              <CardHeader>
                <div className="flex items-center space-x-2">
                  <Activity className="h-6 w-6 text-primary" />
                  <CardTitle>运行状态监控</CardTitle>
                </div>
                <CardDescription>实时监控应用运行状态和资源使用情况</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-3 mb-4">
                  <div className="flex items-center space-x-2 text-sm text-muted-foreground">
                    <Activity className="h-4 w-4" />
                    <span>实时状态</span>
                  </div>
                  <div className="flex items-center space-x-2 text-sm text-muted-foreground">
                    <Server className="h-4 w-4" />
                    <span>资源使用</span>
                  </div>
                  <div className="flex items-center space-x-2 text-sm text-muted-foreground">
                    <Package className="h-4 w-4" />
                    <span>性能指标</span>
                  </div>
                </div>
                <Button className="w-full" asChild>
                  <a href="/status">
                    查看状态
                    <ArrowRight className="ml-2 h-4 w-4" />
                  </a>
                </Button>
              </CardContent>
            </Card> */}
          </div>
        </div>
      </main>
    </div>
  </AuthGuard>
  )
}
