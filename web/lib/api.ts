import { Application, ApplicationStats, CreateDirectoryResponse, CreateFileResponse, DeleteDirectoryResponse, DeleteFileResponse, GetApplicationActorsResponse, GetApplicationLogsResponse, GetApplicationsResponse, GetComponentLogsResponse, GetDAGResponse, GetFileContentResponse, GetFileTreeResponse, GetRunnerEnvironmentsResponse, SaveFileResponse } from "./model"

// API 客户端工具函数
const API_BASE = "/api"

export class APIError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = "APIError"
  }
}

async function apiRequest<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const url = `${API_BASE}${endpoint}`

  const response = await fetch(url, {
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
    ...options,
  })

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

  if (!response.ok) {
    throw new APIError(response.status, data.message || data.error || "API request failed")
  }

  // 处理后端标准响应格式 {code, message, data}
  if (data.code !== undefined) {
    if (data.code < 200 || data.code >= 300) {
      throw new APIError(data.code, data.message || data.error || "API request failed")
    }
    return data.data
  }

  // 兼容其他响应格式
  return data.data || data
}

// 资源管理 API
import type {
  GetResourceCapacityResponse,
  GetResourceProvidersResponse,
  GetResourceProviderInfoResponse,
  GetResourceProviderCapacityResponse,
  RegisterResourceProviderRequest,
  RegisterResourceProviderResponse,
  UnregisterResourceProviderResponse,
  TestResourceProviderRequest,
  TestResourceProviderResponse,
  UpdateResourceProviderRequest,
  UpdateResourceProviderResponse,
  GetDiscoveredNodesResponse,
} from "./model"

export const resourcesAPI = {
  // 获取资源容量
  getCapacity: () => apiRequest<GetResourceCapacityResponse>("/resource/capacity"),

  // 获取资源提供者列表
  getProviders: () => apiRequest<GetResourceProvidersResponse>("/resource/provider"),

  // 获取资源提供者信息
  getProviderInfo: (id: string) =>
    apiRequest<GetResourceProviderInfoResponse>(`/resource/provider/${id}/info`),

  // 获取资源提供者容量
  getProviderCapacity: (id: string) =>
    apiRequest<GetResourceProviderCapacityResponse>(`/resource/provider/${id}/capacity`),

  // 注册资源提供者
  registerProvider: (request: RegisterResourceProviderRequest) =>
    apiRequest<RegisterResourceProviderResponse>("/resource/provider", {
      method: "POST",
      body: JSON.stringify(request),
    }),

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
