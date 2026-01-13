"use client"

import { useState, useMemo } from "react"
import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Server, Package, Activity, Menu, X, Cpu, LogOut, Shield, Users, Key } from "lucide-react"
import { useIARNetStore } from "@/lib/store"
import { canManageUsers, canAccessAudit, getRoleDisplayName } from "@/lib/permissions"
import { authAPI, getErrorMessage, APIError } from "@/lib/api"
import { toast } from "sonner"

const baseNavigation = [
  {
    name: "资源管理",
    href: "/resources",
    icon: Server,
    description: "接入和管理算力资源",
    requiredRole: "normal" as const, // 所有用户都可以访问
  },
  {
    name: "应用管理",
    href: "/applications",
    icon: Package,
    description: "导入和部署应用",
    requiredRole: "normal" as const, // 所有用户都可以访问
  },
  {
    name: "日志审计",
    href: "/audit",
    icon: Shield,
    description: "查看系统日志和操作记录",
    requiredRole: "platform" as const, // 平台管理员及以上可以访问
  },
  {
    name: "用户管理",
    href: "/users",
    icon: Users,
    description: "管理系统用户和权限",
    requiredRole: "super" as const, // 只有超级管理员可以访问
  },
  // 状态监控功能暂时移除（功能有问题）
  // {
  //   name: "状态监控",
  //   href: "/status",
  //   icon: Activity,
  //   description: "监控本地资源运行状态",
  // },
]

export function Sidebar() {
  const [isCollapsed, setIsCollapsed] = useState(false)
  const [isChangePasswordDialogOpen, setIsChangePasswordDialogOpen] = useState(false)
  const [oldPassword, setOldPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [isChangingPassword, setIsChangingPassword] = useState(false)
  const pathname = usePathname()
  const router = useRouter()
  const currentUser = useIARNetStore((state) => state.currentUser)
  const logout = useIARNetStore((state) => state.logout)

  const navigation = useMemo(() => {
    const userRole = currentUser?.role
    // 根据用户权限过滤导航菜单
    return baseNavigation.filter((item) => {
      switch (item.requiredRole) {
        case "super":
          return canManageUsers(userRole)
        case "platform":
          return canAccessAudit(userRole)
        case "normal":
          return true // 所有用户都可以访问
        default:
          return false
      }
    })
  }, [currentUser])

  const handleLogout = () => {
    logout()
    // 延迟跳转，确保状态更新完成
    setTimeout(() => {
      router.replace("/login")
    }, 0)
  }

  const handleChangePassword = async () => {
    if (!oldPassword || !newPassword || !confirmPassword) {
      toast.error("请填写所有字段")
      return
    }

    if (newPassword !== confirmPassword) {
      toast.error("新密码和确认密码不一致")
      return
    }

    if (newPassword.length < 6) {
      toast.error("新密码长度至少为6位")
      return
    }

    setIsChangingPassword(true)
    try {
      await authAPI.changePassword({
        oldPassword,
        newPassword,
      })
      toast.success("密码修改成功")
      setIsChangePasswordDialogOpen(false)
      setOldPassword("")
      setNewPassword("")
      setConfirmPassword("")
    } catch (error) {
      const errorMessage = error instanceof APIError 
        ? getErrorMessage(error.message) 
        : error instanceof Error 
        ? error.message 
        : "修改密码失败"
      toast.error("修改密码失败", {
        description: errorMessage,
      })
    } finally {
      setIsChangingPassword(false)
    }
  }

  return (
    <div
      className={cn(
        "flex flex-col h-screen bg-sidebar border-r border-sidebar-border transition-all duration-300",
        isCollapsed ? "w-16" : "w-64",
      )}
    >
      {/* Header */}
      <div className="flex items-center justify-between p-4 border-b border-sidebar-border">
        {!isCollapsed && (
          <div className="flex items-center space-x-2">
            <Cpu className="h-8 w-8 text-sidebar-primary" />
            <h1 className="text-xl font-playfair font-bold text-sidebar-foreground">IARNet</h1>
          </div>
        )}
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setIsCollapsed(!isCollapsed)}
          className="text-sidebar-foreground hover:bg-sidebar-accent"
        >
          {isCollapsed ? <Menu className="h-4 w-4" /> : <X className="h-4 w-4" />}
        </Button>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-2 p-4">
        {navigation.map((item) => {
          const isActive = pathname === item.href
          return (
            <Link
              key={item.name}
              href={item.href}
              className={cn(
                "flex items-center space-x-3 px-3 py-2 rounded-lg transition-colors",
                isActive
                  ? "bg-sidebar-primary text-sidebar-primary-foreground"
                  : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
              )}
            >
              <item.icon className="h-5 w-5 flex-shrink-0" />
              {!isCollapsed && (
                <div>
                  <div className="font-medium">{item.name}</div>
                  <div className="text-xs opacity-70">{item.description}</div>
                </div>
              )}
            </Link>
          )
        })}
      </nav>

      {/* Footer */}
      <div className="border-t border-sidebar-border p-4">
        {!isCollapsed && currentUser && (
          <div className="mb-3 space-y-1">
            <div className="text-sm font-semibold text-sidebar-foreground">{currentUser.username}</div>
            {currentUser.role && (
              <div className="text-xs text-sidebar-foreground/60">{getRoleDisplayName(currentUser.role)}</div>
            )}
          </div>
        )}
        <div className="flex gap-2">
          <Button
            variant="ghost"
            size={isCollapsed ? "icon" : "sm"}
            className="flex-1 text-sidebar-foreground hover:bg-sidebar-accent"
            onClick={() => setIsChangePasswordDialogOpen(true)}
          >
            <Key className="h-4 w-4" />
            {!isCollapsed && <span className="ml-2">修改密码</span>}
          </Button>
          <Button
            variant="ghost"
            size={isCollapsed ? "icon" : "sm"}
            className="flex-1 text-sidebar-foreground hover:bg-sidebar-accent"
            onClick={handleLogout}
          >
            <LogOut className="h-4 w-4" />
            {!isCollapsed && <span className="ml-2">退出登录</span>}
          </Button>
        </div>
        {!isCollapsed && <div className="mt-3 text-xs text-sidebar-foreground/60">IARNet v1.0.0</div>}
      </div>

      {/* 修改密码对话框 */}
      <Dialog open={isChangePasswordDialogOpen} onOpenChange={setIsChangePasswordDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>修改密码</DialogTitle>
            <DialogDescription>请输入当前密码和新密码</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="old-password">当前密码</Label>
              <Input
                id="old-password"
                type="password"
                value={oldPassword}
                onChange={(e) => setOldPassword(e.target.value)}
                placeholder="请输入当前密码"
                disabled={isChangingPassword}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-password">新密码</Label>
              <Input
                id="new-password"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="请输入新密码（至少6位）"
                disabled={isChangingPassword}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="confirm-password">确认新密码</Label>
              <Input
                id="confirm-password"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="请再次输入新密码"
                disabled={isChangingPassword}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !isChangingPassword) {
                    handleChangePassword()
                  }
                }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setIsChangePasswordDialogOpen(false)
                setOldPassword("")
                setNewPassword("")
                setConfirmPassword("")
              }}
              disabled={isChangingPassword}
            >
              取消
            </Button>
            <Button onClick={handleChangePassword} disabled={isChangingPassword}>
              {isChangingPassword ? "修改中..." : "确认修改"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
