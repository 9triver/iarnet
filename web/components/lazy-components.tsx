"use client"

import dynamic from "next/dynamic"
import { LoadingSpinner } from "@/components/ui/loading-spinner"

export const LazyResourcesPage = dynamic(
  () => import("@/app/resources/page").then((mod) => ({ default: mod.default })),
  {
    loading: () => (
      <div className="flex items-center justify-center min-h-[400px]">
        <LoadingSpinner size="lg" />
      </div>
    ),
  },
)

export const LazyApplicationsPage = dynamic(
  () => import("@/app/applications/page").then((mod) => ({ default: mod.default })),
  {
    loading: () => (
      <div className="flex items-center justify-center min-h-[400px]">
        <LoadingSpinner size="lg" />
      </div>
    ),
  },
)

export const LazyStatusPage = dynamic(() => import("@/app/status/page").then((mod) => ({ default: mod.default })), {
  loading: () => (
    <div className="flex items-center justify-center min-h-[400px]">
      <LoadingSpinner size="lg" />
    </div>
  ),
})
