import { Application, ApplicationStats, CreateDirectoryResponse, CreateFileResponse, DeleteDirectoryResponse, DeleteFileResponse, GetApplicationActorsResponse, GetApplicationLogsResponse, GetApplicationsResponse, GetComponentLogsResponse, GetDAGResponse, GetFileContentResponse, GetFileTreeResponse, GetRunnerEnvironmentsResponse, SaveFileResponse } from "./model"

// 错误消息映射：将后端返回的英文错误标识转换为中文提示
export function getErrorMessage(error: string | undefined | null): string {
  if (!error) {
    return "操作失败"
  }

  const errorMap: Record<string, string> = {
    "insufficient_permissions": "权限不足",
    "cannot_modify_initial_super_admin_password": "无法修改配置的初始超级管理员账户的密码，只能由该账户自己修改",
    "cannot_delete_initial_super_admin": "无法删除配置的初始超级管理员账户",
    "forbidden": "权限不足",
    "unauthorized": "未授权",
    "authentication required": "需要登录",
    "bad request": "请求参数错误",
    "not found": "资源不存在",
    "internal server error": "服务器内部错误",
  }

  // 先尝试精确匹配
  if (errorMap[error]) {
    return errorMap[error]
  }

  // 尝试不区分大小写匹配
  const lowerError = error.toLowerCase()
  if (errorMap[lowerError]) {
    return errorMap[lowerError]
  }

  // 如果都不匹配，返回原始错误消息（可能是中文或其他格式）
  return error
}

// 在客户端显示 toast 提示的辅助函数
function showUnauthorizedToast() {
  // 只在客户端环境显示 toast
  if (typeof window !== "undefined") {
    // 如果当前在登录页面，不显示"需要重新登录"的提示
    // 因为登录页面会自己处理登录错误
    if (window.location.pathname === "/login" || window.location.pathname.startsWith("/login")) {
      return
    }
    // 使用动态导入避免在服务端构建时出错
    import("sonner").then(({ toast }) => {
      toast.error("当前尚未登录或需要重新登录", {
        description: "请重新登录以继续使用",
      })
    }).catch(() => {
      // 静默忽略导入错误（可能在服务端环境）
    })
  }
}

// API 客户端工具函数
const API_BASE = "/api"
const TOKEN_KEY = "iarnet_auth_token"

// Token 管理函数
export const tokenManager = {
  getToken: (): string | null => {
    if (typeof window === "undefined") return null
    return localStorage.getItem(TOKEN_KEY)
  },
  setToken: (token: string): void => {
    if (typeof window === "undefined") return
    localStorage.setItem(TOKEN_KEY, token)
  },
  removeToken: (): void => {
    if (typeof window === "undefined") return
    localStorage.removeItem(TOKEN_KEY)
  },
  // 检查token是否过期
  isTokenExpired: (token: string | null): boolean => {
    if (!token) return true
    
    try {
      // JWT token格式：header.payload.signature
      const parts = token.split('.')
      if (parts.length !== 3) return true
      
      // 解析payload（base64解码）
      const payload = JSON.parse(atob(parts[1]))
      
      // 检查exp字段（过期时间，Unix时间戳）
      if (payload.exp) {
        const expirationTime = payload.exp * 1000 // 转换为毫秒
        const currentTime = Date.now()
        return currentTime >= expirationTime
      }
      
      // 如果没有exp字段，认为token无效
      return true
    } catch (error) {
      // 解析失败，认为token无效
      console.error("Failed to parse token:", error)
      return true
    }
  },
  // 获取token过期时间（毫秒）
  getTokenExpirationTime: (token: string | null): number | null => {
    if (!token) return null
    
    try {
      const parts = token.split('.')
      if (parts.length !== 3) return null
      
      const payload = JSON.parse(atob(parts[1]))
      if (payload.exp) {
        return payload.exp * 1000 // 转换为毫秒
      }
      
      return null
    } catch (error) {
      console.error("Failed to parse token:", error)
      return null
    }
  },
}

