interface ResourceProvider {
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

interface GetResourceProvidersResponse {
  local_provider: ResourceProvider | null         // 本机 provider（无或一个）
  managed_providers: ResourceProvider[]      // 托管的 provider（无或多个）
  collaborative_providers: ResourceProvider[] // 通过协作发现的 provider（无或多个）
}

interface Application {
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

interface GetApplicationsResponse {
  applications: Application[]
}

interface GetApplicationLogsResponse {
  applicationId: string
  applicationName: string
  logs: string[]
  totalLines: number
  requestedLines: number
}

interface CodeBrowserInfo {
  port: number
  pid: number
  start_time: string
  status: "running" | "stopped" | "error"
  work_dir: string
  cmd: string
}

interface StartCodeBrowserResponse {
  message: string
  port: number
  url: string
}

interface StopCodeBrowserResponse {
  message: string
}

interface GetCodeBrowserStatusResponse {
  browser: CodeBrowserInfo | null
}

interface FileInfo {
  name: string
  path: string
  is_dir: boolean
  size: number
  mod_time: string
}

interface GetFileTreeResponse {
  files: FileInfo[]
}

interface GetFileContentResponse {
  content: string
  language: string
  path: string
}

interface SaveFileResponse {
  message: string
  filePath: string
}

interface CreateFileResponse {
  message: string
  filePath: string
}

interface DeleteFileResponse {
  message: string
  filePath: string
}

interface CreateDirectoryResponse {
  message: string
  directoryPath: string
}

interface DeleteDirectoryResponse {
  message: string
  directoryPath: string
}