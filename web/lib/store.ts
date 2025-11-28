"use client"

import { create } from "zustand"
import { persist } from "zustand/middleware"
import { hashPassword, verifyPassword } from "@/lib/auth"

// 资源相关类型定义
export interface Resource {
  id: string
  name: string
  type: "kubernetes" | "docker" | "vm"
  url: string
  status: "connected" | "disconnected" | "error"
  cpu: {
    total: number
    used: number
  }
  memory: {
    total: number
    used: number
  }
  storage: {
    total: number
    used: number
  }
  lastUpdated: string
}

// 应用相关类型定义
export interface Application {
  id: string
  name: string
  description: string
  gitUrl: string
  branch: string
  status: "idle" | "running" | "stopped" | "error" | "deploying"
  type: "web" | "api" | "worker" | "database"
  lastDeployed?: string
  runningOn?: string[]
  ports?: number[]
  healthCheck?: string
}

// 应用状态相关类型定义
export interface ApplicationStatus {
  id: string
  name: string
  status: "running" | "stopped" | "error" | "warning"
  uptime: string
  cpu: number
  memory: number
  network: number
  storage: number
  instances: number
  healthCheck: "healthy" | "unhealthy" | "unknown"
  lastRestart: string
  runningOn: string[]
  logs: LogEntry[]
  metrics: MetricData[]
}

export interface LogEntry {
  timestamp: string
  level: "info" | "warn" | "error"
  message: string
}

export interface MetricData {
  timestamp: string
  cpu: number
  memory: number
  network: number
  requests: number
}

// 全局状态接口
interface AsyncState {
  isLoading: boolean
  error: string | null
}

export type UserRole = "admin" | "user"

export interface UserAccount {
  id: string
  username: string
  role: UserRole
  passwordHash: string
}

export type CurrentUser = Omit<UserAccount, "passwordHash">

interface IARNetStore {
  // 资源管理状态
  resources: Resource[]
  addResource: (resource: Omit<Resource, "id">) => void
  updateResource: (id: string, resource: Partial<Resource>) => void
  deleteResource: (id: string) => void

  // 应用管理状态
  applications: Application[]
  addApplication: (app: Omit<Application, "id">) => void
  updateApplication: (id: string, app: Partial<Application>) => void
  deleteApplication: (id: string) => void
  deployApplication: (id: string) => Promise<void>
  stopApplication: (id: string) => void

  // 应用状态监控
  applicationStatuses: ApplicationStatus[]
  updateApplicationStatus: (id: string, status: Partial<ApplicationStatus>) => void
  restartApplication: (id: string) => void

  // 全局加载状态
  loadingStates: Record<string, boolean>
  setLoadingState: (key: string, loading: boolean) => void
  clearLoadingStates: () => void

  // 错误处理
  errors: Record<string, string>
  setError: (key: string, error: string | null) => void
  clearError: (key: string) => void
  clearAllErrors: () => void

  // 初始化数据
  initializeData: () => void

  // 异步操作方法
  fetchResources: () => Promise<void>
  fetchApplications: () => Promise<void>
  fetchApplicationStatuses: () => Promise<void>
  refreshData: () => Promise<void>