export class APIError extends Error {
  constructor(
    public status: number,
    message: string,
    public data?: any,
  ) {
    super(message)
    this.name = "APIError"
  }
}

// 创建带超时的 fetch 请求
async function fetchWithTimeout(url: string, options: RequestInit = {}, timeout: number = 10000): Promise<Response> {
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), timeout)

  try {
    const response = await fetch(url, {
      ...options,
      signal: controller.signal,
    })
    clearTimeout(timeoutId)
    return response
  } catch (error) {
    clearTimeout(timeoutId)
    if (error instanceof Error && error.name === 'AbortError') {
      throw new APIError(408, '请求超时')
    }
    throw error
  }
}

export async function apiRequest<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const url = `${API_BASE}${endpoint}`

  // 获取 token 并检查是否过期
  const token = tokenManager.getToken()
  if (token && tokenManager.isTokenExpired(token)) {
    // Token已过期，清除token并显示提示
    tokenManager.removeToken()
    showUnauthorizedToast()
    throw new APIError(401, "会话已过期，请重新登录")
  }

  const authHeaders: HeadersInit = token
    ? { Authorization: `Bearer ${token}` }
    : {}

  // 如果是 FormData，不设置 Content-Type，让浏览器自动设置（包括 boundary）
  const isFormData = options.body instanceof FormData
  const headers: HeadersInit = isFormData
    ? { ...authHeaders, ...options.headers }
    : {
        "Content-Type": "application/json",
        ...authHeaders,
        ...options.headers,
      }

  const response = await fetchWithTimeout(url, {
    headers,
    ...options,
  }, 10000) // 10 秒超时

  // 检查响应是否有内容
  const contentType = response.headers.get("content-type")
  const hasJsonContent = contentType && contentType.includes("application/json")
  
  let data: any = {}
  
  // 只有在响应有内容且是 JSON 格式时才解析
  if (hasJsonContent) {
    try {
      const text = await response.text()
      if (text.trim()) {
        data = JSON.parse(text)
      }
    } catch (error) {
      // JSON 解析失败，尝试从状态码获取错误信息
      if (!response.ok) {
        throw new APIError(response.status, `请求失败: ${response.statusText}`)
      }
      throw new APIError(response.status, "响应格式错误")
    }
  } else if (!response.ok) {
    // 非 JSON 响应且状态码不是成功，直接抛出错误
    throw new APIError(response.status, response.statusText || "请求失败")
  }

  // 只记录非404错误的日志，404通常表示资源不存在，这在某些情况下是正常的
  // if (response.status !== 404) {
  //   console.log("API Response:", response.status, data)
  // }

  const payload =
    data && typeof data === "object"
      ? ("data" in data ? (data as Record<string, any>).data : data)
      : undefined

  // 处理 401 未授权错误，清除 token 并显示提示
  if (response.status === 401) {
    tokenManager.removeToken()
    // 在客户端显示 toast 提示
    showUnauthorizedToast()
    throw new APIError(
      response.status,
      (data && (data.message || data.error)) || "认证失败，请重新登录",
      payload,
    )
  }

  if (!response.ok) {
    // 优先使用 error 字段（包含具体错误信息），如果没有则使用 message 字段
    const errorMsg = data && (data.error || data.message)
    throw new APIError(
      response.status,
      errorMsg || "API request failed",
      payload,
    )
  }

  // 处理后端标准响应格式 {code, message, data}
  if (data.code !== undefined) {
    // 确保 code 是数字类型
    const code = typeof data.code === 'number' ? data.code : parseInt(String(data.code), 10)
    // 业务码 401 表示未授权（即使 HTTP 状态是 200）
    if (code === 401) {
      tokenManager.removeToken()
      showUnauthorizedToast()
      // 优先使用 error 字段（包含具体错误信息），如果没有则使用 message 字段
      throw new APIError(code, data.error || data.message || "认证失败，请重新登录", payload)
    }
    if (isNaN(code) || code < 200 || code >= 300) {
      // 优先使用 error 字段（包含具体错误信息），如果没有则使用 message 字段
      const errorMsg = data.error || data.message
      throw new APIError(code, errorMsg || "API request failed", payload)
    }
    // 确保返回 data.data，如果 data.data 存在
    if (data.data !== undefined) {
      return data.data
    }
    // 如果 data.data 不存在，返回整个 data 对象（向后兼容）
    return data
  }

  // 兼容其他响应格式：如果 data.data 存在，返回它；否则返回 data
  if (data.data !== undefined) {
    return data.data
  }
  return data
}

