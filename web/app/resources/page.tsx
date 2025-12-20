"use client"

import { useState, useEffect } from "react"
import { Sidebar } from "@/components/sidebar"
import { resourcesAPI } from "@/lib/api"
import type {
  GetResourceCapacityResponse,
  GetResourceProvidersResponse,
  ProviderItem,
  RegisterResourceProviderRequest,
  GetResourceProviderCapacityResponse,
  TestResourceProviderResponse,
  DiscoveredNodeItem,
  GetNodeInfoResponse,
  ResourceTagsInfo,
} from "@/lib/model"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { useForm } from "react-hook-form"
import { Plus, Server, Cpu, HardDrive, Activity, Trash2, Edit, RefreshCw, MemoryStick, Network, Upload, Download, FileSpreadsheet } from "lucide-react"
import { formatMemory, formatNumber, formatDateTime } from "@/lib/utils"
import { Skeleton } from "@/components/ui/skeleton"
import { AuthGuard } from "@/components/auth-guard"
import { toast } from "sonner"

// 本地使用的资源类型（包含 CPU 和内存使用情况）
// 注意：后端 ProviderItem 不包含 cpu_usage 和 memory_usage
// 这些数据需要从其他地方获取或计算
interface Resource {
  id: string
  name: string
  type: "kubernetes" | "docker" | "vm"
  host: string
  port: number
  category?: "local_providers" | "remote_providers" // 资源分类：本地、远程
  status: "connected" | "disconnected" | "error"
  cpu: {
    total: number
    used: number
  }
  memory: {
    total: number
    used: number
  }
  lastUpdated: string
  resourceTags?: ResourceTagsInfo
}


interface ResourceFormData {
  name: string
  type: "kubernetes" | "docker" | "vm"
  url: string  // 格式: host:port，例如: localhost:50051
  token: string
  description?: string
}

interface NodeFormData {
  name: string
  host: string
  port: number
  description?: string
}

interface Usage {
  cpu: number
  memory: number
  gpu: number
}

interface Capacity {
  total: Usage
  used: Usage
  available: Usage
}

