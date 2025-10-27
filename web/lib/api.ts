import { Application, CreateDirectoryResponse, CreateFileResponse, DeleteDirectoryResponse, DeleteFileResponse, GetApplicationLogsParsedResponse, GetApplicationLogsResponse, GetApplicationsResponse, GetCodeBrowserStatusResponse, GetDAGResponse, GetFileContentResponse, GetFileTreeResponse, GetRunnerEnvironmentsResponse, SaveFileResponse, StartCodeBrowserResponse } from "./model"

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

  const data = await response.json()

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
      throw new APIError(data.code, data.message || "API request failed")
    }
    return data.data
  }

  // 兼容其他响应格式
  return data.data || data
}

// 资源管理 API
export const resourcesAPI = {
  getAll: () => apiRequest("/resources"),
  getCapacity: () => apiRequest("/resource/capacity"),
  getProviders: () => apiRequest("/resource/providers"),
  create: (resource: any) =>
    apiRequest("/resource/providers", {
      method: "POST",
      body: JSON.stringify(resource),
    }),
  update: (id: string, resource: any) =>
    apiRequest(`/resources/${id}`, {
      method: "PUT",
      body: JSON.stringify(resource),
    }),
  delete: (id: string) =>
    apiRequest(`/resources/${id}`, {
      method: "DELETE",
    }),
}

// 应用管理 API
export const applicationsAPI = {
  getAll: () => apiRequest<GetApplicationsResponse>("/application/apps"),
  getStats: () => apiRequest("/application/stats"),
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