// 资源管理 API
import type {
  GetResourceCapacityResponse,
  GetResourceProvidersResponse,
  GetResourceProviderInfoResponse,
  GetResourceProviderCapacityResponse,
  GetResourceProviderUsageResponse,
  RegisterResourceProviderRequest,
  RegisterResourceProviderResponse,
  UnregisterResourceProviderResponse,
  TestResourceProviderRequest,
  TestResourceProviderResponse,
  UpdateResourceProviderRequest,
  UpdateResourceProviderResponse,
  GetDiscoveredNodesResponse,
  GetNodeInfoResponse,
  BatchRegisterResourceProviderResponse,
} from "./model"

export const resourcesAPI = {
  // 获取资源容量
  getCapacity: () => apiRequest<GetResourceCapacityResponse>("/resource/capacity"),

  // 获取当前节点与域信息
  getNodeInfo: () => apiRequest<GetNodeInfoResponse>("/resource/node/info"),

  // 获取资源提供者列表
  getProviders: () => apiRequest<GetResourceProvidersResponse>("/resource/provider"),

  // 获取资源提供者信息
  getProviderInfo: (id: string) =>
    apiRequest<GetResourceProviderInfoResponse>(`/resource/provider/${id}/info`),

  // 获取资源提供者容量
  getProviderCapacity: (id: string) =>
    apiRequest<GetResourceProviderCapacityResponse>(`/resource/provider/${id}/capacity`),

  // 获取资源提供者实时使用情况
  getProviderUsage: (id: string) =>
    apiRequest<GetResourceProviderUsageResponse>(`/resource/provider/${id}/usage`),

  // 注册资源提供者
  registerProvider: (request: RegisterResourceProviderRequest) =>
    apiRequest<RegisterResourceProviderResponse>("/resource/provider", {
      method: "POST",
      body: JSON.stringify(request),
    }),

  // 批量注册资源提供者
  batchRegisterProvider: (file: File) => {
    const formData = new FormData()
    formData.append("file", file)
    return apiRequest<BatchRegisterResourceProviderResponse>("/resource/provider/batch", {
      method: "POST",
      body: formData,
    })
  },

  // 注销资源提供者
  unregisterProvider: (id: string) =>
    apiRequest<UnregisterResourceProviderResponse>(`/resource/provider/${id}`, {
      method: "DELETE",
    }),

  // 更新资源提供者
  updateProvider: (id: string, request: UpdateResourceProviderRequest) =>
    apiRequest<UpdateResourceProviderResponse>(`/resource/provider/${id}`, {
      method: "PUT",
      body: JSON.stringify(request),
    }),

  // 测试资源提供者连接
  testProvider: (request: TestResourceProviderRequest) =>
    apiRequest<TestResourceProviderResponse>("/resource/provider/test", {
      method: "POST",
      body: JSON.stringify(request),
    }),

  // 获取发现的节点列表（通过 gossip）
  getDiscoveredNodes: () =>
    apiRequest<GetDiscoveredNodesResponse>("/resource/discovery/nodes", {
      method: "GET",
    }),
}

// 应用管理 API
export interface GetApplicationLogsParams {
  limit?: number
  offset?: number
  level?: string
  startTime?: string
  endTime?: string
}

