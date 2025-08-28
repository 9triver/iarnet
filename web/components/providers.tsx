"use client"

import type React from "react"

import { useEffect } from "react"
import { useIARNetStore } from "@/lib/store"

export function Providers({ children }: { children: React.ReactNode }) {
  const initializeData = useIARNetStore((state) => state.initializeData)

  useEffect(() => {
    // 初始化数据
    initializeData()
  }, [initializeData])

  return <>{children}</>
}
