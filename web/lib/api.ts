import { Application, ApplicationStats, CreateDirectoryResponse, CreateFileResponse, DeleteDirectoryResponse, DeleteFileResponse, GetApplicationLogsParsedResponse, GetApplicationLogsResponse, GetApplicationsResponse, GetCodeBrowserStatusResponse, GetDAGResponse, GetFileContentResponse, GetFileTreeResponse, GetRunnerEnvironmentsResponse, SaveFileResponse, StartCodeBrowserResponse } from "./model"

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
}

// 应用管理 API
export const applicationsAPI = {
  getAll: () => apiRequest<GetApplicationsResponse>("/application/apps"),
  getStats: () => apiRequest<ApplicationStats>("/application/stats"),
  getById: (id: string) => apiRequest<Application>(`/application/apps/${id}`),
  getLogs: (id: string, lines?: number) => apiRequest<GetApplicationLogsResponse>(`/application/apps/${id}/logs${lines ? `?lines=${lines}` : ''}`),
  getLogsParsed: (id: string, lines?: number) => apiRequest<GetApplicationLogsParsedResponse>(`/application/apps/${id}/logs/parsed${lines ? `?lines=${lines}` : ''}`),
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
  // 代码浏览器相关API
  startCodeBrowser: (id: string) =>
    apiRequest<StartCodeBrowserResponse>(`/application/apps/${id}/code-browser`, {
      method: "POST",
    }),
  stopCodeBrowser: (id: string) =>
    apiRequest(`/application/apps/${id}/code-browser`, {
      method: "DELETE",
    }),
  getCodeBrowserStatus: (id: string) =>
    apiRequest<GetCodeBrowserStatusResponse>(`/application/apps/${id}/code-browser/status`),
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
  getAppDAG: (id: string) =>
    apiRequest<GetDAGResponse>(`/application/apps/${id}/dag`),
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
