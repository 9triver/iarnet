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

  if (!response.ok) {
    throw new APIError(response.status, data.error || "API request failed")
  }

  return data.data
}

// 资源管理 API
export const resourcesAPI = {
  getAll: () => apiRequest("/resources"),
  getCapacity: () => apiRequest("/resource/capacity"),
  getProviders: () => apiRequest("/resource/providers"),
  create: (resource: any) =>
    apiRequest("/resources", {
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
  create: (app: any) =>
    apiRequest("/application/create", {
      method: "POST",
      body: JSON.stringify(app),
    }),
  update: (id: string, app: any) =>
    apiRequest(`/applications/${id}`, {
      method: "PUT",
      body: JSON.stringify(app),
    }),
  delete: (id: string) =>
    apiRequest(`/applications/${id}`, {
      method: "DELETE",
    }),
  deploy: (id: string) =>
    apiRequest(`/applications/${id}/deploy`, {
      method: "POST",
    }),
  stop: (id: string) =>
    apiRequest(`/applications/${id}/stop`, {
      method: "POST",
    }),
}

// 状态监控 API
export const statusAPI = {
  getAll: () => apiRequest("/status"),
  restart: (id: string) =>
    apiRequest(`/status/${id}/restart`, {
      method: "POST",
    }),
}