export default function ResourcesPage() {
  const [resources, setResources] = useState<Resource[]>([])
  const [discoveredNodes, setDiscoveredNodes] = useState<DiscoveredNodeItem[]>([])
  const [capacity, setCapacity] = useState<Capacity | null>(null)
  const [nodeInfo, setNodeInfo] = useState<GetNodeInfoResponse | null>(null)
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [isNodeDialogOpen, setIsNodeDialogOpen] = useState(false)
  const [isBatchDialogOpen, setIsBatchDialogOpen] = useState(false)
  const [editingResource, setEditingResource] = useState<Resource | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshingIds, setRefreshingIds] = useState<Set<string>>(new Set())
  const [testingConnection, setTestingConnection] = useState(false)
  const [testResult, setTestResult] = useState<TestResourceProviderResponse | null>(null)
  const [csvFile, setCsvFile] = useState<File | null>(null)
  const [csvData, setCsvData] = useState<Array<{ name: string; host: string; port: number }>>([])
  const [batchProcessing, setBatchProcessing] = useState(false)
  const [batchResults, setBatchResults] = useState<Array<{ name: string; success: boolean; message: string }>>([])

  // 状态转换函数
  const convertStatus = (status: string | number | any): "connected" | "disconnected" | "error" => {
    // 确保status是字符串类型
    const statusStr = String(status || '').toLowerCase()
    switch (statusStr) {
      case "connected":
      case "online":
      case "active":
      case "1":
        return "connected"
      case "disconnected":
      case "offline":
      case "inactive":
      case "2":
        return "disconnected"
      default:
        return "error"
    }
  }

  const form = useForm<ResourceFormData>({
    defaultValues: {
      name: "",
      type: "kubernetes",
      url: "",
      token: "",
      description: "",
    },
  })

  // 获取资源容量数据
  const fetchCapacity = async () => {
    try {
      const data = await resourcesAPI.getCapacity()
      // 后端返回的CPU单位是毫核（millicores），需要除以1000转换为核（cores）
      // 内存单位是 bytes，需要转换为合适的单位（在显示时转换）
      const capacity: Capacity = {
        total: {
          cpu: data.total.cpu / 1000, // 转换为核
          memory: data.total.memory,    // 保持 bytes，显示时转换
          gpu: data.total.gpu,
        },
        used: {
          cpu: data.used.cpu / 1000,
          memory: data.used.memory,
          gpu: data.used.gpu,
        },
        available: {
          cpu: data.available.cpu / 1000,
          memory: data.available.memory,
          gpu: data.available.gpu,
        },
      }
      setCapacity(capacity)
    } catch (error) {
      console.error('Failed to fetch capacity:', error)
    }
  }

  // 获取发现的节点数据
  const fetchDiscoveredNodes = async () => {
    try {
      const response = await resourcesAPI.getDiscoveredNodes()
      setDiscoveredNodes(response.nodes || [])
    } catch (error) {
      console.error('Failed to fetch discovered nodes:', error)
      setDiscoveredNodes([])
    }
  }

  // 获取当前节点与域信息
  const fetchNodeInfo = async () => {
    try {
      const data = await resourcesAPI.getNodeInfo()
      setNodeInfo(data)
    } catch (error) {
      console.error('Failed to fetch node info:', error)
      setNodeInfo(null)
    }
  }

  const fetchData = async () => {
    try {
      setLoading(true)
      await Promise.all([
        fetchProviders(),
        fetchCapacity(),
        fetchDiscoveredNodes(),
        fetchNodeInfo(),
      ])
    } finally {
      setLoading(false)
    }
  }



  // 获取资源提供者数据
  const fetchProviders = async () => {
    try {
      const response = await resourcesAPI.getProviders()
      
      // 转换API数据格式为前端需要的格式
      // 为每个 provider 获取容量信息
      const convertProvider = async (provider: ProviderItem): Promise<Resource> => {
        let cpu = { total: 0, used: 0 }
        let memory = { total: 0, used: 0 }
        
        // 获取 provider 的容量信息
        try {
          const capacity = await resourcesAPI.getProviderCapacity(provider.id)
          // 后端返回的CPU单位是毫核（millicores），需要除以1000转换为核（cores）
          cpu = {
            total: capacity.total.cpu / 1000,
            used: capacity.used.cpu / 1000,
          }
          // 内存单位是 bytes，保持原样，显示时会转换
          memory = {
            total: capacity.total.memory,
            used: capacity.used.memory,
          }
        } catch (error) {
          console.warn(`Failed to get capacity for provider ${provider.id}:`, error)
          // 如果获取容量失败，保持默认值 0
        }
        
        return {
          id: provider.id,
          name: provider.name,
          type: provider.type.toLowerCase() as "kubernetes" | "docker" | "vm",
          host: provider.host,
          port: provider.port,
          category: "local_providers", // 全部作为本地资源处理
          status: convertStatus(provider.status),
          cpu,
          memory,
          lastUpdated: provider.last_update_time,
          resourceTags: provider.resource_tags,
        }
      }
      
      // 只处理本地资源，远程资源暂不接入数据
      // 使用 Promise.all 并行获取所有 provider 的容量信息
      const localResources = await Promise.all(
        (response.providers || []).map(convertProvider)
      )
      
      setResources(localResources)
    } catch (error) {
      console.error('Failed to fetch providers:', error)
      // 如果API调用失败，保持使用模拟数据
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  // 解析 URL 字符串为 host 和 port
  const parseURL = (url: string): { host: string; port: number } => {
    if (!url || !url.trim()) {
      throw new Error('连接地址不能为空')
    }
    
    const trimmedUrl = url.trim()
    const parts = trimmedUrl.split(':')
    
    if (parts.length !== 2) {
      throw new Error('URL 格式错误，应为 host:port，例如: localhost:50051')
    }
    
    const host = parts[0].trim()
    const portStr = parts[1].trim()
    
    if (!host) {
      throw new Error('主机地址不能为空')
    }
    
    const port = parseInt(portStr, 10)
    if (isNaN(port)) {
      throw new Error('端口必须是数字')
    }
    
    if (port <= 0 || port > 65535) {
      throw new Error('端口必须在 1-65535 之间')
    }
    
    return { host, port }
  }

  const onSubmit = async (data: ResourceFormData) => {
    try {
      if (editingResource) {
        // 编辑现有资源 - 调用后端API
        // 目前只支持更新名称，连接地址不可编辑
        const request = {
          name: data.name,
        }
        
        const response = await resourcesAPI.updateProvider(editingResource.id, request)
        console.log('Provider updated successfully:', response)
        
        // 重新获取资源列表以显示最新数据
        await fetchProviders()
      } else {
        // 添加新资源 - 调用后端API
        // 解析 URL
        const { host, port } = parseURL(data.url)
        
        const request: RegisterResourceProviderRequest = {
          name: data.name,
          host,
          port,
        }
        
        const response = await resourcesAPI.registerProvider(request)
        console.log('Provider registered successfully:', response)
        
        // 重新获取资源列表以显示最新数据
        await fetchProviders()
      }

      setIsDialogOpen(false)
      // 状态清除由 onOpenChange 处理，避免重复操作
    } catch (error) {
      console.error('Failed to process form:', error)
      // 如果是编辑模式，错误信息设置到 name 字段
      // 如果是添加模式，错误信息设置到 url 字段
      const errorMessage = error instanceof Error ? error.message : '操作失败'
      if (editingResource) {
        form.setError('name', {
          type: 'manual',
          message: errorMessage,
        })
      } else {
        form.setError('url', {
          type: 'manual',
          message: errorMessage,
        })
      }
    }
  }

  const handleEdit = (resource: Resource) => {
    setEditingResource(resource)
    setTestResult(null)
    form.setValue("name", resource.name)
    form.setValue("type", resource.type)
    form.setValue("url", `${resource.host}:${resource.port}`)
    setIsDialogOpen(true)
  }

  // CSV 样例下载
  const downloadCsvTemplate = () => {
    const csvContent = `资源名称,连接地址（提交时删除该行）
资源1,192.168.1.100:50051
资源2,192.168.1.101:50051
资源3,192.168.1.102:50051`
    
    const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' })
    const link = document.createElement('a')
    const url = URL.createObjectURL(blob)
    link.setAttribute('href', url)
    link.setAttribute('download', '资源批量接入模板.csv')
    link.style.visibility = 'hidden'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  // 解析 CSV 文件
  const parseCsvFile = async (file: File): Promise<Array<{ name: string; host: string; port: number }>> => {
    return new Promise((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = (e) => {
        try {
          let text = e.target?.result as string
          
          // 移除 BOM (Byte Order Mark)
          if (text.charCodeAt(0) === 0xFEFF) {
            text = text.slice(1)
          }
          
          // 处理不同的换行符（\r\n, \n, \r）
          const lines = text.split(/\r?\n|\r/).filter(line => line.trim())
          
          if (lines.length < 2) {
            reject(new Error('CSV 文件至少需要包含表头和数据行'))
            return
          }
          
          // 跳过表头
          const dataLines = lines.slice(1)
          const parsed: Array<{ name: string; host: string; port: number }> = []
          
          // 简单的 CSV 解析函数，处理引号内的逗号
          const parseCsvLine = (line: string): string[] => {
            const result: string[] = []
            let current = ''
            let inQuotes = false
            
            for (let i = 0; i < line.length; i++) {
              const char = line[i]
              
              if (char === '"') {
                if (inQuotes && line[i + 1] === '"') {
                  // 转义的引号
                  current += '"'
                  i++
                } else {
                  // 切换引号状态
                  inQuotes = !inQuotes
                }
              } else if (char === ',' && !inQuotes) {
                // 字段分隔符
                result.push(current.trim())
                current = ''
              } else {
                current += char
              }
            }
            
            // 添加最后一个字段
            result.push(current.trim())
            return result
          }
          
          for (let i = 0; i < dataLines.length; i++) {
            const line = dataLines[i].trim()
            if (!line) continue // 跳过空行
            
            const columns = parseCsvLine(line)
            if (columns.length >= 2) {
              const name = columns[0].replace(/^"|"$/g, '')
              const addressStr = columns[1].replace(/^"|"$/g, '')
              
              if (!name || !addressStr) {
                continue // 跳过无效行
              }
              
              // 解析地址:端口格式
              const addressParts = addressStr.split(':')
              if (addressParts.length !== 2) {
                reject(new Error(`第 ${i + 2} 行地址格式错误: ${addressStr}（格式应为 主机地址:端口，例如：192.168.1.100:50051）`))
                return
              }
              
              const host = addressParts[0].trim()
              const portStr = addressParts[1].trim()
              const port = parseInt(portStr, 10)
              
              if (!host) {
                reject(new Error(`第 ${i + 2} 行主机地址为空`))
                return
              }
              
              if (isNaN(port) || port <= 0 || port > 65535) {
                reject(new Error(`第 ${i + 2} 行端口无效: ${portStr}（端口必须在 1-65535 之间）`))
                return
              }
              
              parsed.push({ name, host, port })
            } else {
              reject(new Error(`第 ${i + 1} 行列数不足，需要至少 2 列（资源名称、地址:端口）`))
              return
            }
          }
          
          if (parsed.length === 0) {
            reject(new Error('CSV 文件中没有有效的资源数据'))
            return
          }
          
          resolve(parsed)
        } catch (error) {
          reject(error instanceof Error ? error : new Error('CSV 文件格式错误，请检查文件内容'))
        }
      }
      reader.onerror = () => reject(new Error('文件读取失败'))
      reader.readAsText(file, 'UTF-8')
    })
  }

  // 处理 CSV 文件上传
  const handleCsvFileChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return

    if (!file.name.endsWith('.csv')) {
      toast.error('文件格式错误', { description: '请上传 CSV 格式的文件' })
      return
    }

    try {
      setCsvFile(file)
      const parsed = await parseCsvFile(file)
      if (parsed.length === 0) {
        toast.warning('文件内容为空', { description: 'CSV 文件中没有有效的节点数据' })
        setCsvFile(null)
        setCsvData([])
        return
      }
      setCsvData(parsed)
      setBatchResults([])
      toast.success('文件解析成功', { description: `成功解析 ${parsed.length} 条资源记录` })
    } catch (error) {
      toast.error('文件解析失败', { description: error instanceof Error ? error.message : '解析 CSV 文件失败' })
      setCsvFile(null)
      setCsvData([])
    }
  }

  // 批量注册节点
  const handleBatchRegister = async () => {
    if (!csvFile) {
      toast.error('请先上传 CSV 文件')
      return
    }

    setBatchProcessing(true)
    setBatchResults([])

    try {
      const response = await resourcesAPI.batchRegisterProvider(csvFile)
      
      // 转换响应结果为前端格式
      const results: Array<{ name: string; success: boolean; message: string }> = 
        response.results.map((r: { name: string; success: boolean; message: string }) => ({
          name: r.name,
          success: r.success,
          message: r.message,
        }))
      
      setBatchResults(results)

      // 统计结果
      const successCount = response.success
      const failCount = response.failed

      if (successCount > 0 && failCount === 0) {
        toast.success('批量接入成功', { description: `成功接入 ${successCount} 个资源` })
        await fetchProviders()
      } else if (successCount > 0 && failCount > 0) {
        toast.warning('部分接入成功', { description: `成功 ${successCount} 个，失败 ${failCount} 个` })
        await fetchProviders()
      } else {
        toast.error('批量接入失败', { description: `所有 ${failCount} 个资源接入失败` })
      }

      // 如果有 CSV 解析错误，显示警告
      if (response.errors && response.errors.length > 0) {
        toast.error('CSV 解析错误', { 
          description: response.errors.slice(0, 3).join('; ') + (response.errors.length > 3 ? '...' : '')
        })
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : '批量接入失败'
      toast.error('批量接入失败', { description: errorMessage })
    } finally {
      setBatchProcessing(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await resourcesAPI.unregisterProvider(id)
      console.log('Provider unregistered successfully:', id)
      
      // 重新获取资源列表以显示最新数据
      await fetchProviders()
    } catch (error) {
      console.error('Failed to unregister provider:', error)
      // 可以在这里添加错误提示
    }
  }

  // 刷新单个 provider 的数据
  const handleRefreshProvider = async (id: string) => {
    // 设置刷新状态
    setRefreshingIds(prev => new Set(prev).add(id))
    
    // 记录开始时间，确保最小显示时间（避免闪烁）
    const startTime = Date.now()
    const minDisplayTime = 300 // 最小显示时间 300ms
    
    try {
      // 获取 provider 的最新信息和容量
      const [providerInfo, capacity] = await Promise.all([
        resourcesAPI.getProviderInfo(id),
        resourcesAPI.getProviderCapacity(id),
      ])
      
      // 转换数据格式
      const updatedResource: Resource = {
        id: providerInfo.id,
        name: providerInfo.name,
        type: providerInfo.type.toLowerCase() as "kubernetes" | "docker" | "vm",
        host: providerInfo.host,
        port: providerInfo.port,
        category: "local_providers",
        status: convertStatus(providerInfo.status),
        cpu: {
          // 后端返回的CPU单位是毫核（millicores），需要除以1000转换为核（cores）
          total: capacity.total.cpu / 1000,
          used: capacity.used.cpu / 1000,
        },
        memory: {
          total: capacity.total.memory,
          used: capacity.used.memory,
        },
        lastUpdated: providerInfo.last_update_time,
        resourceTags: providerInfo.resource_tags,
      }
      
      // 更新 resources 状态中对应的 provider
      setResources(prev => 
        prev.map(resource => 
          resource.id === id ? updatedResource : resource
        )
      )
      
      // 确保最小显示时间
      const elapsedTime = Date.now() - startTime
      if (elapsedTime < minDisplayTime) {
        await new Promise(resolve => setTimeout(resolve, minDisplayTime - elapsedTime))
      }
    } catch (error) {
      console.error(`Failed to refresh provider ${id}:`, error)
      
      // 检查是否是 APIError 且状态码为 404
      if (error && typeof error === 'object' && 'status' in error) {
        const apiError = error as { status: number; message?: string }
        if (apiError.status === 404) {
          // 404 错误，说明 provider 已被删除，从列表中移除
          setResources(prev => prev.filter(resource => resource.id !== id))
          console.log(`Provider ${id} not found, removed from list`)
        } else {
          // 其他 HTTP 错误，更新状态为 error
          setResources(prev => 
            prev.map(resource => 
              resource.id === id 
                ? { ...resource, status: "error" as const }
                : resource
            )
          )
        }
      } else {
        // 其他类型的错误，更新状态为 error
        setResources(prev => 
          prev.map(resource => 
            resource.id === id 
              ? { ...resource, status: "error" as const }
              : resource
          )
        )
      }
      
      // 即使出错，也确保最小显示时间
      const elapsedTime = Date.now() - startTime
      if (elapsedTime < minDisplayTime) {
        await new Promise(resolve => setTimeout(resolve, minDisplayTime - elapsedTime))
      }
    } finally {
      // 清除刷新状态
      setRefreshingIds(prev => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
    }
  }

  const getStatusBadge = (status: Resource["status"]) => {
    switch (status) {
      case "connected":
        return (
          <Badge variant="default" className="bg-green-500">
            已连接
          </Badge>
        )
      case "disconnected":
        return <Badge variant="secondary">已断开</Badge>
      case "error":
        return <Badge variant="destructive">错误</Badge>
    }
  }

  const getTypeIcon = (type: Resource["type"]) => {
    switch (type) {
      case "kubernetes":
        return <Server className="h-4 w-4" />
      case "docker":
        return <Cpu className="h-4 w-4" />
      case "vm":
        return <HardDrive className="h-4 w-4" />
    }
  }

  const renderResourceTags = (tags?: ResourceTagsInfo) => {
    const tagMap: { key: keyof ResourceTagsInfo; label: string }[] = [
      { key: "cpu", label: "CPU" },
      { key: "gpu", label: "GPU" },
      { key: "memory", label: "Memory" },
      { key: "camera", label: "Camera" },
    ]

    if (!tags) {
      return <span className="text-xs text-muted-foreground">-</span>
    }

    const activeTags = tagMap.filter(tag => tags[tag.key])
    if (activeTags.length === 0) {
      return <span className="text-xs text-muted-foreground">-</span>
    }

    return activeTags.map(tag => (
      <Badge key={tag.key} variant="outline" className="text-xs">
        {tag.label}
      </Badge>
    ))
  }

  // 骨架行组件
  const ResourceTableSkeleton = () => (
    <TableRow>
      <TableCell className="w-64">
        <div className="flex items-center space-x-2">
          <Skeleton className="h-4 w-4 bg-gray-200 dark:bg-gray-700" />
          <div>
            <Skeleton className="h-4 w-40 mb-1 bg-gray-200 dark:bg-gray-700" />
            <Skeleton className="h-3 w-48 bg-gray-200 dark:bg-gray-700" />
          </div>
        </div>
      </TableCell>
      <TableCell className="w-20">
        <Skeleton className="h-6 w-16 bg-gray-200 dark:bg-gray-700" />
      </TableCell>
      <TableCell className="w-20">
        <Skeleton className="h-6 w-16 bg-gray-200 dark:bg-gray-700" />
      </TableCell>
      <TableCell className="w-32">
        <div className="flex items-center space-x-2">
          <Skeleton className="h-4 w-10 bg-gray-200 dark:bg-gray-700" />
          <Skeleton className="h-4 w-10 bg-gray-200 dark:bg-gray-700" />
        </div>
      </TableCell>
      <TableCell className="w-32">
        <div className="flex items-center space-x-2">
          <Skeleton className="h-4 w-8 bg-gray-200 dark:bg-gray-700" />
          <Skeleton className="h-3 w-20 bg-gray-200 dark:bg-gray-700" />
        </div>
      </TableCell>
      <TableCell className="w-32">
        <div className="flex items-center space-x-2">
          <Skeleton className="h-4 w-8 bg-gray-200 dark:bg-gray-700" />
          <Skeleton className="h-3 w-20 bg-gray-200 dark:bg-gray-700" />
        </div>
      </TableCell>
      <TableCell className="w-40">
        <Skeleton className="h-3 w-32 bg-gray-200 dark:bg-gray-700" />
      </TableCell>
      <TableCell className="w-32">
        <div className="flex items-center space-x-2">
          <Skeleton className="h-8 w-8 bg-gray-200 dark:bg-gray-700" />
          <Skeleton className="h-8 w-8 bg-gray-200 dark:bg-gray-700" />
          <Skeleton className="h-8 w-8 bg-gray-200 dark:bg-gray-700" />
        </div>
      </TableCell>
    </TableRow>
  )

  return (
    <AuthGuard>
      <div className="flex h-screen bg-background">
      <Sidebar />

      <main className="flex-1 overflow-auto">
        <div className="p-8">
          {/* Header */}
          <div className="flex items-center justify-between mb-8">
            <div>
              <h1 className="text-3xl font-playfair font-bold text-foreground mb-2">算力资源管理</h1>
              <p className="text-muted-foreground">接入和管理您的算力资源，包括CPU、GPU、内存和网络资源</p>
            </div>

            <div className="flex items-center space-x-3">
              <Button
                variant="outline"
                onClick={() => {
                  fetchData()
                }}
                disabled={loading}
              >
                <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
                刷新数据
              </Button>
              
              {/* 动态添加节点功能尚未实现，暂时注释 */}
              {/* <Dialog open={isNodeDialogOpen} onOpenChange={setIsNodeDialogOpen}>
                <DialogTrigger asChild>
                  <Button variant="outline">
                    <Plus className="mr-2 h-4 w-4" />
                    接入节点
                  </Button>
                </DialogTrigger>
                <DialogContent className="sm:max-w-[425px]">
                  <DialogHeader>
                    <DialogTitle>接入远程节点</DialogTitle>
                    <DialogDescription>连接到远程算力节点，发现并使用其资源</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4">
                    <div className="space-y-2">
                      <label className="text-sm font-medium">节点名称</label>
                      <Input placeholder="输入节点名称" />
                    </div>
                    <div className="space-y-2">
                      <label className="text-sm font-medium">主机地址</label>
                      <Input placeholder="例如: 192.168.1.200" />
                    </div>
                    <div className="space-y-2">
                      <label className="text-sm font-medium">端口</label>
                      <Input type="number" placeholder="例如: 8080" />
                    </div>
                    <div className="space-y-2">
                      <label className="text-sm font-medium">描述</label>
                      <Textarea placeholder="节点描述信息（可选）" className="min-h-[80px]" />
                    </div>
                  </div>
                  <DialogFooter>
                    <Button type="button" variant="outline" onClick={() => setIsNodeDialogOpen(false)}>
                      取消
                    </Button>
                    <Button type="button" onClick={() => setIsNodeDialogOpen(false)}>
                      接入节点
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog> */}

              <Dialog open={isBatchDialogOpen} onOpenChange={(open) => {
                setIsBatchDialogOpen(open)
                if (!open) {
                  // 关闭时清除状态
                  setTimeout(() => {
                    setCsvFile(null)
                    setCsvData([])
                    setBatchResults([])
                  }, 150)
                }
              }}>
                <DialogTrigger asChild>
                  <Button variant="outline">
                    <Upload className="mr-2 h-4 w-4" />
                    批量接入资源
                  </Button>
                </DialogTrigger>
                <DialogContent className="sm:max-w-[600px]">
                  <DialogHeader>
                    <DialogTitle>批量接入资源</DialogTitle>
                    <DialogDescription>
                      上传 CSV 文件批量接入多个资源。CSV 文件应包含资源名称和地址:端口两列。
                    </DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4">
                    {/* CSV 样例下载 */}
                    <div className="flex items-center justify-between p-3 bg-muted rounded-lg">
                      <div className="flex items-center space-x-2">
                        <FileSpreadsheet className="h-4 w-4 text-muted-foreground" />
                        <span className="text-sm text-muted-foreground">下载 CSV 文件样例</span>
                      </div>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={downloadCsvTemplate}
                      >
                        <Download className="mr-2 h-4 w-4" />
                        下载模板
                      </Button>
                    </div>

                    {/* 文件上传 */}
                    <div className="space-y-2">
                      <label className="text-sm font-medium">上传 CSV 文件</label>
                      <div className="flex items-center space-x-2">
                        <Input
                          type="file"
                          accept=".csv"
                          onChange={handleCsvFileChange}
                          className="cursor-pointer"
                        />
                      </div>
                      {csvFile && (
                        <p className="text-sm text-muted-foreground">
                          已选择文件: {csvFile.name} ({csvData.length} 条记录)
                        </p>
                      )}
                    </div>

                    {/* 预览数据 */}
                    {csvData.length > 0 && (
                      <div className="space-y-2">
                        <label className="text-sm font-medium">预览数据</label>
                        <div className="border rounded-lg max-h-48 overflow-auto">
                          <Table>
                            <TableHeader>
                              <TableRow>
                                <TableHead>资源名称</TableHead>
                                <TableHead>地址:端口</TableHead>
                              </TableRow>
                            </TableHeader>
                            <TableBody>
                              {csvData.map((item, index) => (
                                <TableRow key={index}>
                                  <TableCell>{item.name}</TableCell>
                                  <TableCell>{item.host}:{item.port}</TableCell>
                                </TableRow>
                              ))}
                            </TableBody>
                          </Table>
                        </div>
                      </div>
                    )}

                    {/* 处理结果 */}
                    {batchResults.length > 0 && (
                      <div className="space-y-2">
                        <label className="text-sm font-medium">处理结果</label>
                        <div className="border rounded-lg max-h-48 overflow-auto">
                          <Table>
                            <TableHeader>
                              <TableRow>
                                <TableHead>资源名称</TableHead>
                                <TableHead>状态</TableHead>
                                <TableHead>消息</TableHead>
                              </TableRow>
                            </TableHeader>
                            <TableBody>
                              {batchResults.map((result, index) => (
                                <TableRow key={index}>
                                  <TableCell>{result.name}</TableCell>
                                  <TableCell>
                                    {result.success ? (
                                      <Badge variant="default" className="bg-green-500">
                                        成功
                                      </Badge>
                                    ) : (
                                      <Badge variant="destructive">失败</Badge>
                                    )}
                                  </TableCell>
                                  <TableCell className="text-sm text-muted-foreground">
                                    {result.message}
                                  </TableCell>
                                </TableRow>
                              ))}
                            </TableBody>
                          </Table>
                        </div>
                      </div>
                    )}
                  </div>
                  <DialogFooter>
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => setIsBatchDialogOpen(false)}
                      disabled={batchProcessing}
                    >
                      取消
                    </Button>
                    <Button
                      type="button"
                      onClick={handleBatchRegister}
                      disabled={csvData.length === 0 || batchProcessing}
                    >
                      {batchProcessing ? (
                        <>
                          <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                          处理中...
                        </>
                      ) : (
                        <>
                          <Upload className="mr-2 h-4 w-4" />
                          开始批量接入
                        </>
                      )}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
              
              <Button
                onClick={() => {
                  // 点击接入资源时，清除所有状态并打开对话框
                  // 先清除编辑状态，确保对话框显示正确的标题
                  setEditingResource(null)
                  setTestResult(null)
                  // 重置表单到默认值
                  form.reset({
                    name: "",
                    type: "docker",
                    url: "",
                    token: "",
                  })
                  // 清除表单验证错误
                  form.clearErrors()
                  // 打开对话框
                  // 注意：由于 React 状态更新是批处理的，editingResource 会在同一渲染周期更新
                  setIsDialogOpen(true)
                }}
              >
                <Plus className="mr-2 h-4 w-4" />
                接入资源
              </Button>
              
              <Dialog 
                open={isDialogOpen} 
                onOpenChange={(open) => {
                  if (!open) {
                    // 关闭时清除所有相关状态
                    // 使用 setTimeout 延迟清除，避免在对话框关闭动画期间触发重新渲染
                    setTimeout(() => {
                      setEditingResource(null)
                      setTestResult(null)
                      form.reset({
                        name: "",
                        type: "docker",
                        url: "",
                        token: "",
                      })
                    }, 150) // 延迟 150ms，等待对话框关闭动画完成
                  }
                  setIsDialogOpen(open)
                }}
              >
                <DialogContent className="sm:max-w-[425px]">
                  <DialogHeader>
                    <DialogTitle>{editingResource ? "编辑资源" : "接入资源"}</DialogTitle>
                    <DialogDescription>
                      {editingResource 
                        ? "修改资源名称和连接地址" 
                        : "配置新的算力资源节点连接信息"}
                    </DialogDescription>
                  </DialogHeader>
                  <Form {...form}>
                    <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
                      <FormField
                        control={form.control}
                        name="name"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>资源名称</FormLabel>
                            <FormControl>
                              <Input placeholder="输入资源名称" {...field} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="url"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>连接地址</FormLabel>
                            <FormControl>
                              <Input 
                                placeholder="例如: localhost:50051 或 192.168.1.100:6443" 
                                {...field}
                                disabled={!!editingResource}
                                readOnly={!!editingResource}
                              />
                            </FormControl>
                            <FormDescription>
                              格式: host:port，例如: localhost:50051
                            </FormDescription>
                            <FormMessage />
                            {!editingResource && (
                              <div className="flex items-center gap-2 mt-2">
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="sm"
                                  disabled={testingConnection || !field.value}
                                  onClick={async () => {
                                    try {
                                      setTestingConnection(true)
                                      setTestResult(null)
                                      
                                      const { host, port } = parseURL(field.value)
                                      const name = form.getValues("name") || "test"
                                      
                                      const result = await resourcesAPI.testProvider({
                                        name,
                                        host,
                                        port,
                                      })
                                      
                                      // 如果后端返回 success: false，也需要处理错误消息
                                      if (!result.success && result.message) {
                                        let errorMessage = result.message
                                        const errorMsg = errorMessage.toLowerCase()
                                        
                                        // 检查是否是认证相关的错误
                                        if (errorMsg.includes('authentication') || 
                                            errorMsg.includes('provider_id is required') ||
                                            errorMsg.includes('authenticated requests')) {
                                          errorMessage = '认证失败，当前 provider 已被其他节点注册'
                                        } else if (errorMsg.includes('连接失败')) {
                                          // 提取原始错误信息，去掉 "连接失败: " 前缀
                                          const originalMsg = result.message.replace(/^连接失败:\s*/i, '')
                                          const originalMsgLower = originalMsg.toLowerCase()
                                          
                                          // 再次检查是否是认证错误
                                          if (originalMsgLower.includes('authentication') || 
                                              originalMsgLower.includes('provider_id is required') ||
                                              originalMsgLower.includes('authenticated requests')) {
                                            errorMessage = '认证失败，当前 provider 已被其他节点注册'
                                          }
                                        }
                                        
                                        setTestResult({
                                          ...result,
                                          message: errorMessage,
                                        })
                                      } else {
                                        setTestResult(result)
                                      }
                                    } catch (error) {
                                      let errorMessage = '测试连接失败'
                                      
                                      if (error instanceof Error) {
                                        const errorMsg = error.message.toLowerCase()
                                        // 检查是否是认证相关的错误（后端返回的错误消息可能包含 "连接失败: " 前缀）
                                        if (errorMsg.includes('authentication') || 
                                            errorMsg.includes('provider_id is required') ||
                                            errorMsg.includes('authenticated requests') ||
                                            errorMsg.includes('已被其他节点注册')) {
                                          errorMessage = '认证失败，当前 provider 已被其他节点注册'
                                        } else if (errorMsg.includes('connection') || 
                                                   errorMsg.includes('connect') ||
                                                   errorMsg.includes('连接失败')) {
                                          // 提取原始错误信息，去掉 "连接失败: " 前缀
                                          const originalMsg = error.message.replace(/^连接失败:\s*/i, '')
                                          const originalMsgLower = originalMsg.toLowerCase()
                                          
                                          // 再次检查是否是认证错误
                                          if (originalMsgLower.includes('authentication') || 
                                              originalMsgLower.includes('provider_id is required') ||
                                              originalMsgLower.includes('authenticated requests')) {
                                            errorMessage = '认证失败，当前 provider 已被其他节点注册'
                                          } else {
                                            errorMessage = '连接失败，请检查地址和端口是否正确'
                                          }
                                        } else {
                                          errorMessage = error.message
                                        }
                                      }
                                      
                                      setTestResult({
                                        success: false,
                                        type: "",
                                        message: errorMessage,
                                      })
                                    } finally {
                                      setTestingConnection(false)
                                    }
                                  }}
                                >
                                  {testingConnection ? (
                                    <>
                                      <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                                      测试中...
                                    </>
                                  ) : (
                                    "测试连接"
                                  )}
                                </Button>
                                {testResult && (
                                  <div className={`text-sm ${testResult.success ? 'text-green-600' : 'text-red-600'}`}>
                                    {testResult.success ? (
                                      <div className="flex items-center gap-2">
                                        <span>✓ 连接成功</span>
                                        {testResult.type && (
                                          <Badge variant="outline">{testResult.type}</Badge>
                                        )}
                                      </div>
                                    ) : (
                                      <span>✗ {testResult.message}</span>
                                    )}
                                  </div>
                                )}
                              </div>
                            )}
                            {!editingResource && testResult?.success && testResult.capacity && (
                              <div className="mt-2 p-3 bg-green-50 dark:bg-green-900/20 rounded-md border border-green-200 dark:border-green-800">
                                <div className="text-sm font-medium text-green-900 dark:text-green-100 mb-2">
                                  资源容量
                                </div>
                                <div className="grid grid-cols-3 gap-2 text-xs">
                                  <div>
                                    <div className="text-muted-foreground">CPU</div>
                                    <div className="font-medium">{(testResult.capacity.cpu / 1000).toFixed(3)} 核心</div>
                                  </div>
                                  <div>
                                    <div className="text-muted-foreground">内存</div>
                                    <div className="font-medium">{formatMemory(testResult.capacity.memory)}</div>
                                  </div>
                                  <div>
                                    <div className="text-muted-foreground">GPU</div>
                                    <div className="font-medium">{testResult.capacity.gpu || 0}</div>
                                  </div>
                                </div>
                              </div>
                            )}
                          </FormItem>
                        )}
                      />
                      <DialogFooter>
                        <Button 
                          type="button" 
                          variant="outline" 
                          onClick={() => {
                            setIsDialogOpen(false)
                            // 状态清除由 onOpenChange 处理，避免重复操作
                          }}
                        >
                          取消
                        </Button>
                        <Button type="submit">{editingResource ? "保存" : "接入资源"}</Button>
                      </DialogFooter>
                    </form>
                  </Form>
                </DialogContent>
              </Dialog>
            </div>
          </div>

          {/* Node & Domain info */}
          <Card className="mb-6">
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-medium">当前节点信息</CardTitle>
              <CardDescription>展示本节点标识</CardDescription>
            </CardHeader>
            <CardContent>
              {nodeInfo ? (
                <div>
                  <div>
                    <div className="text-xs text-muted-foreground mb-1">节点</div>
                    <div className="text-xl font-semibold">
                      {nodeInfo.node_name || "未命名节点"}
                    </div>
                    <div className="text-xs text-muted-foreground font-mono break-all mt-1">
                      ID: {nodeInfo.node_id || "--"}
                    </div>
                  </div>
                </div>
              ) : (
                <div>
                  <Skeleton className="h-16 w-full" />
                </div>
              )}
            </CardContent>
          </Card>

          {/* Stats Cards */}
          <div className="grid grid-cols-1 md:grid-cols-5 gap-6 mb-8">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">本地资源数</CardTitle>
                <Server className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{resources.length}</div>
                <p className="text-xs text-muted-foreground">已接入资源节点</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">在线资源</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-green-600">
                  {resources.filter((r) => r.status === "connected").length}
                </div>
                <p className="text-xs text-muted-foreground">正常连接中</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">已发现节点</CardTitle>
                <Network className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-purple-600">
                  {discoveredNodes.filter((n) => n.status === "online").length + 1}
                </div>
                <p className="text-xs text-muted-foreground">
                  已发现 {discoveredNodes.length} 个节点
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">总CPU核心</CardTitle>
                <Cpu className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {(() => {
                    if (loading) return "加载中..."
                    // 计算本地资源
                    const localCPU = capacity ? capacity.total.cpu : 0
                    // 计算远程节点资源（CPU单位是millicores，需要转换为cores）
                    const remoteCPU = discoveredNodes
                      .filter((n) => n.status === "online" && n.cpu)
                      .reduce((sum, n) => sum + (n.cpu?.total || 0) / 1000, 0)
                    const totalCPU = localCPU + remoteCPU
                    return formatNumber(totalCPU)
                  })()}
                </div>
                <p className="text-xs text-muted-foreground">
                  已使用 {(() => {
                    if (loading) return "--"
                    const localUsedCPU = capacity ? capacity.used.cpu : 0
                    const remoteUsedCPU = discoveredNodes
                      .filter((n) => n.status === "online" && n.cpu)
                      .reduce((sum, n) => sum + (n.cpu?.used || 0) / 1000, 0)
                    const totalUsedCPU = localUsedCPU + remoteUsedCPU
                    return formatNumber(totalUsedCPU)
                  })()} 核心
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">总内存</CardTitle>
                <MemoryStick className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {(() => {
                    if (loading) return "加载中..."
                    // 计算本地资源
                    const localMemory = capacity ? capacity.total.memory : 0
                    // 计算远程节点资源（内存单位是bytes）
                    const remoteMemory = discoveredNodes
                      .filter((n) => n.status === "online" && n.memory)
                      .reduce((sum, n) => sum + (n.memory?.total || 0), 0)
                    const totalMemory = localMemory + remoteMemory
                    return formatMemory(totalMemory)
                  })()}
                </div>
                <p className="text-xs text-muted-foreground">
                  已使用 {(() => {
                    if (loading) return "--"
                    const localUsedMemory = capacity ? capacity.used.memory : 0
                    const remoteUsedMemory = discoveredNodes
                      .filter((n) => n.status === "online" && n.memory)
                      .reduce((sum, n) => sum + (n.memory?.used || 0), 0)
                    const totalUsedMemory = localUsedMemory + remoteUsedMemory
                    return formatMemory(totalUsedMemory)
                  })()}
                </p>
              </CardContent>
            </Card>
          </div>

          {/* 本地资源面板 */}
          <Card className="mb-8">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="flex items-center space-x-2">
                    <div className="w-3 h-3 bg-blue-500 rounded-full"></div>
                    <span>本地资源</span>
                  </CardTitle>
                  <CardDescription>手动配置并托管的本地算力资源</CardDescription>
                </div>
                <div className="flex items-center space-x-4">
                  <div className="text-sm text-muted-foreground">
                    {resources.filter(r => r.category === 'local_providers').length} 个资源
                  </div>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                  <TableHead className="w-64">资源名称</TableHead>
                  <TableHead className="w-20">类型</TableHead>
                  <TableHead className="w-20">状态</TableHead>
                  <TableHead className="w-32">资源标签</TableHead>
                  <TableHead className="w-32">CPU使用率</TableHead>
                  <TableHead className="w-32">内存使用率</TableHead>
                  <TableHead className="w-40">最后更新</TableHead>
                  <TableHead className="w-32">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    Array.from({ length: 1 }).map((_, index) => (
                      <ResourceTableSkeleton key={`local-skeleton-${index}`} />
                    ))
                  ) : resources.filter(r => r.category === 'local_providers').length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={8} className="text-center py-8 text-muted-foreground">
                        暂无本地资源，点击上方"接入资源"按钮开始配置
                      </TableCell>
                    </TableRow>
                  ) : (
                    resources
                      .filter(r => r.category === 'local_providers')
                      .sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
                      .map((resource) => (
                      <TableRow key={resource.id}>
                        <TableCell className="w-64">
                          <div className="flex items-center space-x-2">
                            {getTypeIcon(resource.type)}
                            <div>
                              <div className="font-medium">{resource.name}</div>
                              <div className="text-sm text-muted-foreground">{resource.host}:{resource.port}</div>
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="w-20">
                          <Badge variant="outline">
                            {resource.type === "kubernetes" ? "K8s" : resource.type === "docker" ? "Docker" : "VM"}
                          </Badge>
                        </TableCell>
                        <TableCell className="w-20">{getStatusBadge(resource.status)}</TableCell>
                        <TableCell className="w-32">
                          <div className="flex flex-wrap gap-1">
                            {renderResourceTags(resource.resourceTags)}
                          </div>
                        </TableCell>
                        <TableCell className="w-32">
                          <div className="flex items-center space-x-2">
                            <div className="text-sm">
                              {resource.cpu.total > 0
                                ? `${Math.round((resource.cpu.used / resource.cpu.total) * 100)}%`
                                : "0%"}
                            </div>
                            <div className="text-xs text-muted-foreground">
                              {formatNumber(resource.cpu.used)}/{formatNumber(resource.cpu.total)} 核
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="w-32">
                          <div className="flex items-center space-x-2">
                            <div className="text-sm">
                              {resource.memory.total > 0
                                ? `${Math.round((resource.memory.used / resource.memory.total) * 100)}%`
                                : "0%"}
                            </div>
                            <div className="text-xs text-muted-foreground">
                              {formatMemory(resource.memory.used)}/{formatMemory(resource.memory.total)}
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="w-40 text-xs text-muted-foreground">{formatDateTime(resource.lastUpdated)}</TableCell>
                        <TableCell className="w-32">
                          <div className="flex items-center space-x-2">
                            <Button variant="ghost" size="sm" onClick={() => handleEdit(resource)}>
                              <Edit className="h-4 w-4" />
                            </Button>
                            <Button variant="ghost" size="sm" onClick={() => handleDelete(resource.id)}>
                              <Trash2 className="h-4 w-4" />
                            </Button>
                            <Button 
                              variant="ghost" 
                              size="sm" 
                              onClick={() => handleRefreshProvider(resource.id)}
                              disabled={refreshingIds.has(resource.id)}
                            >
                              <RefreshCw className={`h-4 w-4 ${refreshingIds.has(resource.id) ? 'animate-spin' : ''}`} />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          {/* 远程资源面板 */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="flex items-center space-x-2">
                    <div className="w-3 h-3 bg-purple-500 rounded-full"></div>
                    <span>远程资源</span>
                  </CardTitle>
                  <CardDescription>网络中自动发现的可协作算力资源</CardDescription>
                </div>
                <div className="flex items-center space-x-4">
                  <div className="text-sm text-muted-foreground">
                    {discoveredNodes.length} 个节点
                  </div>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-64">节点名称</TableHead>
                    <TableHead className="w-32">地址</TableHead>
                    <TableHead className="w-20">状态</TableHead>
                    <TableHead className="w-32">CPU使用率</TableHead>
                    <TableHead className="w-32">内存使用率</TableHead>
                    <TableHead className="w-40">最后更新</TableHead>
                    <TableHead className="w-32">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    Array.from({ length: 1 }).map((_, index) => (
                      <ResourceTableSkeleton key={`remote-skeleton-${index}`} />
                    ))
                  ) : discoveredNodes.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                        暂无远程资源，系统将自动发现网络中节点提供的资源，点击上方"接入节点"按钮可以接入新节点到网络中
                      </TableCell>
                    </TableRow>
                  ) : (
                    [...discoveredNodes]
                      .sort((a, b) => (a.node_name || '').localeCompare(b.node_name || '', 'zh-CN'))
                      .map((node) => {
                      // 计算 CPU 使用率（CPU 单位是 millicores，需要转换为 cores）
                      const cpuTotal = node.cpu ? node.cpu.total / 1000 : 0
                      const cpuUsed = node.cpu ? node.cpu.used / 1000 : 0
                      const cpuUsage = cpuTotal > 0 ? Math.round((cpuUsed / cpuTotal) * 100) : 0
                      
                      // 计算内存使用率
                      const memoryTotal = node.memory ? node.memory.total : 0
                      const memoryUsed = node.memory ? node.memory.used : 0
                      const memoryUsage = memoryTotal > 0 ? Math.round((memoryUsed / memoryTotal) * 100) : 0

                      return (
                        <TableRow key={node.node_id}>
                          <TableCell className="w-64">
                            <div className="flex items-center space-x-2">
                              <Server className="h-4 w-4 text-purple-500" />
                              <div>
                                <div className="font-medium">{node.node_name}</div>
                                <div className="text-xs text-muted-foreground font-mono">{node.node_id}</div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell className="w-32">
                            <div className="text-sm font-mono">{node.address}</div>
                          </TableCell>
                          <TableCell className="w-20">
                            {node.status === "online" ? (
                              <Badge variant="default" className="bg-green-500">在线</Badge>
                            ) : node.status === "offline" ? (
                              <Badge variant="secondary">离线</Badge>
                            ) : (
                              <Badge variant="destructive">错误</Badge>
                            )}
                          </TableCell>
                          <TableCell className="w-32">
                            {node.cpu ? (
                              <div className="flex items-center space-x-2">
                                <div className="text-sm">
                                  {cpuUsage}%
                                </div>
                                <div className="text-xs text-muted-foreground">
                                  {formatNumber(cpuUsed)}/{formatNumber(cpuTotal)} 核
                                </div>
                              </div>
                            ) : (
                              <span className="text-sm text-muted-foreground">-</span>
                            )}
                          </TableCell>
                          <TableCell className="w-32">
                            {node.memory ? (
                              <div className="flex items-center space-x-2">
                                <div className="text-sm">
                                  {memoryUsage}%
                                </div>
                                <div className="text-xs text-muted-foreground">
                                  {formatMemory(memoryUsed)}/{formatMemory(memoryTotal)}
                                </div>
                              </div>
                            ) : (
                              <span className="text-sm text-muted-foreground">-</span>
                            )}
                          </TableCell>
                          <TableCell className="w-40 text-xs text-muted-foreground">
                            {formatDateTime(node.last_seen)}
                          </TableCell>
                          <TableCell className="w-32">
                            <div className="flex items-center space-x-2">
                              <Button 
                                variant="ghost" 
                                size="sm" 
                                onClick={() => fetchDiscoveredNodes()}
                                title="刷新节点信息"
                              >
                                <RefreshCw className="h-4 w-4" />
                              </Button>
                            </div>
                          </TableCell>
                        </TableRow>
                      )
                    })
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>


        </div>
      </main>
    </div>
  </AuthGuard>
  )
}

