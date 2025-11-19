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

// UpdateResourceProviderRequest 更新资源提供者请求
export interface UpdateResourceProviderRequest {
  name?: string // 提供者名称（可选，目前唯一支持的字段）
}

// UpdateResourceProviderResponse 更新资源提供者响应
export interface UpdateResourceProviderResponse {
  id: string      // 提供者 ID
  name: string    // 更新后的名称
  message: string // 响应消息
}

export interface Application {
  id: string
  name: string
  description: string
  gitUrl?: string
  branch?: string
  status: "idle" | "running" | "stopped" | "error" | "deploying" | "cloning"
  lastDeployed?: string
  runnerEnv?: string
  containerId?: string
  executeCmd?: string
  envInstallCmd?: string
}

export interface GetApplicationsResponse {
  applications: Application[]
}

export interface LogFieldKV {
  key: string
  value: string
}

export interface LogCallerInfo {
  file?: string
  line?: number
  function?: string
}

export interface ApplicationLogPayload {
  timestamp: string
  level: string
  message: string
  fields?: LogFieldKV[]
  caller?: LogCallerInfo
}

export interface LogEntry {
  id: string
  timestamp: string
  level: string
  app?: string
  message: string
  details?: string
  fields?: LogFieldKV[]
  caller?: LogCallerInfo
}

export interface GetApplicationLogsResponse {
  applicationId: string
  logs: ApplicationLogPayload[]
  total: number
  hasMore: boolean
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
  environments: string[]
}

// ApplicationStats 应用统计信息
export interface ApplicationStats {
  total: number      // 总应用数
  running: number    // 运行中的应用数
  stopped: number    // 已停止的应用数
  undeployed: number // 未部署的应用数
  failed: number     // 失败的应用数
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

export interface ActorComponentInfo {
  id?: string
  name?: string
  image?: string
  provider_id?: string
  providerId?: string
  providerID?: string
  resource_usage?: ComponentResourceUsage
  resourceUsage?: ComponentResourceUsage
  [key: string]: any
}

export interface ActorLatencyInfo {
  CalcLatency?: number
  LinkLatency?: number
  calcLatency?: number
  linkLatency?: number
  calc_latency?: number
  link_latency?: number
  [key: string]: any
}

export interface ActorRecord {
  id?: string
  ID?: string
  actor_id?: string
  actorId?: string
  actorID?: string
  name?: string
  component?: ActorComponentInfo
  info?: ActorLatencyInfo
  [key: string]: any
}

export type GetApplicationActorsResponse = Record<string, ActorRecord[] | undefined>

export interface GetComponentsResponse {
  components: Component[]
}

export interface ComponentLogEntry {
  timestamp?: string
  level?: string
  message: string
  fields?: LogFieldKV[]
  caller?: LogCallerInfo
}

export interface GetComponentLogsResponse {
  componentId?: string
  logs: ComponentLogEntry[]
  total?: number
  hasMore?: boolean
}

export type DAGNodeStatus = "pending" | "ready" | "running" | "done" | "failed"

export interface ControlNode {
  id: string
  status: DAGNodeStatus
  functionName: string
  params: Record<string, string>
}

export interface DataNode {
  id: string
  status: DAGNodeStatus
  lambda: string
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
  sessions: string[]
  selectedSessionId?: string
}