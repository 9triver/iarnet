"use client"

import type { ReactNode } from "react"
import { useEffect, useState } from "react"
import { useRouter, usePathname } from "next/navigation"
import { useIARNetStore } from "@/lib/store"
import { FullscreenLoader } from "@/components/ui/fullscreen-loader"

interface AuthGuardProps {
  children: ReactNode
}

export function AuthGuard({ children }: AuthGuardProps) {
  const router = useRouter()
  const pathname = usePathname()
  const currentUser = useIARNetStore((state) => state.currentUser)
  const [hydrated, setHydrated] = useState(useIARNetStore.persist?.hasHydrated?.() ?? false)

  useEffect(() => {
    // 检查是否已经完成 hydration
    if (useIARNetStore.persist?.hasHydrated?.()) {
      setHydrated(true)
    } else {
      // 如果还没有完成 hydration，等待一下再检查
      const timer = setTimeout(() => {
        if (useIARNetStore.persist?.hasHydrated?.()) {
          setHydrated(true)
        }
      }, 100)
      return () => clearTimeout(timer)
    }
  }, [])

  useEffect(() => {
    if (!hydrated) return
    if (!currentUser) {
      // 如果用户未登录，跳转到登录页面，并携带当前路径作为 redirect 参数
      const redirectPath = pathname && pathname !== "/login" ? pathname : undefined
      const loginUrl = redirectPath ? `/login?redirect=${encodeURIComponent(redirectPath)}` : "/login"
      router.replace(loginUrl)
      return
    }
  }, [hydrated, currentUser, router, pathname])

  if (!hydrated || !currentUser) {
    return <FullscreenLoader message="正在校验访问权限..." />
  }

  return <>{children}</>
}

