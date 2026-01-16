import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import { sha256 } from "js-sha256"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// 格式化内存大小，自动选择合适的单位
export function formatMemory(bytes: number): string {
  if (bytes === 0) return "0 B"
  
  const units = ["B", "KB", "MB", "GB", "TB", "PB"]
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + units[i]
}

// 格式化数字，保留三位小数
export function formatNumber(num: number): string {
  return num.toFixed(3)
}

// 格式化时间，将 ISO 8601 格式转换为易读的格式
export function formatDateTime(dateString: string): string {
  try {
    const date = new Date(dateString)
    
    // 检查日期是否有效
    if (isNaN(date.getTime())) {
      return dateString // 如果解析失败，返回原始字符串
    }
    
    // 格式化为 YYYY-MM-DD HH:mm:ss
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    const hours = String(date.getHours()).padStart(2, '0')
    const minutes = String(date.getMinutes()).padStart(2, '0')
    const seconds = String(date.getSeconds()).padStart(2, '0')
    
    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  } catch (error) {
    return dateString // 如果出错，返回原始字符串
  }
}

// 格式化相对时间（如 "2小时前"、"刚刚"）
export function formatRelativeTime(dateString: string): string {
  try {
    const date = new Date(dateString)
    const now = new Date()
    
    // 检查日期是否有效
    if (isNaN(date.getTime())) {
      return dateString
    }
    
    const diffMs = now.getTime() - date.getTime()
    const diffSeconds = Math.floor(diffMs / 1000)
    const diffMinutes = Math.floor(diffSeconds / 60)
    const diffHours = Math.floor(diffMinutes / 60)
    const diffDays = Math.floor(diffHours / 24)
    
    if (diffSeconds < 60) {
      return "刚刚"
    } else if (diffMinutes < 60) {
      return `${diffMinutes}分钟前`
    } else if (diffHours < 24) {
      return `${diffHours}小时前`
    } else if (diffDays < 7) {
      return `${diffDays}天前`
    } else {
      // 超过7天，显示绝对时间
      return formatDateTime(dateString)
    }
  } catch (error) {
    return dateString
  }
}

/**
 * 对密码进行 SHA-256 哈希处理
 * 用于在传输前对密码进行哈希，避免明文传输
 * 使用 js-sha256 库，不依赖安全上下文（HTTPS）
 * 
 * @param password - 原始密码
 * @returns Promise<string> - 哈希后的密码（十六进制字符串）
 */
export async function hashPassword(password: string): Promise<string> {
  // 使用 js-sha256 进行 SHA-256 哈希，不依赖安全上下文
  return sha256(password)
}

/**
 * 验证重定向 URL 是否安全
 * 基于白名单验证，只允许跳转到应用内的相对路径
 * 
 * @param redirectUrl - 要验证的重定向 URL
 * @param defaultPath - 如果 URL 不安全，返回的默认路径
 * @returns 安全的相对路径
 */
export function validateRedirectUrl(redirectUrl: string | null, defaultPath: string = "/resources"): string {
  // 如果没有提供 redirect URL，返回默认路径
  if (!redirectUrl) {
    return defaultPath
  }

  // 解码 URL 编码
  let decodedUrl: string
  try {
    decodedUrl = decodeURIComponent(redirectUrl)
  } catch {
    // 如果解码失败，返回默认路径
    return defaultPath
  }

  // 白名单：允许的路径模式
  // 支持精确匹配和通配符模式
  const allowedPaths = [
    "/",                    // 首页
    "/resources",           // 资源管理
    "/applications",        // 应用管理
    "/applications/*",     // 应用详情页
    "/audit",               // 审计日志
    "/status",              // 状态监控
    "/users",               // 用户管理
  ]

  // 检查是否是绝对 URL（包含协议）
  if (decodedUrl.match(/^https?:\/\//i)) {
    // 不允许外部 URL
    return defaultPath
  }

  // 检查是否包含域名
  if (decodedUrl.match(/^\/\/[^\/]/)) {
    // 不允许协议相对 URL（//example.com）
    return defaultPath
  }

  // 检查路径遍历攻击（../）
  if (decodedUrl.includes("..")) {
    return defaultPath
  }

  // 确保是相对路径（以 / 开头）
  if (!decodedUrl.startsWith("/")) {
    return defaultPath
  }

  // 移除查询参数和锚点，只检查路径部分
  const pathOnly = decodedUrl.split("?")[0].split("#")[0]

  // 检查是否在白名单中
  for (const allowedPath of allowedPaths) {
    if (allowedPath.endsWith("/*")) {
      // 通配符匹配：检查路径是否以允许的前缀开头
      const prefix = allowedPath.slice(0, -2)
      if (pathOnly === prefix || pathOnly.startsWith(prefix + "/")) {
        return pathOnly
      }
    } else {
      // 精确匹配
      if (pathOnly === allowedPath) {
        return pathOnly
      }
    }
  }

  // 如果不在白名单中，返回默认路径
  return defaultPath
}
