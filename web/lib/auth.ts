"use client"

import { sha256 } from "js-sha256"

/**
 * 对密码进行 SHA-256 哈希处理
 * 使用 js-sha256 库，不依赖安全上下文（HTTPS），可在 HTTP 环境下使用
 * 
 * @param password - 原始密码
 * @returns Promise<string> - 哈希后的密码（十六进制字符串）
 */
export async function hashPassword(password: string): Promise<string> {
  // 使用 js-sha256 进行 SHA-256 哈希，不依赖安全上下文
  return sha256(password)
}

export async function verifyPassword(password: string, hash: string): Promise<boolean> {
  const calculated = await hashPassword(password)
  return calculated === hash
}

