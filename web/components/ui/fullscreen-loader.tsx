"use client"

import { Loader2 } from "lucide-react"

export function FullscreenLoader({ message = "加载中..." }: { message?: string }) {
  return (
    <div className="flex h-screen w-full flex-col items-center justify-center space-y-4 text-muted-foreground">
      <Loader2 className="h-8 w-8 animate-spin text-primary" />
      <p className="text-sm">{message}</p>
    </div>
  )
}

