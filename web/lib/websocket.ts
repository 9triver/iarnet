// WebSocket客户端管理器，用于实时DAG状态更新

export interface DAGStateEvent {
  type: string
  applicationId: string
  nodeId?: string
  nodeState?: string
  timestamp: number
  data?: Record<string, any>
}

export type DAGStateEventHandler = (event: DAGStateEvent) => void

export class WebSocketManager {
  private ws: WebSocket | null = null
  private applicationId: string
  private handlers: Set<DAGStateEventHandler> = new Set()
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000
  private isConnecting = false

  constructor(applicationId: string) {
    this.applicationId = applicationId
  }

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        resolve()
        return
      }

      if (this.isConnecting) {
        reject(new Error('Already connecting'))
        return
      }

      this.isConnecting = true

      try {
        // 构建WebSocket URL
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
        // 前端运行在3001端口，后端运行在8083端口
        const backendHost = window.location.hostname + ':8083'
        const wsUrl = `${protocol}//${backendHost}/ws/dag-state?appId=${this.applicationId}`

        console.log('Connecting to WebSocket:', wsUrl)
        this.ws = new WebSocket(wsUrl)

        this.ws.onopen = () => {
          console.log('WebSocket connected for application:', this.applicationId)
          this.isConnecting = false
          this.reconnectAttempts = 0
          resolve()
        }

        this.ws.onmessage = (event) => {
          try {
            const data: DAGStateEvent = JSON.parse(event.data)
            console.log('Received DAG state event:', data)
            
            // 通知所有处理器
            this.handlers.forEach(handler => {
              try {
                handler(data)
              } catch (error) {
                console.error('Error in DAG state event handler:', error)
              }
            })
          } catch (error) {
            console.error('Error parsing WebSocket message:', error)
          }
        }

        this.ws.onclose = (event) => {
          console.log('WebSocket closed:', event.code, event.reason)
          this.isConnecting = false
          this.ws = null

          // 如果不是正常关闭，尝试重连
          if (event.code !== 1000 && this.reconnectAttempts < this.maxReconnectAttempts) {
            this.scheduleReconnect()
          }
        }

        this.ws.onerror = (error) => {
          console.error('WebSocket error:', error)
          this.isConnecting = false
          
          if (this.reconnectAttempts === 0) {
            reject(error)
          }
        }

      } catch (error) {
        this.isConnecting = false
        reject(error)
      }
    })
  }

  private scheduleReconnect() {
    this.reconnectAttempts++
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1)
    
    console.log(`Scheduling reconnect attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts} in ${delay}ms`)
    
    setTimeout(() => {
      this.connect().catch(error => {
        console.error('Reconnect failed:', error)
        
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
          console.error('Max reconnect attempts reached, giving up')
        }
      })
    }, delay)
  }

  addHandler(handler: DAGStateEventHandler) {
    this.handlers.add(handler)
  }

  removeHandler(handler: DAGStateEventHandler) {
    this.handlers.delete(handler)
  }

  disconnect() {
    if (this.ws) {
      this.ws.close(1000, 'Client disconnect')
      this.ws = null
    }
    this.handlers.clear()
    this.reconnectAttempts = 0
  }

  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }
}

// 全局WebSocket管理器实例
const wsManagers = new Map<string, WebSocketManager>()

export function getWebSocketManager(applicationId: string): WebSocketManager {
  if (!wsManagers.has(applicationId)) {
    wsManagers.set(applicationId, new WebSocketManager(applicationId))
  }
  return wsManagers.get(applicationId)!
}

export function disconnectWebSocketManager(applicationId: string) {
  const manager = wsManagers.get(applicationId)
  if (manager) {
    manager.disconnect()
    wsManagers.delete(applicationId)
  }
}