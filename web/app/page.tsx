"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { AuthGuard } from "@/components/auth-guard"
import { FullscreenLoader } from "@/components/ui/fullscreen-loader"
import { useIARNetStore } from "@/lib/store"

export default function HomePage() {
  const router = useRouter()
  const currentUser = useIARNetStore((state) => state.currentUser)

  useEffect(() => {
    // 如果用户已登录，直接重定向到资源管理页面
    if (currentUser) {
      router.replace("/resources")
    }
  }, [currentUser, router])

  return (
    <AuthGuard>
      <FullscreenLoader message="正在跳转到资源管理页面..." />
    </AuthGuard>
  )
}