export interface GetComponentLogsParams {
  limit?: number
  offset?: number
  level?: string
  startTime?: string
  endTime?: string
  /**
   * @deprecated 请使用 limit
   */
  lines?: number
}

export const applicationsAPI = {
  getAll: () => apiRequest<GetApplicationsResponse>("/application/apps"),
  getStats: () => apiRequest<ApplicationStats>("/application/stats"),
  getById: (id: string) => apiRequest<Application>(`/application/apps/${id}`),
  getLogs: (id: string, params?: GetApplicationLogsParams) => {
    const searchParams = new URLSearchParams()
    if (params?.limit !== undefined) {
      searchParams.set("limit", params.limit.toString())
    }
    if (params?.offset !== undefined) {
      searchParams.set("offset", params.offset.toString())
    }
    if (params?.level) {
      searchParams.set("level", params.level)
    }
    if (params?.startTime) {
      searchParams.set("start_time", params.startTime)
    }
    if (params?.endTime) {
      searchParams.set("end_time", params.endTime)
    }
    const query = searchParams.toString()
    const endpoint = query ? `/application/apps/${id}/logs?${query}` : `/application/apps/${id}/logs`
    return apiRequest<GetApplicationLogsResponse>(endpoint)
  },
  create: (app: any) =>
    apiRequest("/application/apps", {
      method: "POST",
      body: JSON.stringify(app),
    }),
  update: (id: string, app: any) =>
    apiRequest(`/application/apps/${id}`, {
      method: "PUT",
      body: JSON.stringify(app),
    }),
  delete: (id: string) =>
    apiRequest(`/application/apps/${id}`, {
      method: "DELETE",
    }),
  run: (id: string) =>
    apiRequest<Application>(`/application/apps/${id}/run`, {
      method: "POST",
    }),
  stop: (id: string) =>
    apiRequest<Application>(`/application/apps/${id}/stop`, {
      method: "POST",
    }),
  // 文件管理相关API
  getFileTree: (id: string, path: string = '') =>
    apiRequest<GetFileTreeResponse>(`/application/apps/${id}/files${path ? `?path=${encodeURIComponent(path)}` : ''}`),
  getFileContent: (id: string, filePath: string) =>
    apiRequest<GetFileContentResponse>(`/application/apps/${id}/files/content?path=${encodeURIComponent(filePath)}`),
  saveFileContent: (id: string, filePath: string, content: string) =>
    apiRequest<SaveFileResponse>(`/application/apps/${id}/files/content?path=${encodeURIComponent(filePath)}`, {
      method: "PUT",
      body: JSON.stringify({
        content,
      }),
    }),
  createFile: (id: string, filePath: string) =>
    apiRequest<CreateFileResponse>(`/application/apps/${id}/files`, {
      method: "POST",
      body: JSON.stringify({ filePath }),
    }),
  deleteFile: (id: string, filePath: string) =>
    apiRequest<DeleteFileResponse>(`/application/apps/${id}/files`, {
      method: "DELETE",
      body: JSON.stringify({ filePath }),
    }),
  createDirectory: (id: string, dirPath: string) =>
    apiRequest<CreateDirectoryResponse>(`/application/apps/${id}/directories`, {
      method: "POST",
      body: JSON.stringify({ dirPath }),
    }),
  deleteDirectory: (id: string, dirPath: string) =>
    apiRequest<DeleteDirectoryResponse>(`/application/apps/${id}/directories`, {
      method: "DELETE",
      body: JSON.stringify({ dirPath }),
    }),
  // Actor组件相关API
  getAppDAG: (id: string, sessionId?: string) =>
    apiRequest<GetDAGResponse>(
      `/application/apps/${id}/dag${sessionId ? `?session_id=${encodeURIComponent(sessionId)}` : ""}`
    ),
  analyzeApplication: (id: string) =>
    apiRequest(`/application/apps/${id}/analyze`, {
      method: "POST",
    }),
  deployComponents: (id: string) =>
    apiRequest(`/application/apps/${id}/deploy-components`, {
      method: "POST",
    }),
  getRunnerEnvironments: () =>
    apiRequest<GetRunnerEnvironmentsResponse>("/application/runner-environments"),
  getComponents: (id: string) =>
    apiRequest(`/application/apps/${id}/components`),
  getActors: (id: string) =>
    apiRequest<GetApplicationActorsResponse>(`/application/apps/${id}/actors`),
  getComponentLogs: async (_id: string, componentId: string, params?: GetComponentLogsParams): Promise<GetComponentLogsResponse> => {
    const searchParams = new URLSearchParams()
    const limit = params?.limit ?? params?.lines
    if (limit !== undefined) {
      searchParams.set("limit", limit.toString())
    }
    if (params?.offset !== undefined) {
      searchParams.set("offset", params.offset.toString())
    }
    if (params?.level) {
      searchParams.set("level", params.level)
    }
    if (params?.startTime) {
      searchParams.set("start_time", params.startTime)
    }
    if (params?.endTime) {
      searchParams.set("end_time", params.endTime)
    }
    const query = searchParams.toString()
    const endpoint = query
      ? `/resource/components/${componentId}/logs?${query}`
      : `/resource/components/${componentId}/logs`
    const raw = await apiRequest<any>(endpoint)
    const logs = Array.isArray(raw.logs) ? raw.logs : []
    return {
      componentId: raw.component_id ?? raw.componentId ?? componentId,
      logs,
      total: raw.total ?? raw.total_lines ?? raw.totalLines ?? logs.length,
      hasMore: Boolean(raw.has_more ?? raw.hasMore ?? false),
    }
  },
}

