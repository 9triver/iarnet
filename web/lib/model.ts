export interface ResourceProvider {
  id: string
  name: string
  host: string
  port: number
  type: string
  status: number
  cpu_usage: {
    used: number
    total: number
  }
  memory_usage: {
    used: number
    total: number
  }
  last_update_time: string
}

export interface GetResourceProvidersResponse {
  local_providers: ResourceProvider[]
  remote_providers: ResourceProvider[]
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