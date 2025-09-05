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
    description: string
  }
  memory_usage: {
    used: number
    total: number
  }
  last_update_time: string
}

interface GetResourceProvidersResponse {
  local_provider: ResourceProvider | null     // 本地 provider（无或一个）
  remote_providers: ResourceProvider[]    // 远程添加的 provider（无或多个）
  discovered_providers: ResourceProvider[] // 通过 gossip 协议感知到的 provider（无或多个）
}

interface Application {
  id: string
  name: string
  description: string
  importType: "git" | "docker"
  gitUrl?: string
  branch?: string
  dockerImage?: string
  dockerTag?: string
  status: "idle" | "running" | "stopped" | "error" | "deploying"
  type: "web" | "api" | "worker" | "database"
  lastDeployed?: string
  runningOn?: string[]
  ports?: number[]
  healthCheck?: string
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