// Actor组件管理 API
export const componentsAPI = {
  start: (appId: string, componentId: string) =>
    apiRequest(`/application/apps/${appId}/components/${componentId}/start`, {
      method: "POST",
    }),
  stop: (appId: string, componentId: string) =>
    apiRequest(`/application/apps/${appId}/components/${componentId}/stop`, {
      method: "POST",
    }),
  getStatus: (appId: string, componentId: string) =>
    apiRequest(`/application/apps/${appId}/components/${componentId}/status`),
  getLogs: (appId: string, componentId: string, lines?: number) =>
    apiRequest(`/application/apps/${appId}/components/${componentId}/logs${lines ? `?lines=${lines}` : ''}`),
  getResourceUsage: (appId: string, componentId: string) =>
    apiRequest(`/application/apps/${appId}/components/${componentId}/resource-usage`),
}

// 状态监控 API
export const statusAPI = {
  getAll: () => apiRequest("/status"),
  restart: (id: string) =>
    apiRequest(`/status/${id}/restart`, {
      method: "POST",
    }),
}

// 认证 API
export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  username: string
  token: string
  role?: string
}

export interface GetCurrentUserResponse {
  username: string
  role: string
}

export interface ChangePasswordRequest {
  oldPassword: string
  newPassword: string
}

export const authAPI = {
  login: async (request: LoginRequest): Promise<LoginResponse> => {
    // 导入密码哈希函数
    const { hashPassword } = await import("@/lib/utils")
    
    // 对密码进行哈希处理，避免明文传输
    const hashedPassword = await hashPassword(request.password)
    
    // 发送哈希后的密码
    const response = await apiRequest<LoginResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify({
        username: request.username,
        password: hashedPassword, // 发送哈希后的密码
      }),
    })
    // 保存 token
    if (response.token) {
      tokenManager.setToken(response.token)
    }
    return response
  },
  logout: async (): Promise<void> => {
    try {
      // 发送退出登录请求到后端
      await apiRequest<void>("/auth/logout", {
        method: "POST",
      })
    } catch (error) {
      // 即使请求失败，也清除本地token
      console.error("Logout request failed:", error)
    } finally {
      // 无论请求成功与否，都清除本地token
      tokenManager.removeToken()
    }
  },
  changePassword: async (request: ChangePasswordRequest): Promise<void> => {
    // 导入密码哈希函数
    const { hashPassword } = await import("@/lib/utils")
    
    // 对密码进行哈希处理，避免明文传输
    const hashedOldPassword = await hashPassword(request.oldPassword)
    const hashedNewPassword = await hashPassword(request.newPassword)
    
    // 发送哈希后的密码
    await apiRequest<void>("/auth/change-password", {
      method: "POST",
      body: JSON.stringify({
        old_password: hashedOldPassword,
        new_password: hashedNewPassword,
      }),
    })
  },
  getCurrentUser: () =>
    apiRequest<GetCurrentUserResponse>("/auth/me"),
}

