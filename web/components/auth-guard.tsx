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
  const [hydrated, setHydrated] = useState(false)
  const [isRedirecting, setIsRedirecting] = useState(false)

  useEffect(() => {
    // 检查是否已经完成 hydration
    const checkHydration = () => {
      if (useIARNetStore.persist?.hasHydrated?.()) {
        setHydrated(true)
        return true
      }
      return false
    }

    // 立即检查一次
    if (checkHydration()) {
      return
    }

    // 如果还没有完成 hydration，定期检查
    const interval = setInterval(() => {
      if (checkHydration()) {
        clearInterval(interval)
      }
    }, 50)

    // 设置超时，避免无限等待
    const timeout = setTimeout(() => {
      clearInterval(interval)
      setHydrated(true)
    }, 1000)

    return () => {
      clearInterval(interval)
      clearTimeout(timeout)
    }
  }, [])

  useEffect(() => {
    if (!hydrated) return
    
    if (!currentUser) {
      // 如果用户未登录，跳转到登录页面，并携带当前路径作为 redirect 参数
      setIsRedirecting(true)
      const redirectPath = pathname && pathname !== "/login" ? pathname : undefined
      const loginUrl = redirectPath ? `/login?redirect=${encodeURIComponent(redirectPath)}` : "/login"
      router.replace(loginUrl)
      return
    }
    
    // 如果用户已登录，确保 isRedirecting 为 false
    setIsRedirecting(false)
  }, [hydrated, currentUser, router, pathname])

  // 只有在 hydration 未完成或正在重定向时才显示加载器
  // 一旦 hydration 完成且用户已登录，立即显示内容，避免闪烁
  if (!hydrated || isRedirecting || !currentUser) {
    return <FullscreenLoader message="正在校验访问权限..." />
  }

  return <>{children}</>
}
