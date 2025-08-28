"use client"

import * as React from "react"
import { cn } from "@/lib/utils"
import { X } from "lucide-react"

interface ToastProps {
  title?: string
  description?: string
  variant?: "default" | "destructive" | "success"
  onClose?: () => void
}

export function Toast({ title, description, variant = "default", onClose }: ToastProps) {
  const [isVisible, setIsVisible] = React.useState(true)

  React.useEffect(() => {
    const timer = setTimeout(() => {
      setIsVisible(false)
      onClose?.()
    }, 5000)

    return () => clearTimeout(timer)
  }, [onClose])

  if (!isVisible) return null

  const variantStyles = {
    default: "bg-background border-border text-foreground",
    destructive: "bg-destructive text-destructive-foreground",
    success: "bg-green-600 text-white",
  }

  return (
    <div
      className={cn("fixed top-4 right-4 z-50 w-full max-w-sm rounded-lg border p-4 shadow-lg", variantStyles[variant])}
    >
      <div className="flex items-start gap-3">
        <div className="flex-1">
          {title && <div className="font-semibold">{title}</div>}
          {description && <div className="text-sm opacity-90">{description}</div>}
        </div>
        <button
          onClick={() => {
            setIsVisible(false)
            onClose?.()
          }}
          className="opacity-70 hover:opacity-100"
        >
          <X className="h-4 w-4" />
        </button>
      </div>
    </div>
  )
}

// Toast 管理器
interface ToastState {
  id: string
  title?: string
  description?: string
  variant?: "default" | "destructive" | "success"
}

export function useToast() {
  const [toasts, setToasts] = React.useState<ToastState[]>([])

  const toast = React.useCallback((props: Omit<ToastState, "id">) => {
    const id = Date.now().toString()
    setToasts((prev) => [...prev, { ...props, id }])
  }, [])

  const dismiss = React.useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  return {
    toasts,
    toast,
    dismiss,
  }
}