// 验证码 API
export interface CaptchaResponse {
  captchaId: string
  imageUrl: string
  expiresAt: number // 过期时间戳（毫秒）
}

export interface VerifyCaptchaRequest {
  captchaId: string
  answer: string
}

export interface VerifyCaptchaResponse {
  valid: boolean
  message: string
}

// 用户管理 API
export interface UserInfo {
  name: string
  role: string
  locked: boolean
  locked_until?: string
  failed_count: number
}

export interface GetUsersResponse {
  users: UserInfo[]
  total: number
}

export interface CreateUserRequest {
  name: string
  password: string
  role: string
}

export interface UpdateUserRequest {
  password?: string
  role?: string
}

// 恢复 API
export interface RecoveryUnlockRequest {
  username: string
  password: string
}

export interface RecoveryUnlockResponse {
  message: string
  username: string
}

export const recoveryAPI = {
  // 紧急解锁超级管理员账户（需要配置文件中的密码）
  unlockSuperAdmin: async (request: RecoveryUnlockRequest) => {
    // 导入密码哈希函数
    const { hashPassword } = await import("@/lib/utils")
    
    // 对密码进行哈希处理，避免明文传输
    const hashedPassword = await hashPassword(request.password)
    
    // 发送哈希后的密码
    return apiRequest<RecoveryUnlockResponse>("/auth/recovery/unlock-super-admin", {
      method: "POST",
      body: JSON.stringify({
        ...request,
        password: hashedPassword,
      }),
    })
  },
}

export const usersAPI = {
  // 获取用户列表（仅超级管理员）
  getUsers: () => apiRequest<GetUsersResponse>("/auth/users"),
  
  // 获取单个用户信息（仅超级管理员）
  getUser: (username: string) => apiRequest<UserInfo>(`/auth/users/${username}`),
  
  // 创建用户（仅超级管理员）
  createUser: async (request: CreateUserRequest) => {
    // 导入密码哈希函数
    const { hashPassword } = await import("@/lib/utils")
    
    // 对密码进行哈希处理，避免明文传输
    const hashedPassword = await hashPassword(request.password)
    
    // 发送哈希后的密码
    return apiRequest<UserInfo>("/auth/users", {
      method: "POST",
      body: JSON.stringify({
        ...request,
        password: hashedPassword,
      }),
    })
  },
  
  // 更新用户（仅超级管理员）
  updateUser: async (username: string, request: UpdateUserRequest) => {
    // 如果请求中包含密码，对密码进行哈希处理
    const requestBody: UpdateUserRequest = { ...request }
    if (request.password) {
      // 导入密码哈希函数
      const { hashPassword } = await import("@/lib/utils")
      // 对密码进行哈希处理，避免明文传输
      requestBody.password = await hashPassword(request.password)
    }
    
    // 发送请求
    return apiRequest<UserInfo>(`/auth/users/${username}`, {
      method: "PUT",
      body: JSON.stringify(requestBody),
    })
  },
  
  // 删除用户（仅超级管理员）
  deleteUser: (username: string) =>
    apiRequest<void>(`/auth/users/${username}`, {
      method: "DELETE",
    }),
  
  // 解锁用户（仅超级管理员）
  unlockUser: (username: string) =>
    apiRequest<void>(`/auth/users/${username}/unlock`, {
      method: "POST",
    }),
}

