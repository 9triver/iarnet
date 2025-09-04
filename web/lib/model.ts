interface ResourceProvider {
  id: string
  name: string
  url: string
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
  providers: ResourceProvider[]
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