  // 认证与用户管理
  setupCompleted: boolean
  users: UserAccount[]
  currentUser: CurrentUser | null
  createInitialAdmin: (payload: { username: string; password: string }) => Promise<void>
  addUserAccount: (payload: { username: string; password: string; role?: UserRole }) => Promise<void>
  deleteUserAccount: (id: string) => Promise<void>
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const generateId = () => {
  if (typeof crypto !== "undefined" && crypto.randomUUID) {
    return crypto.randomUUID()
  }
  return Date.now().toString()
}

const toPublicUser = (user: UserAccount): CurrentUser => ({
  id: user.id,
  username: user.username,
  role: user.role,
})

export const useIARNetStore = create<IARNetStore>()(
  persist(
    (set, get) => ({
      // 初始状态
      resources: [],
      applications: [],
      applicationStatuses: [],
      loadingStates: {},
      errors: {},
      setupCompleted: false,
      users: [],
      currentUser: null,

      // 资源管理方法
      addResource: (resource) => {
        const newResource: Resource = {
          ...resource,
          id: Date.now().toString(),
        }
        set((state) => ({
          resources: [...state.resources, newResource],
        }))
      },

      updateResource: (id, resource) => {
        set((state) => ({
          resources: state.resources.map((r) => (r.id === id ? { ...r, ...resource } : r)),
        }))
      },

      deleteResource: (id) => {
        set((state) => ({
          resources: state.resources.filter((r) => r.id !== id),
        }))
      },

      // 应用管理方法
      addApplication: (app) => {
        const newApp: Application = {
          ...app,
          id: Date.now().toString(),
          status: "idle",
        }
        set((state) => ({
          applications: [...state.applications, newApp],
        }))
      },

      updateApplication: (id, app) => {
        set((state) => ({
          applications: state.applications.map((a) => (a.id === id ? { ...a, ...app } : a)),
        }))
      },

      deleteApplication: (id) => {
        set((state) => ({
          applications: state.applications.filter((a) => a.id !== id),
          applicationStatuses: state.applicationStatuses.filter((s) => s.id !== id),
        }))
      },

      deployApplication: async (id) => {
        const { setLoadingState, setError } = get()
        try {
          setLoadingState(`deploy-${id}`, true)
          setError(`deploy-${id}`, null)

          set((state) => ({
            applications: state.applications.map((app) =>
              app.id === id
                ? {
                    ...app,
                    status: "deploying" as const,
                    lastDeployed: new Date().toLocaleString(),
                    runningOn: ["生产环境集群"],
                  }
                : app,
            ),
          }))

          const response = await fetch(`/api/applications/${id}/deploy`, {
            method: "POST",
          })

          if (!response.ok) throw new Error("部署失败")

          // 模拟部署过程
          setTimeout(() => {
            set((state) => ({
              applications: state.applications.map((app) =>
                app.id === id ? { ...app, status: "running" as const } : app,
              ),
            }))
            setLoadingState(`deploy-${id}`, false)
          }, 3000)
        } catch (error) {
          setError(`deploy-${id}`, error instanceof Error ? error.message : "部署失败")
          set((state) => ({
            applications: state.applications.map((app) => (app.id === id ? { ...app, status: "error" as const } : app)),
          }))
          setLoadingState(`deploy-${id}`, false)
        }
      },

      stopApplication: (id) => {
        set((state) => ({
          applications: state.applications.map((app) =>
            app.id === id ? { ...app, status: "stopped" as const, runningOn: undefined } : app,
          ),
        }))
      },

      // 应用状态监控方法
      updateApplicationStatus: (id, status) => {
        set((state) => ({
          applicationStatuses: state.applicationStatuses.map((appStatus) =>
            appStatus.id === id ? { ...appStatus, ...status } : appStatus,
          ),
        }))
      },

      restartApplication: (id) => {
        set((state) => ({
          applicationStatuses: state.applicationStatuses.map((appStatus) =>
            appStatus.id === id
              ? {
                  ...appStatus,
                  lastRestart: new Date().toLocaleString(),
                  uptime: "0分钟",
                }
              : appStatus,
          ),
        }))
      },

      // 全局状态方法
      setLoadingState: (key, loading) => {
        set((state) => ({
          loadingStates: { ...state.loadingStates, [key]: loading },
        }))
      },

      clearLoadingStates: () => {
        set({ loadingStates: {} })
      },

      setError: (key, error) => {
        set((state) => {
          const newErrors = { ...state.errors }
          if (error) {
            newErrors[key] = error
          } else {
            delete newErrors[key]
          }
          return { errors: newErrors }
        })
      },

      clearError: (key) => {
        set((state) => {
          const newErrors = { ...state.errors }
          delete newErrors[key]
          return { errors: newErrors }
        })
      },

      clearAllErrors: () => {
        set({ errors: {} })
      },

      // 初始化数据
      initializeData: () => {
        const { resources, applications, applicationStatuses } = get()

        // 如果没有数据，初始化示例数据
        if (resources.length === 0) {
          set({
            resources: [
              {
                id: "1",
                name: "生产环境集群",
                type: "kubernetes",
                url: "https://k8s-prod.example.com",
                status: "connected",
                cpu: { total: 32, used: 18 },
                memory: { total: 128, used: 76 },
                storage: { total: 2048, used: 1024 },
                lastUpdated: "2024-01-15 14:30:00",
              },
              {
                id: "2",
                name: "开发环境",
                type: "docker",
                url: "https://docker-dev.example.com",
                status: "connected",
                cpu: { total: 16, used: 8 },
                memory: { total: 64, used: 32 },
                storage: { total: 1024, used: 256 },
                lastUpdated: "2024-01-15 14:25:00",
              },
            ],
          })
        }

        if (applications.length === 0) {
          set({
            applications: [
              {
                id: "1",
                name: "用户管理系统",
                description: "基于React和Node.js的用户管理后台系统",
                gitUrl: "https://github.com/company/user-management",
                branch: "main",
                status: "running",
                type: "web",
                lastDeployed: "2024-01-15 14:30:00",
                runningOn: ["生产环境集群"],
                ports: [3000],
                healthCheck: "/health",
              },
              {
                id: "2",
                name: "数据处理服务",
                description: "Python数据处理和分析服务",
                gitUrl: "https://github.com/company/data-processor",
                branch: "develop",
                status: "idle",
                type: "worker",
                lastDeployed: "2024-01-14 10:15:00",
                ports: [8080],
              },
              {
                id: "3",
                name: "API网关",
                description: "微服务API网关和路由服务",
                gitUrl: "https://github.com/company/api-gateway",
                branch: "main",
                status: "running",
                type: "api",
                lastDeployed: "2024-01-15 09:45:00",
                runningOn: ["生产环境集群", "开发环境"],
                ports: [8000],
                healthCheck: "/api/health",
              },
            ],
          })
        }

        if (applicationStatuses.length === 0) {
          set({
            applicationStatuses: [
              {
                id: "1",
                name: "用户管理系统",
                status: "running",
                uptime: "7天 12小时 30分钟",
                cpu: 45,
                memory: 68,
                network: 23,
                storage: 34,
                instances: 3,
                healthCheck: "healthy",
                lastRestart: "2024-01-08 09:15:00",
                runningOn: ["生产环境集群"],
                logs: [
                  { timestamp: "2024-01-15 14:30:00", level: "info", message: "用户登录成功: user@example.com" },
                  { timestamp: "2024-01-15 14:29:45", level: "warn", message: "数据库连接池使用率达到80%" },
                  { timestamp: "2024-01-15 14:29:30", level: "info", message: "处理用户请求: GET /api/users" },
                ],
                metrics: [
                  { timestamp: "14:25", cpu: 42, memory: 65, network: 20, requests: 150 },
                  { timestamp: "14:26", cpu: 45, memory: 67, network: 22, requests: 165 },
                  { timestamp: "14:27", cpu: 48, memory: 69, network: 25, requests: 180 },
                  { timestamp: "14:28", cpu: 44, memory: 66, network: 21, requests: 155 },
                  { timestamp: "14:29", cpu: 46, memory: 68, network: 24, requests: 170 },
                  { timestamp: "14:30", cpu: 45, memory: 68, network: 23, requests: 160 },
                ],
              },
              {
                id: "2",
                name: "数据处理服务",
                status: "warning",
                uptime: "2天 5小时 15分钟",
                cpu: 78,
                memory: 85,
                network: 45,
                storage: 67,
                instances: 2,
                healthCheck: "unhealthy",
                lastRestart: "2024-01-13 08:45:00",
                runningOn: ["开发环境"],
                logs: [
                  { timestamp: "2024-01-15 14:30:00", level: "error", message: "数据处理任务失败: timeout" },
                  { timestamp: "2024-01-15 14:29:30", level: "warn", message: "内存使用率过高: 85%" },
                  { timestamp: "2024-01-15 14:29:00", level: "info", message: "开始处理数据批次: batch_001" },
                ],
                metrics: [
                  { timestamp: "14:25", cpu: 75, memory: 82, network: 42, requests: 80 },
                  { timestamp: "14:26", cpu: 78, memory: 84, network: 44, requests: 85 },
                  { timestamp: "14:27", cpu: 80, memory: 86, network: 46, requests: 90 },
                  { timestamp: "14:28", cpu: 76, memory: 83, network: 43, requests: 82 },
                  { timestamp: "14:29", cpu: 79, memory: 85, network: 45, requests: 88 },
                  { timestamp: "14:30", cpu: 78, memory: 85, network: 45, requests: 85 },
                ],
              },
              {
                id: "3",
                name: "API网关",
                status: "running",
                uptime: "15天 3小时 45分钟",
                cpu: 32,
                memory: 45,
                network: 67,
                storage: 23,
                instances: 5,
                healthCheck: "healthy",
                lastRestart: "2023-12-31 10:30:00",
                runningOn: ["生产环境集群", "开发环境"],
                logs: [
                  { timestamp: "2024-01-15 14:30:00", level: "info", message: "API请求处理: POST /api/auth/login" },
                  { timestamp: "2024-01-15 14:29:45", level: "info", message: "负载均衡器状态正常" },
                  { timestamp: "2024-01-15 14:29:30", level: "info", message: "处理API请求: GET /api/status" },
                ],
                metrics: [
                  { timestamp: "14:25", cpu: 30, memory: 42, network: 65, requests: 320 },
                  { timestamp: "14:26", cpu: 32, memory: 44, network: 66, requests: 340 },
                  { timestamp: "14:27", cpu: 35, memory: 46, network: 68, requests: 360 },
                  { timestamp: "14:28", cpu: 31, memory: 43, network: 64, requests: 330 },
                  { timestamp: "14:29", cpu: 33, memory: 45, network: 67, requests: 350 },
                  { timestamp: "14:30", cpu: 32, memory: 45, network: 67, requests: 345 },
                ],
              },
            ],
          })
        }
      },

      fetchResources: async () => {
        const { setLoadingState, setError } = get()
        try {
          setLoadingState("resources", true)
          setError("resources", null)

          const response = await fetch("/api/resources")
          if (!response.ok) throw new Error("获取资源列表失败")

          const resources = await response.json()
          set({ resources })
        } catch (error) {
          setError("resources", error instanceof Error ? error.message : "未知错误")
        } finally {
          setLoadingState("resources", false)
        }
      },

      fetchApplications: async () => {
        const { setLoadingState, setError } = get()
        try {
          setLoadingState("applications", true)
          setError("applications", null)

          const response = await fetch("/api/applications")
          if (!response.ok) throw new Error("获取应用列表失败")

          const applications = await response.json()
          set({ applications })
        } catch (error) {
          setError("applications", error instanceof Error ? error.message : "未知错误")
        } finally {
          setLoadingState("applications", false)
        }
      },

      fetchApplicationStatuses: async () => {
        const { setLoadingState, setError } = get()
        try {
          setLoadingState("statuses", true)
          setError("statuses", null)

          const response = await fetch("/api/status")
          if (!response.ok) throw new Error("获取状态信息失败")

          const statuses = await response.json()
          set({ applicationStatuses: statuses })
        } catch (error) {
          setError("statuses", error instanceof Error ? error.message : "未知错误")
        } finally {
          setLoadingState("statuses", false)
        }
      },

      refreshData: async () => {
        const { fetchResources, fetchApplications, fetchApplicationStatuses } = get()
        await Promise.all([fetchResources(), fetchApplications(), fetchApplicationStatuses()])
      },

      createInitialAdmin: async ({ username, password }) => {
        const trimmedName = username.trim()
        if (!trimmedName) {
          throw new Error("管理员用户名不能为空")
        }
        if (password.length < 6) {
          throw new Error("密码至少需要 6 位")
        }
        if (get().setupCompleted) {
          throw new Error("管理员已配置完成")
        }
        const passwordHash = await hashPassword(password)
        const adminUser: UserAccount = {
          id: generateId(),
          username: trimmedName,
          role: "admin",
          passwordHash,
        }
        set({
          users: [adminUser],
          setupCompleted: true,
          currentUser: toPublicUser(adminUser),
        })
      },

      addUserAccount: async ({ username, password, role = "user" }) => {
        const trimmedName = username.trim()
        const state = get()
        if (!state.setupCompleted) {
          throw new Error("请先完成管理员配置")
        }
        if (state.currentUser?.role !== "admin") {
          throw new Error("只有管理员可以添加用户")
        }
        if (!trimmedName) {
          throw new Error("用户名不能为空")
        }
        if (password.length < 6) {
          throw new Error("密码至少需要 6 位")
        }
        if (state.users.some((user) => user.username === trimmedName)) {
          throw new Error("用户名已存在")
        }
        const passwordHash = await hashPassword(password)
        const newUser: UserAccount = {
          id: generateId(),
          username: trimmedName,
          role,
          passwordHash,
        }
        set({
          users: [...state.users, newUser],
        })
      },

      deleteUserAccount: async (id: string) => {
        const state = get()
        if (state.currentUser?.role !== "admin") {
          throw new Error("只有管理员可以删除用户")
        }
        const target = state.users.find((user) => user.id === id)
        if (!target) {
          return
        }
        if (target.role === "admin") {
          const adminCount = state.users.filter((user) => user.role === "admin").length
          if (adminCount <= 1) {
            throw new Error("至少需要保留一个管理员账号")
          }
        }
        set({
          users: state.users.filter((user) => user.id !== id),
          currentUser: state.currentUser?.id === id ? null : state.currentUser,
        })
      },

      login: async (username, password) => {
        const state = get()
        if (!state.setupCompleted) {
          throw new Error("请先完成管理员配置")
        }
        const trimmedName = username.trim()
        if (!trimmedName) {
          throw new Error("用户名不能为空")
        }
        const target = state.users.find((user) => user.username === trimmedName)
        if (!target) {
          throw new Error("账号不存在")
        }
        const matched = await verifyPassword(password, target.passwordHash)
        if (!matched) {
          throw new Error("密码错误")
        }
        set({
          currentUser: toPublicUser(target),
        })
      },

      logout: () => {
        set({ currentUser: null })
      },
    }),
    {
      name: "iarnet-storage",
      partialize: (state) => ({
        resources: state.resources,
        applications: state.applications,
        applicationStatuses: state.applicationStatuses,
        setupCompleted: state.setupCompleted,
        users: state.users,
        currentUser: state.currentUser,
      }),
    },
  ),
)