export const captchaAPI = {
  // 获取验证码图片 URL 和 ID
  getCaptcha: async (): Promise<CaptchaResponse> => {
    const response = await fetch("/api/captcha", {
      method: "GET",
      cache: "no-store",
    })
    if (!response.ok) {
      throw new Error("Failed to fetch captcha")
    }
    const captchaId = response.headers.get("X-Captcha-Id") || ""
    const expiresAtHeader = response.headers.get("X-Captcha-Expires-At")
    const expiresAt = expiresAtHeader ? parseInt(expiresAtHeader, 10) : Date.now() + 2 * 60 * 1000
    const blob = await response.blob()
    const imageUrl = URL.createObjectURL(blob)
    return {
      captchaId,
      imageUrl,
      expiresAt,
    }
  },
  
  // 验证验证码
  verifyCaptcha: (request: VerifyCaptchaRequest) =>
    apiRequest<VerifyCaptchaResponse>("/captcha", {
      method: "POST",
      body: JSON.stringify(request),
    }),
}

// Audit API - 系统日志
export interface GetAuditLogsParams {
  limit?: number
  offset?: number
  level?: string
  startTime?: string
  endTime?: string
}

export interface AuditLogEntry {
  timestamp: number // Unix 纳秒时间戳
  level: number // 日志级别 (0-7)
  message: string
  fields?: Array<{ key: string; value: string }>
  caller?: {
    file: string
    line: number
    function: string
  }
}

export interface GetAuditLogsResponse {
  logs: AuditLogEntry[]
  total: number
}

// Audit API - 操作日志
export interface GetOperationsParams {
  limit?: number
  offset?: number
  startTime?: string
  endTime?: string
  user?: string
  operation?: string
  resourceId?: string
}

export interface OperationLogEntry {
  id: string
  user: string
  operation: string
  resource_id: string
  resource_type: string
  action: string
  before?: Record<string, any>
  after?: Record<string, any>
  timestamp: string // ISO 8601 格式时间字符串
  ip?: string
}

export interface GetOperationsResponse {
  logs: OperationLogEntry[]
  total: number
  has_more?: boolean
}

export const auditAPI = {
  getLogs: (params?: GetAuditLogsParams) => {
    const searchParams = new URLSearchParams()
    if (params?.limit !== undefined) {
      searchParams.set("limit", params.limit.toString())
    }
    if (params?.offset !== undefined) {
      searchParams.set("offset", params.offset.toString())
    }
    if (params?.level) {
      searchParams.set("level", params.level)
    }
    if (params?.startTime) {
      searchParams.set("start_time", params.startTime)
    }
    if (params?.endTime) {
      searchParams.set("end_time", params.endTime)
    }
    const query = searchParams.toString()
    const endpoint = query ? `/audit/logs?${query}` : "/audit/logs"
    return apiRequest<GetAuditLogsResponse>(endpoint)
  },
  getOperations: (params?: GetOperationsParams) => {
    const searchParams = new URLSearchParams()
    if (params?.limit !== undefined) {
      searchParams.set("limit", params.limit.toString())
    }
    if (params?.offset !== undefined) {
      searchParams.set("offset", params.offset.toString())
    }
    if (params?.startTime) {
      searchParams.set("start_time", params.startTime)
    }
    if (params?.endTime) {
      searchParams.set("end_time", params.endTime)
    }
    if (params?.user) {
      searchParams.set("user", params.user)
    }
    if (params?.operation) {
      searchParams.set("operation", params.operation)
    }
    if (params?.resourceId) {
      searchParams.set("resource_id", params.resourceId)
    }
    const query = searchParams.toString()
    const endpoint = query ? `/audit/operations?${query}` : "/audit/operations"
    return apiRequest<GetOperationsResponse>(endpoint)
  },
}
