// ========== 资源管理相关类型 ==========

// ResourceInfo 资源信息（CPU单位：millicores，内存单位：bytes）
export interface ResourceInfo {
  cpu: number    // CPU（millicores）
  memory: number // 内存（bytes）
  gpu: number    // GPU 数量
}

// GetResourceCapacityResponse 获取资源容量响应
export interface GetResourceCapacityResponse {
  total: ResourceInfo     // 总资源
  used: ResourceInfo     // 已使用资源
  available: ResourceInfo // 可用资源
}

// ProviderItem 资源提供者列表项
export interface ProviderItem {
  id: string    // 提供者 ID
  name: string  // 提供者名称
  type: string  // 提供者类型
  host: string  // 主机地址
  port: number  // 端口
  status: string // 状态 (connected/disconnected/unknown)
  last_update_time: string // 最后更新时间（ISO 8601 格式）
}

// GetResourceProvidersResponse 获取资源提供者列表响应
export interface GetResourceProvidersResponse {
  providers: ProviderItem[] // 提供者列表
  total: number             // 总数
}

// RegisterResourceProviderRequest 注册资源提供者请求
export interface RegisterResourceProviderRequest {
  name: string // 提供者名称
  host: string // 主机地址
  port: number // 端口
}

// RegisterResourceProviderResponse 注册资源提供者响应
export interface RegisterResourceProviderResponse {
  id: string   // 提供者 ID
  name: string // 提供者名称
}

// UnregisterResourceProviderResponse 注销资源提供者响应
export interface UnregisterResourceProviderResponse {
  id: string      // 提供者 ID
  message: string // 响应消息
}

// GetResourceProviderInfoResponse 获取资源提供者信息响应
export interface GetResourceProviderInfoResponse {
  id: string    // 提供者 ID
  name: string  // 提供者名称
  type: string  // 提供者类型
  host: string  // 主机地址
  port: number  // 端口
  status: string // 状态 (connected/disconnected/unknown)
  last_update_time: string // 最后更新时间（ISO 8601 格式）
}

// GetResourceProviderCapacityResponse 获取资源提供者容量响应
export interface GetResourceProviderCapacityResponse {
  total: ResourceInfo     // 总资源
  used: ResourceInfo     // 已使用资源
  available: ResourceInfo // 可用资源
}

// TestResourceProviderRequest 测试资源提供者连接请求
export interface TestResourceProviderRequest {
  name: string  // 提供者名称
  host: string  // 主机地址
  port: number  // 端口
}

// TestResourceProviderResponse 测试资源提供者连接响应
export interface TestResourceProviderResponse {
  success: boolean      // 连接是否成功
  type: string          // 提供者类型
  capacity?: ResourceInfo // 资源容量（连接成功时返回）
  message: string       // 响应消息或错误信息
}

export interface Application {
  id: string
  name: string
  description: string
  gitUrl?: string
  branch?: string
  status: "idle" | "running" | "stopped" | "error" | "deploying"
  type: "web" | "api" | "worker" | "database"
  lastDeployed?: string
  runningOn?: string[]
  ports?: number[]
  healthCheck?: string
  executeCmd: string
  runnerEnv?: string
}

export interface GetApplicationsResponse {
  applications: Application[]
}

export interface GetApplicationLogsResponse {
  applicationId: string
  applicationName: string
  logs: string[]
  totalLines: number
  requestedLines: number
}

export interface LogEntry {
  id: string
  timestamp: string
  level: string
  app: string
  message: string
  details: string
}

export interface GetApplicationLogsParsedResponse {
  applicationId: string
  applicationName: string
  logs: LogEntry[]
  totalLines: number
  requestedLines: number
}

export interface CodeBrowserInfo {
  port: number
  pid: number
  start_time: string
  status: "running" | "stopped" | "error"
  work_dir: string
  cmd: string
}

export interface StartCodeBrowserResponse {
  message: string
  port: number
  url: string
}

export interface StopCodeBrowserResponse {
  message: string
}

export interface GetCodeBrowserStatusResponse {
  browser: CodeBrowserInfo | null
}

export interface FileInfo {
  name: string
  path: string
  is_dir: boolean
  size: number
  mod_time: string
}

export interface GetFileTreeResponse {
  files: FileInfo[]
}

export interface GetFileContentResponse {
  content: string
  language: string
  path: string
}

export interface SaveFileResponse {
  message: string
  filePath: string
}

export interface CreateFileResponse {
  message: string
  filePath: string
}

export interface DeleteFileResponse {
  message: string
  filePath: string
}

export interface CreateDirectoryResponse {
  message: string
  directoryPath: string
}

export interface DeleteDirectoryResponse {
  message: string
  directoryPath: string
}

export interface RunnerEnvironment {
  name: string
}

export interface GetRunnerEnvironmentsResponse {
  environments: RunnerEnvironment[]
}

export interface ComponentResourceUsage {
  cpu: number
  memory: number
  gpu: number
}

export interface Component {
  name: string
  image: string
  status: "pending" | "deploying" | "running" | "stopped" | "failed"
  provider_id: string
  resource_usage: ComponentResourceUsage
  deployed_at?: string
  created_at: string
  updated_at: string
}

export interface GetComponentsResponse {
  components: Component[]
}

export interface GetComponentLogsResponse {
  logs: string[]
}

export interface ControlNode {
  id: string
  done: boolean
  functionName: string
  params: Record<string, string>
  current: number
  dataNode: string
  preDataNodes: string[]
  functionType: string
}

export interface DataNode {
  id: string
  done: boolean
  lambda: string
  ready: boolean
  parentNode?: string
  childNode: string[]
}

export interface DAGNode_ControlNode {
  controlNode: ControlNode
}

export interface DAGNode_DataNode {
  dataNode: DataNode
}

export interface DAGNode {
  type: "ControlNode" | "DataNode"
  node: ControlNode | DataNode
}

export interface DAGEdge {
  fromNodeId: string
  toNodeId: string
  info: string
}

export interface DAG {
  nodes: DAGNode[]
  edges: DAGEdge[]
}

export interface GetDAGResponse {
  dag: DAG
}