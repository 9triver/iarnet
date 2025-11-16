import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

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
