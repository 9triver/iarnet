"use client"

import { useState, useMemo } from "react"
import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Server, Package, Activity, Menu, X, Cpu, Users, LogOut } from "lucide-react"
import { useIARNetStore } from "@/lib/store"

const baseNavigation = [
  {
    name: "算力资源管理",
    href: "/resources",
    icon: Server,
    description: "接入和管理算力资源",
  },
  {
    name: "应用管理",
    href: "/applications",
    icon: Package,
    description: "导入和部署应用",
  },
  {
    name: "运行状态",
    href: "/status",
    icon: Activity,
    description: "监控应用运行状态",
  },
]

export function Sidebar() {
  const [isCollapsed, setIsCollapsed] = useState(false)
  const pathname = usePathname()
  const router = useRouter()
  const currentUser = useIARNetStore((state) => state.currentUser)
  const logout = useIARNetStore((state) => state.logout)

  const navigation = useMemo(() => {
    if (currentUser?.role === "admin") {
      return [
        ...baseNavigation,
        {
          name: "用户管理",
          href: "/users",
          icon: Users,
          description: "维护平台用户",
        },
      ]
    }
    return baseNavigation
  }, [currentUser])

  const handleLogout = () => {
    logout()
    router.replace("/login")
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
            <div className="text-xs text-sidebar-foreground/70">
              {currentUser.role === "admin" ? "管理员" : "普通用户"}
            </div>
          </div>
        )}
        <Button
          variant="ghost"
          size={isCollapsed ? "icon" : "sm"}
          className="w-full text-sidebar-foreground hover:bg-sidebar-accent"
          onClick={handleLogout}
        >
          <LogOut className="h-4 w-4" />
          {!isCollapsed && <span className="ml-2">退出登录</span>}
        </Button>
        {!isCollapsed && <div className="mt-3 text-xs text-sidebar-foreground/60">IARNet v1.0.0</div>}
      </div>
    </div>
  )
}
