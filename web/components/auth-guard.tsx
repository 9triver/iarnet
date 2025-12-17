"use client"

import type { ReactNode } from "react"
import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { useIARNetStore } from "@/lib/store"
import { FullscreenLoader } from "@/components/ui/fullscreen-loader"

interface AuthGuardProps {
  children: ReactNode
}

export function AuthGuard({ children }: AuthGuardProps) {
  const router = useRouter()
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
    if (!currentUser) {
      router.replace("/login")
      return
    }
  }, [hydrated, currentUser, router])

  if (!hydrated || !currentUser) {
    return <FullscreenLoader message="正在校验访问权限..." />
  }

  return <>{children}</>
}

