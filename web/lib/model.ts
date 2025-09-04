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