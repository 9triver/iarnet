"use client"

import type { ReactNode } from "react"
import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { useIARNetStore } from "@/lib/store"
import { FullscreenLoader } from "@/components/ui/fullscreen-loader"

interface AuthGuardProps {
  children: ReactNode
  requireAdmin?: boolean
}

export function AuthGuard({ children, requireAdmin = false }: AuthGuardProps) {
  const router = useRouter()
  const setupCompleted = useIARNetStore((state) => state.setupCompleted)
  const currentUser = useIARNetStore((state) => state.currentUser)
  const [hydrated, setHydrated] = useState(useIARNetStore.persist?.hasHydrated?.() ?? false)

  useEffect(() => {
    const unsubHydrate = useIARNetStore.persist?.on?.("hydrate", () => setHydrated(false))
    const unsubFinish = useIARNetStore.persist?.on?.("finishHydration", () => setHydrated(true))
    if (useIARNetStore.persist?.hasHydrated?.()) {
      setHydrated(true)
    }
    return () => {
      unsubHydrate?.()
      unsubFinish?.()
    }
  }, [])

  useEffect(() => {
    if (!hydrated) return
    if (!setupCompleted) {
      router.replace("/setup")
      return
    }
    if (!currentUser) {
      router.replace("/login")
      return
    }
    if (requireAdmin && currentUser.role !== "admin") {
      router.replace("/")
    }
  }, [hydrated, setupCompleted, currentUser, requireAdmin, router])

  if (!hydrated || !setupCompleted || !currentUser) {
    return <FullscreenLoader message="正在校验访问权限..." />
  }

  if (requireAdmin && currentUser.role !== "admin") {
    return <FullscreenLoader message="正在跳转..." />
  }

  return <>{children}</>
}

